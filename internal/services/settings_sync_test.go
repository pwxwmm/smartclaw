package services

import (
	"context"
	"testing"
	"time"
)

func TestNewSettingsSync(t *testing.T) {
	sync, err := NewSettingsSync()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if sync == nil {
		t.Fatal("Expected non-nil SettingsSync")
	}
}

func TestSettingsSyncGetSettings(t *testing.T) {
	sync, _ := NewSettingsSync()

	settings := sync.GetSettings()
	if settings == nil {
		t.Error("Expected non-nil settings")
	}
}

func TestSettingsSyncUpdateSettings(t *testing.T) {
	sync, _ := NewSettingsSync()

	updates := map[string]any{
		"model": "claude-opus-4-6",
	}

	err := sync.UpdateSettings(context.Background(), updates)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	settings := sync.GetSettings()
	if settings.Model != "claude-opus-4-6" {
		t.Errorf("Expected model 'claude-opus-4-6', got '%s'", settings.Model)
	}
}

func TestSettingsSyncConfigureRemote(t *testing.T) {
	sync, _ := NewSettingsSync()

	sync.ConfigureRemote("https://api.example.com", "token123")

	if sync.remoteURL != "https://api.example.com" {
		t.Errorf("Expected remote URL 'https://api.example.com', got '%s'", sync.remoteURL)
	}

	if sync.authToken != "token123" {
		t.Errorf("Expected auth token 'token123', got '%s'", sync.authToken)
	}
}

func TestSettingsSyncSetSyncEnabled(t *testing.T) {
	sync, _ := NewSettingsSync()

	sync.SetSyncEnabled(true)

	if !sync.syncEnabled {
		t.Error("Expected syncEnabled to be true")
	}
}

func TestSettingsSyncGetVersionHistory(t *testing.T) {
	sync, _ := NewSettingsSync()

	history := sync.GetVersionHistory()
	if history == nil {
		t.Error("Expected non-nil history")
	}
}

func TestSettingsSyncGetLastSyncTime(t *testing.T) {
	sync, _ := NewSettingsSync()

	time := sync.GetLastSyncTime()
	if time.IsZero() {
		t.Log("LastSyncTime is zero (expected for new sync)")
	}
}

func TestSettings(t *testing.T) {
	settings := Settings{
		Model:      "claude-sonnet-4-5",
		MaxTokens:  4096,
		Permission: "ask",
	}

	if settings.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected model 'claude-sonnet-4-5', got '%s'", settings.Model)
	}

	if settings.MaxTokens != 4096 {
		t.Errorf("Expected max tokens 4096, got %d", settings.MaxTokens)
	}
}

func TestSettingsVersion(t *testing.T) {
	version := SettingsVersion{
		Version:   1,
		Timestamp: MustParseTime("2024-01-01T00:00:00Z"),
		Changes:   []string{"Initial version"},
	}

	if version.Version != 1 {
		t.Errorf("Expected version 1, got %d", version.Version)
	}

	if len(version.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(version.Changes))
	}
}

func TestConflictResolution(t *testing.T) {
	if ConflictLocalWins != 0 {
		t.Errorf("Expected ConflictLocalWins to be 0, got %d", ConflictLocalWins)
	}

	if ConflictRemoteWins != 1 {
		t.Errorf("Expected ConflictRemoteWins to be 1, got %d", ConflictRemoteWins)
	}

	if ConflictMerge != 2 {
		t.Errorf("Expected ConflictMerge to be 2, got %d", ConflictMerge)
	}
}

func MustParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
