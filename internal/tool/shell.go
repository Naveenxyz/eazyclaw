package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/config"
)

// ShellTool executes shell commands in the workspace.
type ShellTool struct {
	workspaceDir  string
	denyPatterns  []*regexp.Regexp
	timeout       time.Duration
	workspaceOnly bool
}

// NewShellTool creates a new ShellTool from config.
func NewShellTool(cfg config.ShellConfig, workspaceDir string) *ShellTool {
	patterns := make([]*regexp.Regexp, 0, len(cfg.DenyPatterns))
	for _, p := range cfg.DenyPatterns {
		re, err := regexp.Compile(p)
		if err == nil {
			patterns = append(patterns, re)
		}
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &ShellTool{
		workspaceDir:  workspaceDir,
		denyPatterns:  patterns,
		timeout:       timeout,
		workspaceOnly: cfg.WorkspaceOnly,
	}
}

func (t *ShellTool) Name() string { return "shell" }

func (t *ShellTool) Description() string { return "Execute shell commands in the workspace" }

func (t *ShellTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {"type": "string", "description": "Shell command to execute"}
  },
  "required": ["command"]
}`)
}

func (t *ShellTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &Result{Error: fmt.Sprintf("invalid arguments: %v", err), IsError: true}, nil
	}
	if params.Command == "" {
		return &Result{Error: "command is required", IsError: true}, nil
	}

	// Check against deny patterns.
	for _, re := range t.denyPatterns {
		if re.MatchString(params.Command) {
			return &Result{
				Error:   fmt.Sprintf("command denied by pattern: %s", re.String()),
				IsError: true,
			}, nil
		}
	}

	execCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", params.Command)

	if t.workspaceOnly {
		cmd.Dir = t.workspaceDir
	}

	// Strip sensitive env vars from the environment.
	cmd.Env = filterEnv(os.Environ())

	output, err := cmd.CombinedOutput()
	result := string(output)

	// Truncate at 50000 chars.
	if len(result) > 50000 {
		result = result[:50000] + "\n... [output truncated at 50000 chars]"
	}

	if err != nil {
		return &Result{
			Content: result,
			Error:   err.Error(),
			IsError: true,
		}, nil
	}

	return &Result{Content: result}, nil
}

// filterEnv removes environment variables that may contain API keys or secrets.
func filterEnv(environ []string) []string {
	sensitiveKeys := []string{
		"API_KEY", "SECRET", "TOKEN", "PASSWORD", "CREDENTIAL",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "AWS_SECRET",
	}
	filtered := make([]string, 0, len(environ))
	for _, env := range environ {
		key := strings.SplitN(env, "=", 2)[0]
		upper := strings.ToUpper(key)
		skip := false
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(upper, sensitive) {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, env)
		}
	}
	return filtered
}
