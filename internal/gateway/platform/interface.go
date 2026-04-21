package platform

import (
	"context"

	"github.com/instructkr/smartclaw/internal/gateway"
)

// PlatformAdapter defines the interface that all platform adapters must implement.
// Each adapter handles sending responses to users and receiving incoming messages
// for a specific messaging platform.
type PlatformAdapter interface {
	// Name returns the platform identifier (e.g. "discord", "whatsapp").
	Name() string

	// Send delivers a gateway response to the specified user on this platform.
	Send(userID string, response *gateway.GatewayResponse) error

	// Start begins receiving messages from the platform. Blocks until the
	// context is cancelled or Stop is called.
	Start(ctx context.Context) error

	// Stop performs a graceful shutdown of the adapter.
	Stop() error
}
