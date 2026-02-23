package state

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/config"
	_ "modernc.org/sqlite"
)

// PendingApproval represents a user awaiting admin approval.
type PendingApproval struct {
	UserID       string
	Username     string
	Preview      string
	MessageCount int
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
}

// Store provides SQLite-backed persistence for mutable runtime state
// (allowlists, policies, pending approvals).
type Store struct {
	db *sql.DB
}

// Open creates or opens the state database at {dataDir}/state.db.
func Open(dataDir string) (*Store, error) {
	return OpenPath(dataDir + "/state.db")
}

// OpenPath creates or opens the state database at the given path.
// Use ":memory:" for an in-memory database (useful in tests).
func OpenPath(path string) (*Store, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	if path == ":memory:" {
		dsn = ":memory:"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("state: open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("state: migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS allowed_users (
			channel    TEXT NOT NULL,
			user_id    TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (channel, user_id)
		);

		CREATE TABLE IF NOT EXISTS channel_policies (
			channel TEXT NOT NULL,
			key     TEXT NOT NULL,
			value   TEXT NOT NULL,
			PRIMARY KEY (channel, key)
		);

		CREATE TABLE IF NOT EXISTS pending_approvals (
			channel       TEXT    NOT NULL,
			user_id       TEXT    NOT NULL,
			username      TEXT    NOT NULL DEFAULT '',
			preview       TEXT    NOT NULL DEFAULT '',
			message_count INTEGER NOT NULL DEFAULT 1,
			first_seen_at TEXT    NOT NULL,
			last_seen_at  TEXT    NOT NULL,
			PRIMARY KEY (channel, user_id)
		);
	`)
	return err
}

// ---------- Allowed Users ----------

// AllowedUsers returns all allowed user IDs for a channel.
func (s *Store) AllowedUsers(channel string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT user_id FROM allowed_users WHERE channel = ? ORDER BY user_id`,
		channel,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IsAllowed checks whether a user is in the allowlist for a channel.
func (s *Store) IsAllowed(channel, userID string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM allowed_users WHERE channel = ? AND user_id = ?`,
		channel, userID,
	).Scan(&n)
	return n > 0, err
}

// AddAllowedUser adds a user to the allowlist (no-op if already present).
func (s *Store) AddAllowedUser(channel, userID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO allowed_users (channel, user_id) VALUES (?, ?)`,
		channel, userID,
	)
	return err
}

// RemoveAllowedUser removes a user from the allowlist.
func (s *Store) RemoveAllowedUser(channel, userID string) error {
	_, err := s.db.Exec(
		`DELETE FROM allowed_users WHERE channel = ? AND user_id = ?`,
		channel, userID,
	)
	return err
}

// SetAllowedUsers replaces the entire allowlist for a channel atomically.
func (s *Store) SetAllowedUsers(channel string, users []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM allowed_users WHERE channel = ?`, channel); err != nil {
		return err
	}
	for _, u := range users {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO allowed_users (channel, user_id) VALUES (?, ?)`,
			channel, u,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------- Policies ----------

// Policy returns the value of a policy key for a channel.
// Returns empty string if not set.
func (s *Store) Policy(channel, key string) (string, error) {
	var val string
	err := s.db.QueryRow(
		`SELECT value FROM channel_policies WHERE channel = ? AND key = ?`,
		channel, key,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetPolicy sets a policy key/value for a channel (upsert).
func (s *Store) SetPolicy(channel, key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO channel_policies (channel, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(channel, key) DO UPDATE SET value = excluded.value`,
		channel, key, value,
	)
	return err
}

// ---------- Pending Approvals ----------

// PendingApprovals returns all pending approvals for a channel, ordered by last_seen_at desc.
func (s *Store) PendingApprovals(channel string) ([]PendingApproval, error) {
	rows, err := s.db.Query(
		`SELECT user_id, username, preview, message_count, first_seen_at, last_seen_at
		 FROM pending_approvals WHERE channel = ? ORDER BY last_seen_at DESC`,
		channel,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var approvals []PendingApproval
	for rows.Next() {
		var p PendingApproval
		var first, last string
		if err := rows.Scan(&p.UserID, &p.Username, &p.Preview, &p.MessageCount, &first, &last); err != nil {
			return nil, err
		}
		p.FirstSeenAt, _ = time.Parse(time.RFC3339, first)
		p.LastSeenAt, _ = time.Parse(time.RFC3339, last)
		approvals = append(approvals, p)
	}
	return approvals, rows.Err()
}

// UpsertPending inserts or updates a pending approval. On conflict, increments
// message_count and updates preview/username/last_seen_at.
func (s *Store) UpsertPending(channel string, p PendingApproval) error {
	_, err := s.db.Exec(
		`INSERT INTO pending_approvals (channel, user_id, username, preview, message_count, first_seen_at, last_seen_at)
		 VALUES (?, ?, ?, ?, 1, ?, ?)
		 ON CONFLICT(channel, user_id) DO UPDATE SET
			username      = CASE WHEN excluded.username != '' THEN excluded.username ELSE pending_approvals.username END,
			preview       = excluded.preview,
			message_count = pending_approvals.message_count + 1,
			last_seen_at  = excluded.last_seen_at`,
		channel, p.UserID, p.Username, p.Preview,
		p.FirstSeenAt.Format(time.RFC3339), p.LastSeenAt.Format(time.RFC3339),
	)
	return err
}

// DeletePending removes a pending approval.
func (s *Store) DeletePending(channel, userID string) error {
	_, err := s.db.Exec(
		`DELETE FROM pending_approvals WHERE channel = ? AND user_id = ?`,
		channel, userID,
	)
	return err
}

// ---------- Seed Migration ----------

// SeedFromConfig seeds the store from config on first boot.
// It is idempotent: if rows already exist for a channel, it skips that channel.
func (s *Store) SeedFromConfig(channels config.ChannelsConfig) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	seed := func(channel string, users []string, groupPolicy, dmPolicy string) error {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM allowed_users WHERE channel = ?`, channel).Scan(&count); err != nil {
			return err
		}
		// Also check policies to determine if this channel was seeded
		var policyCount int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM channel_policies WHERE channel = ?`, channel).Scan(&policyCount); err != nil {
			return err
		}

		if count == 0 && policyCount == 0 {
			for _, u := range users {
				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}
				if _, err := tx.Exec(
					`INSERT OR IGNORE INTO allowed_users (channel, user_id) VALUES (?, ?)`,
					channel, u,
				); err != nil {
					return err
				}
			}
			if groupPolicy != "" {
				if _, err := tx.Exec(
					`INSERT OR IGNORE INTO channel_policies (channel, key, value) VALUES (?, 'group_policy', ?)`,
					channel, groupPolicy,
				); err != nil {
					return err
				}
			}
			if dmPolicy != "" {
				if _, err := tx.Exec(
					`INSERT OR IGNORE INTO channel_policies (channel, key, value) VALUES (?, 'dm_policy', ?)`,
					channel, dmPolicy,
				); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := seed("discord", channels.Discord.AllowedUsers, channels.Discord.GroupPolicy, channels.Discord.DM.Policy); err != nil {
		return err
	}
	if err := seed("telegram", channels.Telegram.AllowedUsers, channels.Telegram.GroupPolicy, channels.Telegram.DM.Policy); err != nil {
		return err
	}
	if err := seed("whatsapp", channels.WhatsApp.AllowedUsers, channels.WhatsApp.GroupPolicy, channels.WhatsApp.DM.Policy); err != nil {
		return err
	}

	return tx.Commit()
}
