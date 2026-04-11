package query

import (
	"context"
	"sync"
)

type TokenBudget struct {
	MaxTokens      int
	UsedTokens     int
	ReservedTokens int
	mu             sync.Mutex
}

func NewTokenBudget(maxTokens int) *TokenBudget {
	return &TokenBudget{
		MaxTokens:      maxTokens,
		UsedTokens:     0,
		ReservedTokens: 0,
	}
}

func (b *TokenBudget) Allocate(tokens int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.UsedTokens+tokens > b.MaxTokens {
		return false
	}

	b.UsedTokens += tokens
	return true
}

func (b *TokenBudget) Reserve(tokens int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.UsedTokens+b.ReservedTokens+tokens > b.MaxTokens {
		return false
	}

	b.ReservedTokens += tokens
	return true
}

func (b *TokenBudget) Commit(tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ReservedTokens >= tokens {
		b.ReservedTokens -= tokens
		b.UsedTokens += tokens
	}
}

func (b *TokenBudget) Release(tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.ReservedTokens >= tokens {
		b.ReservedTokens -= tokens
	}
}

func (b *TokenBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.UsedTokens = 0
	b.ReservedTokens = 0
}

func (b *TokenBudget) Available() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.MaxTokens - b.UsedTokens - b.ReservedTokens
}

func (b *TokenBudget) GetUsed() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.UsedTokens
}

func (b *TokenBudget) SetMaxTokens(max int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.MaxTokens = max
}

type QueryConfig struct {
	Model         string
	MaxTokens     int
	Temperature   float64
	TopP          float64
	StopSequences []string
	SystemPrompt  string
	EnableCaching bool
}

func DefaultQueryConfig() *QueryConfig {
	return &QueryConfig{
		Model:         "claude-sonnet-4-5",
		MaxTokens:     4096,
		Temperature:   1.0,
		TopP:          1.0,
		StopSequences: []string{},
		EnableCaching: true,
	}
}

type QueryDeps struct {
	Config       *QueryConfig
	TokenBudget  *TokenBudget
	ToolRegistry any
	StateStore   any
}

func NewQueryDeps(config *QueryConfig) *QueryDeps {
	if config == nil {
		config = DefaultQueryConfig()
	}

	return &QueryDeps{
		Config:      config,
		TokenBudget: NewTokenBudget(config.MaxTokens),
	}
}

type StopHook func(ctx context.Context, reason string) error

type StopHooks struct {
	hooks []StopHook
	mu    sync.RWMutex
}

func NewStopHooks() *StopHooks {
	return &StopHooks{
		hooks: make([]StopHook, 0),
	}
}

func (s *StopHooks) Add(hook StopHook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = append(s.hooks, hook)
}

func (s *StopHooks) Remove(hook StopHook) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, h := range s.hooks {
		if &h == &hook {
			s.hooks = append(s.hooks[:i], s.hooks[i+1:]...)
			break
		}
	}
}

func (s *StopHooks) Execute(ctx context.Context, reason string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, hook := range s.hooks {
		if err := hook(ctx, reason); err != nil {
			return err
		}
	}
	return nil
}

func (s *StopHooks) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = s.hooks[:0]
}
