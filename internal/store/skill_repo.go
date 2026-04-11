package store

import (
	"database/sql"
	"fmt"
	"time"
)

type SkillRecord struct {
	Name        string
	Description string
	Content     string
	Source      string
	UseCount    int
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *Store) UpsertSkill(skill *SkillRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO skills (name, description, content, source, use_count, last_used_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			content = excluded.content,
			source = excluded.source,
			use_count = excluded.use_count,
			last_used_at = excluded.last_used_at,
			updated_at = CURRENT_TIMESTAMP
	`, skill.Name, skill.Description, skill.Content, skill.Source, skill.UseCount, nullTime(skill.LastUsedAt))
	if err != nil {
		return fmt.Errorf("store: upsert skill: %w", err)
	}
	return nil
}

func (s *Store) IncrementSkillUseCount(name string) error {
	_, err := s.db.Exec(`
		UPDATE skills SET use_count = use_count + 1, last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE name = ?
	`, name)
	if err != nil {
		return fmt.Errorf("store: increment skill use count: %w", err)
	}
	return nil
}

type StaleSkill struct {
	Name       string
	UseCount   int
	LastUsedAt *time.Time
	Source     string
}

func (s *Store) GetStaleSkills(olderThan time.Duration) ([]*StaleSkill, error) {
	cutoff := time.Now().Add(-olderThan)
	rows, err := s.db.Query(`
		SELECT name, use_count, last_used_at, source
		FROM skills
		WHERE source = 'learned'
		  AND (last_used_at IS NULL OR last_used_at < ?)
		  AND use_count < 3
		ORDER BY last_used_at ASC NULLS FIRST
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("store: get stale skills: %w", err)
	}
	defer rows.Close()

	var skills []*StaleSkill
	for rows.Next() {
		skill := &StaleSkill{}
		var lastUsed sql.NullString
		if err := rows.Scan(&skill.Name, &skill.UseCount, &lastUsed, &skill.Source); err != nil {
			return nil, fmt.Errorf("store: scan stale skill: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", lastUsed.String); err == nil {
			skill.LastUsedAt = &t
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func (s *Store) DeleteSkill(name string) error {
	_, err := s.db.Exec(`DELETE FROM skills WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("store: delete skill: %w", err)
	}
	return nil
}

func (s *Store) GetAllLearnedSkills() ([]*SkillRecord, error) {
	rows, err := s.db.Query(`
		SELECT name, description, content, source, use_count, last_used_at, created_at, updated_at
		FROM skills WHERE source = 'learned'
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("store: get learned skills: %w", err)
	}
	defer rows.Close()

	var skills []*SkillRecord
	for rows.Next() {
		skill := &SkillRecord{}
		var lastUsed, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&skill.Name, &skill.Description, &skill.Content, &skill.Source,
			&skill.UseCount, &lastUsed, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("store: scan skill: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", lastUsed.String); err == nil {
			skill.LastUsedAt = &t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt.String); err == nil {
			skill.CreatedAt = t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
			skill.UpdatedAt = t
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02 15:04:05")
}
