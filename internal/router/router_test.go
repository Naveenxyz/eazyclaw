package router

import (
	"testing"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
)

func TestRouterIsAllowedUsesUserIDWhenPresent(t *testing.T) {
	r := NewRouter(config.ChannelsConfig{
		Discord: config.DiscordChannelConfig{
			AllowedUsers: []string{"12345"},
		},
	})

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
