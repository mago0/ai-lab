# ai-lab Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a personal AI assistant that wraps Claude Code CLI with Go orchestration for Discord DMs, cron scheduling, and an HTMX monitoring dashboard.

**Architecture:** Thin Go process wrapping Claude Code CLI in two modes - long-running stream-json sessions (Discord) and one-shot print executions (cron). SQLite stores messages, jobs, runs, and activity. HTMX dashboard with SSE provides real-time observability.

**Tech Stack:** Go 1.26, SQLite (modernc.org/sqlite), discordgo, robfig/cron/v3, chi router, HTMX + Tailwind CDN

---

## Phase 1: Foundation

### Task 1: Go Project Setup

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `.env.example`
- Create: `main.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/mattw/ai-lab`

**Step 2: Create .gitignore**

```
.env
data/
*.db
ai-lab
```

**Step 3: Create .env.example**

```
DISCORD_BOT_TOKEN=
DISCORD_USER_ID=
DASHBOARD_PORT=8080
DASHBOARD_HOST=0.0.0.0
CLAUDE_MODEL=opus
CLAUDE_CRON_MODEL=sonnet
SOUL_MD_PATH=./SOUL.md
DB_PATH=./data/ai-lab.db
CRON_LOG_DIR=./data/cron-logs
```

**Step 4: Create Makefile**

```makefile
.PHONY: build run dev clean migrate

build:
	go build -o ai-lab .

run: build
	./ai-lab

dev:
	go run .

clean:
	rm -f ai-lab

test:
	go test ./...

test-v:
	go test -v ./...
```

**Step 5: Create minimal main.go**

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Fprintf(os.Stderr, "ai-lab starting...\n")
}
```

**Step 6: Add dependencies**

Run: `go get github.com/joho/godotenv`

**Step 7: Verify build**

Run: `go build -o ai-lab . && echo "BUILD OK"`
Expected: BUILD OK

**Step 8: Commit**

```bash
git add go.mod go.sum main.go Makefile .gitignore .env.example
git commit -m "feat: initialize Go project with module, Makefile, and entry point"
```

---

### Task 2: Config Loading

**Files:**
- Create: `internal/config/config.go`
- Modify: `main.go`

**Step 1: Write config.go**

```go
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
```

**Step 2: Update main.go to load config**

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/mattw/ai-lab/internal/config"
)

func main() {
	_ = godotenv.Load()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	fmt.Fprintf(os.Stderr, "ai-lab starting on %s:%d\n", cfg.DashboardHost, cfg.DashboardPort)
}
```

**Step 3: Verify build**

Run: `go build -o ai-lab . && echo "BUILD OK"`
Expected: BUILD OK

**Step 4: Commit**

```bash
git add internal/config/config.go main.go
git commit -m "feat: add config loading from environment variables"
```

---

### Task 3: SQLite Database + Migrations

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/migrations/001_initial.sql`
- Modify: `main.go`

**Step 1: Add SQLite dependency**

Run: `go get modernc.org/sqlite && go get database/sql`

**Step 2: Create migration SQL**

```sql
-- 001_initial.sql

CREATE TABLE IF NOT EXISTS messages (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     TEXT,
    role           TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content        TEXT NOT NULL,
    discord_msg_id TEXT,
    cost_usd       REAL,
    model          TEXT,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cron_jobs (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    name            TEXT NOT NULL UNIQUE,
    description     TEXT,
    schedule        TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    prompt          TEXT NOT NULL,
    model           TEXT DEFAULT 'sonnet',
    working_dir     TEXT NOT NULL,
    allowed_tools   TEXT,
    disallowed_tools TEXT,
    max_budget_usd  REAL DEFAULT 1.00,
    timeout_seconds INTEGER DEFAULT 600,
    retry_max       INTEGER DEFAULT 0,
    retry_delay_s   INTEGER DEFAULT 60,
    on_failure      TEXT DEFAULT 'alert',
    tags            TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cron_runs (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    job_id          TEXT NOT NULL REFERENCES cron_jobs(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    attempt         INTEGER NOT NULL DEFAULT 1,
    session_id      TEXT,
    pid             INTEGER,
    exit_code       INTEGER,
    output_text     TEXT,
    error_output    TEXT,
    cost_usd        REAL,
    duration_ms     INTEGER,
    stream_log_path TEXT,
    started_at      DATETIME,
    finished_at     DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    source      TEXT NOT NULL,
    cron_job_id TEXT REFERENCES cron_jobs(id),
    status      TEXT DEFAULT 'active',
    model       TEXT,
    total_cost  REAL DEFAULT 0,
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    ended_at    DATETIME
);

CREATE TABLE IF NOT EXISTS activity_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source      TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    summary     TEXT,
    metadata    TEXT,
    session_id  TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_cron_runs_job ON cron_runs(job_id);
CREATE INDEX IF NOT EXISTS idx_cron_runs_status ON cron_runs(status);
CREATE INDEX IF NOT EXISTS idx_activity_log_source ON activity_log(source);
CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at);
```

**Step 3: Create db.go**

```go
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		data, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}
	}

	return nil
}
```

**Step 4: Update main.go to open database**

Add DB opening after config load:

```go
database, err := db.Open(cfg.DBPath)
if err != nil {
    log.Fatalf("db: %v", err)
}
defer database.Close()

log.Printf("database ready at %s", cfg.DBPath)
```

**Step 5: Verify build + run**

Run: `go build -o ai-lab . && ./ai-lab && echo "OK"`
Expected: logs showing database ready

**Step 6: Commit**

```bash
git add internal/db/ main.go go.mod go.sum
git commit -m "feat: add SQLite database with embedded migrations"
```

---

### Task 4: CLAUDE.md + SOUL.md

**Files:**
- Create: `CLAUDE.md`
- Create: `SOUL.md`

**Step 1: Create CLAUDE.md**

```markdown
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
```

**Step 2: Create SOUL.md**

```markdown
# SOUL

You are Matt's personal AI assistant. You run autonomously on a dedicated server, handling tasks through Discord messages and scheduled cron jobs.

## Personality

- Direct and concise - no filler or pleasantries unless the context calls for it
- Technically skilled - you have deep knowledge of software engineering, systems, and infrastructure
- Proactive - when you notice something worth mentioning, say it
- Honest about uncertainty - say "I don't know" rather than guessing

## Communication Style

- Match the formality of the message you receive
- Use short responses for simple questions
- Use structured responses (lists, headers) for complex topics
- Never use em dashes or en dashes - only hyphens
- Skip emojis unless the conversation is casual

## Context

- You run on a Debian Forky VM via Proxmox
- You have access to the file system, git, and shell commands
- Your memory persists via claude-mem plugin across sessions
- Your cron jobs execute with full autonomy (bypassPermissions mode)
```

**Step 3: Commit**

```bash
git add CLAUDE.md SOUL.md
git commit -m "feat: add CLAUDE.md project instructions and SOUL.md personality"
```

---

## Phase 2: Claude Code Integration

### Task 5: Stream-JSON Type Definitions

**Files:**
- Create: `internal/claude/types.go`

**Step 1: Create types.go**

Based on live protocol testing, the stream-json protocol uses these message types:

```go
package claude

import "encoding/json"

// StreamEvent is a raw event from Claude Code's stream-json output.
// Each line of stdout is one JSON object with a "type" field.
type StreamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

// SystemEvent is emitted for system-level events (init, hooks).
type SystemEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`

	// init subtype fields
	CWD   string   `json:"cwd,omitempty"`
	Tools []string `json:"tools,omitempty"`
	Model string   `json:"model,omitempty"`

	// hook fields
	HookID    string `json:"hook_id,omitempty"`
	HookName  string `json:"hook_name,omitempty"`
	HookEvent string `json:"hook_event,omitempty"`
	Output    string `json:"output,omitempty"`
}

// ContentBlock represents a content block in an assistant message.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Input    any    `json:"input,omitempty"`
}

// AssistantMessage is the message payload inside an assistant event.
type AssistantMessage struct {
	Model      string         `json:"model"`
	ID         string         `json:"id"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	StopReason *string        `json:"stop_reason"`
	Usage      *Usage         `json:"usage,omitempty"`
}

// AssistantEvent is emitted when Claude produces output.
type AssistantEvent struct {
	Type             string           `json:"type"`
	Message          AssistantMessage `json:"message"`
	SessionID        string           `json:"session_id"`
	ParentToolUseID  *string          `json:"parent_tool_use_id"`
}

// ResultEvent is the final event with summary data.
type ResultEvent struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	DurationMS   int     `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	Result       string  `json:"result"`
	StopReason   string  `json:"stop_reason"`
	SessionID    string  `json:"session_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        *Usage  `json:"usage,omitempty"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// UserInput is the JSON format for sending messages via stream-json stdin.
type UserInput struct {
	Type    string      `json:"type"`
	Message UserMessage `json:"message"`
}

// UserMessage is the message payload for user input.
type UserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewUserInput creates a properly formatted user input message.
func NewUserInput(content string) UserInput {
	return UserInput{
		Type: "user",
		Message: UserMessage{
			Role:    "user",
			Content: content,
		},
	}
}
```

**Step 2: Verify build**

Run: `go build ./internal/claude/...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/claude/types.go
git commit -m "feat: add stream-json protocol type definitions for Claude Code CLI"
```

---

### Task 6: Stream-JSON Parser

**Files:**
- Create: `internal/claude/streaming.go`
- Create: `internal/claude/streaming_test.go`

**Step 1: Write streaming_test.go**

```go
package claude

import (
	"strings"
	"testing"
)

func TestParseStreamEvents(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"abc-123","cwd":"/tmp","model":"claude-sonnet-4-6"}`,
		`{"type":"assistant","message":{"model":"claude-sonnet-4-6","id":"msg_01","role":"assistant","content":[{"type":"text","text":"Hello!"}],"stop_reason":"end_turn"},"session_id":"abc-123"}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":1000,"num_turns":1,"result":"Hello!","session_id":"abc-123","total_cost_usd":0.05}`,
	}, "\n")

	reader := strings.NewReader(input)
	events, err := ParseAllEvents(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Type != "system" {
		t.Errorf("event 0: expected type system, got %s", events[0].Type)
	}
	if events[1].Type != "assistant" {
		t.Errorf("event 1: expected type assistant, got %s", events[1].Type)
	}
	if events[2].Type != "result" {
		t.Errorf("event 2: expected type result, got %s", events[2].Type)
	}
}

func TestParseInitEvent(t *testing.T) {
	line := `{"type":"system","subtype":"init","session_id":"abc-123","cwd":"/tmp","model":"claude-sonnet-4-6","tools":["Bash","Read"]}`

	evt, err := ParseSystemEvent([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evt.SessionID != "abc-123" {
		t.Errorf("expected session_id abc-123, got %s", evt.SessionID)
	}
	if evt.Subtype != "init" {
		t.Errorf("expected subtype init, got %s", evt.Subtype)
	}
	if evt.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %s", evt.Model)
	}
}

func TestParseAssistantEvent(t *testing.T) {
	line := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","id":"msg_01","role":"assistant","content":[{"type":"text","text":"4"}],"stop_reason":"end_turn"},"session_id":"abc-123"}`

	evt, err := ParseAssistantEvent([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evt.SessionID != "abc-123" {
		t.Errorf("expected session_id abc-123, got %s", evt.SessionID)
	}

	texts := evt.TextContent()
	if len(texts) != 1 || texts[0] != "4" {
		t.Errorf("expected text [4], got %v", texts)
	}
}

func TestParseResultEvent(t *testing.T) {
	line := `{"type":"result","subtype":"success","is_error":false,"duration_ms":3030,"num_turns":1,"result":"Hello!","stop_reason":"end_turn","session_id":"abc-123","total_cost_usd":0.063}`

	evt, err := ParseResultEvent([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evt.IsError {
		t.Error("expected is_error false")
	}
	if evt.Result != "Hello!" {
		t.Errorf("expected result Hello!, got %s", evt.Result)
	}
	if evt.TotalCostUSD != 0.063 {
		t.Errorf("expected cost 0.063, got %f", evt.TotalCostUSD)
	}
}

func TestStreamScanner(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"abc"}`,
		`{"type":"assistant","message":{"model":"m","id":"x","role":"assistant","content":[{"type":"text","text":"hi"}]},"session_id":"abc"}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"hi","session_id":"abc","total_cost_usd":0.01}`,
	}, "\n")

	scanner := NewStreamScanner(strings.NewReader(input))

	var types []string
	for scanner.Scan() {
		evt := scanner.Event()
		types = append(types, evt.Type)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	expected := []string{"system", "assistant", "result"}
	if len(types) != len(expected) {
		t.Fatalf("expected %d events, got %d", len(expected), len(types))
	}
	for i, typ := range types {
		if typ != expected[i] {
			t.Errorf("event %d: expected %s, got %s", i, expected[i], typ)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude/ -v`
Expected: FAIL (functions not defined)

**Step 3: Write streaming.go**

```go
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// TextContent extracts all text content blocks from an assistant event.
func (e *AssistantEvent) TextContent() []string {
	var texts []string
	for _, block := range e.Message.Content {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	return texts
}

// FullText joins all text content blocks with newlines.
func (e *AssistantEvent) FullText() string {
	texts := e.TextContent()
	if len(texts) == 0 {
		return ""
	}
	result := texts[0]
	for _, t := range texts[1:] {
		result += "\n" + t
	}
	return result
}

// ParseStreamEvent parses a single JSON line into a StreamEvent with raw bytes.
func ParseStreamEvent(data []byte) (StreamEvent, error) {
	var evt StreamEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return evt, fmt.Errorf("parse stream event: %w", err)
	}
	evt.Raw = json.RawMessage(append([]byte{}, data...))
	return evt, nil
}

// ParseSystemEvent parses a system event from raw JSON.
func ParseSystemEvent(data []byte) (*SystemEvent, error) {
	var evt SystemEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, fmt.Errorf("parse system event: %w", err)
	}
	return &evt, nil
}

// ParseAssistantEvent parses an assistant event from raw JSON.
func ParseAssistantEvent(data []byte) (*AssistantEvent, error) {
	var evt AssistantEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, fmt.Errorf("parse assistant event: %w", err)
	}
	return &evt, nil
}

// ParseResultEvent parses a result event from raw JSON.
func ParseResultEvent(data []byte) (*ResultEvent, error) {
	var evt ResultEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, fmt.Errorf("parse result event: %w", err)
	}
	return &evt, nil
}

// ParseAllEvents reads all newline-delimited JSON events from a reader.
func ParseAllEvents(r io.Reader) ([]StreamEvent, error) {
	var events []StreamEvent
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		evt, err := ParseStreamEvent(line)
		if err != nil {
			return events, err
		}
		events = append(events, evt)
	}
	return events, scanner.Err()
}

// StreamScanner provides an iterator over stream-json events.
type StreamScanner struct {
	scanner *bufio.Scanner
	event   StreamEvent
	err     error
}

// NewStreamScanner creates a scanner that reads stream-json events.
func NewStreamScanner(r io.Reader) *StreamScanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 1024*1024), 1024*1024)
	return &StreamScanner{scanner: s}
}

// Scan advances to the next event. Returns false when done or on error.
func (s *StreamScanner) Scan() bool {
	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		evt, err := ParseStreamEvent(line)
		if err != nil {
			s.err = err
			return false
		}
		s.event = evt
		return true
	}
	s.err = s.scanner.Err()
	return false
}

// Event returns the current event.
func (s *StreamScanner) Event() StreamEvent {
	return s.event
}

// Err returns any error from scanning.
func (s *StreamScanner) Err() error {
	return s.err
}
```

**Step 4: Run tests**

Run: `go test ./internal/claude/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/claude/streaming.go internal/claude/streaming_test.go
git commit -m "feat: add stream-json parser with scanner interface"
```

---

### Task 7: SessionManager (Long-Running Claude Code)

**Files:**
- Create: `internal/claude/session.go`

**Step 1: Write session.go**

```go
package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SessionConfig holds configuration for a Claude Code session.
type SessionConfig struct {
	Model         string
	WorkingDir    string
	SoulMDPath    string
	SessionID     string // for --resume
}

// SessionManager manages a long-running Claude Code subprocess.
type SessionManager struct {
	config    SessionConfig
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	sessionID string
	mu        sync.Mutex
	events    chan StreamEvent
	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewSessionManager creates a new session manager.
func NewSessionManager(cfg SessionConfig) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &SessionManager{
		config: cfg,
		events: make(chan StreamEvent, 100),
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start spawns the Claude Code subprocess.
func (sm *SessionManager) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	args := []string{
		"--input-format", "stream-json",
		"--output-format", "stream-json",
	}

	if sm.config.Model != "" {
		args = append(args, "--model", sm.config.Model)
	}

	if sm.config.SoulMDPath != "" {
		args = append(args, "--append-system-prompt-file", sm.config.SoulMDPath)
	}

	if sm.config.SessionID != "" {
		args = append(args, "--resume", sm.config.SessionID)
	}

	sm.cmd = exec.CommandContext(sm.ctx, "claude", args...)

	if sm.config.WorkingDir != "" {
		sm.cmd.Dir = sm.config.WorkingDir
	}

	// Strip CLAUDECODE env var to avoid nested session detection
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			filtered = append(filtered, e)
		}
	}
	sm.cmd.Env = filtered

	var err error
	sm.stdin, err = sm.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	sm.stdout, err = sm.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	sm.stderr, err = sm.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := sm.cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	go sm.readEvents()
	go sm.readStderr()

	return nil
}

// Send sends a user message to the Claude Code session.
func (sm *SessionManager) Send(content string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.stdin == nil {
		return fmt.Errorf("session not started")
	}

	input := NewUserInput(content)
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal input: %w", err)
	}

	data = append(data, '\n')
	if _, err := sm.stdin.Write(data); err != nil {
		return fmt.Errorf("write to stdin: %w", err)
	}

	return nil
}

// Events returns the channel of stream events from Claude.
func (sm *SessionManager) Events() <-chan StreamEvent {
	return sm.events
}

// SessionID returns the session ID (available after init event).
func (sm *SessionManager) SessionID() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessionID
}

// Done returns a channel that closes when the session ends.
func (sm *SessionManager) Done() <-chan struct{} {
	return sm.done
}

// Stop gracefully stops the session.
func (sm *SessionManager) Stop() error {
	sm.cancel()

	sm.mu.Lock()
	if sm.stdin != nil {
		sm.stdin.Close()
	}
	sm.mu.Unlock()

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		if sm.cmd != nil && sm.cmd.Process != nil {
			done <- sm.cmd.Wait()
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		if sm.cmd != nil && sm.cmd.Process != nil {
			sm.cmd.Process.Kill()
		}
		return fmt.Errorf("session kill timeout")
	}
}

func (sm *SessionManager) readEvents() {
	defer close(sm.done)
	defer close(sm.events)

	scanner := bufio.NewScanner(sm.stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		evt, err := ParseStreamEvent(line)
		if err != nil {
			log.Printf("parse event error: %v", err)
			continue
		}

		// Capture session ID from init event
		if evt.Type == "system" && evt.Subtype == "init" {
			sysEvt, err := ParseSystemEvent(line)
			if err == nil {
				sm.mu.Lock()
				sm.sessionID = sysEvt.SessionID
				sm.mu.Unlock()
			}
		}

		select {
		case sm.events <- evt:
		case <-sm.ctx.Done():
			return
		}
	}
}

func (sm *SessionManager) readStderr() {
	scanner := bufio.NewScanner(sm.stderr)
	for scanner.Scan() {
		log.Printf("[claude stderr] %s", scanner.Text())
	}
}
```

**Step 2: Verify build**

Run: `go build ./internal/claude/...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/claude/session.go
git commit -m "feat: add SessionManager for long-running Claude Code sessions"
```

---

### Task 8: One-Shot Execution

**Files:**
- Create: `internal/claude/oneshot.go`
- Create: `internal/claude/oneshot_test.go`

**Step 1: Write oneshot_test.go**

```go
package claude

import (
	"testing"
)

func TestBuildOneshotArgs(t *testing.T) {
	cfg := OneshotConfig{
		Prompt:     "hello world",
		Model:      "sonnet",
		WorkingDir: "/tmp",
		SoulMDPath: "./SOUL.md",
		MaxBudget:  1.50,
		Timeout:    600,
	}

	args := cfg.Args()

	expected := map[string]bool{
		"--print":                        true,
		"--output-format":               true,
		"stream-json":                   true,
		"--model":                       true,
		"sonnet":                        true,
		"--append-system-prompt-file":   true,
		"./SOUL.md":                     true,
		"--max-budget-usd":             true,
		"--dangerously-skip-permissions": true,
	}

	for _, arg := range args {
		delete(expected, arg)
	}

	// prompt should be last arg
	lastArg := args[len(args)-1]
	if lastArg != "hello world" {
		t.Errorf("expected last arg to be prompt, got %s", lastArg)
	}
}

func TestBuildOneshotArgsWithTools(t *testing.T) {
	cfg := OneshotConfig{
		Prompt:          "test",
		AllowedTools:    []string{"Bash", "Read"},
		DisallowedTools: []string{"Write"},
	}

	args := cfg.Args()

	hasAllowed := false
	hasDisallowed := false
	for i, arg := range args {
		if arg == "--allowedTools" && i+1 < len(args) {
			hasAllowed = true
		}
		if arg == "--disallowedTools" && i+1 < len(args) {
			hasDisallowed = true
		}
	}

	if !hasAllowed {
		t.Error("expected --allowedTools flag")
	}
	if !hasDisallowed {
		t.Error("expected --disallowedTools flag")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/claude/ -run TestBuildOneshot -v`
Expected: FAIL

**Step 3: Write oneshot.go**

```go
package claude

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

// OneshotConfig configures a one-shot Claude Code execution.
type OneshotConfig struct {
	Prompt          string
	Model           string
	WorkingDir      string
	SoulMDPath      string
	MaxBudget       float64
	Timeout         int
	AllowedTools    []string
	DisallowedTools []string
}

// Args builds the CLI argument list for one-shot execution.
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

	args = append(args, c.Prompt)
	return args
}

// OneshotResult holds the result of a one-shot execution.
type OneshotResult struct {
	SessionID    string
	ResultText   string
	ErrorOutput  string
	ExitCode     int
	CostUSD      float64
	DurationMS   int
	Events       []StreamEvent
	RawOutput    []byte
}

// RunOneshot executes a one-shot Claude Code command and captures all output.
func RunOneshot(ctx context.Context, cfg OneshotConfig, logWriter io.Writer) (*OneshotResult, error) {
	args := cfg.Args()
	cmd := exec.CommandContext(ctx, "claude", args...)

	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	// Strip CLAUDECODE env var
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = filtered

	var stdout, stderr bytes.Buffer

	if logWriter != nil {
		cmd.Stdout = io.MultiWriter(&stdout, logWriter)
	} else {
		cmd.Stdout = &stdout
	}
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &OneshotResult{
		ErrorOutput: stderr.String(),
		RawOutput:   stdout.Bytes(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	// Parse events from stdout
	events, parseErr := ParseAllEvents(bytes.NewReader(stdout.Bytes()))
	if parseErr != nil {
		log.Printf("warning: parse events: %v", parseErr)
	}
	result.Events = events

	// Extract data from events
	for _, evt := range events {
		switch evt.Type {
		case "system":
			if evt.Subtype == "init" {
				sysEvt, e := ParseSystemEvent(evt.Raw)
				if e == nil {
					result.SessionID = sysEvt.SessionID
				}
			}
		case "assistant":
			aEvt, e := ParseAssistantEvent(evt.Raw)
			if e == nil {
				text := aEvt.FullText()
				if text != "" {
					result.ResultText = text
				}
			}
		case "result":
			rEvt, e := ParseResultEvent(evt.Raw)
			if e == nil {
				result.CostUSD = rEvt.TotalCostUSD
				result.DurationMS = rEvt.DurationMS
				if rEvt.Result != "" {
					result.ResultText = rEvt.Result
				}
			}
		}
	}

	return result, err
}
```

**Step 4: Run tests**

Run: `go test ./internal/claude/ -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/claude/oneshot.go internal/claude/oneshot_test.go
git commit -m "feat: add one-shot Claude Code execution for cron jobs"
```

---

## Phase 3: Discord Bot

### Task 9: Discord Bot Setup

**Files:**
- Create: `internal/discord/bot.go`

**Step 1: Add discordgo dependency**

Run: `go get github.com/bwmarrin/discordgo`

**Step 2: Write bot.go**

```go
package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// Bot wraps a discordgo session with DM-only filtering.
type Bot struct {
	session       *discordgo.Session
	allowedUserID string
	onMessage     func(userID, content, msgID string)
}

// NewBot creates a Discord bot that only accepts DMs from the allowed user.
func NewBot(token, allowedUserID string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	dg.Identify.Intents = discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	bot := &Bot{
		session:       dg,
		allowedUserID: allowedUserID,
	}

	dg.AddHandler(bot.handleMessage)

	return bot, nil
}

// OnMessage sets the handler for incoming DM messages.
func (b *Bot) OnMessage(fn func(userID, content, msgID string)) {
	b.onMessage = fn
}

// Start opens the Discord websocket connection.
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("open discord: %w", err)
	}
	log.Printf("discord bot connected")
	return nil
}

// Stop closes the Discord connection.
func (b *Bot) Stop() error {
	return b.session.Close()
}

// SendDM sends a message to the allowed user's DM channel.
func (b *Bot) SendDM(content string) error {
	channel, err := b.session.UserChannelCreate(b.allowedUserID)
	if err != nil {
		return fmt.Errorf("create DM channel: %w", err)
	}

	chunks := SplitMessage(content, 2000)
	for _, chunk := range chunks {
		if _, err := b.session.ChannelMessageSend(channel.ID, chunk); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
	}
	return nil
}

// SendTyping sends a typing indicator to the user's DM channel.
func (b *Bot) SendTyping() {
	channel, err := b.session.UserChannelCreate(b.allowedUserID)
	if err != nil {
		return
	}
	_ = b.session.ChannelTyping(channel.ID)
}

func (b *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only accept DMs from allowed user
	if m.Author.ID != b.allowedUserID {
		log.Printf("discord: ignoring message from %s", m.Author.ID)
		return
	}

	// Only accept DMs (no guild messages)
	if m.GuildID != "" {
		return
	}

	if b.onMessage != nil {
		b.onMessage(m.Author.ID, m.Content, m.Message.ID)
	}
}
```

**Step 3: Verify build**

Run: `go build ./internal/discord/...`
Expected: no errors

**Step 4: Commit**

```bash
git add internal/discord/bot.go go.mod go.sum
git commit -m "feat: add Discord bot with DM-only filtering"
```

---

### Task 10: Message Formatting

**Files:**
- Create: `internal/discord/format.go`
- Create: `internal/discord/format_test.go`

**Step 1: Write format_test.go**

```go
package discord

import (
	"strings"
	"testing"
)

func TestSplitMessageShort(t *testing.T) {
	msg := "Hello, world!"
	chunks := SplitMessage(msg, 2000)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != msg {
		t.Errorf("expected %q, got %q", msg, chunks[0])
	}
}

func TestSplitMessageLong(t *testing.T) {
	msg := strings.Repeat("a", 4500)
	chunks := SplitMessage(msg, 2000)
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len(chunk) > 2000 {
			t.Errorf("chunk too long: %d chars", len(chunk))
		}
	}
	// Verify all content preserved
	joined := strings.Join(chunks, "")
	if joined != msg {
		t.Error("split did not preserve content")
	}
}

func TestSplitMessageAtNewline(t *testing.T) {
	// Build a message where a newline falls near the split point
	line1 := strings.Repeat("a", 1900)
	line2 := strings.Repeat("b", 200)
	msg := line1 + "\n" + line2

	chunks := SplitMessage(msg, 2000)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != line1 {
		t.Errorf("expected first chunk to be line1")
	}
}

func TestSplitMessageEmpty(t *testing.T) {
	chunks := SplitMessage("", 2000)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestSplitMessageCodeBlock(t *testing.T) {
	// Code block that spans across the split point
	code := "```go\n" + strings.Repeat("x := 1\n", 300) + "```"
	chunks := SplitMessage(code, 2000)
	for _, chunk := range chunks {
		if len(chunk) > 2000 {
			t.Errorf("chunk too long: %d chars", len(chunk))
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/discord/ -v`
Expected: FAIL

**Step 3: Write format.go**

```go
package discord

import "strings"

// SplitMessage splits a message into chunks that fit within maxLen.
// It tries to split at newlines when possible.
func SplitMessage(msg string, maxLen int) []string {
	if msg == "" {
		return nil
	}

	if len(msg) <= maxLen {
		return []string{msg}
	}

	var chunks []string
	remaining := msg

	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			chunks = append(chunks, remaining)
			break
		}

		// Try to find a good split point (newline) within the limit
		splitAt := maxLen
		nlIdx := strings.LastIndex(remaining[:maxLen], "\n")
		if nlIdx > maxLen/2 {
			splitAt = nlIdx
		}

		chunks = append(chunks, remaining[:splitAt])
		remaining = remaining[splitAt:]

		// Skip leading newline from split
		if len(remaining) > 0 && remaining[0] == '\n' {
			remaining = remaining[1:]
		}
	}

	return chunks
}
```

**Step 4: Run tests**

Run: `go test ./internal/discord/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/discord/format.go internal/discord/format_test.go
git commit -m "feat: add Discord message splitting at 2000 char limit"
```

---

### Task 11: Discord-Claude Bridge

**Files:**
- Create: `internal/discord/bridge.go`

**Step 1: Write bridge.go**

```go
package discord

import (
	"database/sql"
	"log"
	"time"

	"github.com/mattw/ai-lab/internal/claude"
	"github.com/mattw/ai-lab/internal/eventbus"
)

// Bridge connects Discord messages to a Claude Code session.
type Bridge struct {
	bot     *Bot
	session *claude.SessionManager
	db      *sql.DB
	bus     *eventbus.EventBus
}

// NewBridge creates a new Discord-Claude bridge.
func NewBridge(bot *Bot, session *claude.SessionManager, db *sql.DB, bus *eventbus.EventBus) *Bridge {
	return &Bridge{
		bot:     bot,
		session: session,
		db:      db,
		bus:     bus,
	}
}

// Start begins processing messages bidirectionally.
func (b *Bridge) Start() {
	b.bot.OnMessage(func(userID, content, msgID string) {
		b.handleIncoming(content, msgID)
	})

	go b.handleOutgoing()
}

func (b *Bridge) handleIncoming(content, msgID string) {
	log.Printf("discord incoming: %s", truncate(content, 100))

	// Store user message
	b.storeMessage("user", content, msgID)

	// Send typing indicator
	b.bot.SendTyping()

	// Publish event
	b.bus.Publish(eventbus.Event{
		Source:    "discord",
		Type:      "message_received",
		Summary:   truncate(content, 200),
		SessionID: b.session.SessionID(),
	})

	// Forward to Claude
	if err := b.session.Send(content); err != nil {
		log.Printf("error sending to claude: %v", err)
		b.bot.SendDM("Error communicating with Claude: " + err.Error())
	}
}

func (b *Bridge) handleOutgoing() {
	for evt := range b.session.Events() {
		switch evt.Type {
		case "assistant":
			aEvt, err := claude.ParseAssistantEvent(evt.Raw)
			if err != nil {
				continue
			}
			text := aEvt.FullText()
			if text == "" {
				continue
			}

			if err := b.bot.SendDM(text); err != nil {
				log.Printf("error sending to discord: %v", err)
			}

			// Store assistant message
			b.storeMessage("assistant", text, "")

			b.bus.Publish(eventbus.Event{
				Source:    "discord",
				Type:      "message_sent",
				Summary:   truncate(text, 200),
				SessionID: b.session.SessionID(),
			})

		case "result":
			rEvt, err := claude.ParseResultEvent(evt.Raw)
			if err != nil {
				continue
			}
			log.Printf("claude turn complete: cost=$%.4f duration=%dms", rEvt.TotalCostUSD, rEvt.DurationMS)
		}
	}
}

func (b *Bridge) storeMessage(role, content, discordMsgID string) {
	if b.db == nil {
		return
	}

	sessionID := b.session.SessionID()
	_, err := b.db.Exec(
		`INSERT INTO messages (session_id, role, content, discord_msg_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		sessionID, role, content, discordMsgID, time.Now().UTC(),
	)
	if err != nil {
		log.Printf("store message error: %v", err)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
```

**Step 2: Verify build**

Run: `go build ./internal/discord/...`
Expected: no errors (may need eventbus package first - see Task 14)

**Step 3: Commit**

```bash
git add internal/discord/bridge.go
git commit -m "feat: add Discord-Claude message bridge with persistence"
```

---

## Phase 4: Cron Scheduler

### Task 12: Command Builder

**Files:**
- Create: `internal/cron/command_builder.go`
- Create: `internal/cron/command_builder_test.go`

**Step 1: Write command_builder_test.go**

```go
package cron

import (
	"testing"

	"github.com/mattw/ai-lab/internal/claude"
)

func TestBuildCommand(t *testing.T) {
	job := &Job{
		ID:         "abc123",
		Name:       "daily-check",
		Prompt:     "Check system health",
		Model:      "sonnet",
		WorkingDir: "/home/user/project",
		MaxBudget:  2.50,
		Timeout:    300,
	}

	cfg := BuildOneshotConfig(job, "./SOUL.md")

	if cfg.Prompt != job.Prompt {
		t.Errorf("expected prompt %q, got %q", job.Prompt, cfg.Prompt)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("expected model sonnet, got %s", cfg.Model)
	}
	if cfg.WorkingDir != "/home/user/project" {
		t.Errorf("expected working dir /home/user/project, got %s", cfg.WorkingDir)
	}
	if cfg.MaxBudget != 2.50 {
		t.Errorf("expected max budget 2.50, got %f", cfg.MaxBudget)
	}
	if cfg.SoulMDPath != "./SOUL.md" {
		t.Errorf("expected soul path ./SOUL.md, got %s", cfg.SoulMDPath)
	}

	// Verify args produce valid CLI invocation
	args := cfg.Args()
	if len(args) == 0 {
		t.Fatal("expected non-empty args")
	}

	// Last arg should be the prompt
	if args[len(args)-1] != "Check system health" {
		t.Errorf("expected last arg to be prompt")
	}
}

func TestBuildCommandWithTools(t *testing.T) {
	job := &Job{
		ID:              "abc123",
		Name:            "restricted",
		Prompt:          "test",
		WorkingDir:      "/tmp",
		AllowedTools:    `["Bash","Read"]`,
		DisallowedTools: `["Write"]`,
	}

	cfg := BuildOneshotConfig(job, "")

	if len(cfg.AllowedTools) != 2 {
		t.Errorf("expected 2 allowed tools, got %d", len(cfg.AllowedTools))
	}
	if len(cfg.DisallowedTools) != 1 {
		t.Errorf("expected 1 disallowed tool, got %d", len(cfg.DisallowedTools))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cron/ -v`
Expected: FAIL

**Step 3: Write command_builder.go**

```go
package cron

import (
	"encoding/json"
	"log"

	"github.com/mattw/ai-lab/internal/claude"
)

// Job represents a cron job configuration from the database.
type Job struct {
	ID              string
	Name            string
	Description     string
	Schedule        string
	Enabled         bool
	Prompt          string
	Model           string
	WorkingDir      string
	AllowedTools    string // JSON array
	DisallowedTools string // JSON array
	MaxBudget       float64
	Timeout         int
	RetryMax        int
	RetryDelay      int
	OnFailure       string
	Tags            string // JSON array
}

// BuildOneshotConfig converts a Job into a OneshotConfig for execution.
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
			log.Printf("warning: parse allowed_tools for %s: %v", job.Name, err)
		} else {
			cfg.AllowedTools = tools
		}
	}

	if job.DisallowedTools != "" {
		var tools []string
		if err := json.Unmarshal([]byte(job.DisallowedTools), &tools); err != nil {
			log.Printf("warning: parse disallowed_tools for %s: %v", job.Name, err)
		} else {
			cfg.DisallowedTools = tools
		}
	}

	return cfg
}
```

**Step 4: Run tests**

Run: `go test ./internal/cron/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cron/command_builder.go internal/cron/command_builder_test.go
git commit -m "feat: add cron job command builder for Claude CLI args"
```

---

### Task 13: Job Executor

**Files:**
- Create: `internal/cron/executor.go`

**Step 1: Write executor.go**

```go
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

// Executor runs cron jobs via Claude Code one-shot mode.
type Executor struct {
	db         *sql.DB
	bus        *eventbus.EventBus
	soulMDPath string
	cronLogDir string
	alertFn    func(string) // called on failure to alert user (e.g., Discord DM)
}

// NewExecutor creates a new job executor.
func NewExecutor(db *sql.DB, bus *eventbus.EventBus, soulMDPath, cronLogDir string) *Executor {
	return &Executor{
		db:         db,
		bus:        bus,
		soulMDPath: soulMDPath,
		cronLogDir: cronLogDir,
	}
}

// SetAlertFunc sets the function called when a job fails and on_failure='alert'.
func (e *Executor) SetAlertFunc(fn func(string)) {
	e.alertFn = fn
}

// Run executes a job, handling retries and logging.
func (e *Executor) Run(ctx context.Context, job *Job) {
	for attempt := 1; attempt <= max(1, job.RetryMax+1); attempt++ {
		runID := e.createRun(job.ID, attempt)

		e.bus.Publish(eventbus.Event{
			Source:  "cron",
			Type:    "job_started",
			Summary: fmt.Sprintf("Job %q started (attempt %d)", job.Name, attempt),
		})

		result, err := e.executeOnce(ctx, job, runID)

		if err == nil && result.ExitCode == 0 {
			e.completeRun(runID, result, "success")
			e.bus.Publish(eventbus.Event{
				Source:  "cron",
				Type:    "job_completed",
				Summary: fmt.Sprintf("Job %q completed ($%.4f)", job.Name, result.CostUSD),
			})
			return
		}

		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		if result != nil && result.ErrorOutput != "" {
			errMsg += " " + result.ErrorOutput
		}

		e.completeRun(runID, result, "failed")

		log.Printf("cron job %q attempt %d failed: %s", job.Name, attempt, errMsg)

		if attempt < max(1, job.RetryMax+1) {
			delay := time.Duration(job.RetryDelay) * time.Second
			log.Printf("retrying job %q in %v", job.Name, delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}
	}

	// All attempts failed
	e.bus.Publish(eventbus.Event{
		Source:  "cron",
		Type:    "job_failed",
		Summary: fmt.Sprintf("Job %q failed after all attempts", job.Name),
	})

	if job.OnFailure == "alert" && e.alertFn != nil {
		e.alertFn(fmt.Sprintf("Cron job %q failed after %d attempts", job.Name, max(1, job.RetryMax+1)))
	}
}

func (e *Executor) executeOnce(ctx context.Context, job *Job, runID string) (*claude.OneshotResult, error) {
	cfg := BuildOneshotConfig(job, e.soulMDPath)

	timeout := time.Duration(job.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create log file
	var logWriter *os.File
	if e.cronLogDir != "" {
		if err := os.MkdirAll(e.cronLogDir, 0755); err != nil {
			log.Printf("warning: create cron log dir: %v", err)
		} else {
			logPath := filepath.Join(e.cronLogDir, fmt.Sprintf("%s-%s.jsonl", job.Name, runID))
			f, err := os.Create(logPath)
			if err != nil {
				log.Printf("warning: create log file: %v", err)
			} else {
				logWriter = f
				defer f.Close()
				e.updateRunLogPath(runID, logPath)
			}
		}
	}

	e.updateRunStatus(runID, "running")

	result, err := claude.RunOneshot(ctx, cfg, logWriter)
	return result, err
}

func (e *Executor) createRun(jobID string, attempt int) string {
	var runID string
	err := e.db.QueryRow(
		`INSERT INTO cron_runs (job_id, status, attempt, started_at) VALUES (?, 'running', ?, ?) RETURNING id`,
		jobID, attempt, time.Now().UTC(),
	).Scan(&runID)
	if err != nil {
		log.Printf("create run error: %v", err)
		return ""
	}
	return runID
}

func (e *Executor) updateRunStatus(runID, status string) {
	if runID == "" {
		return
	}
	_, err := e.db.Exec(`UPDATE cron_runs SET status = ? WHERE id = ?`, status, runID)
	if err != nil {
		log.Printf("update run status error: %v", err)
	}
}

func (e *Executor) updateRunLogPath(runID, path string) {
	if runID == "" {
		return
	}
	_, err := e.db.Exec(`UPDATE cron_runs SET stream_log_path = ? WHERE id = ?`, path, runID)
	if err != nil {
		log.Printf("update run log path error: %v", err)
	}
}

func (e *Executor) completeRun(runID string, result *claude.OneshotResult, status string) {
	if runID == "" {
		return
	}

	var exitCode int
	var outputText, errorOutput, sessionID string
	var costUSD float64
	var durationMS int

	if result != nil {
		exitCode = result.ExitCode
		outputText = result.ResultText
		errorOutput = result.ErrorOutput
		sessionID = result.SessionID
		costUSD = result.CostUSD
		durationMS = result.DurationMS
	}

	_, err := e.db.Exec(
		`UPDATE cron_runs SET status = ?, exit_code = ?, output_text = ?, error_output = ?,
		 session_id = ?, cost_usd = ?, duration_ms = ?, finished_at = ? WHERE id = ?`,
		status, exitCode, outputText, errorOutput, sessionID, costUSD, durationMS, time.Now().UTC(), runID,
	)
	if err != nil {
		log.Printf("complete run error: %v", err)
	}
}
```

**Step 2: Verify build**

Run: `go build ./internal/cron/...`
Expected: no errors (needs eventbus)

**Step 3: Commit**

```bash
git add internal/cron/executor.go
git commit -m "feat: add cron job executor with retry and logging"
```

---

### Task 14: EventBus (needed by Bridge and Executor)

**Files:**
- Create: `internal/eventbus/eventbus.go`

**Step 1: Write eventbus.go**

```go
package eventbus

import (
	"sync"
	"time"
)

// Event represents an activity event in the system.
type Event struct {
	Source    string `json:"source"`
	Type     string `json:"type"`
	Summary  string `json:"summary"`
	SessionID string `json:"session_id,omitempty"`
	Metadata  any    `json:"metadata,omitempty"`
	Time     time.Time `json:"time"`
}

// EventBus provides a simple pub/sub mechanism for internal events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[int]chan Event
	nextID      int
}

// New creates a new EventBus.
func New() *EventBus {
	return &EventBus{
		subscribers: make(map[int]chan Event),
	}
}

// Subscribe returns a channel that receives all published events.
// Call the returned function to unsubscribe.
func (eb *EventBus) Subscribe() (<-chan Event, func()) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := eb.nextID
	eb.nextID++

	ch := make(chan Event, 50)
	eb.subscribers[id] = ch

	unsub := func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		delete(eb.subscribers, id)
		close(ch)
	}

	return ch, unsub
}

// Publish sends an event to all subscribers.
func (eb *EventBus) Publish(evt Event) {
	if evt.Time.IsZero() {
		evt.Time = time.Now()
	}

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, ch := range eb.subscribers {
		select {
		case ch <- evt:
		default:
			// Drop if subscriber is too slow
		}
	}
}
```

**Step 2: Verify build**

Run: `go build ./internal/eventbus/...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/eventbus/eventbus.go
git commit -m "feat: add internal event bus for pub/sub between components"
```

---

### Task 15: Cron Scheduler

**Files:**
- Create: `internal/cron/scheduler.go`

**Step 1: Add cron dependency**

Run: `go get github.com/robfig/cron/v3`

**Step 2: Write scheduler.go**

```go
package cron

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// Scheduler manages scheduled Claude Code jobs.
type Scheduler struct {
	cron     *cron.Cron
	db       *sql.DB
	executor *Executor
	sem      chan struct{} // concurrency limiter
	mu       sync.Mutex
	entries  map[string]cron.EntryID // job ID -> cron entry ID
}

// NewScheduler creates a new cron scheduler.
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

// LoadJobs loads all enabled jobs from the database and schedules them.
func (s *Scheduler) LoadJobs() error {
	rows, err := s.db.Query(
		`SELECT id, name, description, schedule, enabled, prompt, model, working_dir,
		 allowed_tools, disallowed_tools, max_budget_usd, timeout_seconds,
		 retry_max, retry_delay_s, on_failure, tags
		 FROM cron_jobs WHERE enabled = 1`,
	)
	if err != nil {
		return fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var job Job
		var desc, allowedTools, disallowedTools, tags sql.NullString
		err := rows.Scan(
			&job.ID, &job.Name, &desc, &job.Schedule, &job.Enabled,
			&job.Prompt, &job.Model, &job.WorkingDir,
			&allowedTools, &disallowedTools, &job.MaxBudget, &job.Timeout,
			&job.RetryMax, &job.RetryDelay, &job.OnFailure, &tags,
		)
		if err != nil {
			return fmt.Errorf("scan job: %w", err)
		}
		job.Description = desc.String
		job.AllowedTools = allowedTools.String
		job.DisallowedTools = disallowedTools.String
		job.Tags = tags.String

		if err := s.AddJob(&job); err != nil {
			log.Printf("warning: could not schedule job %q: %v", job.Name, err)
		}
	}
	return rows.Err()
}

// AddJob schedules a job.
func (s *Scheduler) AddJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing entry for this job if any
	if entryID, ok := s.entries[job.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, job.ID)
	}

	jobCopy := *job
	entryID, err := s.cron.AddFunc(job.Schedule, func() {
		// Acquire semaphore
		s.sem <- struct{}{}
		defer func() { <-s.sem }()

		log.Printf("cron: running job %q", jobCopy.Name)
		s.executor.Run(context.Background(), &jobCopy)
	})
	if err != nil {
		return fmt.Errorf("schedule job %q: %w", job.Name, err)
	}

	s.entries[job.ID] = entryID
	log.Printf("cron: scheduled job %q with schedule %q", job.Name, job.Schedule)
	return nil
}

// RemoveJob unschedules a job.
func (s *Scheduler) RemoveJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[jobID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, jobID)
	}
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Printf("cron: scheduler started with %d jobs", len(s.entries))
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

// RunJobNow triggers immediate execution of a job by ID.
func (s *Scheduler) RunJobNow(ctx context.Context, jobID string) error {
	var job Job
	var desc, allowedTools, disallowedTools, tags sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, description, schedule, enabled, prompt, model, working_dir,
		 allowed_tools, disallowed_tools, max_budget_usd, timeout_seconds,
		 retry_max, retry_delay_s, on_failure, tags
		 FROM cron_jobs WHERE id = ?`, jobID,
	).Scan(
		&job.ID, &job.Name, &desc, &job.Schedule, &job.Enabled,
		&job.Prompt, &job.Model, &job.WorkingDir,
		&allowedTools, &disallowedTools, &job.MaxBudget, &job.Timeout,
		&job.RetryMax, &job.RetryDelay, &job.OnFailure, &tags,
	)
	if err != nil {
		return fmt.Errorf("load job %s: %w", jobID, err)
	}
	job.Description = desc.String
	job.AllowedTools = allowedTools.String
	job.DisallowedTools = disallowedTools.String
	job.Tags = tags.String

	go func() {
		s.sem <- struct{}{}
		defer func() { <-s.sem }()
		s.executor.Run(ctx, &job)
	}()

	return nil
}
```

**Step 3: Verify build**

Run: `go build ./internal/cron/...`
Expected: no errors

**Step 4: Commit**

```bash
git add internal/cron/scheduler.go go.mod go.sum
git commit -m "feat: add cron scheduler with concurrency control"
```

---

## Phase 5: Dashboard

### Task 16: HTTP Server + Router

**Files:**
- Create: `internal/dashboard/server.go`
- Create: `internal/dashboard/routes.go`
- Create: `internal/dashboard/handlers.go`
- Create: `internal/dashboard/sse.go`

**Step 1: Add chi dependency**

Run: `go get github.com/go-chi/chi/v5`

**Step 2: Write server.go**

```go
package dashboard

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mattw/ai-lab/internal/cron"
	"github.com/mattw/ai-lab/internal/eventbus"
)

//go:embed ../../web/templates/*
var templateFS embed.FS

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
func NewServer(db *sql.DB, bus *eventbus.EventBus, scheduler *cron.Scheduler, soulPath string) (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "web/templates/*.html", "web/templates/partials/*.html")
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
```

**Step 3: Write routes.go**

```go
package dashboard

func (s *Server) registerRoutes() {
	s.router.Get("/", s.handleHome)
	s.router.Get("/messages", s.handleMessages)
	s.router.Get("/crons", s.handleCrons)
	s.router.Get("/crons/{id}", s.handleCronDetail)
	s.router.Get("/crons/new", s.handleCronForm)
	s.router.Post("/crons", s.handleCronCreate)
	s.router.Post("/crons/{id}/toggle", s.handleCronToggle)
	s.router.Post("/crons/{id}/run", s.handleCronRunNow)
	s.router.Get("/soul", s.handleSoul)
	s.router.Post("/soul", s.handleSoulSave)
	s.router.Get("/activity/stream", s.handleSSE)
}
```

**Step 4: Write handlers.go**

```go
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

func (s *Server) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	type stats struct {
		TotalMessages int
		TotalCronJobs int
		TotalCronRuns int
		RecentActivity []activityRow
	}

	var st stats
	s.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&st.TotalMessages)
	s.db.QueryRow("SELECT COUNT(*) FROM cron_jobs").Scan(&st.TotalCronJobs)
	s.db.QueryRow("SELECT COUNT(*) FROM cron_runs").Scan(&st.TotalCronRuns)

	rows, _ := s.db.Query(
		"SELECT source, event_type, summary, created_at FROM activity_log ORDER BY created_at DESC LIMIT 20",
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

type activityRow struct {
	Source    string
	EventType string
	Summary  string
	CreatedAt string
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	type messageRow struct {
		ID        int
		SessionID string
		Role      string
		Content   string
		Model     string
		CreatedAt string
	}

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

func (s *Server) handleCrons(w http.ResponseWriter, r *http.Request) {
	type cronRow struct {
		ID       string
		Name     string
		Schedule string
		Enabled  bool
		Model    string
		LastRun  string
	}

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

	var jobs []cronRow
	for rows.Next() {
		var j cronRow
		rows.Scan(&j.ID, &j.Name, &j.Schedule, &j.Enabled, &j.Model, &j.LastRun)
		jobs = append(jobs, j)
	}

	s.render(w, "crons.html", pageData{Title: "Cron Jobs", Content: jobs})
}

func (s *Server) handleCronDetail(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	type detail struct {
		Job  cronJobDetail
		Runs []cronRunRow
	}

	var d detail
	var desc, allowedTools, disallowedTools, tags sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, description, schedule, enabled, prompt, model, working_dir,
		 allowed_tools, disallowed_tools, max_budget_usd, timeout_seconds, retry_max, on_failure, tags
		 FROM cron_jobs WHERE id = ?`, jobID,
	).Scan(
		&d.Job.ID, &d.Job.Name, &desc, &d.Job.Schedule, &d.Job.Enabled,
		&d.Job.Prompt, &d.Job.Model, &d.Job.WorkingDir,
		&allowedTools, &disallowedTools, &d.Job.MaxBudget, &d.Job.Timeout,
		&d.Job.RetryMax, &d.Job.OnFailure, &tags,
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

	// Reload scheduler
	s.scheduler.LoadJobs()

	http.Redirect(w, r, "/crons", http.StatusSeeOther)
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
```

**Step 5: Write sse.go**

```go
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events, unsub := s.bus.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
```

**Step 6: Verify build**

Run: `go build ./internal/dashboard/...`
Expected: no errors

**Step 7: Commit**

```bash
git add internal/dashboard/ go.mod go.sum
git commit -m "feat: add dashboard HTTP server with handlers and SSE"
```

---

### Task 17: HTML Templates

**Files:**
- Create: `web/templates/layout.html`
- Create: `web/templates/home.html`
- Create: `web/templates/messages.html`
- Create: `web/templates/crons.html`
- Create: `web/templates/cron_detail.html`
- Create: `web/templates/cron_form.html`
- Create: `web/templates/soul.html`
- Create: `web/templates/partials/.gitkeep`

**Step 1: Create layout.html**

Use HTMX CDN + Tailwind CDN. Functional first - simple sidebar nav, clean content area.

**Step 2: Create page templates**

Each page extends layout and renders its content. Use HTMX attributes for interactive elements (cron toggle, run now, SSE feed).

**Step 3: Commit**

```bash
git add web/
git commit -m "feat: add HTMX dashboard templates with Tailwind styling"
```

Note: The frontend-design skill should be invoked for this task to ensure high-quality UI.

---

### Task 18: Wire Everything in main.go

**Files:**
- Modify: `main.go`

**Step 1: Wire all components in main.go**

Connect config, DB, EventBus, SessionManager, Discord Bot, Bridge, Cron Scheduler, and Dashboard server. Handle graceful shutdown with signal handling.

```go
package main

import (
	"context"
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

func main() {
	_ = godotenv.Load()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

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
	srv, err := dashboard.NewServer(database, bus, scheduler, cfg.SoulMDPath)
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
	ctx := scheduler.Stop()
	<-ctx.Done()
	httpServer.Shutdown(context.Background())
	session.Stop()
}
```

**Step 2: Verify build**

Run: `go build -o ai-lab . && echo "BUILD OK"`
Expected: BUILD OK

**Step 3: Commit**

```bash
git add main.go
git commit -m "feat: wire all components in main.go with graceful shutdown"
```

---

## Phase 6: Deployment

### Task 19: Systemd Service + Setup Script

**Files:**
- Create: `deploy/systemd/ai-lab.service`
- Create: `deploy/setup.sh`

**Step 1: Create systemd unit file**

```ini
[Unit]
Description=ai-lab Personal AI Assistant
After=network.target

[Service]
Type=simple
User=mattw
WorkingDirectory=/home/mattw/Projects/personal/ai-lab
ExecStart=/home/mattw/Projects/personal/ai-lab/ai-lab
Restart=always
RestartSec=5
Environment=HOME=/home/mattw

[Install]
WantedBy=multi-user.target
```

**Step 2: Create setup.sh**

Bootstrap script that installs deps, builds, and configures the service.

**Step 3: Commit**

```bash
git add deploy/
git commit -m "feat: add systemd service and bootstrap setup script"
```

---

## Dependency Graph

Tasks that can run in parallel:
- Tasks 1-4 are sequential (foundation)
- Task 5 depends on Task 1 (go.mod)
- Tasks 5, 14 can start once Task 1 is done
- Tasks 6, 7, 8 depend on Task 5
- Tasks 9, 10 can be parallel with Tasks 6-8
- Task 11 depends on Tasks 9, 14, and 7
- Tasks 12, 13 depend on Tasks 5, 14
- Task 15 depends on Tasks 12, 13
- Task 16 depends on Tasks 14, 15
- Task 17 depends on Task 16
- Task 18 depends on everything
- Task 19 depends on Task 18
