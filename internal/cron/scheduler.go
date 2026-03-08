package cron

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// Scheduler manages cron job scheduling and execution with concurrency limits.
type Scheduler struct {
	cron     *cron.Cron
	db       *sql.DB
	executor *Executor
	sem      chan struct{}
	mu       sync.Mutex
	entries  map[string]cron.EntryID
}

// NewScheduler creates a new Scheduler with the given database, executor, and
// maximum concurrency limit. If maxConcurrent is 0 or negative, it defaults to 3.
func NewScheduler(db *sql.DB, executor *Executor, maxConcurrent int) *Scheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		db:       db,
		executor: executor,
		sem:      make(chan struct{}, maxConcurrent),
		entries:  make(map[string]cron.EntryID),
	}
}

// LoadJobs queries all enabled cron jobs from the database and schedules them.
func (s *Scheduler) LoadJobs() error {
	rows, err := s.db.Query(
		`SELECT id, name, description, schedule, enabled, prompt, model, working_dir,
		        allowed_tools, disallowed_tools, max_budget_usd, timeout_seconds,
		        retry_max, retry_delay_s, on_failure, tags
		 FROM cron_jobs WHERE enabled = 1`,
	)
	if err != nil {
		return fmt.Errorf("query cron jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var job Job
		var description, allowedTools, disallowedTools, onFailure, tags sql.NullString
		var model sql.NullString
		var maxBudget sql.NullFloat64
		var timeout, retryMax, retryDelay sql.NullInt64

		err := rows.Scan(
			&job.ID, &job.Name, &description, &job.Schedule, &job.Enabled,
			&job.Prompt, &model, &job.WorkingDir,
			&allowedTools, &disallowedTools, &maxBudget, &timeout,
			&retryMax, &retryDelay, &onFailure, &tags,
		)
		if err != nil {
			log.Printf("cron: failed to scan job row: %v", err)
			continue
		}

		job.Description = description.String
		job.Model = model.String
		job.AllowedTools = allowedTools.String
		job.DisallowedTools = disallowedTools.String
		job.MaxBudget = maxBudget.Float64
		job.Timeout = int(timeout.Int64)
		job.RetryMax = int(retryMax.Int64)
		job.RetryDelay = int(retryDelay.Int64)
		job.OnFailure = onFailure.String
		job.Tags = tags.String

		if err := s.AddJob(&job); err != nil {
			log.Printf("cron: failed to schedule job %q: %v", job.Name, err)
		}
	}

	return rows.Err()
}

// AddJob schedules a job. If the job is already scheduled, the existing entry
// is removed before adding the new one.
func (s *Scheduler) AddJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing entry if present.
	if entryID, ok := s.entries[job.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, job.ID)
	}

	// Capture job for the closure.
	j := *job

	entryID, err := s.cron.AddFunc(j.Schedule, func() {
		s.sem <- struct{}{} // Acquire semaphore.
		defer func() { <-s.sem }()

		s.executor.Run(context.Background(), &j)
	})
	if err != nil {
		return fmt.Errorf("add cron entry for %q: %w", job.Name, err)
	}

	s.entries[job.ID] = entryID
	log.Printf("cron: scheduled job %q (%s) with schedule %q", job.Name, job.ID, job.Schedule)
	return nil
}

// RemoveJob removes a scheduled job by its ID.
func (s *Scheduler) RemoveJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[jobID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, jobID)
		log.Printf("cron: removed job %s", jobID)
	}
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("cron: scheduler started")
}

// Stop halts the cron scheduler and waits for running jobs to finish.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("cron: scheduler stopped")
}

// RunJobNow loads a job by ID from the database and runs it immediately in a
// goroutine, respecting the concurrency semaphore.
func (s *Scheduler) RunJobNow(ctx context.Context, jobID string) error {
	job, err := s.loadJob(jobID)
	if err != nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}

	go func() {
		s.sem <- struct{}{} // Acquire semaphore.
		defer func() { <-s.sem }()

		s.executor.Run(ctx, job)
	}()

	return nil
}

// loadJob reads a single job from the database by ID.
func (s *Scheduler) loadJob(jobID string) (*Job, error) {
	var job Job
	var description, allowedTools, disallowedTools, onFailure, tags sql.NullString
	var model sql.NullString
	var maxBudget sql.NullFloat64
	var timeout, retryMax, retryDelay sql.NullInt64

	err := s.db.QueryRow(
		`SELECT id, name, description, schedule, enabled, prompt, model, working_dir,
		        allowed_tools, disallowed_tools, max_budget_usd, timeout_seconds,
		        retry_max, retry_delay_s, on_failure, tags
		 FROM cron_jobs WHERE id = ?`,
		jobID,
	).Scan(
		&job.ID, &job.Name, &description, &job.Schedule, &job.Enabled,
		&job.Prompt, &model, &job.WorkingDir,
		&allowedTools, &disallowedTools, &maxBudget, &timeout,
		&retryMax, &retryDelay, &onFailure, &tags,
	)
	if err != nil {
		return nil, err
	}

	job.Description = description.String
	job.Model = model.String
	job.AllowedTools = allowedTools.String
	job.DisallowedTools = disallowedTools.String
	job.MaxBudget = maxBudget.Float64
	job.Timeout = int(timeout.Int64)
	job.RetryMax = int(retryMax.Int64)
	job.RetryDelay = int(retryDelay.Int64)
	job.OnFailure = onFailure.String
	job.Tags = tags.String

	return &job, nil
}
