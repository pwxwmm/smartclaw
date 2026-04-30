package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScheduleCronToolSchedule(t *testing.T) {
	tool := &ScheduleCronTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"action":   "schedule",
		"schedule": "*/5 * * * *",
		"command":  "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["status"] != "scheduled" {
		t.Errorf("Expected status 'scheduled', got %v", resultMap["status"])
	}
	if resultMap["id"] == "" {
		t.Error("Expected task ID")
	}

	taskID := resultMap["id"].(string)
	taskPath := resultMap["path"].(string)
	if _, err := os.Stat(taskPath); err != nil {
		t.Errorf("Cron task file should exist at %s", taskPath)
	}

	cleanup, _ := filepath.Match("cron_*.json", filepath.Base(taskPath))
	_ = cleanup
	os.Remove(taskPath)

	_ = taskID
}

func TestScheduleCronToolListCov(t *testing.T) {
	tmpDir := t.TempDir()

	tool := &ScheduleCronTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	_ = tmpDir
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["count"] == nil {
		t.Error("Expected count in result")
	}
}

func TestScheduleCronToolDeleteMissingIDCov(t *testing.T) {
	tool := &ScheduleCronTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "delete",
	})
	if err == nil {
		t.Error("Expected error for missing task_id")
	}
}

func TestScheduleCronToolUnknownAction(t *testing.T) {
	tool := &ScheduleCronTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "unknown",
	})
	if err == nil {
		t.Error("Expected error for unknown action")
	}
}

func TestRemoteTriggerToolMissingHost(t *testing.T) {
	tool := &RemoteTriggerTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"command": "ls",
	})
	if err == nil {
		t.Error("Expected error for missing host")
	}
}

func TestRemoteTriggerToolMissingCommand(t *testing.T) {
	tool := &RemoteTriggerTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"host": "user@example.com",
	})
	if err == nil {
		t.Error("Expected error for missing command")
	}
}

func TestRemoteTriggerToolSchema(t *testing.T) {
	tool := &RemoteTriggerTool{}
	if tool.Name() != "remote_trigger" {
		t.Errorf("Expected name 'remote_trigger', got '%s'", tool.Name())
	}
}

func TestSendMessageToolMissingChannel(t *testing.T) {
	tool := &SendMessageTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"message": "hello",
	})
	if err == nil {
		t.Error("Expected error for missing channel")
	}
}

func TestSendMessageToolMissingMessage(t *testing.T) {
	tool := &SendMessageTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"channel": "general",
	})
	if err == nil {
		t.Error("Expected error for missing message")
	}
}

func TestSendMessageToolSchema(t *testing.T) {
	tool := &SendMessageTool{}
	if tool.Name() != "send_message" {
		t.Errorf("Expected name 'send_message', got '%s'", tool.Name())
	}
}

func TestNotebookEditToolMissingPath(t *testing.T) {
	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"cell_number": 0,
	})
	if err == nil {
		t.Error("Expected error for missing path")
	}
}

func TestNotebookEditToolNonExistentPath(t *testing.T) {
	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":        "/nonexistent/notebook.ipynb",
		"cell_number": 0,
	})
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestNotebookEditToolWithNotebook(t *testing.T) {
	tmpDir := t.TempDir()
	nbPath := filepath.Join(tmpDir, "test.ipynb")

	notebook := `{
		"cells": [{"cell_type": "code", "source": ["print('hello')"]}],
		"metadata": {},
		"nbformat": 4,
		"nbformat_minor": 4
	}`
	os.WriteFile(nbPath, []byte(notebook), 0644)

	tool := &NotebookEditTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":        nbPath,
		"cell_number": 0,
		"source":      "print('world')",
		"cell_type":   "code",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Expected success=true")
	}
}

func TestNotebookEditToolInvalidNotebook(t *testing.T) {
	tmpDir := t.TempDir()
	nbPath := filepath.Join(tmpDir, "bad.ipynb")
	os.WriteFile(nbPath, []byte("not a notebook"), 0644)

	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":        nbPath,
		"cell_number": 0,
	})
	if err == nil {
		t.Error("Expected error for invalid notebook format")
	}
}

func TestNotebookEditToolInvalidCellNumber(t *testing.T) {
	tmpDir := t.TempDir()
	nbPath := filepath.Join(tmpDir, "test.ipynb")

	notebook := `{
		"cells": [{"cell_type": "code", "source": ["print('hello')"]}],
		"metadata": {},
		"nbformat": 4,
		"nbformat_minor": 4
	}`
	os.WriteFile(nbPath, []byte(notebook), 0644)

	tool := &NotebookEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":        nbPath,
		"cell_number": 99,
	})
	if err == nil {
		t.Error("Expected error for invalid cell number")
	}
}

func TestBrowseToolMissingURL(t *testing.T) {
	tool := &BrowseTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing url")
	}
}

func TestBrowseToolSchema(t *testing.T) {
	tool := &BrowseTool{}
	if tool.Name() != "browse" {
		t.Errorf("Expected name 'browse', got '%s'", tool.Name())
	}
}

func TestAttachToolList(t *testing.T) {
	tool := &AttachTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap := result.(map[string]any)
	if resultMap["action"] != "list" {
		t.Errorf("Expected action 'list', got %v", resultMap["action"])
	}
}

func TestAttachToolInspectMissingPID(t *testing.T) {
	tool := &AttachTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "inspect",
	})
	if err == nil {
		t.Error("Expected error for missing pid")
	}
}

func TestAttachToolSchema(t *testing.T) {
	tool := &AttachTool{}
	if tool.Name() != "attach" {
		t.Errorf("Expected name 'attach', got '%s'", tool.Name())
	}
}

func TestDebugToolSchema(t *testing.T) {
	tool := &DebugTool{}
	if tool.Name() != "debug" {
		t.Errorf("Expected name 'debug', got '%s'", tool.Name())
	}
}

func TestEnvToolSchema(t *testing.T) {
	tool := &EnvTool{}
	if tool.Name() != "env" {
		t.Errorf("Expected name 'env', got '%s'", tool.Name())
	}
}

func TestSleepToolSchema(t *testing.T) {
	tool := &SleepTool{}
	if tool.Name() != "sleep" {
		t.Errorf("Expected name 'sleep', got '%s'", tool.Name())
	}
}

func TestToolSearchToolSchema(t *testing.T) {
	tool := &ToolSearchTool{}
	if tool.Name() != "tool_search" {
		t.Errorf("Expected name 'tool_search', got '%s'", tool.Name())
	}
}

func TestCacheToolSchema(t *testing.T) {
	tool := &CacheTool{}
	if tool.Name() != "cache" {
		t.Errorf("Expected name 'cache', got '%s'", tool.Name())
	}
}

func TestThinkToolSchema(t *testing.T) {
	tool := &ThinkTool{}
	if tool.Name() != "think" {
		t.Errorf("Expected name 'think', got '%s'", tool.Name())
	}
}

func TestDeepThinkToolSchema(t *testing.T) {
	tool := &DeepThinkTool{}
	if tool.Name() != "deepthink" {
		t.Errorf("Expected name 'deepthink', got '%s'", tool.Name())
	}
}

func TestForkToolSchema(t *testing.T) {
	tool := &ForkTool{}
	if tool.Name() != "fork" {
		t.Errorf("Expected name 'fork', got '%s'", tool.Name())
	}
}

func TestObserveToolSchema(t *testing.T) {
	tool := &ObserveTool{}
	if tool.Name() != "observe" {
		t.Errorf("Expected name 'observe', got '%s'", tool.Name())
	}
}

func TestLazyToolSchema(t *testing.T) {
	tool := &LazyTool{}
	if tool.Name() != "lazy" {
		t.Errorf("Expected name 'lazy', got '%s'", tool.Name())
	}
}

func TestBriefToolSchema(t *testing.T) {
	tool := &BriefTool{}
	if tool.Name() != "brief" {
		t.Errorf("Expected name 'brief', got '%s'", tool.Name())
	}
}

func TestConfigToolExecuteInvalidOperation(t *testing.T) {
	tool := &ConfigTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "invalid_op",
	})
	if err == nil {
		t.Error("Expected error for invalid operation")
	}
}

func TestConfigToolGetMissingKey(t *testing.T) {
	tool := &ConfigTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "get",
	})
	if err == nil {
		t.Error("Expected error for missing key in get operation")
	}
}

func TestConfigToolGetUnknownKey(t *testing.T) {
	tool := &ConfigTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "get",
		"key":       "nonexistent_key_xyz",
	})
	if err != nil {
		t.Fatalf("Execute should not error: %v", err)
	}
	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Error("Expected success=false for unknown key")
	}
}

func TestConfigToolList(t *testing.T) {
	tool := &ConfigTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "list",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap := result.(map[string]any)
	if _, ok := resultMap["settings"]; !ok {
		t.Error("Expected settings in result")
	}
}

func TestConfigToolSet(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &ConfigTool{configDir: tmpDir}

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "set",
		"key":       "test_key",
		"value":     "test_value",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Expected success=true for set operation")
	}
}

func TestNewConfigTool(t *testing.T) {
	tool := NewConfigTool()
	if tool == nil {
		t.Fatal("NewConfigTool returned nil")
	}
	if tool.configDir == "" {
		t.Error("configDir should not be empty")
	}
}

func TestExecuteCodeToolSchema(t *testing.T) {
	tool := &ExecuteCodeTool{}
	if tool.Name() != "execute_code" {
		t.Errorf("Expected name 'execute_code', got '%s'", tool.Name())
	}
}

func TestDockerSandboxToolSchema(t *testing.T) {
	tool := &DockerSandboxTool{}
	if tool.Name() != "docker_exec" {
		t.Errorf("Expected name 'docker_exec', got '%s'", tool.Name())
	}
}

func TestREPLToolSchema(t *testing.T) {
	tool := &REPLTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestEnterPlanModeToolSchema(t *testing.T) {
	tool := &EnterPlanModeTool{}
	if tool.Name() != "enter_plan_mode" {
		t.Errorf("Expected name 'enter_plan_mode', got '%s'", tool.Name())
	}
}

func TestExitPlanModeToolSchema(t *testing.T) {
	tool := &ExitPlanModeTool{}
	if tool.Name() != "exit_plan_mode" {
		t.Errorf("Expected name 'exit_plan_mode', got '%s'", tool.Name())
	}
}

func TestBriefToolExecute(t *testing.T) {
	tool := &BriefTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"topic":  "Go programming",
		"length": "medium",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["topic"] != "Go programming" {
		t.Errorf("Expected topic 'Go programming', got %v", m["topic"])
	}
	if m["length"] != "medium" {
		t.Errorf("Expected length 'medium', got %v", m["length"])
	}
}

func TestBriefToolMissingTopic(t *testing.T) {
	tool := &BriefTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing topic")
	}
}

func TestBriefToolDefaultLength(t *testing.T) {
	tool := &BriefTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"topic": "test",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["length"] != "short" {
		t.Errorf("Expected default length 'short', got %v", m["length"])
	}
}

func TestSleepToolExecuteZero(t *testing.T) {
	tool := &SleepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"seconds": 0.0,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["slept"] != 0.0 {
		t.Errorf("Expected slept=0, got %v", m["slept"])
	}
}

func TestToolSearchToolExecute(t *testing.T) {
	tool := &ToolSearchTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"query": "file",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if _, ok := m["matches"]; !ok {
		t.Error("Expected matches in result")
	}
}

func TestToolSearchToolMissingQuery(t *testing.T) {
	tool := &ToolSearchTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute should not error: %v", err)
	}
	m := result.(map[string]any)
	if m["query"] != "" {
		t.Error("Expected empty query")
	}
}
