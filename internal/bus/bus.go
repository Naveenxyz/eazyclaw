package bus

import "time"

// Message represents an inbound message from a channel.
type Message struct {
	ID        string
	ChannelID string // "telegram" | "discord" | "web"
	SenderID  string // platform-specific user ID
	UserID    string // stable author ID when SenderID carries a chat/channel ID
	GroupID   string // empty for DMs
	Text      string
	Timestamp time.Time
	ReplyTo   string // message ID being replied to
}

// OutboundMessage represents a message to send back through a channel.
type OutboundMessage struct {
	ChannelID string
	ChatID    string // platform-specific chat ID (user or group)
	Text      string
	ReplyTo   string // message ID to reply to
}

// Bus provides typed channels for message passing between channels and the agent.
type Bus struct {
	Inbound  chan Message
	Outbound chan OutboundMessage
}

// New creates a new Bus with buffered channels.
func New(bufferSize int) *Bus {
	return &Bus{
		Inbound:  make(chan Message, bufferSize),
		Outbound: make(chan OutboundMessage, bufferSize),
	}
}
