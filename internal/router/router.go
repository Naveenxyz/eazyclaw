package router

import (
	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
)

// Router determines session routing and access control for inbound messages.
type Router struct {
	allowedUsers map[string]map[string]bool // channel -> set of allowed user IDs
}

// NewRouter creates a new Router from channel configuration.
func NewRouter(cfg config.ChannelsConfig) *Router {
	allowed := make(map[string]map[string]bool)

	if len(cfg.Telegram.AllowedUsers) > 0 {
		m := make(map[string]bool, len(cfg.Telegram.AllowedUsers))
		for _, u := range cfg.Telegram.AllowedUsers {
			m[u] = true
		}
		allowed["telegram"] = m
	}

	if len(cfg.Discord.AllowedUsers) > 0 {
		m := make(map[string]bool, len(cfg.Discord.AllowedUsers))
		for _, u := range cfg.Discord.AllowedUsers {
			m[u] = true
		}
		allowed["discord"] = m
	}

	return &Router{
		allowedUsers: allowed,
	}
}

// SessionID returns a unique session identifier for a message.
// For group messages, the session is per-group. For DMs, it is per-sender.
func (r *Router) SessionID(msg bus.Message) string {
	if msg.GroupID != "" {
		return msg.ChannelID + ":" + msg.GroupID
	}
	return msg.ChannelID + ":" + msg.SenderID
}

// IsAllowed checks whether the sender of a message is permitted.
// If no allowlist is configured for the channel, all users are allowed.
func (r *Router) IsAllowed(msg bus.Message) bool {
	userSet, exists := r.allowedUsers[msg.ChannelID]
	if !exists {
		// No allowlist for this channel means all users are allowed.
		return true
	}
	userID := msg.UserID
	if userID == "" {
		userID = msg.SenderID
	}
	return userSet[userID]
}
