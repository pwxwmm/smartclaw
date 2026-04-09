package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/store"
)

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
	cronTrigger *CronTrigger
	learning    *learning.LearningLoop
	memory      *memory.MemoryManager
	store       *store.Store

	engineFactory EngineFactory
	mu            sync.RWMutex
	engines       map[string]*runtime.QueryEngine
}

type EngineFactory func() *runtime.QueryEngine

func NewGateway(engineFactory EngineFactory, memManager *memory.MemoryManager, learningLoop *learning.LearningLoop) *Gateway {
	s := memManager.GetStore()

	gw := &Gateway{
		router:        NewSessionRouter(s),
		delivery:      NewDeliveryManager(),
		learning:      learningLoop,
		memory:        memManager,
		store:         s,
		engineFactory: engineFactory,
		engines:       make(map[string]*runtime.QueryEngine),
	}

	gw.cronTrigger = NewCronTrigger(s, gw)

	return gw
}

func (g *Gateway) HandleMessage(ctx context.Context, userID, platform, content string) (*GatewayResponse, error) {
	session := g.router.Route(userID)

	engine := g.getOrCreateEngine(session.ID)

	memCtx := g.memory.BuildSystemContext(ctx, content)

	if memCtx != "" {
		engine.SetSystemPrompt(memCtx)
	}

	if g.learning != nil {
		engine.SetLearningLoop(g.learning)
	}

	result, err := engine.Query(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("gateway: query: %w", err)
	}

	response := &GatewayResponse{
		Content:   extractContent(result.Message.Content),
		SessionID: session.ID,
		Platform:  platform,
		Duration:  result.Duration,
		Usage: UsageInfo{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			Cost:         result.Cost,
		},
	}

	g.persistMessage(session.ID, "user", content, platform)
	g.persistMessage(session.ID, "assistant", response.Content, platform)

	g.delivery.Deliver(userID, platform, response)

	return response, nil
}

func (g *Gateway) getOrCreateEngine(sessionID string) *runtime.QueryEngine {
	g.mu.RLock()
	if e, ok := g.engines[sessionID]; ok {
		g.mu.RUnlock()
		return e
	}
	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()

	if e, ok := g.engines[sessionID]; ok {
		return e
	}

	engine := g.engineFactory()
	g.engines[sessionID] = engine
	return engine
}

func (g *Gateway) persistMessage(sessionID, role, content, source string) {
	if g.store == nil {
		return
	}

	msg := &store.Message{
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	if _, err := g.store.InsertMessage(msg); err != nil {
		slog.Warn("gateway: failed to persist message", "error", err)
	}
}

func (g *Gateway) Close() error {
	if g.cronTrigger != nil {
		g.cronTrigger.Stop()
	}
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

func extractContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
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
