package claude

import (
	"testing"
)

func TestBuildOneshotArgs(t *testing.T) {
	cfg := OneshotConfig{
		Prompt:     "What is 2+2?",
		Model:      "claude-sonnet-4-20250514",
		SoulMDPath: "/tmp/soul.md",
		MaxBudget:  1.50,
	}

	args := cfg.Args()

	// Verify required flags are present.
	assertContains(t, args, "--print")
	assertContains(t, args, "--output-format")
	assertContains(t, args, "stream-json")
	assertContains(t, args, "--dangerously-skip-permissions")

	// Verify model flag.
	assertContains(t, args, "--model")
	assertContains(t, args, "claude-sonnet-4-20250514")

	// Verify soul path flag.
	assertContains(t, args, "--append-system-prompt-file")
	assertContains(t, args, "/tmp/soul.md")

	// Verify budget flag.
	assertContains(t, args, "--max-budget-usd")
	assertContains(t, args, "1.50")

	// Prompt must be the last argument.
	if args[len(args)-1] != "What is 2+2?" {
		t.Errorf("last arg = %q, want prompt %q", args[len(args)-1], "What is 2+2?")
	}
}

func TestBuildOneshotArgsWithTools(t *testing.T) {
	cfg := OneshotConfig{
		Prompt:          "Run tests",
		AllowedTools:    []string{"Bash", "Read", "Write"},
		DisallowedTools: []string{"WebSearch", "WebFetch"},
	}

	args := cfg.Args()

	// Verify --allowedTools flag is present with comma-joined value.
	assertContains(t, args, "--allowedTools")
	assertContains(t, args, "Bash,Read,Write")

	// Verify --disallowedTools flag is present with comma-joined value.
	assertContains(t, args, "--disallowedTools")
	assertContains(t, args, "WebSearch,WebFetch")

	// Prompt must still be the last argument.
	if args[len(args)-1] != "Run tests" {
		t.Errorf("last arg = %q, want prompt %q", args[len(args)-1], "Run tests")
	}
}

// assertContains checks that the value exists somewhere in the args slice.
func assertContains(t *testing.T, args []string, value string) {
	t.Helper()
	for _, a := range args {
		if a == value {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, value)
}
