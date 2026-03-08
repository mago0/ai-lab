package dashboard

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

type pageData struct {
	Title   string
	Content any
}

func (s *Server) render(w http.ResponseWriter, page string, data pageData) {
	tmpl, ok := s.templates[page]
	if !ok {
		log.Printf("template not found: %s", page)
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("template error (%s): %v", page, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

type homeStats struct {
	TotalMessages  int
	TotalCronJobs  int
	TotalCronRuns  int
	RecentActivity []activityRow
}

type activityRow struct {
	Source    string
	EventType string
	Summary  string
	CreatedAt string
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	var st homeStats
	s.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&st.TotalMessages)
	s.db.QueryRow("SELECT COUNT(*) FROM cron_jobs").Scan(&st.TotalCronJobs)
	s.db.QueryRow("SELECT COUNT(*) FROM cron_runs").Scan(&st.TotalCronRuns)

	rows, _ := s.db.Query(
		"SELECT source, event_type, COALESCE(summary,''), created_at FROM activity_log ORDER BY created_at DESC LIMIT 20",
	)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var a activityRow
			rows.Scan(&a.Source, &a.EventType, &a.Summary, &a.CreatedAt)
			st.RecentActivity = append(st.RecentActivity, a)
		}
	}

	s.render(w, "home.html", pageData{Title: "Dashboard", Content: st})
}

type messageRow struct {
	ID        int
	SessionID string
	Role      string
	Content   string
	Model     string
	CreatedAt string
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		"SELECT id, COALESCE(session_id,''), role, content, COALESCE(model,''), created_at FROM messages ORDER BY created_at DESC LIMIT 100",
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []messageRow
	for rows.Next() {
		var m messageRow
		rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Model, &m.CreatedAt)
		messages = append(messages, m)
	}

	s.render(w, "messages.html", pageData{Title: "Messages", Content: messages})
}

type cronListRow struct {
	ID       string
	Name     string
	Schedule string
	Enabled  bool
	Model    string
	LastRun  string
}

func (s *Server) handleCrons(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT j.id, j.name, j.schedule, j.enabled, j.model,
		 COALESCE((SELECT finished_at FROM cron_runs WHERE job_id = j.id ORDER BY created_at DESC LIMIT 1), '')
		 FROM cron_jobs j ORDER BY j.name`,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var jobs []cronListRow
	for rows.Next() {
		var j cronListRow
		rows.Scan(&j.ID, &j.Name, &j.Schedule, &j.Enabled, &j.Model, &j.LastRun)
		jobs = append(jobs, j)
	}

	s.render(w, "crons.html", pageData{Title: "Cron Jobs", Content: jobs})
}

type cronJobDetail struct {
	ID          string
	Name        string
	Description string
	Schedule    string
	Enabled     bool
	Prompt      string
	Model       string
	WorkingDir  string
	MaxBudget   float64
	Timeout     int
	RetryMax    int
	OnFailure   string
}

type cronRunRow struct {
	ID         string
	Status     string
	Attempt    int
	ExitCode   int
	CostUSD    float64
	DurationMS int
	StartedAt  string
	FinishedAt string
}

type cronDetailData struct {
	Job  cronJobDetail
	Runs []cronRunRow
}

func (s *Server) handleCronDetail(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	var d cronDetailData
	var desc sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, description, schedule, enabled, prompt, model, working_dir,
		 max_budget_usd, timeout_seconds, retry_max, on_failure
		 FROM cron_jobs WHERE id = ?`, jobID,
	).Scan(
		&d.Job.ID, &d.Job.Name, &desc, &d.Job.Schedule, &d.Job.Enabled,
		&d.Job.Prompt, &d.Job.Model, &d.Job.WorkingDir,
		&d.Job.MaxBudget, &d.Job.Timeout, &d.Job.RetryMax, &d.Job.OnFailure,
	)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	d.Job.Description = desc.String

	rows, _ := s.db.Query(
		`SELECT id, status, attempt, COALESCE(exit_code, -1), COALESCE(cost_usd, 0),
		 COALESCE(duration_ms, 0), COALESCE(started_at, ''), COALESCE(finished_at, '')
		 FROM cron_runs WHERE job_id = ? ORDER BY created_at DESC LIMIT 20`, jobID,
	)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var run cronRunRow
			rows.Scan(&run.ID, &run.Status, &run.Attempt, &run.ExitCode, &run.CostUSD,
				&run.DurationMS, &run.StartedAt, &run.FinishedAt)
			d.Runs = append(d.Runs, run)
		}
	}

	s.render(w, "cron_detail.html", pageData{Title: d.Job.Name, Content: d})
}

func (s *Server) handleCronForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "cron_form.html", pageData{Title: "New Cron Job"})
}

func (s *Server) handleCronCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := s.db.Exec(
		`INSERT INTO cron_jobs (name, description, schedule, prompt, model, working_dir, max_budget_usd, timeout_seconds)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.FormValue("name"), r.FormValue("description"), r.FormValue("schedule"),
		r.FormValue("prompt"), r.FormValue("model"), r.FormValue("working_dir"),
		r.FormValue("max_budget_usd"), r.FormValue("timeout_seconds"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.scheduler.LoadJobs()
	http.Redirect(w, r, "/crons", http.StatusSeeOther)
}

func (s *Server) handleCronEditForm(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	var job cronJobDetail
	var desc sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, description, schedule, enabled, prompt, model, working_dir,
		 max_budget_usd, timeout_seconds, retry_max, on_failure
		 FROM cron_jobs WHERE id = ?`, jobID,
	).Scan(
		&job.ID, &job.Name, &desc, &job.Schedule, &job.Enabled,
		&job.Prompt, &job.Model, &job.WorkingDir,
		&job.MaxBudget, &job.Timeout, &job.RetryMax, &job.OnFailure,
	)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	job.Description = desc.String
	s.render(w, "cron_edit.html", pageData{Title: "Edit " + job.Name, Content: job})
}

func (s *Server) handleCronUpdate(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := s.db.Exec(
		`UPDATE cron_jobs SET name=?, description=?, schedule=?, prompt=?, model=?,
		 working_dir=?, max_budget_usd=?, timeout_seconds=?, updated_at=?
		 WHERE id=?`,
		r.FormValue("name"), r.FormValue("description"), r.FormValue("schedule"),
		r.FormValue("prompt"), r.FormValue("model"), r.FormValue("working_dir"),
		r.FormValue("max_budget_usd"), r.FormValue("timeout_seconds"),
		time.Now().UTC(), jobID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.scheduler.LoadJobs()
	http.Redirect(w, r, fmt.Sprintf("/crons/%s", jobID), http.StatusSeeOther)
}

func (s *Server) handleCronDelete(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	_, _ = s.db.Exec("DELETE FROM cron_runs WHERE job_id = ?", jobID)
	_, err := s.db.Exec("DELETE FROM cron_jobs WHERE id = ?", jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.scheduler.LoadJobs()
	w.Header().Set("HX-Redirect", "/crons")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleCronToggle(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	_, err := s.db.Exec(
		`UPDATE cron_jobs SET enabled = NOT enabled, updated_at = ? WHERE id = ?`,
		time.Now().UTC(), jobID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.scheduler.LoadJobs()
	http.Redirect(w, r, "/crons", http.StatusSeeOther)
}

func (s *Server) handleCronRunNow(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if err := s.scheduler.RunJobNow(r.Context(), jobID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/crons/%s", jobID))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSoul(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile(s.soulPath)
	if err != nil {
		content = []byte("(SOUL.md not found)")
	}
	s.render(w, "soul.html", pageData{Title: "SOUL.md", Content: string(content)})
}

func (s *Server) handleSoulSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if err := os.WriteFile(s.soulPath, []byte(content), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/soul", http.StatusSeeOther)
}
