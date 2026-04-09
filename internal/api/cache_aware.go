package api

import "sync"

type CacheAwareClient struct {
	client     *Client
	lastSystem string
	cacheValid bool
	mu         sync.RWMutex
}

func NewCacheAwareClient(client *Client) *CacheAwareClient {
	return &CacheAwareClient{client: client}
}

func (cac *CacheAwareClient) CreateMessage(messages []Message, system string) (*MessageResponse, error) {
	cac.mu.Lock()
	if cac.lastSystem != system {
		cac.cacheValid = false
		cac.lastSystem = system
	}
	cac.mu.Unlock()

	return cac.client.CreateMessage(messages, system)
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
