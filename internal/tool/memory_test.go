package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryWriteToolRoutesProfileFactsToUserFile(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	memDir := filepath.Join(base, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir memory: %v", err)
	}
	tool := NewMemoryWriteTool(base)

	args, _ := json.Marshal(map[string]any{
		"path": "MEMORY.md",
		"content": strings.Join([]string{
			"# USER.md - About The User",
			"",
			"- Name: Naveen",
			"- Preferred name: Naveen",
		}, "\n"),
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "routed from MEMORY.md") {
		t.Fatalf("expected routing notice, got: %q", res.Content)
	}

	userData, err := os.ReadFile(filepath.Join(memDir, "USER.md"))
	if err != nil {
		t.Fatalf("read USER.md: %v", err)
	}
	if !strings.Contains(string(userData), "- Name: Naveen") {
		t.Fatalf("expected USER.md to contain routed profile data")
	}
}

func TestMemoryWriteToolKeepsRegularMemoryWritesInMemoryFile(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	memDir := filepath.Join(base, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir memory: %v", err)
	}
	tool := NewMemoryWriteTool(base)

	args, _ := json.Marshal(map[string]any{
		"path":    "MEMORY.md",
		"content": "Durable project decision: choose Railway deployment.",
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Error)
	}
	if strings.Contains(res.Content, "routed from") {
		t.Fatalf("did not expect routing for non-profile durable memory write")
	}

	memData, err := os.ReadFile(filepath.Join(memDir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	if !strings.Contains(string(memData), "Railway deployment") {
		t.Fatalf("expected write to remain in MEMORY.md")
	}
}
