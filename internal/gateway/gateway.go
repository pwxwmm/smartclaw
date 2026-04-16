package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/store"
)

type GatewayConfig struct {
	JSONLFallbackDir string
	CronConfig       CronConfig
}

type GatewayResponse struct {
	Content   string
	SessionID string
	Platform  string
	Usage     UsageInfo
	Duration  time.Duration
}

type UsageInfo struct {
	InputTokens  int
	OutputTokens int
	Cost         float64
}

type Gateway struct {
	router      *SessionRouter
	delivery    *DeliveryManager
	pairing     *PairingManager
	cronTrigger *CronTrigger
	learning    *learning.LearningLoop
	memory      *memory.MemoryManager
	store       *store.Store
	jsonlWriter *store.JSONLWriter

	engineFactory EngineFactory
	mu            sync.RWMutex
	engines       map[string]*runtime.QueryEngine
	lastAccess    map[string]time.Time
	maxIdle       time.Duration
	stopCleanup   chan struct{}
	cleanupOnce   sync.Once
}

type EngineFactory func() *runtime.QueryEngine

func NewGateway(engineFactory EngineFactory, memManager *memory.MemoryManager, learningLoop *learning.LearningLoop) *Gateway {
	return NewGatewayWithConfig(engineFactory, memManager, learningLoop, GatewayConfig{})
}

func NewGatewayWithConfig(engineFactory EngineFactory, memManager *memory.MemoryManager, learningLoop *learning.LearningLoop, cfg GatewayConfig) *Gateway {
	s := memManager.GetStore()

	var jsonlDir string
	if s == nil {
		if cfg.JSONLFallbackDir != "" {
			jsonlDir = cfg.JSONLFallbackDir
		} else {
			home, _ := os.UserHomeDir()
			jsonlDir = filepath.Join(home, ".smartclaw", "jsonl")
		}
		slog.Warn("gateway: SQLite unavailable, falling back to JSONL persistence")
	}

	gw := &Gateway{
		router:        NewSessionRouter(s),
		delivery:      NewDeliveryManager(),
		pairing:       NewPairingManager(),
		learning:      learningLoop,
		memory:        memManager,
		store:         s,
		engineFactory: engineFactory,
		engines:       make(map[string]*runtime.QueryEngine),
		lastAccess:    make(map[string]time.Time),
		maxIdle:       30 * time.Minute,
		stopCleanup:   make(chan struct{}),
	}

	if jsonlDir != "" {
		gw.jsonlWriter = store.NewJSONLWriter(jsonlDir)
	}

	gw.cronTrigger = NewCronTriggerWithConfig(s, gw, cfg.CronConfig)

	go gw.cleanupLoop()

	return gw
}

func (g *Gateway) HandleMessage(ctx context.Context, userID, platform, content string) (*GatewayResponse, error) {
	session := g.router.Route(userID)

	g.pairing.PairSession(userID, platform, session.ID)

	return g.HandleMessageWithSession(ctx, userID, platform, content, session.ID)
}

func (g *Gateway) HandleMessageWithSession(ctx context.Context, userID, platform, content, sessionID string) (*GatewayResponse, error) {
	engine := g.getOrCreateEngine(sessionID)

	g.memory.FreezeSnapshot(sessionID)

	memCtx := g.memory.BuildSystemContextWithSnapshot(ctx, content, sessionID)

	if memCtx != "" {
		engine.SetSystemPrompt(memCtx)
	}

	if g.learning != nil {
		engine.SetLearningLoop(g.learning)
	}

	// Invalidate cache when memory context changes
	if cacheClient := engine.GetCacheClient(); cacheClient != nil {
		cacheClient.OnMemoryChanged()
	}

	result, err := engine.Query(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("gateway: query: %w", err)
	}

	response := &GatewayResponse{
		Content:   extractContent(result.Message.Content),
		SessionID: sessionID,
		Platform:  platform,
		Duration:  result.Duration,
		Usage: UsageInfo{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			Cost:         result.Cost,
		},
	}

	g.persistMessage(sessionID, "user", content, platform)
	g.persistMessage(sessionID, "assistant", response.Content, platform)

	g.delivery.Deliver(userID, platform, response)

	return response, nil
}

func (g *Gateway) getOrCreateEngine(sessionID string) *runtime.QueryEngine {
	g.mu.RLock()
	if e, ok := g.engines[sessionID]; ok {
		g.mu.RUnlock()
		g.mu.Lock()
		g.lastAccess[sessionID] = time.Now()
		g.mu.Unlock()
		return e
	}
	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()

	if e, ok := g.engines[sessionID]; ok {
		g.lastAccess[sessionID] = time.Now()
		return e
	}

	engine := g.engineFactory()
	g.engines[sessionID] = engine
	g.lastAccess[sessionID] = time.Now()
	return engine
}

func (g *Gateway) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCleanup:
			return
		case <-ticker.C:
			g.evictIdleEngines()
		}
	}
}

func (g *Gateway) evictIdleEngines() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	var evicted []string

	for sessionID, last := range g.lastAccess {
		if now.Sub(last) > g.maxIdle {
			if engine, ok := g.engines[sessionID]; ok {
				engine.Shutdown()
				delete(g.engines, sessionID)
			}
			delete(g.lastAccess, sessionID)
			evicted = append(evicted, sessionID)
		}
	}

	if len(evicted) > 0 {
		slog.Info("gateway: evicted idle engines", "count", len(evicted), "remaining", len(g.engines))
	}
}

func (g *Gateway) persistMessage(sessionID, role, content, source string) {
	msg := &store.Message{
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	if g.store != nil {
		if err := g.store.InsertMessage(context.Background(), msg); err != nil {
			slog.Warn("gateway: failed to persist message to SQLite", "error", err)
			g.persistToJSONL(msg)
		}
		return
	}

	g.persistToJSONL(msg)
}

func (g *Gateway) persistToJSONL(msg *store.Message) {
	if g.jsonlWriter == nil {
		return
	}
	if err := g.jsonlWriter.Append(msg); err != nil {
		slog.Warn("gateway: failed to persist message to JSONL", "error", err)
	}
}

func (g *Gateway) Close() error {
	g.cleanupOnce.Do(func() {
		close(g.stopCleanup)
	})

	if g.cronTrigger != nil {
		g.cronTrigger.Stop()
	}

	g.mu.Lock()
	for _, engine := range g.engines {
		engine.Shutdown()
	}
	g.engines = make(map[string]*runtime.QueryEngine)
	g.lastAccess = make(map[string]time.Time)
	g.mu.Unlock()

	if g.store != nil {
		return g.store.Close()
	}
	return nil
}

func (g *Gateway) GetRouter() *SessionRouter {
	return g.router
}

func (g *Gateway) GetCronTrigger() *CronTrigger {
	return g.cronTrigger
}

func (g *Gateway) GetDelivery() *DeliveryManager {
	return g.delivery
}

func (g *Gateway) GetPairing() *PairingManager {
	return g.pairing
}

func extractContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return fmt.Sprintf("%v", content)
}
