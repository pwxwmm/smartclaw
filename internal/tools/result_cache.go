package tools

import (
	"container/list"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type CacheKey struct {
	Tool      string
	InputHash string
}

type CacheEntry struct {
	Key      CacheKey
	Result   any
	CachedAt time.Time
	DepFiles map[string]time.Time
}

type ResultCache struct {
	mu      sync.RWMutex
	cache   map[CacheKey]*list.Element
	lru     *list.List
	maxSize int
	ttl     time.Duration
}

func NewResultCache(maxSize int, ttl time.Duration) *ResultCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ResultCache{
		cache:   make(map[CacheKey]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (rc *ResultCache) Get(tool string, input map[string]any) (any, bool) {
	key := makeKey(tool, input)

	rc.mu.RLock()
	elem, ok := rc.cache[key]
	rc.mu.RUnlock()

	if !ok {
		return nil, false
	}

	entry := elem.Value.(*CacheEntry)

	if time.Since(entry.CachedAt) > rc.ttl {
		rc.mu.Lock()
		rc.removeElement(elem)
		rc.mu.Unlock()
		return nil, false
	}

	for path, cachedMtime := range entry.DepFiles {
		info, err := os.Stat(path)
		if err != nil || info.ModTime() != cachedMtime {
			rc.mu.Lock()
			rc.removeElement(elem)
			rc.mu.Unlock()
			return nil, false
		}
	}

	rc.mu.Lock()
	rc.lru.MoveToFront(elem)
	rc.mu.Unlock()

	return entry.Result, true
}

func (rc *ResultCache) Set(tool string, input map[string]any, result any, depFiles []string) {
	key := makeKey(tool, input)

	fileMtimes := make(map[string]time.Time)
	for _, f := range depFiles {
		info, err := os.Stat(f)
		if err == nil {
			fileMtimes[f] = info.ModTime()
		}
	}

	entry := &CacheEntry{
		Key:      key,
		Result:   result,
		CachedAt: time.Now(),
		DepFiles: fileMtimes,
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	if elem, ok := rc.cache[key]; ok {
		rc.lru.MoveToFront(elem)
		elem.Value = entry
		return
	}

	elem := rc.lru.PushFront(entry)
	rc.cache[key] = elem

	for rc.lru.Len() > rc.maxSize {
		oldest := rc.lru.Back()
		if oldest != nil {
			rc.removeElement(oldest)
		}
	}
}

func (rc *ResultCache) Invalidate(paths []string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	pathSet := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		pathSet[p] = struct{}{}
	}

	var toRemove []*list.Element
	for _, elem := range rc.cache {
		entry := elem.Value.(*CacheEntry)
		for depPath := range entry.DepFiles {
			if _, hit := pathSet[depPath]; hit {
				toRemove = append(toRemove, elem)
				break
			}
		}
	}

	for _, elem := range toRemove {
		rc.removeElement(elem)
	}

	if len(toRemove) > 0 {
		slog.Debug("tool cache: invalidated entries", "count", len(toRemove), "files", len(paths))
	}
}

func (rc *ResultCache) InvalidateAll() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = make(map[CacheKey]*list.Element)
	rc.lru.Init()
}

func (rc *ResultCache) Size() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.cache)
}

func (rc *ResultCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = make(map[CacheKey]*list.Element)
	rc.lru.Init()
}

func (rc *ResultCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*CacheEntry)
	delete(rc.cache, entry.Key)
	rc.lru.Remove(elem)
}

func makeKey(tool string, input map[string]any) CacheKey {
	data, err := json.Marshal(input)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", input))
	}
	hash := sha256.Sum256(data)
	return CacheKey{
		Tool:      tool,
		InputHash: fmt.Sprintf("%x", hash[:16]),
	}
}
