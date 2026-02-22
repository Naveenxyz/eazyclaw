package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
)

// HeartbeatRunner sends periodic synthetic messages to the bus,
// prompting the agent to review HEARTBEAT.md for active tasks.
type HeartbeatRunner struct {
	interval time.Duration
	bus      *bus.Bus
}

// NewHeartbeatRunner creates a new HeartbeatRunner.
func NewHeartbeatRunner(interval time.Duration, b *bus.Bus) *HeartbeatRunner {
	return &HeartbeatRunner{
		interval: interval,
		bus:      b,
	}
}

// Start begins the heartbeat ticker loop. It blocks until ctx is cancelled.
func (h *HeartbeatRunner) Start(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	slog.Info("heartbeat runner started", "interval", h.interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("heartbeat runner stopped")
			return
		case t := <-ticker.C:
			msg := bus.Message{
				ID:        fmt.Sprintf("heartbeat-%d", t.UnixNano()),
				ChannelID: "heartbeat",
				SenderID:  "system",
				Text:      "[heartbeat] Check HEARTBEAT.md for active tasks",
				Timestamp: t,
			}
			select {
			case h.bus.Inbound <- msg:
				slog.Debug("heartbeat tick sent", "time", t)
			case <-ctx.Done():
				return
			}
		}
	}
}
