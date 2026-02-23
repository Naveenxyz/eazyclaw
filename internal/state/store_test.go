package state

import (
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/config"
)

func mustOpen(t *testing.T) *Store {
	t.Helper()
	s, err := OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAllowedUsersEmpty(t *testing.T) {
	s := mustOpen(t)
	users, err := s.AllowedUsers("discord")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty, got %v", users)
	}
}

func TestAddAndIsAllowed(t *testing.T) {
	s := mustOpen(t)
	if err := s.AddAllowedUser("discord", "123"); err != nil {
		t.Fatal(err)
	}
	ok, err := s.IsAllowed("discord", "123")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected allowed")
	}
	ok, _ = s.IsAllowed("discord", "999")
	if ok {
		t.Fatal("expected not allowed")
	}
	// Cross-channel isolation
	ok, _ = s.IsAllowed("telegram", "123")
	if ok {
		t.Fatal("expected not allowed on different channel")
	}
}

func TestAddAllowedUserIdempotent(t *testing.T) {
	s := mustOpen(t)
	if err := s.AddAllowedUser("discord", "123"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddAllowedUser("discord", "123"); err != nil {
		t.Fatal("duplicate insert should not fail:", err)
	}
	users, _ := s.AllowedUsers("discord")
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestRemoveAllowedUser(t *testing.T) {
	s := mustOpen(t)
	s.AddAllowedUser("discord", "123")
	if err := s.RemoveAllowedUser("discord", "123"); err != nil {
		t.Fatal(err)
	}
	ok, _ := s.IsAllowed("discord", "123")
	if ok {
		t.Fatal("expected removed")
	}
}

func TestSetAllowedUsersReplacesAll(t *testing.T) {
	s := mustOpen(t)
	s.AddAllowedUser("discord", "old1")
	s.AddAllowedUser("discord", "old2")

	if err := s.SetAllowedUsers("discord", []string{"new1", "new2", "new3"}); err != nil {
		t.Fatal(err)
	}
	users, _ := s.AllowedUsers("discord")
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
	ok, _ := s.IsAllowed("discord", "old1")
	if ok {
		t.Fatal("old user should be removed")
	}
}

func TestPolicy(t *testing.T) {
	s := mustOpen(t)
	val, err := s.Policy("discord", "group_policy")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Fatalf("expected empty, got %q", val)
	}

	if err := s.SetPolicy("discord", "group_policy", "allowlist"); err != nil {
		t.Fatal(err)
	}
	val, _ = s.Policy("discord", "group_policy")
	if val != "allowlist" {
		t.Fatalf("expected allowlist, got %q", val)
	}

	// Upsert
	if err := s.SetPolicy("discord", "group_policy", "open"); err != nil {
		t.Fatal(err)
	}
	val, _ = s.Policy("discord", "group_policy")
	if val != "open" {
		t.Fatalf("expected open, got %q", val)
	}
}

func TestPendingApprovals(t *testing.T) {
	s := mustOpen(t)

	now := time.Now().Truncate(time.Second)
	p := PendingApproval{
		UserID:      "u1",
		Username:    "tester",
		Preview:     "hello",
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
	if err := s.UpsertPending("discord", p); err != nil {
		t.Fatal(err)
	}

	approvals, err := s.PendingApprovals("discord")
	if err != nil {
		t.Fatal(err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected 1, got %d", len(approvals))
	}
	if approvals[0].UserID != "u1" || approvals[0].MessageCount != 1 {
		t.Fatalf("unexpected: %+v", approvals[0])
	}

	// Upsert increments count
	p.LastSeenAt = now.Add(time.Minute)
	p.Preview = "second msg"
	if err := s.UpsertPending("discord", p); err != nil {
		t.Fatal(err)
	}
	approvals, _ = s.PendingApprovals("discord")
	if approvals[0].MessageCount != 2 {
		t.Fatalf("expected count 2, got %d", approvals[0].MessageCount)
	}
	if approvals[0].Preview != "second msg" {
		t.Fatalf("expected updated preview, got %q", approvals[0].Preview)
	}

	// Delete
	if err := s.DeletePending("discord", "u1"); err != nil {
		t.Fatal(err)
	}
	approvals, _ = s.PendingApprovals("discord")
	if len(approvals) != 0 {
		t.Fatal("expected empty after delete")
	}
}

func TestSeedFromConfigIdempotent(t *testing.T) {
	s := mustOpen(t)
	cfg := config.ChannelsConfig{
		Discord: config.DiscordChannelConfig{
			AllowedUsers: []string{"d1", "d2"},
			GroupPolicy:  "allowlist",
			DM:           config.DiscordDMConfig{Policy: "allow"},
		},
		Telegram: config.TelegramChannelConfig{
			AllowedUsers: []string{"t1"},
			GroupPolicy:  "open",
			DM:           config.TelegramDMConfig{Policy: "deny"},
		},
		WhatsApp: config.WhatsAppChannelConfig{
			AllowedUsers: []string{},
			GroupPolicy:  "allowlist",
			DM:           config.WhatsAppDMConfig{Policy: "allow"},
		},
	}

	if err := s.SeedFromConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Verify Discord seeded
	users, _ := s.AllowedUsers("discord")
	if len(users) != 2 {
		t.Fatalf("expected 2 discord users, got %d", len(users))
	}
	val, _ := s.Policy("discord", "group_policy")
	if val != "allowlist" {
		t.Fatalf("expected allowlist, got %q", val)
	}

	// Verify Telegram seeded
	users, _ = s.AllowedUsers("telegram")
	if len(users) != 1 {
		t.Fatalf("expected 1 telegram user, got %d", len(users))
	}

	// Verify WhatsApp policies seeded
	val, _ = s.Policy("whatsapp", "dm_policy")
	if val != "allow" {
		t.Fatalf("expected allow, got %q", val)
	}

	// Now modify and re-seed — should be no-op
	cfg.Discord.AllowedUsers = []string{"d1", "d2", "d3"}
	if err := s.SeedFromConfig(cfg); err != nil {
		t.Fatal(err)
	}
	users, _ = s.AllowedUsers("discord")
	if len(users) != 2 {
		t.Fatalf("re-seed should not add users, got %d", len(users))
	}
}
