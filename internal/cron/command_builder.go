package cron

import (
	"encoding/json"
	"log"

	"github.com/mattw/ai-lab/internal/claude"
)

// Job represents a scheduled cron job stored in the database.
type Job struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Schedule        string  `json:"schedule"`
	Enabled         bool    `json:"enabled"`
	Prompt          string  `json:"prompt"`
	Model           string  `json:"model"`
	WorkingDir      string  `json:"working_dir"`
	AllowedTools    string  `json:"allowed_tools"`
	DisallowedTools string  `json:"disallowed_tools"`
	MaxBudget       float64 `json:"max_budget_usd"`
	Timeout         int     `json:"timeout_seconds"`
	RetryMax        int     `json:"retry_max"`
	RetryDelay      int     `json:"retry_delay_s"`
	OnFailure       string  `json:"on_failure"`
	Tags            string  `json:"tags"`
}

// BuildOneshotConfig converts a Job into a claude.OneshotConfig suitable for
// a one-shot CLI invocation.
func BuildOneshotConfig(job *Job, soulMDPath string) claude.OneshotConfig {
	cfg := claude.OneshotConfig{
		Prompt:     job.Prompt,
		Model:      job.Model,
		WorkingDir: job.WorkingDir,
		SoulMDPath: soulMDPath,
		MaxBudget:  job.MaxBudget,
		Timeout:    job.Timeout,
	}

	if job.AllowedTools != "" {
		var tools []string
		if err := json.Unmarshal([]byte(job.AllowedTools), &tools); err != nil {
			log.Printf("cron: failed to parse allowed_tools for job %s: %v", job.ID, err)
		} else {
			cfg.AllowedTools = tools
		}
	}

	if job.DisallowedTools != "" {
		var tools []string
		if err := json.Unmarshal([]byte(job.DisallowedTools), &tools); err != nil {
			log.Printf("cron: failed to parse disallowed_tools for job %s: %v", job.ID, err)
		} else {
			cfg.DisallowedTools = tools
		}
	}

	return cfg
}
