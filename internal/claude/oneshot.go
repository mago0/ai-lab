package claude

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// OneshotConfig holds configuration for a one-shot Claude Code execution,
// typically used for cron jobs or automated tasks.
type OneshotConfig struct {
	Prompt           string
	Model            string
	WorkingDir       string
	SoulMDPath       string
	MaxBudget        float64
	Timeout          int
	AllowedTools     []string
	DisallowedTools  []string
}

// Args builds the CLI arguments for a one-shot claude invocation.
func (c *OneshotConfig) Args() []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
	}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	if c.SoulMDPath != "" {
		args = append(args, "--append-system-prompt-file", c.SoulMDPath)
	}
	if c.MaxBudget > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", c.MaxBudget))
	}
	if len(c.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(c.AllowedTools, ","))
	}
	if len(c.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(c.DisallowedTools, ","))
	}
	// Prompt is always the last argument.
	args = append(args, c.Prompt)
	return args
}

// OneshotResult holds the outcome of a one-shot execution.
type OneshotResult struct {
	SessionID  string
	ResultText string
	ErrorOutput string
	ExitCode   int
	CostUSD    float64
	DurationMS int
	Events     []StreamEvent
	RawOutput  []byte
}

// RunOneshot executes a one-shot Claude CLI invocation and returns the parsed result.
// If logWriter is non-nil, stdout is tee'd to it in real time.
func RunOneshot(ctx context.Context, cfg OneshotConfig, logWriter io.Writer) (*OneshotResult, error) {
	args := cfg.Args()
	cmd := exec.CommandContext(ctx, "claude", args...)

	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	// Strip CLAUDECODE env var from subprocess environment.
	cmd.Env = stripEnvVar(os.Environ(), "CLAUDECODE")

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	if logWriter != nil {
		cmd.Stdout = io.MultiWriter(&stdoutBuf, logWriter)
	} else {
		cmd.Stdout = &stdoutBuf
	}
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("run claude: %w", err)
		}
	}

	rawOutput := stdoutBuf.Bytes()

	// Parse all events from the captured output.
	events, parseErr := ParseAllEvents(bytes.NewReader(rawOutput))
	if parseErr != nil {
		return nil, fmt.Errorf("parse events: %w", parseErr)
	}

	result := &OneshotResult{
		ErrorOutput: stderrBuf.String(),
		ExitCode:    exitCode,
		Events:      events,
		RawOutput:   rawOutput,
	}

	// Extract session_id, cost, and result text from events.
	for _, ev := range events {
		switch ev.Type {
		case "system":
			if ev.Subtype == "init" {
				sysEv, sErr := ParseSystemEvent(ev.Raw)
				if sErr == nil {
					result.SessionID = sysEv.SessionID
				}
			}
		case "result":
			resEv, rErr := ParseResultEvent(ev.Raw)
			if rErr == nil {
				result.CostUSD = resEv.TotalCostUSD
				result.DurationMS = resEv.DurationMS
				result.ResultText = resEv.Result
			}
		}
	}

	return result, nil
}
