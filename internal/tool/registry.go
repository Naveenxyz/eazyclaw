package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/eazyclaw/eazyclaw/internal/provider"
)

// Tool is the interface for agent tools.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage // JSON Schema
	Execute(ctx context.Context, args json.RawMessage) (*Result, error)
}

// Result represents the output of a tool execution.
type Result struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
	IsError bool   `json:"is_error"`
}

// Registry manages available tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Execute runs a tool by name with the given arguments.
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (*Result, error) {
	t, ok := r.Get(name)
	if !ok {
		return &Result{Error: fmt.Sprintf("tool not found: %s", name), IsError: true}, nil
	}
	return t.Execute(ctx, args)
}

// ToolDefs returns provider-compatible tool definitions for all registered tools.
func (r *Registry) ToolDefs() []provider.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]provider.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, provider.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// List returns names of all registered tools.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Descriptions returns a map of tool name to description for all registered tools.
func (r *Registry) Descriptions() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	descs := make(map[string]string, len(r.tools))
	for name, t := range r.tools {
		descs[name] = t.Description()
	}
	return descs
}
