package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// maxScannerBuffer is the maximum line size the stream scanner will handle (1 MB).
const maxScannerBuffer = 1024 * 1024

// TextContent returns the concatenated text from all "text" content blocks.
func (e *AssistantEvent) TextContent() string {
	var parts []string
	for _, block := range e.Message.Content {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "")
}

// FullText returns the concatenated text from all content blocks that carry
// displayable text - both "text" and "thinking" blocks.
func (e *AssistantEvent) FullText() string {
	var parts []string
	for _, block := range e.Message.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "thinking":
			if block.Thinking != "" {
				parts = append(parts, block.Thinking)
			}
		}
	}
	return strings.Join(parts, "")
}

// ParseStreamEvent unmarshals a raw JSON line into a StreamEvent, preserving
// the raw bytes for further parsing.
func ParseStreamEvent(data []byte) (StreamEvent, error) {
	var ev StreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return ev, fmt.Errorf("parse stream event: %w", err)
	}
	ev.Raw = make(json.RawMessage, len(data))
	copy(ev.Raw, data)
	return ev, nil
}

// ParseSystemEvent unmarshals raw JSON bytes into a SystemEvent.
func ParseSystemEvent(data []byte) (SystemEvent, error) {
	var ev SystemEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return ev, fmt.Errorf("parse system event: %w", err)
	}
	return ev, nil
}

// ParseAssistantEvent unmarshals raw JSON bytes into an AssistantEvent.
func ParseAssistantEvent(data []byte) (AssistantEvent, error) {
	var ev AssistantEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return ev, fmt.Errorf("parse assistant event: %w", err)
	}
	return ev, nil
}

// ParseResultEvent unmarshals raw JSON bytes into a ResultEvent.
func ParseResultEvent(data []byte) (ResultEvent, error) {
	var ev ResultEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return ev, fmt.Errorf("parse result event: %w", err)
	}
	return ev, nil
}

// ParseAllEvents reads all JSON lines from r and returns the parsed StreamEvents.
// Empty lines are skipped.
func ParseAllEvents(r io.Reader) ([]StreamEvent, error) {
	var events []StreamEvent
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, maxScannerBuffer), maxScannerBuffer)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		ev, err := ParseStreamEvent([]byte(line))
		if err != nil {
			return events, err
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scan stream: %w", err)
	}
	return events, nil
}

// StreamScanner reads newline-delimited JSON events from an io.Reader one at
// a time. Call Scan to advance, Event to retrieve the current event, and Err
// to check for errors after Scan returns false.
type StreamScanner struct {
	scanner *bufio.Scanner
	event   StreamEvent
	err     error
}

// NewStreamScanner creates a StreamScanner that reads from r with a 1 MB buffer.
func NewStreamScanner(r io.Reader) *StreamScanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, maxScannerBuffer), maxScannerBuffer)
	return &StreamScanner{scanner: s}
}

// Scan advances to the next event. It returns false when there are no more
// events or an error occurs.
func (s *StreamScanner) Scan() bool {
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" {
			continue
		}
		ev, err := ParseStreamEvent([]byte(line))
		if err != nil {
			s.err = err
			return false
		}
		s.event = ev
		return true
	}
	if err := s.scanner.Err(); err != nil {
		s.err = fmt.Errorf("scan stream: %w", err)
	}
	return false
}

// Event returns the most recently scanned StreamEvent.
func (s *StreamScanner) Event() StreamEvent {
	return s.event
}

// Err returns the first non-EOF error encountered by the scanner.
func (s *StreamScanner) Err() error {
	return s.err
}
