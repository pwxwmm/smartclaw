package contextmgr

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/utils"
)

const cacheTTL = 30 * time.Second

type AssembledContext struct {
	Content    string
	Sources    []string
	TokenCount int
	Items      []ScoredContextItem
}

type ContextManager struct {
	providers  []ContextProvider
	scorer     *RelevanceScorer
	priorities []SourcePriority
	cache      map[string]cacheEntry
	mu         sync.RWMutex
}

type cacheEntry struct {
	items     []ScoredContextItem
	expiresAt time.Time
}

func NewContextManager(providers ...ContextProvider) *ContextManager {
	return &ContextManager{
		providers:  providers,
		scorer:     NewRelevanceScorer(),
		priorities: DefaultSourcePriorities(),
		cache:      make(map[string]cacheEntry),
	}
}

func (cm *ContextManager) SetPriorities(priorities []SourcePriority) {
	cm.priorities = priorities
}

func (cm *ContextManager) AssembleContext(ctx context.Context, query string, maxTokens int) (*AssembledContext, error) {
	if cached := cm.checkCache(query); cached != nil {
		return cm.buildFromCached(cached, maxTokens), nil
	}

	allItems := cm.collectFromProviders(ctx, query, maxTokens)

	scored := cm.scorer.ScoreItems(ctx, allItems, query)

	cm.recordFrequency(scored)

	cm.storeCache(query, scored)

	return cm.buildFromCached(scored, maxTokens), nil
}

func (cm *ContextManager) InvalidateCache() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cache = make(map[string]cacheEntry)
}

func (cm *ContextManager) FormatContext(ac *AssembledContext) string {
	var sb strings.Builder

	bySource := make(map[string][]ScoredContextItem)
	for _, item := range ac.Items {
		bySource[item.Item.Source] = append(bySource[item.Item.Source], item)
	}

	sourceOrder := []string{}
	for _, p := range cm.priorities {
		sourceOrder = append(sourceOrder, p.Source)
	}
	sourceSet := make(map[string]bool)
	for _, s := range sourceOrder {
		sourceSet[s] = true
	}
	for s := range bySource {
		if !sourceSet[s] {
			sourceOrder = append(sourceOrder, s)
		}
	}

	for _, source := range sourceOrder {
		items, ok := bySource[source]
		if !ok || len(items) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", sourceTitle(source)))
		for _, si := range items {
			item := si.Item
			if item.FilePath != "" {
				sb.WriteString(fmt.Sprintf("### %s", item.FilePath))
				if item.StartLine > 0 {
					sb.WriteString(fmt.Sprintf(":%d", item.StartLine))
					if item.EndLine > item.StartLine {
						sb.WriteString(fmt.Sprintf("-%d", item.EndLine))
					}
				}
				sb.WriteString("\n")
			}
			sb.WriteString(item.Content)
			if !strings.HasSuffix(item.Content, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (cm *ContextManager) collectFromProviders(ctx context.Context, query string, maxTokens int) []ContextItem {
	type providerResult struct {
		items []ContextItem
		err   error
		name  string
	}

	results := make(chan providerResult, len(cm.providers))

	for _, p := range cm.providers {
		go func(provider ContextProvider) {
			perProviderBudget := maxTokens / len(cm.providers)
			if perProviderBudget < 500 {
				perProviderBudget = 500
			}
			items, err := provider.Provide(ctx, query, perProviderBudget)
			results <- providerResult{items: items, err: err, name: provider.Name()}
		}(p)
	}

	var allItems []ContextItem
	for range cm.providers {
		r := <-results
		if r.err != nil {
			slog.Warn("contextmgr: provider failed", "provider", r.name, "error", r.err)
			continue
		}
		allItems = append(allItems, r.items...)
	}

	return allItems
}

func (cm *ContextManager) recordFrequency(items []ScoredContextItem) {
	for _, si := range items {
		cm.scorer.RecordSeen(frequencyKey(si.Item))
	}
}

func (cm *ContextManager) checkCache(query string) []ScoredContextItem {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entry, ok := cm.cache[query]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.items
}

func (cm *ContextManager) storeCache(query string, items []ScoredContextItem) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.cache[query] = cacheEntry{
		items:     items,
		expiresAt: time.Now().Add(cacheTTL),
	}
}

func (cm *ContextManager) buildFromCached(scored []ScoredContextItem, maxTokens int) *AssembledContext {
	allocations := Allocate(maxTokens, cm.priorities)
	sourceUsed := make(map[string]int)

	var selected []ScoredContextItem
	var sources []string
	sourceSeen := make(map[string]bool)
	totalTokens := 0

	for _, si := range scored {
		source := si.Item.Source
		budget := allocations[source]
		used := sourceUsed[source]

		if used+si.Item.TokenCount > budget {
			continue
		}

		selected = append(selected, si)
		sourceUsed[source] = used + si.Item.TokenCount
		totalTokens += si.Item.TokenCount

		if !sourceSeen[source] {
			sources = append(sources, source)
			sourceSeen[source] = true
		}
	}

	content := cm.FormatContext(&AssembledContext{
		Items:   selected,
		Sources: sources,
	})

	return &AssembledContext{
		Content:    content,
		Sources:    sources,
		TokenCount: totalTokens,
		Items:      selected,
	}
}

func sourceTitle(source string) string {
	titles := map[string]string{
		"system_prompt": "System Prompt",
		"conversation":  "Conversation",
		"files":         "Files",
		"symbols":       "Symbols",
		"memory":        "Memory",
		"skills":        "Skills",
		"search":        "Search Results",
		"git":           "Git Context",
	}
	if t, ok := titles[source]; ok {
		return t
	}
	return strings.Title(source)
}

func EstimateTokens(s string) int {
	return utils.CountTokens(s)
}
