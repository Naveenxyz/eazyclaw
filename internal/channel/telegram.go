package channel

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/state"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const telegramMaxMessageLength = 4096

// TelegramPendingApproval captures a disallowed DM sender awaiting approval.
type TelegramPendingApproval struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Preview      string    `json:"preview"`
	MessageCount int       `json:"message_count"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// TelegramAdminState is surfaced in the config console for DM approvals.
type TelegramAdminState struct {
	GroupPolicy  string                    `json:"group_policy"`
	DMPolicy     string                    `json:"dm_policy"`
	AllowedUsers []string                  `json:"allowed_users"`
	Pending      []TelegramPendingApproval `json:"pending_approvals"`
}

// TelegramChannel integrates with the Telegram Bot API.
type TelegramChannel struct {
	token       string
	cfg         config.TelegramChannelConfig
	store       *state.Store
	bot         *bot.Bot
	bus         *bus.Bus
	cancel      context.CancelFunc
	botUsername string
	stateMu     sync.RWMutex // protects cfg only
}

// NewTelegramChannel creates a new TelegramChannel.
func NewTelegramChannel(token string, cfg config.TelegramChannelConfig, store *state.Store) *TelegramChannel {
	return &TelegramChannel{
		token: token,
		cfg:   cfg,
		store: store,
	}
}

// Name returns the channel identifier.
func (t *TelegramChannel) Name() string { return "telegram" }

// Start begins listening for Telegram messages and pushes them to the bus.
func (t *TelegramChannel) Start(ctx context.Context, b *bus.Bus) error {
	t.bus = b

	opts := []bot.Option{
		bot.WithDefaultHandler(t.defaultHandler),
	}

	tgBot, err := bot.New(t.token, opts...)
	if err != nil {
		return fmt.Errorf("telegram: failed to create bot: %w", err)
	}
	t.bot = tgBot

	// Fetch bot info for mention detection.
	me, err := tgBot.GetMe(ctx)
	if err != nil {
		slog.Warn("telegram: failed to get bot info", "error", err)
	} else if me != nil {
		t.botUsername = me.Username
		slog.Info("telegram: bot username", "username", t.botUsername)
	}

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go t.bot.Start(ctx)
	slog.Info("telegram channel started")
	return nil
}

// defaultHandler processes incoming Telegram updates.
func (t *TelegramChannel) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	senderID := strconv.FormatInt(update.Message.From.ID, 10)

	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	isGroup := update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup"
	isDM := update.Message.Chat.Type == "private"

	// DM policy check.
	if isDM {
		if t.cfg.DM.Policy == "deny" {
			slog.Debug("telegram: DM denied by policy", "sender_id", senderID)
			return
		}
		if !t.isDMUserAllowed(senderID) {
			username := ""
			if update.Message.From != nil {
				username = update.Message.From.Username
				if username == "" {
					username = strings.TrimSpace(update.Message.From.FirstName + " " + update.Message.From.LastName)
				}
			}
			t.recordPendingDM(senderID, username, update.Message.Text, time.Unix(int64(update.Message.Date), 0))
			slog.Warn("telegram: DM from disallowed user", "sender_id", senderID)
			return
		}
	} else if isGroup {
		// For group messages, check user allowlist if non-empty.
		if !t.isGroupUserAllowed(senderID) {
			slog.Warn("telegram: message from disallowed user", "sender_id", senderID)
			return
		}
	}

	// Group chat filtering.
	if isGroup {
		allowed, requireMention := t.isGroupChatAllowed(chatID)
		if !allowed {
			slog.Debug("telegram: group chat rejected", "chat_id", chatID)
			return
		}

		if requireMention {
			if !t.isBotMentioned(update.Message.Text) {
				return
			}
		}
	}

	// Strip bot mention from text.
	text := update.Message.Text
	if t.botUsername != "" {
		text = strings.ReplaceAll(text, "@"+t.botUsername, "")
		text = strings.TrimSpace(text)
	}

	var groupID string
	if isGroup {
		groupID = chatID
	}

	msg := bus.Message{
		ID:        strconv.Itoa(update.Message.ID),
		ChannelID: "telegram",
		SenderID:  senderID,
		UserID:    senderID,
		GroupID:   groupID,
		Text:      text,
		Timestamp: time.Unix(int64(update.Message.Date), 0),
	}

	t.bus.Inbound <- msg
	slog.Debug("telegram: message received", "sender_id", senderID, "chat_id", chatID)
}

// isGroupChatAllowed checks if a group chat is permitted and whether mention is required.
func (t *TelegramChannel) isGroupChatAllowed(chatID string) (bool, bool) {
	if t.cfg.GroupPolicy == "open" {
		// Open policy: all groups allowed. Check for specific chat config.
		if chatCfg, ok := t.cfg.AllowedChats[chatID]; ok {
			return chatCfg.Allow, chatCfg.RequireMention
		}
		return true, true // default: require mention
	}

	// Allowlist policy: chat must be in AllowedChats map.
	chatCfg, ok := t.cfg.AllowedChats[chatID]
	if !ok {
		return false, false
	}
	return chatCfg.Allow, chatCfg.RequireMention
}

// isBotMentioned checks if the bot's username is mentioned in the text.
func (t *TelegramChannel) isBotMentioned(text string) bool {
	if t.botUsername == "" {
		return true // Can't check, allow through.
	}
	return strings.Contains(text, "@"+t.botUsername)
}

// Send sends an outbound message to Telegram, chunking if necessary.
func (t *TelegramChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %q: %w", msg.ChatID, err)
	}

	chunks := chunkText(msg.Text, telegramMaxMessageLength)
	for _, chunk := range chunks {
		params := &bot.SendMessageParams{
			ChatID: chatID,
			Text:   chunk,
		}
		if msg.ReplyTo != "" {
			replyID, parseErr := strconv.Atoi(msg.ReplyTo)
			if parseErr == nil {
				params.ReplyParameters = &models.ReplyParameters{
					MessageID: replyID,
				}
			}
		}
		if _, err := t.bot.SendMessage(ctx, params); err != nil {
			return fmt.Errorf("telegram: failed to send message: %w", err)
		}
	}
	return nil
}

// Stop gracefully shuts down the Telegram channel.
func (t *TelegramChannel) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	slog.Info("telegram channel stopped")
	return nil
}

// isDMUserAllowed enforces stricter DM defaults for allowlist mode.
// In allowlist mode, DMs are denied unless sender is explicitly allowlisted.
func (t *TelegramChannel) isDMUserAllowed(senderID string) bool {
	t.stateMu.RLock()
	groupPolicy := t.cfg.GroupPolicy
	t.stateMu.RUnlock()

	ok, _ := t.store.IsAllowed("telegram", senderID)
	if groupPolicy == "allowlist" {
		return ok
	}
	if ok {
		return true
	}
	users, _ := t.store.AllowedUsers("telegram")
	return len(users) == 0
}

func (t *TelegramChannel) isGroupUserAllowed(senderID string) bool {
	ok, _ := t.store.IsAllowed("telegram", senderID)
	if ok {
		return true
	}
	users, _ := t.store.AllowedUsers("telegram")
	return len(users) == 0
}

func (t *TelegramChannel) recordPendingDM(userID, username, content string, at time.Time) {
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

	t.store.UpsertPending("telegram", state.PendingApproval{
		UserID:      userID,
		Username:    strings.TrimSpace(username),
		Preview:     preview,
		FirstSeenAt: at,
		LastSeenAt:  at,
	})
}

// Snapshot returns a thread-safe snapshot for Telegram admin APIs.
func (t *TelegramChannel) Snapshot() TelegramAdminState {
	t.stateMu.RLock()
	groupPolicy := t.cfg.GroupPolicy
	dmPolicy := t.cfg.DM.Policy
	t.stateMu.RUnlock()

	allowed, _ := t.store.AllowedUsers("telegram")
	if allowed == nil {
		allowed = []string{}
	}

	rawPending, _ := t.store.PendingApprovals("telegram")
	pending := make([]TelegramPendingApproval, 0, len(rawPending))
	for _, p := range rawPending {
		pending = append(pending, TelegramPendingApproval{
			UserID:       p.UserID,
			Username:     p.Username,
			Preview:      p.Preview,
			MessageCount: p.MessageCount,
			FirstSeenAt:  p.FirstSeenAt,
			LastSeenAt:   p.LastSeenAt,
		})
	}

	return TelegramAdminState{
		GroupPolicy:  groupPolicy,
		DMPolicy:     dmPolicy,
		AllowedUsers: allowed,
		Pending:      pending,
	}
}

// ApplyConfig updates runtime Telegram policy without restart.
// Only static config (policies, chats) is updated here; allowlist is in the store.
func (t *TelegramChannel) ApplyConfig(cfg config.TelegramChannelConfig) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.cfg = cfg
}

// ApproveUser adds a user to allowlist and removes any pending approval entry.
func (t *TelegramChannel) ApproveUser(userID string) bool {
	id := strings.TrimSpace(userID)
	if id == "" {
		return false
	}

	t.store.AddAllowedUser("telegram", id)
	t.store.DeletePending("telegram", id)
	return true
}

// RejectUser clears a pending approval entry.
func (t *TelegramChannel) RejectUser(userID string) bool {
	id := strings.TrimSpace(userID)
	if id == "" {
		return false
	}

	pending, _ := t.store.PendingApprovals("telegram")
	existed := false
	for _, p := range pending {
		if p.UserID == id {
			existed = true
			break
		}
	}
	t.store.DeletePending("telegram", id)
	return existed
}

// chunkText splits text into chunks of at most maxLen characters.
func chunkText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		end := maxLen
		if end > len(text) {
			end = len(text)
		}
		// Try to break at a newline within the last 200 chars.
		if end < len(text) {
			search := end
			if search > 200 {
				for i := end - 1; i >= end-200; i-- {
					if text[i] == '\n' {
						search = i + 1
						break
					}
				}
				if search != end {
					end = search
				}
			}
		}
		chunks = append(chunks, text[:end])
		text = text[end:]
	}
	return chunks
}
