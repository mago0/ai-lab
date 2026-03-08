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
