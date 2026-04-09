package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewStore(t *testing.T) {
	s := newTestStore(t)

	dbPath := s.DBPath()
	if dbPath == "" {
		t.Error("DBPath should not be empty")
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should exist")
	}
}

func TestUpsertAndGetSession(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "test-session-1",
		UserID:    "user-1",
		Source:    "cli",
		Model:     "claude-3",
		Title:     "Test Session",
		Tokens:    100,
		Cost:      0.05,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.UpsertSession(session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	got, err := s.GetSession("test-session-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil {
		t.Fatal("GetSession should return session")
	}
	if got.ID != "test-session-1" {
		t.Errorf("ID = %q, want %q", got.ID, "test-session-1")
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", got.UserID, "user-1")
	}
	if got.Model != "claude-3" {
		t.Errorf("Model = %q, want %q", got.Model, "claude-3")
	}
}

func TestUpsertSessionUpdate(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "test-session-1",
		UserID:    "user-1",
		Title:     "V1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(session); err != nil {
		t.Fatalf("UpsertSession v1: %v", err)
	}

	session.Title = "V2"
	session.UpdatedAt = time.Now()
	if err := s.UpsertSession(session); err != nil {
		t.Fatalf("UpsertSession v2: %v", err)
	}

	got, _ := s.GetSession("test-session-1")
	if got.Title != "V2" {
		t.Errorf("Title = %q, want %q", got.Title, "V2")
	}
}

func TestListSessions(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 3; i++ {
		session := &Session{
			ID:        filepath.Base(t.TempDir()) + "-" + time.Now().Format("150405") + "-" + string(rune('A'+i)),
			UserID:    "user-1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.UpsertSession(session); err != nil {
			t.Fatalf("UpsertSession %d: %v", i, err)
		}
	}

	sessions, err := s.ListSessions("user-1", 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("ListSessions count = %d, want 3", len(sessions))
	}
}

func TestGetSessionNotFound(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetSession("nonexistent")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got != nil {
		t.Error("GetSession should return nil for nonexistent")
	}
}

func TestInsertAndGetMessages(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "msg-test-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	msgs := []*Message{
		{SessionID: "msg-test-session", Role: "user", Content: "Hello", Timestamp: time.Now()},
		{SessionID: "msg-test-session", Role: "assistant", Content: "Hi there", Timestamp: time.Now()},
	}

	for _, msg := range msgs {
		if _, err := s.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}

	got, err := s.GetSessionMessages("msg-test-session")
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("message count = %d, want 2", len(got))
	}
}

func TestFTS5Search(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "search-test-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	msgs := []*Message{
		{SessionID: "search-test-session", Role: "user", Content: "debug the Go test failure in auth package", Timestamp: time.Now()},
		{SessionID: "search-test-session", Role: "assistant", Content: "I found the bug in the auth middleware", Timestamp: time.Now()},
		{SessionID: "search-test-session", Role: "user", Content: "deploy to production", Timestamp: time.Now()},
	}

	for _, msg := range msgs {
		if _, err := s.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}

	results, err := s.SearchMessages("debug", 10)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(results) == 0 {
		t.Error("FTS5 search should find results for 'debug'")
	}

	results2, err := s.SearchMessages("auth", 10)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if len(results2) == 0 {
		t.Error("FTS5 search should find results for 'auth'")
	}
}

func TestSearchMessagesEmptyQuery(t *testing.T) {
	s := newTestStore(t)

	results, err := s.SearchMessages("", 10)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if results != nil {
		t.Error("empty query should return nil")
	}
}

func TestDeleteSession(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "delete-test-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	if err := s.DeleteSession("delete-test-session"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	got, _ := s.GetSession("delete-test-session")
	if got != nil {
		t.Error("session should be deleted")
	}
}

func TestGetMessageCount(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "count-test-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	for i := 0; i < 5; i++ {
		s.InsertMessage(&Message{SessionID: "count-test-session", Role: "user", Content: "msg", Timestamp: time.Now()})
	}

	count, err := s.GetMessageCount("count-test-session")
	if err != nil {
		t.Fatalf("GetMessageCount: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

func TestJSONLWriter(t *testing.T) {
	dir := t.TempDir()
	writer := NewJSONLWriter(dir)

	msg := &Message{
		ID:        1,
		SessionID: "jsonl-test",
		Role:      "user",
		Content:   "test message",
		Timestamp: time.Now(),
	}

	if err := writer.Append(msg); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Error("JSONL file should be created")
	}
}
