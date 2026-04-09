package transports

import (
	"context"
)

type Message struct {
	Type    string
	Content []byte
}

type Transport interface {
	Connect(ctx context.Context) error
	Send(message []byte) error
	Receive() ([]byte, error)
	Close() error
	IsConnected() bool
}

type TransportType string

const (
	TransportSSE       TransportType = "sse"
	TransportWebSocket TransportType = "websocket"
	TransportHybrid    TransportType = "hybrid"
)

type Config struct {
	Type     TransportType
	Endpoint string
	Headers  map[string]string
	Timeout  int
}

func NewTransport(cfg *Config) Transport {
	switch cfg.Type {
	case TransportSSE:
		return NewSSETransport(cfg)
	case TransportWebSocket:
		return NewWebSocketTransport(cfg)
	case TransportHybrid:
		return NewHybridTransport(cfg)
	default:
		return NewSSETransport(cfg)
	}
}
