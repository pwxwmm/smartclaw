package store

import (
	"context"
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

	if err := s.UpsertSession(context.Background(), session); err != nil {
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
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession v1: %v", err)
	}

	session.Title = "V2"
	session.UpdatedAt = time.Now()
	if err := s.UpsertSession(context.Background(), session); err != nil {
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
		if err := s.UpsertSession(context.Background(), session); err != nil {
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
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	msgs := []*Message{
		{SessionID: "msg-test-session", Role: "user", Content: "Hello", Timestamp: time.Now()},
		{SessionID: "msg-test-session", Role: "assistant", Content: "Hi there", Timestamp: time.Now()},
	}

	for _, msg := range msgs {
		if err := s.InsertMessage(context.Background(), msg); err != nil {
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
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	msgs := []*Message{
		{SessionID: "search-test-session", Role: "user", Content: "debug the Go test failure in auth package", Timestamp: time.Now()},
		{SessionID: "search-test-session", Role: "assistant", Content: "I found the bug in the auth middleware", Timestamp: time.Now()},
		{SessionID: "search-test-session", Role: "user", Content: "deploy to production", Timestamp: time.Now()},
	}

	for _, msg := range msgs {
		if err := s.InsertMessage(context.Background(), msg); err != nil {
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
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	if err := s.DeleteSession(context.Background(), "delete-test-session"); err != nil {
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
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	for i := 0; i < 5; i++ {
		s.InsertMessage(context.Background(), &Message{SessionID: "count-test-session", Role: "user", Content: "msg", Timestamp: time.Now()})
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

func TestUpsertAndGetSkill(t *testing.T) {
	s := newTestStore(t)

	skill := &SkillRecord{
		Name:        "test-skill",
		Description: "A test skill",
		Content:     "# Test Skill\nDescription",
		Source:      "learned",
		UseCount:    0,
	}

	if err := s.UpsertSkill(context.Background(), skill); err != nil {
		t.Fatalf("UpsertSkill: %v", err)
	}

	skills, err := s.GetAllLearnedSkills()
	if err != nil {
		t.Fatalf("GetAllLearnedSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "test-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "test-skill")
	}
}

func TestIncrementSkillUseCount(t *testing.T) {
	s := newTestStore(t)

	skill := &SkillRecord{
		Name:    "use-count-test",
		Source:  "learned",
		Content: "test",
	}
	if err := s.UpsertSkill(context.Background(), skill); err != nil {
		t.Fatalf("UpsertSkill: %v", err)
	}

	if err := s.IncrementSkillUseCount(context.Background(), "use-count-test"); err != nil {
		t.Fatalf("IncrementSkillUseCount: %v", err)
	}
	if err := s.IncrementSkillUseCount(context.Background(), "use-count-test"); err != nil {
		t.Fatalf("IncrementSkillUseCount 2: %v", err)
	}

	skills, _ := s.GetAllLearnedSkills()
	if len(skills) != 1 || skills[0].UseCount != 2 {
		t.Errorf("UseCount = %d, want 2", skills[0].UseCount)
	}
}

func TestGetStaleSkills(t *testing.T) {
	s := newTestStore(t)

	skill := &SkillRecord{
		Name:    "stale-skill",
		Source:  "learned",
		Content: "test",
	}
	if err := s.UpsertSkill(context.Background(), skill); err != nil {
		t.Fatalf("UpsertSkill: %v", err)
	}

	stale, err := s.GetStaleSkills(24 * time.Hour)
	if err != nil {
		t.Fatalf("GetStaleSkills: %v", err)
	}
	if len(stale) != 1 {
		t.Errorf("expected 1 stale skill, got %d", len(stale))
	}
	if stale[0].Name != "stale-skill" {
		t.Errorf("stale skill name = %q, want %q", stale[0].Name, "stale-skill")
	}
}

func TestDeleteSkill(t *testing.T) {
	s := newTestStore(t)

	skill := &SkillRecord{
		Name:    "delete-test",
		Source:  "learned",
		Content: "test",
	}
	if err := s.UpsertSkill(context.Background(), skill); err != nil {
		t.Fatalf("UpsertSkill: %v", err)
	}

	if err := s.DeleteSkill("delete-test"); err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}

	skills, _ := s.GetAllLearnedSkills()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills after delete, got %d", len(skills))
	}
}

func TestToolCallsColumn(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "toolcalls-test-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	msg := &Message{
		SessionID: "toolcalls-test-session",
		Role:      "assistant",
		Content:   "I will run a tool",
		ToolCalls: `[{"id":"call_1","type":"function","function":{"name":"bash","arguments":"ls -la"}}]`,
		Timestamp: time.Now(),
	}
	if err := s.InsertMessage(context.Background(), msg); err != nil {
		t.Fatalf("InsertMessage with tool_calls: %v", err)
	}

	got, err := s.GetSessionMessages("toolcalls-test-session")
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	if got[0].ToolCalls != msg.ToolCalls {
		t.Errorf("ToolCalls = %q, want %q", got[0].ToolCalls, msg.ToolCalls)
	}
}

func TestSessionsFTS(t *testing.T) {
	s := newTestStore(t)

	sessions := []*Session{
		{ID: "fts-sess-1", UserID: "user-1", Title: "Debugging Go authentication", Summary: "Resolved JWT token expiry bug in middleware", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "fts-sess-2", UserID: "user-1", Title: "Deploy to production", Summary: "Deployed v2.3 to kubernetes cluster", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "fts-sess-3", UserID: "user-2", Title: "Writing documentation", Summary: "Updated API docs for authentication endpoints", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	for _, sess := range sessions {
		if err := s.UpsertSession(context.Background(), sess); err != nil {
			t.Fatalf("UpsertSession: %v", err)
		}
	}

	results, err := s.SearchSessions("authentication", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) == 0 {
		t.Error("SearchSessions should find results for 'authentication'")
	}

	found := false
	for _, r := range results {
		if r.SessionID == "fts-sess-1" || r.SessionID == "fts-sess-3" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SearchSessions should match sessions with 'authentication' in title or summary")
	}

	results2, err := s.SearchSessions("kubernetes", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results2) == 0 {
		t.Error("SearchSessions should find results for 'kubernetes'")
	}
	if len(results2) > 0 && results2[0].SessionID != "fts-sess-2" {
		t.Errorf("SearchSessions kubernetes result = %q, want fts-sess-2", results2[0].SessionID)
	}
}

func TestMessagesFTS_ToolFields(t *testing.T) {
	s := newTestStore(t)

	session := &Session{
		ID:        "toolfts-test-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	msgs := []*Message{
		{
			SessionID:  "toolfts-test-session",
			Role:       "assistant",
			Content:    "I will read the config file",
			ToolName:   "read_file",
			ToolInput:  `{"path": "/etc/app/config.yaml"}`,
			ToolResult: "database_url: postgres://localhost:5432/myapp",
			Timestamp:  time.Now(),
		},
		{
			SessionID:  "toolfts-test-session",
			Role:       "assistant",
			Content:    "Running database migration",
			ToolName:   "bash",
			ToolInput:  `{"command": "migrate -path ./migrations -database $DB_URL up"}`,
			ToolResult: "Migration completed successfully",
			Timestamp:  time.Now(),
		},
		{
			SessionID: "toolfts-test-session",
			Role:      "user",
			Content:   "Check the logs for errors",
			Timestamp: time.Now(),
		},
	}

	for _, msg := range msgs {
		if err := s.InsertMessage(context.Background(), msg); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}

	results, err := s.SearchMessages("config", 10)
	if err != nil {
		t.Fatalf("SearchMessages for tool_input: %v", err)
	}
	if len(results) == 0 {
		t.Error("FTS5 search should find 'config' in tool_input field")
	}

	results2, err := s.SearchMessages("postgres", 10)
	if err != nil {
		t.Fatalf("SearchMessages for tool_result: %v", err)
	}
	if len(results2) == 0 {
		t.Error("FTS5 search should find 'postgres' in tool_result field")
	}

	results3, err := s.SearchMessages("migration", 10)
	if err != nil {
		t.Fatalf("SearchMessages for combined content+tool: %v", err)
	}
	if len(results3) == 0 {
		t.Error("FTS5 search should find 'migration' in content or tool fields")
	}
}

func TestMessagesFTS_Rebuild(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()

	session := &Session{
		ID:        "rebuild-test-session",
		UserID:    "user-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	msg := &Message{
		SessionID:  "rebuild-test-session",
		Role:       "assistant",
		Content:    "Checking server health",
		ToolName:   "bash",
		ToolInput:  `{"command": "curl http://localhost:8080/health"}`,
		ToolResult: `{"status": "ok", "uptime": 3600}`,
		Timestamp:  time.Now(),
	}
	if err := s.InsertMessage(context.Background(), msg); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	// Verify search works before rebuild
	results, err := s.SearchMessages("health", 10)
	if err != nil {
		t.Fatalf("SearchMessages before rebuild: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search should find 'health' before rebuild")
	}

	// Run the migration (should be idempotent since schema already has the new columns)
	if err := MigrateMessagesFTSExtended(db); err != nil {
		t.Fatalf("MigrateMessagesFTSExtended: %v", err)
	}

	// Verify search still works after rebuild
	results2, err := s.SearchMessages("health", 10)
	if err != nil {
		t.Fatalf("SearchMessages after rebuild: %v", err)
	}
	if len(results2) == 0 {
		t.Error("Search should still find 'health' after rebuild")
	}

	// Verify tool fields are still searchable
	results3, err := s.SearchMessages("uptime", 10)
	if err != nil {
		t.Fatalf("SearchMessages for tool_result after rebuild: %v", err)
	}
	if len(results3) == 0 {
		t.Error("Search should find 'uptime' in tool_result after rebuild")
	}
}

func TestSearchSessions(t *testing.T) {
	s := newTestStore(t)

	sessions := []*Session{
		{ID: "search-sess-1", UserID: "user-1", Title: "Refactor database layer", Summary: "Replaced ORM with raw SQL queries for performance", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "search-sess-2", UserID: "user-1", Title: "Fix memory leak", Summary: "Fixed goroutine leak in WebSocket handler", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "search-sess-3", UserID: "user-2", Title: "Add caching layer", Summary: "Implemented Redis caching for database queries", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	for _, sess := range sessions {
		if err := s.UpsertSession(context.Background(), sess); err != nil {
			t.Fatalf("UpsertSession: %v", err)
		}
	}

	results, err := s.SearchSessions("database", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("SearchSessions 'database' found %d results, want at least 2", len(results))
	}

	for _, r := range results {
		if r.SessionID == "" {
			t.Error("SessionSearchResult.SessionID should not be empty")
		}
		if r.Rank == 0 {
			t.Error("SessionSearchResult.Rank should not be zero")
		}
	}

	emptyResults, err := s.SearchSessions("nonexistent_xyz_12345", 10)
	if err != nil {
		t.Fatalf("SearchSessions for nonexistent: %v", err)
	}
	if len(emptyResults) != 0 {
		t.Errorf("SearchSessions for nonexistent should return 0 results, got %d", len(emptyResults))
	}

	nilResults, err := s.SearchSessions("", 10)
	if err != nil {
		t.Fatalf("SearchSessions for empty: %v", err)
	}
	if nilResults != nil {
		t.Error("SearchSessions for empty query should return nil")
	}
}

func TestSearchSessionsByUser(t *testing.T) {
	s := newTestStore(t)

	sessions := []*Session{
		{ID: "user-sess-1", UserID: "alice", Title: "Setup CI pipeline", Summary: "Configured GitHub Actions for automated testing", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "user-sess-2", UserID: "bob", Title: "Setup deployment", Summary: "Configured GitHub Actions for deployment", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "user-sess-3", UserID: "alice", Title: "Code review", Summary: "Reviewed PR for authentication module", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	for _, sess := range sessions {
		if err := s.UpsertSession(context.Background(), sess); err != nil {
			t.Fatalf("UpsertSession: %v", err)
		}
	}

	results, err := s.SearchSessionsByUser("GitHub", "alice", 10)
	if err != nil {
		t.Fatalf("SearchSessionsByUser: %v", err)
	}

	for _, r := range results {
		if r.SessionID == "user-sess-2" {
			t.Error("SearchSessionsByUser should not return bob's session")
		}
	}

	if len(results) == 0 {
		t.Error("SearchSessionsByUser should find alice's GitHub sessions")
	}

	defaultResults, err := s.SearchSessionsByUser("GitHub", "default", 10)
	if err != nil {
		t.Fatalf("SearchSessionsByUser with default user: %v", err)
	}
	if len(defaultResults) < 2 {
		t.Errorf("SearchSessionsByUser with default user should search all sessions, got %d", len(defaultResults))
	}
}
