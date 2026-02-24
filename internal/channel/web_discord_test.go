package channel

import (
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/state"
)

func TestChannelDiscordSnapshotInitializesSlices(t *testing.T) {
	state := channelDiscordSnapshot(&config.ChannelsConfig{}, nil, nil)

	if state.AllowedUsers == nil {
		t.Fatalf("allowed users should be initialized")
	}
	if state.Pending == nil {
		t.Fatalf("pending approvals should be initialized")
	}
}

func TestChannelDiscordSnapshotReadsFromStore(t *testing.T) {
	store := mustOpenStore(t)
	store.AddAllowedUser("discord", "u1")
	store.AddAllowedUser("discord", "u2")

	state := channelDiscordSnapshot(&config.ChannelsConfig{}, nil, store)

	if len(state.AllowedUsers) != 2 {
		t.Fatalf("expected 2 users from store, got %d", len(state.AllowedUsers))
	}
}

func TestChannelDiscordSnapshotReadsPoliciesAndPendingFromStore(t *testing.T) {
	store := mustOpenStore(t)
	store.SetPolicy("discord", "group_policy", "allowlist")
	store.SetPolicy("discord", "dm_policy", "deny")
	store.UpsertPending("discord", state.PendingApproval{
		UserID:      "u-pending",
		Username:    "tester",
		Preview:     "needs approval",
		FirstSeenAt: time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
	})

	s := channelDiscordSnapshot(&config.ChannelsConfig{}, nil, store)
	if s.GroupPolicy != "allowlist" {
		t.Fatalf("expected group policy from store, got %q", s.GroupPolicy)
	}
	if s.DMPolicy != "deny" {
		t.Fatalf("expected dm policy from store, got %q", s.DMPolicy)
	}
	if len(s.Pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(s.Pending))
	}
	if s.Pending[0].UserID != "u-pending" {
		t.Fatalf("unexpected pending user id: %s", s.Pending[0].UserID)
	}
}
