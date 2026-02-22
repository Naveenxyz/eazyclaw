package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validatePath resolves a path and ensures it is within the workspace directory.
func validatePath(path, workspaceDir string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceDir, path)
	}
	path = filepath.Clean(path)

	rel, err := filepath.Rel(workspaceDir, path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %s is outside workspace %s", path, workspaceDir)
	}
	return path, nil
}

// --- ReadFileTool ---

// ReadFileTool reads file contents from the workspace.
type ReadFileTool struct {
	workspaceDir string
}

// NewReadFileTool creates a new ReadFileTool.
func NewReadFileTool(workspaceDir string) *ReadFileTool {
	return &ReadFileTool{workspaceDir: workspaceDir}
}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string  { return "Read the contents of a file" }
func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "File path to read"},
    "offset": {"type": "integer", "description": "Line offset to start reading from (0-based)"},
    "limit": {"type": "integer", "description": "Maximum number of lines to read"}
  },
  "required": ["path"]
}`)
}

func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Path   string `json:"path"`
		Offset *int   `json:"offset,omitempty"`
		Limit  *int   `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}

	absPath, err := validatePath(params.Path, t.workspaceDir)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read file: %v", err), IsError: true}, nil
	}

	content := string(data)

	// Apply offset and limit if provided.
	if params.Offset != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		offset := 0
		if params.Offset != nil && *params.Offset > 0 {
			offset = *params.Offset
		}
		if offset > len(lines) {
			offset = len(lines)
		}
		lines = lines[offset:]

		if params.Limit != nil && *params.Limit > 0 && *params.Limit < len(lines) {
			lines = lines[:*params.Limit]
		}
		content = strings.Join(lines, "\n")
	}

	return &Result{Content: content}, nil
}

// --- WriteFileTool ---

// WriteFileTool writes content to a file in the workspace.
type WriteFileTool struct {
	workspaceDir string
}

// NewWriteFileTool creates a new WriteFileTool.
func NewWriteFileTool(workspaceDir string) *WriteFileTool {
	return &WriteFileTool{workspaceDir: workspaceDir}
}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string  { return "Write content to a file" }
func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "File path to write to"},
    "content": {"type": "string", "description": "Content to write"}
  },
  "required": ["path", "content"]
}`)
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}

	absPath, err := validatePath(params.Path, t.workspaceDir)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	// Create parent directories if needed.
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &Result{Error: fmt.Sprintf("failed to create directories: %v", err), IsError: true}, nil
	}

	if err := os.WriteFile(absPath, []byte(params.Content), 0o644); err != nil {
		return &Result{Error: fmt.Sprintf("failed to write file: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("wrote %d bytes to %s", len(params.Content), absPath)}, nil
}

// --- EditFileTool ---

// EditFileTool performs string replacement edits on files in the workspace.
type EditFileTool struct {
	workspaceDir string
}

// NewEditFileTool creates a new EditFileTool.
func NewEditFileTool(workspaceDir string) *EditFileTool {
	return &EditFileTool{workspaceDir: workspaceDir}
}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string  { return "Edit a file by replacing a string" }
func (t *EditFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "File path to edit"},
    "old_string": {"type": "string", "description": "String to find and replace"},
    "new_string": {"type": "string", "description": "Replacement string"}
  },
  "required": ["path", "old_string", "new_string"]
}`)
}

func (t *EditFileTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}

	absPath, err := validatePath(params.Path, t.workspaceDir)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read file: %v", err), IsError: true}, nil
	}

	content := string(data)
	if !strings.Contains(content, params.OldString) {
		return &Result{Error: "old_string not found in file", IsError: true}, nil
	}

	newContent := strings.Replace(content, params.OldString, params.NewString, 1)
	if err := os.WriteFile(absPath, []byte(newContent), 0o644); err != nil {
		return &Result{Error: fmt.Sprintf("failed to write file: %v", err), IsError: true}, nil
	}

	return &Result{Content: fmt.Sprintf("edited %s", absPath)}, nil
}

// --- ListDirTool ---

// ListDirTool lists directory contents in the workspace.
type ListDirTool struct {
	workspaceDir string
}

// NewListDirTool creates a new ListDirTool.
func NewListDirTool(workspaceDir string) *ListDirTool {
	return &ListDirTool{workspaceDir: workspaceDir}
}

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string  { return "List directory contents" }
func (t *ListDirTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Directory path to list"}
  },
  "required": ["path"]
}`)
}

func (t *ListDirTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}

	absPath, err := validatePath(params.Path, t.workspaceDir)
	if err != nil {
		return &Result{Error: err.Error(), IsError: true}, nil
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to list directory: %v", err), IsError: true}, nil
	}

	var sb strings.Builder
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		sb.WriteString(name)
		sb.WriteString("\n")
	}

	return &Result{Content: sb.String()}, nil
}
