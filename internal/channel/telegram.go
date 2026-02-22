package channel

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const telegramMaxMessageLength = 4096

// TelegramChannel integrates with the Telegram Bot API.
type TelegramChannel struct {
	token        string
	cfg          config.TelegramChannelConfig
	allowedUsers map[string]bool
	bot          *bot.Bot
	bus          *bus.Bus
	cancel       context.CancelFunc
	botUsername   string
}

// NewTelegramChannel creates a new TelegramChannel.
func NewTelegramChannel(token string, cfg config.TelegramChannelConfig) *TelegramChannel {
	allowed := make(map[string]bool, len(cfg.AllowedUsers))
	for _, u := range cfg.AllowedUsers {
		allowed[u] = true
	}
	return &TelegramChannel{
		token:        token,
		cfg:          cfg,
		allowedUsers: allowed,
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

	// Check allowlist.
	if len(t.allowedUsers) > 0 && !t.allowedUsers[senderID] {
		slog.Warn("telegram: message from disallowed user", "sender_id", senderID)
		return
	}

	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	isGroup := update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup"
	isDM := update.Message.Chat.Type == "private"

	// DM policy check.
	if isDM {
		if t.cfg.DM.Policy == "deny" {
			slog.Debug("telegram: DM denied by policy", "sender_id", senderID)
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
