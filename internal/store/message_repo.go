package store

import (
	"database/sql"
	"fmt"
	"time"
)

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
}

func (s *Store) InsertMessage(msg *Message) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO messages (session_id, role, content, tokens, tool_calls, tool_name, tool_input, tool_result, finish_reason, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.SessionID, msg.Role, msg.Content, msg.Tokens,
		nullStr(msg.ToolCalls), nullStr(msg.ToolName), nullStr(msg.ToolInput), nullStr(msg.ToolResult),
		nullStr(msg.FinishReason), msg.Timestamp)
	if err != nil {
		return 0, fmt.Errorf("store: insert message: %w", err)
	}

	id, _ := result.LastInsertId()
	return id, nil
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
