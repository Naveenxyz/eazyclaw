package channel

import (
	"context"

	"github.com/eazyclaw/eazyclaw/internal/bus"
)

// Channel is the interface for messaging platform integrations.
type Channel interface {
	// Name returns the channel identifier (e.g., "telegram", "discord").
	Name() string

	// Start begins listening for messages and pushes them to the bus.
	Start(ctx context.Context, b *bus.Bus) error

	// Send sends an outbound message through this channel.
	Send(ctx context.Context, msg bus.OutboundMessage) error

	// Stop gracefully shuts down the channel.
	Stop() error
}
