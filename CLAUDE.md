# ai-lab

Personal AI assistant built on Claude Code CLI.

## Build & Run

- `make build` - Build binary
- `make dev` - Run with `go run`
- `make test` - Run all tests
- `make test-v` - Run tests verbose

## Architecture

Go orchestration layer wrapping Claude Code CLI:
- `internal/config/` - Environment config loading
- `internal/db/` - SQLite with embedded migrations
- `internal/claude/` - Claude Code CLI stream-json integration
- `internal/discord/` - Discord DM bot bridge
- `internal/cron/` - Cron scheduler with job management
- `internal/dashboard/` - HTMX dashboard with SSE
- `internal/eventbus/` - Internal pub/sub for real-time events

## Tech Stack

Go, SQLite (modernc.org/sqlite), discordgo, robfig/cron/v3, chi, HTMX + Tailwind CDN

## Key Patterns

- Claude Code invoked via CLI subprocess, NOT the API directly
- Two modes: long-running stream-json (Discord), one-shot print (cron)
- Must unset CLAUDECODE env var when spawning subprocesses
- SOUL.md injected via --append-system-prompt-file for personality
- Never use em dashes or en dashes - only hyphens
