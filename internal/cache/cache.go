package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CacheEntry struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	HitCount  int         `json:"hit_count"`
}

type Cache struct {
	items    map[string]*CacheEntry
	basePath string
	maxSize  int64
	mu       sync.RWMutex
}

func NewCache() (*Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	basePath := filepath.Join(home, ".smartclaw", "cache")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	cache := &Cache{
		items:    make(map[string]*CacheEntry),
		basePath: basePath,
		maxSize:  100 * 1024 * 1024,
	}

	cache.loadAll()

	return cache, nil
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		HitCount:  0,
	}

	c.items[key] = entry

	return c.saveToFile(entry)
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(c.items, key)
		_ = c.deleteFile(key)
		return nil, false
	}

	entry.HitCount++
	_ = c.saveToFile(entry)

	return entry.Value, true
}

func (c *Cache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)

	return c.deleteFile(key)
}

func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheEntry)

	entries, err := ioutil.ReadDir(c.basePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			_ = os.Remove(filepath.Join(c.basePath, entry.Name()))
		}
	}

	return nil
}

func (c *Cache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return false
	}

	return time.Now().Before(entry.ExpiresAt)
}

func (c *Cache) GetOrSet(key string, fn func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	if value, ok := c.Get(key); ok {
		return value, nil
	}

	value, err := fn()
	if err != nil {
		return nil, err
	}

	if err := c.Set(key, value, ttl); err != nil {
		return nil, err
	}

	return value, nil
}

func (c *Cache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalHits int
	var totalSize int64

	for _, entry := range c.items {
		totalHits += entry.HitCount
		data, _ := json.Marshal(entry.Value)
		totalSize += int64(len(data))
	}

	return map[string]interface{}{
		"items":      len(c.items),
		"total_hits": totalHits,
		"total_size": totalSize,
	}
}

func (c *Cache) Cleanup() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.items {
		if now.After(entry.ExpiresAt) {
			delete(c.items, key)
			_ = c.deleteFile(key)
		}
	}

	return nil
}

func (c *Cache) HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func (c *Cache) saveToFile(entry *CacheEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	hashedKey := c.HashKey(entry.Key)
	path := filepath.Join(c.basePath, hashedKey+".json")
	return ioutil.WriteFile(path, data, 0644)
}

func (c *Cache) deleteFile(key string) error {
	hashedKey := c.HashKey(key)
	path := filepath.Join(c.basePath, hashedKey+".json")
	return os.Remove(path)
}

func (c *Cache) loadAll() error {
	entries, err := ioutil.ReadDir(c.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(c.basePath, entry.Name())
		data, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			continue
		}

		if now.Before(cacheEntry.ExpiresAt) {
			c.items[cacheEntry.Key] = &cacheEntry
		} else {
			_ = os.Remove(path)
		}
	}

	return nil
}
