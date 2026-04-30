package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var (
	globalEmbedder Embedder
	embedderMu     sync.RWMutex
)

func SetEmbedder(e Embedder) {
	embedderMu.Lock()
	defer embedderMu.Unlock()
	globalEmbedder = e
}

func GetGlobalEmbedder() Embedder {
	embedderMu.RLock()
	defer embedderMu.RUnlock()
	return globalEmbedder
}

func getEmbedder() Embedder {
	embedderMu.RLock()
	defer embedderMu.RUnlock()
	return globalEmbedder
}

type Message struct {
	ID           int64
	SessionID    string
	Role         string
	Content      string
	Tokens       int
	ToolCalls    string
	ToolName     string
	ToolInput    string
	ToolResult   string
	FinishReason string
	Timestamp    time.Time
}

type SearchResult struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	Timestamp time.Time
	Rank      float64
	Snippet   string
}

type SessionSearchResult struct {
	SessionID string
	Title     string
	Summary   string
	Rank      float64
	Snippet   string
}

// SearchOptions configures an advanced FTS5 search query.
type SearchOptions struct {
	UserID    string    // Filter by user via sessions.user_id
	SessionID string    // Filter to a specific session
	Role      string    // Filter by role ("user" or "assistant")
	Since     time.Time // Only results at or after this time
	Until     time.Time // Only results before or at this time
	Limit     int       // Max results (default 20)
	Offset    int       // Pagination offset
}

func (s *Store) InsertMessage(ctx context.Context, msg *Message) error {
	err := s.WriteWithRetry(ctx, `
		INSERT INTO messages (session_id, role, content, tokens, tool_calls, tool_name, tool_input, tool_result, finish_reason, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.SessionID, msg.Role, msg.Content, msg.Tokens,
		nullStr(msg.ToolCalls), nullStr(msg.ToolName), nullStr(msg.ToolInput), nullStr(msg.ToolResult),
		nullStr(msg.FinishReason), msg.Timestamp)
	if err != nil {
		return err
	}

	go embedAsync(s, msg.SessionID, "message", msg.Content)

	return nil
}

func embedAsync(st *Store, sourceID, sourceType, content string) {
	emb := getEmbedder()
	if emb == nil || content == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vec, err := emb.Embed(ctx, content)
	if err != nil {
		slog.Warn("embedder: async embed failed", "source_type", sourceType, "source_id", sourceID, "error", err)
		return
	}

	if err := st.StoreEmbedding(ctx, sourceType, sourceID, content, vec); err != nil {
		slog.Warn("embedder: failed to store embedding", "source_type", sourceType, "source_id", sourceID, "error", err)
	}
}

func (s *Store) GetSessionMessages(sessionID string) ([]*Message, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, tokens, tool_calls, tool_name, tool_input, tool_result, finish_reason, timestamp
		FROM messages WHERE session_id = ?
		ORDER BY timestamp ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("store: get session messages: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (s *Store) SearchMessages(query string, limit int) ([]*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	ftsQuery := query
	if ftsQuery == "" {
		return nil, nil
	}

	rows, err := s.db.Query(`
		SELECT m.id, m.session_id, m.role, m.content, m.timestamp, f.rank
		FROM messages_fts f
		JOIN messages m ON m.id = f.rowid
		WHERE messages_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("store: search messages: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		r := &SearchResult{}
		var ts sql.NullString
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Content, &ts, &r.Rank); err != nil {
			return nil, fmt.Errorf("store: scan search result: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", val(ts)); err == nil {
			r.Timestamp = t
		}
		results = append(results, r)
	}

	return results, nil
}

func (s *Store) SearchMessagesByUser(query string, userID string, limit int) ([]*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	if query == "" {
		return nil, nil
	}

	if userID == "" || userID == "default" {
		return s.SearchMessages(query, limit)
	}

	rows, err := s.db.Query(`
		SELECT m.id, m.session_id, m.role, m.content, m.timestamp, f.rank
		FROM messages_fts f
		JOIN messages m ON m.id = f.rowid
		JOIN sessions s ON m.session_id = s.id
		WHERE messages_fts MATCH ? AND s.user_id = ?
		ORDER BY f.rank
		LIMIT ?
	`, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: search messages by user: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		r := &SearchResult{}
		var ts sql.NullString
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Content, &ts, &r.Rank); err != nil {
			return nil, fmt.Errorf("store: scan search result: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", val(ts)); err == nil {
			r.Timestamp = t
		}
		results = append(results, r)
	}

	return results, nil
}

func (s *Store) GetMessageCount(sessionID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id = ?`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: message count: %w", err)
	}
	return count, nil
}

func (s *Store) GetMessageCountsBatch() (map[string]int64, error) {
	rows, err := s.db.Query("SELECT session_id, COUNT(*) FROM messages GROUP BY session_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var sessionID string
		var count int64
		if err := rows.Scan(&sessionID, &count); err != nil {
			continue
		}
		counts[sessionID] = count
	}
	return counts, nil
}

func (s *Store) GetRecentMessages(sessionID string, limit int) ([]*Message, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, tokens, tool_calls, tool_name, tool_input, tool_result, finish_reason, timestamp
		FROM messages WHERE session_id = ?
		ORDER BY timestamp DESC LIMIT ?
	`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: recent messages: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}

func scanMessages(rows *sql.Rows) ([]*Message, error) {
	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		var toolCalls, toolName, toolInput, toolResult, finishReason sql.NullString
		var ts sql.NullString

		if err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.Tokens,
			&toolCalls, &toolName, &toolInput, &toolResult, &finishReason, &ts,
		); err != nil {
			return nil, fmt.Errorf("store: scan message: %w", err)
		}

		msg.ToolCalls = val(toolCalls)
		msg.ToolName = val(toolName)
		msg.ToolInput = val(toolInput)
		msg.ToolResult = val(toolResult)
		msg.FinishReason = val(finishReason)
		if t, err := time.Parse("2006-01-02 15:04:05", val(ts)); err == nil {
			msg.Timestamp = t
		}

		messages = append(messages, msg)
	}
	return messages, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// SearchMessagesAdvanced performs an FTS5 search with dynamic filtering.
// Supports FTS5 advanced syntax: "exact phrase", AND, OR, NEAR, * (prefix).
func (s *Store) SearchMessagesAdvanced(query string, opts SearchOptions) ([]*SearchResult, error) {
	if query == "" {
		return nil, nil
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	var (
		clauses []string
		args    []any
	)

	clauses = append(clauses, "messages_fts MATCH ?")
	args = append(args, query)

	if opts.SessionID != "" {
		clauses = append(clauses, "m.session_id = ?")
		args = append(args, opts.SessionID)
	}

	if opts.Role != "" {
		clauses = append(clauses, "m.role = ?")
		args = append(args, opts.Role)
	}

	if !opts.Since.IsZero() {
		clauses = append(clauses, "m.timestamp >= ?")
		args = append(args, opts.Since.Format("2006-01-02 15:04:05"))
	}

	if !opts.Until.IsZero() {
		clauses = append(clauses, "m.timestamp <= ?")
		args = append(args, opts.Until.Format("2006-01-02 15:04:05"))
	}

	needUserJoin := opts.UserID != "" && opts.UserID != "default"
	if needUserJoin {
		clauses = append(clauses, "s.user_id = ?")
		args = append(args, opts.UserID)
	}

	where := "WHERE " + strings.Join(clauses, " AND ")

	joinClause := "JOIN messages m ON m.id = f.rowid"
	if needUserJoin {
		joinClause += " JOIN sessions s ON m.session_id = s.id"
	}

	q := fmt.Sprintf(`
		SELECT m.id, m.session_id, m.role, m.content, m.timestamp, f.rank,
		       COALESCE(snippet(messages_fts, 0, '>>', '<<', '...', 32), '') ||
		       CASE WHEN snippet(messages_fts, 1, '>>', '<<', '...', 16) != '' AND snippet(messages_fts, 0, '>>', '<<', '...', 32) != '' THEN ' ... ' ELSE '' END ||
		       COALESCE(snippet(messages_fts, 1, '>>', '<<', '...', 16), '') ||
		       CASE WHEN snippet(messages_fts, 2, '>>', '<<', '...', 16) != '' AND (snippet(messages_fts, 0, '>>', '<<', '...', 32) != '' OR snippet(messages_fts, 1, '>>', '<<', '...', 16) != '') THEN ' ... ' ELSE '' END ||
		       COALESCE(snippet(messages_fts, 2, '>>', '<<', '...', 16), '') AS snippet
		FROM messages_fts f
		%s
		%s
		ORDER BY f.rank
		LIMIT ? OFFSET ?
	`, joinClause, where)

	args = append(args, opts.Limit, opts.Offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: search messages advanced: %w", err)
	}
	defer rows.Close()

	return scanSearchResultsWithSnippet(rows)
}

// SearchMessagesByTimeRange searches within a time window.
func (s *Store) SearchMessagesByTimeRange(query string, since, until time.Time, limit int) ([]*SearchResult, error) {
	return s.SearchMessagesAdvanced(query, SearchOptions{
		Since: since,
		Until: until,
		Limit: limit,
	})
}

// SearchMessagesBySession searches within a single session.
func (s *Store) SearchMessagesBySession(query string, sessionID string, limit int) ([]*SearchResult, error) {
	return s.SearchMessagesAdvanced(query, SearchOptions{
		SessionID: sessionID,
		Limit:     limit,
	})
}

// GetSearchSnippets returns a highlighted snippet around matches for a specific document.
func (s *Store) GetSearchSnippets(query string, docID int64, maxTokens int) (string, error) {
	if query == "" {
		return "", nil
	}

	numTokens := 32
	if maxTokens > 0 {
		numTokens = maxTokens / 4
		if numTokens < 8 {
			numTokens = 8
		}
	}

	var snippet string
	err := s.db.QueryRow(`
		SELECT snippet(messages_fts, 0, '>>', '<<', '...', ?)
		FROM messages_fts
		WHERE messages_fts MATCH ? AND rowid = ?
	`, numTokens, query, docID).Scan(&snippet)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("store: get search snippet: %w", err)
	}

	return snippet, nil
}

func scanSearchResultsWithSnippet(rows *sql.Rows) ([]*SearchResult, error) {
	var results []*SearchResult
	for rows.Next() {
		r := &SearchResult{}
		var ts sql.NullString
		var snippet sql.NullString
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Content, &ts, &r.Rank, &snippet); err != nil {
			return nil, fmt.Errorf("store: scan search result with snippet: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", val(ts)); err == nil {
			r.Timestamp = t
		}
		r.Snippet = val(snippet)
		results = append(results, r)
	}
	return results, nil
}

func (s *Store) SearchSessions(query string, limit int) ([]*SessionSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if query == "" {
		return nil, nil
	}

	rows, err := s.db.Query(`
		SELECT s.id, s.title, s.summary, f.rank,
		       snippet(sessions_fts, 0, '>>', '<<', '...', 32) AS snippet
		FROM sessions_fts f
		JOIN sessions s ON s.rowid = f.rowid
		WHERE sessions_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("store: search sessions: %w", err)
	}
	defer rows.Close()

	return scanSessionSearchResults(rows)
}

func (s *Store) SearchSessionsByUser(query string, userID string, limit int) ([]*SessionSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if query == "" {
		return nil, nil
	}
	if userID == "" || userID == "default" {
		return s.SearchSessions(query, limit)
	}

	rows, err := s.db.Query(`
		SELECT s.id, s.title, s.summary, f.rank,
		       snippet(sessions_fts, 0, '>>', '<<', '...', 32) AS snippet
		FROM sessions_fts f
		JOIN sessions s ON s.rowid = f.rowid
		WHERE sessions_fts MATCH ? AND s.user_id = ?
		ORDER BY f.rank
		LIMIT ?
	`, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: search sessions by user: %w", err)
	}
	defer rows.Close()

	return scanSessionSearchResults(rows)
}

func scanSessionSearchResults(rows *sql.Rows) ([]*SessionSearchResult, error) {
	var results []*SessionSearchResult
	for rows.Next() {
		r := &SessionSearchResult{}
		var snippet sql.NullString
		if err := rows.Scan(&r.SessionID, &r.Title, &r.Summary, &r.Rank, &snippet); err != nil {
			return nil, fmt.Errorf("store: scan session search result: %w", err)
		}
		r.Snippet = val(snippet)
		results = append(results, r)
	}
	return results, nil
}
