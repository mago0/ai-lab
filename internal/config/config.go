package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DiscordBotToken string
	DiscordUserID   string
	DashboardPort   int
	DashboardHost   string
	ClaudeModel     string
	ClaudeCronModel string
	SoulMDPath      string
	DBPath          string
	CronLogDir      string
}

func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("DASHBOARD_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid DASHBOARD_PORT: %w", err)
	}

	cfg := &Config{
		DiscordBotToken: os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordUserID:   os.Getenv("DISCORD_USER_ID"),
		DashboardPort:   port,
		DashboardHost:   getEnv("DASHBOARD_HOST", "0.0.0.0"),
		ClaudeModel:     getEnv("CLAUDE_MODEL", "opus"),
		ClaudeCronModel: getEnv("CLAUDE_CRON_MODEL", "sonnet"),
		SoulMDPath:      getEnv("SOUL_MD_PATH", "./SOUL.md"),
		DBPath:          getEnv("DB_PATH", "./data/ai-lab.db"),
		CronLogDir:      getEnv("CRON_LOG_DIR", "./data/cron-logs"),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
