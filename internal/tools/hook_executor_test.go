package tools

import (
	"context"
	"testing"

	"github.com/instructkr/smartclaw/internal/hooks"
)

type MockTool struct {
	name        string
	description string
	executed    bool
	result      any
	err         error
}

func (t *MockTool) Name() string                        { return t.name }
func (t *MockTool) Description() string                 { return t.description }
func (t *MockTool) InputSchema() map[string]any { return nil }
func (t *MockTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	t.executed = true
	return t.result, t.err
}

func TestHookAwareExecutorBasic(t *testing.T) {
	registry := NewRegistry()
	mockTool := &MockTool{name: "mock", description: "mock tool", result: "ok"}
	registry.Register(mockTool)

	executor := NewHookAwareExecutor(registry, nil)

	result, err := executor.ExecuteWithHooks(context.Background(), "mock", nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "ok" {
		t.Errorf("Expected result 'ok', got %v", result)
	}

	if !mockTool.executed {
		t.Error("Expected tool to be executed")
	}
}

func TestHookAwareExecutorWithHooks(t *testing.T) {
	registry := NewRegistry()
	mockTool := &MockTool{name: "mock", description: "mock tool", result: "ok"}
	registry.Register(mockTool)

	hookManager := hooks.NewHookManager("/tmp", "test-session")
	executor := NewHookAwareExecutor(registry, hookManager)

	result, err := executor.ExecuteWithHooks(context.Background(), "mock", map[string]any{})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "ok" {
		t.Errorf("Expected result 'ok', got %v", result)
	}
}

func TestHookAwareExecutorUnknownTool(t *testing.T) {
	registry := NewRegistry()
	executor := NewHookAwareExecutor(registry, nil)

	_, err := executor.ExecuteWithHooks(context.Background(), "unknown", nil)
	if err == nil {
		t.Error("Expected error for unknown tool, got nil")
	}
}

func TestHookAwareExecutorBypassHooks(t *testing.T) {
	registry := NewRegistry()
	mockTool := &MockTool{name: "mock", description: "mock tool", result: "ok"}
	registry.Register(mockTool)

	hookManager := hooks.NewHookManager("/tmp", "test-session")
	executor := NewHookAwareExecutor(registry, hookManager)

	result, err := executor.ExecuteWithoutHooks(context.Background(), "mock", nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "ok" {
		t.Errorf("Expected result 'ok', got %v", result)
	}
}

func TestHookAwareExecutorListTools(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&MockTool{name: "tool1", description: "first tool"})
	registry.Register(&MockTool{name: "tool2", description: "second tool"})

	executor := NewHookAwareExecutor(registry, nil)

	tools := executor.ListTools()
	if len(tools) < 2 {
		t.Errorf("Expected at least 2 tools, got %d", len(tools))
	}
}

func TestInitHookAwareExecutor(t *testing.T) {
	InitHookAwareExecutor(nil)

	executor := GetHookAwareExecutor()
	if executor == nil {
		t.Error("Expected non-nil executor")
	}
}

func TestExecuteWithGlobalHooks(t *testing.T) {
	InitHookAwareExecutor(nil)

	_, err := ExecuteWithGlobalHooks(context.Background(), "read_file", map[string]any{"path": "/nonexistent"})
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}
