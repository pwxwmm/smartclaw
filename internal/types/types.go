package types

import "time"

type Config struct {
	APIKey         string            `json:"api_key"`
	Model          string            `json:"model"`
	BaseURL        string            `json:"base_url"`
	MaxTokens      int               `json:"max_tokens"`
	Temperature    float64           `json:"temperature"`
	Permissions    string            `json:"permissions"`
	SandboxEnabled bool              `json:"sandbox_enabled"`
	CustomTools    []string          `json:"custom_tools"`
	Env            map[string]string `json:"env"`
}

type ToolCall struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]any `json:"input"`
}

type ToolResult struct {
	ToolUseID string      `json:"tool_use_id"`
	Content   any `json:"content"`
	IsError   bool        `json:"is_error"`
}

type Message struct {
	Role    string      `json:"role"`
	Content any `json:"content"`
}

type ContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Source    *ImageSource           `json:"source,omitempty"`
	Thinking  string                 `json:"thinking,omitempty"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
	URL       string `json:"url,omitempty"`
}

type Usage struct {
	InputTokens   int `json:"input_tokens"`
	OutputTokens  int `json:"output_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
}

type StreamEvent struct {
	Type string      `json:"type"`
	Data any `json:"data"`
}

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Message
}

type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Model     string                 `json:"model"`
	Messages  []Message              `json:"messages"`
	Metadata  map[string]any `json:"metadata"`
}

func NewSession(id string) *Session {
	return &Session{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]Message, 0),
		Metadata:  make(map[string]any),
	}
}

func (s *Session) AddMessage(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

func (s *Session) GetMessages() []Message {
	return s.Messages
}

type APIResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type Permission struct {
	Mode  string   `json:"mode"`
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
	Ask   []string `json:"ask"`
}

type Credentials struct {
	APIKey       string    `json:"api_key"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (c *Credentials) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

func (c *Credentials) IsValid() bool {
	return c.APIKey != "" || c.AccessToken != ""
}

type CacheEntry struct {
	Key       string      `json:"key"`
	Value     any `json:"value"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt *time.Time  `json:"expires_at"`
}

func (e *CacheEntry) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*e.ExpiresAt)
}

type Progress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

func (p *Progress) Percent() float64 {
	if p.Total == 0 {
		return 0
	}
	return float64(p.Completed) / float64(p.Total) * 100
}

func (p *Progress) IsComplete() bool {
	return p.Completed+p.Failed >= p.Total
}
