package gateway

import (
	"log/slog"
	"sync"
)

type DeliveryManager struct {
	adapters map[string]PlatformAdapter
	mu       sync.RWMutex
}

type PlatformAdapter interface {
	Send(userID string, response *GatewayResponse) error
	Name() string
}

func NewDeliveryManager() *DeliveryManager {
	return &DeliveryManager{
		adapters: make(map[string]PlatformAdapter),
	}
}

func (dm *DeliveryManager) RegisterAdapter(adapter PlatformAdapter) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.adapters[adapter.Name()] = adapter
	slog.Info("delivery: registered platform adapter", "platform", adapter.Name())
}

func (dm *DeliveryManager) Deliver(userID, platform string, response *GatewayResponse) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if adapter, ok := dm.adapters[platform]; ok {
		if err := adapter.Send(userID, response); err != nil {
			slog.Warn("delivery: failed to deliver", "platform", platform, "error", err)
		}
	}
}

func (dm *DeliveryManager) GetAdapter(platform string) PlatformAdapter {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.adapters[platform]
}

func (dm *DeliveryManager) ListPlatforms() []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	platforms := make([]string, 0, len(dm.adapters))
	for name := range dm.adapters {
		platforms = append(platforms, name)
	}
	return platforms
}
