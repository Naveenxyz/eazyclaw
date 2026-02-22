package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileToolBlocksReservedMemoryRootFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tool := NewWriteFileTool(workspace)

	args, _ := json.Marshal(map[string]any{
		"path":    "USER.md",
		"content": "bad write target",
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error when writing reserved memory file via write_file")
	}
	if !strings.Contains(res.Error, "reserved memory file") {
		t.Fatalf("expected reserved memory guidance, got: %q", res.Error)
	}
}

func TestReadFileToolBlocksReservedMemoryRootFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "USER.md")
	if err := os.WriteFile(path, []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tool := NewReadFileTool(workspace)
	args, _ := json.Marshal(map[string]any{"path": "USER.md"})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error when reading reserved memory file via read_file")
	}
}

func TestWriteFileToolAllowsNormalWorkspaceFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tool := NewWriteFileTool(workspace)

	args, _ := json.Marshal(map[string]any{
		"path":    "notes/todo.md",
		"content": "hello",
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Error)
	}

	data, err := os.ReadFile(filepath.Join(workspace, "notes", "todo.md"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}
