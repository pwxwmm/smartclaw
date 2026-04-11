package tools

import (
	"os"
	"testing"
	"time"
)

func TestResultCache_SetAndGet(t *testing.T) {
	cache := NewResultCache(10, 5*time.Minute)

	input := map[string]any{"path": "/test/file.go"}

	cache.Set("read_file", input, "file content here", nil)

	result, ok := cache.Get("read_file", input)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if result != "file content here" {
		t.Errorf("expected 'file content here', got %v", result)
	}
}

func TestResultCache_Miss(t *testing.T) {
	cache := NewResultCache(10, 5*time.Minute)

	_, ok := cache.Get("read_file", map[string]any{"path": "/nonexistent"})
	if ok {
		t.Error("expected cache miss")
	}
}

func TestResultCache_DifferentInputs(t *testing.T) {
	cache := NewResultCache(10, 5*time.Minute)

	input1 := map[string]any{"path": "/file1.go"}
	input2 := map[string]any{"path": "/file2.go"}

	cache.Set("read_file", input1, "content1", nil)
	cache.Set("read_file", input2, "content2", nil)

	result1, ok := cache.Get("read_file", input1)
	if !ok || result1 != "content1" {
		t.Errorf("expected content1, got %v, ok=%v", result1, ok)
	}

	result2, ok := cache.Get("read_file", input2)
	if !ok || result2 != "content2" {
		t.Errorf("expected content2, got %v, ok=%v", result2, ok)
	}
}

func TestResultCache_TTLExpiry(t *testing.T) {
	cache := NewResultCache(10, 50*time.Millisecond)

	input := map[string]any{"path": "/test"}
	cache.Set("read_file", input, "content", nil)

	result, ok := cache.Get("read_file", input)
	if !ok || result != "content" {
		t.Fatal("expected cache hit before TTL")
	}

	time.Sleep(80 * time.Millisecond)

	_, ok = cache.Get("read_file", input)
	if ok {
		t.Error("expected cache miss after TTL")
	}
}

func TestResultCache_LRUEviction(t *testing.T) {
	cache := NewResultCache(3, 5*time.Minute)

	for i := 0; i < 5; i++ {
		input := map[string]any{"path": string(rune('a' + i))}
		cache.Set("read_file", input, i, nil)
	}

	if cache.Size() > 3 {
		t.Errorf("expected at most 3 entries, got %d", cache.Size())
	}

	firstInput := map[string]any{"path": "a"}
	_, ok := cache.Get("read_file", firstInput)
	if ok {
		t.Error("expected oldest entry to be evicted")
	}
}

func TestResultCache_InvalidateByPaths(t *testing.T) {
	cache := NewResultCache(10, 5*time.Minute)

	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.go"

	os.WriteFile(testFile, []byte("hello"), 0644)

	input := map[string]any{"path": testFile}
	cache.Set("read_file", input, "content", []string{testFile})

	result, ok := cache.Get("read_file", input)
	if !ok {
		t.Fatal("expected cache hit before invalidation")
	}
	if result != "content" {
		t.Errorf("expected content, got %v", result)
	}

	cache.Invalidate([]string{testFile})

	_, ok = cache.Get("read_file", input)
	if ok {
		t.Error("expected cache miss after invalidation")
	}
}

func TestResultCache_InvalidateAll(t *testing.T) {
	cache := NewResultCache(10, 5*time.Minute)

	for i := 0; i < 5; i++ {
		input := map[string]any{"path": string(rune('a' + i))}
		cache.Set("read_file", input, i, nil)
	}

	cache.InvalidateAll()

	if cache.Size() != 0 {
		t.Errorf("expected 0 entries after invalidate all, got %d", cache.Size())
	}
}

func TestResultCache_UpdateExisting(t *testing.T) {
	cache := NewResultCache(10, 5*time.Minute)

	input := map[string]any{"path": "/test"}
	cache.Set("read_file", input, "old", nil)
	cache.Set("read_file", input, "new", nil)

	result, ok := cache.Get("read_file", input)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if result != "new" {
		t.Errorf("expected 'new', got %v", result)
	}
}

func TestResultCache_Defaults(t *testing.T) {
	cache := NewResultCache(0, 0)
	if cache.maxSize != 100 {
		t.Errorf("expected default maxSize 100, got %d", cache.maxSize)
	}
	if cache.ttl != 5*time.Minute {
		t.Errorf("expected default ttl 5m, got %v", cache.ttl)
	}
}

func TestExtractDepFiles(t *testing.T) {
	tests := []struct {
		tool  string
		input map[string]any
		want  int
	}{
		{"read_file", map[string]any{"path": "/test.go"}, 1},
		{"glob", map[string]any{"path": "/src", "pattern": "*.go"}, 2},
		{"bash", map[string]any{"command": "ls"}, 0},
	}

	for _, tt := range tests {
		got := extractDepFiles(tt.tool, tt.input)
		if len(got) < tt.want {
			t.Errorf("extractDepFiles(%s, %v) = %d files, want at least %d", tt.tool, tt.input, len(got), tt.want)
		}
	}
}
