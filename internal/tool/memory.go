package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- MemoryReadTool ---

// MemoryReadTool reads files from the memory directory.
type MemoryReadTool struct {
	memoryDir string
}

// NewMemoryReadTool creates a new MemoryReadTool.
func NewMemoryReadTool(dataDir string) *MemoryReadTool {
	return &MemoryReadTool{memoryDir: filepath.Join(dataDir, "memory")}
}

func (t *MemoryReadTool) Name() string        { return "memory_read" }
func (t *MemoryReadTool) Description() string  { return "Read a file from the memory directory" }
func (t *MemoryReadTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Relative path within the memory directory"}
  },
  "required": ["path"]
}`)
}

func (t *MemoryReadTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}

	absPath, err := validatePath(params.Path, t.memoryDir)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read memory file: %v", err), IsError: true}, nil
	}

	return &Result{Content: string(data)}, nil
}

// --- MemoryWriteTool ---

// MemoryWriteTool writes or appends to files in the memory directory.
type MemoryWriteTool struct {
	memoryDir string
}

// NewMemoryWriteTool creates a new MemoryWriteTool.
func NewMemoryWriteTool(dataDir string) *MemoryWriteTool {
	return &MemoryWriteTool{memoryDir: filepath.Join(dataDir, "memory")}
}

func (t *MemoryWriteTool) Name() string        { return "memory_write" }
func (t *MemoryWriteTool) Description() string  { return "Write or append to a file in the memory directory" }
func (t *MemoryWriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Relative path within the memory directory"},
    "content": {"type": "string", "description": "Content to write"},
    "append": {"type": "boolean", "description": "Append to file instead of overwriting"}
  },
  "required": ["path", "content"]
}`)
}

func (t *MemoryWriteTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Append  bool   `json:"append"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}

	absPath, err := validatePath(params.Path, t.memoryDir)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	// Create parent directories if needed.
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &Result{Error: fmt.Sprintf("failed to create directories: %v", err), IsError: true}, nil
	}

	if params.Append {
		f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return &Result{Error: fmt.Sprintf("failed to open file for append: %v", err), IsError: true}, nil
		}
		defer f.Close()
		if _, err := f.WriteString(params.Content); err != nil {
			return &Result{Error: fmt.Sprintf("failed to append to file: %v", err), IsError: true}, nil
		}
	} else {
		if err := os.WriteFile(absPath, []byte(params.Content), 0o644); err != nil {
			return &Result{Error: fmt.Sprintf("failed to write file: %v", err), IsError: true}, nil
		}
	}

	action := "wrote"
	if params.Append {
		action = "appended"
	}
	return &Result{Content: fmt.Sprintf("%s %d bytes to %s", action, len(params.Content), absPath)}, nil
}

// --- MemorySearchTool ---

// MemorySearchTool searches through memory files for matching content.
type MemorySearchTool struct {
	memoryDir string
}

// NewMemorySearchTool creates a new MemorySearchTool.
func NewMemorySearchTool(dataDir string) *MemorySearchTool {
	return &MemorySearchTool{memoryDir: filepath.Join(dataDir, "memory")}
}

func (t *MemorySearchTool) Name() string        { return "memory_search" }
func (t *MemorySearchTool) Description() string  { return "Search through memory files for matching content" }
func (t *MemorySearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {"type": "string", "description": "Search query (case-insensitive)"}
  },
  "required": ["query"]
}`)
}

func (t *MemorySearchTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.Query == "" {
		return &Result{Error: "query is required", IsError: true}, nil
	}

	queryLower := strings.ToLower(params.Query)
	var sb strings.Builder

	err := filepath.Walk(t.memoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip files we can't access
		}
		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		lines := strings.Split(string(data), "\n")
		relPath, _ := filepath.Rel(t.memoryDir, path)
		var matches []string
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), queryLower) {
				matches = append(matches, fmt.Sprintf("  L%d: %s", i+1, strings.TrimSpace(line)))
			}
		}
		if len(matches) > 0 {
			sb.WriteString(fmt.Sprintf("File: %s\n", relPath))
			for _, m := range matches {
				sb.WriteString(m)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
		return nil
	})
	if err != nil {
		return &Result{Error: fmt.Sprintf("search failed: %v", err), IsError: true}, nil
	}

	result := sb.String()
	if result == "" {
		return &Result{Content: "No matches found."}, nil
	}
	return &Result{Content: result}, nil
}
