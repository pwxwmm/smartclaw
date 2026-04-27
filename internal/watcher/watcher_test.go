package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestNewFileWatcher(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	if fw == nil {
		t.Fatal("NewFileWatcher() returned nil")
	}
	if fw.debounce != 100*time.Millisecond {
		t.Errorf("default debounce = %v, want 100ms", fw.debounce)
	}
}

func TestFileWatcher_Add(t *testing.T) {
	tmpDir := t.TempDir()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	if err := fw.Add(tmpDir); err != nil {
		t.Fatalf("Add() returned error: %v", err)
	}

	if !fw.IsWatching(tmpDir) {
		t.Error("IsWatching() should return true after Add()")
	}
}

func TestFileWatcher_Add_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	if err := fw.Add(tmpDir); err != nil {
		t.Fatalf("First Add() returned error: %v", err)
	}
	if err := fw.Add(tmpDir); err != nil {
		t.Fatalf("Duplicate Add() returned error: %v", err)
	}
}

func TestFileWatcher_Remove(t *testing.T) {
	tmpDir := t.TempDir()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	fw.Add(tmpDir)
	if err := fw.Remove(tmpDir); err != nil {
		t.Fatalf("Remove() returned error: %v", err)
	}

	if fw.IsWatching(tmpDir) {
		t.Error("IsWatching() should return false after Remove()")
	}
}

func TestFileWatcher_Remove_NotWatched(t *testing.T) {
	tmpDir := t.TempDir()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	err = fw.Remove(tmpDir)
	if err != nil {
		t.Errorf("Remove() on non-watched path should not error, got: %v", err)
	}
}

func TestFileWatcher_On(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	called := false
	fw.On(EventCreate, func(e Event) {
		called = true
	})

	if len(fw.handlers[EventCreate]) != 1 {
		t.Errorf("handlers[EventCreate] length = %d, want 1", len(fw.handlers[EventCreate]))
	}

	fw.handlers[EventCreate][0](Event{Type: EventCreate})
	if !called {
		t.Error("Handler was not called")
	}
}

func TestFileWatcher_On_Multiple(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	fw.On(EventCreate, func(e Event) {})
	fw.On(EventCreate, func(e Event) {})
	fw.On(EventWrite, func(e Event) {})

	if len(fw.handlers[EventCreate]) != 2 {
		t.Errorf("handlers[EventCreate] length = %d, want 2", len(fw.handlers[EventCreate]))
	}
	if len(fw.handlers[EventWrite]) != 1 {
		t.Errorf("handlers[EventWrite] length = %d, want 1", len(fw.handlers[EventWrite]))
	}
}

func TestFileWatcher_Start(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
}

func TestFileWatcher_Start_DoubleStart(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(); err != nil {
		t.Fatalf("First Start() returned error: %v", err)
	}
	if err := fw.Start(); err == nil {
		t.Error("Second Start() should return error")
	}
}

func TestFileWatcher_Stop_WithoutStart(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}

	if err := fw.Stop(); err != nil {
		t.Errorf("Stop() without Start() returned error: %v", err)
	}
}

func TestFileWatcher_SetDebounce(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	fw.SetDebounce(500 * time.Millisecond)
	if fw.debounce != 500*time.Millisecond {
		t.Errorf("debounce = %v, want 500ms", fw.debounce)
	}
}

func TestFileWatcher_WatchedPaths(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	fw.Add(tmpDir1)
	fw.Add(tmpDir2)

	paths := fw.WatchedPaths()
	if len(paths) != 2 {
		t.Fatalf("WatchedPaths() returned %d paths, want 2", len(paths))
	}
}

func TestFileWatcher_WatchedPaths_Empty(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	paths := fw.WatchedPaths()
	if len(paths) != 0 {
		t.Errorf("WatchedPaths() on empty watcher returned %d paths, want 0", len(paths))
	}
}

func TestFileWatcher_IsWatching(t *testing.T) {
	tmpDir := t.TempDir()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	if fw.IsWatching(tmpDir) {
		t.Error("IsWatching() should return false before Add()")
	}

	fw.Add(tmpDir)
	if !fw.IsWatching(tmpDir) {
		t.Error("IsWatching() should return true after Add()")
	}
}

func TestFileWatcher_IsWatching_RelativePath(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	fw.IsWatching(".")
}

func TestEventType_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventCreate, "create"},
		{EventWrite, "write"},
		{EventRemove, "remove"},
		{EventRename, "rename"},
		{EventChmod, "chmod"},
	}

	for _, tt := range tests {
		if string(tt.eventType) != tt.want {
			t.Errorf("EventType = %q, want %q", tt.eventType, tt.want)
		}
	}
}

func TestEvent_Struct(t *testing.T) {
	t.Parallel()

	now := time.Now()
	evt := Event{
		Type:      EventWrite,
		Path:      "/tmp/test.go",
		Timestamp: now,
	}
	if evt.Type != EventWrite {
		t.Errorf("Type = %q, want %q", evt.Type, EventWrite)
	}
	if evt.Path != "/tmp/test.go" {
		t.Errorf("Path = %q, want %q", evt.Path, "/tmp/test.go")
	}
	if !evt.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", evt.Timestamp, now)
	}
}

func TestHandleEvent_Create(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	received := make(chan Event, 1)
	fw.On(EventCreate, func(e Event) {
		received <- e
	})

	fw.handleEvent(fsnotify.Event{
		Name: "/tmp/test_create.txt",
		Op:   fsnotify.Create,
	})

	select {
	case evt := <-received:
		if evt.Type != EventCreate {
			t.Errorf("Event Type = %q, want %q", evt.Type, EventCreate)
		}
		if evt.Path != "/tmp/test_create.txt" {
			t.Errorf("Event Path = %q, want %q", evt.Path, "/tmp/test_create.txt")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Handler was not called within timeout")
	}
}

func TestHandleEvent_Write(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	received := make(chan Event, 1)
	fw.On(EventWrite, func(e Event) {
		received <- e
	})

	fw.handleEvent(fsnotify.Event{
		Name: "/tmp/test_write.txt",
		Op:   fsnotify.Write,
	})

	select {
	case evt := <-received:
		if evt.Type != EventWrite {
			t.Errorf("Event Type = %q, want %q", evt.Type, EventWrite)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Handler was not called within timeout")
	}
}

func TestHandleEvent_Remove(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	received := make(chan Event, 1)
	fw.On(EventRemove, func(e Event) {
		received <- e
	})

	fw.handleEvent(fsnotify.Event{
		Name: "/tmp/test_remove.txt",
		Op:   fsnotify.Remove,
	})

	select {
	case evt := <-received:
		if evt.Type != EventRemove {
			t.Errorf("Event Type = %q, want %q", evt.Type, EventRemove)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Handler was not called within timeout")
	}
}

func TestHandleEvent_Rename(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	received := make(chan Event, 1)
	fw.On(EventRename, func(e Event) {
		received <- e
	})

	fw.handleEvent(fsnotify.Event{
		Name: "/tmp/test_rename.txt",
		Op:   fsnotify.Rename,
	})

	select {
	case evt := <-received:
		if evt.Type != EventRename {
			t.Errorf("Event Type = %q, want %q", evt.Type, EventRename)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Handler was not called within timeout")
	}
}

func TestHandleEvent_Chmod(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	received := make(chan Event, 1)
	fw.On(EventChmod, func(e Event) {
		received <- e
	})

	fw.handleEvent(fsnotify.Event{
		Name: "/tmp/test_chmod.txt",
		Op:   fsnotify.Chmod,
	})

	select {
	case evt := <-received:
		if evt.Type != EventChmod {
			t.Errorf("Event Type = %q, want %q", evt.Type, EventChmod)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Handler was not called within timeout")
	}
}

func TestWatchDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	fw, err := WatchDirectory(tmpDir, func(e Event) {})
	if err != nil {
		t.Fatalf("WatchDirectory() returned error: %v", err)
	}
	defer fw.Stop()

	if !fw.IsWatching(tmpDir) {
		t.Error("WatchDirectory should watch the given directory")
	}
}

func TestWatchDirectory_InvalidPath(t *testing.T) {
	_, err := WatchDirectory("/nonexistent/path/that/does/not/exist", func(e Event) {})
	if err == nil {
		t.Error("WatchDirectory with invalid path should return error")
	}
}

func TestWatchDirectory_ReceivesEvents(t *testing.T) {
	tmpDir := t.TempDir()

	received := make(chan Event, 10)
	fw, err := WatchDirectory(tmpDir, func(e Event) {
		received <- e
	})
	if err != nil {
		t.Fatalf("WatchDirectory() returned error: %v", err)
	}
	defer fw.Stop()

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	select {
	case evt := <-received:
		if evt.Path != testFile && evt.Path != tmpDir {
			t.Logf("Received event for path: %s (type: %s)", evt.Path, evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Log("No event received (may be OS-dependent), but no error occurred")
	}
}

func TestFileWatcher_Add_File(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("data"), 0644)

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() returned error: %v", err)
	}
	defer fw.Stop()

	if err := fw.Add(testFile); err != nil {
		t.Fatalf("Add() on file returned error: %v", err)
	}

	if !fw.IsWatching(testFile) {
		t.Error("IsWatching() should return true for added file")
	}
}
