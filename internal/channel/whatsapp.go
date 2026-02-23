package channel

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/state"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

const whatsappMaxMessageLength = 4096

// WhatsAppPendingApproval captures a disallowed DM sender awaiting approval.
type WhatsAppPendingApproval struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Preview      string    `json:"preview"`
	MessageCount int       `json:"message_count"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// WhatsAppAdminState is surfaced in the config console for DM approvals.
type WhatsAppAdminState struct {
	GroupPolicy  string                    `json:"group_policy"`
	DMPolicy     string                    `json:"dm_policy"`
	AllowedUsers []string                  `json:"allowed_users"`
	Pending      []WhatsAppPendingApproval `json:"pending_approvals"`
	Status       string                    `json:"status"`       // "disconnected" | "qr_pending" | "connected"
	PhoneNumber  string                    `json:"phone_number"` // connected phone
	QRCode       string                    `json:"qr_code"`      // current QR data (empty if paired)
}

// WhatsAppChannel integrates with the WhatsApp API via whatsmeow.
type WhatsAppChannel struct {
	cfg       config.WhatsAppChannelConfig
	store     *state.Store
	client    *whatsmeow.Client
	container *sqlstore.Container
	bus       *bus.Bus
	qrChan    <-chan whatsmeow.QRChannelItem
	status    atomic.Value // stores string
	latestQR  atomic.Value // stores string (latest QR code data)
	phoneNum  string
	dataDir   string
	stateMu   sync.RWMutex // protects cfg and phoneNum
	cancel    context.CancelFunc
}

// NewWhatsAppChannel creates a new WhatsAppChannel.
func NewWhatsAppChannel(cfg config.WhatsAppChannelConfig, dataDir string, store *state.Store) *WhatsAppChannel {
	ch := &WhatsAppChannel{
		cfg:     cfg,
		dataDir: dataDir,
		store:   store,
	}
	ch.status.Store("disconnected")
	return ch
}

// Name returns the channel identifier.
func (w *WhatsAppChannel) Name() string { return "whatsapp" }

// Start begins listening for WhatsApp messages and pushes them to the bus.
func (w *WhatsAppChannel) Start(ctx context.Context, b *bus.Bus) error {
	w.bus = b

	dbPath := fmt.Sprintf("file:%s/whatsapp.db?_pragma=foreign_keys(1)", w.dataDir)
	container, err := sqlstore.New(ctx, "sqlite", dbPath, nil)
	if err != nil {
		return fmt.Errorf("whatsapp: failed to open store: %w", err)
	}
	w.container = container

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp: failed to get device: %w", err)
	}

	client := whatsmeow.NewClient(deviceStore, nil)
	w.client = client

	client.AddEventHandler(w.eventHandler)

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	if client.Store.ID == nil {
		// No session yet — need QR code login.
		qrChan, err := client.GetQRChannel(ctx)
		if err != nil {
			cancel()
			return fmt.Errorf("whatsapp: failed to get QR channel: %w", err)
		}
		w.qrChan = qrChan
		if err := client.Connect(); err != nil {
			cancel()
			return fmt.Errorf("whatsapp: failed to connect: %w", err)
		}
		w.status.Store("qr_pending")
		go w.handleQREvents(ctx)
	} else {
		if err := client.Connect(); err != nil {
			cancel()
			return fmt.Errorf("whatsapp: failed to connect: %w", err)
		}
		w.status.Store("connected")
		w.phoneNum = client.Store.ID.User
	}

	slog.Info("whatsapp channel started")
	return nil
}

// handleQREvents processes QR code events from the login flow.
func (w *WhatsAppChannel) handleQREvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-w.qrChan:
			if !ok {
				return
			}
			switch evt.Event {
			case "code":
				w.status.Store("qr_pending")
				w.latestQR.Store(evt.Code)
				slog.Info("whatsapp: QR code received, scan to authenticate")
			case "login":
				w.status.Store("connected")
				if w.client.Store.ID != nil {
					w.phoneNum = w.client.Store.ID.User
				}
				slog.Info("whatsapp: login successful", "phone", w.phoneNum)
			case "timeout":
				w.status.Store("qr_timeout")
				slog.Warn("whatsapp: QR code timed out")
			case "error":
				w.status.Store("error")
				slog.Error("whatsapp: QR login error")
			}
		}
	}
}

// eventHandler processes incoming WhatsApp events.
func (w *WhatsAppChannel) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		w.handleMessage(v)
	case *events.Connected:
		w.status.Store("connected")
		if w.client.Store.ID != nil {
			w.phoneNum = w.client.Store.ID.User
		}
		slog.Info("whatsapp: connected", "phone", w.phoneNum)
	case *events.Disconnected:
		w.status.Store("disconnected")
		slog.Warn("whatsapp: disconnected")
	case *events.LoggedOut:
		w.status.Store("logged_out")
		slog.Warn("whatsapp: logged out")
	}
}

// handleMessage processes an incoming WhatsApp message.
func (w *WhatsAppChannel) handleMessage(evt *events.Message) {
	if evt.Info.IsFromMe {
		return
	}

	text := ""
	if evt.Message.GetConversation() != "" {
		text = evt.Message.GetConversation()
	} else if evt.Message.GetExtendedTextMessage() != nil {
		text = evt.Message.GetExtendedTextMessage().GetText()
	}
	if text == "" {
		return
	}

	senderJID := evt.Info.Sender
	senderID := senderJID.User
	isGroup := evt.Info.IsGroup
	isDM := !isGroup

	if isDM {
		if w.cfg.DM.Policy == "deny" {
			slog.Debug("whatsapp: DM denied by policy", "sender_id", senderID)
			return
		}
		if !w.isAllowed(senderJID) {
			username := senderJID.User
			w.recordPendingDM(senderID, username, text, evt.Info.Timestamp)
			slog.Warn("whatsapp: DM from disallowed user", "sender_id", senderID)
			return
		}
	} else {
		if !w.isGroupUserAllowed(senderID) {
			slog.Warn("whatsapp: message from disallowed user", "sender_id", senderID)
			return
		}
	}

	var groupID string
	if isGroup {
		groupID = evt.Info.Chat.String()
	}

	chatID := evt.Info.Chat.String()
	if isDM {
		chatID = senderJID.String()
	}

	msg := bus.Message{
		ID:        evt.Info.ID,
		ChannelID: "whatsapp",
		SenderID:  chatID,
		UserID:    senderID,
		GroupID:   groupID,
		Text:      text,
		Timestamp: evt.Info.Timestamp,
	}

	w.bus.Inbound <- msg
	slog.Debug("whatsapp: message received", "sender_id", senderID, "chat", evt.Info.Chat.String())
}

// Send sends an outbound message to WhatsApp, chunking if necessary.
func (w *WhatsAppChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	jid, err := types.ParseJID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("whatsapp: invalid chat JID %q: %w", msg.ChatID, err)
	}

	chunks := chunkText(msg.Text, whatsappMaxMessageLength)
	for _, chunk := range chunks {
		_, err := w.client.SendMessage(ctx, jid, w.buildTextMessage(chunk))
		if err != nil {
			return fmt.Errorf("whatsapp: failed to send message: %w", err)
		}
	}
	return nil
}

// buildTextMessage creates a WhatsApp text proto message.
func (w *WhatsAppChannel) buildTextMessage(text string) *waE2E.Message {
	return &waE2E.Message{
		Conversation: proto.String(text),
	}
}

// Stop gracefully shuts down the WhatsApp channel.
func (w *WhatsAppChannel) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	if w.client != nil {
		w.client.Disconnect()
	}
	slog.Info("whatsapp channel stopped")
	return nil
}

// isAllowed checks if a sender JID is in the allowlist.
func (w *WhatsAppChannel) isAllowed(senderJID types.JID) bool {
	w.stateMu.RLock()
	groupPolicy := w.cfg.GroupPolicy
	w.stateMu.RUnlock()

	ok, _ := w.store.IsAllowed("whatsapp", senderJID.User)
	if groupPolicy == "allowlist" {
		return ok
	}
	if ok {
		return true
	}
	users, _ := w.store.AllowedUsers("whatsapp")
	return len(users) == 0
}

func (w *WhatsAppChannel) isGroupUserAllowed(senderID string) bool {
	ok, _ := w.store.IsAllowed("whatsapp", senderID)
	if ok {
		return true
	}
	users, _ := w.store.AllowedUsers("whatsapp")
	return len(users) == 0
}

func (w *WhatsAppChannel) recordPendingDM(userID, username, content string, at time.Time) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}
	preview := strings.TrimSpace(content)
	if len(preview) > 120 {
		preview = preview[:120] + "..."
	}
	if preview == "" {
		preview = "(no text)"
	}
	if at.IsZero() {
		at = time.Now()
	}

	w.store.UpsertPending("whatsapp", state.PendingApproval{
		UserID:      userID,
		Username:    strings.TrimSpace(username),
		Preview:     preview,
		FirstSeenAt: at,
		LastSeenAt:  at,
	})
}

// Snapshot returns a thread-safe snapshot for WhatsApp admin APIs.
func (w *WhatsAppChannel) Snapshot() WhatsAppAdminState {
	w.stateMu.RLock()
	groupPolicy := w.cfg.GroupPolicy
	dmPolicy := w.cfg.DM.Policy
	w.stateMu.RUnlock()

	allowed, _ := w.store.AllowedUsers("whatsapp")
	if allowed == nil {
		allowed = []string{}
	}

	rawPending, _ := w.store.PendingApprovals("whatsapp")
	pending := make([]WhatsAppPendingApproval, 0, len(rawPending))
	for _, p := range rawPending {
		pending = append(pending, WhatsAppPendingApproval{
			UserID:       p.UserID,
			Username:     p.Username,
			Preview:      p.Preview,
			MessageCount: p.MessageCount,
			FirstSeenAt:  p.FirstSeenAt,
			LastSeenAt:   p.LastSeenAt,
		})
	}

	return WhatsAppAdminState{
		GroupPolicy:  groupPolicy,
		DMPolicy:     dmPolicy,
		AllowedUsers: allowed,
		Pending:      pending,
		Status:       w.Status(),
		PhoneNumber:  w.PhoneNumber(),
		QRCode:       w.QRCode(),
	}
}

// ApplyConfig updates runtime WhatsApp policy without restart.
// Only static config (policies) is updated here; allowlist is in the store.
func (w *WhatsAppChannel) ApplyConfig(cfg config.WhatsAppChannelConfig) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	w.cfg = cfg
}

// ApproveUser adds a user to allowlist and removes any pending approval entry.
func (w *WhatsAppChannel) ApproveUser(userID string) bool {
	id := strings.TrimSpace(userID)
	if id == "" {
		return false
	}

	w.store.AddAllowedUser("whatsapp", id)
	w.store.DeletePending("whatsapp", id)
	return true
}

// RejectUser clears a pending approval entry.
func (w *WhatsAppChannel) RejectUser(userID string) bool {
	id := strings.TrimSpace(userID)
	if id == "" {
		return false
	}

	pending, _ := w.store.PendingApprovals("whatsapp")
	existed := false
	for _, p := range pending {
		if p.UserID == id {
			existed = true
			break
		}
	}
	w.store.DeletePending("whatsapp", id)
	return existed
}

// RemoveUser removes a user from the allowlist.
func (w *WhatsAppChannel) RemoveUser(userID string) bool {
	id := strings.TrimSpace(userID)
	if id == "" {
		return false
	}

	if err := w.store.RemoveAllowedUser("whatsapp", id); err != nil {
		return false
	}
	return true
}

// QRCode returns the current QR code string if awaiting scan.
func (w *WhatsAppChannel) QRCode() string {
	status := w.status.Load().(string)
	if status != "qr_pending" {
		return ""
	}
	if v := w.latestQR.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// Status returns the current connection status.
func (w *WhatsAppChannel) Status() string {
	return w.status.Load().(string)
}

// PhoneNumber returns the connected phone number, if any.
func (w *WhatsAppChannel) PhoneNumber() string {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	return w.phoneNum
}

// Disconnect disconnects the WhatsApp client without stopping the channel.
func (w *WhatsAppChannel) Disconnect() {
	if w.client != nil {
		w.client.Disconnect()
	}
	w.status.Store("disconnected")
}

// Ensure WhatsAppChannel satisfies the Channel interface at compile time.
var _ Channel = (*WhatsAppChannel)(nil)
