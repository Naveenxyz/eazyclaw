package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
)

// HeartbeatStatus holds the current state of the heartbeat runner.
type HeartbeatStatus struct {
	Enabled         bool      `json:"enabled"`
	Interval        string    `json:"interval"`
	LastRun         time.Time `json:"last_run"`
	Running         bool      `json:"running"`
	DeliveryChannel string    `json:"delivery_channel,omitempty"`
	DeliveryChatID  string    `json:"delivery_chat_id,omitempty"`
}

// HeartbeatRunner sends periodic synthetic messages to the bus,
// prompting the agent to review HEARTBEAT.md for active tasks.
type HeartbeatRunner struct {
	interval        time.Duration
	bus             *bus.Bus
	heartbeatPath   string
	deliveryChannel string
	deliveryChatID  string

	mu      sync.RWMutex
	lastRun time.Time
	running bool
}

// NewHeartbeatRunner creates a new HeartbeatRunner.
func NewHeartbeatRunner(interval time.Duration, b *bus.Bus, heartbeatPath, deliveryChannel, deliveryChatID string) *HeartbeatRunner {
	deliveryChannel, deliveryChatID, _ = normalizeHeartbeatDeliveryTarget(deliveryChannel, deliveryChatID)
	return &HeartbeatRunner{
		interval:        interval,
		bus:             b,
		heartbeatPath:   heartbeatPath,
		deliveryChannel: deliveryChannel,
		deliveryChatID:  deliveryChatID,
	}
}

// Status returns the current heartbeat status.
func (h *HeartbeatRunner) Status() HeartbeatStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return HeartbeatStatus{
		Enabled:         true,
		Interval:        h.interval.String(),
		LastRun:         h.lastRun,
		Running:         h.running,
		DeliveryChannel: h.deliveryChannel,
		DeliveryChatID:  h.deliveryChatID,
	}
}

// SetDeliveryTarget updates the outbound delivery target for heartbeat responses.
func (h *HeartbeatRunner) SetDeliveryTarget(channel, chatID string) error {
	channel, chatID, err := normalizeHeartbeatDeliveryTarget(channel, chatID)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deliveryChannel = channel
	h.deliveryChatID = chatID
	return nil
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
			if !heartbeatHasActionableTasks(h.heartbeatPath) {
				slog.Debug("heartbeat tick skipped: no actionable tasks")
				continue
			}
			h.mu.RLock()
			deliveryChannel := h.deliveryChannel
			deliveryChatID := h.deliveryChatID
			h.mu.RUnlock()
			msg := bus.Message{
				ID:             fmt.Sprintf("heartbeat-%d", t.UnixNano()),
				ChannelID:      "heartbeat",
				SenderID:       "heartbeat",
				UserID:         "system",
				ReplyChannelID: deliveryChannel,
				ReplyChatID:    deliveryChatID,
				Text:           "[heartbeat] Check HEARTBEAT.md for active tasks",
				Timestamp:      t,
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

func heartbeatHasActionableTasks(path string) bool {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return true
	}
	data, err := os.ReadFile(trimmedPath)
	if err != nil {
		return false
	}
	return heartbeatContentHasActionableTasks(string(data))
}

func heartbeatContentHasActionableTasks(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	ignoredExact := map[string]struct{}{
		"List periodic checks and proactive tasks.":                                                           {},
		"- If there is nothing actionable, keep this short and respond with HEARTBEAT_OK in heartbeat turns.": {},
		"- [ ]": {},
		"* [ ]": {},
		"- [x]": {},
		"* [x]": {},
		"- [X]": {},
		"* [X]": {},
	}

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "<!--") {
			continue
		}
		if _, ignored := ignoredExact[line]; ignored {
			continue
		}
		if strings.HasPrefix(line, "<!--") && strings.HasSuffix(line, "-->") {
			continue
		}
		return true
	}
	return false
}

func normalizeHeartbeatDeliveryTarget(channel, chatID string) (string, string, error) {
	channel = strings.TrimSpace(channel)
	chatID = strings.TrimSpace(chatID)
	if (channel == "") != (chatID == "") {
		return "", "", fmt.Errorf("delivery_channel and delivery_chat_id must both be set or both empty")
	}
	return channel, chatID, nil
}
