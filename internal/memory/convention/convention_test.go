package convention

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
}

func TestNewStore_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "conventions")
	s := NewStore(dir)
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
	err := s.Add(&Convention{
		Category:    "naming",
		Title:       "Test",
		Description: "Test convention",
		Confidence:  0.9,
		Source:      "user",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Store directory was not created")
	}
}

func TestStore_Add_AutoID(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{
		Category:    "naming",
		Title:       "Use camelCase",
		Description: "Variables use camelCase",
		Confidence:  0.9,
		Source:      "user",
	}
	if err := s.Add(conv); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if conv.ID == "" {
		t.Error("ID should be auto-generated when empty")
	}
}

func TestStore_Add_SetsTimestamps(t *testing.T) {
	s := NewStore(t.TempDir())
	before := time.Now()
	conv := &Convention{
		Category:    "naming",
		Title:       "Test",
		Description: "Test",
		Confidence:  0.9,
		Source:      "user",
	}
	s.Add(conv)
	after := time.Now()
	if conv.CreatedAt.Before(before) || conv.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, want between %v and %v", conv.CreatedAt, before, after)
	}
	if conv.LastUsed.Before(before) || conv.LastUsed.After(after) {
		t.Errorf("LastUsed = %v, want between %v and %v", conv.LastUsed, before, after)
	}
}

func TestStore_Add_PreservesExistingID(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{
		ID:          "custom-id-123",
		Category:    "naming",
		Title:       "Test",
		Description: "Test",
		Confidence:  0.9,
		Source:      "user",
	}
	s.Add(conv)
	if conv.ID != "custom-id-123" {
		t.Errorf("ID = %q, want %q", conv.ID, "custom-id-123")
	}
}

func TestStore_Add_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	conv := &Convention{
		Category:    "pattern",
		Title:       "Singleton",
		Description: "Use singleton pattern",
		Confidence:  0.8,
		Source:      "agent",
	}
	s.Add(conv)

	path := filepath.Join(dir, conv.ID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Convention file was not created on disk")
	}

	s2 := NewStore(dir)
	got, err := s2.Get(conv.ID)
	if err != nil {
		t.Fatalf("Get after reload failed: %v", err)
	}
	if got.Title != "Singleton" {
		t.Errorf("Title = %q, want %q", got.Title, "Singleton")
	}
}

func TestStore_Get(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{
		Category:    "naming",
		Title:       "Test",
		Description: "Test",
		Confidence:  0.9,
		Source:      "user",
	}
	s.Add(conv)

	got, err := s.Get(conv.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Test" {
		t.Errorf("Title = %q, want %q", got.Title, "Test")
	}
}

func TestStore_Get_Missing(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Error("Get should return error for missing convention")
	}
}

func TestStore_List_FilterByCategory(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "N1", Description: "D1", Confidence: 0.9, Source: "user"})
	s.Add(&Convention{Category: "pattern", Title: "P1", Description: "D2", Confidence: 0.9, Source: "user"})
	s.Add(&Convention{Category: "naming", Title: "N2", Description: "D3", Confidence: 0.9, Source: "user"})

	result := s.List(Filter{Category: "naming"})
	if len(result) != 2 {
		t.Errorf("List with Category filter = %d items, want 2", len(result))
	}
}

func TestStore_List_FilterBySource(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "T1", Description: "D1", Confidence: 0.9, Source: "user"})
	s.Add(&Convention{Category: "pattern", Title: "T2", Description: "D2", Confidence: 0.9, Source: "agent"})

	result := s.List(Filter{Source: "agent"})
	if len(result) != 1 {
		t.Errorf("List with Source filter = %d items, want 1", len(result))
	}
}

func TestStore_List_FilterByAgentID(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "T1", Description: "D1", Confidence: 0.9, Source: "agent", AgentID: "agent-1"})
	s.Add(&Convention{Category: "naming", Title: "T2", Description: "D2", Confidence: 0.9, Source: "agent", AgentID: "agent-2"})

	result := s.List(Filter{AgentID: "agent-1"})
	if len(result) != 1 {
		t.Errorf("List with AgentID filter = %d items, want 1", len(result))
	}
}

func TestStore_List_FilterByMinConfidence(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "T1", Description: "D1", Confidence: 0.5, Source: "user"})
	s.Add(&Convention{Category: "naming", Title: "T2", Description: "D2", Confidence: 0.9, Source: "user"})

	result := s.List(Filter{MinConfidence: 0.8})
	if len(result) != 1 {
		t.Errorf("List with MinConfidence = %d items, want 1", len(result))
	}
}

func TestStore_List_EmptyFilter(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "T1", Description: "D1", Confidence: 0.9, Source: "user"})
	s.Add(&Convention{Category: "pattern", Title: "T2", Description: "D2", Confidence: 0.8, Source: "agent"})

	result := s.List(Filter{})
	if len(result) != 2 {
		t.Errorf("List with empty filter = %d items, want 2", len(result))
	}
}

func TestStore_Delete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	conv := &Convention{Category: "naming", Title: "T1", Description: "D1", Confidence: 0.9, Source: "user"}
	s.Add(conv)

	if err := s.Delete(conv.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := s.Get(conv.ID); err == nil {
		t.Error("Get should fail after Delete")
	}
}

func TestStore_Delete_Missing(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Delete("nonexistent"); err == nil {
		t.Error("Delete should return error for missing convention")
	}
}

func TestStore_Delete_RemovesFromShares(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{Category: "naming", Title: "T1", Description: "D1", Confidence: 0.9, Source: "user"}
	s.Add(conv)
	s.Share(conv.ID, "agent-1")
	s.Delete(conv.ID)
	shares := s.SharedWith(conv.ID)
	if len(shares) != 0 {
		t.Errorf("SharedWith after delete = %v, want empty", shares)
	}
}

func TestStore_Search(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "Use camelCase for vars", Description: "Variables use camelCase", Confidence: 0.9, Source: "user"})
	s.Add(&Convention{Category: "pattern", Title: "Singleton pattern", Description: "Use singleton for managers", Confidence: 0.9, Source: "user"})

	result := s.Search("camelcase", 0)
	if len(result) != 1 {
		t.Errorf("Search = %d results, want 1", len(result))
	}
}

func TestStore_Search_CaseInsensitive(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "Use CamelCase", Description: "Important rule", Confidence: 0.9, Source: "user"})

	result := s.Search("camelcase", 0)
	if len(result) != 1 {
		t.Errorf("Case-insensitive search = %d results, want 1", len(result))
	}
}

func TestStore_Search_Description(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "Rule", Description: "Important naming convention", Confidence: 0.9, Source: "user"})

	result := s.Search("naming convention", 0)
	if len(result) != 1 {
		t.Errorf("Search in description = %d results, want 1", len(result))
	}
}

func TestStore_Search_WithLimit(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Add(&Convention{Category: "naming", Title: "Test One", Description: "test", Confidence: 0.9, Source: "user"})
	s.Add(&Convention{Category: "naming", Title: "Test Two", Description: "test", Confidence: 0.9, Source: "user"})
	s.Add(&Convention{Category: "naming", Title: "Test Three", Description: "test", Confidence: 0.9, Source: "user"})

	result := s.Search("test", 2)
	if len(result) != 2 {
		t.Errorf("Search with limit = %d results, want 2", len(result))
	}
}

func TestComputeContentHash(t *testing.T) {
	lines := []string{"hello", "world"}
	h1 := ComputeContentHash(lines)
	h2 := ComputeContentHash(lines)
	if h1 != h2 {
		t.Error("Same input should produce same hash")
	}

	different := ComputeContentHash([]string{"different"})
	if h1 == different {
		t.Error("Different input should produce different hash")
	}
}

func TestComputeContentHash_Deterministic(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	hash := ComputeContentHash(lines)
	if len(hash) != 64 { // SHA256 hex = 64 chars
		t.Errorf("Hash length = %d, want 64", len(hash))
	}
}

func TestVerifyConvention_NilCitation(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{
		ID:         "test-conv",
		Category:   "naming",
		Title:      "Test",
		Confidence: 0.9,
		Source:     "user",
	}
	result := s.VerifyConvention(context.Background(), conv, "")
	if !result.Valid {
		t.Errorf("Nil citation should be valid, got reason: %s", result.Reason)
	}
}

func TestVerifyConvention_MissingFile(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{
		ID:       "test-conv",
		Category: "naming",
		Title:    "Test",
		Citation: &Citation{
			File:        "nonexistent.go",
			StartLine:   1,
			EndLine:     5,
			ContentHash: "abc",
		},
	}
	result := s.VerifyConvention(context.Background(), conv, t.TempDir())
	if result.Valid {
		t.Error("Missing file should be invalid")
	}
	if result.Reason != "file not found" {
		t.Errorf("Reason = %q, want %q", result.Reason, "file not found")
	}
}

func TestVerifyConvention_ContentHashMismatch(t *testing.T) {
	s := NewStore(t.TempDir())
	projectRoot := t.TempDir()
	filePath := filepath.Join(projectRoot, "test.go")
	os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0644)

	conv := &Convention{
		ID:       "test-conv",
		Category: "naming",
		Title:    "Test",
		Citation: &Citation{
			File:        "test.go",
			StartLine:   1,
			EndLine:     3,
			ContentHash: "wronghash",
		},
	}
	result := s.VerifyConvention(context.Background(), conv, projectRoot)
	if result.Valid {
		t.Error("Hash mismatch should be invalid")
	}
	if result.Reason != "content mismatch" {
		t.Errorf("Reason = %q, want %q", result.Reason, "content mismatch")
	}
}

func TestVerifyConvention_ValidCitation(t *testing.T) {
	s := NewStore(t.TempDir())
	projectRoot := t.TempDir()
	lines := []string{"package main", "", "func main() {}"}
	content := "package main\n\nfunc main() {}\n"
	filePath := filepath.Join(projectRoot, "test.go")
	os.WriteFile(filePath, []byte(content), 0644)

	hash := ComputeContentHash(lines[:1]) // just line 1

	conv := &Convention{
		ID:       "test-conv",
		Category: "naming",
		Title:    "Test",
		Citation: &Citation{
			File:        "test.go",
			StartLine:   1,
			EndLine:     1,
			ContentHash: hash,
		},
	}
	result := s.VerifyConvention(context.Background(), conv, projectRoot)
	if !result.Valid {
		t.Errorf("Matching hash should be valid, got reason: %s", result.Reason)
	}
}

func TestStore_Share(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{Category: "naming", Title: "Test", Description: "D", Confidence: 0.9, Source: "user"}
	s.Add(conv)

	if err := s.Share(conv.ID, "agent-1"); err != nil {
		t.Fatalf("Share failed: %v", err)
	}
}

func TestStore_Share_MissingConvention(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Share("nonexistent", "agent-1"); err == nil {
		t.Error("Share should fail for missing convention")
	}
}

func TestStore_Share_Idempotent(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{Category: "naming", Title: "Test", Description: "D", Confidence: 0.9, Source: "user"}
	s.Add(conv)

	s.Share(conv.ID, "agent-1")
	s.Share(conv.ID, "agent-1")

	shares := s.SharedWith(conv.ID)
	if len(shares) != 1 {
		t.Errorf("Shares after idempotent Share = %d, want 1", len(shares))
	}
}

func TestStore_SharedWith(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{Category: "naming", Title: "Test", Description: "D", Confidence: 0.9, Source: "user"}
	s.Add(conv)
	s.Share(conv.ID, "agent-1")
	s.Share(conv.ID, "agent-2")

	shares := s.SharedWith(conv.ID)
	if len(shares) != 2 {
		t.Errorf("SharedWith = %d agents, want 2", len(shares))
	}
}

func TestStore_SharedWith_None(t *testing.T) {
	s := NewStore(t.TempDir())
	shares := s.SharedWith("nonexistent")
	if shares != nil {
		t.Errorf("SharedWith for nonexistent = %v, want nil", shares)
	}
}

func TestStore_ReceiveShared(t *testing.T) {
	s := NewStore(t.TempDir())
	conv := &Convention{
		ID:          "shared-conv",
		Category:    "naming",
		Title:       "Shared Rule",
		Description: "From another agent",
		Confidence:  0.8,
		Source:      "user",
	}
	if err := s.ReceiveShared(conv); err != nil {
		t.Fatalf("ReceiveShared failed: %v", err)
	}
	if conv.Source != "agent" {
		t.Errorf("Source = %q, want %q", conv.Source, "agent")
	}
	got, err := s.Get(conv.ID)
	if err != nil {
		t.Fatalf("Get after ReceiveShared failed: %v", err)
	}
	if got.Source != "agent" {
		t.Errorf("Stored source = %q, want %q", got.Source, "agent")
	}
}
