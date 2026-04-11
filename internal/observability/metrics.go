package observability

import (
	"sync"
	"sync/atomic"
	"time"
)

type MetricsSnapshot struct {
	QueryCount        int64
	QueryTotalTime    time.Duration
	CacheHits         int64
	CacheMisses       int64
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCacheRead    int64
	TotalCacheCreate  int64
	ToolExecutions    map[string]ToolStats
	MemoryLayerSizes  map[string]int
	ModelQueryCounts  map[string]int64
}

type ToolStats struct {
	Count    int64
	Errors   int64
	Duration time.Duration
}

type Metrics struct {
	queryCount        atomic.Int64
	queryTotalTimeNs  atomic.Int64
	cacheHits         atomic.Int64
	cacheMisses       atomic.Int64
	totalInputTokens  atomic.Int64
	totalOutputTokens atomic.Int64
	totalCacheRead    atomic.Int64
	totalCacheCreate  atomic.Int64

	toolMu      sync.RWMutex
	toolStats   map[string]*ToolStats
	layerMu     sync.RWMutex
	layerSizes  map[string]int
	modelMu     sync.RWMutex
	modelCounts map[string]int64
}

var DefaultMetrics = NewMetrics()

func NewMetrics() *Metrics {
	return &Metrics{
		toolStats:   make(map[string]*ToolStats),
		layerSizes:  make(map[string]int),
		modelCounts: make(map[string]int64),
	}
}

func (m *Metrics) RecordQueryDuration(duration time.Duration, model string) {
	m.queryCount.Add(1)
	m.queryTotalTimeNs.Add(int64(duration))

	m.modelMu.Lock()
	m.modelCounts[model]++
	m.modelMu.Unlock()
}

func (m *Metrics) RecordCacheHit(hit bool) {
	if hit {
		m.cacheHits.Add(1)
	} else {
		m.cacheMisses.Add(1)
	}
}

func (m *Metrics) RecordTokenUsage(input, output, cacheRead, cacheCreation int, model string) {
	m.totalInputTokens.Add(int64(input))
	m.totalOutputTokens.Add(int64(output))
	m.totalCacheRead.Add(int64(cacheRead))
	m.totalCacheCreate.Add(int64(cacheCreation))
}

func (m *Metrics) RecordMemoryLayerSize(layerName string, chars int) {
	m.layerMu.Lock()
	m.layerSizes[layerName] = chars
	m.layerMu.Unlock()
}

func (m *Metrics) RecordToolExecution(toolName string, duration time.Duration, success bool) {
	m.toolMu.Lock()
	defer m.toolMu.Unlock()

	stats, ok := m.toolStats[toolName]
	if !ok {
		stats = &ToolStats{}
		m.toolStats[toolName] = stats
	}
	stats.Count++
	stats.Duration += duration
	if !success {
		stats.Errors++
	}
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	m.toolMu.RLock()
	toolCopy := make(map[string]ToolStats, len(m.toolStats))
	for k, v := range m.toolStats {
		toolCopy[k] = ToolStats{
			Count:    v.Count,
			Errors:   v.Errors,
			Duration: v.Duration,
		}
	}
	m.toolMu.RUnlock()

	m.layerMu.RLock()
	layerCopy := make(map[string]int, len(m.layerSizes))
	for k, v := range m.layerSizes {
		layerCopy[k] = v
	}
	m.layerMu.RUnlock()

	m.modelMu.RLock()
	modelCopy := make(map[string]int64, len(m.modelCounts))
	for k, v := range m.modelCounts {
		modelCopy[k] = v
	}
	m.modelMu.RUnlock()

	return MetricsSnapshot{
		QueryCount:        m.queryCount.Load(),
		QueryTotalTime:    time.Duration(m.queryTotalTimeNs.Load()),
		CacheHits:         m.cacheHits.Load(),
		CacheMisses:       m.cacheMisses.Load(),
		TotalInputTokens:  m.totalInputTokens.Load(),
		TotalOutputTokens: m.totalOutputTokens.Load(),
		TotalCacheRead:    m.totalCacheRead.Load(),
		TotalCacheCreate:  m.totalCacheCreate.Load(),
		ToolExecutions:    toolCopy,
		MemoryLayerSizes:  layerCopy,
		ModelQueryCounts:  modelCopy,
	}
}

func (m *Metrics) Reset() {
	m.queryCount.Store(0)
	m.queryTotalTimeNs.Store(0)
	m.cacheHits.Store(0)
	m.cacheMisses.Store(0)
	m.totalInputTokens.Store(0)
	m.totalOutputTokens.Store(0)
	m.totalCacheRead.Store(0)
	m.totalCacheCreate.Store(0)

	m.toolMu.Lock()
	m.toolStats = make(map[string]*ToolStats)
	m.toolMu.Unlock()

	m.layerMu.Lock()
	m.layerSizes = make(map[string]int)
	m.layerMu.Unlock()

	m.modelMu.Lock()
	m.modelCounts = make(map[string]int64)
	m.modelMu.Unlock()
}

// Convenience functions that delegate to DefaultMetrics

func RecordQueryDuration(duration time.Duration, model string) {
	DefaultMetrics.RecordQueryDuration(duration, model)
}

func RecordCacheHit(hit bool) {
	DefaultMetrics.RecordCacheHit(hit)
}

func RecordTokenUsage(input, output, cacheRead, cacheCreation int, model string) {
	DefaultMetrics.RecordTokenUsage(input, output, cacheRead, cacheCreation, model)
}

func RecordMemoryLayerSize(layerName string, chars int) {
	DefaultMetrics.RecordMemoryLayerSize(layerName, chars)
}

func RecordToolExecution(toolName string, duration time.Duration, success bool) {
	DefaultMetrics.RecordToolExecution(toolName, duration, success)
}
