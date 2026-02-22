package tool

import "testing"

func TestFilterEnv_PreservesGitHubTokens(t *testing.T) {
	input := []string{
		"GH_TOKEN=ghp_test",
		"GITHUB_TOKEN=ghs_test",
		"OPENAI_API_KEY=sk-test",
		"PATH=/usr/bin:/bin",
	}

	filtered := filterEnv(input)

	if !containsEnv(filtered, "GH_TOKEN=ghp_test") {
		t.Fatalf("expected GH_TOKEN to be preserved")
	}
	if !containsEnv(filtered, "GITHUB_TOKEN=ghs_test") {
		t.Fatalf("expected GITHUB_TOKEN to be preserved")
	}
	if !containsEnv(filtered, "OPENAI_API_KEY=sk-test") {
		t.Fatalf("expected OPENAI_API_KEY to be preserved")
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
		"GH_TOKEN=ghp_test",
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
	if !containsKey(filtered, "GH_TOKEN") {
		t.Fatalf("expected GH_TOKEN to remain")
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

