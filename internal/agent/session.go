package agent

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/provider"
	_ "modernc.org/sqlite"
)

// Session holds conversation history for a channel+chat pair.
type Session struct {
	ID                         string             `json:"id"`
	Messages                   []provider.Message `json:"messages"`
	CompactionCount            int                `json:"compaction_count,omitempty"`
	MemoryFlushCompactionCount *int               `json:"memory_flush_compaction_count,omitempty"`
	LastPromptTokens           int                `json:"last_prompt_tokens,omitempty"`
	TotalInputTokens           int                `json:"total_input_tokens,omitempty"`
	TotalOutputTokens          int                `json:"total_output_tokens,omitempty"`
	LastTurnInputTokens        int                `json:"last_turn_input_tokens,omitempty"`
	LastTurnOutputTokens       int                `json:"last_turn_output_tokens,omitempty"`
	Created                    time.Time          `json:"created"`
	Updated                    time.Time          `json:"updated"`
}

// SessionMessagesPage is a keyset-paginated view of one session's messages.
type SessionMessagesPage struct {
	ID            string             `json:"id"`
	Messages      []provider.Message `json:"messages"`
	Created       time.Time          `json:"created"`
	Updated       time.Time          `json:"updated"`
	TotalMessages int                `json:"total_messages"`
	Limit         int                `json:"limit"`
	HasMore       bool               `json:"has_more"`
	NextBeforeSeq *int64             `json:"next_before_seq,omitempty"`
}

// SessionStore manages SQLite-backed persistence of sessions.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore(basePath string) (*SessionStore, error) {
	if stringsTrim(basePath) == "" {
		return nil, errors.New("session: base path is required")
	}

	dbPath := basePath
	if basePath != ":memory:" {
		if err := os.MkdirAll(basePath, 0o755); err != nil {
			return nil, fmt.Errorf("session: create directory %s: %w", basePath, err)
		}
		dbPath = filepath.Join(basePath, "sessions.db")
	}

	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("session: open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := migrateSessionStore(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("session: migrate: %w", err)
	}

	return &SessionStore{db: db}, nil
}

func migrateSessionStore(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id                            TEXT PRIMARY KEY,
			compaction_count              INTEGER NOT NULL DEFAULT 0,
			memory_flush_compaction_count INTEGER,
			last_prompt_tokens            INTEGER NOT NULL DEFAULT 0,
			total_input_tokens            INTEGER NOT NULL DEFAULT 0,
			total_output_tokens           INTEGER NOT NULL DEFAULT 0,
			last_turn_input_tokens        INTEGER NOT NULL DEFAULT 0,
			last_turn_output_tokens       INTEGER NOT NULL DEFAULT 0,
			created_at                    TEXT NOT NULL,
			updated_at                    TEXT NOT NULL,
			message_count                 INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS session_messages (
			session_id  TEXT NOT NULL,
			seq         INTEGER NOT NULL,
			message_json TEXT NOT NULL,
			created_at  TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (session_id, seq),
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC, id);
		CREATE INDEX IF NOT EXISTS idx_session_messages_seq ON session_messages(session_id, seq DESC);
	`)
	if err != nil {
		return err
	}

	type colDef struct {
		name string
		def  string
	}
	requiredCols := []colDef{
		{name: "total_input_tokens", def: "INTEGER NOT NULL DEFAULT 0"},
		{name: "total_output_tokens", def: "INTEGER NOT NULL DEFAULT 0"},
		{name: "last_turn_input_tokens", def: "INTEGER NOT NULL DEFAULT 0"},
		{name: "last_turn_output_tokens", def: "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, c := range requiredCols {
		ok, err := sessionColumnExists(db, "sessions", c.name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN ` + c.name + ` ` + c.def); err != nil {
			return fmt.Errorf("session: add column %s: %w", c.name, err)
		}
	}
	return nil
}

// Close closes the underlying SQLite connection.
func (s *SessionStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Load reads a full session and all messages, creating an empty session model if absent.
func (s *SessionStore) Load(id string) (*Session, error) {
	_, session, err := s.loadSessionMeta(id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		now := time.Now().UTC()
		return &Session{
			ID:       id,
			Messages: []provider.Message{},
			Created:  now,
			Updated:  now,
		}, nil
	}

	rows, err := s.db.Query(
		`SELECT message_json FROM session_messages WHERE session_id = ? ORDER BY seq ASC`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("session: load messages for %s: %w", id, err)
	}
	defer rows.Close()

	session.Messages = make([]provider.Message, 0, 64)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("session: scan message for %s: %w", id, err)
		}
		var msg provider.Message
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			return nil, fmt.Errorf("session: decode message for %s: %w", id, err)
		}
		session.Messages = append(session.Messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate messages for %s: %w", id, err)
	}
	return session, nil
}

// Save writes a full session snapshot atomically.
func (s *SessionStore) Save(session *Session) error {
	if session == nil {
		return errors.New("session: save nil session")
	}
	if stringsTrim(session.ID) == "" {
		return errors.New("session: missing session ID")
	}
	now := time.Now().UTC()
	if session.Created.IsZero() {
		if _, existing, err := s.loadSessionMeta(session.ID); err == nil && existing != nil {
			session.Created = existing.Created
		}
	}
	if session.Created.IsZero() {
		session.Created = now
	}
	session.Updated = now

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("session: begin save tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO sessions (
			id, compaction_count, memory_flush_compaction_count, last_prompt_tokens, total_input_tokens, total_output_tokens, last_turn_input_tokens, last_turn_output_tokens, created_at, updated_at, message_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			compaction_count = excluded.compaction_count,
			memory_flush_compaction_count = excluded.memory_flush_compaction_count,
			last_prompt_tokens = excluded.last_prompt_tokens,
			total_input_tokens = excluded.total_input_tokens,
			total_output_tokens = excluded.total_output_tokens,
			last_turn_input_tokens = excluded.last_turn_input_tokens,
			last_turn_output_tokens = excluded.last_turn_output_tokens,
			updated_at = excluded.updated_at,
			message_count = excluded.message_count`,
		session.ID,
		session.CompactionCount,
		nullableInt(session.MemoryFlushCompactionCount),
		session.LastPromptTokens,
		session.TotalInputTokens,
		session.TotalOutputTokens,
		session.LastTurnInputTokens,
		session.LastTurnOutputTokens,
		session.Created.UTC().Format(time.RFC3339Nano),
		session.Updated.UTC().Format(time.RFC3339Nano),
		len(session.Messages),
	)
	if err != nil {
		return fmt.Errorf("session: upsert metadata for %s: %w", session.ID, err)
	}

	if _, err := tx.Exec(`DELETE FROM session_messages WHERE session_id = ?`, session.ID); err != nil {
		return fmt.Errorf("session: clear messages for %s: %w", session.ID, err)
	}

	stmt, err := tx.Prepare(`INSERT INTO session_messages (session_id, seq, message_json, created_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("session: prepare insert for %s: %w", session.ID, err)
	}
	defer stmt.Close()

	createdAt := now.Format(time.RFC3339Nano)
	for i, msg := range session.Messages {
		raw, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("session: encode message %d for %s: %w", i, session.ID, err)
		}
		if _, err := stmt.Exec(session.ID, i+1, string(raw), createdAt); err != nil {
			return fmt.Errorf("session: insert message %d for %s: %w", i, session.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("session: commit save for %s: %w", session.ID, err)
	}
	return nil
}

// Trim keeps only the last maxMessages messages in the session.
// If maxMessages is <= 0, it defaults to 100.
// After trimming, it sanitizes the result to fix orphaned tool messages.
func (s *SessionStore) Trim(session *Session, maxMessages int) {
	if maxMessages <= 0 {
		maxMessages = 100
	}
	if len(session.Messages) > maxMessages {
		session.Messages = session.Messages[len(session.Messages)-maxMessages:]
	}
	// Sanitize after trimming to repair any orphaned tool call/result pairs.
	session.Messages, _ = SanitizeMessages(session.Messages)
}

// SessionSummary holds metadata about a session for listing.
type SessionSummary struct {
	ID           string    `json:"id"`
	MessageCount int       `json:"message_count"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

// SessionListPage is an offset-paginated session summary set.
type SessionListPage struct {
	Items   []SessionSummary `json:"items"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	Total   int              `json:"total"`
	HasMore bool             `json:"has_more"`
}

// ListSessions returns all session summaries ordered by most recently updated.
func (s *SessionStore) ListSessions() ([]SessionSummary, error) {
	rows, err := s.db.Query(
		`SELECT id, message_count, created_at, updated_at FROM sessions ORDER BY updated_at DESC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("session: list sessions: %w", err)
	}
	defer rows.Close()

	summaries := make([]SessionSummary, 0, 64)
	for rows.Next() {
		var (
			id          string
			messageCnt  int
			createdText string
			updatedText string
		)
		if err := rows.Scan(&id, &messageCnt, &createdText, &updatedText); err != nil {
			return nil, fmt.Errorf("session: scan summary: %w", err)
		}
		created, err := parseTime(createdText)
		if err != nil {
			return nil, fmt.Errorf("session: parse created time for %s: %w", id, err)
		}
		updated, err := parseTime(updatedText)
		if err != nil {
			return nil, fmt.Errorf("session: parse updated time for %s: %w", id, err)
		}
		summaries = append(summaries, SessionSummary{
			ID:           id,
			MessageCount: messageCnt,
			Created:      created,
			Updated:      updated,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate summaries: %w", err)
	}
	return summaries, nil
}

// ListSessionsPage returns session summaries with offset pagination metadata.
func (s *SessionStore) ListSessionsPage(limit, offset int) (*SessionListPage, error) {
	limit = normalizeLimit(limit, 50, 200)
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&total); err != nil {
		return nil, fmt.Errorf("session: count sessions: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT id, message_count, created_at, updated_at
		 FROM sessions
		 ORDER BY updated_at DESC, id ASC
		 LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("session: list sessions page: %w", err)
	}
	defer rows.Close()

	items := make([]SessionSummary, 0, limit)
	for rows.Next() {
		var (
			id          string
			messageCnt  int
			createdText string
			updatedText string
		)
		if err := rows.Scan(&id, &messageCnt, &createdText, &updatedText); err != nil {
			return nil, fmt.Errorf("session: scan paged summary: %w", err)
		}
		created, err := parseTime(createdText)
		if err != nil {
			return nil, fmt.Errorf("session: parse created time for %s: %w", id, err)
		}
		updated, err := parseTime(updatedText)
		if err != nil {
			return nil, fmt.Errorf("session: parse updated time for %s: %w", id, err)
		}
		items = append(items, SessionSummary{
			ID:           id,
			MessageCount: messageCnt,
			Created:      created,
			Updated:      updated,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate paged summaries: %w", err)
	}

	return &SessionListPage{
		Items:   items,
		Limit:   limit,
		Offset:  offset,
		Total:   total,
		HasMore: offset+len(items) < total,
	}, nil
}

// LoadMessagesPage returns keyset-paginated messages for one session.
// If beforeSeq is nil, it returns the latest `limit` messages.
// If beforeSeq is set, it returns messages strictly older than beforeSeq.
func (s *SessionStore) LoadMessagesPage(id string, limit int, beforeSeq *int64) (*SessionMessagesPage, error) {
	limit = normalizeLimit(limit, 120, 500)

	meta, session, err := s.loadSessionMeta(id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		now := time.Now().UTC()
		return &SessionMessagesPage{
			ID:            id,
			Messages:      []provider.Message{},
			Created:       now,
			Updated:       now,
			TotalMessages: 0,
			Limit:         limit,
			HasMore:       false,
		}, nil
	}

	query := `SELECT seq, message_json FROM session_messages WHERE session_id = ?`
	args := []any{id}
	if beforeSeq != nil && *beforeSeq > 0 {
		query += ` AND seq < ?`
		args = append(args, *beforeSeq)
	}
	query += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("session: list paged messages for %s: %w", id, err)
	}
	defer rows.Close()

	descMsgs := make([]provider.Message, 0, limit)
	var minSeq int64
	for rows.Next() {
		var seq int64
		var raw string
		if err := rows.Scan(&seq, &raw); err != nil {
			return nil, fmt.Errorf("session: scan paged message for %s: %w", id, err)
		}
		if minSeq == 0 || seq < minSeq {
			minSeq = seq
		}
		var msg provider.Message
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			return nil, fmt.Errorf("session: decode paged message for %s: %w", id, err)
		}
		descMsgs = append(descMsgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: iterate paged messages for %s: %w", id, err)
	}

	messages := reverseMessages(descMsgs)
	hasMore := minSeq > 1
	var nextBefore *int64
	if hasMore {
		v := minSeq
		nextBefore = &v
	}

	return &SessionMessagesPage{
		ID:            session.ID,
		Messages:      messages,
		Created:       session.Created,
		Updated:       session.Updated,
		TotalMessages: meta.MessageCount,
		Limit:         limit,
		HasMore:       hasMore,
		NextBeforeSeq: nextBefore,
	}, nil
}

type sessionMeta struct {
	ID                         string
	CompactionCount            int
	MemoryFlushCompactionCount *int
	LastPromptTokens           int
	TotalInputTokens           int
	TotalOutputTokens          int
	LastTurnInputTokens        int
	LastTurnOutputTokens       int
	Created                    time.Time
	Updated                    time.Time
	MessageCount               int
}

func (s *SessionStore) loadSessionMeta(id string) (*sessionMeta, *Session, error) {
	var (
		meta                  sessionMeta
		memoryFlushCompaction sql.NullInt64
		createdText           string
		updatedText           string
	)
	err := s.db.QueryRow(
		`SELECT id, compaction_count, memory_flush_compaction_count, last_prompt_tokens, total_input_tokens, total_output_tokens, last_turn_input_tokens, last_turn_output_tokens, created_at, updated_at, message_count
		 FROM sessions WHERE id = ?`,
		id,
	).Scan(
		&meta.ID,
		&meta.CompactionCount,
		&memoryFlushCompaction,
		&meta.LastPromptTokens,
		&meta.TotalInputTokens,
		&meta.TotalOutputTokens,
		&meta.LastTurnInputTokens,
		&meta.LastTurnOutputTokens,
		&createdText,
		&updatedText,
		&meta.MessageCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("session: load metadata for %s: %w", id, err)
	}
	created, err := parseTime(createdText)
	if err != nil {
		return nil, nil, fmt.Errorf("session: parse created time for %s: %w", id, err)
	}
	updated, err := parseTime(updatedText)
	if err != nil {
		return nil, nil, fmt.Errorf("session: parse updated time for %s: %w", id, err)
	}
	meta.Created = created
	meta.Updated = updated
	if memoryFlushCompaction.Valid {
		v := int(memoryFlushCompaction.Int64)
		meta.MemoryFlushCompactionCount = &v
	}

	session := &Session{
		ID:                         meta.ID,
		Messages:                   []provider.Message{},
		CompactionCount:            meta.CompactionCount,
		MemoryFlushCompactionCount: meta.MemoryFlushCompactionCount,
		LastPromptTokens:           meta.LastPromptTokens,
		TotalInputTokens:           meta.TotalInputTokens,
		TotalOutputTokens:          meta.TotalOutputTokens,
		LastTurnInputTokens:        meta.LastTurnInputTokens,
		LastTurnOutputTokens:       meta.LastTurnOutputTokens,
		Created:                    meta.Created,
		Updated:                    meta.Updated,
	}
	return &meta, session, nil
}

func sessionColumnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, fmt.Errorf("session: table info %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			typ       string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("session: scan table info %s: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("session: iterate table info %s: %w", table, err)
	}
	return false, nil
}

func parseTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, v)
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func normalizeLimit(v, def, max int) int {
	if v <= 0 {
		v = def
	}
	if v > max {
		v = max
	}
	return v
}

func reverseMessages(in []provider.Message) []provider.Message {
	n := len(in)
	out := make([]provider.Message, n)
	for i := 0; i < n; i++ {
		out[i] = in[n-1-i]
	}
	return out
}

func stringsTrim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
