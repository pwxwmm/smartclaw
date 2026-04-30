package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type ChatSearchResult struct {
	SessionID    string `json:"sessionId"`
	SessionTitle string `json:"sessionTitle"`
	Role         string `json:"role"`
	Content      string `json:"content"`
	Timestamp    string `json:"timestamp"`
	MatchIndex   int    `json:"matchIndex"`
}

func (h *Handler) handleChatSearchWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid chat search request")
		return
	}

	query, _ := data["query"].(string)
	if query == "" {
		h.sendToClient(client, WSResponse{Type: "chat_search", Data: []any{}})
		return
	}

	limit := 20
	if l, ok := data["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	searchInCode := false
	if v, ok := data["code"].(bool); ok {
		searchInCode = v
	}

	var since, until time.Time
	if s, ok := data["since"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		} else if t, err := time.Parse("2006-01-02", s); err == nil {
			since = t
		}
	}
	if u, ok := data["until"].(string); ok && u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			until = t
		} else if t, err := time.Parse("2006-01-02", u); err == nil {
			until = t
		}
	}
	role, _ := data["role"].(string)
	sessionID, _ := data["session_id"].(string)

	if !since.IsZero() || !until.IsZero() || role != "" || sessionID != "" {
		opts := store.SearchOptions{
			UserID:    client.UserID,
			Limit:     limit,
			Since:     since,
			Until:     until,
			Role:      role,
			SessionID: sessionID,
		}
		results, err := h.searchMessagesAdvanced(query, opts, searchInCode)
		if err != nil {
			h.sendError(client, "Search failed: "+err.Error())
			return
		}
		if results == nil {
			results = []ChatSearchResult{}
		}
		h.sendToClient(client, WSResponse{Type: "chat_search", Data: results})
		return
	}

	results, err := h.searchMessages(query, client.UserID, limit, searchInCode)
	if err != nil {
		h.sendError(client, "Search failed: "+err.Error())
		return
	}

	if results == nil {
		results = []ChatSearchResult{}
	}
	h.sendToClient(client, WSResponse{Type: "chat_search", Data: results})
}

func (h *Handler) searchMessages(query, userID string, limit int, codeOnly bool) ([]ChatSearchResult, error) {
	if h.dataStore == nil {
		return nil, nil
	}

	db := h.dataStore.DB()
	if db == nil {
		return nil, nil
	}

	ftsAvailable := true
	var rows interface {
		Close() error
		Next() bool
		Scan(dest ...any) error
	}

	codeBlockLike := "%" + "```" + "%"
	if codeOnly {
		ftsQuery := query
		var err error
		if userID != "" && userID != "default" {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ? AND s.user_id = ? AND (m.content LIKE ? OR m.tool_name != '')
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, userID, codeBlockLike, limit)
		} else {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ? AND (m.content LIKE ? OR m.tool_name != '')
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, codeBlockLike, limit)
		}
		if err != nil {
			ftsAvailable = false
		}
	} else {
		ftsQuery := query
		var err error
		if userID != "" && userID != "default" {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ? AND s.user_id = ?
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, userID, limit)
		} else {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ?
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, limit)
		}
		if err != nil {
			ftsAvailable = false
		}
	}

	if !ftsAvailable {
		likePattern := "%" + query + "%"
		var err error
		if userID != "" && userID != "default" {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, 0
				FROM messages m
				JOIN sessions s ON m.session_id = s.id
				WHERE m.content LIKE ? AND s.user_id = ?
				ORDER BY m.timestamp DESC
				LIMIT ?
			`, likePattern, userID, limit)
		} else {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, 0
				FROM messages m
				JOIN sessions s ON m.session_id = s.id
				WHERE m.content LIKE ?
				ORDER BY m.timestamp DESC
				LIMIT ?
			`, likePattern, limit)
		}
		if err != nil {
			return nil, fmt.Errorf("search query failed: %w", err)
		}
	}
	defer rows.Close()

	var results []ChatSearchResult
	for rows.Next() {
		var sessionID, role, content, sessionTitle string
		var ts string
		var rank float64
		if err := rows.Scan(&sessionID, &role, &content, &ts, &sessionTitle, &rank); err != nil {
			continue
		}

		if codeOnly && !strings.Contains(content, "```") {
			continue
		}

		matchIndex := strings.Index(strings.ToLower(content), strings.ToLower(query))

		excerpt := truncateAroundMatch(content, query, 200)

		parsedTime := ts
		if t, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
			parsedTime = t.Format(time.RFC3339)
		}

		results = append(results, ChatSearchResult{
			SessionID:    sessionID,
			SessionTitle: sessionTitle,
			Role:         role,
			Content:      excerpt,
			Timestamp:    parsedTime,
			MatchIndex:   matchIndex,
		})
	}

	return results, nil
}

// truncateAroundMatch returns a substring of content centered around the first
// occurrence of query, limited to maxLen characters. If query is not found,
// it returns the first maxLen characters.
func truncateAroundMatch(content, query string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx < 0 {
		return content[:maxLen] + "..."
	}

	half := maxLen / 2
	start := idx - half
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}

	result := content[start:end]
	if start > 0 {
		result = "..." + result
	}
	if end < len(content) {
		result = result + "..."
	}
	return result
}

func (h *Handler) searchMessagesAdvanced(query string, opts store.SearchOptions, codeOnly bool) ([]ChatSearchResult, error) {
	if h.dataStore == nil {
		return nil, nil
	}

	results, err := h.dataStore.SearchMessagesAdvanced(query, opts)
	if err != nil {
		return nil, err
	}

	var chatResults []ChatSearchResult
	for _, r := range results {
		if codeOnly && !strings.Contains(r.Content, "```") {
			continue
		}

		matchIndex := strings.Index(strings.ToLower(r.Content), strings.ToLower(query))
		excerpt := truncateAroundMatch(r.Content, query, 200)

		sessionTitle := ""
		if h.dataStore != nil {
			if sess, err := h.dataStore.GetSession(r.SessionID); err == nil && sess != nil {
				sessionTitle = sess.Title
			}
		}

		chatResults = append(chatResults, ChatSearchResult{
			SessionID:    r.SessionID,
			SessionTitle: sessionTitle,
			Role:         r.Role,
			Content:      excerpt,
			Timestamp:    r.Timestamp.Format(time.RFC3339),
			MatchIndex:   matchIndex,
		})
	}

	return chatResults, nil
}

func (s *WebServer) handleChatSearchAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	codeOnly := r.URL.Query().Get("code") == "true"

	userID := "default"
	if s.handler.dataStore != nil {
		userID = ""
	}

	var since, until time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		} else if t, err := time.Parse("2006-01-02", s); err == nil {
			since = t
		}
	}
	if u := r.URL.Query().Get("until"); u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			until = t
		} else if t, err := time.Parse("2006-01-02", u); err == nil {
			until = t
		}
	}
	role := r.URL.Query().Get("role")
	sessionID := r.URL.Query().Get("session_id")

	if !since.IsZero() || !until.IsZero() || role != "" || sessionID != "" {
		opts := store.SearchOptions{
			UserID:    userID,
			Limit:     limit,
			Since:     since,
			Until:     until,
			Role:      role,
			SessionID: sessionID,
		}
		results, err := s.handler.searchMessagesAdvanced(query, opts, codeOnly)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if results == nil {
			results = []ChatSearchResult{}
		}
		writeJSON(w, http.StatusOK, results)
		return
	}

	results, err := s.handler.searchMessages(query, userID, limit, codeOnly)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if results == nil {
		results = []ChatSearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}
