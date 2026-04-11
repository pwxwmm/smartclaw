package provider

import (
	"os"
	"testing"
)

func TestResolverRegister(t *testing.T) {
	r := NewResolver()

	r.Register("zhipu", &Provider{
		Name:   "zhipu",
		APIKey: "test-key",
		Model:  "glm-5",
		Mode:   ModeChatCompletions,
	})

	p, err := r.Resolve("zhipu")
	if err != nil {
		t.Fatalf("Resolve zhipu: %v", err)
	}
	if p.Model != "glm-5" {
		t.Errorf("expected model glm-5, got %s", p.Model)
	}

	// First non-fallback should be primary
	primary, err := r.Primary()
	if err != nil {
		t.Fatalf("Primary: %v", err)
	}
	if primary.Name != "zhipu" {
		t.Errorf("expected primary zhipu, got %s", primary.Name)
	}
}

func TestResolverFallback(t *testing.T) {
	r := NewResolver()

	r.Register("zhipu", &Provider{
		Name:   "zhipu",
		APIKey: "key1",
		Model:  "glm-5",
		Mode:   ModeChatCompletions,
	})
	r.Register("openrouter", &Provider{
		Name:     "openrouter",
		APIKey:   "key2",
		Model:    "anthropic/claude-sonnet-4-5",
		Mode:     ModeChatCompletions,
		Fallback: true,
	})

	fallbacks := r.Fallbacks()
	if len(fallbacks) != 1 {
		t.Fatalf("expected 1 fallback, got %d", len(fallbacks))
	}
	if fallbacks[0].Name != "openrouter" {
		t.Errorf("expected fallback openrouter, got %s", fallbacks[0].Name)
	}

	// Primary should still be zhipu
	primary, _ := r.Primary()
	if primary.Name != "zhipu" {
		t.Errorf("expected primary zhipu, got %s", primary.Name)
	}
}

func TestResolverModelMatch(t *testing.T) {
	r := NewResolver()

	r.Register("zhipu", &Provider{
		Name:   "zhipu",
		APIKey: "key1",
		Model:  "glm-5",
		Mode:   ModeChatCompletions,
	})
	r.Register("anthropic", &Provider{
		Name:   "anthropic",
		APIKey: "key2",
		Model:  "claude-sonnet-4-5",
		Mode:   ModeAnthropicMessages,
	})

	tests := []struct {
		query    string
		wantName string
	}{
		{"glm-5", "zhipu"},
		{"claude-sonnet-4-5", "anthropic"},
		{"claude", "anthropic"}, // prefix match
		{"unknown", "zhipu"},    // falls back to primary
	}

	for _, tt := range tests {
		p, err := r.ResolveModel(tt.query)
		if err != nil {
			t.Errorf("ResolveModel(%q): %v", tt.query, err)
			continue
		}
		if p.Name != tt.wantName {
			t.Errorf("ResolveModel(%q) = %s, want %s", tt.query, p.Name, tt.wantName)
		}
	}
}

func TestResolverEnvFallback(t *testing.T) {
	r := NewResolver()

	// Set env vars
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("OPENAI_API_KEY")

	r.loadFromEnv()

	if len(r.providers) < 2 {
		t.Fatalf("expected at least 2 providers from env, got %d", len(r.providers))
	}

	p, err := r.Resolve("anthropic")
	if err != nil {
		t.Fatalf("Resolve anthropic: %v", err)
	}
	if p.APIKey != "test-anthropic-key" {
		t.Errorf("expected anthropic key, got %s", p.APIKey)
	}
	if p.Mode != ModeAnthropicMessages {
		t.Errorf("expected anthropic_messages mode, got %s", p.Mode)
	}
}

func TestProviderIsOpenAI(t *testing.T) {
	tests := []struct {
		mode APIMode
		want bool
	}{
		{ModeChatCompletions, true},
		{ModeAnthropicMessages, false},
		{"", true}, // default
	}

	for _, tt := range tests {
		p := &Provider{Mode: tt.mode}
		if got := p.IsOpenAICompatible(); got != tt.want {
			t.Errorf("Mode %q: IsOpenAICompatible() = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestResolverSetPrimary(t *testing.T) {
	r := NewResolver()

	r.Register("a", &Provider{Name: "a", APIKey: "k1", Model: "m1"})
	r.Register("b", &Provider{Name: "b", APIKey: "k2", Model: "m2"})

	// Primary should be "a" (first non-fallback)
	p, _ := r.Primary()
	if p.Name != "a" {
		t.Errorf("initial primary = %s, want a", p.Name)
	}

	// Change primary
	if err := r.SetPrimary("b"); err != nil {
		t.Fatalf("SetPrimary: %v", err)
	}

	p, _ = r.Primary()
	if p.Name != "b" {
		t.Errorf("after SetPrimary: primary = %s, want b", p.Name)
	}

	// Invalid key
	if err := r.SetPrimary("nonexistent"); err == nil {
		t.Error("expected error for nonexistent provider")
	}
}
