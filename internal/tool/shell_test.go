package tool

import "testing"

func TestFilterEnv_StripsSecretLikeVars(t *testing.T) {
	input := []string{
		"GH_TOKEN=ghp_test",
		"GITHUB_TOKEN=ghs_test",
		"OPENAI_API_KEY=sk-test",
		"PATH=/usr/bin:/bin",
		"LANG=en_US.UTF-8",
	}

	filtered := filterEnv(input)

	if containsKey(filtered, "GH_TOKEN") {
		t.Fatalf("expected GH_TOKEN to be filtered")
	}
	if containsKey(filtered, "GITHUB_TOKEN") {
		t.Fatalf("expected GITHUB_TOKEN to be filtered")
	}
	if containsKey(filtered, "OPENAI_API_KEY") {
		t.Fatalf("expected OPENAI_API_KEY to be filtered")
	}
	if !containsKey(filtered, "PATH") {
		t.Fatalf("expected PATH to remain")
	}
	if !containsKey(filtered, "LANG") {
		t.Fatalf("expected LANG to remain")
	}
}

func TestFilterEnv_BlocksRuntimeHijackKeys(t *testing.T) {
	input := []string{
		"BASH_ENV=/tmp/evil.sh",
		"LD_PRELOAD=/tmp/evil.so",
		"DYLD_INSERT_LIBRARIES=/tmp/evil.dylib",
		"BASH_FUNC_echo%%=() { :; }",
		"SHELL=/bin/zsh",
		"PATH=/usr/bin:/bin",
		"HOME=/workspace",
	}

	filtered := filterEnv(input)

	if containsKey(filtered, "BASH_ENV") {
		t.Fatalf("expected BASH_ENV to be filtered")
	}
	if containsKey(filtered, "LD_PRELOAD") {
		t.Fatalf("expected LD_PRELOAD to be filtered")
	}
	if containsKey(filtered, "DYLD_INSERT_LIBRARIES") {
		t.Fatalf("expected DYLD_INSERT_LIBRARIES to be filtered")
	}
	if containsKey(filtered, "BASH_FUNC_echo%%") {
		t.Fatalf("expected BASH_FUNC_* to be filtered")
	}
	if containsKey(filtered, "SHELL") {
		t.Fatalf("expected SHELL to be filtered")
	}
	if !containsKey(filtered, "PATH") {
		t.Fatalf("expected PATH to remain")
	}
	if !containsKey(filtered, "HOME") {
		t.Fatalf("expected HOME to remain")
	}
}

func TestValidateWorkspaceCommand_BlocksOutsideAbsolutePaths(t *testing.T) {
	err := validateWorkspaceCommand("cat /etc/passwd", "/workspace")
	if err == nil {
		t.Fatalf("expected absolute path outside workspace to be blocked")
	}
}

func TestValidateWorkspaceCommand_AllowsWorkspaceAbsolutePaths(t *testing.T) {
	err := validateWorkspaceCommand("cat /workspace/project/file.txt", "/workspace")
	if err != nil {
		t.Fatalf("expected workspace path to be allowed, got error: %v", err)
	}
}

func TestValidateWorkspaceCommand_BlocksParentTraversal(t *testing.T) {
	err := validateWorkspaceCommand("cat ../secrets.txt", "/workspace")
	if err == nil {
		t.Fatalf("expected parent traversal to be blocked")
	}
}

func containsEnv(environ []string, item string) bool {
	for _, env := range environ {
		if env == item {
			return true
		}
	}
	return false
}

func containsKey(environ []string, key string) bool {
	prefix := key + "="
	for _, env := range environ {
		if len(env) >= len(prefix) && env[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
