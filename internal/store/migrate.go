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

// MigrateSessionsFTS creates the sessions_fts virtual table and sync triggers
// if they don't already exist. Safe to call idempotently.
func MigrateSessionsFTS(db *sql.DB) error {
	// Check if sessions_fts already exists
	var exists bool
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name='sessions_fts'`)
	if err != nil {
		return fmt.Errorf("migrate: check sessions_fts: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		exists = true
	}
	rows.Close()
	if exists {
		return nil
	}

	// Create sessions_fts and triggers
	stmts := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS sessions_fts USING fts5(title, summary, content='sessions', content_rowid='rowid')`,
		`CREATE TRIGGER IF NOT EXISTS sessions_fts_insert AFTER INSERT ON sessions BEGIN
			INSERT INTO sessions_fts(rowid, title, summary) VALUES (new.rowid, new.title, new.summary);
		END`,
		`CREATE TRIGGER IF NOT EXISTS sessions_fts_delete AFTER DELETE ON sessions BEGIN
			INSERT INTO sessions_fts(sessions_fts, rowid, title, summary) VALUES ('delete', old.rowid, old.title, old.summary);
		END`,
		`CREATE TRIGGER IF NOT EXISTS sessions_fts_update AFTER UPDATE ON sessions BEGIN
			INSERT INTO sessions_fts(sessions_fts, rowid, title, summary) VALUES ('delete', old.rowid, old.title, old.summary);
			INSERT INTO sessions_fts(rowid, title, summary) VALUES (new.rowid, new.title, new.summary);
		END`,
		// Backfill from existing sessions
		`INSERT INTO sessions_fts(rowid, title, summary) SELECT rowid, title, summary FROM sessions`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: create sessions_fts: %w", err)
		}
	}

	slog.Info("migrate: created sessions_fts with sync triggers")
	return nil
}

// MigrateMessagesFTSExtended rebuilds messages_fts to include tool_input and
// tool_result columns. Safe to call idempotently — it checks column count first.
func MigrateMessagesFTSExtended(db *sql.DB) error {
	// Check how many columns messages_fts currently has
	colCount := 0
	rows, err := db.Query(`PRAGMA table_info(messages_fts)`)
	if err != nil {
		return fmt.Errorf("migrate: check messages_fts columns: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		colCount++
		var cid int
		var name, ctype string
		var notNull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
	}
	_ = rows.Close()

	// New schema has 3 columns: content, tool_input, tool_result
	// Old schema had 1 column: content
	// FTS5 adds hidden columns, so we count only named ones.
	// If colCount >= 3 (content + tool_input + tool_result + possibly hidden), already migrated.
	if colCount >= 3 {
		return nil
	}

	slog.Info("migrate: rebuilding messages_fts with tool_input and tool_result columns")

	// Drop old triggers first
	dropTriggers := []string{
		`DROP TRIGGER IF EXISTS messages_fts_insert`,
		`DROP TRIGGER IF EXISTS messages_fts_delete`,
		`DROP TRIGGER IF EXISTS messages_fts_update`,
	}
	for _, stmt := range dropTriggers {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: drop old triggers: %w", err)
		}
	}

	// Drop old FTS table
	if _, err := db.Exec(`DROP TABLE IF EXISTS messages_fts`); err != nil {
		return fmt.Errorf("migrate: drop old messages_fts: %w", err)
	}

	// Create new FTS table with extended columns
	if _, err := db.Exec(`CREATE VIRTUAL TABLE messages_fts USING fts5(content, tool_input, tool_result, content='messages', content_rowid='id', tokenize='unicode61')`); err != nil {
		return fmt.Errorf("migrate: create new messages_fts: %w", err)
	}

	// Backfill from existing messages
	if _, err := db.Exec(`INSERT INTO messages_fts(rowid, content, tool_input, tool_result) SELECT id, content, COALESCE(tool_input, ''), COALESCE(tool_result, '') FROM messages`); err != nil {
		return fmt.Errorf("migrate: backfill messages_fts: %w", err)
	}

	// Recreate triggers
	createTriggers := []string{
		`CREATE TRIGGER messages_fts_insert AFTER INSERT ON messages BEGIN
			INSERT INTO messages_fts(rowid, content, tool_input, tool_result) VALUES (new.id, new.content, new.tool_input, new.tool_result);
		END`,
		`CREATE TRIGGER messages_fts_delete AFTER DELETE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, content, tool_input, tool_result) VALUES ('delete', old.id, old.content, old.tool_input, old.tool_result);
		END`,
		`CREATE TRIGGER messages_fts_update AFTER UPDATE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, content, tool_input, tool_result) VALUES ('delete', old.id, old.content, old.tool_input, old.tool_result);
			INSERT INTO messages_fts(rowid, content, tool_input, tool_result) VALUES (new.id, new.content, new.tool_input, new.tool_result);
		END`,
	}
	for _, stmt := range createTriggers {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: create new triggers: %w", err)
		}
	}

	slog.Info("migrate: messages_fts rebuilt with tool_input and tool_result")
	return nil
}

func MigrateTeamsTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS teams (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			settings TEXT DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS team_members (
			team_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT DEFAULT 'member',
			joined_at TEXT NOT NULL,
			PRIMARY KEY (team_id, user_id),
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS team_memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			team_id TEXT NOT NULL,
			memory_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			type TEXT DEFAULT 'conversation',
			visibility TEXT DEFAULT 'team',
			tags TEXT DEFAULT '[]',
			author_id TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_team_memories_team ON team_memories(team_id)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: create teams tables: %w", err)
		}
	}

	slog.Info("migrate: teams tables ensured")
	return nil
}

// MigrateSkillOutcomesDetails adds a details column to skill_outcomes if it
// doesn't already exist. The column stores free-text failure descriptions.
func MigrateSkillOutcomesDetails(db *sql.DB) error {
	var hasDetails bool
	rows, err := db.Query(`PRAGMA table_info(skill_outcomes)`)
	if err != nil {
		return fmt.Errorf("migrate: check skill_outcomes columns: %w", err)
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
		if name == "details" {
			hasDetails = true
			break
		}
	}
	if !hasDetails {
		if _, err := db.Exec(`ALTER TABLE skill_outcomes ADD COLUMN details TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate: add details to skill_outcomes: %w", err)
		}
		slog.Info("migrate: added details column to skill_outcomes")
	}
	return nil
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
