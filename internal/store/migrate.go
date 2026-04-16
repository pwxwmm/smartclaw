package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MigrateJSONToSQLite migrates sessions from JSON files into the SQLite store.
// Deprecated: Use Store.MigrateJSONSessions instead.
func MigrateJSONToSQLite(sessionDir string, s *Store) error {
	_, err := s.MigrateJSONSessions(sessionDir)
	return err
}

// MigrateJSONSessions migrates sessions from JSON files to SQLite.
// Called once at startup. Skips sessions already in SQLite (by ID).
func (s *Store) MigrateJSONSessions(sessionsDir string) (int, error) {
	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("migrate: read dir: %w", err)
	}

	migrated := 0

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		sessionID := strings.TrimSuffix(file.Name(), ".json")

		if existing, _ := s.GetSession(sessionID); existing != nil {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionsDir, file.Name()))
		if err != nil {
			slog.Warn("migrate: failed to read file", "file", file.Name(), "error", err)
			continue
		}

		var sess struct {
			ID        string    `json:"id"`
			UserID    string    `json:"user_id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Model     string    `json:"model"`
			Tokens    int       `json:"tokens"`
			Cost      float64   `json:"cost"`
			Title     string    `json:"title"`
			Messages  []struct {
				Role      string    `json:"role"`
				Content   string    `json:"content"`
				Timestamp time.Time `json:"timestamp"`
				Tokens    int       `json:"tokens"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &sess); err != nil {
			slog.Warn("migrate: failed to parse JSON", "file", file.Name(), "error", err)
			continue
		}

		if sess.ID == "" {
			sess.ID = sessionID
		}

		storeSession := &Session{
			ID:        sess.ID,
			UserID:    sess.UserID,
			Source:    "web",
			Model:     sess.Model,
			Title:     sess.Title,
			Tokens:    sess.Tokens,
			Cost:      sess.Cost,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
		}
		if storeSession.UserID == "" {
			storeSession.UserID = "default"
		}
		if storeSession.CreatedAt.IsZero() {
			storeSession.CreatedAt = time.Now()
		}
		if storeSession.UpdatedAt.IsZero() {
			storeSession.UpdatedAt = time.Now()
		}

		if err := s.UpsertSession(context.Background(), storeSession); err != nil {
			slog.Warn("migrate: failed to upsert session", "id", sess.ID, "error", err)
			continue
		}

		for _, msg := range sess.Messages {
			storeMsg := &Message{
				SessionID: sess.ID,
				Role:      msg.Role,
				Content:   msg.Content,
				Tokens:    msg.Tokens,
				Timestamp: msg.Timestamp,
			}
			if storeMsg.Timestamp.IsZero() {
				storeMsg.Timestamp = time.Now()
			}

			if err := s.InsertMessage(context.Background(), storeMsg); err != nil {
				slog.Warn("migrate: failed to insert message", "error", err)
			}
		}

		migrated++
	}

	if migrated > 0 {
		slog.Info("migrate: JSON sessions migrated", "count", migrated)
	}
	return migrated, nil
}

func MigrateJSONLToSQLite(jsonlDir string, s *Store) error {
	if s == nil {
		return fmt.Errorf("migrate: store is nil")
	}

	entries, err := os.ReadDir(jsonlDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("migrate: read dir: %w", err)
	}

	migrated := 0

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		path := filepath.Join(jsonlDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := splitLines(string(data))
		for _, line := range lines {
			if line == "" {
				continue
			}

			var entry struct {
				SessionID string `json:"session_id"`
				Role      string `json:"role"`
				Content   string `json:"content"`
				Timestamp string `json:"timestamp"`
			}

			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}

			if entry.SessionID == "" {
				continue
			}

			msg := &Message{
				SessionID: entry.SessionID,
				Role:      entry.Role,
				Content:   entry.Content,
				Timestamp: time.Now(),
			}

			if err := s.InsertMessage(context.Background(), msg); err != nil {
				continue
			}
			migrated++
		}
	}

	slog.Info("migrate: JSONL complete", "migrated", migrated)
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func MigrateUserObservationsUserID(db *sql.DB) error {
	var hasUserID bool
	rows, err := db.Query(`PRAGMA table_info(user_observations)`)
	if err != nil {
		return fmt.Errorf("migrate: check user_observations columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == "user_id" {
			hasUserID = true
			break
		}
	}

	if !hasUserID {
		if _, err := db.Exec(`ALTER TABLE user_observations ADD COLUMN user_id TEXT NOT NULL DEFAULT 'default'`); err != nil {
			return fmt.Errorf("migrate: add user_id to user_observations: %w", err)
		}
		if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_observations_user ON user_observations(user_id, category)`); err != nil {
			return fmt.Errorf("migrate: create idx_observations_user: %w", err)
		}
		slog.Info("migrate: added user_id column to user_observations")
	}

	return nil
}
