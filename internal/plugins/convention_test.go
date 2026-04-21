package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func createTestPluginDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "convention-plugin-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	type pluginEntry struct {
		ptype PluginType
		name  string
		typ   string
		main  string
	}

	entries := []pluginEntry{
		{PluginTypeTool, "my-tool", "tool", "main.sh"},
		{PluginTypeTool, "another-tool", "tool", "main.py"},
		{PluginTypeCommand, "my-cmd", "command", "handler.sh"},
		{PluginTypeAdapter, "discord", "adapter", "adapter.py"},
		{PluginTypeMemory, "redis", "memory", "provider.py"},
		{PluginTypeHook, "notify", "hook", "webhook.sh"},
	}

	for _, e := range entries {
		pluginDir := filepath.Join(dir, string(e.ptype), e.name)
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}

		manifest := map[string]any{
			"name":        e.name,
			"version":     "1.0.0",
			"description": e.name + " plugin",
			"type":        e.typ,
			"main":        e.main,
			"enabled":     true,
			"config": map[string]string{
				"key": "value",
			},
		}
		data, _ := json.Marshal(manifest)
		if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pluginDir, e.main), []byte("#!/bin/sh\necho hello"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func createTypeMismatchDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "convention-mismatch-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	pluginDir := filepath.Join(dir, "tool", "bad-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := map[string]any{
		"name":    "bad-plugin",
		"version": "1.0.0",
		"type":    "command",
		"main":    "main.sh",
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createMissingManifestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "convention-missing-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	pluginDir := filepath.Join(dir, "tool", "no-manifest")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNewConventionPluginLoader(t *testing.T) {
	loader := NewConventionPluginLoader("/tmp/test-plugins")
	if loader.baseDir != "/tmp/test-plugins" {
		t.Errorf("expected baseDir /tmp/test-plugins, got %s", loader.baseDir)
	}
	if loader.plugins == nil {
		t.Error("plugins map should be initialized")
	}
}

func TestLoadAll(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)

	if err := loader.LoadAll(); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	all := loader.ListAll()
	if len(all) != 6 {
		t.Errorf("expected 6 plugins, got %d", len(all))
	}
}

func TestLoadSpecificPlugin(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)

	cp, err := loader.Load(PluginTypeTool, "my-tool")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cp.Type != PluginTypeTool {
		t.Errorf("expected type tool, got %s", cp.Type)
	}
	if cp.Name != "my-tool" {
		t.Errorf("expected name my-tool, got %s", cp.Name)
	}
	if cp.Manifest.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", cp.Manifest.Version)
	}
	if cp.Manifest.Main != "main.sh" {
		t.Errorf("expected main main.sh, got %s", cp.Manifest.Main)
	}
	if !cp.Enabled {
		t.Error("plugin should be enabled")
	}
	if cp.LoadedAt.IsZero() {
		t.Error("LoadedAt should be set")
	}
	if cp.Config["key"] != "value" {
		t.Errorf("expected config key=value, got %s", cp.Config["key"])
	}
}

func TestGet(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)
	loader.LoadAll()

	cp := loader.Get(PluginTypeTool, "my-tool")
	if cp == nil {
		t.Fatal("expected plugin, got nil")
	}
	if cp.Name != "my-tool" {
		t.Errorf("expected my-tool, got %s", cp.Name)
	}

	missing := loader.Get(PluginTypeTool, "nonexistent")
	if missing != nil {
		t.Error("expected nil for nonexistent plugin")
	}
}

func TestListByType(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)
	loader.LoadAll()

	tools := loader.ListByType(PluginTypeTool)
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}

	commands := loader.ListByType(PluginTypeCommand)
	if len(commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(commands))
	}

	empty := loader.ListByType(PluginType("nonexistent"))
	if len(empty) != 0 {
		t.Errorf("expected 0 for invalid type, got %d", len(empty))
	}
}

func TestListAll(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)
	loader.LoadAll()

	all := loader.ListAll()
	if len(all) != 6 {
		t.Errorf("expected 6 plugins, got %d", len(all))
	}
}

func TestEnableDisable(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)
	loader.LoadAll()

	if err := loader.Disable(PluginTypeTool, "my-tool"); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}

	cp := loader.Get(PluginTypeTool, "my-tool")
	if cp.Enabled {
		t.Error("plugin should be disabled")
	}
	if cp.Manifest.Enabled {
		t.Error("manifest should also be disabled")
	}

	if err := loader.Enable(PluginTypeTool, "my-tool"); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	cp = loader.Get(PluginTypeTool, "my-tool")
	if !cp.Enabled {
		t.Error("plugin should be enabled")
	}

	err := loader.Enable(PluginTypeTool, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}

	err = loader.Disable(PluginTypeTool, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestTypeMismatchDetection(t *testing.T) {
	dir := createTypeMismatchDir(t)
	loader := NewConventionPluginLoader(dir)

	_, err := loader.Load(PluginTypeTool, "bad-plugin")
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
}

func TestMissingPluginJSON(t *testing.T) {
	dir := createMissingManifestDir(t)
	loader := NewConventionPluginLoader(dir)

	_, err := loader.Load(PluginTypeTool, "no-manifest")
	if err == nil {
		t.Fatal("expected error for missing plugin.json")
	}
}

func TestGetTools(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)
	loader.LoadAll()

	tools := loader.GetTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools via GetTools, got %d", len(tools))
	}
}

func TestGetCommands(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)
	loader.LoadAll()

	cmds := loader.GetCommands()
	if len(cmds) != 1 {
		t.Errorf("expected 1 command via GetCommands, got %d", len(cmds))
	}
}

func TestInvalidPluginType(t *testing.T) {
	dir := createTestPluginDir(t)
	loader := NewConventionPluginLoader(dir)

	_, err := loader.Load(PluginType("invalid"), "foo")
	if err == nil {
		t.Fatal("expected error for invalid plugin type")
	}
}

func TestLoadAllSkipsBadPlugins(t *testing.T) {
	dir := createTypeMismatchDir(t)
	loader := NewConventionPluginLoader(dir)

	if err := loader.LoadAll(); err != nil {
		t.Fatalf("LoadAll should not fail on individual plugin errors: %v", err)
	}

	all := loader.ListAll()
	if len(all) != 0 {
		t.Errorf("expected 0 plugins with bad manifest, got %d", len(all))
	}
}

func TestLoadAllNonexistentBaseDir(t *testing.T) {
	loader := NewConventionPluginLoader("/nonexistent/path")

	if err := loader.LoadAll(); err != nil {
		t.Fatalf("LoadAll should not fail on nonexistent baseDir: %v", err)
	}

	all := loader.ListAll()
	if len(all) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(all))
	}
}
