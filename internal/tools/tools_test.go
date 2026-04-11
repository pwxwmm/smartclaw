package tools

import (
	"context"
	"testing"
)

func TestBashTool(t *testing.T) {
	tool := &BashTool{}

	if tool.Name() != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestBashToolExecute(t *testing.T) {
	tool := NewBashTool("")

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hello",
	})

	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	resultMap, ok := result.(*BashToolResult)
	if !ok {
		t.Errorf("Result should be a BashToolResult, got %T", result)
	}

	if resultMap.Stdout == "" {
		t.Error("Expected some stdout output")
	}

	t.Logf("Exit code: %d, Stdout: %q", resultMap.ExitCode, resultMap.Stdout)
}

func TestReadFileTool(t *testing.T) {
	tool := &ReadFileTool{}

	if tool.Name() != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", tool.Name())
	}
}

func TestWriteFileTool(t *testing.T) {
	tool := &WriteFileTool{}

	if tool.Name() != "write_file" {
		t.Errorf("Expected name 'write_file', got '%s'", tool.Name())
	}
}

func TestEditFileTool(t *testing.T) {
	tool := &EditFileTool{}

	if tool.Name() != "edit_file" {
		t.Errorf("Expected name 'edit_file', got '%s'", tool.Name())
	}
}

func TestGlobTool(t *testing.T) {
	tool := &GlobTool{}

	if tool.Name() != "glob" {
		t.Errorf("Expected name 'glob', got '%s'", tool.Name())
	}
}

func TestGrepTool(t *testing.T) {
	tool := &GrepTool{}

	if tool.Name() != "grep" {
		t.Errorf("Expected name 'grep', got '%s'", tool.Name())
	}
}

func TestToolRegistry(t *testing.T) {
	registry := NewRegistry()

	tool := &BashTool{}
	registry.Register(tool)

	if registry.Get("bash") == nil {
		t.Error("Tool should be registered")
	}

	if len(registry.All()) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(registry.All()))
	}

	if len(registry.Names()) != 1 {
		t.Errorf("Expected 1 name, got %d", len(registry.Names()))
	}
}

func TestDefaultRegistry(t *testing.T) {
	registry := GetRegistry()

	if registry == nil {
		t.Error("Default registry should not be nil")
	}

	tools := registry.All()
	if len(tools) == 0 {
		t.Error("Default registry should have tools")
	}
}

func TestError(t *testing.T) {
	err := ErrRequiredField("test")

	if err.Code != "REQUIRED_FIELD" {
		t.Errorf("Expected code 'REQUIRED_FIELD', got '%s'", err.Code)
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}
