package provider

// APIMode determines how to format requests and parse responses.
type APIMode string

const (
	// ModeChatCompletions uses OpenAI-compatible /v1/chat/completions.
	ModeChatCompletions APIMode = "chat_completions"
	// ModeAnthropicMessages uses Anthropic /v1/messages.
	ModeAnthropicMessages APIMode = "anthropic_messages"
)

// Provider represents a single LLM provider configuration.
type Provider struct {
	Name     string  `json:"name"`
	APIKey   string  `json:"api_key"`
	BaseURL  string  `json:"base_url"`
	Model    string  `json:"model"`
	Mode     APIMode `json:"mode"`
	Fallback bool    `json:"fallback,omitempty"` // use as fallback when primary fails

	// Optional overrides
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// IsOpenAICompatible returns true if the provider uses the OpenAI chat completions format.
func (p *Provider) IsOpenAICompatible() bool {
	return p.Mode == ModeChatCompletions || p.Mode == ""
}

// EffectiveMode returns the API mode, defaulting to chat_completions.
func (p *Provider) EffectiveMode() APIMode {
	if p.Mode == "" {
		return ModeChatCompletions
	}
	return p.Mode
}
