package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
)

const noReplyToken = "NO_REPLY"

const defaultAgentsTemplate = `# AGENTS.md

This workspace is your continuity layer.

## Every Session
1. Read SOUL.md.
2. Read USER.md.
3. Read memory/YYYY-MM-DD.md for today and yesterday.
4. In direct/private sessions, also use MEMORY.md.

## First Run
If BOOTSTRAP.md exists, follow it, fill IDENTITY.md and USER.md, then delete BOOTSTRAP.md.

## Memory Rules
- Write durable facts, preferences, and decisions to MEMORY.md.
- Write user profile/identity facts (name, preferred name, timezone, pronouns) to USER.md.
- Write agent self-identity facts (vibe, emoji, avatar) to IDENTITY.md.
- Write day-wise notes to memory/YYYY-MM-DD.md (append-only).
- Do not rely on chat history for long-term recall.
- In shared/group chats, do not leak private MEMORY.md details.

## Safety
- Use tools for real actions; never fabricate tool output.
- Ask before destructive or external side-effect actions.
`

const defaultSoulTemplate = `# SOUL.md

You are EazyClaw. Not a generic chatbot.

## Core Truths
- Be genuinely helpful, not performatively helpful.
- Skip filler; lead with useful action.
- Have clear opinions when they help.
- Be resourceful before asking for clarification.
- Earn trust through competence and discretion.

## Boundaries
- Keep private things private.
- Ask before external side effects.
- Never fabricate tool outputs.
- In group contexts, be helpful without leaking private memory.

## Vibe
- Concise when possible, thorough when needed.
- Human, grounded, and direct.
- Confident without bluffing.
`

const defaultIdentityTemplate = `# IDENTITY.md - Agent Identity

- Name: EazyClaw
- Role: Personal AI assistant
- Vibe:
- Emoji:
- Avatar:
`

const defaultUserTemplate = `# USER.md - About The User

- Name:
- Preferred name:
- Pronouns:
- Timezone:
- Communication style:
- Important preferences:
- Project context:

## Context

Capture durable context that improves future help.
`

const defaultLongTermMemoryTemplate = `# MEMORY.md

Long-term durable memory for this assistant.

## User Preferences

## Ongoing Projects

## Stable Facts

## Open Threads
`

const defaultHeartbeatTemplate = `# HEARTBEAT.md

List periodic checks and proactive tasks.
- If there is nothing actionable, keep this short and respond with HEARTBEAT_OK in heartbeat turns.
`

const defaultDailyMemoryTemplate = `# Daily Memory

Append day-wise memory updates only.
- Keep entries concise and durable.
- Prefer notes, decisions, and outcomes over raw transcript dumps.
`

const defaultBootstrapTemplate = `# BOOTSTRAP.md - Hello, World

You just came online. This is a fresh workspace.

Start by learning:
- who you are (IDENTITY.md)
- who your human is (USER.md)

Suggested first line:
"Hey. I just came online. Who am I? Who are you?"

After the first onboarding conversation:
- update IDENTITY.md
- update USER.md
- refine SOUL.md
- then delete this file
`

const bootstrapStateVersion = 1

type bootstrapState struct {
	Version               int    `json:"version"`
	BootstrapSeededAt     string `json:"bootstrap_seeded_at,omitempty"`
	OnboardingCompletedAt string `json:"onboarding_completed_at,omitempty"`
}

var (
	userNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*i(?:'m| am)\s+([A-Za-z][A-Za-z0-9 .'\-]{0,48})[.!]?\s*$`),
		regexp.MustCompile(`(?i)\bmy name is\s+([A-Za-z][A-Za-z0-9 .'\-]{0,48})(?:[.!?,]|$)`),
	}
	userPreferredNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcall me\s+([A-Za-z][A-Za-z0-9 .'\-]{0,48})(?:[.!?,]|$)`),
		regexp.MustCompile(`(?i)\bi prefer(?: to be)? called\s+([A-Za-z][A-Za-z0-9 .'\-]{0,48})(?:[.!?,]|$)`),
	}
	userTimezonePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:my\s+)?timezone is\s+([A-Za-z0-9_./+\-: ]{2,64})(?:[.!?,]|$)`),
	}
	userPronounsPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bmy pronouns are\s+([A-Za-z/ -]{2,32})(?:[.!?,]|$)`),
	}
)

// MemoryOptions controls context injection, compaction, and background digest behavior.
type MemoryOptions struct {
	Enabled                 bool
	ContextMaxChars         int
	DailyFilesInContext     int
	CompactionEnabled       bool
	CompactionTriggerMsgs   int
	CompactionKeepRecent    int
	CompactionSummaryChars  int
	ContextWindowTokens     int
	ReserveTokensFloor      int
	SoftThresholdTokens     int
	PreCompactionFlush      bool
	BackgroundDigestEnabled bool
	BackgroundDigestEvery   time.Duration
	BackgroundDigestMaxRuns int
	ModelContextWindows     map[string]int
}

// MemoryManager owns OpenClaw-style file memory behavior.
type MemoryManager struct {
	baseDir        string
	memoryDir      string
	agentsPath     string
	soulPath       string
	bootstrapPath  string
	statePath      string
	identityPath   string
	userPath       string
	heartbeatPath  string
	longTermPath   string
	contextMax     int
	dailyInContext int
	opts           MemoryOptions

	mu            sync.Mutex
	lastDigestIdx map[string]int
}

// NewMemoryManager creates a MemoryManager with sane defaults.
func NewMemoryManager(baseDir, memoryDir string, opts MemoryOptions) *MemoryManager {
	if baseDir == "" {
		baseDir = "/data/eazyclaw"
	}
	if memoryDir == "" {
		memoryDir = filepath.Join(baseDir, "memory")
	}

	if opts.ContextMaxChars <= 0 {
		opts.ContextMaxChars = 12_000
	}
	if opts.DailyFilesInContext <= 0 {
		opts.DailyFilesInContext = 2
	}
	if opts.CompactionTriggerMsgs <= 0 {
		opts.CompactionTriggerMsgs = 80
	}
	if opts.CompactionKeepRecent <= 0 {
		opts.CompactionKeepRecent = 30
	}
	if opts.CompactionSummaryChars <= 0 {
		opts.CompactionSummaryChars = 3_500
	}
	if opts.ContextWindowTokens <= 0 {
		opts.ContextWindowTokens = 200_000
	}
	if opts.ReserveTokensFloor <= 0 {
		opts.ReserveTokensFloor = 20_000
	}
	if opts.SoftThresholdTokens <= 0 {
		opts.SoftThresholdTokens = 4_000
	}
	if opts.BackgroundDigestEvery <= 0 {
		opts.BackgroundDigestEvery = 10 * time.Minute
	}
	if opts.BackgroundDigestMaxRuns <= 0 {
		opts.BackgroundDigestMaxRuns = 20
	}

	return &MemoryManager{
		baseDir:        baseDir,
		memoryDir:      memoryDir,
		agentsPath:     filepath.Join(memoryDir, "AGENTS.md"),
		soulPath:       filepath.Join(memoryDir, "SOUL.md"),
		bootstrapPath:  filepath.Join(memoryDir, "BOOTSTRAP.md"),
		statePath:      filepath.Join(memoryDir, ".bootstrap-state.json"),
		identityPath:   filepath.Join(memoryDir, "IDENTITY.md"),
		userPath:       filepath.Join(memoryDir, "USER.md"),
		heartbeatPath:  filepath.Join(memoryDir, "HEARTBEAT.md"),
		longTermPath:   filepath.Join(memoryDir, "MEMORY.md"),
		contextMax:     opts.ContextMaxChars,
		dailyInContext: opts.DailyFilesInContext,
		opts:           opts,
		lastDigestIdx:  make(map[string]int),
	}
}

func (m *MemoryManager) Enabled() bool {
	return m != nil && m.opts.Enabled
}

func (m *MemoryManager) SoulPath() string {
	if m == nil {
		return ""
	}
	return m.soulPath
}

func (m *MemoryManager) AgentsPath() string {
	if m == nil {
		return ""
	}
	return m.agentsPath
}

func (m *MemoryManager) IdentityPath() string {
	if m == nil {
		return ""
	}
	return m.identityPath
}

func (m *MemoryManager) BootstrapPath() string {
	if m == nil {
		return ""
	}
	return m.bootstrapPath
}

func (m *MemoryManager) UserPath() string {
	if m == nil {
		return ""
	}
	return m.userPath
}

func (m *MemoryManager) MemoryDir() string {
	if m == nil {
		return ""
	}
	return m.memoryDir
}

func (m *MemoryManager) HeartbeatPath() string {
	if m == nil {
		return ""
	}
	return m.heartbeatPath
}

func (m *MemoryManager) LongTermPath() string {
	if m == nil {
		return ""
	}
	return m.longTermPath
}

func (m *MemoryManager) ContextWindowTokens() int {
	if m == nil {
		return 0
	}
	return m.opts.ContextWindowTokens
}

// ContextWindowForModel returns the context window size for a specific model.
// Falls back to the default ContextWindowTokens if no per-model override exists.
func (m *MemoryManager) ContextWindowForModel(model string) int {
	if m == nil {
		return 0
	}
	if len(m.opts.ModelContextWindows) > 0 && model != "" {
		if v, ok := m.opts.ModelContextWindows[model]; ok && v > 0 {
			return v
		}
	}
	return m.opts.ContextWindowTokens
}

func (m *MemoryManager) CompactionEnabled() bool {
	return m.Enabled() && m.opts.CompactionEnabled
}

func (m *MemoryManager) PreCompactionFlushEnabled() bool {
	return m.Enabled() && m.opts.CompactionEnabled && m.opts.PreCompactionFlush
}

func (m *MemoryManager) ShouldCompact(msgCount int) bool {
	if !m.CompactionEnabled() {
		return false
	}
	return msgCount >= m.opts.CompactionTriggerMsgs
}

func (m *MemoryManager) ShouldCompactByTokens(estimatedTokens int, contextWindowTokens int) bool {
	if !m.CompactionEnabled() {
		return false
	}
	if contextWindowTokens <= 0 {
		contextWindowTokens = m.opts.ContextWindowTokens
	}
	threshold := contextWindowTokens - m.opts.ReserveTokensFloor
	if threshold <= 0 {
		return false
	}
	return estimatedTokens >= threshold
}

func (m *MemoryManager) ShouldFlushBeforeCompaction(estimatedTokens int, contextWindowTokens int, session *Session) bool {
	if !m.PreCompactionFlushEnabled() {
		return false
	}
	if contextWindowTokens <= 0 {
		contextWindowTokens = m.opts.ContextWindowTokens
	}
	threshold := contextWindowTokens - m.opts.ReserveTokensFloor - m.opts.SoftThresholdTokens
	if threshold <= 0 || estimatedTokens < threshold {
		return false
	}
	if session == nil {
		return true
	}
	if session.MemoryFlushCompactionCount == nil {
		return true
	}
	return *session.MemoryFlushCompactionCount != session.CompactionCount
}

func (m *MemoryManager) KeepRecentMessages() int {
	return m.opts.CompactionKeepRecent
}

func (m *MemoryManager) CompactionSummaryChars() int {
	return m.opts.CompactionSummaryChars
}

func (m *MemoryManager) BackgroundDigestEnabled() bool {
	return m.Enabled() && m.opts.BackgroundDigestEnabled
}

func (m *MemoryManager) BackgroundDigestInterval() time.Duration {
	return m.opts.BackgroundDigestEvery
}

func (m *MemoryManager) BackgroundDigestMaxRuns() int {
	return m.opts.BackgroundDigestMaxRuns
}

// EnsureBootstrapFiles seeds AGENTS.md, SOUL.md, IDENTITY.md, USER.md, HEARTBEAT.md, MEMORY.md, and today's daily file.
func (m *MemoryManager) EnsureBootstrapFiles(now time.Time) error {
	if !m.Enabled() {
		return nil
	}
	if err := os.MkdirAll(m.baseDir, 0o755); err != nil {
		return fmt.Errorf("memory: create base dir: %w", err)
	}
	if err := os.MkdirAll(m.memoryDir, 0o755); err != nil {
		return fmt.Errorf("memory: create memory dir: %w", err)
	}
	if err := writeFileIfMissing(m.agentsPath, defaultAgentsTemplate); err != nil {
		return fmt.Errorf("memory: seed AGENTS.md: %w", err)
	}
	if err := writeFileIfMissing(m.soulPath, defaultSoulTemplate); err != nil {
		return fmt.Errorf("memory: seed SOUL.md: %w", err)
	}
	if err := writeFileIfMissing(m.identityPath, defaultIdentityTemplate); err != nil {
		return fmt.Errorf("memory: seed IDENTITY.md: %w", err)
	}
	if err := writeFileIfMissing(m.userPath, defaultUserTemplate); err != nil {
		return fmt.Errorf("memory: seed USER.md: %w", err)
	}
	if err := writeFileIfMissing(m.heartbeatPath, defaultHeartbeatTemplate); err != nil {
		return fmt.Errorf("memory: seed HEARTBEAT.md: %w", err)
	}
	if err := writeFileIfMissing(m.longTermPath, defaultLongTermMemoryTemplate); err != nil {
		return fmt.Errorf("memory: seed MEMORY.md: %w", err)
	}
	if err := m.ensureBootstrapLifecycle(now); err != nil {
		return fmt.Errorf("memory: seed BOOTSTRAP.md lifecycle: %w", err)
	}
	if err := writeFileIfMissing(m.DailyPath(now), defaultDailyMemoryTemplate); err != nil {
		return fmt.Errorf("memory: seed daily file: %w", err)
	}
	return nil
}

func (m *MemoryManager) ensureBootstrapLifecycle(now time.Time) error {
	state, err := m.readBootstrapState()
	if err != nil {
		return err
	}
	stateDirty := false
	mark := func(update func(*bootstrapState)) {
		update(&state)
		stateDirty = true
	}
	nowIso := now.UTC().Format(time.RFC3339)

	bootstrapExists := fileExists(m.bootstrapPath)
	if state.BootstrapSeededAt == "" && bootstrapExists {
		mark(func(s *bootstrapState) {
			s.BootstrapSeededAt = nowIso
		})
	}
	if state.OnboardingCompletedAt == "" && state.BootstrapSeededAt != "" && !bootstrapExists {
		mark(func(s *bootstrapState) {
			s.OnboardingCompletedAt = nowIso
		})
	}

	if state.BootstrapSeededAt == "" && state.OnboardingCompletedAt == "" && !bootstrapExists {
		identityContent, _ := os.ReadFile(m.identityPath)
		userContent, _ := os.ReadFile(m.userPath)
		legacyOnboardingCompleted := strings.TrimSpace(string(identityContent)) != strings.TrimSpace(defaultIdentityTemplate) ||
			strings.TrimSpace(string(userContent)) != strings.TrimSpace(defaultUserTemplate)
		if legacyOnboardingCompleted {
			mark(func(s *bootstrapState) {
				s.OnboardingCompletedAt = nowIso
			})
		} else {
			if err := writeFileIfMissing(m.bootstrapPath, defaultBootstrapTemplate); err != nil {
				return err
			}
			if fileExists(m.bootstrapPath) {
				mark(func(s *bootstrapState) {
					s.BootstrapSeededAt = nowIso
				})
			}
		}
	}

	if !stateDirty {
		return nil
	}
	return m.writeBootstrapState(state)
}

func (m *MemoryManager) readBootstrapState() (bootstrapState, error) {
	state := bootstrapState{Version: bootstrapStateVersion}
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return bootstrapState{Version: bootstrapStateVersion}, nil
	}
	if state.Version == 0 {
		state.Version = bootstrapStateVersion
	}
	return state, nil
}

func (m *MemoryManager) writeBootstrapState(state bootstrapState) error {
	if state.Version == 0 {
		state.Version = bootstrapStateVersion
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(m.statePath, data, 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeFileIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// DailyPath returns the date-scoped memory file path.
func (m *MemoryManager) DailyPath(t time.Time) string {
	day := t.Format("2006-01-02")
	return filepath.Join(m.memoryDir, day+".md")
}

// EnsureDailyFile creates today's memory file if missing.
func (m *MemoryManager) EnsureDailyFile(t time.Time) error {
	if !m.Enabled() {
		return nil
	}
	if err := os.MkdirAll(m.memoryDir, 0o755); err != nil {
		return err
	}
	return writeFileIfMissing(m.DailyPath(t), defaultDailyMemoryTemplate)
}

// BuildMemoryContext injects long-term memory and short daily snippets.
func (m *MemoryManager) BuildMemoryContext(isDirect bool, now time.Time) string {
	if !m.Enabled() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Memory policy:\n")
	sb.WriteString("- Durable facts go in memory/MEMORY.md.\n")
	sb.WriteString("- Day-wise notes go in memory/YYYY-MM-DD.md.\n")
	sb.WriteString("- Before answering prior-work questions, use memory_search then memory_read.\n\n")

	remaining := m.contextMax

	appendFile := func(header, path string) {
		if remaining <= 0 {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			return
		}
		trimmed := trimToLimit(string(data), remaining)
		if strings.TrimSpace(trimmed) == "" {
			return
		}
		sb.WriteString(header)
		sb.WriteString("\n")
		sb.WriteString(trimmed)
		sb.WriteString("\n\n")
		remaining -= len(trimmed)
	}

	if isDirect {
		appendFile("### memory/MEMORY.md", m.longTermPath)
	}

	for i := 0; i < m.dailyInContext; i++ {
		day := now.AddDate(0, 0, -i)
		title := "### memory/" + day.Format("2006-01-02") + ".md"
		appendFile(title, m.DailyPath(day))
	}

	return strings.TrimSpace(sb.String())
}

func trimToLimit(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit < 20 {
		return s[:limit]
	}
	return s[:limit-20] + "\n...[truncated]..."
}

// BuildPreCompactionFlushPrompt returns the silent memory flush prompt.
func (m *MemoryManager) BuildPreCompactionFlushPrompt(now time.Time) string {
	day := now.Format("2006-01-02")
	return strings.Join([]string{
		"Pre-compaction memory flush.",
		"Store durable memories now (use memory/" + day + ".md; create memory/ if needed).",
		"IMPORTANT: If the file already exists, APPEND new content only and do not overwrite existing entries.",
		"Update memory/MEMORY.md for long-term facts when relevant.",
		"If there is nothing to store, reply with " + noReplyToken + ".",
		"Current time: " + now.Format(time.RFC3339),
	}, " ")
}

// BuildCompactionPrompt returns the compaction summarization prompt.
func (m *MemoryManager) BuildCompactionPrompt() string {
	return fmt.Sprintf(
		"Summarize old conversation context. Include durable facts, decisions, open tasks, and user preferences. Keep under %d chars.",
		m.CompactionSummaryChars(),
	)
}

// RecordCompaction appends a short compaction marker to today's daily memory file.
func (m *MemoryManager) RecordCompaction(sessionID string, summarizedCount int, summary string, now time.Time) error {
	if !m.Enabled() {
		return nil
	}
	if err := m.EnsureDailyFile(now); err != nil {
		return err
	}
	lines := []string{
		"",
		"## " + now.Format(time.RFC3339),
		"- source: compaction",
		fmt.Sprintf("- session: %s", sessionID),
		fmt.Sprintf("- summarized_messages: %d", summarizedCount),
	}
	if summaryLine := sanitizeForMemory(summary, 600); summaryLine != "" {
		lines = append(lines, "- summary: "+summaryLine)
	}
	lines = append(lines, "")
	entry := strings.Join(lines, "\n")
	f, err := os.OpenFile(m.DailyPath(now), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

// DigestSession appends new user/assistant messages into today's daily memory file.
func (m *MemoryManager) DigestSession(session *Session, now time.Time, source string) error {
	if !m.Enabled() || session == nil {
		return nil
	}
	if source == "turn" {
		// Keep turn-level writes out of day memory to avoid transcript dumping.
		return nil
	}

	m.mu.Lock()
	start := m.lastDigestIdx[session.ID]
	if start < 0 {
		start = 0
	}
	if start >= len(session.Messages) {
		m.mu.Unlock()
		return nil
	}
	end := len(session.Messages)
	m.lastDigestIdx[session.ID] = end
	m.mu.Unlock()

	summary := summarizeDigestWindow(session.Messages[start:end])
	if summary == "" {
		return nil
	}

	if err := m.EnsureDailyFile(now); err != nil {
		return err
	}
	entry := strings.Join([]string{
		"",
		"## " + now.Format(time.RFC3339),
		"- source: " + source,
		"- session: " + session.ID,
		summary,
		"",
	}, "\n")

	f, err := os.OpenFile(m.DailyPath(now), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

func summarizeDigestWindow(messages []providerPkg.Message) string {
	var latestUser string
	var latestAssistant string

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		switch msg.Role {
		case "user":
			if latestUser == "" {
				latestUser = sanitizeForMemory(msg.Content, 260)
			}
		case "assistant":
			if latestAssistant == "" {
				content := strings.TrimSpace(msg.Content)
				if strings.HasPrefix(content, "[COMPACTION SUMMARY]") {
					continue
				}
				latestAssistant = sanitizeForMemory(content, 260)
			}
		}
		if latestUser != "" && latestAssistant != "" {
			break
		}
	}

	lines := make([]string, 0, 2)
	if latestUser != "" {
		lines = append(lines, "- latest_user_intent: "+latestUser)
	}
	if latestAssistant != "" && !strings.EqualFold(latestAssistant, noReplyToken) {
		lines = append(lines, "- latest_assistant_outcome: "+latestAssistant)
	}
	return strings.Join(lines, "\n")
}

func sanitizeForMemory(s string, maxLen int) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.Join(strings.Fields(t), " ")
	if maxLen > 3 && len(t) > maxLen {
		t = t[:maxLen-3] + "..."
	}
	return t
}

// MaybeCaptureUserProfileFromMessage updates USER.md for explicit profile facts in direct chats.
func (m *MemoryManager) MaybeCaptureUserProfileFromMessage(content string) (bool, error) {
	if !m.Enabled() {
		return false, nil
	}
	name := extractFirstCapture(content, userNamePatterns)
	preferred := extractFirstCapture(content, userPreferredNamePatterns)
	timezone := extractFirstCapture(content, userTimezonePatterns)
	pronouns := extractFirstCapture(content, userPronounsPatterns)

	if name == "" && preferred == "" && timezone == "" && pronouns == "" {
		return false, nil
	}

	data, err := os.ReadFile(m.userPath)
	if err != nil {
		return false, err
	}
	original := string(data)
	updated := original

	if name != "" {
		updated = upsertUserField(updated, "Name", name)
		// If preferred name is still blank and user introduced themselves, mirror it.
		if fieldValue(updated, "Preferred name") == "" {
			updated = upsertUserField(updated, "Preferred name", name)
		}
	}
	if preferred != "" {
		updated = upsertUserField(updated, "Preferred name", preferred)
	}
	if timezone != "" {
		updated = upsertUserField(updated, "Timezone", timezone)
	}
	if pronouns != "" {
		updated = upsertUserField(updated, "Pronouns", pronouns)
	}

	if updated == original {
		return false, nil
	}
	if err := os.WriteFile(m.userPath, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func extractFirstCapture(content string, patterns []*regexp.Regexp) string {
	for _, re := range patterns {
		matches := re.FindStringSubmatch(content)
		if len(matches) < 2 {
			continue
		}
		value := normalizeProfileValue(matches[1])
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeProfileValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, " \t\r\n\"'`")
	v = strings.TrimRight(v, ".,;:!?")
	v = strings.Join(strings.Fields(v), " ")
	if len(v) > 64 {
		v = v[:64]
	}
	return strings.TrimSpace(v)
}

func upsertUserField(content, field, value string) string {
	line := fmt.Sprintf("- %s: %s", field, value)
	re := regexp.MustCompile(`(?im)^-\s*` + regexp.QuoteMeta(field) + `\s*:\s*.*$`)
	if re.MatchString(content) {
		return re.ReplaceAllString(content, line)
	}
	return strings.TrimRight(content, "\n") + "\n" + line + "\n"
}

func fieldValue(content, field string) string {
	re := regexp.MustCompile(`(?im)^-\s*` + regexp.QuoteMeta(field) + `\s*:\s*(.*)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	return normalizeProfileValue(matches[1])
}

// CloneMessages returns a shallow copy of message history.
func CloneMessages(in []providerPkg.Message) []providerPkg.Message {
	out := make([]providerPkg.Message, len(in))
	copy(out, in)
	return out
}
