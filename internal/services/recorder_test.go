package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSessionRecorder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recorder-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.jsonl")
	recorder, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatalf("NewSessionRecorder failed: %v", err)
	}

	if recorder.IsRecording() {
		t.Error("Recorder should not be recording initially")
	}

	if recorder.GetPath() != path {
		t.Errorf("Expected path %s, got %s", path, recorder.GetPath())
	}
}

func TestNewSessionRecorderDefaultPath(t *testing.T) {
	recorder, err := NewSessionRecorder("")
	if err != nil {
		t.Fatalf("NewSessionRecorder with empty path failed: %v", err)
	}

	if recorder.GetPath() == "" {
		t.Error("Expected non-empty default path")
	}
}

func TestSessionRecorderStartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recorder-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.jsonl")
	recorder, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := recorder.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !recorder.IsRecording() {
		t.Error("Recorder should be recording after Start")
	}

	if err := recorder.Start(); err == nil {
		t.Error("Expected error when starting already recording session")
	}

	if err := recorder.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if recorder.IsRecording() {
		t.Error("Recorder should not be recording after Stop")
	}
}

func TestSessionRecorderRecordMessage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recorder-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.jsonl")
	recorder, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := recorder.RecordMessage("user", "hello"); err != nil {
		t.Errorf("RecordMessage should be no-op when not recording, got: %v", err)
	}

	if err := recorder.Start(); err != nil {
		t.Fatal(err)
	}

	if err := recorder.RecordMessage("user", "hello"); err != nil {
		t.Fatalf("RecordMessage failed: %v", err)
	}

	if err := recorder.RecordMessage("assistant", "hi there"); err != nil {
		t.Fatalf("RecordMessage failed: %v", err)
	}

	entries := recorder.GetEntries()
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	if entries[0].Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", entries[0].Type)
	}

	if entries[0].Data["role"] != "user" {
		t.Errorf("Expected role 'user', got '%v'", entries[0].Data["role"])
	}

	if err := recorder.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionRecorderRecordToolCall(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recorder-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.jsonl")
	recorder, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := recorder.Start(); err != nil {
		t.Fatal(err)
	}
	defer recorder.Stop()

	if err := recorder.RecordToolCall("bash", map[string]any{"command": "ls"}); err != nil {
		t.Fatalf("RecordToolCall failed: %v", err)
	}

	entries := recorder.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Type != "tool_call" {
		t.Errorf("Expected type 'tool_call', got '%s'", entries[0].Type)
	}
}

func TestPlaybackLoadAndRead(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "playback-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.jsonl")

	recorder, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := recorder.Start(); err != nil {
		t.Fatal(err)
	}

	recorder.RecordMessage("user", "hello")
	recorder.RecordMessage("assistant", "hi there")
	recorder.RecordToolCall("bash", map[string]any{"command": "ls"})

	if err := recorder.Stop(); err != nil {
		t.Fatal(err)
	}

	playback, err := NewPlayback(path)
	if err != nil {
		t.Fatalf("NewPlayback failed: %v", err)
	}
	defer playback.Close()

	if err := playback.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	entries := playback.GetAll()
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	entry, ok := playback.Next()
	if !ok {
		t.Fatal("Expected entry from Next")
	}
	if entry.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", entry.Type)
	}

	playback.Rewind()
	entry, ok = playback.Next()
	if !ok {
		t.Fatal("Expected entry after Rewind")
	}
	if entry.Data["role"] != "user" {
		t.Errorf("Expected first entry role 'user', got '%v'", entry.Data["role"])
	}

	for {
		_, ok := playback.Next()
		if !ok {
			break
		}
	}
	_, ok = playback.Next()
	if ok {
		t.Error("Expected no more entries after consuming all")
	}
}

func TestPlaybackNonExistentFile(t *testing.T) {
	_, err := NewPlayback("/nonexistent/file.jsonl")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestGetEntriesReturnsCopy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recorder-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.jsonl")
	recorder, err := NewSessionRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := recorder.Start(); err != nil {
		t.Fatal(err)
	}
	defer recorder.Stop()

	recorder.RecordMessage("user", "hello")

	entries1 := recorder.GetEntries()
	entries2 := recorder.GetEntries()

	if len(entries1) != len(entries2) {
		t.Error("GetEntries should return consistent copies")
	}

	if &entries1[0] == &entries2[0] {
		t.Error("GetEntries should return copies, not references to same slice elements")
	}
}
