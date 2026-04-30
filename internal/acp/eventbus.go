package acp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/mcp"
)

type EventType string

const (
	EventToolCall     EventType = "tool_call"
	EventToolResult   EventType = "tool_result"
	EventMemoryUpdate EventType = "memory_update"
	EventSessionStart EventType = "session_start"
	EventSessionEnd   EventType = "session_end"
	EventError        EventType = "error"
	EventNotification EventType = "notification"
)

type Event struct {
	ID        string         `json:"id"`
	Type      EventType      `json:"type"`
	Source    string         `json:"source"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

type EventHandler func(ctx context.Context, event Event) error

type subscription struct {
	id      string
	handler EventHandler
}

// EventBus provides an in-process pub/sub for ACP events.
type EventBus struct {
	handlers map[EventType][]subscription
	mu       sync.RWMutex
	counter  uint64
}

func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]subscription),
	}
}

// Subscribe registers handler for eventType, returns a subscription ID for Unsubscribe.
func (b *EventBus) Subscribe(eventType EventType, handler EventHandler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counter++
	subID := fmt.Sprintf("sub-%d", b.counter)

	b.handlers[eventType] = append(b.handlers[eventType], subscription{
		id:      subID,
		handler: handler,
	})

	return subID
}

func (b *EventBus) Unsubscribe(eventType EventType, subID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.handlers[eventType]
	for i, s := range subs {
		if s.id == subID {
			b.handlers[eventType] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// Publish invokes every handler for the event type. Stops at first error.
func (b *EventBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	subs := make([]subscription, len(b.handlers[event.Type]))
	copy(subs, b.handlers[event.Type])
	b.mu.RUnlock()

	for _, s := range subs {
		if err := s.handler(ctx, event); err != nil {
			return fmt.Errorf("event handler %s failed: %w", s.id, err)
		}
	}
	return nil
}

func (b *EventBus) PublishAsync(ctx context.Context, event Event) {
	b.mu.RLock()
	subs := make([]subscription, len(b.handlers[event.Type]))
	copy(subs, b.handlers[event.Type])
	b.mu.RUnlock()

	for _, s := range subs {
		go func(h EventHandler) {
			if err := h(ctx, event); err != nil {
				slog.Debug("acp eventbus: handler error", "error", err)
			}
		}(s.handler)
	}
}

type eventsSubscribeParams struct {
	EventType string `json:"eventType"`
}

type eventsPublishParams struct {
	Event Event `json:"event"`
}

func (s *ACPServer) handleEventsSubscribe(req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	var params eventsSubscribeParams
	if err := parseParams(req.Params, &params); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	subID := s.eventBus.Subscribe(EventType(params.EventType), func(_ context.Context, _ Event) error {
		return nil
	})

	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"subscriptionId": subID},
	}
}

func (s *ACPServer) handleEventsPublish(ctx context.Context, req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	var params eventsPublishParams
	if err := parseParams(req.Params, &params); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	if params.Event.Timestamp.IsZero() {
		params.Event.Timestamp = time.Now()
	}

	if err := s.eventBus.Publish(ctx, params.Event); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32603, Message: fmt.Sprintf("publish failed: %v", err)},
		}
	}

	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"published": true},
	}
}
