package agent

import (
	"encoding/json"
	"strings"
	"testing"

	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
)

func TestSanitizeMessages_DropsOrphanedLeadingToolMessages(t *testing.T) {
	t.Parallel()
	messages := []providerPkg.Message{
		{Role: "tool", ToolCallID: "orphan-1", Content: "some result"},
		{Role: "tool", ToolCallID: "orphan-2", Content: "another result"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	result, report := SanitizeMessages(messages)

	if report.DroppedOrphanedToolMessages != 2 {
		t.Fatalf("expected 2 dropped orphaned tool messages, got %d", report.DroppedOrphanedToolMessages)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" || result[1].Role != "assistant" {
		t.Fatalf("expected [user, assistant], got [%s, %s]", result[0].Role, result[1].Role)
	}
}

func TestSanitizeMessages_DropsToolWithoutMatchingAssistant(t *testing.T) {
	t.Parallel()
	messages := []providerPkg.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ToolCalls: []providerPkg.ToolCall{
			{ID: "tc-1", Name: "shell"},
		}},
		{Role: "tool", ToolCallID: "tc-1", Content: "ok"},
		{Role: "tool", ToolCallID: "tc-999", Content: "orphaned result"},
		{Role: "user", Content: "next"},
	}

	result, report := SanitizeMessages(messages)

	if report.DroppedOrphanedToolMessages != 1 {
		t.Fatalf("expected 1 dropped orphaned, got %d", report.DroppedOrphanedToolMessages)
	}
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
}

func TestSanitizeMessages_InsertsSyntheticForMissingToolResult(t *testing.T) {
	t.Parallel()
	messages := []providerPkg.Message{
		{Role: "user", Content: "do something"},
		{Role: "assistant", Content: "", ToolCalls: []providerPkg.ToolCall{
			{ID: "tc-1", Name: "shell", Arguments: json.RawMessage(`{}`)},
		}},
		// No tool result for tc-1 follows.
		{Role: "user", Content: "next question"},
	}

	result, report := SanitizeMessages(messages)

	if report.InsertedSyntheticToolResults != 1 {
		t.Fatalf("expected 1 synthetic insertion, got %d", report.InsertedSyntheticToolResults)
	}
	// Should be: user, assistant, synthetic-tool, user
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	if result[2].Role != "tool" || result[2].ToolCallID != "tc-1" {
		t.Fatalf("expected synthetic tool result at index 2, got role=%s id=%s", result[2].Role, result[2].ToolCallID)
	}
	if !strings.Contains(result[2].Content, "synthetic error inserted") {
		t.Fatalf("expected synthetic error content, got %q", result[2].Content)
	}
}

func TestSanitizeMessages_PreservesValidHistory(t *testing.T) {
	t.Parallel()
	messages := []providerPkg.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ToolCalls: []providerPkg.ToolCall{
			{ID: "tc-1", Name: "shell"},
		}},
		{Role: "tool", ToolCallID: "tc-1", Content: "result"},
		{Role: "assistant", Content: "done"},
		{Role: "user", Content: "thanks"},
		{Role: "assistant", Content: "welcome"},
	}

	result, report := SanitizeMessages(messages)

	if report.DroppedOrphanedToolMessages != 0 || report.InsertedSyntheticToolResults != 0 || report.DroppedInvalidAssistantTurns != 0 {
		t.Fatalf("expected no repairs, got dropped=%d synthetic=%d invalid=%d",
			report.DroppedOrphanedToolMessages, report.InsertedSyntheticToolResults, report.DroppedInvalidAssistantTurns)
	}
	if len(result) != len(messages) {
		t.Fatalf("expected %d messages, got %d", len(messages), len(result))
	}
}

func TestSanitizeMessages_DropsAssistantToolCallsAtStart(t *testing.T) {
	t.Parallel()
	messages := []providerPkg.Message{
		{Role: "assistant", Content: "", ToolCalls: []providerPkg.ToolCall{
			{ID: "tc-1", Name: "shell"},
		}},
		{Role: "tool", ToolCallID: "tc-1", Content: "result"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	result, report := SanitizeMessages(messages)

	if report.DroppedInvalidAssistantTurns != 1 {
		t.Fatalf("expected 1 dropped invalid assistant turn, got %d", report.DroppedInvalidAssistantTurns)
	}
	// The tool result for tc-1 is now orphaned since its assistant was dropped.
	if report.DroppedOrphanedToolMessages != 1 {
		t.Fatalf("expected 1 dropped orphaned tool message, got %d", report.DroppedOrphanedToolMessages)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" || result[1].Role != "assistant" {
		t.Fatalf("expected [user, assistant], got [%s, %s]", result[0].Role, result[1].Role)
	}
}

func TestSanitizeMessages_MultipleToolCallsPartialResults(t *testing.T) {
	t.Parallel()
	messages := []providerPkg.Message{
		{Role: "user", Content: "do two things"},
		{Role: "assistant", Content: "", ToolCalls: []providerPkg.ToolCall{
			{ID: "tc-1", Name: "shell", Arguments: json.RawMessage(`{}`)},
			{ID: "tc-2", Name: "web", Arguments: json.RawMessage(`{}`)},
		}},
		// Only tc-1 has a result; tc-2 is missing.
		{Role: "tool", ToolCallID: "tc-1", Content: "shell output"},
		{Role: "user", Content: "next"},
	}

	result, report := SanitizeMessages(messages)

	if report.InsertedSyntheticToolResults != 1 {
		t.Fatalf("expected 1 synthetic insertion, got %d", report.InsertedSyntheticToolResults)
	}
	// user, assistant, synthetic-for-tc-2, tool(tc-1), user = 5
	// Actually: the synthetic is inserted right after the assistant for tc-2 since tc-1 exists later.
	// Let me trace: i=0 user->keep. i=1 assistant with tc-1,tc-2. tc-1 exists in messages[2:]? yes (tool tc-1 at i=2). tc-2 exists in messages[2:]? no. So insert synthetic for tc-2.
	// result so far: [user, assistant, synthetic-tc-2]. i=2 tool tc-1, owning assistant at result[1], keep. i=3 user, keep.
	// Final: [user, assistant, synthetic-tc-2, tool-tc-1, user] = 5
	if len(result) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(result))
	}
	// The synthetic should be for tc-2.
	syntheticIdx := -1
	for i, m := range result {
		if m.Role == "tool" && strings.Contains(m.Content, "synthetic error") {
			syntheticIdx = i
			break
		}
	}
	if syntheticIdx < 0 {
		t.Fatal("expected to find synthetic tool result")
	}
	if result[syntheticIdx].ToolCallID != "tc-2" {
		t.Fatalf("expected synthetic for tc-2, got %s", result[syntheticIdx].ToolCallID)
	}
	if !strings.Contains(result[syntheticIdx].Content, "web") {
		t.Fatalf("expected synthetic to mention tool name 'web', got %q", result[syntheticIdx].Content)
	}
}
