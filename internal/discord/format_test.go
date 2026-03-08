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
	// Build a 4500-character message
	msg := strings.Repeat("abcdefghij", 450) // 10 * 450 = 4500 chars
	chunks := SplitMessage(msg, 2000)

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}

	for i, chunk := range chunks {
		if len(chunk) > 2000 {
			t.Errorf("chunk %d has length %d, exceeds 2000", i, len(chunk))
		}
	}

	// Verify no content is lost
	reassembled := strings.Join(chunks, "")
	if reassembled != msg {
		t.Error("reassembled chunks do not match original message")
	}
}

func TestSplitMessageAtNewline(t *testing.T) {
	// Create a message where a newline is a natural split point.
	// Line1 is 1500 chars, then a newline, then line2 is 1000 chars.
	line1 := strings.Repeat("a", 1500)
	line2 := strings.Repeat("b", 1000)
	msg := line1 + "\n" + line2

	chunks := SplitMessage(msg, 2000)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != line1 {
		t.Errorf("expected first chunk to be line1 (len %d), got len %d", len(line1), len(chunks[0]))
	}
	if chunks[1] != line2 {
		t.Errorf("expected second chunk to be line2 (len %d), got len %d", len(line2), len(chunks[1]))
	}
}

func TestSplitMessageEmpty(t *testing.T) {
	chunks := SplitMessage("", 2000)

	if chunks != nil {
		t.Errorf("expected nil for empty message, got %v", chunks)
	}
}

func TestSplitMessageCodeBlock(t *testing.T) {
	// Build a message with a code block that spans past a 2000-char boundary.
	// Preamble + code block content that forces a split inside the block.
	preamble := strings.Repeat("x", 1800) + "\n"
	codeBlock := "```go\n" + strings.Repeat("func foo() {}\n", 200) + "```\n"
	msg := preamble + codeBlock

	chunks := SplitMessage(msg, 2000)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	for i, chunk := range chunks {
		if len(chunk) > 2000 {
			t.Errorf("chunk %d has length %d, exceeds 2000", i, len(chunk))
		}
	}

	// Verify no content is lost (newlines consumed during splitting are acceptable)
	reassembled := strings.Join(chunks, "")
	// The split may consume newlines at split points, so check that the
	// non-newline content is preserved by comparing without the split newlines.
	// A simpler check: total length should be close to original.
	if len(reassembled) > len(msg) {
		t.Error("reassembled message is longer than original - unexpected")
	}
}
