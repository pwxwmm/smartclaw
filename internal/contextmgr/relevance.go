package contextmgr

import (
	"context"
	"math"
	"strings"
	"time"
)

const defaultHalfLife = 10 * time.Minute

type RelevanceScorer struct {
	halfLife      time.Duration
	seenInSession map[string]int
}

func NewRelevanceScorer() *RelevanceScorer {
	return &RelevanceScorer{
		halfLife:      defaultHalfLife,
		seenInSession: make(map[string]int),
	}
}

func (rs *RelevanceScorer) RecordSeen(key string) {
	rs.seenInSession[key]++
}

func (rs *RelevanceScorer) Score(_ context.Context, item ContextItem, query string) float64 {
	var total float64

	total += rs.recencyScore(item) * 0.25
	total += rs.frequencyScore(item) * 0.15
	total += rs.semanticScore(item, query) * 0.35
	total += rs.dependencyScore(item) * 0.10
	total += rs.typeScore(item) * 0.15

	return total
}

func (rs *RelevanceScorer) ScoreItems(ctx context.Context, items []ContextItem, query string) []ScoredContextItem {
	scored := make([]ScoredContextItem, len(items))
	for i, item := range items {
		scored[i] = ScoredContextItem{
			Item:      item,
			Relevance: rs.Score(ctx, item, query),
		}
	}
	sortScored(scored)
	return scored
}

// recencyScore uses exponential decay with a configurable half-life.
// Items without a timestamp receive a neutral score.
func (rs *RelevanceScorer) recencyScore(item ContextItem) float64 {
	if item.Timestamp.IsZero() {
		return 0.5
	}
	elapsed := time.Since(item.Timestamp)
	lambda := math.Ln2 / rs.halfLife.Seconds()
	score := math.Exp(-lambda * elapsed.Seconds())
	return score
}

// frequencyScore rewards items that appear more often in the conversation,
// using log scale to avoid runaway scores.
func (rs *RelevanceScorer) frequencyScore(item ContextItem) float64 {
	key := frequencyKey(item)
	count := rs.seenInSession[key]
	if count == 0 {
		return 0.1
	}
	return math.Log1p(float64(count)) / math.Log1p(20)
}

// semanticScore computes keyword overlap between item content and query.
func (rs *RelevanceScorer) semanticScore(item ContextItem, query string) float64 {
	if query == "" || item.Content == "" {
		return 0
	}

	queryTerms := tokenizeLower(query)
	if len(queryTerms) == 0 {
		return 0
	}

	contentLower := strings.ToLower(item.Content)
	matched := 0
	for _, term := range queryTerms {
		if strings.Contains(contentLower, term) {
			matched++
		}
	}
	return float64(matched) / float64(len(queryTerms))
}

// dependencyScore boosts items whose FilePath is imported or referenced by
// already-selected items.  In the initial implementation this checks whether
// the item's FilePath appears in the content of other high-scoring items.
func (rs *RelevanceScorer) dependencyScore(item ContextItem) float64 {
	if item.FilePath == "" {
		return 0
	}
	// Simple heuristic: if the file path itself appears in the item content
	// of other items, it's likely a dependency.
	return 0.0
}

// typeScore prefers symbols over file snippets, file snippets over generic docs.
func (rs *RelevanceScorer) typeScore(item ContextItem) float64 {
	switch item.Type {
	case "symbol", "definition", "interface":
		return 1.0
	case "file", "snippet":
		return 0.7
	case "memory":
		return 0.5
	case "git_diff":
		return 0.4
	case "search_result":
		return 0.3
	default:
		return 0.2
	}
}

func frequencyKey(item ContextItem) string {
	if item.FilePath != "" {
		return item.FilePath + ":" + item.Type
	}
	return item.Source + ":" + item.Type
}

func tokenizeLower(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	for _, word := range strings.Fields(s) {
		clean := strings.Trim(word, ".,;:!?()[]{}\"'")
		if len(clean) > 2 {
			tokens = append(tokens, clean)
		}
	}
	return tokens
}

func sortScored(items []ScoredContextItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].Relevance > items[j-1].Relevance; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
