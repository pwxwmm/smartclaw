package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	path, err := getConfigPath()
	if err != nil {
		t.Errorf("Expected no error getting config path, got %v", err)
	}

	if path == "" {
		t.Error("Expected non-empty config path")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got '%s'", path)
	}
}

func TestLoadConfigNonexistent(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Errorf("Expected no error for nonexistent config, got %v", err)
	}

	if cfg == nil {
		t.Error("Expected non-nil config")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-auth-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg := &Config{
		APIKey:       "test-api-key-123",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    1234567890,
		Model:        "claude-sonnet-4-5",
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.APIKey != cfg.APIKey {
		t.Errorf("Expected API key '%s', got '%s'", cfg.APIKey, loaded.APIKey)
	}

	if loaded.AccessToken != cfg.AccessToken {
		t.Errorf("Expected access token '%s', got '%s'", cfg.AccessToken, loaded.AccessToken)
	}

	if loaded.Model != cfg.Model {
		t.Errorf("Expected model '%s', got '%s'", cfg.Model, loaded.Model)
	}
}

func TestGetAPIKeyFromEnv(t *testing.T) {
	oldKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "env-api-key")
	defer os.Setenv("ANTHROPIC_API_KEY", oldKey)

	key := GetAPIKey()
	if key != "env-api-key" {
		t.Errorf("Expected API key 'env-api-key', got '%s'", key)
	}
}

func TestGetAPIKeyFromConfig(t *testing.T) {
	oldKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Setenv("ANTHROPIC_API_KEY", oldKey)

	tmpDir, err := os.MkdirTemp("", "claw-auth-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfg := &Config{APIKey: "config-api-key"}
	SaveConfig(cfg)

	key := GetAPIKey()
	if key != "config-api-key" {
		t.Errorf("Expected API key 'config-api-key', got '%s'", key)
	}
}

func TestSetAPIKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-auth-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Setenv("ANTHROPIC_API_KEY", oldKey)

	if err := SetAPIKey("new-api-key"); err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}

	cfg, _ := LoadConfig()
	if cfg.APIKey != "new-api-key" {
		t.Errorf("Expected API key 'new-api-key', got '%s'", cfg.APIKey)
	}
}

func TestConfigFields(t *testing.T) {
	cfg := &Config{
		APIKey:       "key123",
		AccessToken:  "token123",
		RefreshToken: "refresh123",
		ExpiresAt:    9999999999,
		Model:        "claude-opus-4-6",
		BaseURL:      "https://custom.api.com",
	}

	if cfg.APIKey != "key123" {
		t.Errorf("Expected API key 'key123', got '%s'", cfg.APIKey)
	}

	if cfg.AccessToken != "token123" {
		t.Errorf("Expected access token 'token123', got '%s'", cfg.AccessToken)
	}

	if cfg.RefreshToken != "refresh123" {
		t.Errorf("Expected refresh token 'refresh123', got '%s'", cfg.RefreshToken)
	}

	if cfg.Model != "claude-opus-4-6" {
		t.Errorf("Expected model 'claude-opus-4-6', got '%s'", cfg.Model)
	}

	if cfg.BaseURL != "https://custom.api.com" {
		t.Errorf("Expected base URL 'https://custom.api.com', got '%s'", cfg.BaseURL)
	}
}
