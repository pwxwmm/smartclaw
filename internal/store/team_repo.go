package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type TeamRecord struct {
	ID          string
	Name        string
	Description string
	Settings    string // JSON
	CreatedAt   string
	UpdatedAt   string
}

type TeamMemoryRecord struct {
	ID         int64
	TeamID     string
	MemoryID   string
	Title      string
	Content    string
	Type       string
	Visibility string
	Tags       string // JSON array
	AuthorID   string
	CreatedAt  string
}

func (s *Store) SaveTeam(ctx context.Context, team *TeamRecord) error {
	return s.WriteWithRetry(ctx, `
		INSERT OR REPLACE INTO teams (id, name, description, settings, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, team.ID, team.Name, team.Description, team.Settings, team.CreatedAt, team.UpdatedAt)
}

func (s *Store) GetTeam(ctx context.Context, id string) (*TeamRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, settings, created_at, updated_at
		FROM teams WHERE id = ?
	`, id)

	team := &TeamRecord{}
	var description, settings sql.NullString
	err := row.Scan(&team.ID, &team.Name, &description, &settings, &team.CreatedAt, &team.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get team: %w", err)
	}

	team.Description = val2(description)
	team.Settings = val2(settings)
	return team, nil
}

func (s *Store) ListTeams(ctx context.Context) ([]*TeamRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, settings, created_at, updated_at
		FROM teams ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("store: list teams: %w", err)
	}
	defer rows.Close()

	var teams []*TeamRecord
	for rows.Next() {
		team := &TeamRecord{}
		var description, settings sql.NullString
		if err := rows.Scan(&team.ID, &team.Name, &description, &settings, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan team: %w", err)
		}
		team.Description = val2(description)
		team.Settings = val2(settings)
		teams = append(teams, team)
	}
	return teams, nil
}

func (s *Store) DeleteTeam(ctx context.Context, id string) error {
	return s.WriteWithRetry(ctx, `DELETE FROM teams WHERE id = ?`, id)
}

func (s *Store) SaveTeamMemory(ctx context.Context, mem *TeamMemoryRecord) error {
	if mem.CreatedAt == "" {
		mem.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	return s.WriteWithRetry(ctx, `
		INSERT INTO team_memories (team_id, memory_id, title, content, type, visibility, tags, author_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, mem.TeamID, mem.MemoryID, mem.Title, mem.Content, mem.Type, mem.Visibility, mem.Tags, mem.AuthorID, mem.CreatedAt)
}

func (s *Store) GetTeamMemories(ctx context.Context, teamID string) ([]*TeamMemoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, team_id, memory_id, title, content, type, visibility, tags, author_id, created_at
		FROM team_memories WHERE team_id = ?
		ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		return nil, fmt.Errorf("store: get team memories: %w", err)
	}
	defer rows.Close()

	return scanTeamMemoryRows(rows)
}

func (s *Store) SearchTeamMemories(ctx context.Context, teamID, query string) ([]*TeamMemoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, team_id, memory_id, title, content, type, visibility, tags, author_id, created_at
		FROM team_memories
		WHERE team_id = ? AND (title LIKE ? OR content LIKE ?)
		ORDER BY created_at DESC
	`, teamID, "%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("store: search team memories: %w", err)
	}
	defer rows.Close()

	return scanTeamMemoryRows(rows)
}

func (s *Store) DeleteTeamMemory(ctx context.Context, id int64) error {
	return s.WriteWithRetry(ctx, `DELETE FROM team_memories WHERE id = ?`, id)
}

func scanTeamMemoryRows(rows *sql.Rows) ([]*TeamMemoryRecord, error) {
	var memories []*TeamMemoryRecord
	for rows.Next() {
		mem := &TeamMemoryRecord{}
		var memoryID, content, tags, authorID sql.NullString
		if err := rows.Scan(&mem.ID, &mem.TeamID, &memoryID, &mem.Title, &content,
			&mem.Type, &mem.Visibility, &tags, &authorID, &mem.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan team memory: %w", err)
		}
		mem.MemoryID = val2(memoryID)
		mem.Content = val2(content)
		mem.Tags = val2(tags)
		mem.AuthorID = val2(authorID)
		memories = append(memories, mem)
	}
	return memories, nil
}

func val2(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
