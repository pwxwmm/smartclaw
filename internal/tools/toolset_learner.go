package tools

import (
	"sort"
	"sync"
	"time"
)

type ToolUsageLearner struct {
	mu       sync.RWMutex
	usage    map[float64]map[string]int
	total    map[float64]int
	lastAdj  time.Time
	interval time.Duration
}

func NewToolUsageLearner() *ToolUsageLearner {
	return &ToolUsageLearner{
		usage:    make(map[float64]map[string]int),
		total:    make(map[float64]int),
		interval: 5 * time.Minute,
		lastAdj:  time.Now(),
	}
}

func (l *ToolUsageLearner) RecordUsage(toolName string, complexity float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket := roundBucket(complexity)
	if l.usage[bucket] == nil {
		l.usage[bucket] = make(map[string]int)
	}
	l.usage[bucket][toolName]++
	l.total[bucket]++
}

func (l *ToolUsageLearner) GetSuggestedTools(complexity float64, minUsageRatio float64) []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	bucket := roundBucket(complexity)
	usageMap, ok := l.usage[bucket]
	if !ok || l.total[bucket] == 0 {
		return nil
	}

	threshold := float64(l.total[bucket]) * minUsageRatio
	type toolCount struct {
		name  string
		count int
	}
	var candidates []toolCount
	for name, count := range usageMap {
		if float64(count) >= threshold {
			candidates = append(candidates, toolCount{name, count})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})

	result := make([]string, len(candidates))
	for i, c := range candidates {
		result[i] = c.name
	}
	return result
}

func (l *ToolUsageLearner) ShouldAdjust() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return time.Since(l.lastAdj) > l.interval
}

func (l *ToolUsageLearner) MarkAdjusted() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastAdj = time.Now()
}

func (l *ToolUsageLearner) GetUsageStats() map[float64]map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[float64]map[string]int, len(l.usage))
	for bucket, usageMap := range l.usage {
		result[bucket] = make(map[string]int, len(usageMap))
		for k, v := range usageMap {
			result[bucket][k] = v
		}
	}
	return result
}

func roundBucket(complexity float64) float64 {
	return float64(int(complexity*4+0.5)) / 4
}
