package router

import (
	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/state"
)

// Router determines session routing and access control for inbound messages.
type Router struct {
	store *state.Store
}

// NewRouter creates a new Router backed by a state store.
func NewRouter(store *state.Store) *Router {
	return &Router{store: store}
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
	userID := msg.UserID
	if userID == "" {
		userID = msg.SenderID
	}

	ok, err := r.store.IsAllowed(msg.ChannelID, userID)
	if err != nil {
		return false
	}
	if ok {
		return true
	}

	// Empty allowlist means allow all.
	users, err := r.store.AllowedUsers(msg.ChannelID)
	if err != nil {
		return false
	}
	return len(users) == 0
}
