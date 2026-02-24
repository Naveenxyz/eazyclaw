package agent

import (
	"context"
	"fmt"
	"github.com/eazyclaw/eazyclaw/internal/bus"
	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
	"github.com/eazyclaw/eazyclaw/internal/router"
	"github.com/eazyclaw/eazyclaw/internal/tool"
	"log/slog"
	"strings"
	"time"
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

	// Handle /clear command — wipe session history.
	if strings.TrimSpace(msg.Text) == "/clear" {
		session.Messages = nil
		session.LastPromptTokens = 0
		session.TotalInputTokens = 0
		session.TotalOutputTokens = 0
		session.LastTurnInputTokens = 0
		session.LastTurnOutputTokens = 0
		session.CompactionCount = 0
		a.saveSession(session)
		a.sendReply(msg, "Session cleared.")
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

	// Use latest known prompt tokens; ensure we include the just-appended user message.
	actualTokens := session.LastPromptTokens
	currentEstimate := roughTokenEstimate(session.Messages) + roughToolDefTokens(toolDefs)
	if currentEstimate > actualTokens {
		actualTokens = currentEstimate
	}
	remainingBeforeLimit := remainingTokens(contextWindowTokens, actualTokens)

	// Build system prompt.
	systemPrompt := a.context.BuildFor(PromptContext{
		SessionID:                sessionID,
		Channel:                  msg.ChannelID,
		IsDirect:                 isDirect,
		IsHeartbeat:              isHeartbeat,
		Now:                      msg.Timestamp,
		Provider:                 prov.Name(),
		Model:                    model,
		ContextWindowTokens:      contextWindowTokens,
		EstimatedContextTokens:   actualTokens,
		EstimatedRemainingTokens: remainingBeforeLimit,
		SessionTotalInputTokens:  session.TotalInputTokens,
		SessionTotalOutputTokens: session.TotalOutputTokens,
		SessionLastTurnInput:     session.LastTurnInputTokens,
		SessionLastTurnOutput:    session.LastTurnOutputTokens,
	})

	// Pre-compaction memory flush + compaction summary.
	if err := a.runCompactionIfNeeded(ctx, session, prov, model, systemPrompt, toolDefs, msg.Timestamp, isHeartbeat, contextWindowTokens); err != nil {
		slog.Warn("agent: compaction step failed", "session_id", sessionID, "error", err)
	}

	// Rebuild system prompt after potential compaction.
	actualTokens = session.LastPromptTokens
	remainingBeforeLimit = remainingTokens(contextWindowTokens, actualTokens)
	systemPrompt = a.context.BuildFor(PromptContext{
		SessionID:                sessionID,
		Channel:                  msg.ChannelID,
		IsDirect:                 isDirect,
		IsHeartbeat:              isHeartbeat,
		Now:                      msg.Timestamp,
		Provider:                 prov.Name(),
		Model:                    model,
		ContextWindowTokens:      contextWindowTokens,
		EstimatedContextTokens:   actualTokens,
		EstimatedRemainingTokens: remainingBeforeLimit,
		SessionTotalInputTokens:  session.TotalInputTokens,
		SessionTotalOutputTokens: session.TotalOutputTokens,
		SessionLastTurnInput:     session.LastTurnInputTokens,
		SessionLastTurnOutput:    session.LastTurnOutputTokens,
	})

	// Main tool loop for the user-visible turn.
	promptEstimateBeforeRun := roughPromptEstimate(session.Messages, toolDefs, systemPrompt)
	historyLenBeforeRun := len(session.Messages)
	updated, usageStats, err := a.runToolLoop(ctx, session.Messages, prov, model, systemPrompt, toolDefs, &msg, true, a.maxIterations, contextWindowTokens)
	if err != nil {
		slog.Error("agent: provider error", "error", err)
		a.sendError(msg, fmt.Sprintf("LLM error: %v", err))
		a.saveSession(session)
		return
	}
	session.Messages = updated
	turnInputTokens := usageStats.TotalInputTokens
	if turnInputTokens <= 0 {
		turnInputTokens = promptEstimateBeforeRun
	}
	turnOutputTokens := usageStats.TotalOutputTokens
	if turnOutputTokens <= 0 {
		turnOutputTokens = roughAssistantOutputTokens(session.Messages, historyLenBeforeRun)
	}

	if turnInputTokens > 0 {
		session.TotalInputTokens += turnInputTokens
		session.LastTurnInputTokens = turnInputTokens
	} else {
		session.LastTurnInputTokens = 0
	}
	if turnOutputTokens > 0 {
		session.TotalOutputTokens += turnOutputTokens
		session.LastTurnOutputTokens = turnOutputTokens
	} else {
		session.LastTurnOutputTokens = 0
	}

	if usageStats.LastInputTokens > 0 {
		session.LastPromptTokens = usageStats.LastInputTokens
	} else {
		session.LastPromptTokens = roughPromptEstimate(session.Messages, toolDefs, systemPrompt)
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
	actualTokensBefore := session.LastPromptTokens // 0 if unknown
	msgCountBefore := len(session.Messages)

	// Parse optional custom instructions after "/compact".
	customInstructions := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(msg.Text), "/compact"))

	// Build system prompt for compaction.
	isDirect := msg.GroupID == ""
	systemPrompt := a.context.BuildFor(PromptContext{
		SessionID:                sessionID,
		Channel:                  msg.ChannelID,
		IsDirect:                 isDirect,
		Now:                      msg.Timestamp,
		Provider:                 prov.Name(),
		Model:                    model,
		ContextWindowTokens:      contextWindowTokens,
		EstimatedContextTokens:   actualTokensBefore,
		EstimatedRemainingTokens: remainingTokens(contextWindowTokens, actualTokensBefore),
		SessionTotalInputTokens:  session.TotalInputTokens,
		SessionTotalOutputTokens: session.TotalOutputTokens,
		SessionLastTurnInput:     session.LastTurnInputTokens,
		SessionLastTurnOutput:    session.LastTurnOutputTokens,
	})

	// Pre-compaction memory flush.
	toolDefs := a.tools.ToolDefs()
	var compactUsage llmUsageStats
	if a.memory.PreCompactionFlushEnabled() {
		flushPrompt := a.memory.BuildPreCompactionFlushPrompt(msg.Timestamp)
		flushMessages := CloneMessages(session.Messages)
		flushMessages = append(flushMessages, providerPkg.Message{
			Role:    "user",
			Content: flushPrompt,
		})
		flushSystemPrompt := systemPrompt + "\n\nThis is a silent pre-compaction housekeeping turn. Reply with NO_REPLY when done."
		if _, usageStats, err := a.runToolLoop(ctx, flushMessages, prov, model, flushSystemPrompt, toolDefs, nil, false, 6, contextWindowTokens); err != nil {
			slog.Warn("agent: manual compact pre-flush failed", "session_id", sessionID, "error", err)
		} else {
			count := session.CompactionCount
			session.MemoryFlushCompactionCount = &count
			compactUsage.Merge(usageStats)
		}
	}

	// Run compaction.
	keep := a.memory.KeepRecentMessages()
	if keep >= len(session.Messages) {
		a.sendReply(msg, fmt.Sprintf("Session only has %d messages (keep_recent=%d) — nothing to compact.", len(session.Messages), keep))
		a.saveSession(session)
		return
	}

	// Find a safe split point at a user message boundary to avoid
	// orphaning tool messages whose assistant+tool_calls was removed.
	splitIdx := findSafeSplitPoint(session.Messages, keep)
	if splitIdx <= 0 || splitIdx >= len(session.Messages) {
		a.sendReply(msg, "Cannot find a safe compaction boundary — try again after more messages.")
		a.saveSession(session)
		return
	}
	summarized := session.Messages[:splitIdx]
	recent := session.Messages[splitIdx:]

	// Clone messages for rollback on failure.
	backup := CloneMessages(session.Messages)

	compactionSystemPrompt := systemPrompt
	if customInstructions != "" {
		compactionSystemPrompt += "\n\nAdditional instructions: " + customInstructions
	}

	summary, summaryUsage, err := a.generateCompactionSummary(ctx, prov, model, summarized, compactionSystemPrompt, contextWindowTokens)
	if err != nil {
		slog.Warn("agent: manual compact summary failed, attempting emergency compaction", "session_id", sessionID, "error", err)
		compactUsage.Merge(summaryUsage)
		a.emergencyCompaction(session)
		mergeUsageIntoSessionTotals(session, compactUsage)
		session.LastTurnInputTokens = compactUsage.TotalInputTokens
		session.LastTurnOutputTokens = compactUsage.TotalOutputTokens
		a.saveSession(session)
		reply := fmt.Sprintf(
			"Emergency compaction performed (summary generation failed).\n\nBefore: %d messages\nAfter: %d messages",
			msgCountBefore, len(session.Messages),
		)
		a.sendReply(msg, reply)
		return
	}
	compactUsage.Merge(summaryUsage)
	if summary == "" {
		// Empty summary — restore original messages.
		session.Messages = backup
		mergeUsageIntoSessionTotals(session, compactUsage)
		session.LastTurnInputTokens = compactUsage.TotalInputTokens
		session.LastTurnOutputTokens = compactUsage.TotalOutputTokens
		a.sendError(msg, "Compaction produced empty summary — session unchanged.")
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

	// Sanitize to fix any orphaned tool messages from the split.
	sanitized, report := SanitizeMessages(next)
	if report.DroppedOrphanedToolMessages > 0 || report.InsertedSyntheticToolResults > 0 {
		slog.Info("agent: post-compact sanitize repaired messages",
			"session_id", sessionID,
			"dropped_orphaned", report.DroppedOrphanedToolMessages,
			"inserted_synthetic", report.InsertedSyntheticToolResults)
	}
	session.Messages = sanitized
	session.CompactionCount++
	session.LastPromptTokens = roughPromptEstimate(session.Messages, toolDefs, systemPrompt)
	mergeUsageIntoSessionTotals(session, compactUsage)
	session.LastTurnInputTokens = compactUsage.TotalInputTokens
	session.LastTurnOutputTokens = compactUsage.TotalOutputTokens

	if err := a.memory.RecordCompaction(sessionID, len(summarized), summary, msg.Timestamp); err != nil {
		slog.Warn("agent: failed to write compaction memory note", "session_id", sessionID, "error", err)
	}

	msgCountAfter := len(session.Messages)
	a.saveSession(session)

	var reply string
	if actualTokensBefore > 0 {
		reply = fmt.Sprintf(
			"Compacted.\n\nBefore: %d messages (%dk tokens)\nAfter: %d messages\nContext window: %dk tokens",
			msgCountBefore, actualTokensBefore/1000,
			msgCountAfter, contextWindowTokens/1000,
		)
	} else {
		reply = fmt.Sprintf(
			"Compacted.\n\nBefore: %d messages\nAfter: %d messages\nContext window: %dk tokens",
			msgCountBefore, msgCountAfter, contextWindowTokens/1000,
		)
	}
	a.sendReply(msg, reply)
}

type llmUsageStats struct {
	TotalInputTokens  int
	TotalOutputTokens int
	LastInputTokens   int
	LastOutputTokens  int
}

func (s *llmUsageStats) Add(usage providerPkg.Usage) {
	if usage.InputTokens > 0 {
		s.TotalInputTokens += usage.InputTokens
		s.LastInputTokens = usage.InputTokens
	}
	if usage.OutputTokens > 0 {
		s.TotalOutputTokens += usage.OutputTokens
		s.LastOutputTokens = usage.OutputTokens
	}
}

func (s *llmUsageStats) Merge(other llmUsageStats) {
	s.TotalInputTokens += other.TotalInputTokens
	s.TotalOutputTokens += other.TotalOutputTokens
	if other.LastInputTokens > 0 {
		s.LastInputTokens = other.LastInputTokens
	}
	if other.LastOutputTokens > 0 {
		s.LastOutputTokens = other.LastOutputTokens
	}
}

func mergeUsageIntoSessionTotals(session *Session, usage llmUsageStats) {
	if session == nil {
		return
	}
	if usage.TotalInputTokens > 0 {
		session.TotalInputTokens += usage.TotalInputTokens
	}
	if usage.TotalOutputTokens > 0 {
		session.TotalOutputTokens += usage.TotalOutputTokens
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
) ([]providerPkg.Message, llmUsageStats, error) {
	history := CloneMessages(messages)
	var usageStats llmUsageStats

	if maxIterations <= 0 {
		maxIterations = 1
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		lastInputHint := usageStats.LastInputTokens
		callPrompt := systemPrompt + buildRuntimeSnapshotPrompt(
			prov.Name(),
			model,
			contextWindowTokens,
			history,
			toolDefs,
			iteration+1,
			maxIterations,
			lastInputHint, // actual from previous iteration, 0 on first
		)
		req := &providerPkg.ChatRequest{
			Model:        model,
			Messages:     history,
			Tools:        toolDefs,
			SystemPrompt: callPrompt,
		}

		resp, err := prov.ChatCompletion(ctx, req)
		if err != nil {
			return history, usageStats, err
		}
		usageStats.Add(resp.Usage)

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
			return history, usageStats, nil
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

	return history, usageStats, nil
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

	// Use latest known prompt tokens, but never below the current rough estimate.
	estimatedTokens := session.LastPromptTokens
	currentEstimate := roughPromptEstimate(session.Messages, toolDefs, systemPrompt)
	if currentEstimate > estimatedTokens {
		estimatedTokens = currentEstimate
	}
	tokenTrigger := estimatedTokens > 0 && a.memory.ShouldCompactByTokens(estimatedTokens, contextWindowTokens)
	messageTrigger := a.memory.ShouldCompact(len(session.Messages))
	compactionNeeded := tokenTrigger || messageTrigger

	shouldFlush := false
	if !isHeartbeat {
		if estimatedTokens > 0 {
			shouldFlush = a.memory.ShouldFlushBeforeCompaction(estimatedTokens, contextWindowTokens, session)
		} else if compactionNeeded && a.memory.PreCompactionFlushEnabled() {
			// When token telemetry is not yet available, run one pre-compaction flush
			// if compaction is already required by message count.
			shouldFlush = session.MemoryFlushCompactionCount == nil || *session.MemoryFlushCompactionCount != session.CompactionCount
		}
	}

	if shouldFlush {
		flushPrompt := a.memory.BuildPreCompactionFlushPrompt(now)
		flushMessages := CloneMessages(session.Messages)
		flushMessages = append(flushMessages, providerPkg.Message{
			Role:    "user",
			Content: flushPrompt,
		})
		flushSystemPrompt := systemPrompt + "\n\nThis is a silent pre-compaction housekeeping turn. Reply with NO_REPLY when done."
		if _, usageStats, err := a.runToolLoop(
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
			mergeUsageIntoSessionTotals(session, usageStats)
			if usageStats.LastInputTokens > 0 {
				session.LastPromptTokens = usageStats.LastInputTokens
			}
		}
	}

	// Trigger compaction on message count (always) or actual tokens (when known).
	if !compactionNeeded {
		return nil
	}

	keep := a.memory.KeepRecentMessages()
	if keep >= len(session.Messages) {
		return nil
	}

	// Clone messages for rollback on failure.
	backup := CloneMessages(session.Messages)

	// Find a safe split point at a user message boundary to avoid
	// orphaning tool messages whose assistant+tool_calls was removed.
	splitIdx := findSafeSplitPoint(session.Messages, keep)
	if splitIdx <= 0 || splitIdx >= len(session.Messages) {
		return nil
	}
	summarized := session.Messages[:splitIdx]
	recent := session.Messages[splitIdx:]

	summary, summaryUsage, err := a.generateCompactionSummary(ctx, prov, model, summarized, systemPrompt, contextWindowTokens)
	if err != nil {
		slog.Warn("agent: compaction summary failed, attempting emergency compaction", "session_id", session.ID, "error", err)
		mergeUsageIntoSessionTotals(session, summaryUsage)
		a.emergencyCompaction(session)
		return nil
	}
	mergeUsageIntoSessionTotals(session, summaryUsage)
	if summary == "" {
		// Empty summary — restore original messages.
		session.Messages = backup
		return fmt.Errorf("compaction produced empty summary")
	}

	compactionMsg := providerPkg.Message{
		Role:    "assistant",
		Content: "[COMPACTION SUMMARY]\n" + summary,
	}
	next := make([]providerPkg.Message, 0, 1+len(recent))
	next = append(next, compactionMsg)
	next = append(next, recent...)

	// Sanitize to fix any orphaned tool messages from the split.
	sanitized, report := SanitizeMessages(next)
	if report.DroppedOrphanedToolMessages > 0 || report.InsertedSyntheticToolResults > 0 {
		slog.Info("agent: post-compaction sanitize repaired messages",
			"session_id", session.ID,
			"dropped_orphaned", report.DroppedOrphanedToolMessages,
			"inserted_synthetic", report.InsertedSyntheticToolResults)
	}
	session.Messages = sanitized
	session.CompactionCount++
	session.LastPromptTokens = roughPromptEstimate(session.Messages, toolDefs, systemPrompt)

	if err := a.memory.RecordCompaction(session.ID, len(summarized), summary, now); err != nil {
		slog.Warn("agent: failed to write compaction memory note", "session_id", session.ID, "error", err)
	}

	return nil
}

// emergencyCompaction drops the oldest 50% of messages and sanitizes the result.
// Used as a fallback when normal compaction summary generation fails.
func (a *AgentLoop) emergencyCompaction(session *Session) {
	if len(session.Messages) < 2 {
		return
	}
	halfIdx := len(session.Messages) / 2
	// Try to align to a safe split point.
	safeSplit := findSafeSplitPoint(session.Messages, len(session.Messages)-halfIdx)
	if safeSplit <= 0 {
		safeSplit = halfIdx
	}

	note := providerPkg.Message{
		Role:    "assistant",
		Content: "[EMERGENCY COMPACTION] Oldest messages were dropped because compaction summary generation failed. Some earlier context may be missing.",
	}
	remaining := session.Messages[safeSplit:]
	next := make([]providerPkg.Message, 0, 1+len(remaining))
	next = append(next, note)
	next = append(next, remaining...)

	sanitized, _ := SanitizeMessages(next)
	session.Messages = sanitized
	session.CompactionCount++
	session.LastPromptTokens = roughTokenEstimate(session.Messages)
	slog.Warn("agent: emergency compaction performed", "session_id", session.ID, "dropped_messages", safeSplit)
}

// findSafeSplitPoint finds the best index to split messages for compaction.
// It ensures the split happens at a user message boundary AND that the preceding
// message is not an assistant with tool calls (which would orphan the tool results
// that follow it in the kept portion).
func findSafeSplitPoint(messages []providerPkg.Message, desiredKeep int) int {
	if desiredKeep >= len(messages) {
		return 0
	}
	targetIdx := len(messages) - desiredKeep

	// Collect all valid split points: user messages where the previous message
	// is NOT an assistant with tool calls.
	var validSplits []int
	for i := 1; i < len(messages); i++ {
		if messages[i].Role != "user" {
			continue
		}
		prev := messages[i-1]
		if prev.Role == "assistant" && len(prev.ToolCalls) > 0 {
			continue
		}
		validSplits = append(validSplits, i)
	}

	if len(validSplits) == 0 {
		return 0
	}

	// Pick the valid split point closest to targetIdx.
	best := validSplits[0]
	bestDist := abs(best - targetIdx)
	for _, idx := range validSplits[1:] {
		d := abs(idx - targetIdx)
		if d < bestDist {
			best = idx
			bestDist = d
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// roughTokenEstimate is a byte-based fallback for when actual provider
// token counts are unavailable (e.g., first iteration of tool loop).
// Uses bytes (not runes) to stay conservative for multibyte text.
func roughTokenEstimate(messages []providerPkg.Message) int {
	totalBytes := 0
	for _, m := range messages {
		totalBytes += len(m.Content) + len(m.Role) + len(m.ToolCallID)
		for _, tc := range m.ToolCalls {
			totalBytes += len(tc.ID) + len(tc.Name) + len(tc.Arguments)
		}
	}
	return totalBytes / 4
}

func roughToolDefTokens(toolDefs []providerPkg.ToolDef) int {
	totalBytes := 0
	for _, td := range toolDefs {
		totalBytes += len(td.Name) + len(td.Description) + len(td.Parameters)
	}
	return totalBytes / 4
}

func roughPromptEstimate(messages []providerPkg.Message, toolDefs []providerPkg.ToolDef, systemPrompt string) int {
	est := roughTokenEstimate(messages) + roughToolDefTokens(toolDefs)
	if systemPrompt != "" {
		est += len(systemPrompt) / 4
	}
	return est
}

func roughAssistantOutputTokens(messages []providerPkg.Message, from int) int {
	if from < 0 {
		from = 0
	}
	if from >= len(messages) {
		return 0
	}
	totalBytes := 0
	for i := from; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}
		totalBytes += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			totalBytes += len(tc.ID) + len(tc.Name) + len(tc.Arguments)
		}
	}
	return totalBytes / 4
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
	actualInputTokens int, // from last provider response, 0 if unknown
) string {
	if providerName == "" && model == "" && contextWindowTokens <= 0 {
		return ""
	}

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

	if actualInputTokens > 0 {
		// Real token count from provider API.
		sb.WriteString(fmt.Sprintf("- Input tokens (actual): %d\n", actualInputTokens))
		sb.WriteString(fmt.Sprintf("- Estimated input tokens this call: %d (actual)\n", actualInputTokens))
		if contextWindowTokens > 0 {
			remaining := remainingTokens(contextWindowTokens, actualInputTokens)
			sb.WriteString(fmt.Sprintf("- Context window: %d tokens\n", contextWindowTokens))
			sb.WriteString(fmt.Sprintf("- Remaining: %d tokens\n", remaining))
		}
	} else {
		// No actual data yet — show rough estimate labeled as such.
		est := roughTokenEstimate(history) + roughToolDefTokens(toolDefs)
		sb.WriteString(fmt.Sprintf("- Input tokens (rough estimate): ~%d\n", est))
		sb.WriteString(fmt.Sprintf("- Estimated input tokens this call: ~%d (rough)\n", est))
		if contextWindowTokens > 0 {
			remaining := remainingTokens(contextWindowTokens, est)
			sb.WriteString(fmt.Sprintf("- Context window: %d tokens\n", contextWindowTokens))
			sb.WriteString(fmt.Sprintf("- Remaining (estimated): ~%d tokens\n", remaining))
		}
	}
	return sb.String()
}

func (a *AgentLoop) generateCompactionSummary(
	ctx context.Context,
	prov providerPkg.Provider,
	model string,
	history []providerPkg.Message,
	systemPrompt string,
	contextWindowTokens int,
) (string, llmUsageStats, error) {
	if len(history) == 0 {
		return "No prior messages to summarize.", llmUsageStats{}, nil
	}

	// Filter out oversized messages (>50% of context window in bytes).
	maxContentBytes := contextWindowTokens * 2 // rough: 50% of context in bytes
	var filtered []providerPkg.Message
	skippedCount := 0
	if maxContentBytes > 0 {
		for _, m := range history {
			if len(m.Content) > maxContentBytes {
				skippedCount++
				continue
			}
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		filtered = history
		skippedCount = 0
	}

	// Add compaction prompt once here — singleSummarize/multiPartSummarize use it as-is.
	fullPrompt := systemPrompt + "\n\n" + a.memory.BuildCompactionPrompt()

	var summary string
	var usageStats llmUsageStats
	var err error

	if len(filtered) > 40 {
		summary, usageStats, err = a.multiPartSummarize(ctx, prov, model, filtered, fullPrompt)
	} else {
		summary, usageStats, err = a.singleSummarize(ctx, prov, model, filtered, fullPrompt)
	}
	if err != nil {
		return "", usageStats, err
	}

	if skippedCount > 0 {
		summary += fmt.Sprintf("\n\n[Note: %d oversized messages were omitted from this summary.]", skippedCount)
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "Compaction completed (empty summary)."
	}

	maxChars := a.memory.CompactionSummaryChars()
	if maxChars > 0 && len(summary) > maxChars {
		summary = summary[:maxChars]
	}
	return summary, usageStats, nil
}

// singleSummarize generates a compaction summary from a single LLM call.
func (a *AgentLoop) singleSummarize(
	ctx context.Context,
	prov providerPkg.Provider,
	model string,
	history []providerPkg.Message,
	systemPrompt string,
) (string, llmUsageStats, error) {
	transcript := serializeMessagesAsText(history)

	req := &providerPkg.ChatRequest{
		Model: model,
		Messages: []providerPkg.Message{
			{Role: "user", Content: transcript},
		},
		SystemPrompt: systemPrompt,
		MaxTokens:    900,
	}

	resp, err := prov.ChatCompletion(ctx, req)
	if err != nil {
		return "", llmUsageStats{}, err
	}
	usageStats := llmUsageStats{}
	usageStats.Add(resp.Usage)
	return strings.TrimSpace(resp.Content), usageStats, nil
}

// multiPartSummarize splits large histories at the midpoint, summarizes each
// half, then merges with a third LLM call.
func (a *AgentLoop) multiPartSummarize(
	ctx context.Context,
	prov providerPkg.Provider,
	model string,
	history []providerPkg.Message,
	systemPrompt string,
) (string, llmUsageStats, error) {
	mid := len(history) / 2
	usageStats := llmUsageStats{}

	firstSummary, firstUsage, err := a.singleSummarize(ctx, prov, model, history[:mid], systemPrompt)
	if err != nil {
		return "", usageStats, err
	}
	usageStats.Merge(firstUsage)

	secondSummary, secondUsage, err := a.singleSummarize(ctx, prov, model, history[mid:], systemPrompt)
	if err != nil {
		return "", usageStats, err
	}
	usageStats.Merge(secondUsage)

	// Merge the two summaries with a third LLM call.
	mergePrompt := "Merge these two conversation summaries into one concise summary. Preserve all key facts, decisions, and open tasks.\n\n" +
		"PART 1:\n" + firstSummary + "\n\nPART 2:\n" + secondSummary

	mergeReq := &providerPkg.ChatRequest{
		Model: model,
		Messages: []providerPkg.Message{
			{Role: "user", Content: mergePrompt},
		},
		SystemPrompt: systemPrompt,
		MaxTokens:    900,
	}

	resp, err := prov.ChatCompletion(ctx, mergeReq)
	if err != nil {
		// Fall back to concatenation if merge fails.
		slog.Warn("agent: multi-part merge call failed, falling back to concatenation", "error", err)
		return firstSummary + "\n\n" + secondSummary, usageStats, nil
	}
	usageStats.Add(resp.Usage)
	return strings.TrimSpace(resp.Content), usageStats, nil
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
	channelID := msg.ChannelID
	chatID := msg.GroupID
	if chatID == "" {
		chatID = msg.SenderID
	}
	if msg.ReplyChannelID != "" && msg.ReplyChatID != "" {
		channelID = msg.ReplyChannelID
		chatID = msg.ReplyChatID
	}
	a.bus.Outbound <- bus.OutboundMessage{
		ChannelID: channelID,
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
