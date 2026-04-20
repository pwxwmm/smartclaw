package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCodebaseIndex(t *testing.T) {
	dir := t.TempDir()
	idx := NewCodebaseIndex(dir)

	if idx == nil {
		t.Fatal("NewCodebaseIndex returned nil")
	}
	if idx.RootPath() == "" {
		t.Error("RootPath should not be empty")
	}
	if idx.rootPath != dir {
		// RootPath returns abs, compare with resolution
		abs, _ := filepath.Abs(dir)
		if idx.rootPath != abs {
			t.Errorf("rootPath = %q, want %q", idx.rootPath, abs)
		}
	}

	stats := idx.GetStats()
	if stats.FileCount != 0 {
		t.Errorf("new index FileCount = %d, want 0", stats.FileCount)
	}
	if stats.SymbolCount != 0 {
		t.Errorf("new index SymbolCount = %d, want 0", stats.SymbolCount)
	}
	if stats.ChunkCount != 0 {
		t.Errorf("new index ChunkCount = %d, want 0", stats.ChunkCount)
	}
}

func TestIndexWithGoFile(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "sample.go")
	content := `package sample

// Hello greets the world.
func Hello() string {
	return "hello"
}

type Server struct {
	Name string
}

func (s *Server) Start() error {
	return nil
}

const Version = "1.0"

var Debug = false
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.Index(); err != nil {
		t.Fatalf("Index error: %v", err)
	}

	stats := idx.GetStats()
	if stats.FileCount < 1 {
		t.Errorf("FileCount = %d, want >= 1", stats.FileCount)
	}
	if stats.SymbolCount < 1 {
		t.Errorf("SymbolCount = %d, want >= 1", stats.SymbolCount)
	}
	if stats.ChunkCount < 1 {
		t.Errorf("ChunkCount = %d, want >= 1", stats.ChunkCount)
	}
	if stats.LastIndexed.IsZero() {
		t.Error("LastIndexed should be set after Index()")
	}

	// Check symbols extracted
	syms := idx.AllSymbols()
	foundHello := false
	foundServer := false
	foundStart := false
	foundVersion := false
	foundDebug := false
	for _, s := range syms {
		switch s.Name {
		case "Hello":
			foundHello = true
			if s.Kind != "function" {
				t.Errorf("Hello.Kind = %q, want %q", s.Kind, "function")
			}
		case "Server":
			foundServer = true
			if s.Kind != "struct" {
				t.Errorf("Server.Kind = %q, want %q", s.Kind, "struct")
			}
		case "Start":
			foundStart = true
			if s.Kind != "method" {
				t.Errorf("Start.Kind = %q, want %q", s.Kind, "method")
			}
			if s.Receiver != "Server" {
				t.Errorf("Start.Receiver = %q, want %q", s.Receiver, "Server")
			}
		case "Version":
			foundVersion = true
			if s.Kind != "constant" {
				t.Errorf("Version.Kind = %q, want %q", s.Kind, "constant")
			}
		case "Debug":
			foundDebug = true
			if s.Kind != "variable" {
				t.Errorf("Debug.Kind = %q, want %q", s.Kind, "variable")
			}
		}
	}
	if !foundHello {
		t.Error("expected symbol Hello not found")
	}
	if !foundServer {
		t.Error("expected symbol Server not found")
	}
	if !foundStart {
		t.Error("expected symbol Start not found")
	}
	if !foundVersion {
		t.Error("expected symbol Version not found")
	}
	if !foundDebug {
		t.Error("expected symbol Debug not found")
	}
}

func TestIndexFile(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "single.go")
	content := `package single

func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile error: %v", err)
	}

	stats := idx.GetStats()
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}
	if stats.SymbolCount < 1 {
		t.Errorf("SymbolCount = %d, want >= 1", stats.SymbolCount)
	}

	// Check file info
	fi := idx.GetFile("single.go")
	if fi == nil {
		t.Fatal("GetFile returned nil")
	}
	if fi.Language != "go" {
		t.Errorf("Language = %q, want %q", fi.Language, "go")
	}
	if fi.LinesCount < 1 {
		t.Errorf("LinesCount = %d, want >= 1", fi.LinesCount)
	}
}

func TestRemoveFile(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "remove_me.go")
	content := `package remove

func Gone() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile error: %v", err)
	}

	statsBefore := idx.GetStats()
	if statsBefore.FileCount != 1 {
		t.Fatalf("FileCount before remove = %d, want 1", statsBefore.FileCount)
	}

	idx.RemoveFile(goFile)

	statsAfter := idx.GetStats()
	if statsAfter.FileCount != 0 {
		t.Errorf("FileCount after remove = %d, want 0", statsAfter.FileCount)
	}
	if statsAfter.SymbolCount != 0 {
		t.Errorf("SymbolCount after remove = %d, want 0", statsAfter.SymbolCount)
	}
	if statsAfter.ChunkCount != 0 {
		t.Errorf("ChunkCount after remove = %d, want 0", statsAfter.ChunkCount)
	}
}

func TestGetStats(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "stats.go")
	content := `package stats

type Counter int

func (c Counter) Inc() Counter {
	return c + 1
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile error: %v", err)
	}

	stats := idx.GetStats()
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}
	// Counter (type), Inc (method) — at least 2 symbols
	if stats.SymbolCount < 2 {
		t.Errorf("SymbolCount = %d, want >= 2", stats.SymbolCount)
	}
	if stats.ChunkCount < 1 {
		t.Errorf("ChunkCount = %d, want >= 1", stats.ChunkCount)
	}
	// IndexFile does not set LastIndexed; only full Index() does
	_ = stats.LastIndexed
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "persist.go")
	content := `package persist

func SaveMe() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile error: %v", err)
	}

	statsBefore := idx.GetStats()

	indexPath := filepath.Join(dir, ".idx")
	if err := idx.Save(indexPath); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(filepath.Join(indexPath, "codebase.json")); os.IsNotExist(err) {
		t.Error("codebase.json was not created")
	}

	// Load into a new index
	idx2 := NewCodebaseIndex(dir)
	if err := idx2.Load(indexPath); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	statsAfter := idx2.GetStats()
	if statsAfter.FileCount != statsBefore.FileCount {
		t.Errorf("FileCount after load = %d, want %d", statsAfter.FileCount, statsBefore.FileCount)
	}
	if statsAfter.SymbolCount != statsBefore.SymbolCount {
		t.Errorf("SymbolCount after load = %d, want %d", statsAfter.SymbolCount, statsBefore.SymbolCount)
	}
	if statsAfter.ChunkCount != statsBefore.ChunkCount {
		t.Errorf("ChunkCount after load = %d, want %d", statsAfter.ChunkCount, statsBefore.ChunkCount)
	}

	// Verify symbol data persisted
	syms := idx2.AllSymbols()
	foundSaveMe := false
	for _, s := range syms {
		if s.Name == "SaveMe" {
			foundSaveMe = true
			if s.Kind != "function" {
				t.Errorf("SaveMe.Kind = %q, want %q", s.Kind, "function")
			}
		}
	}
	if !foundSaveMe {
		t.Error("SaveMe symbol not found after Load")
	}
}

func TestGenerateEmbedding(t *testing.T) {
	vec := GenerateEmbedding("hello world function")
	if len(vec) != 128 {
		t.Errorf("embedding length = %d, want 128", len(vec))
	}

	// Non-zero vector
	allZero := true
	for _, v := range vec {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("embedding should not be all zeros for non-empty input")
	}

	// Empty input returns zero vector
	emptyVec := GenerateEmbedding("")
	if len(emptyVec) != 128 {
		t.Errorf("empty embedding length = %d, want 128", len(emptyVec))
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Same vector → similarity = 1.0
	vec := GenerateEmbedding("hello world")
	sim := CosineSimilarity(vec, vec)
	if sim < 0.99 {
		t.Errorf("self-similarity = %f, want ~1.0", sim)
	}

	// Different vectors → similarity < 1.0
	vec2 := GenerateEmbedding("completely different quantum physics")
	sim2 := CosineSimilarity(vec, vec2)
	if sim2 >= 1.0 {
		t.Errorf("different vector similarity = %f, want < 1.0", sim2)
	}

	// Zero vectors → similarity = 0
	zero := make([]float64, 128)
	sim3 := CosineSimilarity(zero, zero)
	if sim3 != 0 {
		t.Errorf("zero vector similarity = %f, want 0", sim3)
	}

	// Mismatched lengths → similarity = 0
	sim4 := CosineSimilarity([]float64{1.0}, []float64{1.0, 2.0})
	if sim4 != 0 {
		t.Errorf("mismatched length similarity = %f, want 0", sim4)
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "search.go")
	content := `package search

// FindUser locates a user by ID.
func FindUser(id string) (*User, error) {
	return nil, nil
}

type User struct {
	ID   string
	Name string
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile error: %v", err)
	}

	result, err := idx.Search(context.Background(), SearchQuery{
		Query:      "FindUser",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if result.Total == 0 {
		t.Error("Search returned 0 results, expected at least 1")
	}
	if len(result.Items) == 0 {
		t.Error("Search returned no items")
	}

	// The top result should be related to FindUser
	found := false
	for _, item := range result.Items {
		if item.Score > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one result with score > 0")
	}
}

func TestSearchEmptyIndex(t *testing.T) {
	dir := t.TempDir()
	idx := NewCodebaseIndex(dir)

	result, err := idx.Search(context.Background(), SearchQuery{
		Query: "anything",
	})
	if err != nil {
		t.Fatalf("Search on empty index error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("empty index search Total = %d, want 0", result.Total)
	}
}

func TestSearchSymbols(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "syms.go")
	content := `package syms

func ProcessData() {}
func HandleRequest() {}
type Handler struct{}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile error: %v", err)
	}

	results, err := idx.SearchSymbols(context.Background(), "Process", "", 10)
	if err != nil {
		t.Fatalf("SearchSymbols error: %v", err)
	}

	found := false
	for _, sym := range results {
		if sym.Name == "ProcessData" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SearchSymbols did not find ProcessData")
	}
}

func TestGetFileSymbols(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "filesyms.go")
	content := `package filesyms

func Alpha() {}
func Beta() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.IndexFile(goFile); err != nil {
		t.Fatalf("IndexFile error: %v", err)
	}

	syms := idx.GetFileSymbols("filesyms.go")
	if len(syms) < 2 {
		t.Errorf("GetFileSymbols returned %d symbols, want >= 2", len(syms))
	}

	names := make(map[string]bool)
	for _, s := range syms {
		names[s.Name] = true
	}
	if !names["Alpha"] || !names["Beta"] {
		t.Errorf("expected Alpha and Beta, got names: %v", names)
	}
}

func TestSimilaritySearch(t *testing.T) {
	vec1 := GenerateEmbedding("database connection pool")
	vec2 := GenerateEmbedding("http server handler")
	vec3 := GenerateEmbedding("database query builder")

	candidates := map[string][]float64{
		"db_pool":    vec1,
		"http_hdl":   vec2,
		"db_query":   vec3,
	}

	query := GenerateEmbedding("database connection")
	results := SimilaritySearch(query, candidates, 2)

	if len(results) > 2 {
		t.Errorf("top-K returned %d, want at most 2", len(results))
	}
	if len(results) > 0 && results[0].Score <= 0 {
		t.Errorf("top result score = %f, want > 0", results[0].Score)
	}

	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: [%d]=%f > [%d]=%f", i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestIndexSkipsDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create .git directory with a Go file — should be skipped
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "hook.go"), []byte("package git\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a normal Go file — should be indexed
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCodebaseIndex(dir)
	if err := idx.Index(); err != nil {
		t.Fatalf("Index error: %v", err)
	}

	stats := idx.GetStats()
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (.git should be skipped)", stats.FileCount)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.go", "go"},
		{"foo.py", "python"},
		{"foo.ts", "typescript"},
		{"foo.rs", "rust"},
		{"foo.txt", ""},
	}

	for _, tc := range tests {
		got := detectLanguage(tc.path)
		if got != tc.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
