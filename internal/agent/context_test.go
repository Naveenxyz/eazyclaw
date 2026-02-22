package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextBuilderBuildForRespectsTurnTypeAndSessionType(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	memDir := filepath.Join(base, "memory")
	now := time.Date(2026, 2, 22, 10, 30, 0, 0, time.UTC)

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:             true,
		CompactionEnabled:   true,
		DailyFilesInContext: 2,
	})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	if err := os.WriteFile(mm.SoulPath(), []byte("SOUL_CONTENT"), 0o644); err != nil {
		t.Fatalf("write soul: %v", err)
	}
	if err := os.WriteFile(mm.AgentsPath(), []byte("AGENTS_CONTENT"), 0o644); err != nil {
		t.Fatalf("write agents: %v", err)
	}
	if err := os.WriteFile(mm.IdentityPath(), []byte("IDENTITY_CONTENT"), 0o644); err != nil {
		t.Fatalf("write identity: %v", err)
	}
	if err := os.WriteFile(mm.UserPath(), []byte("USER_CONTENT"), 0o644); err != nil {
		t.Fatalf("write user: %v", err)
	}
	if err := os.WriteFile(mm.LongTermPath(), []byte("LONG_TERM_PRIVATE"), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}

	heartbeatPath := filepath.Join(base, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte("HEARTBEAT_TASKS"), 0o644); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	cb := NewContextBuilder(mm.LongTermPath())
	cb.SetAgentsPath(mm.AgentsPath())
	cb.SetSoulPath(mm.SoulPath())
	cb.SetIdentityPath(mm.IdentityPath())
	cb.SetUserPath(mm.UserPath())
	cb.SetHeartbeatPath(heartbeatPath)
	cb.SetMemoryManager(mm)
	cb.SetTools([]string{"memory_read", "memory_search"})
	cb.SetToolDescriptions(map[string]string{
		"memory_read":   "read memory file",
		"memory_search": "search memory files",
	})

	dmPrompt := cb.BuildFor(PromptContext{
		SessionID:   "telegram:dm",
		IsDirect:    true,
		IsHeartbeat: false,
		Now:         now,
	})

	if !strings.Contains(dmPrompt, "AGENTS_CONTENT") || !strings.Contains(dmPrompt, "SOUL_CONTENT") {
		t.Fatalf("dm prompt should include AGENTS and SOUL content")
	}
	if !strings.Contains(dmPrompt, "IDENTITY_CONTENT") || !strings.Contains(dmPrompt, "USER_CONTENT") {
		t.Fatalf("dm prompt should include IDENTITY and USER content")
	}
	if !strings.Contains(dmPrompt, "LONG_TERM_PRIVATE") {
		t.Fatalf("dm prompt should include long-term memory content")
	}
	if strings.Contains(dmPrompt, "HEARTBEAT_TASKS") {
		t.Fatalf("non-heartbeat prompt must not include heartbeat tasks")
	}

	groupHeartbeatPrompt := cb.BuildFor(PromptContext{
		SessionID:   "telegram:group",
		IsDirect:    false,
		IsHeartbeat: true,
		Now:         now,
	})
	if !strings.Contains(groupHeartbeatPrompt, "HEARTBEAT_TASKS") {
		t.Fatalf("heartbeat prompt should include heartbeat tasks")
	}
	if strings.Contains(groupHeartbeatPrompt, "LONG_TERM_PRIVATE") {
		t.Fatalf("group prompt should not include long-term memory")
	}
}

func TestContextBuilderRuntimeInfoIncludesModelAndContextEstimates(t *testing.T) {
	t.Parallel()

	cb := NewContextBuilder("")
	cb.SetTools([]string{"memory_read", "memory_write"})
	cb.SetSkills([]string{"skill-a"})

	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)
	prompt := cb.BuildFor(PromptContext{
		SessionID:                "discord:dm",
		IsDirect:                 true,
		Now:                      now,
		Provider:                 "anthropic",
		Model:                    "claude-sonnet-4-6",
		ContextWindowTokens:      200000,
		EstimatedContextTokens:   4200,
		EstimatedRemainingTokens: 195800,
	})

	if !strings.Contains(prompt, "- Provider: anthropic") {
		t.Fatalf("expected provider runtime info")
	}
	if !strings.Contains(prompt, "- Model: claude-sonnet-4-6") {
		t.Fatalf("expected model runtime info")
	}
	if !strings.Contains(prompt, "- Context window (configured): 200000 tokens") {
		t.Fatalf("expected context window runtime info")
	}
	if !strings.Contains(prompt, "- Estimated context length now: 4200 tokens") {
		t.Fatalf("expected estimated context token runtime info")
	}
	if !strings.Contains(prompt, "- Estimated remaining before limit: 195800 tokens") {
		t.Fatalf("expected remaining token runtime info")
	}
}
