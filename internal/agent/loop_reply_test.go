package agent

import (
	"testing"

	"github.com/eazyclaw/eazyclaw/internal/bus"
)

func TestSendReplyUsesDeliveryOverrideForSyntheticMessages(t *testing.T) {
	b := bus.New(1)
	loop := &AgentLoop{bus: b}

	loop.sendReply(bus.Message{
		ID:             "m1",
		ChannelID:      "cron",
		SenderID:       "cron",
		ReplyChannelID: "telegram",
		ReplyChatID:    "123",
	}, "hello")

	msg := <-b.Outbound
	if msg.ChannelID != "telegram" || msg.ChatID != "123" {
		t.Fatalf("unexpected outbound routing: %#v", msg)
	}
}

func TestSendReplyFallsBackToOriginChatWhenNoOverride(t *testing.T) {
	b := bus.New(1)
	loop := &AgentLoop{bus: b}

	loop.sendReply(bus.Message{
		ID:        "m2",
		ChannelID: "discord",
		SenderID:  "dm-channel",
	}, "hello")

	msg := <-b.Outbound
	if msg.ChannelID != "discord" || msg.ChatID != "dm-channel" {
		t.Fatalf("unexpected outbound routing: %#v", msg)
	}
}
