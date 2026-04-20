package layers

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type EnhancedSearchOptions struct {
	Query     string
	UserID    string
	SessionID string
	Role      string
	Since     time.Time
	Until     time.Time
	Limit     int
	Offset    int
	Summarize bool
	MaxTokens int
}

type RankedResult struct {
	Fragment SessionFragment
	Score    float64
	Snippet  string
}

type SearchSummarizeFunc func(ctx context.Context, fragments []SessionFragment, query string, maxTokens int) (string, error)

type EnhancedSessionSearch struct {
	store      *store.Store
	baseSearch *SessionSearch
	llmFunc    SearchSummarizeFunc
}

func NewEnhancedSessionSearch(s *store.Store, base *SessionSearch, llmFunc SearchSummarizeFunc) *EnhancedSessionSearch {
	return &EnhancedSessionSearch{
		store:      s,
		baseSearch: base,
		llmFunc:    llmFunc,
	}
}

func (ess *EnhancedSessionSearch) Search(ctx context.Context, opts EnhancedSearchOptions) ([]*RankedResult, error) {
	if ess.store == nil || opts.Query == "" {
		return nil, nil
	}

	results, err := ess.store.SearchMessagesAdvanced(opts.Query, store.SearchOptions{
		UserID:    opts.UserID,
		SessionID: opts.SessionID,
		Role:      opts.Role,
		Since:     opts.Since,
		Until:     opts.Until,
		Limit:     opts.Limit,
		Offset:    opts.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("enhanced search: %w", err)
	}

	ranked := make([]*RankedResult, 0, len(results))
	now := time.Now()
	for _, r := range results {
		fragment := SessionFragment{
			SessionID:  r.SessionID,
			Timestamp:  r.Timestamp,
			Role:       r.Role,
			Content:    r.Content,
			Relevance:  r.Rank,
			SourceTurn: int(r.ID),
		}
		score := ess.computeScore(r.Rank, r.Timestamp, r.Role, now)
		ranked = append(ranked, &RankedResult{
			Fragment: fragment,
			Score:    score,
			Snippet:  r.Snippet,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked, nil
}

func (ess *EnhancedSessionSearch) SearchAndSummarize(ctx context.Context, opts EnhancedSearchOptions) (string, []*RankedResult, error) {
	ranked, err := ess.Search(ctx, opts)
	if err != nil {
		return "", nil, err
	}

	if len(ranked) == 0 {
		return "", ranked, nil
	}

	if !opts.Summarize || ess.llmFunc == nil {
		summary := formatFragments(extractFragments(ranked), opts.MaxTokens)
		return summary, ranked, nil
	}

	summary, err := ess.llmFunc(ctx, extractFragments(ranked), opts.Query, opts.MaxTokens)
	if err != nil {
		fallback := formatFragments(extractFragments(ranked), opts.MaxTokens)
		return fallback, ranked, nil
	}

	return summary, ranked, nil
}

func (ess *EnhancedSessionSearch) CrossSessionSearch(ctx context.Context, query string, userID string, limit int) ([]*RankedResult, error) {
	if ess.store == nil || query == "" {
		return nil, nil
	}

	opts := EnhancedSearchOptions{
		Query:  query,
		UserID: userID,
		Limit:  limit * 2,
	}

	ranked, err := ess.Search(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("cross-session search: %w", err)
	}

	ranked = deduplicateBySession(ranked)

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	return ranked, nil
}

const recencyHalfLife = 7 * 24 * time.Hour

func (ess *EnhancedSessionSearch) computeScore(ftsRank float64, timestamp time.Time, role string, now time.Time) float64 {
	baseScore := -ftsRank
	if baseScore <= 0 {
		baseScore = 0.1
	}

	recencyBoost := 1.0
	if !timestamp.IsZero() {
		age := now.Sub(timestamp)
		if age < 0 {
			age = 0
		}
		recencyBoost = math.Exp2(-float64(age) / float64(recencyHalfLife))
	}

	roleWeight := 1.0
	if role == "user" {
		roleWeight = 1.2
	} else if role == "assistant" {
		roleWeight = 0.9
	}

	return baseScore * recencyBoost * roleWeight
}

func deduplicateBySession(results []*RankedResult) []*RankedResult {
	seen := make(map[string]bool)
	deduped := make([]*RankedResult, 0, len(results))

	for _, r := range results {
		key := r.Fragment.SessionID + ":" + r.Fragment.Content
		if len(key) > 256 {
			key = key[:256]
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, r)
	}

	return deduped
}

func extractFragments(ranked []*RankedResult) []SessionFragment {
	fragments := make([]SessionFragment, 0, len(ranked))
	for _, r := range ranked {
		fragments = append(fragments, r.Fragment)
	}
	return fragments
}
