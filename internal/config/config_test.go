package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("Expected non-nil default config")
	}

	if cfg.Model == "" {
		t.Error("Expected default model to be set")
	}

	if cfg.MaxTokens <= 0 {
		t.Error("Expected positive max tokens")
	}

	if cfg.Permission == "" {
		t.Error("Expected default permission to be set")
	}
}

func TestLoadNonexistent(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("Expected no error for nonexistent file, got %v", err)
	}

	if cfg == nil {
		t.Error("Expected non-nil config")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claw-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Model:      "claude-opus-4-6",
		MaxTokens:  8192,
		Permission: "workspace-write",
		Plugins:    []string{"plugin1", "plugin2"},
	}

	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.Model != cfg.Model {
		t.Errorf("Expected model '%s', got '%s'", cfg.Model, loaded.Model)
	}

	if loaded.MaxTokens != cfg.MaxTokens {
		t.Errorf("Expected max tokens %d, got %d", cfg.MaxTokens, loaded.MaxTokens)
	}

	if loaded.Permission != cfg.Permission {
		t.Errorf("Expected permission '%s', got '%s'", cfg.Permission, loaded.Permission)
	}
}

func TestConfigWithMCPServers(t *testing.T) {
	cfg := &Config{
		Model: "claude-sonnet-4-5",
		MCPServers: map[string]MCPServer{
			"test-server": {
				Command: "test-command",
				Args:    []string{"--arg1", "value1"},
				Env: map[string]string{
					"ENV1": "value1",
				},
			},
		},
	}

	if len(cfg.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server, got %d", len(cfg.MCPServers))
	}

	server, ok := cfg.MCPServers["test-server"]
	if !ok {
		t.Error("Expected 'test-server' to exist")
	}

	if server.Command != "test-command" {
		t.Errorf("Expected command 'test-command', got '%s'", server.Command)
	}
}

func TestConfigWithHooks(t *testing.T) {
	cfg := &Config{
		Hooks: map[string][]Hook{
			"PreToolUse": {
				{
					Type:    "PreToolUse",
					Command: "echo 'before'",
					Tools:   []string{"bash", "read_file"},
				},
			},
		},
	}

	if len(cfg.Hooks) != 1 {
		t.Errorf("Expected 1 hook type, got %d", len(cfg.Hooks))
	}

	hooks := cfg.Hooks["PreToolUse"]
	if len(hooks) != 1 {
		t.Errorf("Expected 1 PreToolUse hook, got %d", len(hooks))
	}

	if hooks[0].Command != "echo 'before'" {
		t.Errorf("Expected command 'echo before', got '%s'", hooks[0].Command)
	}
}

func TestConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Errorf("Expected no error getting config path, got %v", err)
	}

	if path == "" {
		t.Error("Expected non-empty config path")
	}
}

func TestConfigSet(t *testing.T) {
	err := Set("model", "claude-opus-4-6")
	if err != nil {
		t.Logf("Set returned error (expected if no config): %v", err)
	}
}

func TestConfigCustomFields(t *testing.T) {
	cfg := &Config{
		Model: "claude-sonnet-4-5",
		Custom: map[string]interface{}{
			"custom_key": "custom_value",
			"number":     42,
		},
	}

	if len(cfg.Custom) != 2 {
		t.Errorf("Expected 2 custom fields, got %d", len(cfg.Custom))
	}

	if cfg.Custom["custom_key"] != "custom_value" {
		t.Error("Expected custom_key to be custom_value")
	}
}

func TestMCPServer(t *testing.T) {
	server := MCPServer{
		Command: "/usr/bin/node",
		Args:    []string{"server.js", "--port", "8080"},
		Env: map[string]string{
			"NODE_ENV": "production",
		},
	}

	if server.Command != "/usr/bin/node" {
		t.Errorf("Expected command '/usr/bin/node', got '%s'", server.Command)
	}

	if len(server.Args) != 3 {
		t.Errorf("Expected 3 args, got %d", len(server.Args))
	}

	if server.Env["NODE_ENV"] != "production" {
		t.Error("Expected NODE_ENV to be production")
	}
}

func TestHook(t *testing.T) {
	hook := Hook{
		Type:    "PostToolUse",
		Command: "notify-send 'Done'",
		Tools:   []string{"bash"},
	}

	if hook.Type != "PostToolUse" {
		t.Errorf("Expected type 'PostToolUse', got '%s'", hook.Type)
	}

	if len(hook.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(hook.Tools))
	}
}
