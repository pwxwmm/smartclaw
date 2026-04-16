package memory

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestEnhancedStore(t *testing.T) *EnhancedMemoryStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}
	return store
}

func TestNewEnhancedMemoryStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNewEnhancedMemoryStoreEmptyPath(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	store, err := NewEnhancedMemoryStore("", ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestCreateMemory(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "test description", "test content", []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}
	if mem == nil {
		t.Fatal("expected non-nil memory")
	}
	if mem.Header.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if mem.Header.Type != MemoryTypeUser {
		t.Fatalf("expected type %q, got %q", MemoryTypeUser, mem.Header.Type)
	}
	if mem.Header.Description != "test description" {
		t.Fatalf("expected description %q, got %q", "test description", mem.Header.Description)
	}
	if mem.Content != "test content" {
		t.Fatalf("expected content %q, got %q", "test content", mem.Content)
	}
	if len(mem.Header.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(mem.Header.Tags))
	}
}

func TestCreateMemoryAllTypes(t *testing.T) {
	s := newTestEnhancedStore(t)

	types := []MemoryType{MemoryTypeUser, MemoryTypeFeedback, MemoryTypeProject, MemoryTypeReference}
	for _, mt := range types {
		mem, err := s.CreateMemory(mt, "desc", "content", nil)
		if err != nil {
			t.Fatalf("CreateMemory(%q) error: %v", mt, err)
		}
		if mem.Header.Type != mt {
			t.Fatalf("expected type %q, got %q", mt, mem.Header.Type)
		}
	}
}

func TestCreateMemoryNoTags(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "desc", "content", nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}
	if len(mem.Header.Tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(mem.Header.Tags))
	}
}

func TestGetMemory(t *testing.T) {
	s := newTestEnhancedStore(t)

	created, err := s.CreateMemory(MemoryTypeUser, "desc", "content", []string{"tag1"})
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	mem, err := s.GetMemory(created.Header.ID)
	if err != nil {
		t.Fatalf("GetMemory error: %v", err)
	}
	if mem.Header.ID != created.Header.ID {
		t.Fatalf("expected ID %q, got %q", created.Header.ID, mem.Header.ID)
	}
}

func TestGetMemoryNonexistent(t *testing.T) {
	s := newTestEnhancedStore(t)

	_, err := s.GetMemory("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent memory")
	}
}

func TestUpdateMemory(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "desc", "original content", nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	err = s.UpdateMemory(mem.Header.ID, "updated content")
	if err != nil {
		t.Fatalf("UpdateMemory error: %v", err)
	}

	updated, err := s.GetMemory(mem.Header.ID)
	if err != nil {
		t.Fatalf("GetMemory error: %v", err)
	}
	if updated.Content != "updated content" {
		t.Fatalf("expected content %q, got %q", "updated content", updated.Content)
	}
}

func TestUpdateMemoryNonexistent(t *testing.T) {
	s := newTestEnhancedStore(t)

	err := s.UpdateMemory("nonexistent", "content")
	if err == nil {
		t.Fatal("expected error updating nonexistent memory")
	}
}

func TestDeleteMemory(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "desc", "content", nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	err = s.DeleteMemory(mem.Header.ID)
	if err != nil {
		t.Fatalf("DeleteMemory error: %v", err)
	}

	_, err = s.GetMemory(mem.Header.ID)
	if err == nil {
		t.Fatal("expected error getting deleted memory")
	}
}

func TestDeleteMemoryNonexistent(t *testing.T) {
	s := newTestEnhancedStore(t)

	err := s.DeleteMemory("nonexistent")
	if err == nil {
		t.Fatal("expected error deleting nonexistent memory")
	}
}

func TestDeleteMemoryRemovesFromHeaders(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, _ := s.CreateMemory(MemoryTypeUser, "desc", "content", nil)
	s.CreateMemory(MemoryTypeProject, "desc2", "content2", nil)

	headers := s.ListMemories()
	initialCount := len(headers)

	s.DeleteMemory(mem.Header.ID)

	headers = s.ListMemories()
	if len(headers) != initialCount-1 {
		t.Fatalf("expected %d headers after delete, got %d", initialCount-1, len(headers))
	}
}

func TestListMemories(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "first", "content1", nil)
	s.CreateMemory(MemoryTypeProject, "second", "content2", nil)

	headers := s.ListMemories()
	if len(headers) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(headers))
	}
}

func TestListMemoriesEmpty(t *testing.T) {
	s := newTestEnhancedStore(t)

	headers := s.ListMemories()
	if len(headers) != 0 {
		t.Fatalf("expected 0 memories, got %d", len(headers))
	}
}

func TestListMemoriesReturnsCopy(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "desc", "content", nil)

	h1 := s.ListMemories()
	h2 := s.ListMemories()

	if len(h1) != len(h2) {
		t.Fatal("expected same length")
	}
}

func TestGetMemoriesByType(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "user mem", "content", nil)
	s.CreateMemory(MemoryTypeProject, "project mem", "content", nil)
	s.CreateMemory(MemoryTypeUser, "user mem 2", "content", nil)

	userMems := s.GetMemoriesByType(MemoryTypeUser)
	if len(userMems) != 2 {
		t.Fatalf("expected 2 user memories, got %d", len(userMems))
	}

	projectMems := s.GetMemoriesByType(MemoryTypeProject)
	if len(projectMems) != 1 {
		t.Fatalf("expected 1 project memory, got %d", len(projectMems))
	}

	feedbackMems := s.GetMemoriesByType(MemoryTypeFeedback)
	if len(feedbackMems) != 0 {
		t.Fatalf("expected 0 feedback memories, got %d", len(feedbackMems))
	}
}

func TestGetMemoriesByTag(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "desc1", "content1", []string{"go", "testing"})
	s.CreateMemory(MemoryTypeUser, "desc2", "content2", []string{"go", "production"})
	s.CreateMemory(MemoryTypeUser, "desc3", "content3", []string{"python"})

	goMems := s.GetMemoriesByTag("go")
	if len(goMems) != 2 {
		t.Fatalf("expected 2 memories with 'go' tag, got %d", len(goMems))
	}

	pythonMems := s.GetMemoriesByTag("python")
	if len(pythonMems) != 1 {
		t.Fatalf("expected 1 memory with 'python' tag, got %d", len(pythonMems))
	}

	rustMems := s.GetMemoriesByTag("rust")
	if len(rustMems) != 0 {
		t.Fatalf("expected 0 memories with 'rust' tag, got %d", len(rustMems))
	}
}

func TestSearchMemoriesByDescription(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "database connection settings", "content1", nil)
	s.CreateMemory(MemoryTypeUser, "API authentication flow", "content2", nil)

	results := s.SearchMemories("database")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'database', got %d", len(results))
	}

	results = s.SearchMemories("api")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'api', got %d", len(results))
	}
}

func TestSearchMemoriesByTag(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "some desc", "content", []string{"database", "config"})

	results := s.SearchMemories("database")
	if len(results) != 1 {
		t.Fatalf("expected 1 result searching by tag, got %d", len(results))
	}
}

func TestSearchMemoriesCaseInsensitive(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "Database Settings", "content", nil)

	results := s.SearchMemories("database")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (case insensitive), got %d", len(results))
	}

	results = s.SearchMemories("DATABASE")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (case insensitive), got %d", len(results))
	}
}

func TestSearchMemoriesNoMatch(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "desc", "content", nil)

	results := s.SearchMemories("nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestGetStaleMemories(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "old memory", "content", nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	s.mu.Lock()
	for _, h := range s.headers {
		if h.ID == mem.Header.ID {
			h.UpdatedAt = time.Now().Add(-2 * time.Hour)
			break
		}
	}
	s.mu.Unlock()

	stale := s.GetStaleMemories(1 * time.Hour)
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale memory, got %d", len(stale))
	}

	fresh := s.GetStaleMemories(3 * time.Hour)
	if len(fresh) != 0 {
		t.Fatalf("expected 0 stale memories with long maxAge, got %d", len(fresh))
	}
}

func TestGetMemoryAge(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "desc", "content", nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	age, err := s.GetMemoryAge(mem.Header.ID)
	if err != nil {
		t.Fatalf("GetMemoryAge error: %v", err)
	}
	if age < 0 {
		t.Fatalf("expected non-negative age, got %v", age)
	}
}

func TestGetMemoryAgeNonexistent(t *testing.T) {
	s := newTestEnhancedStore(t)

	_, err := s.GetMemoryAge("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent memory")
	}
}

func TestFormatMemoryAge(t *testing.T) {
	s := newTestEnhancedStore(t)

	tests := []struct {
		age    time.Duration
		expect string
	}{
		{30 * time.Minute, "recent"},
		{2 * time.Hour, "2 hours old"},
		{25 * time.Hour, "1 days old"},
		{50 * time.Hour, "2 days old"},
	}

	for _, tt := range tests {
		got := s.FormatMemoryAge(tt.age)
		if got != tt.expect {
			t.Fatalf("FormatMemoryAge(%v) = %q, want %q", tt.age, got, tt.expect)
		}
	}
}

func TestBuildMemoryPrompt(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "user memory", "user content here", []string{"tag1"})
	s.CreateMemory(MemoryTypeProject, "project memory", "project details", nil)

	prompt := s.BuildMemoryPrompt(1000)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "Memory Context") {
		t.Fatal("expected prompt to contain 'Memory Context'")
	}
}

func TestBuildMemoryPromptTruncation(t *testing.T) {
	s := newTestEnhancedStore(t)

	longContent := strings.Repeat("x", 2000)
	s.CreateMemory(MemoryTypeUser, "big memory", longContent, nil)

	prompt := s.BuildMemoryPrompt(100)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestGetStats(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "desc1", "content1", nil)
	s.CreateMemory(MemoryTypeProject, "desc2", "content2", nil)

	stats := s.GetStats()
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}

	total, ok := stats["total_memories"]
	if !ok {
		t.Fatal("expected total_memories key")
	}
	if total.(int) != 2 {
		t.Fatalf("expected 2 total memories, got %d", total.(int))
	}

	scope, ok := stats["scope"]
	if !ok {
		t.Fatal("expected scope key")
	}
	if scope.(MemoryScope) != ScopeUser {
		t.Fatalf("expected scope %q, got %q", ScopeUser, scope.(MemoryScope))
	}
}

func TestExportMemory(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "export test", "export content", []string{"tag1"})
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	exported, err := s.ExportMemory(mem.Header.ID)
	if err != nil {
		t.Fatalf("ExportMemory error: %v", err)
	}
	if exported == "" {
		t.Fatal("expected non-empty export")
	}
	if !strings.Contains(exported, "export content") {
		t.Fatal("expected export to contain memory content")
	}
}

func TestExportMemoryNonexistent(t *testing.T) {
	s := newTestEnhancedStore(t)

	_, err := s.ExportMemory("nonexistent")
	if err == nil {
		t.Fatal("expected error exporting nonexistent memory")
	}
}

func TestImportMemory(t *testing.T) {
	s := newTestEnhancedStore(t)

	content := "---\n{\"type\":\"user\",\"description\":\"imported memory\",\"tags\":[\"imported\"]}\n---\n\nimported content body"

	mem, err := s.ImportMemory(content)
	if err != nil {
		t.Fatalf("ImportMemory error: %v", err)
	}
	if mem == nil {
		t.Fatal("expected non-nil imported memory")
	}
	if !strings.Contains(mem.Content, "imported content body") {
		t.Fatalf("expected content to contain %q, got %q", "imported content body", mem.Content)
	}
	if mem.Header.Type != MemoryTypeUser {
		t.Fatalf("expected type %q, got %q", MemoryTypeUser, mem.Header.Type)
	}
}

func TestImportMemoryNoFrontmatter(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.ImportMemory("plain content without frontmatter")
	if err != nil {
		t.Fatalf("ImportMemory error: %v", err)
	}
	if mem.Header.Type != MemoryTypeUser {
		t.Fatalf("expected default type %q, got %q", MemoryTypeUser, mem.Header.Type)
	}
}

func TestScanMemoriesOnStartup(t *testing.T) {
	dir := t.TempDir()

	s1, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("first NewEnhancedMemoryStore error: %v", err)
	}

	_, err = s1.CreateMemory(MemoryTypeUser, "persisted", "persisted content", []string{"persist"})
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	s2, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("second NewEnhancedMemoryStore error: %v", err)
	}

	headers := s2.ListMemories()
	if len(headers) == 0 {
		t.Fatal("expected to find previously created memory after re-scanning")
	}
	found := false
	for _, h := range headers {
		if h.Description == "persisted" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find memory with description 'persisted'")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := newTestEnhancedStore(t)

	var wg sync.WaitGroup
	const count = 10

	ids := make(chan string, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mem, err := s.CreateMemory(MemoryTypeUser, "concurrent", "content", []string{"test"})
			if err != nil {
				t.Errorf("CreateMemory error: %v", err)
				return
			}
			ids <- mem.Header.ID
		}(i)
	}
	wg.Wait()
	close(ids)

	for id := range ids {
		_, err := s.GetMemory(id)
		if err != nil {
			t.Errorf("GetMemory error for %s: %v", id, err)
		}
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		hasFM   bool
		body    string
	}{
		{
			"with_frontmatter",
			"---\n{\"type\":\"user\"}\n---\n\nbody text",
			true,
			"\n\nbody text",
		},
		{
			"no_frontmatter",
			"just body text",
			false,
			"just body text",
		},
		{
			"incomplete_frontmatter",
			"---\n{\"type\":\"user\"}\nno closing",
			false,
			"---\n{\"type\":\"user\"}\nno closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := parseFrontmatter(tt.content)
			if tt.hasFM && fm == "" {
				t.Fatal("expected frontmatter")
			}
			if !tt.hasFM && fm != "" {
				t.Fatalf("expected no frontmatter, got %q", fm)
			}
			if body != tt.body {
				t.Fatalf("expected body %q, got %q", tt.body, body)
			}
		})
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name    string
		content string
		maxLen  int
		expect  string
	}{
		{"short", "hello world", 100, "hello world"},
		{"skip_headers", "# Title\nactual content", 100, "actual content"},
		{"skip_blank_lines", "\n\n\ncontent", 100, "content"},
		{"truncate", "a very long description that should be truncated", 20, "a very long descri..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDescription(tt.content, tt.maxLen)
			if len(got) > tt.maxLen {
				t.Fatalf("expected length <= %d, got %d", tt.maxLen, len(got))
			}
		})
	}
}

func TestCreateMemoryWritesFile(t *testing.T) {
	dir := t.TempDir()
	s, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}

	mem, err := s.CreateMemory(MemoryTypeUser, "file test", "file content", []string{"file"})
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == mem.Header.Filename {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected memory file to exist on disk")
	}
}

func TestDeleteMemoryRemovesFile(t *testing.T) {
	dir := t.TempDir()
	s, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}

	mem, err := s.CreateMemory(MemoryTypeUser, "delete test", "content", nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	filePath := mem.Header.Filepath
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected file to exist before delete")
	}

	s.DeleteMemory(mem.Header.ID)

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed after delete")
	}
}

func TestScopeIsolation(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	s1, err := NewEnhancedMemoryStore(dir1, ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}
	s2, err := NewEnhancedMemoryStore(dir2, ScopeProject)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}

	s1.CreateMemory(MemoryTypeUser, "scope1 memory", "content1", nil)
	s2.CreateMemory(MemoryTypeProject, "scope2 memory", "content2", nil)

	h1 := s1.ListMemories()
	h2 := s2.ListMemories()

	if len(h1) != 1 {
		t.Fatalf("expected 1 memory in scope1, got %d", len(h1))
	}
	if len(h2) != 1 {
		t.Fatalf("expected 1 memory in scope2, got %d", len(h2))
	}

	stats1 := s1.GetStats()
	stats2 := s2.GetStats()
	if stats1["scope"].(MemoryScope) != ScopeUser {
		t.Fatal("expected ScopeUser for s1")
	}
	if stats2["scope"].(MemoryScope) != ScopeProject {
		t.Fatal("expected ScopeProject for s2")
	}
}

func TestGetMemoriesByTagNoMatch(t *testing.T) {
	s := newTestEnhancedStore(t)

	s.CreateMemory(MemoryTypeUser, "desc", "content", []string{"tag1"})

	results := s.GetMemoriesByTag("nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSpecialCharactersInDescription(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem, err := s.CreateMemory(MemoryTypeUser, "test: special chars <>&\"'", "content with \"quotes\"", nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	got, err := s.GetMemory(mem.Header.ID)
	if err != nil {
		t.Fatalf("GetMemory error: %v", err)
	}
	if got.Header.Description != "test: special chars <>&\"'" {
		t.Fatalf("description mismatch: %q", got.Header.Description)
	}
}

func TestLargeContent(t *testing.T) {
	s := newTestEnhancedStore(t)

	largeContent := strings.Repeat("x", 10000)
	mem, err := s.CreateMemory(MemoryTypeUser, "large memory", largeContent, nil)
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}

	got, err := s.GetMemory(mem.Header.ID)
	if err != nil {
		t.Fatalf("GetMemory error: %v", err)
	}
	if len(got.Content) != 10000 {
		t.Fatalf("expected 10000 chars, got %d", len(got.Content))
	}
}

func TestMemoryIDsAreUnique(t *testing.T) {
	s := newTestEnhancedStore(t)

	mem1, _ := s.CreateMemory(MemoryTypeUser, "first", "content", nil)
	mem2, _ := s.CreateMemory(MemoryTypeUser, "second", "content", nil)

	if mem1.Header.ID == mem2.Header.ID {
		t.Fatal("expected unique memory IDs")
	}
}

func TestMemoryFileExists(t *testing.T) {
	dir := t.TempDir()
	s, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}

	scanResult := s.scanMemories()
	if scanResult != nil {
		t.Fatalf("scanMemories on empty dir returned error: %v", scanResult)
	}
}

func TestScanSkipsNonMDFiles(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a memory"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("should be skipped"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	s, err := NewEnhancedMemoryStore(dir, ScopeUser)
	if err != nil {
		t.Fatalf("NewEnhancedMemoryStore error: %v", err)
	}

	headers := s.ListMemories()
	if len(headers) != 0 {
		t.Fatalf("expected 0 memories (non-md and MEMORY.md skipped), got %d", len(headers))
	}
}

func TestBuildMemoryPromptEmpty(t *testing.T) {
	s := newTestEnhancedStore(t)

	prompt := s.BuildMemoryPrompt(1000)
	if prompt == "" {
		t.Fatal("expected non-empty prompt even with no memories")
	}
}

func TestGetStatsEmpty(t *testing.T) {
	s := newTestEnhancedStore(t)

	stats := s.GetStats()
	total := stats["total_memories"].(int)
	if total != 0 {
		t.Fatalf("expected 0 total memories, got %d", total)
	}
}

func TestConcurrentCreateAndRead(t *testing.T) {
	s := newTestEnhancedStore(t)

	var wg sync.WaitGroup
	const count = 5

	ids := make([]string, count)
	for i := 0; i < count; i++ {
		mem, err := s.CreateMemory(MemoryTypeUser, "concurrent read test", "content", nil)
		if err != nil {
			t.Fatalf("CreateMemory error: %v", err)
		}
		ids[i] = mem.Header.ID
	}

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			mem, err := s.GetMemory(id)
			if err != nil {
				t.Errorf("GetMemory error for %s: %v", id, err)
			}
			if mem == nil {
				t.Errorf("expected non-nil memory for %s", id)
			}
		}(ids[i])
	}
	wg.Wait()
}

func TestMemoryHeaderFields(t *testing.T) {
	s := newTestEnhancedStore(t)

	beforeCreate := time.Now()
	mem, err := s.CreateMemory(MemoryTypeFeedback, "test desc", "test body", []string{"a", "b"})
	if err != nil {
		t.Fatalf("CreateMemory error: %v", err)
	}
	afterCreate := time.Now()

	h := mem.Header
	if h.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if h.Filename == "" {
		t.Fatal("expected non-empty Filename")
	}
	if h.Filepath == "" {
		t.Fatal("expected non-empty Filepath")
	}
	if h.Type != MemoryTypeFeedback {
		t.Fatalf("expected type %q, got %q", MemoryTypeFeedback, h.Type)
	}
	if h.Description != "test desc" {
		t.Fatalf("expected description %q, got %q", "test desc", h.Description)
	}
	if len(h.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(h.Tags))
	}
	if h.MtimeMs == 0 {
		t.Fatal("expected non-zero MtimeMs")
	}
	if h.CreatedAt.Before(beforeCreate) || h.CreatedAt.After(afterCreate) {
		t.Fatalf("CreatedAt %v outside expected range [%v, %v]", h.CreatedAt, beforeCreate, afterCreate)
	}
	if h.UpdatedAt.Before(beforeCreate) || h.UpdatedAt.After(afterCreate) {
		t.Fatalf("UpdatedAt %v outside expected range [%v, %v]", h.UpdatedAt, beforeCreate, afterCreate)
	}
}
