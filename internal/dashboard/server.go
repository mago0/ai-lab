package dashboard

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mattw/ai-lab/internal/cron"
	"github.com/mattw/ai-lab/internal/eventbus"
)

// Server is the dashboard HTTP server.
type Server struct {
	router    *chi.Mux
	db        *sql.DB
	bus       *eventbus.EventBus
	scheduler *cron.Scheduler
	soulPath  string
	templates *template.Template
}

// NewServer creates a new dashboard server.
func NewServer(db *sql.DB, bus *eventbus.EventBus, scheduler *cron.Scheduler, soulPath string, templateFS fs.FS) (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{
		router:    chi.NewRouter(),
		db:        db,
		bus:       bus,
		scheduler: scheduler,
		soulPath:  soulPath,
		templates: tmpl,
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	s.registerRoutes()
	return s, nil
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}
