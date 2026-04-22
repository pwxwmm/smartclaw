package playbook

import (
	"context"
	"testing"
)

func TestPlaybookExecuteToolSchema(t *testing.T) {
	t.Parallel()

	tool := &PlaybookExecuteTool{}
	if tool.Name() != "playbook_execute" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "playbook_execute")
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
	if _, ok := props["name"]; !ok {
		t.Error("schema missing name property")
	}
}

func TestPlaybookExecuteToolNoManager(t *testing.T) {
	t.Parallel()

	tool := &PlaybookExecuteTool{manager: nil}
	_, err := tool.Execute(context.Background(), map[string]any{"name": "test"})
	if err == nil {
		t.Error("expected error with nil manager")
	}
}

func TestPlaybookExecuteToolNotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	tool := &PlaybookExecuteTool{manager: m}

	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "nonexistent",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	rmap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if rmap["status"] != "failed" {
		t.Errorf("status = %v, want failed", rmap["status"])
	}
}

func TestPlaybookListToolSchema(t *testing.T) {
	t.Parallel()

	tool := &PlaybookListTool{}
	if tool.Name() != "playbook_list" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "playbook_list")
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
}

func TestPlaybookListToolEmpty(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	tool := &PlaybookListTool{manager: m}

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	rmap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	playbooks, _ := rmap["playbooks"].([]map[string]string)
	if len(playbooks) != 0 {
		t.Errorf("playbooks count = %d, want 0", len(playbooks))
	}
}

func TestPlaybookCreateToolSchema(t *testing.T) {
	t.Parallel()

	tool := &PlaybookCreateTool{}
	if tool.Name() != "playbook_create" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "playbook_create")
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
	if _, ok := props["yaml_content"]; !ok {
		t.Error("schema missing yaml_content property")
	}
}

func TestPlaybookCreateToolInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	tool := &PlaybookCreateTool{manager: m}

	_, err := tool.Execute(context.Background(), map[string]any{
		"yaml_content": "{{invalid yaml:::",
	})
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestPlaybookCreateToolSuccess(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	tool := &PlaybookCreateTool{manager: m}

	yaml := `
name: created-pb
description: a created playbook
steps:
  - id: s1
    action: run_command
    command: "true"
`
	result, err := tool.Execute(context.Background(), map[string]any{
		"yaml_content": yaml,
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	rmap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if created, _ := rmap["created"].(bool); !created {
		t.Errorf("created = %v, want true", rmap["created"])
	}
	if rmap["name"] != "created-pb" {
		t.Errorf("name = %v, want created-pb", rmap["name"])
	}
}

func TestPlaybookListToolNoManager(t *testing.T) {
	t.Parallel()

	tool := &PlaybookListTool{manager: nil}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error with nil manager")
	}
}

func TestPlaybookCreateToolNoManager(t *testing.T) {
	t.Parallel()

	tool := &PlaybookCreateTool{manager: nil}
	_, err := tool.Execute(context.Background(), map[string]any{"yaml_content": "x"})
	if err == nil {
		t.Error("expected error with nil manager")
	}
}
