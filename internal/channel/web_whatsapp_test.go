package channel

import (
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/state"
)

func TestChannelWhatsAppSnapshotReadsStoreStateWithoutLiveChannel(t *testing.T) {
	store := mustOpenStore(t)
	store.SetPolicy("whatsapp", "group_policy", "allowlist")
	store.SetPolicy("whatsapp", "dm_policy", "deny")
	store.AddAllowedUser("whatsapp", "15550000100")
	store.UpsertPending("whatsapp", state.PendingApproval{
		UserID:      "15550000999",
		Username:    "tester",
		Preview:     "need approval",
		FirstSeenAt: time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
	})

	s := channelWhatsAppSnapshot(&config.ChannelsConfig{}, nil, store)
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
	if s.Pending[0].UserID != "15550000999" {
		t.Fatalf("unexpected pending user id: %s", s.Pending[0].UserID)
	}
}
