package channel

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/state"
)

func mustOpenStore(t *testing.T) *state.Store {
	t.Helper()
	s, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestDiscordDMUserAllowed_AllowlistPolicyFailsClosed(t *testing.T) {
	store := mustOpenStore(t)
	d := NewDiscordChannel("", config.DiscordChannelConfig{
		GroupPolicy:  "allowlist",
		AllowedUsers: []string{},
		DM: config.DiscordDMConfig{
			Policy: "allow",
		},
	}, store)

	if d.isDMUserAllowed("123") {
		t.Fatalf("expected DM to be denied when group_policy=allowlist and user is not allowlisted")
	}
}

func TestDiscordDMUserAllowed_AllowlistPolicyAllowsListedUser(t *testing.T) {
	store := mustOpenStore(t)
	store.AddAllowedUser("discord", "123")
	d := NewDiscordChannel("", config.DiscordChannelConfig{
		GroupPolicy:  "allowlist",
		AllowedUsers: []string{"123"},
		DM: config.DiscordDMConfig{
			Policy: "allow",
		},
	}, store)

	if !d.isDMUserAllowed("123") {
		t.Fatalf("expected allowlisted DM user to be allowed")
	}
}

func TestDiscordDMUserAllowed_OpenPolicyAllowsWhenNoAllowlist(t *testing.T) {
	store := mustOpenStore(t)
	d := NewDiscordChannel("", config.DiscordChannelConfig{
		GroupPolicy:  "open",
		AllowedUsers: []string{},
		DM: config.DiscordDMConfig{
			Policy: "allow",
		},
	}, store)

	if !d.isDMUserAllowed("123") {
		t.Fatalf("expected DM user to be allowed in open policy without user allowlist")
	}
}

func TestDiscordDisallowedDMCreatesPendingApproval(t *testing.T) {
	store := mustOpenStore(t)
	d := NewDiscordChannel("", config.DiscordChannelConfig{
		GroupPolicy:  "allowlist",
		AllowedUsers: []string{},
		DM: config.DiscordDMConfig{
			Policy: "allow",
		},
	}, store)

	d.messageCreateHandler(nil, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "m1",
			ChannelID: "dm1",
			Content:   "hello from dm",
			Author: &discordgo.User{
				ID:       "u-1",
				Username: "tester",
			},
			Timestamp: time.Now().UTC(),
		},
	})

	state := d.Snapshot()
	if len(state.Pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(state.Pending))
	}
	if state.Pending[0].UserID != "u-1" {
		t.Fatalf("unexpected pending user id: %s", state.Pending[0].UserID)
	}
}

func TestDiscordApproveUserRemovesPendingAndAllowsDM(t *testing.T) {
	store := mustOpenStore(t)
	d := NewDiscordChannel("", config.DiscordChannelConfig{
		GroupPolicy:  "allowlist",
		AllowedUsers: []string{},
		DM: config.DiscordDMConfig{
			Policy: "allow",
		},
	}, store)
	d.recordPendingDM("u-1", "tester", "hi", time.Now())

	if !d.ApproveUser("u-1") {
		t.Fatalf("expected approve to succeed")
	}

	state := d.Snapshot()
	if len(state.Pending) != 0 {
		t.Fatalf("expected pending approvals to be empty")
	}
	if !d.isDMUserAllowed("u-1") {
		t.Fatalf("expected approved user to be allowlisted for DM")
	}
}
