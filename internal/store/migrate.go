package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func MigrateJSONToSQLite(sessionDir string, s *Store) error {
	if s == nil {
		return fmt.Errorf("migrate: store is nil")
	}

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("migrate: read dir: %w", err)
	}

	migrated := 0
	skipped := 0

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(sessionDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("migrate: failed to read file", "path", path, "error", err)
			continue
		}

		var session struct {
			ID        string `json:"id"`
			Model     string `json:"model"`
			CreatedAt string `json:"created_at"`
			Messages  []struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				Timestamp string `json:"timestamp"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &session); err != nil {
			slog.Warn("migrate: failed to parse JSON", "path", path, "error", err)
			skipped++
			continue
		}

		if session.ID == "" {
			session.ID = "migrated_" + entry.Name()
		}

		sessionRecord := &Session{
			ID:        session.ID,
			UserID:    "default",
			Source:    "cli",
			Model:     session.Model,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := s.UpsertSession(sessionRecord); err != nil {
			slog.Warn("migrate: failed to upsert session", "id", session.ID, "error", err)
			continue
		}

		for _, msg := range session.Messages {
			storeMsg := &Message{
				SessionID: session.ID,
				Role:      msg.Role,
				Content:   msg.Content,
				Timestamp: time.Now(),
			}

			if _, err := s.InsertMessage(storeMsg); err != nil {
				slog.Warn("migrate: failed to insert message", "error", err)
				continue
			}
		}

		migrated++
	}

	slog.Info("migrate: complete", "migrated", migrated, "skipped", skipped)
	return nil
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

			if _, err := s.InsertMessage(msg); err != nil {
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
