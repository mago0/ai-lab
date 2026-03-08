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
	templates map[string]*template.Template
}

// NewServer creates a new dashboard server.
func NewServer(db *sql.DB, bus *eventbus.EventBus, scheduler *cron.Scheduler, soulPath string, templateFS fs.FS) (*Server, error) {
	templates, err := parseTemplates(templateFS)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{
		router:    chi.NewRouter(),
		db:        db,
		bus:       bus,
		scheduler: scheduler,
		soulPath:  soulPath,
		templates: templates,
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	s.registerRoutes()
	return s, nil
}

// parseTemplates creates a named template for each page by combining layout + page.
func parseTemplates(fsys fs.FS) (map[string]*template.Template, error) {
	layoutBytes, err := fs.ReadFile(fsys, "web/templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("read layout: %w", err)
	}

	pages := []string{"home.html", "messages.html", "crons.html", "cron_detail.html", "cron_form.html", "cron_edit.html", "soul.html"}
	templates := make(map[string]*template.Template, len(pages))

	for _, page := range pages {
		pageBytes, err := fs.ReadFile(fsys, "web/templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", page, err)
		}

		tmpl, err := template.New("layout.html").Parse(string(layoutBytes))
		if err != nil {
			return nil, fmt.Errorf("parse layout for %s: %w", page, err)
		}

		if _, err := tmpl.Parse(string(pageBytes)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}

		templates[page] = tmpl
	}

	return templates, nil
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}
