package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/mattw/ai-lab/internal/claude"
	"github.com/mattw/ai-lab/internal/config"
	"github.com/mattw/ai-lab/internal/cron"
	"github.com/mattw/ai-lab/internal/dashboard"
	"github.com/mattw/ai-lab/internal/db"
	"github.com/mattw/ai-lab/internal/discord"
	"github.com/mattw/ai-lab/internal/eventbus"
)

//go:embed all:web/*
var webFS embed.FS

func main() {
	_ = godotenv.Load()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	fmt.Fprintf(os.Stderr, "ai-lab starting on %s:%d\n", cfg.DashboardHost, cfg.DashboardPort)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()
	log.Printf("database ready at %s", cfg.DBPath)

	bus := eventbus.New()

	// Claude session for Discord
	session := claude.NewSessionManager(claude.SessionConfig{
		Model:      cfg.ClaudeModel,
		SoulMDPath: cfg.SoulMDPath,
	})

	// Discord bot
	var bot *discord.Bot
	if cfg.DiscordBotToken != "" && cfg.DiscordUserID != "" {
		bot, err = discord.NewBot(cfg.DiscordBotToken, cfg.DiscordUserID)
		if err != nil {
			log.Fatalf("discord: %v", err)
		}

		bridge := discord.NewBridge(bot, session, database, bus)

		if err := session.Start(); err != nil {
			log.Fatalf("claude session: %v", err)
		}

		bridge.Start()

		if err := bot.Start(); err != nil {
			log.Fatalf("discord start: %v", err)
		}
		defer bot.Stop()
		log.Printf("discord bot ready")
	} else {
		log.Printf("discord bot disabled (no token/user configured)")
	}

	// Cron scheduler
	executor := cron.NewExecutor(database, bus, cfg.SoulMDPath, cfg.CronLogDir)
	if bot != nil {
		executor.SetAlertFunc(func(msg string) {
			if err := bot.SendDM(msg); err != nil {
				log.Printf("alert error: %v", err)
			}
		})
	}

	scheduler := cron.NewScheduler(database, executor, 3)
	if err := scheduler.LoadJobs(); err != nil {
		log.Printf("warning: load cron jobs: %v", err)
	}
	scheduler.Start()

	// Dashboard
	srv, err := dashboard.NewServer(database, bus, scheduler, cfg.SoulMDPath, webFS)
	if err != nil {
		log.Fatalf("dashboard: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.DashboardHost, cfg.DashboardPort)
	httpServer := &http.Server{Addr: addr, Handler: srv.Handler()}

	go func() {
		log.Printf("dashboard listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	bus.Publish(eventbus.Event{
		Source:  "system",
		Type:    "startup",
		Summary: "ai-lab started",
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("shutting down...")
	scheduler.Stop()
	httpServer.Shutdown(context.Background())
	session.Stop()
}
