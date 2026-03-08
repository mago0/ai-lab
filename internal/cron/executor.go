package cron

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mattw/ai-lab/internal/claude"
	"github.com/mattw/ai-lab/internal/eventbus"
)

// Executor handles running cron jobs, managing retries, logging, and event publishing.
type Executor struct {
	db         *sql.DB
	bus        *eventbus.EventBus
	soulMDPath string
	cronLogDir string
	alertFn    func(string)
}

// NewExecutor creates a new Executor.
func NewExecutor(db *sql.DB, bus *eventbus.EventBus, soulMDPath, cronLogDir string) *Executor {
	return &Executor{
		db:         db,
		bus:        bus,
		soulMDPath: soulMDPath,
		cronLogDir: cronLogDir,
	}
}

// SetAlertFunc sets the function called when a job fails after all retries.
func (e *Executor) SetAlertFunc(fn func(string)) {
	e.alertFn = fn
}

// Run executes a job with retry logic. It creates run records, publishes events,
// and sends alerts on final failure.
func (e *Executor) Run(ctx context.Context, job *Job) {
	maxAttempts := job.RetryMax + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		runID, err := e.createRun(job.ID, attempt)
		if err != nil {
			log.Printf("cron: failed to create run record for job %s: %v", job.ID, err)
			return
		}

		e.bus.Publish(eventbus.Event{
			Source:  "cron",
			Type:    "job_started",
			Summary: fmt.Sprintf("Job %q attempt %d/%d started", job.Name, attempt, maxAttempts),
			Metadata: map[string]any{
				"job_id":  job.ID,
				"run_id":  runID,
				"attempt": attempt,
			},
		})

		result, execErr := e.executeOnce(ctx, job, runID)
		if execErr == nil && result != nil && result.ExitCode == 0 {
			e.completeRun(runID, "success", result)
			e.bus.Publish(eventbus.Event{
				Source:  "cron",
				Type:    "job_completed",
				Summary: fmt.Sprintf("Job %q completed successfully", job.Name),
				Metadata: map[string]any{
					"job_id":   job.ID,
					"run_id":   runID,
					"cost_usd": result.CostUSD,
				},
			})
			return
		}

		// Determine error message for logging.
		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
			lastErr = execErr
		} else if result != nil {
			errMsg = fmt.Sprintf("exit code %d", result.ExitCode)
			lastErr = fmt.Errorf("exit code %d", result.ExitCode)
			e.completeRun(runID, "failed", result)
		} else {
			errMsg = "nil result"
			lastErr = fmt.Errorf("nil result")
			e.updateRunStatus(runID, "failed")
		}

		e.bus.Publish(eventbus.Event{
			Source:  "cron",
			Type:    "job_failed",
			Summary: fmt.Sprintf("Job %q attempt %d/%d failed: %s", job.Name, attempt, maxAttempts, errMsg),
			Metadata: map[string]any{
				"job_id":  job.ID,
				"run_id":  runID,
				"attempt": attempt,
				"error":   errMsg,
			},
		})

		// Wait before retrying, unless this was the last attempt.
		if attempt < maxAttempts {
			delay := time.Duration(job.RetryDelay) * time.Second
			if delay <= 0 {
				delay = 60 * time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}

	// All attempts failed - send alert if configured.
	if e.alertFn != nil && job.OnFailure == "alert" {
		e.alertFn(fmt.Sprintf("Cron job %q failed after %d attempts: %v", job.Name, maxAttempts, lastErr))
	}
}

// executeOnce builds the OneshotConfig, creates a log file, and invokes claude.RunOneshot.
func (e *Executor) executeOnce(ctx context.Context, job *Job, runID string) (*claude.OneshotResult, error) {
	cfg := BuildOneshotConfig(job, e.soulMDPath)

	// Apply job-level timeout if set.
	if job.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(job.Timeout)*time.Second)
		defer cancel()
	}

	// Create log file for this run.
	var logFile *os.File
	if e.cronLogDir != "" {
		if err := os.MkdirAll(e.cronLogDir, 0o755); err != nil {
			log.Printf("cron: failed to create log dir %s: %v", e.cronLogDir, err)
		} else {
			logPath := filepath.Join(e.cronLogDir, fmt.Sprintf("%s_%s.jsonl", job.ID, runID))
			f, err := os.Create(logPath)
			if err != nil {
				log.Printf("cron: failed to create log file %s: %v", logPath, err)
			} else {
				logFile = f
				defer logFile.Close()
				e.updateRunLogPath(runID, logPath)
			}
		}
	}

	result, err := claude.RunOneshot(ctx, cfg, logFile)
	if err != nil {
		e.updateRunStatus(runID, "failed")
		return nil, err
	}

	return result, nil
}

// createRun inserts a new cron_runs record and returns the generated ID.
func (e *Executor) createRun(jobID string, attempt int) (string, error) {
	var id string
	err := e.db.QueryRow(
		`INSERT INTO cron_runs (job_id, status, attempt, started_at) VALUES (?, 'running', ?, CURRENT_TIMESTAMP) RETURNING id`,
		jobID, attempt,
	).Scan(&id)
	return id, err
}

// updateRunStatus updates the status of a run.
func (e *Executor) updateRunStatus(runID, status string) {
	_, err := e.db.Exec(
		`UPDATE cron_runs SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, runID,
	)
	if err != nil {
		log.Printf("cron: failed to update run %s status: %v", runID, err)
	}
}

// updateRunLogPath sets the stream_log_path for a run.
func (e *Executor) updateRunLogPath(runID, logPath string) {
	_, err := e.db.Exec(
		`UPDATE cron_runs SET stream_log_path = ? WHERE id = ?`,
		logPath, runID,
	)
	if err != nil {
		log.Printf("cron: failed to update run %s log path: %v", runID, err)
	}
}

// completeRun updates a run with the final result data.
func (e *Executor) completeRun(runID, status string, result *claude.OneshotResult) {
	_, err := e.db.Exec(
		`UPDATE cron_runs SET status = ?, session_id = ?, exit_code = ?, output_text = ?, error_output = ?, cost_usd = ?, duration_ms = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, result.SessionID, result.ExitCode, result.ResultText, result.ErrorOutput, result.CostUSD, result.DurationMS, runID,
	)
	if err != nil {
		log.Printf("cron: failed to complete run %s: %v", runID, err)
	}
}
