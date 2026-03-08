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

// SessionConfig holds configuration for a long-running Claude Code session.
type SessionConfig struct {
	Model      string
	WorkingDir string
	SoulMDPath string
	SessionID  string // Resume an existing session if set.
}

// SessionManager manages a long-running Claude Code subprocess that
// communicates via stream-json on stdin/stdout.
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

// NewSessionManager creates a SessionManager with the given configuration.
// The events channel is buffered to 100 entries.
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

// Start spawns the claude subprocess and begins reading events.
func (sm *SessionManager) Start() error {
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

	// Strip CLAUDECODE env var from subprocess environment.
	sm.cmd.Env = stripEnvVar(os.Environ(), "CLAUDECODE")

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

// Send writes a user message to the session's stdin.
func (sm *SessionManager) Send(content string) error {
	msg := NewUserInput(content)
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal user input: %w", err)
	}
	data = append(data, '\n')

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, err := sm.stdin.Write(data); err != nil {
		return fmt.Errorf("write to stdin: %w", err)
	}
	return nil
}

// Events returns a read-only channel of parsed stream events.
func (sm *SessionManager) Events() <-chan StreamEvent {
	return sm.events
}

// SessionID returns the session ID captured from the init event.
func (sm *SessionManager) SessionID() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessionID
}

// Done returns a channel that is closed when the subprocess exits.
func (sm *SessionManager) Done() <-chan struct{} {
	return sm.done
}

// Stop gracefully shuts down the session. It cancels the context, closes
// stdin, and waits up to 10 seconds for the process to exit before killing it.
func (sm *SessionManager) Stop() error {
	sm.cancel()

	sm.mu.Lock()
	if sm.stdin != nil {
		sm.stdin.Close()
	}
	sm.mu.Unlock()

	if sm.cmd == nil || sm.cmd.Process == nil {
		return nil
	}

	// Wait for the process to exit with a timeout.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- sm.cmd.Wait()
	}()

	select {
	case err := <-waitDone:
		return err
	case <-time.After(10 * time.Second):
		if err := sm.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("kill process: %w", err)
		}
		return <-waitDone
	}
}

// readEvents reads stdout line by line, parses stream events, captures the
// session ID from init events, and sends parsed events to the events channel.
func (sm *SessionManager) readEvents() {
	defer close(sm.events)
	defer close(sm.done)

	scanner := bufio.NewScanner(sm.stdout)
	scanner.Buffer(make([]byte, 0, maxScannerBuffer), maxScannerBuffer)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		ev, err := ParseStreamEvent([]byte(line))
		if err != nil {
			log.Printf("claude session: parse event: %v", err)
			continue
		}

		// Capture session_id from init events.
		if ev.Type == "system" && ev.Subtype == "init" {
			sysEv, parseErr := ParseSystemEvent(ev.Raw)
			if parseErr == nil && sysEv.SessionID != "" {
				sm.mu.Lock()
				sm.sessionID = sysEv.SessionID
				sm.mu.Unlock()
			}
		}

		select {
		case sm.events <- ev:
		case <-sm.ctx.Done():
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("claude session: scanner error: %v", err)
	}
}

// readStderr reads stderr and logs each line.
func (sm *SessionManager) readStderr() {
	scanner := bufio.NewScanner(sm.stderr)
	for scanner.Scan() {
		log.Printf("claude stderr: %s", scanner.Text())
	}
}

// stripEnvVar returns a copy of env with any entries starting with key= removed.
func stripEnvVar(env []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
