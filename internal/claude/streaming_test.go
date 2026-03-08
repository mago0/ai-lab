package claude

import (
	"strings"
	"testing"
)

// Sample stream-json lines used across tests.
const (
	sampleInit = `{"type":"system","subtype":"init","session_id":"abc-123","cwd":"/tmp","tools":["Bash","Read","Write"],"model":"claude-sonnet-4-20250514"}`

	sampleAssistant = `{"type":"assistant","session_id":"abc-123","message":{"id":"msg_01","model":"claude-sonnet-4-20250514","role":"assistant","content":[{"type":"text","text":"Hello, world!"},{"type":"thinking","thinking":"Let me think about this."}],"stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":25,"cache_read_input_tokens":50,"cache_creation_input_tokens":10}},"parent_tool_use_id":null}`

	sampleResult = `{"type":"result","subtype":"success","is_error":false,"duration_ms":1500,"num_turns":1,"result":"Done.","stop_reason":"end_turn","session_id":"abc-123","total_cost_usd":0.0042,"usage":{"input_tokens":100,"output_tokens":25,"cache_read_input_tokens":50,"cache_creation_input_tokens":10}}`
)

func TestParseStreamEvents(t *testing.T) {
	input := strings.Join([]string{sampleInit, sampleAssistant, sampleResult}, "\n")
	events, err := ParseAllEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAllEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	want := []string{"system", "assistant", "result"}
	for i, ev := range events {
		if ev.Type != want[i] {
			t.Errorf("event[%d].Type = %q, want %q", i, ev.Type, want[i])
		}
	}
}

func TestParseInitEvent(t *testing.T) {
	ev, err := ParseSystemEvent([]byte(sampleInit))
	if err != nil {
		t.Fatalf("ParseSystemEvent: %v", err)
	}
	if ev.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "abc-123")
	}
	if ev.Subtype != "init" {
		t.Errorf("Subtype = %q, want %q", ev.Subtype, "init")
	}
	if ev.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", ev.Model, "claude-sonnet-4-20250514")
	}
	if ev.CWD != "/tmp" {
		t.Errorf("CWD = %q, want %q", ev.CWD, "/tmp")
	}
	if len(ev.Tools) != 3 {
		t.Errorf("len(Tools) = %d, want 3", len(ev.Tools))
	}
}

func TestParseAssistantEvent(t *testing.T) {
	ev, err := ParseAssistantEvent([]byte(sampleAssistant))
	if err != nil {
		t.Fatalf("ParseAssistantEvent: %v", err)
	}

	// TextContent should return only text blocks.
	text := ev.TextContent()
	if text != "Hello, world!" {
		t.Errorf("TextContent() = %q, want %q", text, "Hello, world!")
	}

	// FullText should include both text and thinking blocks.
	full := ev.FullText()
	if !strings.Contains(full, "Hello, world!") {
		t.Errorf("FullText() missing text content")
	}
	if !strings.Contains(full, "Let me think about this.") {
		t.Errorf("FullText() missing thinking content")
	}

	if ev.Message.Role != "assistant" {
		t.Errorf("Role = %q, want %q", ev.Message.Role, "assistant")
	}
	if ev.Message.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if ev.Message.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", ev.Message.Usage.InputTokens)
	}
	if ev.Message.Usage.OutputTokens != 25 {
		t.Errorf("OutputTokens = %d, want 25", ev.Message.Usage.OutputTokens)
	}
}

func TestParseResultEvent(t *testing.T) {
	ev, err := ParseResultEvent([]byte(sampleResult))
	if err != nil {
		t.Fatalf("ParseResultEvent: %v", err)
	}
	if ev.IsError {
		t.Error("IsError = true, want false")
	}
	if ev.Result != "Done." {
		t.Errorf("Result = %q, want %q", ev.Result, "Done.")
	}
	if ev.TotalCostUSD != 0.0042 {
		t.Errorf("TotalCostUSD = %f, want 0.0042", ev.TotalCostUSD)
	}
	if ev.Subtype != "success" {
		t.Errorf("Subtype = %q, want %q", ev.Subtype, "success")
	}
	if ev.DurationMS != 1500 {
		t.Errorf("DurationMS = %d, want 1500", ev.DurationMS)
	}
	if ev.NumTurns != 1 {
		t.Errorf("NumTurns = %d, want 1", ev.NumTurns)
	}
	if ev.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if ev.Usage.CacheReadInputTokens != 50 {
		t.Errorf("CacheReadInputTokens = %d, want 50", ev.Usage.CacheReadInputTokens)
	}
}

func TestStreamScanner(t *testing.T) {
	// Include empty lines to verify they are skipped.
	input := strings.Join([]string{
		sampleInit,
		"",
		sampleAssistant,
		"  ",
		sampleResult,
	}, "\n")

	scanner := NewStreamScanner(strings.NewReader(input))

	want := []string{"system", "assistant", "result"}
	var got []string

	for scanner.Scan() {
		got = append(got, scanner.Event().Type)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("StreamScanner.Err: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("scanned %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d].Type = %q, want %q", i, got[i], want[i])
		}
	}
}
