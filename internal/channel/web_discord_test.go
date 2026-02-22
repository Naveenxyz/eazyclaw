package channel

import (
	"testing"

	"github.com/eazyclaw/eazyclaw/internal/config"
)

func TestChannelDiscordSnapshotInitializesSlices(t *testing.T) {
	state := channelDiscordSnapshot(&config.ChannelsConfig{}, nil)

	if state.AllowedUsers == nil {
		t.Fatalf("allowed users should be initialized")
	}
	if state.Pending == nil {
		t.Fatalf("pending approvals should be initialized")
	}
}
