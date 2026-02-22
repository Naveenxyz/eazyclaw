package channel

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
)

const discordMaxMessageLength = 2000
const discordTypingInterval = 8 * time.Second

type typingState struct {
	refs   int
	cancel context.CancelFunc
}

// DiscordChannel integrates with the Discord API.
type DiscordChannel struct {
	token        string
	cfg          config.DiscordChannelConfig
	allowedUsers map[string]bool
	session      *discordgo.Session
	bus          *bus.Bus
	botUserID    string
	ctx          context.Context
	typingMu     sync.Mutex
	typingByChat map[string]*typingState
}

// NewDiscordChannel creates a new DiscordChannel.
func NewDiscordChannel(token string, cfg config.DiscordChannelConfig) *DiscordChannel {
	allowed := make(map[string]bool, len(cfg.AllowedUsers))
	for _, u := range cfg.AllowedUsers {
		allowed[u] = true
	}
	return &DiscordChannel{
		token:        token,
		cfg:          cfg,
		allowedUsers: allowed,
		typingByChat: make(map[string]*typingState),
	}
}

// Name returns the channel identifier.
func (d *DiscordChannel) Name() string { return "discord" }

// Start begins listening for Discord messages and pushes them to the bus.
func (d *DiscordChannel) Start(ctx context.Context, b *bus.Bus) error {
	d.bus = b
	d.ctx = ctx

	session, err := discordgo.New("Bot " + d.token)
	if err != nil {
		return fmt.Errorf("discord: failed to create session: %w", err)
	}
	d.session = session

	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentMessageContent

	session.AddHandler(d.messageCreateHandler)

	if err := session.Open(); err != nil {
		return fmt.Errorf("discord: failed to open websocket: %w", err)
	}

	// Store the bot's user ID for mention detection.
	d.botUserID = session.State.User.ID

	slog.Info("discord channel started", "bot_user", d.botUserID)
	return nil
}

// messageCreateHandler processes incoming Discord messages.
func (d *DiscordChannel) messageCreateHandler(_ *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots (including ourselves).
	if m.Author == nil || m.Author.Bot {
		return
	}

	senderID := m.Author.ID

	// Check allowlist.
	if len(d.allowedUsers) > 0 && !d.allowedUsers[senderID] {
		slog.Warn("discord: message from disallowed user", "sender_id", senderID)
		return
	}

	isDM := m.GuildID == ""

	// DM policy check.
	if isDM {
		if d.cfg.DM.Policy == "deny" {
			slog.Debug("discord: DM denied by policy", "sender_id", senderID)
			return
		}
		// "allow" and "pairing" both allow the DM through.
	}

	// Guild message filtering.
	if !isDM {
		allowed, requireMention := d.isGuildMessageAllowed(m.GuildID, m.ChannelID)
		if !allowed {
			slog.Debug("discord: guild message rejected", "guild_id", m.GuildID, "channel_id", m.ChannelID)
			return
		}

		if requireMention {
			isMentioned := false
			for _, mention := range m.Mentions {
				if mention.ID == d.botUserID {
					isMentioned = true
					break
				}
			}
			if !isMentioned {
				return
			}
		}
	} else {
		// DMs don't require mention check.
	}

	// Strip the bot mention from the text.
	text := m.Content
	isMentioned := false
	for _, mention := range m.Mentions {
		if mention.ID == d.botUserID {
			isMentioned = true
			break
		}
	}
	if isMentioned {
		text = strings.ReplaceAll(text, "<@"+d.botUserID+">", "")
		text = strings.ReplaceAll(text, "<@!"+d.botUserID+">", "")
		text = strings.TrimSpace(text)
	}

	// Don't push empty messages (e.g. bare mention with no text).
	if text == "" {
		return
	}

	var groupID string
	chatSenderID := senderID
	if !isDM {
		groupID = m.ChannelID
	} else {
		// For DMs, use the DM channel ID as sender so replies route correctly.
		chatSenderID = m.ChannelID
	}

	msg := bus.Message{
		ID:        m.ID,
		ChannelID: "discord",
		SenderID:  chatSenderID,
		GroupID:   groupID,
		Text:      text,
		Timestamp: m.Timestamp,
	}

	d.startTyping(m.ChannelID)
	d.bus.Inbound <- msg
	slog.Debug("discord: message received", "sender_id", senderID, "channel_id", m.ChannelID)
}

// isGuildMessageAllowed checks whether a message from a guild/channel is permitted
// and whether mention is required. Returns (allowed, requireMention).
func (d *DiscordChannel) isGuildMessageAllowed(guildID, channelID string) (bool, bool) {
	// Default requireMention is true.
	defaultRequireMention := true

	if d.cfg.GroupPolicy == "open" {
		// Open policy: all guilds allowed. Check if guild has specific config.
		guild, hasGuild := d.cfg.Guilds[guildID]
		if hasGuild {
			return d.checkGuildChannel(guild, channelID, guild.RequireMention)
		}
		return true, defaultRequireMention
	}

	// Allowlist policy: guild must be in Guilds map.
	guild, hasGuild := d.cfg.Guilds[guildID]
	if !hasGuild {
		return false, false
	}

	return d.checkGuildChannel(guild, channelID, guild.RequireMention)
}

// checkGuildChannel checks channel-level config within a guild.
func (d *DiscordChannel) checkGuildChannel(guild config.DiscordGuildConfig, channelID string, guildRequireMention bool) (bool, bool) {
	// If no channels configured, all channels in guild are allowed.
	if len(guild.Channels) == 0 {
		return true, guildRequireMention
	}

	chCfg, hasChannel := guild.Channels[channelID]
	if !hasChannel {
		// Channel not in map = not allowed (when channels map is non-empty).
		return false, false
	}

	// Check Allow flag.
	if chCfg.Allow != nil && !*chCfg.Allow {
		return false, false
	}

	// Channel RequireMention overrides guild level.
	requireMention := guildRequireMention
	if chCfg.RequireMention != nil {
		requireMention = *chCfg.RequireMention
	}

	return true, requireMention
}

// Send sends an outbound message to Discord, chunking if necessary.
func (d *DiscordChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	defer d.stopTyping(msg.ChatID)

	chunks := chunkTextDiscord(msg.Text, discordMaxMessageLength)
	for i, chunk := range chunks {
		var ref *discordgo.MessageReference
		if i == 0 && msg.ReplyTo != "" {
			ref = &discordgo.MessageReference{
				MessageID: msg.ReplyTo,
				ChannelID: msg.ChatID,
			}
		}
		_, err := d.session.ChannelMessageSendComplex(msg.ChatID, &discordgo.MessageSend{
			Content:   chunk,
			Reference: ref,
		})
		if err != nil {
			return fmt.Errorf("discord: failed to send message: %w", err)
		}
	}
	return nil
}

// Stop gracefully shuts down the Discord channel.
func (d *DiscordChannel) Stop() error {
	d.stopAllTyping()

	if d.session != nil {
		if err := d.session.Close(); err != nil {
			return fmt.Errorf("discord: failed to close session: %w", err)
		}
	}
	slog.Info("discord channel stopped")
	return nil
}

// chunkTextDiscord splits text into chunks of at most maxLen characters.
func chunkTextDiscord(text string, maxLen int) []string {
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
		if end < len(text) && end > 200 {
			for i := end - 1; i >= end-200; i-- {
				if text[i] == '\n' {
					end = i + 1
					break
				}
			}
		}
		chunks = append(chunks, text[:end])
		text = text[end:]
	}
	return chunks
}

func (d *DiscordChannel) startTyping(chatID string) {
	if chatID == "" || d.session == nil {
		return
	}

	d.typingMu.Lock()
	if st, ok := d.typingByChat[chatID]; ok {
		st.refs++
		d.typingMu.Unlock()
		return
	}
	baseCtx := d.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	typingCtx, cancel := context.WithCancel(baseCtx)
	d.typingByChat[chatID] = &typingState{refs: 1, cancel: cancel}
	d.typingMu.Unlock()

	go d.runTypingLoop(typingCtx, chatID)
}

func (d *DiscordChannel) stopTyping(chatID string) {
	if chatID == "" {
		return
	}

	var cancel context.CancelFunc
	d.typingMu.Lock()
	st, ok := d.typingByChat[chatID]
	if ok {
		st.refs--
		if st.refs <= 0 {
			cancel = st.cancel
			delete(d.typingByChat, chatID)
		}
	}
	d.typingMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (d *DiscordChannel) stopAllTyping() {
	d.typingMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(d.typingByChat))
	for chatID, st := range d.typingByChat {
		cancels = append(cancels, st.cancel)
		delete(d.typingByChat, chatID)
	}
	d.typingMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (d *DiscordChannel) runTypingLoop(ctx context.Context, chatID string) {
	pulse := func() {
		if err := d.session.ChannelTyping(chatID); err != nil {
			slog.Debug("discord: typing indicator failed", "chat_id", chatID, "error", err)
		}
	}

	pulse()
	ticker := time.NewTicker(discordTypingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pulse()
		}
	}
}

// Ensure DiscordChannel satisfies the Channel interface at compile time.
var _ Channel = (*DiscordChannel)(nil)
