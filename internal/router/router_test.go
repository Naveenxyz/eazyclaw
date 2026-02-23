package router

import (
	"testing"

	"github.com/eazyclaw/eazyclaw/internal/bus"
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

func TestRouterIsAllowedUsesUserIDWhenPresent(t *testing.T) {
	store := mustOpenStore(t)
	store.AddAllowedUser("discord", "12345")

	r := NewRouter(store)

	// Discord DMs use channel-id as SenderID for reply routing; allowlist should
	// still evaluate the real author via UserID.
	msg := bus.Message{
		ChannelID: "discord",
		SenderID:  "dm-channel-999",
		UserID:    "12345",
	}
	if !r.IsAllowed(msg) {
		t.Fatalf("expected allowed when UserID is allowlisted")
	}
}

func TestRouterIsAllowedEmptyAllowlistAllowsAll(t *testing.T) {
	store := mustOpenStore(t)
	r := NewRouter(store)

	msg := bus.Message{
		ChannelID: "discord",
		SenderID:  "anyone",
		UserID:    "anyone",
	}
	if !r.IsAllowed(msg) {
		t.Fatalf("expected allowed when no allowlist is configured")
	}
}

func TestRouterIsAllowedDeniesUnlisted(t *testing.T) {
	store := mustOpenStore(t)
	store.AddAllowedUser("discord", "12345")
	r := NewRouter(store)

	msg := bus.Message{
		ChannelID: "discord",
		SenderID:  "99999",
		UserID:    "99999",
	}
	if r.IsAllowed(msg) {
		t.Fatalf("expected denied when user is not in allowlist")
	}
}

func TestRouterReflectsLiveUpdates(t *testing.T) {
	store := mustOpenStore(t)
	store.AddAllowedUser("discord", "12345")
	r := NewRouter(store)

	msg := bus.Message{
		ChannelID: "discord",
		SenderID:  "new-user",
		UserID:    "new-user",
	}
	if r.IsAllowed(msg) {
		t.Fatalf("expected denied before adding")
	}

	// Add new user to store — router should reflect immediately.
	store.AddAllowedUser("discord", "new-user")
	if !r.IsAllowed(msg) {
		t.Fatalf("expected allowed after adding to store")
	}
}
