package hooks

import (
	"context"
	"testing"
)

func TestHookRegistry(t *testing.T) {
	registry := NewHookRegistry()

	hook := HookConfig{
		Name:    "test-hook",
		Event:   HookPreToolUse,
		Command: "echo 'test'",
		Enabled: true,
	}

	registry.Register(hook)

	hooks := registry.GetHooks(HookPreToolUse)
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(hooks))
	}

	if hooks[0].Name != "test-hook" {
		t.Errorf("Expected hook name 'test-hook', got '%s'", hooks[0].Name)
	}

	registry.Unregister("test-hook")
	hooks = registry.GetHooks(HookPreToolUse)
	if len(hooks) != 0 {
		t.Errorf("Expected 0 hooks after unregister, got %d", len(hooks))
	}
}

func TestHookExecutor(t *testing.T) {
	registry := NewHookRegistry()
	executor := NewHookExecutor(registry, "/tmp", "test-session")

	input := &HookInput{
		ToolName:  "test_tool",
		ToolInput: map[string]interface{}{"arg": "value"},
	}

	results := executor.Execute(context.Background(), HookPreToolUse, input)
	if results != nil {
		t.Errorf("Expected nil results for empty registry, got %v", results)
	}
}

func TestHookManager(t *testing.T) {
	manager := NewHookManager("/tmp", "test-session")

	ctx := context.Background()

	results := manager.ExecuteSessionStart(ctx)
	if results != nil {
		t.Errorf("Expected nil results for empty hooks, got %v", results)
	}

	results = manager.ExecuteSessionEnd(ctx)
	if results != nil {
		t.Errorf("Expected nil results for empty hooks, got %v", results)
	}
}

func TestHookConfigLoad(t *testing.T) {
	registry := NewHookRegistry()

	err := registry.LoadFromConfig("/nonexistent/path/hooks.json")
	if err != nil {
		t.Errorf("Expected nil error for nonexistent config, got %v", err)
	}
}

func TestHookOutputParsing(t *testing.T) {
	output := HookOutput{
		Continue:      true,
		Decision:      "allow",
		Reason:        "test reason",
		UpdatedInput:  map[string]interface{}{"key": "value"},
		SystemMessage: "test message",
		ExitCode:      0,
		Stdout:        "test output",
	}

	if !output.Continue {
		t.Error("Expected Continue to be true")
	}

	if output.Decision != "allow" {
		t.Errorf("Expected Decision 'allow', got '%s'", output.Decision)
	}

	if output.UpdatedInput["key"] != "value" {
		t.Errorf("Expected UpdatedInput['key'] = 'value', got '%v'", output.UpdatedInput["key"])
	}
}

func TestHookInput(t *testing.T) {
	input := HookInput{
		Event:       HookPreToolUse,
		SessionID:   "test-session",
		ProjectRoot: "/tmp/project",
		Timestamp:   1234567890,
		ToolName:    "bash",
		ToolInput:   map[string]interface{}{"command": "ls"},
	}

	if input.Event != HookPreToolUse {
		t.Errorf("Expected Event HookPreToolUse, got %s", input.Event)
	}

	if input.ToolName != "bash" {
		t.Errorf("Expected ToolName 'bash', got '%s'", input.ToolName)
	}
}

func TestPreToolUseBlock(t *testing.T) {
	manager := NewHookManager("/tmp", "test-session")

	blockingHook := HookConfig{
		Name:    "block-dangerous",
		Event:   HookPreToolUse,
		Command: `echo '{"decision":"block","reason":"Dangerous command blocked"}'`,
		Enabled: true,
	}
	manager.RegisterHook(blockingHook)

	ctx := context.Background()
	_, err := manager.ExecutePreToolUse(ctx, "bash", map[string]interface{}{"command": "rm -rf /"})

	if err == nil {
		t.Error("Expected error for blocked hook, got nil")
	}
}

func TestAllHookEvents(t *testing.T) {
	events := []HookEvent{
		HookPreToolUse,
		HookPostToolUse,
		HookPostToolUseFailure,
		HookPreCompact,
		HookPostCompact,
		HookSessionStart,
		HookSessionEnd,
		HookStop,
		HookStopFailure,
		HookNotification,
		HookPermissionDenied,
		HookPermissionRequest,
		HookUserPromptSubmit,
		HookSubagentStart,
		HookSubagentStop,
		HookTaskCreated,
		HookTaskCompleted,
		HookConfigChange,
		HookCwdChanged,
		HookFileChanged,
		HookSetup,
	}

	for _, event := range events {
		if event == "" {
			t.Errorf("Hook event should not be empty")
		}
	}
}
