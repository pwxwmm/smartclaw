package provider

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

// Router routes API requests to providers with fallback support.
type Router struct {
	resolver *Resolver
}

// NewRouter creates a router backed by the given resolver.
func NewRouter(resolver *Resolver) *Router {
	return &Router{resolver: resolver}
}

// Resolver returns the underlying resolver.
func (r *Router) Resolver() *Resolver {
	return r.resolver
}

// ClientForModel returns an api.Client configured for the given model.
// It resolves the model to a provider and creates a client with the right
// API key, base URL, and mode.
func (r *Router) ClientForModel(model string) (*api.Client, error) {
	p, err := r.resolver.ResolveModel(model)
	if err != nil {
		return nil, err
	}
	return providerToClient(p), nil
}

// ClientForKey returns an api.Client for the named provider key.
func (r *Router) ClientForKey(key string) (*api.Client, error) {
	p, err := r.resolver.Resolve(key)
	if err != nil {
		return nil, err
	}
	return providerToClient(p), nil
}

// PrimaryClient returns an api.Client for the primary provider.
func (r *Router) PrimaryClient() (*api.Client, error) {
	p, err := r.resolver.Primary()
	if err != nil {
		return nil, err
	}
	return providerToClient(p), nil
}

// CreateMessage sends a message using the primary provider, with automatic
// fallback to backup providers on error.
func (r *Router) CreateMessage(messages []api.Message, system string) (*api.MessageResponse, error) {
	primary, err := r.resolver.Primary()
	if err != nil {
		return nil, fmt.Errorf("no primary provider: %w", err)
	}

	client := providerToClient(primary)
	resp, err := client.CreateMessage(messages, system)
	if err == nil {
		return resp, nil
	}

	slog.Warn("primary provider failed, trying fallbacks", "provider", primary.Name, "error", err)
	return r.fallbackCreateMessage(messages, system)
}

// CreateMessageWithProvider sends a message using a specific provider, with fallback.
func (r *Router) CreateMessageWithProvider(providerKey string, messages []api.Message, system string) (*api.MessageResponse, error) {
	p, err := r.resolver.Resolve(providerKey)
	if err != nil {
		return nil, err
	}

	client := providerToClient(p)
	resp, err := client.CreateMessage(messages, system)
	if err == nil {
		return resp, nil
	}

	slog.Warn("provider failed, trying fallbacks", "provider", p.Name, "error", err)
	return r.fallbackCreateMessage(messages, system)
}

// StreamMessage streams a message using the primary provider, with fallback.
func (r *Router) StreamMessage(messages []api.Message, system string, onEvent func(event string, data []byte)) error {
	primary, err := r.resolver.Primary()
	if err != nil {
		return fmt.Errorf("no primary provider: %w", err)
	}

	client := providerToClient(primary)
	err = client.StreamMessage(messages, system, onEvent)
	if err == nil {
		return nil
	}

	slog.Warn("primary provider stream failed, trying fallbacks", "provider", primary.Name, "error", err)

	// Try fallbacks
	fallbacks := r.resolver.Fallbacks()
	for _, fb := range fallbacks {
		fbClient := providerToClient(fb)
		slog.Info("trying fallback provider", "provider", fb.Name)
		if fbErr := fbClient.StreamMessage(messages, system, onEvent); fbErr == nil {
			return nil
		} else {
			slog.Warn("fallback provider failed", "provider", fb.Name, "error", fbErr)
		}
	}

	return fmt.Errorf("all providers failed (primary: %w)", err)
}

// StreamMessageOpenAI streams using OpenAI-compatible providers with fallback.
func (r *Router) StreamMessageOpenAI(ctx context.Context, req *api.MessageRequest, handler func(event string, data []byte) error) error {
	primary, err := r.resolver.Primary()
	if err != nil {
		return fmt.Errorf("no primary provider: %w", err)
	}

	client := providerToClient(primary)
	err = client.StreamMessageOpenAI(ctx, req, handler)
	if err == nil {
		return nil
	}

	slog.Warn("primary provider stream failed, trying fallbacks", "provider", primary.Name, "error", err)

	fallbacks := r.resolver.Fallbacks()
	for _, fb := range fallbacks {
		if !fb.IsOpenAICompatible() {
			continue
		}
		fbClient := providerToClient(fb)
		fbReq := *req
		fbReq.Model = fb.Model
		slog.Info("trying fallback provider", "provider", fb.Name)
		if fbErr := fbClient.StreamMessageOpenAI(ctx, &fbReq, handler); fbErr == nil {
			return nil
		} else {
			slog.Warn("fallback provider failed", "provider", fb.Name, "error", fbErr)
		}
	}

	return fmt.Errorf("all providers failed (primary: %w)", err)
}

func (r *Router) fallbackCreateMessage(messages []api.Message, system string) (*api.MessageResponse, error) {
	fallbacks := r.resolver.Fallbacks()
	var lastErr error

	for _, fb := range fallbacks {
		fbClient := providerToClient(fb)
		slog.Info("trying fallback provider", "provider", fb.Name)
		resp, err := fbClient.CreateMessage(messages, system)
		if err == nil {
			return resp, nil
		}
		slog.Warn("fallback provider failed", "provider", fb.Name, "error", err)
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed (last: %w)", lastErr)
	}
	return nil, fmt.Errorf("all providers failed, no fallbacks available")
}

func providerToClient(p *Provider) *api.Client {
	client := api.NewClientWithModel(p.APIKey, p.BaseURL, p.Model)
	client.SetOpenAI(p.IsOpenAICompatible())
	return client
}

// RouteStats tracks routing statistics.
type RouteStats struct {
	PrimaryCalls     int
	PrimaryErrors    int
	FallbackCalls    int
	FallbackErrors   int
	LastFallbackAt   time.Time
	LastProviderUsed string
}
