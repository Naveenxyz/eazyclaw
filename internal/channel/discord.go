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
	"github.com/eazyclaw/eazyclaw/internal/state"
)

const discordMaxMessageLength = 2000
const discordTypingInterval = 8 * time.Second

type typingState struct {
	refs   int
	cancel context.CancelFunc
}

// DiscordPendingApproval captures a disallowed DM sender awaiting approval.
type DiscordPendingApproval struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Preview      string    `json:"preview"`
	MessageCount int       `json:"message_count"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// DiscordAdminState is surfaced in the config console for DM approvals.
type DiscordAdminState struct {
	GroupPolicy  string                   `json:"group_policy"`
	DMPolicy     string                   `json:"dm_policy"`
	AllowedUsers []string                 `json:"allowed_users"`
	Pending      []DiscordPendingApproval `json:"pending_approvals"`
}

// DiscordChannel integrates with the Discord API.
type DiscordChannel struct {
	token        string
	cfg          config.DiscordChannelConfig
	store        *state.Store
	session      *discordgo.Session
	bus          *bus.Bus
	botUserID    string
	ctx          context.Context
	typingMu     sync.Mutex
	typingByChat map[string]*typingState
	stateMu      sync.RWMutex // protects cfg only (policies, guilds)
}

// NewDiscordChannel creates a new DiscordChannel.
func NewDiscordChannel(token string, cfg config.DiscordChannelConfig, store *state.Store) *DiscordChannel {
	return &DiscordChannel{
		token:        token,
		cfg:          cfg,
		store:        store,
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

	isDM := m.GuildID == ""

	// DM policy check.
	if isDM {
		if d.dmPolicy() == "deny" {
			slog.Debug("discord: DM denied by policy", "sender_id", senderID)
			return
		}
		if !d.isDMUserAllowed(senderID) {
			at := m.Timestamp
			if at.IsZero() {
				at = time.Now()
			}
			d.recordPendingDM(senderID, m.Author.Username, m.Content, at)
			slog.Warn("discord: DM from disallowed user", "sender_id", senderID)
			return
		}
		// "allow" and "pairing" both allow the DM through.
	} else {
		// For guild messages, keep traditional optional user allowlist semantics.
		if !d.isGuildUserAllowed(senderID) {
			slog.Warn("discord: message from disallowed user", "sender_id", senderID)
			return
		}
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
		UserID:    senderID,
		GroupID:   groupID,
		Text:      text,
		Timestamp: m.Timestamp,
	}

	d.startTyping(m.ChannelID)
	d.bus.Inbound <- msg
	slog.Debug("discord: message received", "sender_id", senderID, "channel_id", m.ChannelID)
}

// isDMUserAllowed enforces stricter DM defaults for allowlist mode.
// In allowlist mode, DMs are denied unless sender is explicitly allowlisted.
func (d *DiscordChannel) isDMUserAllowed(senderID string) bool {
	d.stateMu.RLock()
	groupPolicy := d.cfg.GroupPolicy
	d.stateMu.RUnlock()

	ok, _ := d.store.IsAllowed("discord", senderID)
	if groupPolicy == "allowlist" {
		return ok
	}
	if ok {
		return true
	}
	users, _ := d.store.AllowedUsers("discord")
	return len(users) == 0
}

func (d *DiscordChannel) isGuildUserAllowed(senderID string) bool {
	ok, _ := d.store.IsAllowed("discord", senderID)
	if ok {
		return true
	}
	users, _ := d.store.AllowedUsers("discord")
	return len(users) == 0
}

func (d *DiscordChannel) dmPolicy() string {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	return d.cfg.DM.Policy
}

// isGuildMessageAllowed checks whether a message from a guild/channel is permitted
// and whether mention is required. Returns (allowed, requireMention).
func (d *DiscordChannel) isGuildMessageAllowed(guildID, channelID string) (bool, bool) {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()

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

func (d *DiscordChannel) recordPendingDM(userID, username, content string, at time.Time) {
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

	d.store.UpsertPending("discord", state.PendingApproval{
		UserID:      userID,
		Username:    strings.TrimSpace(username),
		Preview:     preview,
		FirstSeenAt: at,
		LastSeenAt:  at,
	})
}

// Snapshot returns a thread-safe snapshot for Discord admin APIs.
func (d *DiscordChannel) Snapshot() DiscordAdminState {
	d.stateMu.RLock()
	groupPolicy := d.cfg.GroupPolicy
	dmPolicy := d.cfg.DM.Policy
	d.stateMu.RUnlock()

	allowed, _ := d.store.AllowedUsers("discord")
	if allowed == nil {
		allowed = []string{}
	}

	rawPending, _ := d.store.PendingApprovals("discord")
	pending := make([]DiscordPendingApproval, 0, len(rawPending))
	for _, p := range rawPending {
		pending = append(pending, DiscordPendingApproval{
			UserID:       p.UserID,
			Username:     p.Username,
			Preview:      p.Preview,
			MessageCount: p.MessageCount,
			FirstSeenAt:  p.FirstSeenAt,
			LastSeenAt:   p.LastSeenAt,
		})
	}

	return DiscordAdminState{
		GroupPolicy:  groupPolicy,
		DMPolicy:     dmPolicy,
		AllowedUsers: allowed,
		Pending:      pending,
	}
}

// ApplyConfig updates runtime Discord policy without restart.
// Only static config (policies, guilds) is updated here; allowlist is in the store.
func (d *DiscordChannel) ApplyConfig(cfg config.DiscordChannelConfig) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	d.cfg = cfg
}

// ApproveUser adds a user to allowlist and removes any pending approval entry.
func (d *DiscordChannel) ApproveUser(userID string) bool {
	id := strings.TrimSpace(userID)
	if id == "" {
		return false
	}

	d.store.AddAllowedUser("discord", id)
	d.store.DeletePending("discord", id)
	return true
}

// RejectUser clears a pending approval entry.
func (d *DiscordChannel) RejectUser(userID string) bool {
	id := strings.TrimSpace(userID)
	if id == "" {
		return false
	}

	pending, _ := d.store.PendingApprovals("discord")
	existed := false
	for _, p := range pending {
		if p.UserID == id {
			existed = true
			break
		}
	}
	d.store.DeletePending("discord", id)
	return existed
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
