package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
	"github.com/eazyclaw/eazyclaw/internal/router"
	"github.com/eazyclaw/eazyclaw/internal/state"
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

type mockProvider struct {
	name         string
	finalText    string
	usage        providerPkg.Usage
	mu           sync.Mutex
	flushCalls   int
	summaryCalls int
	mainCalls    int
	lastPrompt   string
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) ChatCompletion(ctx context.Context, req *providerPkg.ChatRequest) (*providerPkg.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastPrompt = req.SystemPrompt

	lastUser := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUser = req.Messages[i].Content
			break
		}
	}

	if strings.Contains(req.SystemPrompt, "Summarize old conversation context.") {
		m.summaryCalls++
		return &providerPkg.ChatResponse{
			Content: "summary: key decisions and open tasks",
			Usage:   m.usage,
		}, nil
	}
	if strings.Contains(lastUser, "Pre-compaction memory flush.") {
		m.flushCalls++
		return &providerPkg.ChatResponse{
			Content: noReplyToken,
			Usage:   m.usage,
		}, nil
	}

	m.mainCalls++
	return &providerPkg.ChatResponse{
		Content: m.finalText,
		Usage:   m.usage,
	}, nil
}

func (m *mockProvider) ChatCompletionStream(ctx context.Context, req *providerPkg.ChatRequest) (<-chan providerPkg.StreamEvent, error) {
	ch := make(chan providerPkg.StreamEvent)
	close(ch)
	return ch, nil
}

func TestAgentLoopCompactionAndMemoryFlush(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Date(2026, 2, 22, 14, 0, 0, 0, time.UTC)
	memDir := filepath.Join(base, "memory")
	sessionsDir := filepath.Join(base, "sessions")

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:               true,
		CompactionEnabled:     true,
		CompactionTriggerMsgs: 5,
		CompactionKeepRecent:  2,
		ContextWindowTokens:   40,
		ReserveTokensFloor:    10,
		SoftThresholdTokens:   5,
		PreCompactionFlush:    true,
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	msgBus := bus.New(16)
	reg := providerPkg.NewRegistry("mock-model")
	mock := &mockProvider{name: "mock", finalText: "final user answer"}
	reg.Register(mock, "mock-model")

	toolReg := tool.NewRegistry()
	ctxBuilder := NewContextBuilder(mm.LongTermPath())
	ctxBuilder.SetSoulPath(mm.SoulPath())
	ctxBuilder.SetUserPath(mm.UserPath())
	ctxBuilder.SetMemoryManager(mm)
	ctxBuilder.SetTools(toolReg.List())
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())

	store, err := NewSessionStore(sessionsDir)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()
	stateStore, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()
	r := router.NewRouter(stateStore)
	loop := NewAgentLoop(msgBus, reg, toolReg, store, ctxBuilder, mm, r)

	sessionID := "telegram:user-1"
	session := &Session{
		ID:      sessionID,
		Created: now.Add(-2 * time.Hour),
		Updated: now.Add(-1 * time.Hour),
		Messages: []providerPkg.Message{
			{Role: "user", Content: strings.Repeat("u1 ", 20)},
			{Role: "assistant", Content: strings.Repeat("a1 ", 20)},
			{Role: "user", Content: strings.Repeat("u2 ", 20)},
			{Role: "assistant", Content: strings.Repeat("a2 ", 20)},
			{Role: "user", Content: strings.Repeat("u3 ", 20)},
			{Role: "assistant", Content: strings.Repeat("a3 ", 20)},
		},
	}
	if err := store.Save(session); err != nil {
		t.Fatalf("save seed session: %v", err)
	}

	inbound := bus.Message{
		ID:        "m-1",
		ChannelID: "telegram",
		SenderID:  "user-1",
		Text:      "new question",
		Timestamp: now,
	}
	loop.handleMessage(context.Background(), inbound)

	select {
	case out := <-msgBus.Outbound:
		if out.Text != "final user answer" {
			t.Fatalf("unexpected outbound text: %q", out.Text)
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("expected outbound response")
	}

	updated, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("load updated session: %v", err)
	}
	if len(updated.Messages) == 0 || !strings.Contains(updated.Messages[0].Content, "[COMPACTION SUMMARY]") {
		t.Fatalf("expected compaction summary at start of session history")
	}
	if len(updated.Messages) > 8 {
		t.Fatalf("session should be compacted; got %d messages", len(updated.Messages))
	}

	dayData, err := os.ReadFile(mm.DailyPath(now))
	if err != nil {
		t.Fatalf("read daily memory file: %v", err)
	}
	if !strings.Contains(string(dayData), "- source: compaction") || !strings.Contains(string(dayData), "- session: "+sessionID) {
		t.Fatalf("expected compaction entry in daily memory")
	}
	if !strings.Contains(string(dayData), "summary: summary: key decisions and open tasks") {
		t.Fatalf("expected compact compaction summary in daily memory")
	}
	if strings.Contains(string(dayData), strings.Repeat("u1 ", 10)) {
		t.Fatalf("daily memory should not dump full transcript content")
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.flushCalls == 0 || mock.summaryCalls == 0 || mock.mainCalls == 0 {
		t.Fatalf("expected flush/summary/main calls, got flush=%d summary=%d main=%d", mock.flushCalls, mock.summaryCalls, mock.mainCalls)
	}
}

func TestAgentLoopNoReplySuppressed(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Date(2026, 2, 22, 16, 0, 0, 0, time.UTC)
	memDir := filepath.Join(base, "memory")
	sessionsDir := filepath.Join(base, "sessions")

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:           true,
		CompactionEnabled: false,
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	msgBus := bus.New(4)
	reg := providerPkg.NewRegistry("mock-model")
	mock := &mockProvider{name: "mock", finalText: noReplyToken}
	reg.Register(mock, "mock-model")

	toolReg := tool.NewRegistry()
	ctxBuilder := NewContextBuilder(mm.LongTermPath())
	ctxBuilder.SetMemoryManager(mm)
	ctxBuilder.SetTools(toolReg.List())
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())

	store, err := NewSessionStore(sessionsDir)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()
	stateStore, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()
	r := router.NewRouter(stateStore)
	loop := NewAgentLoop(msgBus, reg, toolReg, store, ctxBuilder, mm, r)

	loop.handleMessage(context.Background(), bus.Message{
		ID:        "m-2",
		ChannelID: "telegram",
		SenderID:  "user-2",
		Text:      "housekeeping turn",
		Timestamp: now,
	})

	select {
	case out := <-msgBus.Outbound:
		t.Fatalf("expected no outbound message, got: %+v", out)
	case <-time.After(250 * time.Millisecond):
		// expected
	}

	dayData, err := os.ReadFile(mm.DailyPath(now))
	if err != nil {
		t.Fatalf("read daily memory file: %v", err)
	}
	if strings.Contains(string(dayData), "housekeeping turn") {
		t.Fatalf("expected no per-turn transcript dump in daily memory")
	}
}

func TestAgentLoopInjectsRuntimeSnapshotInPrompt(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Date(2026, 2, 22, 18, 0, 0, 0, time.UTC)
	memDir := filepath.Join(base, "memory")
	sessionsDir := filepath.Join(base, "sessions")

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:             true,
		CompactionEnabled:   false,
		ContextWindowTokens: 1000,
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	msgBus := bus.New(4)
	reg := providerPkg.NewRegistry("mock-model")
	mock := &mockProvider{name: "mock", finalText: "runtime ok"}
	reg.Register(mock, "mock-model")

	toolReg := tool.NewRegistry()
	ctxBuilder := NewContextBuilder(mm.LongTermPath())
	ctxBuilder.SetMemoryManager(mm)
	ctxBuilder.SetTools(toolReg.List())
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())

	store, err := NewSessionStore(sessionsDir)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()
	stateStore, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()
	r := router.NewRouter(stateStore)
	loop := NewAgentLoop(msgBus, reg, toolReg, store, ctxBuilder, mm, r)

	loop.handleMessage(context.Background(), bus.Message{
		ID:        "m-3",
		ChannelID: "discord",
		SenderID:  "user-3",
		Text:      "who are you and which model are you",
		Timestamp: now,
	})

	select {
	case <-msgBus.Outbound:
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("expected outbound response")
	}

	mock.mu.Lock()
	prompt := mock.lastPrompt
	mock.mu.Unlock()

	if !strings.Contains(prompt, "## Runtime Snapshot (live)") {
		t.Fatalf("expected runtime snapshot block in prompt")
	}
	if !strings.Contains(prompt, "- Provider: mock") {
		t.Fatalf("expected provider line in prompt")
	}
	if !strings.Contains(prompt, "- Model: mock-model") {
		t.Fatalf("expected model line in prompt")
	}
	if !strings.Contains(prompt, "- Context window (configured): 1000 tokens") {
		t.Fatalf("expected context window line in prompt")
	}
	if !strings.Contains(prompt, "- Estimated input tokens this call:") {
		t.Fatalf("expected estimated input token line in prompt")
	}
}

func TestAgentLoopInjectsSessionTokenStatsIntoPrompt(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Date(2026, 2, 22, 18, 15, 0, 0, time.UTC)
	memDir := filepath.Join(base, "memory")
	sessionsDir := filepath.Join(base, "sessions")

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:             true,
		CompactionEnabled:   false,
		ContextWindowTokens: 2000,
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	msgBus := bus.New(8)
	reg := providerPkg.NewRegistry("mock-model")
	mock := &mockProvider{
		name:      "mock",
		finalText: "ok",
		usage: providerPkg.Usage{
			InputTokens:  120,
			OutputTokens: 30,
		},
	}
	reg.Register(mock, "mock-model")

	toolReg := tool.NewRegistry()
	ctxBuilder := NewContextBuilder(mm.LongTermPath())
	ctxBuilder.SetMemoryManager(mm)
	ctxBuilder.SetTools(toolReg.List())
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())

	store, err := NewSessionStore(sessionsDir)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()
	stateStore, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()
	r := router.NewRouter(stateStore)
	loop := NewAgentLoop(msgBus, reg, toolReg, store, ctxBuilder, mm, r)

	loop.handleMessage(context.Background(), bus.Message{
		ID:        "m-stats-1",
		ChannelID: "discord",
		SenderID:  "user-stats",
		Text:      "first",
		Timestamp: now,
	})
	select {
	case <-msgBus.Outbound:
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("expected first outbound response")
	}

	loop.handleMessage(context.Background(), bus.Message{
		ID:        "m-stats-2",
		ChannelID: "discord",
		SenderID:  "user-stats",
		Text:      "second",
		Timestamp: now.Add(1 * time.Minute),
	})
	select {
	case <-msgBus.Outbound:
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("expected second outbound response")
	}

	mock.mu.Lock()
	prompt := mock.lastPrompt
	mock.mu.Unlock()

	if !strings.Contains(prompt, "- Session token totals (model API): input=120 output=30 total=150") {
		t.Fatalf("expected session token totals in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "- Last turn token usage: input=120 output=30 total=150") {
		t.Fatalf("expected last turn token usage in prompt")
	}
}

func TestFindSafeSplitPoint_RespectsToolCallGroups(t *testing.T) {
	t.Parallel()
	messages := []providerPkg.Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"}, // idx 2 — valid split
		{Role: "assistant", Content: "", ToolCalls: []providerPkg.ToolCall{{ID: "tc1", Name: "shell"}}}, // idx 3
		{Role: "tool", ToolCallID: "tc1", Content: "result"},                                            // idx 4
		{Role: "assistant", Content: "done"},                                                            // idx 5
		{Role: "user", Content: "q3"},                                                                   // idx 6 — valid split
		{Role: "assistant", Content: "", ToolCalls: []providerPkg.ToolCall{{ID: "tc2", Name: "web"}}},   // idx 7
		{Role: "tool", ToolCallID: "tc2", Content: "web result"},                                        // idx 8
		{Role: "user", Content: "q4"},                                                                   // idx 9 — NOT valid (prev is tool, but let's check)
		{Role: "assistant", Content: "final"},                                                           // idx 10
	}

	// desiredKeep=5 → targetIdx=6. Valid split points: idx 2 (prev is assistant without toolcalls), idx 6 (prev is assistant without toolcalls), idx 9 (prev is tool, not assistant with toolcalls, so valid).
	splitIdx := findSafeSplitPoint(messages, 5)

	// The split must not land right after an assistant with tool calls.
	if splitIdx > 0 {
		prev := messages[splitIdx-1]
		if prev.Role == "assistant" && len(prev.ToolCalls) > 0 {
			t.Fatalf("split at %d lands after assistant with tool calls — would orphan tool results", splitIdx)
		}
	}
	if messages[splitIdx].Role != "user" {
		t.Fatalf("split at %d is not a user message (role=%s)", splitIdx, messages[splitIdx].Role)
	}
}

func TestFindSafeSplitPoint_NeverSplitsInsideToolGroup(t *testing.T) {
	t.Parallel()
	// History where every user message follows an assistant+tool_calls except the first.
	messages := []providerPkg.Message{
		{Role: "user", Content: "q1"}, // idx 0
		{Role: "assistant", Content: "", ToolCalls: []providerPkg.ToolCall{{ID: "tc1", Name: "a"}}}, // idx 1
		{Role: "tool", ToolCallID: "tc1", Content: "r1"},                                            // idx 2
		{Role: "user", Content: "q2"},                                                               // idx 3 — prev is tool, valid
		{Role: "assistant", Content: "", ToolCalls: []providerPkg.ToolCall{{ID: "tc2", Name: "b"}}}, // idx 4
		{Role: "tool", ToolCallID: "tc2", Content: "r2"},                                            // idx 5
		{Role: "user", Content: "q3"},                                                               // idx 6 — prev is tool, valid
	}

	splitIdx := findSafeSplitPoint(messages, 3)
	// targetIdx = 4. Valid splits are idx 3 and idx 6. Closest to 4 is idx 3.
	if splitIdx != 3 {
		t.Fatalf("expected split at 3, got %d", splitIdx)
	}
}

// failingMockProvider is a mock that returns errors on summary calls.
type failingMockProvider struct {
	name      string
	finalText string
	mu        sync.Mutex
	mainCalls int
}

func (m *failingMockProvider) Name() string { return m.name }

func (m *failingMockProvider) ChatCompletion(ctx context.Context, req *providerPkg.ChatRequest) (*providerPkg.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lastUser := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUser = req.Messages[i].Content
			break
		}
	}

	if strings.Contains(req.SystemPrompt, "Summarize old conversation context.") {
		return nil, fmt.Errorf("simulated LLM failure")
	}
	if strings.Contains(lastUser, "Pre-compaction memory flush.") {
		return &providerPkg.ChatResponse{Content: noReplyToken}, nil
	}

	m.mainCalls++
	return &providerPkg.ChatResponse{Content: m.finalText}, nil
}

func (m *failingMockProvider) ChatCompletionStream(ctx context.Context, req *providerPkg.ChatRequest) (<-chan providerPkg.StreamEvent, error) {
	ch := make(chan providerPkg.StreamEvent)
	close(ch)
	return ch, nil
}

func TestCompaction_FailedSummaryDoesNotWipeChat(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Date(2026, 2, 22, 14, 0, 0, 0, time.UTC)
	memDir := filepath.Join(base, "memory")
	sessionsDir := filepath.Join(base, "sessions")

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:               true,
		CompactionEnabled:     true,
		CompactionTriggerMsgs: 5,
		CompactionKeepRecent:  2,
		ContextWindowTokens:   40,
		ReserveTokensFloor:    10,
		SoftThresholdTokens:   5,
		PreCompactionFlush:    false, // Disable flush to simplify
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	msgBus := bus.New(16)
	reg := providerPkg.NewRegistry("mock-model")
	mock := &failingMockProvider{name: "mock", finalText: "answer after failed compaction"}
	reg.Register(mock, "mock-model")

	toolReg := tool.NewRegistry()
	ctxBuilder := NewContextBuilder(mm.LongTermPath())
	ctxBuilder.SetSoulPath(mm.SoulPath())
	ctxBuilder.SetUserPath(mm.UserPath())
	ctxBuilder.SetMemoryManager(mm)
	ctxBuilder.SetTools(toolReg.List())
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())

	store, err := NewSessionStore(sessionsDir)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()
	stateStore, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()
	r := router.NewRouter(stateStore)
	loop := NewAgentLoop(msgBus, reg, toolReg, store, ctxBuilder, mm, r)

	sessionID := "telegram:user-fail"
	session := &Session{
		ID:      sessionID,
		Created: now.Add(-2 * time.Hour),
		Updated: now.Add(-1 * time.Hour),
		Messages: []providerPkg.Message{
			{Role: "user", Content: strings.Repeat("u1 ", 20)},
			{Role: "assistant", Content: strings.Repeat("a1 ", 20)},
			{Role: "user", Content: strings.Repeat("u2 ", 20)},
			{Role: "assistant", Content: strings.Repeat("a2 ", 20)},
			{Role: "user", Content: strings.Repeat("u3 ", 20)},
			{Role: "assistant", Content: strings.Repeat("a3 ", 20)},
		},
	}
	if err := store.Save(session); err != nil {
		t.Fatalf("save seed session: %v", err)
	}

	loop.handleMessage(context.Background(), bus.Message{
		ID:        "m-fail",
		ChannelID: "telegram",
		SenderID:  "user-fail",
		Text:      "new question",
		Timestamp: now,
	})

	select {
	case out := <-msgBus.Outbound:
		if out.Text != "answer after failed compaction" {
			t.Fatalf("unexpected outbound text: %q", out.Text)
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("expected outbound response")
	}

	// Verify session was NOT wiped — should have messages (either emergency compacted or original).
	updated, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("load updated session: %v", err)
	}
	if len(updated.Messages) == 0 {
		t.Fatalf("session was wiped — expected messages to survive failed compaction")
	}
	// Should have emergency compaction note.
	hasEmergency := false
	for _, m := range updated.Messages {
		if strings.Contains(m.Content, "EMERGENCY COMPACTION") {
			hasEmergency = true
			break
		}
	}
	if !hasEmergency {
		t.Fatalf("expected emergency compaction note in session")
	}
}

func TestRoughTokenEstimate_UsesByteCount(t *testing.T) {
	t.Parallel()

	// Multi-byte string: 'é' is 2 bytes in UTF-8 → more bytes → higher estimate.
	// This is intentionally conservative for CJK/multibyte text.
	multiByteContent := "héllo" // 6 bytes
	asciiContent := "hello"     // 5 bytes

	multiByteMessages := []providerPkg.Message{{Role: "user", Content: multiByteContent}}
	asciiMessages := []providerPkg.Message{{Role: "user", Content: asciiContent}}

	multiByteTokens := roughTokenEstimate(multiByteMessages)
	asciiTokens := roughTokenEstimate(asciiMessages)

	// Multi-byte should give equal or higher estimate (conservative).
	if multiByteTokens < asciiTokens {
		t.Fatalf("multibyte estimate should be >= ascii: multibyte=%d ascii=%d", multiByteTokens, asciiTokens)
	}
}

type echoTool struct{}

func (e *echoTool) Name() string { return "echo" }

func (e *echoTool) Description() string { return "echo input" }

func (e *echoTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`)
}

func (e *echoTool) Execute(ctx context.Context, args json.RawMessage) (*tool.Result, error) {
	return &tool.Result{Content: `{"ok":true}`}, nil
}

type toolLoopUsageProvider struct {
	mu    sync.Mutex
	calls int
}

func (p *toolLoopUsageProvider) Name() string { return "mock-usage" }

func (p *toolLoopUsageProvider) ChatCompletion(ctx context.Context, req *providerPkg.ChatRequest) (*providerPkg.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++

	if p.calls == 1 {
		return &providerPkg.ChatResponse{
			Content: "running tool",
			ToolCalls: []providerPkg.ToolCall{
				{ID: "tc-1", Name: "echo", Arguments: []byte(`{"text":"hello"}`)},
			},
			Usage: providerPkg.Usage{
				InputTokens:  400,
				OutputTokens: 30,
			},
		}, nil
	}

	return &providerPkg.ChatResponse{
		Content: "done",
		Usage: providerPkg.Usage{
			InputTokens:  650,
			OutputTokens: 40,
		},
	}, nil
}

func (p *toolLoopUsageProvider) ChatCompletionStream(ctx context.Context, req *providerPkg.ChatRequest) (<-chan providerPkg.StreamEvent, error) {
	ch := make(chan providerPkg.StreamEvent)
	close(ch)
	return ch, nil
}

func TestAgentLoopAccumulatesTokenUsageAcrossToolLoop(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Date(2026, 2, 22, 18, 30, 0, 0, time.UTC)
	memDir := filepath.Join(base, "memory")
	sessionsDir := filepath.Join(base, "sessions")

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:             true,
		CompactionEnabled:   false,
		ContextWindowTokens: 4000,
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	msgBus := bus.New(8)
	reg := providerPkg.NewRegistry("mock-model")
	mock := &toolLoopUsageProvider{}
	reg.Register(mock, "mock-model")

	toolReg := tool.NewRegistry()
	toolReg.Register(&echoTool{})

	ctxBuilder := NewContextBuilder(mm.LongTermPath())
	ctxBuilder.SetMemoryManager(mm)
	ctxBuilder.SetTools(toolReg.List())
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())

	store, err := NewSessionStore(sessionsDir)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()
	stateStore, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()
	r := router.NewRouter(stateStore)
	loop := NewAgentLoop(msgBus, reg, toolReg, store, ctxBuilder, mm, r)

	loop.handleMessage(context.Background(), bus.Message{
		ID:        "m-usage",
		ChannelID: "telegram",
		SenderID:  "user-usage",
		Text:      "run the tool",
		Timestamp: now,
	})

	select {
	case out := <-msgBus.Outbound:
		if out.Text != "done" {
			t.Fatalf("unexpected outbound text: %q", out.Text)
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("expected outbound response")
	}

	sessionID := "telegram:user-usage"
	updated, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("load updated session: %v", err)
	}

	if updated.TotalInputTokens != 1050 {
		t.Fatalf("expected total input tokens=1050, got %d", updated.TotalInputTokens)
	}
	if updated.TotalOutputTokens != 70 {
		t.Fatalf("expected total output tokens=70, got %d", updated.TotalOutputTokens)
	}
	if updated.LastTurnInputTokens != 1050 {
		t.Fatalf("expected last turn input tokens=1050, got %d", updated.LastTurnInputTokens)
	}
	if updated.LastTurnOutputTokens != 70 {
		t.Fatalf("expected last turn output tokens=70, got %d", updated.LastTurnOutputTokens)
	}
	if updated.LastPromptTokens != 650 {
		t.Fatalf("expected last prompt tokens from last call=650, got %d", updated.LastPromptTokens)
	}
}

type zeroUsageCompactionProvider struct{}

func (p *zeroUsageCompactionProvider) Name() string { return "mock-zero-usage" }

func (p *zeroUsageCompactionProvider) ChatCompletion(ctx context.Context, req *providerPkg.ChatRequest) (*providerPkg.ChatResponse, error) {
	if strings.Contains(req.SystemPrompt, "Summarize old conversation context.") {
		return &providerPkg.ChatResponse{Content: "summary from fallback path"}, nil
	}
	return &providerPkg.ChatResponse{Content: "final reply"}, nil
}

func (p *zeroUsageCompactionProvider) ChatCompletionStream(ctx context.Context, req *providerPkg.ChatRequest) (<-chan providerPkg.StreamEvent, error) {
	ch := make(chan providerPkg.StreamEvent)
	close(ch)
	return ch, nil
}

func TestAgentLoopAutoCompactsUsingRoughEstimateWhenUsageMissing(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Date(2026, 2, 22, 19, 0, 0, 0, time.UTC)
	memDir := filepath.Join(base, "memory")
	sessionsDir := filepath.Join(base, "sessions")

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:               true,
		CompactionEnabled:     true,
		CompactionTriggerMsgs: 500, // disable message-count trigger
		CompactionKeepRecent:  2,
		ContextWindowTokens:   120,
		ReserveTokensFloor:    20,
		SoftThresholdTokens:   10,
		PreCompactionFlush:    false,
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	msgBus := bus.New(8)
	reg := providerPkg.NewRegistry("mock-model")
	reg.Register(&zeroUsageCompactionProvider{}, "mock-model")

	toolReg := tool.NewRegistry()
	ctxBuilder := NewContextBuilder(mm.LongTermPath())
	ctxBuilder.SetMemoryManager(mm)
	ctxBuilder.SetTools(toolReg.List())
	ctxBuilder.SetToolDescriptions(toolReg.Descriptions())

	store, err := NewSessionStore(sessionsDir)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()
	stateStore, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()
	r := router.NewRouter(stateStore)
	loop := NewAgentLoop(msgBus, reg, toolReg, store, ctxBuilder, mm, r)

	sessionID := "telegram:user-rough"
	long := strings.Repeat("x", 240)
	seed := &Session{
		ID: sessionID,
		Messages: []providerPkg.Message{
			{Role: "user", Content: "u1 " + long},
			{Role: "assistant", Content: "a1 " + long},
			{Role: "user", Content: "u2 " + long},
			{Role: "assistant", Content: "a2 " + long},
		},
	}
	if err := store.Save(seed); err != nil {
		t.Fatalf("save seed session: %v", err)
	}

	loop.handleMessage(context.Background(), bus.Message{
		ID:        "m-rough",
		ChannelID: "telegram",
		SenderID:  "user-rough",
		Text:      "new prompt",
		Timestamp: now,
	})

	select {
	case out := <-msgBus.Outbound:
		if out.Text != "final reply" {
			t.Fatalf("unexpected outbound text: %q", out.Text)
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("expected outbound response")
	}

	updated, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("load updated session: %v", err)
	}
	if updated.CompactionCount == 0 {
		t.Fatalf("expected compaction to run via rough token estimate trigger")
	}
	if len(updated.Messages) == 0 || !strings.Contains(updated.Messages[0].Content, "[COMPACTION SUMMARY]") {
		t.Fatalf("expected compaction summary in session history")
	}
	if updated.LastPromptTokens <= 0 {
		t.Fatalf("expected last prompt tokens to keep a non-zero rough estimate")
	}
}
