package autonomous

import (
	"context"
	"testing"
)

func TestAutonomousExecuteToolSchema(t *testing.T) {
	t.Parallel()

	tool := &AutonomousExecuteTool{}
	if tool.Name() != "autonomous_execute" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "autonomous_execute")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties is not a map")
	}
	if _, ok := props["task_description"]; !ok {
		t.Error("schema missing task_description property")
	}
	required, ok := schema["required"].([]string)
	if !ok || len(required) == 0 || required[0] != "task_description" {
		t.Errorf("required = %v, want [task_description]", required)
	}
}

func TestAutonomousExecuteToolNoTaskDesc(t *testing.T) {
	t.Parallel()

	tool := &AutonomousExecuteTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for empty task_description")
	}
}

func TestAutonomousExecuteToolSuccess(t *testing.T) {
	origClient := defaultAPIClient
	defaultAPIClient = nil
	defer func() { defaultAPIClient = origClient }()

	tool := &AutonomousExecuteTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"task_description": "hello world",
		"working_dir":      ".",
		"verify":           false,
		"create_pr":        false,
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if success, _ := m["success"].(bool); !success {
		t.Errorf("success = %v, want true", m["success"])
	}
	if stepsTotal, _ := m["steps_total"].(int); stepsTotal < 1 {
		t.Errorf("steps_total = %v, want >= 1", m["steps_total"])
	}
}

func TestAutonomousCheckpointToolSchema(t *testing.T) {
	t.Parallel()

	tool := &AutonomousCheckpointTool{}
	if tool.Name() != "autonomous_checkpoint" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "autonomous_checkpoint")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties is not a map")
	}
	if _, ok := props["action"]; !ok {
		t.Error("schema missing action property")
	}
	if _, ok := props["task_id"]; !ok {
		t.Error("schema missing task_id property")
	}
}

func TestAutonomousCheckpointToolSaveLoad(t *testing.T) {
	dir := t.TempDir()

	tool := &AutonomousCheckpointTool{}

	saveResult, err := tool.Execute(context.Background(), map[string]any{
		"action":  "save",
		"task_id": "test-task-001",
		"dir":     dir,
	})
	if err != nil {
		t.Fatalf("save Execute() error: %v", err)
	}
	saveMap, ok := saveResult.(map[string]any)
	if !ok {
		t.Fatal("save result is not a map")
	}
	if saved, _ := saveMap["saved"].(bool); !saved {
		t.Errorf("saved = %v, want true", saveMap["saved"])
	}

	loadResult, err := tool.Execute(context.Background(), map[string]any{
		"action":  "load",
		"task_id": "test-task-001",
		"dir":     dir,
	})
	if err != nil {
		t.Fatalf("load Execute() error: %v", err)
	}
	loadMap, ok := loadResult.(map[string]any)
	if !ok {
		t.Fatal("load result is not a map")
	}
	if loaded, _ := loadMap["loaded"].(bool); !loaded {
		t.Errorf("loaded = %v, want true", loadMap["loaded"])
	}
}

func TestAutonomousCheckpointToolMissingArgs(t *testing.T) {
	t.Parallel()

	tool := &AutonomousCheckpointTool{}

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing action and task_id")
	}

	_, err = tool.Execute(context.Background(), map[string]any{"action": "save"})
	if err == nil {
		t.Error("expected error for missing task_id")
	}

	_, err = tool.Execute(context.Background(), map[string]any{"task_id": "x"})
	if err == nil {
		t.Error("expected error for missing action")
	}
}

func TestAutonomousCheckpointToolUnknownAction(t *testing.T) {
	t.Parallel()

	tool := &AutonomousCheckpointTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"action":  "delete",
		"task_id": "test",
	})
	if err == nil {
		t.Error("expected error for unknown action")
	}
}
