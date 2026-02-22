package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	providerPkg "github.com/eazyclaw/eazyclaw/internal/provider"
)

func TestMemoryManagerEnsureBootstrapFilesAndContext(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	memDir := filepath.Join(base, "memory")
	now := time.Date(2026, 2, 22, 9, 0, 0, 0, time.UTC)

	mm := NewMemoryManager(base, memDir, MemoryOptions{
		Enabled:             true,
		CompactionEnabled:   true,
		DailyFilesInContext: 2,
	})

	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	for _, p := range []string{
		mm.AgentsPath(),
		mm.SoulPath(),
		mm.BootstrapPath(),
		mm.IdentityPath(),
		mm.UserPath(),
		mm.HeartbeatPath(),
		mm.LongTermPath(),
		mm.DailyPath(now),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s to exist: %v", p, err)
		}
	}

	if err := os.WriteFile(mm.LongTermPath(), []byte("LONG_TERM_NOTE"), 0o644); err != nil {
		t.Fatalf("write long-term memory: %v", err)
	}
	if err := os.WriteFile(mm.DailyPath(now), []byte("TODAY_NOTE"), 0o644); err != nil {
		t.Fatalf("write today memory: %v", err)
	}
	yesterday := now.AddDate(0, 0, -1)
	if err := os.WriteFile(mm.DailyPath(yesterday), []byte("YESTERDAY_NOTE"), 0o644); err != nil {
		t.Fatalf("write yesterday memory: %v", err)
	}

	directCtx := mm.BuildMemoryContext(true, now)
	if !strings.Contains(directCtx, "LONG_TERM_NOTE") {
		t.Fatalf("direct context must include long-term memory")
	}
	if !strings.Contains(directCtx, "TODAY_NOTE") || !strings.Contains(directCtx, "YESTERDAY_NOTE") {
		t.Fatalf("direct context must include day-wise memory snippets")
	}

	groupCtx := mm.BuildMemoryContext(false, now)
	if strings.Contains(groupCtx, "LONG_TERM_NOTE") {
		t.Fatalf("group context must not inject long-term memory")
	}
	if !strings.Contains(groupCtx, "TODAY_NOTE") {
		t.Fatalf("group context should include daily memory snippets")
	}
}

func TestMemoryManagerBootstrapLifecycleDoesNotRecreateAfterCompletion(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	memDir := filepath.Join(base, "memory")
	now := time.Date(2026, 2, 22, 11, 0, 0, 0, time.UTC)

	mm := NewMemoryManager(base, memDir, MemoryOptions{Enabled: true})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}
	if _, err := os.Stat(mm.BootstrapPath()); err != nil {
		t.Fatalf("expected BOOTSTRAP.md to exist after first seed: %v", err)
	}

	if err := os.Remove(mm.BootstrapPath()); err != nil {
		t.Fatalf("remove BOOTSTRAP.md: %v", err)
	}

	if err := mm.EnsureBootstrapFiles(now.Add(5 * time.Minute)); err != nil {
		t.Fatalf("EnsureBootstrapFiles second pass failed: %v", err)
	}
	if _, err := os.Stat(mm.BootstrapPath()); !os.IsNotExist(err) {
		t.Fatalf("expected BOOTSTRAP.md to stay deleted after onboarding completion")
	}
}

func TestMemoryManagerMaybeCaptureUserProfileFromMessage(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	memDir := filepath.Join(base, "memory")
	now := time.Date(2026, 2, 22, 12, 30, 0, 0, time.UTC)

	mm := NewMemoryManager(base, memDir, MemoryOptions{Enabled: true})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	updated, err := mm.MaybeCaptureUserProfileFromMessage("i'm Naveen")
	if err != nil {
		t.Fatalf("capture profile failed: %v", err)
	}
	if !updated {
		t.Fatalf("expected profile update for explicit name")
	}

	updated, err = mm.MaybeCaptureUserProfileFromMessage("my timezone is Asia/Kolkata")
	if err != nil {
		t.Fatalf("capture timezone failed: %v", err)
	}
	if !updated {
		t.Fatalf("expected profile update for timezone")
	}

	data, err := os.ReadFile(mm.UserPath())
	if err != nil {
		t.Fatalf("read USER.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "- Name: Naveen") {
		t.Fatalf("expected USER.md name update, got: %q", content)
	}
	if !strings.Contains(content, "- Preferred name: Naveen") {
		t.Fatalf("expected USER.md preferred name update, got: %q", content)
	}
	if !strings.Contains(content, "- Timezone: Asia/Kolkata") {
		t.Fatalf("expected USER.md timezone update, got: %q", content)
	}
}

func TestMemoryManagerDigestSessionWritesCompactSnapshots(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	memDir := filepath.Join(base, "memory")
	now := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)

	mm := NewMemoryManager(base, memDir, MemoryOptions{Enabled: true})
	if err := mm.EnsureBootstrapFiles(now); err != nil {
		t.Fatalf("EnsureBootstrapFiles failed: %v", err)
	}

	s := &Session{
		ID: "sess-1",
		Messages: []providerPkg.Message{
			{Role: "user", Content: "first message"},
			{Role: "assistant", Content: "first response"},
			{Role: "user", Content: "second message with clearer intent"},
			{Role: "assistant", Content: "second response with outcome"},
		},
	}
	if err := mm.DigestSession(s, now, "background"); err != nil {
		t.Fatalf("DigestSession first background call failed: %v", err)
	}

	s.Messages = append(s.Messages, providerPkg.Message{Role: "user", Content: "third message"})
	if err := mm.DigestSession(s, now, "background"); err != nil {
		t.Fatalf("DigestSession second background call failed: %v", err)
	}

	data, err := os.ReadFile(mm.DailyPath(now))
	if err != nil {
		t.Fatalf("read daily file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "first message") {
		t.Fatalf("digest should avoid raw transcript dumping of old messages: %q", content)
	}
	if !strings.Contains(content, "second message with clearer intent") {
		t.Fatalf("expected compact snapshot with latest intent from first digest")
	}
	if !strings.Contains(content, "third message") {
		t.Fatalf("expected incremental digest snapshot for new message")
	}
}
