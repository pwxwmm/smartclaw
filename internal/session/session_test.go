package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return &Manager{sessionsDir: dir}
}

func TestNewManager(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.sessionsDir == "" {
		t.Fatal("sessionsDir is empty")
	}
	expected := filepath.Join(dir, ".smartclaw", "sessions")
	if m.sessionsDir != expected {
		t.Fatalf("expected sessionsDir %q, got %q", expected, m.sessionsDir)
	}
}

func TestNewSession(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("claude-3-opus", "user-1")
	if s == nil {
		t.Fatal("NewSession returned nil")
	}
	if s.ID == "" {
		t.Fatal("session ID is empty")
	}
	if s.UserID != "user-1" {
		t.Fatalf("expected UserID %q, got %q", "user-1", s.UserID)
	}
	if s.Model != "claude-3-opus" {
		t.Fatalf("expected Model %q, got %q", "claude-3-opus", s.Model)
	}
	if s.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
	if s.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero")
	}
	if len(s.Messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(s.Messages))
	}
	if s.Tokens != 0 {
		t.Fatalf("expected 0 tokens, got %d", s.Tokens)
	}
	if s.Cost != 0 {
		t.Fatalf("expected 0 cost, got %f", s.Cost)
	}
}

func TestNewSessionEmptyModelAndUser(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("", "")
	if s.Model != "" {
		t.Fatalf("expected empty model, got %q", s.Model)
	}
	if s.UserID != "" {
		t.Fatalf("expected empty UserID, got %q", s.UserID)
	}
}

func TestSaveAndLoad(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-42")
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := m.Load(s.ID)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.ID != s.ID {
		t.Fatalf("expected ID %q, got %q", s.ID, loaded.ID)
	}
	if loaded.UserID != s.UserID {
		t.Fatalf("expected UserID %q, got %q", s.UserID, loaded.UserID)
	}
	if loaded.Model != s.Model {
		t.Fatalf("expected Model %q, got %q", s.Model, loaded.Model)
	}
}

func TestLoadNonexistent(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Load("nonexistent_id")
	if err == nil {
		t.Fatal("expected error loading nonexistent session")
	}
}

func TestSaveUpdatesTimestamp(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	s.UpdatedAt = time.Time{}
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := m.Load(s.ID)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set by Save")
	}
}

func TestDelete(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	if err := m.Delete(s.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	_, err := m.Load(s.ID)
	if err == nil {
		t.Fatal("expected error loading deleted session")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	m := newTestManager(t)

	err := m.Delete("nonexistent_id")
	if err == nil {
		t.Fatal("expected error deleting nonexistent session")
	}
}

func TestList(t *testing.T) {
	m := newTestManager(t)

	s1 := m.NewSession("model-a", "user-1")
	s2 := m.NewSession("model-b", "user-2")
	if err := m.Save(s1); err != nil {
		t.Fatalf("Save s1 error: %v", err)
	}
	if err := m.Save(s2); err != nil {
		t.Fatalf("Save s2 error: %v", err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	ids := make(map[string]bool)
	for _, si := range sessions {
		ids[si.ID] = true
	}
	if !ids[s1.ID] || !ids[s2.ID] {
		t.Fatal("expected both session IDs in list")
	}
}

func TestListEmpty(t *testing.T) {
	m := newTestManager(t)

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSortedByUpdatedAt(t *testing.T) {
	m := newTestManager(t)

	s1 := m.NewSession("model-a", "user-1")
	s1.UpdatedAt = time.Now().Add(-2 * time.Hour)
	if err := m.Save(s1); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	s2 := m.NewSession("model-b", "user-2")
	s2.UpdatedAt = time.Now()
	if err := m.Save(s2); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != s2.ID {
		t.Fatal("expected most recently updated session first")
	}
}

func TestListSessionInfo(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	s.Title = "Test Title"
	s.AddMessage("user", "hello")
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	si := sessions[0]
	if si.ID != s.ID {
		t.Fatalf("expected ID %q, got %q", s.ID, si.ID)
	}
	if si.UserID != "user-1" {
		t.Fatalf("expected UserID %q, got %q", "user-1", si.UserID)
	}
	if si.Model != "test-model" {
		t.Fatalf("expected Model %q, got %q", "test-model", si.Model)
	}
	if si.Title != "Test Title" {
		t.Fatalf("expected Title %q, got %q", "Test Title", si.Title)
	}
	if si.MessageCount != 1 {
		t.Fatalf("expected MessageCount 1, got %d", si.MessageCount)
	}
}

func TestListByUser(t *testing.T) {
	m := newTestManager(t)

	s1 := m.NewSession("model-a", "user-1")
	s2 := m.NewSession("model-b", "user-2")
	s3 := m.NewSession("model-c", "")
	for _, s := range []*Session{s1, s2, s3} {
		if err := m.Save(s); err != nil {
			t.Fatalf("Save error: %v", err)
		}
	}

	sessions, err := m.ListByUser("user-1")
	if err != nil {
		t.Fatalf("ListByUser error: %v", err)
	}

	// user-1 sees their own + sessions with empty UserID
	ids := make(map[string]bool)
	for _, si := range sessions {
		ids[si.ID] = true
	}
	if !ids[s1.ID] {
		t.Fatal("expected user-1's session")
	}
	if ids[s2.ID] {
		t.Fatal("did not expect user-2's session")
	}
	if !ids[s3.ID] {
		t.Fatal("expected session with empty UserID to be included for all users")
	}
}

func TestAddMessage(t *testing.T) {
	s := &Session{
		ID:        "test",
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.AddMessage("user", "Hello world")

	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.Messages))
	}
	msg := s.Messages[0]
	if msg.Role != "user" {
		t.Fatalf("expected role %q, got %q", "user", msg.Role)
	}
	if msg.Content != "Hello world" {
		t.Fatalf("expected content %q, got %q", "Hello world", msg.Content)
	}
	if msg.ID == "" {
		t.Fatal("message ID is empty")
	}
	if msg.Timestamp.IsZero() {
		t.Fatal("message timestamp is zero")
	}
}

func TestAddMessageAutoTitle(t *testing.T) {
	s := &Session{
		ID:        "test",
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.AddMessage("user", "This is my first message to the assistant")

	if s.Title != "This is my first message to the assistant" {
		t.Fatalf("expected title from first user message, got %q", s.Title)
	}
}

func TestAddMessageAutoTitleTruncation(t *testing.T) {
	s := &Session{
		ID:        "test",
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	longMsg := "This is a very long message that should be truncated because it exceeds the fifty character limit for titles"
	s.AddMessage("user", longMsg)

	if len(s.Title) > 53 {
		t.Fatalf("expected truncated title (max 53 chars), got %d chars: %q", len(s.Title), s.Title)
	}
	if s.Title[:50] != longMsg[:50] {
		t.Fatalf("title prefix mismatch")
	}
}

func TestAddMessageNoTitleForAssistant(t *testing.T) {
	s := &Session{
		ID:        "test",
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.AddMessage("assistant", "Hello! How can I help?")
	if s.Title != "" {
		t.Fatalf("expected empty title for assistant message, got %q", s.Title)
	}

	s.AddMessage("user", "My first question")
	if s.Title != "" {
		t.Fatalf("expected no auto-title since it's not the first message, got %q", s.Title)
	}
}

func TestAddMessageUpdatesTimestamp(t *testing.T) {
	s := &Session{
		ID:        "test",
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	beforeAdd := s.UpdatedAt
	s.AddMessage("user", "test")
	if !s.UpdatedAt.After(beforeAdd) {
		t.Fatal("expected UpdatedAt to be updated after AddMessage")
	}
}

func TestCleanExpired(t *testing.T) {
	m := newTestManager(t)

	old := m.NewSession("model", "user-1")
	old.UpdatedAt = time.Now().Add(-2 * DefaultSessionTTL)
	if err := m.Save(old); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	recent := m.NewSession("model", "user-2")
	recent.UpdatedAt = time.Now()
	if err := m.Save(recent); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	cleaned, err := m.CleanExpired(DefaultSessionTTL)
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned session, got %d", cleaned)
	}

	_, err = m.Load(old.ID)
	if err == nil {
		t.Fatal("expected old session to be deleted")
	}

	_, err = m.Load(recent.ID)
	if err != nil {
		t.Fatal("expected recent session to still exist")
	}
}

func TestCleanExpiredZeroTTL(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("model", "user-1")
	s.UpdatedAt = time.Now().Add(-1 * time.Hour)
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	cleaned, err := m.CleanExpired(0)
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned (1 hour old < 30 day TTL), got %d", cleaned)
	}
}

func TestCleanExpiredNegativeTTL(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("model", "user-1")
	s.UpdatedAt = time.Now().Add(-1 * time.Hour)
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	cleaned, err := m.CleanExpired(-1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned (negative TTL falls back to default), got %d", cleaned)
	}
}

func TestCleanExpiredCustomTTL(t *testing.T) {
	m := newTestManager(t)

	old := m.NewSession("model", "user-1")
	old.UpdatedAt = time.Now().Add(-2 * time.Hour)
	if err := m.Save(old); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	recent := m.NewSession("model", "user-2")
	recent.UpdatedAt = time.Now().Add(-30 * time.Minute)
	if err := m.Save(recent); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	cleaned, err := m.CleanExpired(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned, got %d", cleaned)
	}
}

func TestCleanExpiredEmptyDir(t *testing.T) {
	m := newTestManager(t)

	cleaned, err := m.CleanExpired(DefaultSessionTTL)
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned for empty dir, got %d", cleaned)
	}
}

func TestStartCleanup(t *testing.T) {
	m := newTestManager(t)

	stop := m.StartCleanup(1 * time.Hour)
	if stop == nil {
		t.Fatal("expected stop function")
	}
	stop()
}

func TestStartCleanupRemovesExpired(t *testing.T) {
	m := newTestManager(t)

	old := m.NewSession("model", "user-1")
	old.UpdatedAt = time.Now().Add(-2 * time.Hour)
	if err := m.Save(old); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	cleaned, err := m.CleanExpired(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned, got %d", cleaned)
	}
}

func TestExportJSON(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	s.Title = "Test Export"
	s.AddMessage("user", "hello")
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	exported, err := m.Export(s.ID, "json")
	if err != nil {
		t.Fatalf("Export json error: %v", err)
	}

	var parsed Session
	if err := json.Unmarshal([]byte(exported), &parsed); err != nil {
		t.Fatalf("failed to parse exported JSON: %v", err)
	}
	if parsed.ID != s.ID {
		t.Fatalf("expected ID %q, got %q", s.ID, parsed.ID)
	}
}

func TestExportMarkdown(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	s.Title = "Markdown Test"
	s.Cost = 0.05
	s.Tokens = 100
	s.AddMessage("user", "hello world")
	s.AddMessage("assistant", "hi there")
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	exported, err := m.Export(s.ID, "markdown")
	if err != nil {
		t.Fatalf("Export markdown error: %v", err)
	}
	if exported == "" {
		t.Fatal("expected non-empty markdown export")
	}
}

func TestExportMarkdownAlias(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	s.Title = "MD Alias Test"
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	exported, err := m.Export(s.ID, "md")
	if err != nil {
		t.Fatalf("Export md error: %v", err)
	}
	if exported == "" {
		t.Fatal("expected non-empty md export")
	}
}

func TestExportUnsupportedFormat(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	_, err := m.Export(s.ID, "xml")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestExportNonexistent(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Export("nonexistent", "json")
	if err == nil {
		t.Fatal("expected error exporting nonexistent session")
	}
}

func TestRename(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("test-model", "user-1")
	s.Title = "Old Title"
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	if err := m.Rename(s.ID, "New Title"); err != nil {
		t.Fatalf("Rename error: %v", err)
	}

	loaded, err := m.Load(s.ID)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Title != "New Title" {
		t.Fatalf("expected title %q, got %q", "New Title", loaded.Title)
	}
}

func TestRenameNonexistent(t *testing.T) {
	m := newTestManager(t)

	err := m.Rename("nonexistent", "Title")
	if err == nil {
		t.Fatal("expected error renaming nonexistent session")
	}
}

func TestGetSessionsDir(t *testing.T) {
	m := newTestManager(t)

	dir := m.GetSessionsDir()
	if dir == "" {
		t.Fatal("expected non-empty sessions dir")
	}
}

func TestListSkipsNonJSON(t *testing.T) {
	m := newTestManager(t)

	if err := os.WriteFile(filepath.Join(m.sessionsDir, "readme.txt"), []byte("not a session"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(m.sessionsDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions (non-JSON files should be skipped), got %d", len(sessions))
	}
}

func TestListSkipsCorruptJSON(t *testing.T) {
	m := newTestManager(t)

	if err := os.WriteFile(filepath.Join(m.sessionsDir, "corrupt.json"), []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions (corrupt files should be skipped), got %d", len(sessions))
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := newTestManager(t)

	var wg sync.WaitGroup
	const goroutines = 10

	ids := make(chan string, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := m.NewSession("model", "user-concurrent")
			s.Title = "Concurrent Session"
			if err := m.Save(s); err != nil {
				t.Errorf("Save error in goroutine %d: %v", i, err)
				return
			}
			ids <- s.ID
		}(i)
	}
	wg.Wait()
	close(ids)

	var loadWg sync.WaitGroup
	for id := range ids {
		loadWg.Add(1)
		go func(id string) {
			defer loadWg.Done()
			_, err := m.Load(id)
			if err != nil {
				t.Errorf("Load error for %s: %v", id, err)
			}
		}(id)
	}
	loadWg.Wait()

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(sessions) != goroutines {
		t.Fatalf("expected %d sessions, got %d", goroutines, len(sessions))
	}
}

func TestSessionFields(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("claude-3-opus", "user-42")
	s.Title = "Test Session"
	s.Tokens = 1500
	s.Cost = 0.75
	s.AddMessage("user", "first message")
	s.AddMessage("assistant", "second message")

	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := m.Load(s.ID)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.Title != "Test Session" {
		t.Fatalf("expected Title %q, got %q", "Test Session", loaded.Title)
	}
	if loaded.Tokens != 1500 {
		t.Fatalf("expected Tokens 1500, got %d", loaded.Tokens)
	}
	if loaded.Cost != 0.75 {
		t.Fatalf("expected Cost 0.75, got %f", loaded.Cost)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded.Messages))
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()

	if id1 == "" {
		t.Fatal("expected non-empty session ID")
	}
	if id1 == id2 {
		t.Fatal("expected unique session IDs")
	}
}

func TestExportMarkdownContent(t *testing.T) {
	s := &Session{
		Title:     "Markdown Test",
		CreatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Model:     "test-model",
		Tokens:    500,
		Cost:      0.25,
		Messages: []Message{
			{Role: "user", Content: "Hello assistant", Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
			{Role: "assistant", Content: "Hello user", Timestamp: time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC)},
		},
	}

	result := exportMarkdown(s)
	if result == "" {
		t.Fatal("expected non-empty markdown")
	}
}

func TestCleanExpiredWithNonJSONFiles(t *testing.T) {
	m := newTestManager(t)

	if err := os.WriteFile(filepath.Join(m.sessionsDir, "notes.txt"), []byte("not a session"), 0644); err != nil {
		t.Fatal(err)
	}

	cleaned, err := m.CleanExpired(DefaultSessionTTL)
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned, got %d", cleaned)
	}
}

func TestListByUserEmpty(t *testing.T) {
	m := newTestManager(t)

	sessions, err := m.ListByUser("user-1")
	if err != nil {
		t.Fatalf("ListByUser error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestMultipleMessages(t *testing.T) {
	s := &Session{
		ID:        "test",
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for i := 0; i < 5; i++ {
		s.AddMessage("user", "message")
	}

	if len(s.Messages) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(s.Messages))
	}
}

func TestSessionIDFormat(t *testing.T) {
	m := newTestManager(t)
	s := m.NewSession("model", "user")

	if len(s.ID) < 10 {
		t.Fatalf("session ID seems too short: %q", s.ID)
	}
}

func TestSaveOverrideExisting(t *testing.T) {
	m := newTestManager(t)

	s := m.NewSession("model", "user-1")
	s.Title = "Original"
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	s.Title = "Updated"
	if err := m.Save(s); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := m.Load(s.ID)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Title != "Updated" {
		t.Fatalf("expected title %q, got %q", "Updated", loaded.Title)
	}
}

func TestListSessionInfoSorted(t *testing.T) {
	m := newTestManager(t)

	s1 := m.NewSession("model-a", "user-1")
	s1.UpdatedAt = time.Now().Add(-3 * time.Hour)
	if err := m.Save(s1); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	s2 := m.NewSession("model-b", "user-2")
	s2.UpdatedAt = time.Now()
	if err := m.Save(s2); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	s3 := m.NewSession("model-c", "user-3")
	s3.UpdatedAt = time.Now().Add(-1 * time.Hour)
	if err := m.Save(s3); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	// Should be sorted by UpdatedAt descending
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	for i := 1; i < len(sessions); i++ {
		if sessions[i].UpdatedAt.After(sessions[i-1].UpdatedAt) {
			t.Fatalf("sessions not sorted by UpdatedAt descending at index %d", i)
		}
	}
}

func TestConcurrentCreateAndDelete(t *testing.T) {
	m := newTestManager(t)

	var wg sync.WaitGroup
	const count = 5

	sessions := make([]*Session, count)
	for i := 0; i < count; i++ {
		s := m.NewSession("model", "user-1")
		s.Title = "Test"
		if err := m.Save(s); err != nil {
			t.Fatalf("Save error: %v", err)
		}
		sessions[i] = s
	}

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := m.Delete(id); err != nil {
				t.Errorf("Delete error: %v", err)
			}
		}(sessions[i].ID)
	}
	wg.Wait()

	list, err := m.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 sessions after deletion, got %d", len(list))
	}
}

func TestSortSliceForList(t *testing.T) {
	sessions := []SessionInfo{
		{ID: "a", UpdatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "b", UpdatedAt: time.Now()},
		{ID: "c", UpdatedAt: time.Now().Add(-1 * time.Hour)},
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	if sessions[0].ID != "b" {
		t.Fatalf("expected most recent first, got %q", sessions[0].ID)
	}
}
