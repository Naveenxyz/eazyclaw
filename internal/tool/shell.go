package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func (t *ShellTool) Description() string {
	return "Execute shell commands in the workspace. Preinstalled CLI tools include bash, git, gh, node, npm, python3, uv, rg, fd, tree, wget, zip, unzip, tmux, and jq."
}

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
	if t.workspaceOnly {
		if err := validateWorkspaceCommand(params.Command, t.workspaceDir); err != nil {
			return &Result{Error: err.Error(), IsError: true}, nil
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

func validateWorkspaceCommand(command, workspaceDir string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("command is required")
	}

	tokens := strings.Fields(command)
	for _, token := range tokens {
		clean := strings.TrimSpace(token)
		clean = strings.Trim(clean, "\"'`(),;")
		clean = strings.TrimLeft(clean, "<>|&!")
		if clean == "" {
			continue
		}
		if strings.Contains(clean, "://") {
			continue
		}
		if strings.HasPrefix(clean, "~") {
			return fmt.Errorf("workspace_only mode blocks home-directory references")
		}
		if strings.Contains(clean, "..") {
			return fmt.Errorf("workspace_only mode blocks parent-directory traversal")
		}
		if strings.HasPrefix(clean, "/") {
			abs := filepath.Clean(clean)
			rel, err := filepath.Rel(workspaceDir, abs)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("workspace_only mode blocks absolute path outside workspace: %s", abs)
			}
		}
	}
	return nil
}

// filterEnv removes environment variables that are dangerous for shell/runtime integrity.
func filterEnv(environ []string) []string {
	// Remove env vars that can hijack runtime loading/shell behavior and keep
	// secret-like keys out of shell execution.
	blockedExactKeys := map[string]struct{}{
		"NODE_OPTIONS":  {},
		"NODE_PATH":     {},
		"PYTHONHOME":    {},
		"PYTHONPATH":    {},
		"PERL5LIB":      {},
		"PERL5OPT":      {},
		"RUBYLIB":       {},
		"RUBYOPT":       {},
		"BASH_ENV":      {},
		"ENV":           {},
		"SHELL":         {},
		"GCONV_PATH":    {},
		"IFS":           {},
		"SSLKEYLOGFILE": {},
	}
	blockedPrefixes := []string{"DYLD_", "LD_", "BASH_FUNC_"}
	allowedExact := map[string]struct{}{
		"PATH":      {},
		"HOME":      {},
		"PWD":       {},
		"LANG":      {},
		"LC_ALL":    {},
		"LC_CTYPE":  {},
		"TERM":      {},
		"COLORTERM": {},
		"TMPDIR":    {},
		"TEMP":      {},
		"TMP":       {},
		"USER":      {},
		"LOGNAME":   {},
		"SHLVL":     {},
	}

	filtered := make([]string, 0, len(environ))
	for _, env := range environ {
		key := strings.SplitN(env, "=", 2)[0]
		upper := strings.ToUpper(key)
		skip := false
		if _, blocked := blockedExactKeys[upper]; blocked {
			skip = true
		}
		if !skip {
			for _, prefix := range blockedPrefixes {
				if strings.HasPrefix(upper, prefix) {
					skip = true
					break
				}
			}
		}
		if !skip && isSensitiveEnvKey(upper) {
			skip = true
		}
		if !skip {
			if _, ok := allowedExact[upper]; !ok {
				// Keep unknown vars out by default to prevent accidental secret leakage.
				continue
			}
		}
		if !skip {
			filtered = append(filtered, env)
		}
	}
	return filtered
}

func isSensitiveEnvKey(key string) bool {
	sensitiveParts := []string{"TOKEN", "SECRET", "PASSWORD", "API_KEY", "AUTH", "COOKIE", "CREDENTIAL"}
	for _, part := range sensitiveParts {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}
