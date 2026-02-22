package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
	"github.com/eazyclaw/eazyclaw/internal/router"
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

const defaultMaxIterations = 20

// AgentLoop is the core agent loop that processes inbound messages,
// calls the LLM provider, executes tools, and sends responses.
type AgentLoop struct {
	bus           *bus.Bus
	providers     *providerPkg.Registry
	tools         *tool.Registry
	sessions      *SessionStore
	context       *ContextBuilder
	memory        *MemoryManager
	maxIterations int
	router        *router.Router
}

// NewAgentLoop creates a new AgentLoop.
func NewAgentLoop(
	b *bus.Bus,
	providers *providerPkg.Registry,
	tools *tool.Registry,
	sessions *SessionStore,
	contextBuilder *ContextBuilder,
	memoryManager *MemoryManager,
	r *router.Router,
) *AgentLoop {
	return &AgentLoop{
		bus:           b,
		providers:     providers,
		tools:         tools,
		sessions:      sessions,
		context:       contextBuilder,
		memory:        memoryManager,
		maxIterations: defaultMaxIterations,
		router:        r,
	}
}

// SetMaxIterations configures the maximum tool-call loop iterations.
func (a *AgentLoop) SetMaxIterations(n int) {
	if n > 0 {
		a.maxIterations = n
	}
}

// Run starts the agent loop, processing messages from the bus until ctx is cancelled.
func (a *AgentLoop) Run(ctx context.Context) error {
	slog.Info("agent loop started")

	if a.memory != nil && a.memory.BackgroundDigestEnabled() {
		go a.runBackgroundDigest(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("agent loop stopped")
			return ctx.Err()
		case msg, ok := <-a.bus.Inbound:
			if !ok {
				slog.Info("agent loop: inbound channel closed")
				return nil
			}
			go a.handleMessage(ctx, msg)
		}
	}
}

// handleMessage processes a single inbound message through the LLM + tool loop.
func (a *AgentLoop) handleMessage(ctx context.Context, msg bus.Message) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Check if the sender is allowed.
	if !a.router.IsAllowed(msg) {
		slog.Warn("agent: message from disallowed user", "channel", msg.ChannelID, "sender", msg.SenderID)
		return
	}

	sessionID := a.router.SessionID(msg)
	slog.Info("agent: handling message", "session_id", sessionID, "sender", msg.SenderID)

	// Load session.
	session, err := a.sessions.Load(sessionID)
	if err != nil {
		slog.Error("agent: failed to load session", "session_id", sessionID, "error", err)
		a.sendError(msg, "Internal error: failed to load session.")
		return
	}

	// Reject empty messages — some providers error on empty user content.
	if msg.Text == "" {
		slog.Debug("agent: ignoring empty message", "session_id", sessionID)
		return
	}

	// Handle /compact command.
	if strings.HasPrefix(strings.TrimSpace(msg.Text), "/compact") {
		a.handleCompactCommand(ctx, session, msg)
		return
	}

	// Append user message to session.
	session.Messages = append(session.Messages, providerPkg.Message{
		Role:    "user",
		Content: msg.Text,
	})
	isDirect := msg.GroupID == ""

	if a.memory != nil {
		if err := a.memory.EnsureDailyFile(msg.Timestamp); err != nil {
			slog.Warn("agent: failed to ensure daily memory file", "error", err)
		}
		if isDirect {
			updated, err := a.memory.MaybeCaptureUserProfileFromMessage(msg.Text)
			if err != nil {
				slog.Warn("agent: failed to update USER.md from direct message", "session_id", sessionID, "error", err)
			} else if updated {
				slog.Info("agent: updated USER.md from direct message", "session_id", sessionID)
			}
		}
	}

	// Get the default provider.
	prov, model, err := a.providers.DefaultProvider()
	if err != nil {
		slog.Error("agent: no default provider", "error", err)
		a.sendError(msg, "Internal error: no LLM provider configured.")
		return
	}

	// Get tool definitions.
	toolDefs := a.tools.ToolDefs()

	// Resolve context window for the active model.
	contextWindowTokens := 0
	if a.memory != nil {
		contextWindowTokens = a.memory.ContextWindowForModel(model)
	}
	isHeartbeat := msg.ChannelID == "heartbeat"

	// Use actual provider-reported prompt tokens when available,
	// fall back to char-based heuristic for fresh sessions.
	estimatedTokens := session.LastPromptTokens
	if estimatedTokens == 0 {
		estimatedTokens = estimateSessionTokens(session.Messages)
	}
	remainingBeforeLimit := remainingTokens(contextWindowTokens, estimatedTokens)

	// Build system prompt.
	systemPrompt := a.context.BuildFor(PromptContext{
		SessionID:                sessionID,
		IsDirect:                 isDirect,
		IsHeartbeat:              isHeartbeat,
		Now:                      msg.Timestamp,
		Provider:                 prov.Name(),
		Model:                    model,
		ContextWindowTokens:      contextWindowTokens,
		EstimatedContextTokens:   estimatedTokens,
		EstimatedRemainingTokens: remainingBeforeLimit,
	})

	// Pre-compaction memory flush + compaction summary.
	if err := a.runCompactionIfNeeded(ctx, session, prov, model, systemPrompt, toolDefs, msg.Timestamp, isHeartbeat, contextWindowTokens); err != nil {
		slog.Warn("agent: compaction step failed", "session_id", sessionID, "error", err)
	}

	estimatedTokens = session.LastPromptTokens
	if estimatedTokens == 0 {
		estimatedTokens = estimateSessionTokens(session.Messages)
	}
	remainingBeforeLimit = remainingTokens(contextWindowTokens, estimatedTokens)
	systemPrompt = a.context.BuildFor(PromptContext{
		SessionID:                sessionID,
		IsDirect:                 isDirect,
		IsHeartbeat:              isHeartbeat,
		Now:                      msg.Timestamp,
		Provider:                 prov.Name(),
		Model:                    model,
		ContextWindowTokens:      contextWindowTokens,
		EstimatedContextTokens:   estimatedTokens,
		EstimatedRemainingTokens: remainingBeforeLimit,
	})

	// Main tool loop for the user-visible turn.
	updated, lastUsage, err := a.runToolLoop(ctx, session.Messages, prov, model, systemPrompt, toolDefs, &msg, true, a.maxIterations, contextWindowTokens)
	if err != nil {
		slog.Error("agent: provider error", "error", err)
		a.sendError(msg, fmt.Sprintf("LLM error: %v", err))
		a.saveSession(session)
		return
	}
	session.Messages = updated
	if lastUsage.InputTokens > 0 {
		session.LastPromptTokens = lastUsage.InputTokens
	}

	// Trim and save session.
	a.sessions.Trim(session, 160)
	a.saveSession(session)
}

// handleCompactCommand processes the /compact slash command.
// It force-runs compaction regardless of thresholds and reports token stats.
func (a *AgentLoop) handleCompactCommand(ctx context.Context, session *Session, msg bus.Message) {
	sessionID := a.router.SessionID(msg)
	slog.Info("agent: manual /compact triggered", "session_id", sessionID)

	if a.memory == nil || !a.memory.CompactionEnabled() {
		a.sendReply(msg, "Compaction is disabled in config.")
		return
	}

	if len(session.Messages) < 2 {
		a.sendReply(msg, "Nothing to compact — session has fewer than 2 messages.")
		return
	}

	prov, model, err := a.providers.DefaultProvider()
	if err != nil {
		a.sendError(msg, "No LLM provider configured for compaction.")
		return
	}

	contextWindowTokens := a.memory.ContextWindowForModel(model)
	promptTokensBefore := session.LastPromptTokens
	msgCountBefore := len(session.Messages)

	// Parse optional custom instructions after "/compact".
	customInstructions := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(msg.Text), "/compact"))

	// Build system prompt for compaction.
	isDirect := msg.GroupID == ""
	systemPrompt := a.context.BuildFor(PromptContext{
		SessionID:           sessionID,
		IsDirect:            isDirect,
		Now:                 msg.Timestamp,
		Provider:            prov.Name(),
		Model:               model,
		ContextWindowTokens: contextWindowTokens,
	})

	// Pre-compaction memory flush.
	toolDefs := a.tools.ToolDefs()
	if a.memory.PreCompactionFlushEnabled() {
		flushPrompt := a.memory.BuildPreCompactionFlushPrompt(msg.Timestamp)
		flushMessages := CloneMessages(session.Messages)
		flushMessages = append(flushMessages, providerPkg.Message{
			Role:    "user",
			Content: flushPrompt,
		})
		flushSystemPrompt := systemPrompt + "\n\nThis is a silent pre-compaction housekeeping turn. Reply with NO_REPLY when done."
		if _, _, err := a.runToolLoop(ctx, flushMessages, prov, model, flushSystemPrompt, toolDefs, nil, false, 6, contextWindowTokens); err != nil {
			slog.Warn("agent: manual compact pre-flush failed", "session_id", sessionID, "error", err)
		} else {
			count := session.CompactionCount
			session.MemoryFlushCompactionCount = &count
		}
	}

	// Run compaction.
	keep := a.memory.KeepRecentMessages()
	if keep >= len(session.Messages) {
		a.sendReply(msg, fmt.Sprintf("Session only has %d messages (keep_recent=%d) — nothing to compact.", len(session.Messages), keep))
		a.saveSession(session)
		return
	}

	summarized := session.Messages[:len(session.Messages)-keep]
	recent := session.Messages[len(session.Messages)-keep:]

	compactionPrompt := a.memory.BuildCompactionPrompt()
	if customInstructions != "" {
		compactionPrompt += "\n\nAdditional instructions: " + customInstructions
	}

	summary, err := a.generateCompactionSummary(ctx, prov, model, summarized, systemPrompt+"\n\n"+compactionPrompt)
	if err != nil {
		a.sendError(msg, fmt.Sprintf("Compaction failed: %v", err))
		a.saveSession(session)
		return
	}

	compactionMsg := providerPkg.Message{
		Role:    "assistant",
		Content: "[COMPACTION SUMMARY]\n" + summary,
	}
	next := make([]providerPkg.Message, 0, 1+len(recent))
	next = append(next, compactionMsg)
	next = append(next, recent...)
	session.Messages = next
	session.CompactionCount++

	if err := a.memory.RecordCompaction(sessionID, len(summarized), summary, msg.Timestamp); err != nil {
		slog.Warn("agent: failed to write compaction memory note", "session_id", sessionID, "error", err)
	}

	msgCountAfter := len(session.Messages)
	a.saveSession(session)

	// Report using actual provider-reported token counts when available.
	if promptTokensBefore > 0 {
		reply := fmt.Sprintf(
			"Compacted.\n\nBefore: %d messages (%dk prompt tokens)\nAfter: %d messages\nContext window: %dk tokens",
			msgCountBefore, promptTokensBefore/1000,
			msgCountAfter,
			contextWindowTokens/1000,
		)
		a.sendReply(msg, reply)
	} else {
		reply := fmt.Sprintf(
			"Compacted.\n\nBefore: %d messages\nAfter: %d messages\nContext window: %dk tokens\n\n(Send a message first to get accurate token counts.)",
			msgCountBefore, msgCountAfter, contextWindowTokens/1000,
		)
		a.sendReply(msg, reply)
	}
}

func (a *AgentLoop) runToolLoop(
	ctx context.Context,
	messages []providerPkg.Message,
	prov providerPkg.Provider,
	model string,
	systemPrompt string,
	toolDefs []providerPkg.ToolDef,
	origin *bus.Message,
	sendReply bool,
	maxIterations int,
	contextWindowTokens int,
) ([]providerPkg.Message, providerPkg.Usage, error) {
	history := CloneMessages(messages)
	var lastUsage providerPkg.Usage

	if maxIterations <= 0 {
		maxIterations = 1
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		callPrompt := systemPrompt + buildRuntimeSnapshotPrompt(
			prov.Name(),
			model,
			contextWindowTokens,
			history,
			toolDefs,
			iteration+1,
			maxIterations,
		)
		req := &providerPkg.ChatRequest{
			Model:        model,
			Messages:     history,
			Tools:        toolDefs,
			SystemPrompt: callPrompt,
		}

		resp, err := prov.ChatCompletion(ctx, req)
		if err != nil {
			return history, lastUsage, err
		}
		lastUsage = resp.Usage

		// If no tool calls, this is the final response.
		if len(resp.ToolCalls) == 0 {
			finalContent := strings.TrimSpace(resp.Content)
			if finalContent != "" {
				history = append(history, providerPkg.Message{
					Role:    "assistant",
					Content: resp.Content,
				})
			}
			if sendReply && origin != nil && finalContent != "" && !strings.EqualFold(finalContent, noReplyToken) {
				a.sendReply(*origin, resp.Content)
			}
			return history, lastUsage, nil
		}

		history = append(history, providerPkg.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			slog.Info("agent: executing tool", "tool", tc.Name, "call_id", tc.ID)

			result, execErr := a.tools.Execute(ctx, tc.Name, tc.Arguments)
			if execErr != nil {
				slog.Error("agent: tool execution error", "tool", tc.Name, "error", execErr)
				result = &tool.Result{
					Error:   fmt.Sprintf("tool execution failed: %v", execErr),
					IsError: true,
				}
			}

			content := result.Content
			if result.IsError && result.Error != "" {
				content = fmt.Sprintf("Error: %s\n%s", result.Error, content)
			}

			history = append(history, providerPkg.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: tc.ID,
			})
		}

		slog.Debug("agent: tool loop iteration complete", "iteration", iteration+1)
	}

	return history, lastUsage, nil
}

func (a *AgentLoop) runCompactionIfNeeded(
	ctx context.Context,
	session *Session,
	prov providerPkg.Provider,
	model string,
	systemPrompt string,
	toolDefs []providerPkg.ToolDef,
	now time.Time,
	isHeartbeat bool,
	contextWindowTokens int,
) error {
	if a.memory == nil {
		return nil
	}

	// Use actual provider-reported tokens when available.
	estimatedTokens := session.LastPromptTokens
	if estimatedTokens == 0 {
		estimatedTokens = estimateSessionTokens(session.Messages)
	}

	if !isHeartbeat && a.memory.ShouldFlushBeforeCompaction(estimatedTokens, session) {
		flushPrompt := a.memory.BuildPreCompactionFlushPrompt(now)
		flushMessages := CloneMessages(session.Messages)
		flushMessages = append(flushMessages, providerPkg.Message{
			Role:    "user",
			Content: flushPrompt,
		})
		flushSystemPrompt := systemPrompt + "\n\nThis is a silent pre-compaction housekeeping turn. Reply with NO_REPLY when done."
		if _, _, err := a.runToolLoop(
			ctx,
			flushMessages,
			prov,
			model,
			flushSystemPrompt,
			toolDefs,
			nil,
			false,
			6,
			contextWindowTokens,
		); err != nil {
			slog.Warn("agent: pre-compaction memory flush failed", "session_id", session.ID, "error", err)
		} else {
			count := session.CompactionCount
			session.MemoryFlushCompactionCount = &count
		}
	}

	if !(a.memory.ShouldCompactByTokens(estimatedTokens) || a.memory.ShouldCompact(len(session.Messages))) {
		return nil
	}

	keep := a.memory.KeepRecentMessages()
	if keep >= len(session.Messages) {
		return nil
	}
	summarized := session.Messages[:len(session.Messages)-keep]
	recent := session.Messages[len(session.Messages)-keep:]

	summary, err := a.generateCompactionSummary(ctx, prov, model, summarized, systemPrompt)
	if err != nil {
		return err
	}

	compactionMsg := providerPkg.Message{
		Role:    "assistant",
		Content: "[COMPACTION SUMMARY]\n" + summary,
	}
	next := make([]providerPkg.Message, 0, 1+len(recent))
	next = append(next, compactionMsg)
	next = append(next, recent...)
	session.Messages = next
	session.CompactionCount++

	if err := a.memory.RecordCompaction(session.ID, len(summarized), summary, now); err != nil {
		slog.Warn("agent: failed to write compaction memory note", "session_id", session.ID, "error", err)
	}

	return nil
}

func estimateSessionTokens(messages []providerPkg.Message) int {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
		totalChars += len(m.Role) + len(m.ToolCallID)
		for _, tc := range m.ToolCalls {
			totalChars += len(tc.ID) + len(tc.Name) + len(tc.Arguments)
		}
	}
	// Rough heuristic: ~4 chars/token in mixed English/code text.
	return totalChars / 4
}

func estimateToolDefTokens(toolDefs []providerPkg.ToolDef) int {
	totalChars := 0
	for _, td := range toolDefs {
		totalChars += len(td.Name) + len(td.Description) + len(td.Parameters)
	}
	return totalChars / 4
}

func remainingTokens(contextWindowTokens, estimatedTokens int) int {
	if contextWindowTokens <= 0 {
		return 0
	}
	remaining := contextWindowTokens - estimatedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

func buildRuntimeSnapshotPrompt(
	providerName string,
	model string,
	contextWindowTokens int,
	history []providerPkg.Message,
	toolDefs []providerPkg.ToolDef,
	iteration int,
	maxIterations int,
) string {
	if providerName == "" && model == "" && contextWindowTokens <= 0 {
		return ""
	}

	messageTokens := estimateSessionTokens(history)
	toolTokens := estimateToolDefTokens(toolDefs)
	estimatedTokens := messageTokens + toolTokens
	remaining := remainingTokens(contextWindowTokens, estimatedTokens)

	var sb strings.Builder
	sb.WriteString("\n\n## Runtime Snapshot (live)\n")
	if providerName != "" {
		sb.WriteString(fmt.Sprintf("- Provider: %s\n", providerName))
	}
	if model != "" {
		sb.WriteString(fmt.Sprintf("- Model: %s\n", model))
	}
	sb.WriteString(fmt.Sprintf("- Tool-loop iteration: %d/%d\n", iteration, maxIterations))
	sb.WriteString(fmt.Sprintf("- Messages in context: %d\n", len(history)))
	sb.WriteString(fmt.Sprintf("- Estimated input tokens this call: %d\n", estimatedTokens))
	if contextWindowTokens > 0 {
		sb.WriteString(fmt.Sprintf("- Context window (configured): %d tokens\n", contextWindowTokens))
		sb.WriteString(fmt.Sprintf("- Estimated remaining before limit: %d tokens\n", remaining))
	}
	sb.WriteString("- Estimates are approximate; use them for planning only\n")
	return sb.String()
}


func (a *AgentLoop) generateCompactionSummary(
	ctx context.Context,
	prov providerPkg.Provider,
	model string,
	history []providerPkg.Message,
	systemPrompt string,
) (string, error) {
	if len(history) == 0 {
		return "No prior messages to summarize.", nil
	}

	// Serialize messages as a plain text timeline (nanobot approach).
	// This avoids sending raw tool_call/tool_result structures to the LLM,
	// which prevents API errors from orphaned tool call IDs after splitting.
	transcript := serializeMessagesAsText(history)

	req := &providerPkg.ChatRequest{
		Model: model,
		Messages: []providerPkg.Message{
			{Role: "user", Content: transcript},
		},
		SystemPrompt: systemPrompt + "\n\n" + a.memory.BuildCompactionPrompt(),
		MaxTokens:    900,
	}

	resp, err := prov.ChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		summary = "Compaction completed (empty summary)."
	}

	maxChars := a.memory.CompactionSummaryChars()
	if maxChars > 0 && len(summary) > maxChars {
		summary = summary[:maxChars]
	}
	return summary, nil
}

// serializeMessagesAsText converts message history into a plain text timeline
// for compaction summarization. Tool calls are represented as "[tools: name1, name2]"
// annotations on assistant messages, and tool results are included as indented output.
// This avoids the tool_use/tool_result pairing problem entirely.
func serializeMessagesAsText(messages []providerPkg.Message) string {
	var sb strings.Builder
	sb.WriteString("## Conversation to Summarize\n\n")
	for _, m := range messages {
		switch m.Role {
		case "user":
			sb.WriteString("USER: ")
			sb.WriteString(m.Content)
			sb.WriteString("\n\n")
		case "assistant":
			sb.WriteString("ASSISTANT")
			if len(m.ToolCalls) > 0 {
				names := make([]string, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					names[i] = tc.Name
				}
				sb.WriteString(fmt.Sprintf(" [tools: %s]", strings.Join(names, ", ")))
			}
			sb.WriteString(": ")
			if m.Content != "" {
				sb.WriteString(m.Content)
			}
			sb.WriteString("\n\n")
		case "tool":
			// Truncate long tool outputs for the summary.
			content := m.Content
			if len(content) > 500 {
				content = content[:500] + "...[truncated]"
			}
			sb.WriteString("  TOOL RESULT: ")
			sb.WriteString(content)
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

func (a *AgentLoop) runBackgroundDigest(ctx context.Context) {
	ticker := time.NewTicker(a.memory.BackgroundDigestInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			summaries, err := a.sessions.ListSessions()
			if err != nil {
				slog.Warn("agent: background memory digest list failed", "error", err)
				continue
			}

			limit := a.memory.BackgroundDigestMaxRuns()
			for i, s := range summaries {
				if i >= limit {
					break
				}
				session, err := a.sessions.Load(s.ID)
				if err != nil {
					continue
				}
				if err := a.memory.DigestSession(session, now, "background"); err != nil {
					slog.Debug("agent: background digest skipped", "session_id", session.ID, "error", err)
				}
			}
		}
	}
}

// sendReply sends a text response back to the originating chat.
func (a *AgentLoop) sendReply(msg bus.Message, text string) {
	chatID := msg.GroupID
	if chatID == "" {
		chatID = msg.SenderID
	}
	a.bus.Outbound <- bus.OutboundMessage{
		ChannelID: msg.ChannelID,
		ChatID:    chatID,
		Text:      text,
		ReplyTo:   msg.ID,
	}
}

// sendError sends an error message back to the originating chat.
func (a *AgentLoop) sendError(msg bus.Message, text string) {
	a.sendReply(msg, text)
}

// saveSession persists the session to disk, logging any errors.
func (a *AgentLoop) saveSession(session *Session) {
	if err := a.sessions.Save(session); err != nil {
		slog.Error("agent: failed to save session", "session_id", session.ID, "error", err)
	}
}
