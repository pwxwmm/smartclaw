package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDebugToolExecuteEnable(t *testing.T) {
	tool := &DebugTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"enable": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["debug"] != true {
		t.Error("Expected debug=true")
	}
	if m["log_level"] != "debug" {
		t.Errorf("Expected log_level='debug', got %v", m["log_level"])
	}
	os.Unsetenv("SMARTCLAW_LOG_LEVEL")
}

func TestDebugToolExecuteDisable(t *testing.T) {
	tool := &DebugTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"enable": false,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["debug"] != false {
		t.Error("Expected debug=false")
	}
	if m["log_level"] != "info" {
		t.Errorf("Expected log_level='info', got %v", m["log_level"])
	}
	os.Unsetenv("SMARTCLAW_LOG_LEVEL")
}

func TestEnvToolExecuteGet(t *testing.T) {
	os.Setenv("TEST_SMARTCLAW_ENV", "hello")
	defer os.Unsetenv("TEST_SMARTCLAW_ENV")

	tool := &EnvTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"key": "TEST_SMARTCLAW_ENV",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["value"] != "hello" {
		t.Errorf("Expected value='hello', got %v", m["value"])
	}
}

func TestEnvToolExecuteSet(t *testing.T) {
	tool := &EnvTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"key":   "TEST_SMARTCLAW_SET",
		"value": "world",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["message"] != "Environment set" {
		t.Errorf("Expected message='Environment set', got %v", m["message"])
	}
	if os.Getenv("TEST_SMARTCLAW_SET") != "world" {
		t.Error("Environment variable should be set")
	}
	os.Unsetenv("TEST_SMARTCLAW_SET")
}

func TestEnvToolExecuteNoArgs(t *testing.T) {
	tool := &EnvTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["message"] != "Environment access" {
		t.Errorf("Expected message='Environment access', got %v", m["message"])
	}
}

func TestThinkToolExecute(t *testing.T) {
	tool := &ThinkTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"prompt": "analyze this code",
		"budget": 5000,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["thinking_enabled"] != true {
		t.Error("Expected thinking_enabled=true")
	}
	if m["budget"] != 5000 {
		t.Errorf("Expected budget=5000, got %v", m["budget"])
	}
	os.Unsetenv("SMARTCLAW_THINKING_ENABLED")
	os.Unsetenv("SMARTCLAW_THINKING_BUDGET")
}

func TestThinkToolExecuteDefaultBudget(t *testing.T) {
	tool := &ThinkTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"prompt": "think",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["budget"] != 10000 {
		t.Errorf("Expected default budget=10000, got %v", m["budget"])
	}
	os.Unsetenv("SMARTCLAW_THINKING_ENABLED")
	os.Unsetenv("SMARTCLAW_THINKING_BUDGET")
}

func TestDeepThinkToolExecute(t *testing.T) {
	tool := &DeepThinkTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"prompt": "complex problem",
		"depth":  8,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["thinking_enabled"] != true {
		t.Error("Expected thinking_enabled=true")
	}
	if m["depth"] != 8 {
		t.Errorf("Expected depth=8, got %v", m["depth"])
	}
	// budget = 10000 + 8*8000 = 74000
	if m["budget"] != 74000 {
		t.Errorf("Expected budget=74000, got %v", m["budget"])
	}
	os.Unsetenv("SMARTCLAW_THINKING_ENABLED")
	os.Unsetenv("SMARTCLAW_THINKING_BUDGET")
}

func TestDeepThinkToolExecuteDefaultDepth(t *testing.T) {
	tool := &DeepThinkTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"prompt": "think deeply",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["depth"] != 5 {
		t.Errorf("Expected default depth=5, got %v", m["depth"])
	}
	// budget = 10000 + 5*8000 = 50000
	if m["budget"] != 50000 {
		t.Errorf("Expected budget=50000, got %v", m["budget"])
	}
	os.Unsetenv("SMARTCLAW_THINKING_ENABLED")
	os.Unsetenv("SMARTCLAW_THINKING_BUDGET")
}

func TestLazyToolExecuteEnable(t *testing.T) {
	tool := &LazyTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"enable": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["lazy"] != true {
		t.Error("Expected lazy=true")
	}
	os.Unsetenv("SMARTCLAW_LAZY_MODE")
}

func TestLazyToolExecuteDisable(t *testing.T) {
	tool := &LazyTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"enable": false,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["lazy"] != false {
		t.Error("Expected lazy=false")
	}
	os.Unsetenv("SMARTCLAW_LAZY_MODE")
}

func TestCacheToolExecuteGetMissingKey(t *testing.T) {
	tool := &CacheTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "get",
	})
	if err == nil {
		t.Error("Expected error for missing key")
	}
}

func TestCacheToolExecuteSetMissingKey(t *testing.T) {
	tool := &CacheTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "set",
		"value":  "test",
	})
	if err == nil {
		t.Error("Expected error for missing key")
	}
}

func TestCacheToolExecuteSetAndClear(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(homeDir, ".smartclaw", "cache"), 0755)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", originalHome)

	tool := &CacheTool{}

	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "set",
		"key":    "test_key_coverage",
		"value":  "test_value",
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	m := result.(map[string]any)
	if m["stored"] != true {
		t.Error("Expected stored=true")
	}

	result, err = tool.Execute(context.Background(), map[string]any{
		"action": "clear",
	})
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	m = result.(map[string]any)
	if m["memory_cache_cleared"] != true {
		t.Error("Expected memory_cache_cleared=true")
	}
}

func TestCacheToolExecuteStats(t *testing.T) {
	tool := &CacheTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "stats",
	})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	m := result.(map[string]any)
	if _, ok := m["disk_items"]; !ok {
		t.Error("Expected disk_items in stats")
	}
}

func TestCacheToolExecuteUnknownAction(t *testing.T) {
	tool := &CacheTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "unknown",
	})
	if err != nil {
		t.Fatalf("Unknown action should not error: %v", err)
	}
	m := result.(map[string]any)
	if m["note"] == nil {
		t.Error("Expected note in result for unknown action")
	}
}

func TestNotebookEditToolMissingCellNumber(t *testing.T) {
	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path": "/some/file.ipynb",
	})
	if err == nil {
		t.Error("Expected error for missing cell_number")
	}
}

func TestNotebookEditToolNoCells(t *testing.T) {
	tmpDir := t.TempDir()
	nbPath := filepath.Join(tmpDir, "nocells.ipynb")
	notebook := `{"metadata": {}, "nbformat": 4}`
	os.WriteFile(nbPath, []byte(notebook), 0644)

	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":        nbPath,
		"cell_number": float64(0),
	})
	if err == nil {
		t.Error("Expected error for notebook with no cells")
	}
}

func TestNotebookEditToolNegativeCellNumber(t *testing.T) {
	tmpDir := t.TempDir()
	nbPath := filepath.Join(tmpDir, "neg.ipynb")
	notebook := `{"cells": [{"cell_type": "code", "source": ["x=1"]}], "metadata": {}, "nbformat": 4}`
	os.WriteFile(nbPath, []byte(notebook), 0644)

	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":        nbPath,
		"cell_number": -1,
	})
	if err == nil {
		t.Error("Expected error for negative cell number")
	}
}

func TestNotebookEditToolInvalidCellFormat(t *testing.T) {
	tmpDir := t.TempDir()
	nbPath := filepath.Join(tmpDir, "badcell.ipynb")
	notebook := `{"cells": ["not a map"], "metadata": {}, "nbformat": 4}`
	os.WriteFile(nbPath, []byte(notebook), 0644)

	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":        nbPath,
		"cell_number": 0,
	})
	if err == nil {
		t.Error("Expected error for invalid cell format")
	}
}

func TestNotebookEditToolChangeCellType(t *testing.T) {
	tmpDir := t.TempDir()
	nbPath := filepath.Join(tmpDir, "type.ipynb")
	notebook := `{"cells": [{"cell_type": "code", "source": ["x=1"]}], "metadata": {}, "nbformat": 4}`
	os.WriteFile(nbPath, []byte(notebook), 0644)

	tool := &NotebookEditTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":        nbPath,
		"cell_number": 0,
		"cell_type":   "markdown",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["success"] != true {
		t.Error("Expected success=true")
	}

	data, _ := os.ReadFile(nbPath)
	var nb map[string]any
	json.Unmarshal(data, &nb)
	cells := nb["cells"].([]any)
	cell := cells[0].(map[string]any)
	if cell["cell_type"] != "markdown" {
		t.Error("Expected cell_type=markdown")
	}
}

func TestBrowseToolExecuteWithURL(t *testing.T) {
	tool := &BrowseTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"url": "https://example.com",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["url"] != "https://example.com" {
		t.Errorf("Expected url in result, got %v", m)
	}
}

func TestAttachToolInspectWithInvalidPID(t *testing.T) {
	tool := &AttachTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "inspect",
		"pid":    "999999999",
	})
	if err == nil {
		t.Error("Expected error for non-existent PID")
	}
}

func TestAttachToolUnknownAction(t *testing.T) {
	tool := &AttachTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "foobar",
	})
	if err == nil {
		t.Error("Expected error for unknown action")
	}
}

func TestAttachToolDefaultAction(t *testing.T) {
	tool := &AttachTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["action"] != "list" {
		t.Errorf("Expected default action 'list', got %v", m["action"])
	}
}

func TestForkToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(homeDir, ".smartclaw", "sessions"), 0755)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", originalHome)

	tool := &ForkTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"label": "test-fork",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["label"] != "test-fork" {
		t.Errorf("Expected label='test-fork', got %v", m["label"])
	}
	if m["session_id"] == nil || m["session_id"] == "" {
		t.Error("Expected session_id")
	}
}

func TestNotebookEditToolInputSchema(t *testing.T) {
	tool := &NotebookEditTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["path"]; !ok {
		t.Error("Expected 'path' in schema")
	}
}

func TestBrowseToolInputSchema(t *testing.T) {
	tool := &BrowseTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestDebugToolInputSchema(t *testing.T) {
	tool := &DebugTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestAttachToolInputSchema(t *testing.T) {
	tool := &AttachTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestCacheToolInputSchema(t *testing.T) {
	tool := &CacheTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestObserveToolInputSchema(t *testing.T) {
	tool := &ObserveTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestLazyToolInputSchema(t *testing.T) {
	tool := &LazyTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestThinkToolInputSchema(t *testing.T) {
	tool := &ThinkTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestDeepThinkToolInputSchema(t *testing.T) {
	tool := &DeepThinkTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestForkToolInputSchema(t *testing.T) {
	tool := &ForkTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestEnvToolInputSchema(t *testing.T) {
	tool := &EnvTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}
