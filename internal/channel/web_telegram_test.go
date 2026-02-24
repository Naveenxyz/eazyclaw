package channel

import (
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/state"
)

func TestChannelTelegramSnapshotReadsPoliciesAndPendingFromStore(t *testing.T) {
	store := mustOpenStore(t)
	store.SetPolicy("telegram", "group_policy", "allowlist")
	store.SetPolicy("telegram", "dm_policy", "deny")
	store.AddAllowedUser("telegram", "u1")
	store.UpsertPending("telegram", state.PendingApproval{
		UserID:      "u-pending",
		Username:    "tester",
		Preview:     "needs approval",
		FirstSeenAt: time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
	})

	s := channelTelegramSnapshot(&config.ChannelsConfig{}, nil, store)
	if s.GroupPolicy != "allowlist" {
		t.Fatalf("expected group policy from store, got %q", s.GroupPolicy)
	}
	if s.DMPolicy != "deny" {
		t.Fatalf("expected dm policy from store, got %q", s.DMPolicy)
	}
	if len(s.AllowedUsers) != 1 {
		t.Fatalf("expected 1 allowed user, got %d", len(s.AllowedUsers))
	}
	if len(s.Pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(s.Pending))
	}
}
