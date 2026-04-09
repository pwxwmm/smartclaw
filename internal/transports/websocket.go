package transports

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type WebSocketTransport struct {
	endpoint  string
	headers   map[string]string
	connected bool
	mu        sync.Mutex
	sendChan  chan []byte
	recvChan  chan []byte
	errChan   chan error
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewWebSocketTransport(cfg *Config) *WebSocketTransport {
	return &WebSocketTransport{
		endpoint: cfg.Endpoint,
		headers:  cfg.Headers,
		sendChan: make(chan []byte, 100),
		recvChan: make(chan []byte, 100),
		errChan:  make(chan error, 1),
	}
}

func (t *WebSocketTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.ctx, t.cancel = context.WithCancel(ctx)
	t.connected = true

	return nil
}

func (t *WebSocketTransport) Send(message []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("not connected")
	}

	select {
	case t.sendChan <- message:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	case <-t.ctx.Done():
		return t.ctx.Err()
	}
}

func (t *WebSocketTransport) Receive() ([]byte, error) {
	select {
	case msg := <-t.recvChan:
		return msg, nil
	case err := <-t.errChan:
		return nil, err
	case <-t.ctx.Done():
		return nil, t.ctx.Err()
	}
}

func (t *WebSocketTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	t.connected = false
	close(t.sendChan)
	close(t.recvChan)
	close(t.errChan)

	return nil
}

func (t *WebSocketTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

func (t *WebSocketTransport) Headers() http.Header {
	h := make(http.Header)
	for k, v := range t.headers {
		h.Set(k, v)
	}
	return h
}
