package channel

import (
	"testing"

	"github.com/eazyclaw/eazyclaw/internal/config"
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
