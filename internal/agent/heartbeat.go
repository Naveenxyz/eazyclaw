package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
)

// HeartbeatStatus holds the current state of the heartbeat runner.
type HeartbeatStatus struct {
	Enabled  bool      `json:"enabled"`
	Interval string    `json:"interval"`
	LastRun  time.Time `json:"last_run"`
	Running  bool      `json:"running"`
}

// HeartbeatRunner sends periodic synthetic messages to the bus,
// prompting the agent to review HEARTBEAT.md for active tasks.
type HeartbeatRunner struct {
	interval time.Duration
	bus      *bus.Bus

	mu      sync.RWMutex
	lastRun time.Time
	running bool
}

// NewHeartbeatRunner creates a new HeartbeatRunner.
func NewHeartbeatRunner(interval time.Duration, b *bus.Bus) *HeartbeatRunner {
	return &HeartbeatRunner{
		interval: interval,
		bus:      b,
	}
}

// Status returns the current heartbeat status.
func (h *HeartbeatRunner) Status() HeartbeatStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return HeartbeatStatus{
		Enabled:  true,
		Interval: h.interval.String(),
		LastRun:  h.lastRun,
		Running:  h.running,
	}
}

// Start begins the heartbeat ticker loop. It blocks until ctx is cancelled.
func (h *HeartbeatRunner) Start(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	h.mu.Lock()
	h.running = true
	h.mu.Unlock()

	slog.Info("heartbeat runner started", "interval", h.interval)

	defer func() {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		slog.Info("heartbeat runner stopped")
	}()

	for {
		select {
		case <-ctx.Done():
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
				h.mu.Lock()
				h.lastRun = t
				h.mu.Unlock()
				slog.Debug("heartbeat tick sent", "time", t)
			case <-ctx.Done():
				return
			}
		}
	}
}
