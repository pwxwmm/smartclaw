package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

const summaryThreshold = 3000

// SearchResultWithSummary wraps search results with an optional LLM-generated summary.
type SearchResultWithSummary struct {
	Query      string                    `json:"query"`
	Results    []ConversationSearchHit   `json:"results"`
	Summary    string                    `json:"summary,omitempty"`
	TotalChars int                       `json:"total_chars"`
}

// ConversationSearchHit is a single hit from a cross-session conversation search.
type ConversationSearchHit struct {
	SessionID string  `json:"session_id"`
	Role      string  `json:"role"`
	Content   string  `json:"content"`
	Timestamp string  `json:"timestamp"`
	Snippet   string  `json:"snippet"`
	Rank      float64 `json:"rank"`
}

// SearchWithSummary performs an FTS5 search across all historical conversations.
// When the combined content of results exceeds 3000 characters and an LLMClient
// is provided, it generates a concise summary of the search results.
func (mm *MemoryManager) SearchWithSummary(ctx context.Context, query string, limit int, llmClient learning.LLMClient) (*SearchResultWithSummary, error) {
	if mm.dataStore == nil {
		return nil, fmt.Errorf("memory: search with summary: store not available")
	}

	if limit <= 0 {
		limit = 10
	}

	opts := store.SearchOptions{
		Limit: limit,
	}

	results, err := mm.dataStore.SearchMessagesAdvanced(query, opts)
	if err != nil {
		return nil, fmt.Errorf("memory: search with summary: %w", err)
	}

	hits := make([]ConversationSearchHit, 0, len(results))
	totalChars := 0
	for _, r := range results {
		hit := ConversationSearchHit{
			SessionID: r.SessionID,
			Role:      r.Role,
			Content:   r.Content,
			Timestamp: r.Timestamp.Format(time.RFC3339),
			Snippet:   r.Snippet,
			Rank:      r.Rank,
		}
		totalChars += len(r.Content)
		hits = append(hits, hit)
	}

	srw := &SearchResultWithSummary{
		Query:      query,
		Results:    hits,
		TotalChars: totalChars,
	}

	if totalChars > summaryThreshold && llmClient != nil {
		summary, err := summarizeResults(ctx, llmClient, hits, query)
		if err != nil {
			slog.Warn("memory: search summary generation failed", "error", err)
		} else {
			srw.Summary = summary
		}
	}

	return srw, nil
}

// SearchAdvancedWithSummary uses the enhanced session search layer for ranked results
// with optional LLM summarization.
func (mm *MemoryManager) SearchAdvancedWithSummary(ctx context.Context, query string, opts layers.EnhancedSearchOptions, llmClient learning.LLMClient) (*SearchResultWithSummary, error) {
	if mm.sessionSearch == nil {
		return nil, fmt.Errorf("memory: advanced search: session search not available")
	}

	ranked, err := mm.sessionSearch.SearchAdvanced(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("memory: advanced search with summary: %w", err)
	}

	hits := make([]ConversationSearchHit, 0, len(ranked))
	totalChars := 0
	for _, r := range ranked {
		hit := ConversationSearchHit{
			SessionID: r.Fragment.SessionID,
			Role:      r.Fragment.Role,
			Content:   r.Fragment.Content,
			Timestamp: r.Fragment.Timestamp.Format(time.RFC3339),
			Snippet:   r.Snippet,
			Rank:      r.Score,
		}
		totalChars += len(r.Fragment.Content)
		hits = append(hits, hit)
	}

	srw := &SearchResultWithSummary{
		Query:      query,
		Results:    hits,
		TotalChars: totalChars,
	}

	if totalChars > summaryThreshold && llmClient != nil {
		summary, err := summarizeResults(ctx, llmClient, hits, query)
		if err != nil {
			slog.Warn("memory: advanced search summary generation failed", "error", err)
		} else {
			srw.Summary = summary
		}
	}

	return srw, nil
}

func summarizeResults(ctx context.Context, llmClient learning.LLMClient, hits []ConversationSearchHit, query string) (string, error) {
	var fragments []string
	for _, h := range hits {
		content := h.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fragments = append(fragments, fmt.Sprintf("[%s %s]: %s", h.Role, h.Timestamp, content))
	}

	userPrompt := fmt.Sprintf(
		"Summarize these conversation fragments about %q. Preserve key facts, decisions, and outcomes. Be concise.\n\n%s",
		query,
		strings.Join(fragments, "\n---\n"),
	)

	systemPrompt := "You are a search result summarizer. Provide a concise summary of the relevant information found in conversation fragments. Focus on key facts, decisions, and outcomes."

	return llmClient.CreateMessage(ctx, systemPrompt, userPrompt)
}
