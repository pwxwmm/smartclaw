package store

import (
	"database/sql"
	"fmt"
	"time"
)

type Session struct {
	ID              string
	UserID          string
	Source          string
	Model           string
	SystemPrompt    string
	ParentSessionID string
	Title           string
	Summary         string
	Tokens          int
	Cost            float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (s *Store) UpsertSession(session *Session) error {
	return s.WriteWithRetry(`
		INSERT INTO sessions (id, user_id, source, model, system_prompt, parent_session_id, title, summary, tokens, cost, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			model = excluded.model,
			system_prompt = excluded.system_prompt,
			title = excluded.title,
			summary = excluded.summary,
			tokens = excluded.tokens,
			cost = excluded.cost,
			updated_at = excluded.updated_at
	`, session.ID, session.UserID, session.Source, session.Model, session.SystemPrompt,
		session.ParentSessionID, session.Title, session.Summary, session.Tokens, session.Cost,
		session.CreatedAt, session.UpdatedAt)
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, source, model, system_prompt, parent_session_id, title, summary, tokens, cost, created_at, updated_at
		FROM sessions WHERE id = ?
	`, id)

	session := &Session{}
	var createdAt, updatedAt sql.NullString
	var userID, source, model, sysPrompt, parentID, title, summary sql.NullString

	err := row.Scan(
		&session.ID, &userID, &source, &model, &sysPrompt, &parentID,
		&title, &summary, &session.Tokens, &session.Cost,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get session: %w", err)
	}

	session.UserID = val(userID)
	session.Source = val(source)
	session.Model = val(model)
	session.SystemPrompt = val(sysPrompt)
	session.ParentSessionID = val(parentID)
	session.Title = val(title)
	session.Summary = val(summary)
	if t, err := time.Parse("2006-01-02 15:04:05", val(createdAt)); err == nil {
		session.CreatedAt = t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", val(updatedAt)); err == nil {
		session.UpdatedAt = t
	}

	return session, nil
}

func (s *Store) ListSessions(userID string, limit int) ([]*Session, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, user_id, source, model, title, summary, tokens, cost, created_at, updated_at
		FROM sessions WHERE user_id = ?
		ORDER BY updated_at DESC LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		session := &Session{}
		var createdAt, updatedAt sql.NullString
		var source, model, title, summary sql.NullString

		if err := rows.Scan(
			&session.ID, &session.UserID, &source, &model,
			&title, &summary, &session.Tokens, &session.Cost,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("store: scan session: %w", err)
		}

		session.Source = val(source)
		session.Model = val(model)
		session.Title = val(title)
		session.Summary = val(summary)
		if t, err := time.Parse("2006-01-02 15:04:05", val(createdAt)); err == nil {
			session.CreatedAt = t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", val(updatedAt)); err == nil {
			session.UpdatedAt = t
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (s *Store) DeleteSession(id string) error {
	return s.WriteWithRetry(`DELETE FROM sessions WHERE id = ?`, id)
}

func val(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
