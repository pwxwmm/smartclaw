package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Memory struct {
	Key       string     `json:"key"`
	Value     any        `json:"value"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
}

type MemoryStore struct {
	items    map[string]*Memory
	basePath string
	mu       sync.RWMutex
}

func NewMemoryStore() (*MemoryStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	basePath := filepath.Join(home, ".smartclaw", "memory")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	store := &MemoryStore{
		items:    make(map[string]*Memory),
		basePath: basePath,
	}

	store.loadAll()

	return store, nil
}

func (s *MemoryStore) Set(key string, value any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	memory := &Memory{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if ttl > 0 {
		expiresAt := now.Add(ttl)
		memory.ExpiresAt = &expiresAt
	}

	s.items[key] = memory

	return s.saveToFile(memory)
}

func (s *MemoryStore) Get(key string) (any, error) {
	s.mu.RLock()
	memory, ok := s.items[key]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("key not found: %s", key)
	}

	if memory.ExpiresAt != nil && time.Now().After(*memory.ExpiresAt) {
		s.mu.RUnlock()
		s.mu.Lock()
		delete(s.items, key)
		s.mu.Unlock()
		if err := s.deleteFile(key); err != nil {
			slog.Warn("failed to delete memory file", "key", key, "error", err)
		}
		return nil, fmt.Errorf("key expired: %s", key)
	}

	s.mu.RUnlock()
	return memory.Value, nil
}

func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[key]; !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(s.items, key)

	return s.deleteFile(key)
}

func (s *MemoryStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.items))
	for key := range s.items {
		keys = append(keys, key)
	}
	return keys
}

func (s *MemoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = make(map[string]*Memory)

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if err := os.Remove(filepath.Join(s.basePath, entry.Name())); err != nil {
				slog.Warn("failed to remove memory file", "error", err, "name", entry.Name())
			}
		}
	}

	return nil
}

func (s *MemoryStore) Search(query string) []*Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*Memory, 0)
	for _, memory := range s.items {
		if matchesQuery(memory, query) {
			results = append(results, memory)
		}
	}
	return results
}

func (s *MemoryStore) AddTag(key string, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	memory, ok := s.items[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	for _, t := range memory.Tags {
		if t == tag {
			return nil
		}
	}

	memory.Tags = append(memory.Tags, tag)
	memory.UpdatedAt = time.Now()

	return s.saveToFile(memory)
}

func (s *MemoryStore) RemoveTag(key string, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	memory, ok := s.items[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	for i, t := range memory.Tags {
		if t == tag {
			memory.Tags = append(memory.Tags[:i], memory.Tags[i+1:]...)
			memory.UpdatedAt = time.Now()
			return s.saveToFile(memory)
		}
	}

	return nil
}

func (s *MemoryStore) GetByTag(tag string) []*Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*Memory, 0)
	for _, memory := range s.items {
		for _, t := range memory.Tags {
			if t == tag {
				results = append(results, memory)
				break
			}
		}
	}
	return results
}

func (s *MemoryStore) saveToFile(memory *Memory) error {
	data, err := json.Marshal(memory)
	if err != nil {
		return err
	}

	path := filepath.Join(s.basePath, memory.Key+".json")
	return os.WriteFile(path, data, 0644)
}

func (s *MemoryStore) deleteFile(key string) error {
	path := filepath.Join(s.basePath, key+".json")
	return os.Remove(path)
}

func (s *MemoryStore) loadAll() error {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.basePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var memory Memory
		if err := json.Unmarshal(data, &memory); err != nil {
			continue
		}

		if memory.ExpiresAt == nil || time.Now().Before(*memory.ExpiresAt) {
			s.items[memory.Key] = &memory
		}
	}

	return nil
}

func matchesQuery(memory *Memory, query string) bool {
	return memory.Key == query
}

func (m *Memory) Age() time.Duration {
	return time.Since(m.CreatedAt)
}

func (m *Memory) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}
