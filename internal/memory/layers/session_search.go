package layers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type SessionFragment struct {
	SessionID  string
	Timestamp  time.Time
	Role       string
	Content    string
	Relevance  float64
	SourceTurn int
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
			SessionID:  r.SessionID,
			Timestamp:  r.Timestamp,
			Role:       r.Role,
			Content:    r.Content,
			Relevance:  r.Rank,
			SourceTurn: int(r.ID),
		})
	}

	return fragments, nil
}

func (ss *SessionSearch) SearchByUser(ctx context.Context, query string, userID string, limit int) ([]SessionFragment, error) {
	if ss.store == nil || query == "" {
		return nil, nil
	}

	results, err := ss.store.SearchMessagesByUser(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("session search by user: %w", err)
	}

	fragments := make([]SessionFragment, 0, len(results))
	for _, r := range results {
		fragments = append(fragments, SessionFragment{
			SessionID:  r.SessionID,
			Timestamp:  r.Timestamp,
			Role:       r.Role,
			Content:    r.Content,
			Relevance:  r.Rank,
			SourceTurn: int(r.ID),
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

func (ss *SessionSearch) IndexSession(ctx context.Context, sessionID string) error {
	if ss.store == nil {
		return nil
	}

	messages, err := ss.store.GetSessionMessages(sessionID)
	if err != nil {
		return fmt.Errorf("session search: index session: %w", err)
	}

	slog.Info("session search: indexed session", "session_id", sessionID, "messages", len(messages))
	return nil
}

func (ss *SessionSearch) ForceReindexSession(ctx context.Context, sessionID string) error {
	if ss.store == nil {
		return nil
	}

	messages, err := ss.store.GetSessionMessages(sessionID)
	if err != nil {
		return fmt.Errorf("session search: reindex session: %w", err)
	}

	db := ss.store.DB()
	for _, msg := range messages {
		db.ExecContext(ctx, "INSERT INTO messages_fts(messages_fts, rowid, content) VALUES('delete', ?, ?)", msg.ID, msg.Content)
		db.ExecContext(ctx, "INSERT INTO messages_fts(rowid, content) VALUES(?, ?)", msg.ID, msg.Content)
	}

	slog.Info("session search: force reindexed session", "session_id", sessionID, "messages", len(messages))
	return nil
}

func (ss *SessionSearch) IndexAllSessions(ctx context.Context) error {
	if ss.store == nil {
		return nil
	}

	sessions, err := ss.store.ListSessions("default", 1000)
	if err != nil {
		return fmt.Errorf("session search: index all: %w", err)
	}

	for _, s := range sessions {
		if err := ss.IndexSession(ctx, s.ID); err != nil {
			slog.Warn("session search: failed to index session", "session_id", s.ID, "error", err)
			continue
		}
	}

	slog.Info("session search: indexed all sessions", "count", len(sessions))
	return nil
}
