package layers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type SessionFragment struct {
	SessionID string
	Timestamp time.Time
	Role      string
	Content   string
	Relevance float64
}

type SessionSearch struct {
	store *store.Store
}

func NewSessionSearch(s *store.Store) *SessionSearch {
	return &SessionSearch{store: s}
}

func (ss *SessionSearch) Search(ctx context.Context, query string, limit int) ([]SessionFragment, error) {
	if ss.store == nil || query == "" {
		return nil, nil
	}

	results, err := ss.store.SearchMessages(query, limit)
	if err != nil {
		return nil, fmt.Errorf("session search: %w", err)
	}

	fragments := make([]SessionFragment, 0, len(results))
	for _, r := range results {
		fragments = append(fragments, SessionFragment{
			SessionID: r.SessionID,
			Timestamp: r.Timestamp,
			Role:      r.Role,
			Content:   r.Content,
			Relevance: r.Rank,
		})
	}

	return fragments, nil
}

func (ss *SessionSearch) SearchAndSummarize(ctx context.Context, query string, maxTokens int, summarizeFunc func(fragments []SessionFragment, query string, maxTokens int) (string, error)) (string, error) {
	fragments, err := ss.Search(ctx, query, 10)
	if err != nil {
		return "", err
	}

	if len(fragments) == 0 {
		return "", nil
	}

	if summarizeFunc != nil {
		summary, err := summarizeFunc(fragments, query, maxTokens)
		if err != nil {
			slog.Warn("session search: summarization failed, using raw fragments", "error", err)
		} else {
			return summary, nil
		}
	}

	return formatFragments(fragments, maxTokens), nil
}

func FormatFragmentsStatic(fragments []SessionFragment, maxTokens int) string {
	return formatFragments(fragments, maxTokens)
}

func formatFragments(fragments []SessionFragment, maxTokens int) string {
	result := ""
	approxChars := maxTokens * 4

	for _, f := range fragments {
		line := fmt.Sprintf("[%s %s]: %s\n", f.Role, f.Timestamp.Format("15:04"), truncateStr(f.Content, 200))
		if len(result)+len(line) > approxChars {
			break
		}
		result += line
	}

	return result
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
