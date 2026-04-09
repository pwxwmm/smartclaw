package cache

import (
	"os"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, err := NewCache()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}
}

func TestCacheSetGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()

	err = cache.Set("test-key", "test-value", time.Hour)
	if err != nil {
		t.Fatalf("Expected no error setting cache, got %v", err)
	}

	value, ok := cache.Get("test-key")
	if !ok {
		t.Error("Expected to find cached value")
	}

	if value != "test-value" {
		t.Errorf("Expected 'test-value', got %v", value)
	}
}

func TestCacheGetNonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent key")
	}
}

func TestCacheDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()
	cache.Set("test-key", "test-value", time.Hour)

	err = cache.Delete("test-key")
	if err != nil {
		t.Errorf("Expected no error deleting, got %v", err)
	}

	_, ok := cache.Get("test-key")
	if ok {
		t.Error("Expected key to be deleted")
	}
}

func TestCacheHas(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()

	if cache.Has("test-key") {
		t.Error("Expected Has to return false for nonexistent key")
	}

	cache.Set("test-key", "value", time.Hour)

	if !cache.Has("test-key") {
		t.Error("Expected Has to return true for existing key")
	}
}

func TestCacheClear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()
	cache.Set("key1", "value1", time.Hour)
	cache.Set("key2", "value2", time.Hour)

	err = cache.Clear()
	if err != nil {
		t.Errorf("Expected no error clearing, got %v", err)
	}

	if cache.Has("key1") || cache.Has("key2") {
		t.Error("Expected all keys to be cleared")
	}
}

func TestCacheExpiration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()
	cache.Set("test-key", "value", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	_, ok := cache.Get("test-key")
	if ok {
		t.Error("Expected expired key to return false")
	}
}

func TestCacheGetOrSet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()

	called := false
	value, err := cache.GetOrSet("test-key", func() (interface{}, error) {
		called = true
		return "computed", nil
	}, time.Hour)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if value != "computed" {
		t.Errorf("Expected 'computed', got %v", value)
	}

	if !called {
		t.Error("Expected function to be called")
	}

	called = false
	value, _ = cache.GetOrSet("test-key", func() (interface{}, error) {
		called = true
		return "computed2", nil
	}, time.Hour)

	if called {
		t.Error("Expected function not to be called for cached value")
	}
}

func TestCacheStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()
	cache.Set("key1", "value1", time.Hour)
	cache.Set("key2", "value2", time.Hour)

	stats := cache.Stats()

	items, ok := stats["items"].(int)
	if !ok || items != 2 {
		t.Errorf("Expected 2 items, got %v", stats["items"])
	}
}

func TestCacheHashKey(t *testing.T) {
	cache := &Cache{}

	hash1 := cache.HashKey("test-key")
	hash2 := cache.HashKey("test-key")

	if hash1 != hash2 {
		t.Error("Expected same hash for same key")
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64 character hash, got %d", len(hash1))
	}
}

func TestCacheCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cache, _ := NewCache()
	cache.Set("expired", "value", 10*time.Millisecond)
	cache.Set("valid", "value", time.Hour)

	time.Sleep(20 * time.Millisecond)

	cache.Cleanup()

	if cache.Has("expired") {
		t.Error("Expected expired key to be removed")
	}

	if !cache.Has("valid") {
		t.Error("Expected valid key to remain")
	}
}

func TestCacheEntry(t *testing.T) {
	entry := CacheEntry{
		Key:       "test",
		Value:     "value",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		HitCount:  5,
	}

	if entry.Key != "test" {
		t.Errorf("Expected key 'test', got '%s'", entry.Key)
	}

	if entry.HitCount != 5 {
		t.Errorf("Expected hit count 5, got %d", entry.HitCount)
	}
}
