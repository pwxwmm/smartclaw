package memory

import (
	"os"
	"testing"
	"time"
)

func TestNewMemoryStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if store == nil {
		t.Fatal("Expected non-nil store")
	}
}

func TestMemoryStoreSetGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()

	err = store.Set("test-key", "test-value", 0)
	if err != nil {
		t.Fatalf("Expected no error setting, got %v", err)
	}

	value, err := store.Get("test-key")
	if err != nil {
		t.Fatalf("Expected no error getting, got %v", err)
	}

	if value != "test-value" {
		t.Errorf("Expected 'test-value', got %v", value)
	}
}

func TestMemoryStoreGetNonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()
	store.Set("test-key", "value", 0)

	err = store.Delete("test-key")
	if err != nil {
		t.Errorf("Expected no error deleting, got %v", err)
	}

	_, err = store.Get("test-key")
	if err == nil {
		t.Error("Expected error getting deleted key")
	}
}

func TestMemoryStoreList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()
	store.Set("key1", "value1", 0)
	store.Set("key2", "value2", 0)

	keys := store.List()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestMemoryStoreClear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()
	store.Set("key1", "value1", 0)
	store.Set("key2", "value2", 0)

	err = store.Clear()
	if err != nil {
		t.Errorf("Expected no error clearing, got %v", err)
	}

	keys := store.List()
	if len(keys) != 0 {
		t.Error("Expected empty store after clear")
	}
}

func TestMemoryStoreExpiration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()
	store.Set("test-key", "value", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	_, err = store.Get("test-key")
	if err == nil {
		t.Error("Expected error for expired key")
	}
}

func TestMemoryStoreTags(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()
	store.Set("test-key", "value", 0)

	err = store.AddTag("test-key", "important")
	if err != nil {
		t.Errorf("Expected no error adding tag, got %v", err)
	}

	memories := store.GetByTag("important")
	if len(memories) != 1 {
		t.Errorf("Expected 1 memory with tag, got %d", len(memories))
	}

	err = store.RemoveTag("test-key", "important")
	if err != nil {
		t.Errorf("Expected no error removing tag, got %v", err)
	}

	memories = store.GetByTag("important")
	if len(memories) != 0 {
		t.Error("Expected no memories with removed tag")
	}
}

func TestMemoryStoreSearch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	store, _ := NewMemoryStore()
	store.Set("test-key", "value", 0)

	results := store.Search("test-key")
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestMemory(t *testing.T) {
	now := time.Now()
	memory := Memory{
		Key:       "test",
		Value:     "value",
		CreatedAt: now,
		UpdatedAt: now,
		Tags:      []string{"tag1", "tag2"},
	}

	if memory.Key != "test" {
		t.Errorf("Expected key 'test', got '%s'", memory.Key)
	}

	if len(memory.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(memory.Tags))
	}
}

func TestMemoryAge(t *testing.T) {
	memory := Memory{
		Key:       "test",
		CreatedAt: time.Now().Add(-time.Hour),
	}

	age := memory.Age()
	if age < time.Hour {
		t.Errorf("Expected age >= 1 hour, got %v", age)
	}
}

func TestMemoryIsExpired(t *testing.T) {
	expiresAt := time.Now().Add(-time.Hour)
	memory := Memory{
		Key:       "test",
		ExpiresAt: &expiresAt,
	}

	if !memory.IsExpired() {
		t.Error("Expected memory to be expired")
	}

	futureExpiry := time.Now().Add(time.Hour)
	memory.ExpiresAt = &futureExpiry

	if memory.IsExpired() {
		t.Error("Expected memory not to be expired")
	}

	memory.ExpiresAt = nil
	if memory.IsExpired() {
		t.Error("Expected memory without expiry not to be expired")
	}
}
