package cron

import (
	"testing"
)

func TestBuildCommand(t *testing.T) {
	job := &Job{
		ID:         "test-123",
		Name:       "daily-review",
		Prompt:     "Review the codebase for issues",
		Model:      "sonnet",
		WorkingDir: "/tmp/project",
		MaxBudget:  2.50,
		Timeout:    300,
	}

	cfg := BuildOneshotConfig(job, "/etc/soul.md")

	if cfg.Prompt != job.Prompt {
		t.Errorf("Prompt = %q, want %q", cfg.Prompt, job.Prompt)
	}
	if cfg.Model != job.Model {
		t.Errorf("Model = %q, want %q", cfg.Model, job.Model)
	}
	if cfg.WorkingDir != job.WorkingDir {
		t.Errorf("WorkingDir = %q, want %q", cfg.WorkingDir, job.WorkingDir)
	}
	if cfg.MaxBudget != job.MaxBudget {
		t.Errorf("MaxBudget = %f, want %f", cfg.MaxBudget, job.MaxBudget)
	}
	if cfg.SoulMDPath != "/etc/soul.md" {
		t.Errorf("SoulMDPath = %q, want %q", cfg.SoulMDPath, "/etc/soul.md")
	}
	if cfg.Timeout != job.Timeout {
		t.Errorf("Timeout = %d, want %d", cfg.Timeout, job.Timeout)
	}
	if len(cfg.AllowedTools) != 0 {
		t.Errorf("AllowedTools = %v, want empty", cfg.AllowedTools)
	}
	if len(cfg.DisallowedTools) != 0 {
		t.Errorf("DisallowedTools = %v, want empty", cfg.DisallowedTools)
	}
}

func TestBuildCommandWithTools(t *testing.T) {
	job := &Job{
		ID:              "test-456",
		Name:            "tool-job",
		Prompt:          "Do work with tools",
		Model:           "opus",
		WorkingDir:      "/tmp/tools",
		AllowedTools:    `["Bash","Read"]`,
		DisallowedTools: `["Write"]`,
		MaxBudget:       1.00,
	}

	cfg := BuildOneshotConfig(job, "")

	if len(cfg.AllowedTools) != 2 {
		t.Fatalf("AllowedTools length = %d, want 2", len(cfg.AllowedTools))
	}
	if cfg.AllowedTools[0] != "Bash" {
		t.Errorf("AllowedTools[0] = %q, want %q", cfg.AllowedTools[0], "Bash")
	}
	if cfg.AllowedTools[1] != "Read" {
		t.Errorf("AllowedTools[1] = %q, want %q", cfg.AllowedTools[1], "Read")
	}

	if len(cfg.DisallowedTools) != 1 {
		t.Fatalf("DisallowedTools length = %d, want 1", len(cfg.DisallowedTools))
	}
	if cfg.DisallowedTools[0] != "Write" {
		t.Errorf("DisallowedTools[0] = %q, want %q", cfg.DisallowedTools[0], "Write")
	}
}
