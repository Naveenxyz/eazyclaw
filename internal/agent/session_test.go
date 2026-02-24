package agent

import (
	"path/filepath"
	"testing"
	"time"

	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
)

func TestSessionStoreSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := NewSessionStore(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()

	session := &Session{
		ID:                   "discord:user-1",
		CompactionCount:      2,
		LastPromptTokens:     1234,
		TotalInputTokens:     9876,
		TotalOutputTokens:    4321,
		LastTurnInputTokens:  111,
		LastTurnOutputTokens: 222,
		Messages: []providerPkg.Message{
			{Role: "user", Content: "hello"},
			{
				Role:    "assistant",
				Content: "running tool",
				ToolCalls: []providerPkg.ToolCall{
					{ID: "tc1", Name: "shell", Arguments: []byte(`{"command":"echo hi"}`)},
				},
			},
			{Role: "tool", ToolCallID: "tc1", Content: "hi"},
		},
	}
	if err := store.Save(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	got, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if got.ID != session.ID {
		t.Fatalf("unexpected ID: got=%q want=%q", got.ID, session.ID)
	}
	if got.CompactionCount != session.CompactionCount {
		t.Fatalf("unexpected compaction count: got=%d want=%d", got.CompactionCount, session.CompactionCount)
	}
	if got.LastPromptTokens != session.LastPromptTokens {
		t.Fatalf("unexpected last prompt tokens: got=%d want=%d", got.LastPromptTokens, session.LastPromptTokens)
	}
	if got.TotalInputTokens != session.TotalInputTokens {
		t.Fatalf("unexpected total input tokens: got=%d want=%d", got.TotalInputTokens, session.TotalInputTokens)
	}
	if got.TotalOutputTokens != session.TotalOutputTokens {
		t.Fatalf("unexpected total output tokens: got=%d want=%d", got.TotalOutputTokens, session.TotalOutputTokens)
	}
	if got.LastTurnInputTokens != session.LastTurnInputTokens {
		t.Fatalf("unexpected last turn input tokens: got=%d want=%d", got.LastTurnInputTokens, session.LastTurnInputTokens)
	}
	if got.LastTurnOutputTokens != session.LastTurnOutputTokens {
		t.Fatalf("unexpected last turn output tokens: got=%d want=%d", got.LastTurnOutputTokens, session.LastTurnOutputTokens)
	}
	if len(got.Messages) != len(session.Messages) {
		t.Fatalf("unexpected message count: got=%d want=%d", len(got.Messages), len(session.Messages))
	}
	if got.Messages[1].Role != "assistant" || len(got.Messages[1].ToolCalls) != 1 {
		t.Fatalf("tool call message was not preserved")
	}

	list, err := store.ListSessions()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session summary, got %d", len(list))
	}
	if list[0].MessageCount != len(session.Messages) {
		t.Fatalf("unexpected summary message_count: got=%d want=%d", list[0].MessageCount, len(session.Messages))
	}
}

func TestSessionStoreListSessionsPage(t *testing.T) {
	t.Parallel()

	store, err := NewSessionStore(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()

	ids := []string{"web:s1", "web:s2", "web:s3"}
	for _, id := range ids {
		if err := store.Save(&Session{
			ID: id,
			Messages: []providerPkg.Message{
				{Role: "user", Content: "hello " + id},
			},
		}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	page1, err := store.ListSessionsPage(2, 0)
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if page1.Total != 3 {
		t.Fatalf("expected total=3, got %d", page1.Total)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("expected 2 items in page1, got %d", len(page1.Items))
	}
	if !page1.HasMore {
		t.Fatalf("expected page1 has_more=true")
	}

	page2, err := store.ListSessionsPage(2, 2)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if len(page2.Items) != 1 {
		t.Fatalf("expected 1 item in page2, got %d", len(page2.Items))
	}
	if page2.HasMore {
		t.Fatalf("expected page2 has_more=false")
	}
}

func TestSessionStoreLoadMessagesPage(t *testing.T) {
	t.Parallel()

	store, err := NewSessionStore(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()

	sessionID := "telegram:user-7"
	if err := store.Save(&Session{
		ID: sessionID,
		Messages: []providerPkg.Message{
			{Role: "user", Content: "u1"},
			{Role: "assistant", Content: "a1"},
			{Role: "user", Content: "u2"},
			{Role: "assistant", Content: "a2"},
			{Role: "user", Content: "u3"},
		},
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}

	latest, err := store.LoadMessagesPage(sessionID, 2, nil)
	if err != nil {
		t.Fatalf("load latest page: %v", err)
	}
	if latest.TotalMessages != 5 {
		t.Fatalf("expected total_messages=5, got %d", latest.TotalMessages)
	}
	if len(latest.Messages) != 2 {
		t.Fatalf("expected 2 latest messages, got %d", len(latest.Messages))
	}
	if latest.Messages[0].Content != "a2" || latest.Messages[1].Content != "u3" {
		t.Fatalf("unexpected latest page content order")
	}
	if !latest.HasMore || latest.NextBeforeSeq == nil || *latest.NextBeforeSeq != 4 {
		t.Fatalf("expected has_more with next_before_seq=4, got has_more=%v next=%v", latest.HasMore, latest.NextBeforeSeq)
	}

	before4 := int64(4)
	mid, err := store.LoadMessagesPage(sessionID, 2, &before4)
	if err != nil {
		t.Fatalf("load middle page: %v", err)
	}
	if len(mid.Messages) != 2 || mid.Messages[0].Content != "a1" || mid.Messages[1].Content != "u2" {
		t.Fatalf("unexpected middle page content order")
	}
	if !mid.HasMore || mid.NextBeforeSeq == nil || *mid.NextBeforeSeq != 2 {
		t.Fatalf("expected middle page has_more with next_before_seq=2")
	}

	before2 := int64(2)
	oldest, err := store.LoadMessagesPage(sessionID, 2, &before2)
	if err != nil {
		t.Fatalf("load oldest page: %v", err)
	}
	if len(oldest.Messages) != 1 || oldest.Messages[0].Content != "u1" {
		t.Fatalf("unexpected oldest page content")
	}
	if oldest.HasMore || oldest.NextBeforeSeq != nil {
		t.Fatalf("expected no more pages at oldest boundary")
	}
}
