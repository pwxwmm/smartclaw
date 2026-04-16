package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateJSONToSQLite_ValidSession(t *testing.T) {
	s := newTestStore(t)
	storeDir := t.TempDir()

	sessionData := map[string]any{
		"id":         "migrate-test-1",
		"user_id":    "user-1",
		"created_at": time.Now().Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
		"model":      "claude-3",
		"tokens":     100,
		"cost":       0.05,
		"title":      "Migrated Session",
		"messages": []map[string]any{
			{"role": "user", "content": "Hello", "timestamp": time.Now().Format(time.RFC3339), "tokens": 10},
			{"role": "assistant", "content": "Hi there", "timestamp": time.Now().Format(time.RFC3339), "tokens": 20},
		},
	}

	data, err := json.Marshal(sessionData)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "migrate-test-1.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err = MigrateJSONToSQLite(storeDir, s)
	if err != nil {
		t.Fatalf("MigrateJSONToSQLite: %v", err)
	}

	got, err := s.GetSession("migrate-test-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil {
		t.Fatal("session should exist after migration")
	}
	if got.Title != "Migrated Session" {
		t.Errorf("Title = %q, want %q", got.Title, "Migrated Session")
	}
	if got.Model != "claude-3" {
		t.Errorf("Model = %q, want %q", got.Model, "claude-3")
	}

	msgs, err := s.GetSessionMessages("migrate-test-1")
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("message count = %d, want 2", len(msgs))
	}
}

func TestMigrateJSONSessions_EmptyDir(t *testing.T) {
	s := newTestStore(t)
	emptyDir := t.TempDir()

	migrated, err := s.MigrateJSONSessions(emptyDir)
	if err != nil {
		t.Fatalf("MigrateJSONSessions: %v", err)
	}
	if migrated != 0 {
		t.Errorf("migrated = %d, want 0", migrated)
	}
}

func TestMigrateJSONSessions_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	storeDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(storeDir, "bad.json"), []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	migrated, err := s.MigrateJSONSessions(storeDir)
	if err != nil {
		t.Fatalf("MigrateJSONSessions: %v", err)
	}
	if migrated != 0 {
		t.Errorf("migrated = %d, want 0 for invalid JSON", migrated)
	}
}

func TestMigrateJSONSessions_NonExistentDir(t *testing.T) {
	s := newTestStore(t)

	migrated, err := s.MigrateJSONSessions("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("MigrateJSONSessions with non-existent dir: %v", err)
	}
	if migrated != 0 {
		t.Errorf("migrated = %d, want 0", migrated)
	}
}

func TestMigrateJSONSessions_SkipsNonJSONFiles(t *testing.T) {
	s := newTestStore(t)
	storeDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(storeDir, "readme.txt"), []byte("not json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "data.csv"), []byte("a,b,c"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	migrated, err := s.MigrateJSONSessions(storeDir)
	if err != nil {
		t.Fatalf("MigrateJSONSessions: %v", err)
	}
	if migrated != 0 {
		t.Errorf("migrated = %d, want 0 when no .json files present", migrated)
	}
}

func TestMigrateJSONSessions_SkipsAlreadyExisting(t *testing.T) {
	s := newTestStore(t)
	storeDir := t.TempDir()

	sessionData := map[string]any{
		"id":       "existing-session",
		"user_id":  "user-1",
		"title":    "Original Title",
		"messages": []map[string]any{},
	}
	data, _ := json.Marshal(sessionData)
	if err := os.WriteFile(filepath.Join(storeDir, "existing-session.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	migrated, err := s.MigrateJSONSessions(storeDir)
	if err != nil {
		t.Fatalf("MigrateJSONSessions first call: %v", err)
	}
	if migrated != 1 {
		t.Errorf("first migration: migrated = %d, want 1", migrated)
	}

	migrated2, err := s.MigrateJSONSessions(storeDir)
	if err != nil {
		t.Fatalf("MigrateJSONSessions second call: %v", err)
	}
	if migrated2 != 0 {
		t.Errorf("second migration: migrated = %d, want 0 (already exists)", migrated2)
	}
}

func TestMigrateJSONSessions_FillsDefaults(t *testing.T) {
	s := newTestStore(t)
	storeDir := t.TempDir()

	sessionData := map[string]any{
		"id":       "minimal-session",
		"messages": []map[string]any{},
	}
	data, _ := json.Marshal(sessionData)
	if err := os.WriteFile(filepath.Join(storeDir, "minimal-session.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	migrated, err := s.MigrateJSONSessions(storeDir)
	if err != nil {
		t.Fatalf("MigrateJSONSessions: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("migrated = %d, want 1", migrated)
	}

	got, _ := s.GetSession("minimal-session")
	if got == nil {
		t.Fatal("session should exist")
	}
	if got.UserID != "default" {
		t.Errorf("UserID = %q, want %q (default)", got.UserID, "default")
	}
	if got.Source != "web" {
		t.Errorf("Source = %q, want %q", got.Source, "web")
	}
}

func TestMigrateJSONLToSQLite_ValidJSONL(t *testing.T) {
	s := newTestStore(t)
	jsonlDir := t.TempDir()

	session := &Session{
		ID:        "jsonl-target-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	lines := []string{
		`{"session_id":"jsonl-target-session","role":"user","content":"Hello from JSONL","timestamp":"2025-01-01T00:00:00Z"}`,
		`{"session_id":"jsonl-target-session","role":"assistant","content":"Hi back","timestamp":"2025-01-01T00:00:01Z"}`,
	}
	jsonlContent := ""
	for i, line := range lines {
		if i > 0 {
			jsonlContent += "\n"
		}
		jsonlContent += line
	}

	if err := os.WriteFile(filepath.Join(jsonlDir, "session.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := MigrateJSONLToSQLite(jsonlDir, s)
	if err != nil {
		t.Fatalf("MigrateJSONLToSQLite: %v", err)
	}

	msgs, err := s.GetSessionMessages("jsonl-target-session")
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(msgs) < 2 {
		t.Errorf("message count = %d, want at least 2", len(msgs))
	}
}

func TestMigrateJSONLToSQLite_EmptyJSONL(t *testing.T) {
	s := newTestStore(t)
	jsonlDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(jsonlDir, "empty.jsonl"), []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := MigrateJSONLToSQLite(jsonlDir, s)
	if err != nil {
		t.Fatalf("MigrateJSONLToSQLite: %v", err)
	}
}

func TestMigrateJSONLToSQLite_SkipsInvalidLines(t *testing.T) {
	s := newTestStore(t)
	jsonlDir := t.TempDir()

	session := &Session{
		ID:        "jsonl-mixed-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	jsonlContent := "not valid json\n" +
		`{"session_id":"jsonl-mixed-session","role":"user","content":"valid message","timestamp":"2025-01-01T00:00:00Z"}` + "\n" +
		"another bad line\n" +
		`{"session_id":"","role":"user","content":"no session id","timestamp":"2025-01-01T00:00:00Z"}` + "\n"

	if err := os.WriteFile(filepath.Join(jsonlDir, "mixed.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := MigrateJSONLToSQLite(jsonlDir, s)
	if err != nil {
		t.Fatalf("MigrateJSONLToSQLite: %v", err)
	}

	msgs, err := s.GetSessionMessages("jsonl-mixed-session")
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("message count = %d, want 1 (only valid line with session_id)", len(msgs))
	}
}

func TestMigrateJSONLToSQLite_NonExistentDir(t *testing.T) {
	s := newTestStore(t)

	err := MigrateJSONLToSQLite("/nonexistent/path/that/does/not/exist", s)
	if err != nil {
		t.Fatalf("MigrateJSONLToSQLite with non-existent dir: %v", err)
	}
}

func TestMigrateJSONLToSQLite_NilStore(t *testing.T) {
	err := MigrateJSONLToSQLite(t.TempDir(), nil)
	if err == nil {
		t.Error("MigrateJSONLToSQLite with nil store should return error")
	}
}

func TestMigrateJSONLToSQLite_SkipsNonJSONLFiles(t *testing.T) {
	s := newTestStore(t)
	jsonlDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(jsonlDir, "data.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := MigrateJSONLToSQLite(jsonlDir, s)
	if err != nil {
		t.Fatalf("MigrateJSONLToSQLite: %v", err)
	}
}

func TestMigrateUserObservationsUserID_AddsColumn(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()

	err := MigrateUserObservationsUserID(db)
	if err != nil {
		t.Fatalf("MigrateUserObservationsUserID: %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(user_observations)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()

	foundUserID := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if name == "user_id" {
			foundUserID = true
		}
	}
	if !foundUserID {
		t.Error("user_id column should exist after migration")
	}
}

func TestMigrateUserObservationsUserID_Idempotent(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()

	if err := MigrateUserObservationsUserID(db); err != nil {
		t.Fatalf("first MigrateUserObservationsUserID: %v", err)
	}
	if err := MigrateUserObservationsUserID(db); err != nil {
		t.Fatalf("second MigrateUserObservationsUserID (idempotent): %v", err)
	}
}

func TestMigrateUserObservationsUserID_InsertObservation(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()

	if err := MigrateUserObservationsUserID(db); err != nil {
		t.Fatalf("MigrateUserObservationsUserID: %v", err)
	}

	_, err := db.Exec(`INSERT INTO user_observations (category, key, value, user_id) VALUES (?, ?, ?, ?)`,
		"preference", "language", "Go", "default")
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	rows, err := db.Query(`SELECT key, value, user_id FROM user_observations WHERE category = ?`, "preference")
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected one observation row")
	}
	var key, value, userID string
	if err := rows.Scan(&key, &value, &userID); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if key != "language" || value != "Go" || userID != "default" {
		t.Errorf("got key=%q value=%q user_id=%q, want language/Go/default", key, value, userID)
	}
}
