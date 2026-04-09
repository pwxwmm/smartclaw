package transports

import (
	"context"
	"fmt"
	"sync"
)

type HybridTransport struct {
	sse  *SSETransport
	ws   *WebSocketTransport
	cfg  *Config
	mu   sync.Mutex
	mode string
}

func NewHybridTransport(cfg *Config) *HybridTransport {
	return &HybridTransport{
		cfg:  cfg,
		mode: "sse",
	}
}

func (t *HybridTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sse = NewSSETransport(t.cfg)
	if err := t.sse.Connect(ctx); err != nil {
		return fmt.Errorf("SSE connect failed: %w", err)
	}

	t.ws = NewWebSocketTransport(t.cfg)
	if err := t.ws.Connect(ctx); err != nil {
		t.sse.Close()
		return fmt.Errorf("WebSocket connect failed: %w", err)
	}

	return nil
}

func (t *HybridTransport) Send(message []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ws != nil && t.ws.IsConnected() {
		return t.ws.Send(message)
	}

	return fmt.Errorf("no send transport available")
}

func (t *HybridTransport) Receive() ([]byte, error) {
	t.mu.Lock()

	if t.sse != nil && t.sse.IsConnected() {
		t.mu.Unlock()
		return t.sse.Receive()
	}

	t.mu.Unlock()

	if t.ws != nil && t.ws.IsConnected() {
		return t.ws.Receive()
	}

	return nil, fmt.Errorf("no receive transport available")
}

func (t *HybridTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var errs []error

	if t.sse != nil {
		if err := t.sse.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if t.ws != nil {
		if err := t.ws.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

func (t *HybridTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return (t.sse != nil && t.sse.IsConnected()) || (t.ws != nil && t.ws.IsConnected())
}

func (t *HybridTransport) SwitchMode(mode string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if mode != "sse" && mode != "websocket" {
		return fmt.Errorf("invalid mode: %s", mode)
	}

	t.mode = mode
	return nil
}

func (t *HybridTransport) GetMode() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mode
}
