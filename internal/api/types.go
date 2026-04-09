package api

import "time"

// ContentBlock represents a content block in a message
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// For tool_use
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`

	// For tool_result
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`

	// For thinking
	Thinking string `json:"thinking,omitempty"`

	PartialJSON string `json:"-"`

	// For image
	Source *ImageSource `json:"source,omitempty"`
}

// ImageSource represents an image source
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
	URL       string `json:"url,omitempty"`
}

// ToolDefinition is defined in tools.go

// MessageRequest is the request body for the messages API
type MessageRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	Messages    []MessageParam   `json:"messages"`
	System      interface{}      `json:"system,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	ToolChoice  interface{}      `json:"tool_choice,omitempty"`
	Metadata    *Metadata        `json:"metadata,omitempty"`

	// Beta features
	Betas []string `json:"-"`
}

// MessageParam represents a message parameter (user or assistant)
type MessageParam struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// SystemBlock represents a system prompt block
type SystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl represents cache control settings
type CacheControl struct {
	Type  string `json:"type"`
	TTL   string `json:"ttl,omitempty"`
	Scope string `json:"scope,omitempty"`
}

// Metadata represents API metadata
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// MessageResponse is the response from the messages API
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`

	// Request ID for tracking
	RequestID string `json:"-"`
}

// Usage represents token usage
type Usage struct {
	InputTokens   int `json:"input_tokens"`
	OutputTokens  int `json:"output_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens,omitempty"`
	CacheRead     int `json:"cache_read_input_tokens,omitempty"`
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	Type    string           `json:"type"`
	Index   int              `json:"index,omitempty"`
	Delta   *ContentDelta    `json:"delta,omitempty"`
	Message *MessageResponse `json:"message,omitempty"`
	Usage   *Usage           `json:"usage,omitempty"`
}

// ContentDelta represents a content delta in streaming
type ContentDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

// APIError represents an API error
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// ClientConfig represents client configuration
type ClientConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	MaxTokens  int
	Timeout    time.Duration
	MaxRetries int
	AuthToken  string // OAuth token
	UserAgent  string
	SessionID  string

	// Beta features
	Betas []string
}

// ToolChoiceAuto represents automatic tool choice
type ToolChoiceAuto struct {
	Type string `json:"type"`
}

// ToolChoiceAny represents any tool choice
type ToolChoiceAny struct {
	Type string `json:"type"`
}

// ToolChoiceTool represents a specific tool choice
type ToolChoiceTool struct {
	Type string `json:"type"`
	Name string `json:"name"`
}
