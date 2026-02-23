package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/provider"
)

// Session holds conversation history for a channel+chat pair.
type Session struct {
	ID                         string             `json:"id"`
	Messages                   []provider.Message `json:"messages"`
	CompactionCount            int                `json:"compaction_count,omitempty"`
	MemoryFlushCompactionCount *int               `json:"memory_flush_compaction_count,omitempty"`
	LastPromptTokens           int                `json:"last_prompt_tokens,omitempty"`
	Created                    time.Time          `json:"created"`
	Updated                    time.Time          `json:"updated"`
}

// SessionStore manages persistence of sessions to disk.
type SessionStore struct {
	basePath string
	mu       sync.Map // map[string]*sync.Mutex, per-session locks
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore(basePath string) *SessionStore {
	return &SessionStore{
		basePath: basePath,
	}
}

// sessionMutex returns a per-session mutex.
func (s *SessionStore) sessionMutex(id string) *sync.Mutex {
	val, _ := s.mu.LoadOrStore(id, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// Load reads a session from disk, creating a new one if it does not exist.
func (s *SessionStore) Load(id string) (*Session, error) {
	mu := s.sessionMutex(id)
	mu.Lock()
	defer mu.Unlock()

	path := s.filePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			now := time.Now()
			return &Session{
				ID:       id,
				Messages: []provider.Message{},
				Created:  now,
				Updated:  now,
			}, nil
		}
		return nil, fmt.Errorf("session: failed to read %s: %w", path, err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("session: failed to unmarshal %s: %w", path, err)
	}
	return &session, nil
}

// Save writes a session to disk as JSON.
func (s *SessionStore) Save(session *Session) error {
	mu := s.sessionMutex(session.ID)
	mu.Lock()
	defer mu.Unlock()

	session.Updated = time.Now()

	if err := os.MkdirAll(s.basePath, 0o755); err != nil {
		return fmt.Errorf("session: failed to create directory %s: %w", s.basePath, err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("session: failed to marshal session %s: %w", session.ID, err)
	}

	path := s.filePath(session.ID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("session: failed to write %s: %w", path, err)
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

// ListSessions returns a summary of all persisted sessions.
func (s *SessionStore) ListSessions() ([]SessionSummary, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("session: failed to read directory %s: %w", s.basePath, err)
	}

	var summaries []SessionSummary
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.basePath, entry.Name()))
		if err != nil {
			continue
		}
		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		summaries = append(summaries, SessionSummary{
			ID:           session.ID,
			MessageCount: len(session.Messages),
			Created:      session.Created,
			Updated:      session.Updated,
		})
	}
	return summaries, nil
}

// filePath returns the file path for a session ID.
// Session IDs may contain colons, which are replaced with underscores for filesystem safety.
func (s *SessionStore) filePath(id string) string {
	safe := sanitizeID(id)
	return filepath.Join(s.basePath, safe+".json")
}

// sanitizeID replaces characters that are problematic in filenames.
func sanitizeID(id string) string {
	var result []byte
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c == ':' || c == '/' || c == '\\' {
			result = append(result, '_')
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
