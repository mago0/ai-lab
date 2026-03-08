package claude

import "encoding/json"

// StreamEvent is a raw event from Claude Code's stream-json output.
type StreamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

// SystemEvent is emitted for system-level events (init, hooks).
type SystemEvent struct {
	Type      string   `json:"type"`
	Subtype   string   `json:"subtype"`
	SessionID string   `json:"session_id"`
	CWD       string   `json:"cwd,omitempty"`
	Tools     []string `json:"tools,omitempty"`
	Model     string   `json:"model,omitempty"`
	HookID    string   `json:"hook_id,omitempty"`
	HookName  string   `json:"hook_name,omitempty"`
	HookEvent string   `json:"hook_event,omitempty"`
	Output    string   `json:"output,omitempty"`
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
	Type            string           `json:"type"`
	Message         AssistantMessage `json:"message"`
	SessionID       string           `json:"session_id"`
	ParentToolUseID *string          `json:"parent_tool_use_id"`
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
