package history

import (
	"os"
	"testing"
	"time"
)

func TestNewHistory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, err := NewHistory()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if history == nil {
		t.Fatal("Expected non-nil history")
	}
}

func TestHistoryAdd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()

	err = history.Add("test command", "session-1", 0)
	if err != nil {
		t.Errorf("Expected no error adding, got %v", err)
	}

	entries := history.Get(10)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Command != "test command" {
		t.Errorf("Expected 'test command', got '%s'", entries[0].Command)
	}
}

func TestHistoryGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()
	history.Add("cmd1", "s1", 0)
	history.Add("cmd2", "s1", 0)
	history.Add("cmd3", "s1", 0)

	entries := history.Get(2)
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

func TestHistorySearch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()
	history.Add("git status", "s1", 0)
	history.Add("git commit", "s1", 0)
	history.Add("ls -la", "s1", 0)

	results := history.Search("git", 10)
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestHistoryClear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()
	history.Add("cmd1", "s1", 0)
	history.Add("cmd2", "s1", 0)

	err = history.Clear()
	if err != nil {
		t.Errorf("Expected no error clearing, got %v", err)
	}

	entries := history.Get(10)
	if len(entries) != 0 {
		t.Error("Expected empty history after clear")
	}
}

func TestHistoryGetUnique(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()
	history.Add("git status", "s1", 0)
	history.Add("git status", "s1", 0)
	history.Add("git commit", "s1", 0)

	unique := history.GetUnique(10)
	if len(unique) != 2 {
		t.Errorf("Expected 2 unique commands, got %d", len(unique))
	}
}

func TestHistoryStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()
	history.Add("cmd1", "s1", 0)
	history.Add("cmd2", "s1", 1)
	history.Add("cmd1", "s1", 0)

	stats := history.Stats()

	total, ok := stats["total_commands"].(int)
	if !ok || total != 3 {
		t.Errorf("Expected 3 total commands, got %v", stats["total_commands"])
	}

	unique, ok := stats["unique_commands"].(int)
	if !ok || unique != 2 {
		t.Errorf("Expected 2 unique commands, got %v", stats["unique_commands"])
	}
}

func TestHistoryLastCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()

	if history.LastCommand() != nil {
		t.Error("Expected nil for empty history")
	}

	history.Add("first", "s1", 0)
	history.Add("last", "s1", 0)

	last := history.LastCommand()
	if last == nil {
		t.Fatal("Expected non-nil last command")
	}

	if last.Command != "last" {
		t.Errorf("Expected 'last', got '%s'", last.Command)
	}
}

func TestHistoryGetBySession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()
	history.Add("cmd1", "session-a", 0)
	history.Add("cmd2", "session-b", 0)
	history.Add("cmd3", "session-a", 0)

	entries := history.GetBySession("session-a")
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries for session-a, got %d", len(entries))
	}
}

func TestHistoryAutocomplete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-history-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	history, _ := NewHistory()
	history.Add("git status", "s1", 0)
	history.Add("git commit", "s1", 0)

	suggestions := history.Autocomplete("git")
	if len(suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(suggestions))
	}
}

func TestHistoryEntry(t *testing.T) {
	entry := HistoryEntry{
		Command:   "test command",
		Timestamp: time.Now(),
		SessionID: "session-123",
		ExitCode:  0,
	}

	if entry.Command != "test command" {
		t.Errorf("Expected 'test command', got '%s'", entry.Command)
	}

	if entry.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", entry.ExitCode)
	}
}
