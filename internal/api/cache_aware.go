package api

import (
	"context"
	"sync"
)

type CacheAwareClient struct {
	client       *Client
	lastSystem   string
	cacheValid   bool
	cacheEnabled bool
	mu           sync.RWMutex
}

func NewCacheAwareClient(client *Client) *CacheAwareClient {
	return &CacheAwareClient{
		client:       client,
		cacheEnabled: true,
	}
}

func (cac *CacheAwareClient) CreateMessage(ctx context.Context, messages []Message, system string) (*MessageResponse, error) {
	cac.mu.Lock()
	if cac.lastSystem != system {
		cac.cacheValid = false
		cac.lastSystem = system
	}
	cacheEnabled := cac.cacheEnabled
	cac.mu.Unlock()

	var systemParam interface{}
	if system != "" {
		cc := &CacheControl{Type: "ephemeral"}
		if !cacheEnabled {
			cc = nil
		}
		systemParam = []SystemBlock{
			{
				Type:         "text",
				Text:         system,
				CacheControl: cc,
			},
		}
	}

	resp, err := cac.client.CreateMessageWithSystem(ctx, messages, systemParam)
	if err != nil {
		return nil, err
	}

	if resp.Usage.CacheRead > 0 {
		cac.SetCacheValid()
	}

	return resp, nil
}

func (cac *CacheAwareClient) MarkCacheInvalid() {
	cac.mu.Lock()
	cac.cacheValid = false
	cac.mu.Unlock()
}

func (cac *CacheAwareClient) IsCacheValid() bool {
	cac.mu.RLock()
	defer cac.mu.RUnlock()
	return cac.cacheValid
}

func (cac *CacheAwareClient) SetCacheValid() {
	cac.mu.Lock()
	cac.cacheValid = true
	cac.mu.Unlock()
}

func (cac *CacheAwareClient) GetClient() *Client {
	return cac.client
}

func (cac *CacheAwareClient) OnMemoryChanged() {
	cac.MarkCacheInvalid()
}

func (cac *CacheAwareClient) OnModelChanged() {
	cac.MarkCacheInvalid()
}

func (cac *CacheAwareClient) OnContextFileChanged() {
	cac.MarkCacheInvalid()
}

func (cac *CacheAwareClient) SetCacheEnabled(enabled bool) {
	cac.mu.Lock()
	defer cac.mu.Unlock()
	cac.cacheEnabled = enabled
}

func (cac *CacheAwareClient) IsCacheEnabled() bool {
	cac.mu.RLock()
	defer cac.mu.RUnlock()
	return cac.cacheEnabled
}

func (cac *CacheAwareClient) GetCacheStats() CacheStats {
	cac.mu.RLock()
	defer cac.mu.RUnlock()
	return CacheStats{
		Valid:   cac.cacheValid,
		Enabled: cac.cacheEnabled,
	}
}

type CacheStats struct {
	Valid   bool
	Enabled bool
}
