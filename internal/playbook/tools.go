package playbook

import (
	"context"
	"fmt"

	"github.com/instructkr/smartclaw/internal/tools"
	"gopkg.in/yaml.v3"
)

// PlaybookExecuteTool runs a playbook by name with given parameters.
type PlaybookExecuteTool struct {
	manager *Manager
}

func (t *PlaybookExecuteTool) Name() string { return "playbook_execute" }

func (t *PlaybookExecuteTool) Description() string {
	return "Execute a parameterized playbook workflow. Playbooks are reusable workflow templates with conditional steps, approval gates, and retry logic."
}

func (t *PlaybookExecuteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Name of the playbook to execute",
			},
			"params": map[string]any{
				"type":                "object",
				"description":         "Parameters to pass to the playbook",
				"additionalProperties": map[string]any{"type": "string"},
			},
		},
		"required": []string{"name"},
	}
}

func (t *PlaybookExecuteTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if t.manager == nil {
		return nil, fmt.Errorf("playbook: manager not initialized")
	}
	name, _ := input["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("playbook: name is required")
	}
	params := make(map[string]string)
	if p, ok := input["params"].(map[string]any); ok {
		for k, v := range p {
			if s, ok := v.(string); ok {
				params[k] = s
			}
		}
	}

	execCtx, err := t.manager.Execute(ctx, name, params, nil)
	if err != nil {
		return map[string]any{"status": "failed", "error": err.Error()}, nil
	}
	return map[string]any{
		"status":       execCtx.Status,
		"current_step": execCtx.CurrentStep,
		"step_results": execCtx.StepResults,
	}, nil
}

// PlaybookListTool lists available playbooks.
type PlaybookListTool struct {
	manager *Manager
}

func (t *PlaybookListTool) Name() string { return "playbook_list" }

func (t *PlaybookListTool) Description() string {
	return "List all available playbooks, including built-in and user-created ones."
}

func (t *PlaybookListTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *PlaybookListTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if t.manager == nil {
		return nil, fmt.Errorf("playbook: manager not initialized")
	}
	playbooks, err := t.manager.List()
	if err != nil {
		return nil, fmt.Errorf("playbook list: %w", err)
	}
	var result []map[string]string
	for _, pb := range playbooks {
		result = append(result, map[string]string{
			"name":        pb.Name,
			"description": pb.Description,
			"version":     pb.Version,
		})
	}
	return map[string]any{"playbooks": result}, nil
}

// PlaybookCreateTool creates a new playbook from a YAML definition.
type PlaybookCreateTool struct {
	manager *Manager
}

func (t *PlaybookCreateTool) Name() string { return "playbook_create" }

func (t *PlaybookCreateTool) Description() string {
	return "Create a new playbook from a YAML definition. The playbook will be saved for later execution."
}

func (t *PlaybookCreateTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"yaml_content": map[string]any{
				"type":        "string",
				"description": "YAML definition of the playbook",
			},
		},
		"required": []string{"yaml_content"},
	}
}

func (t *PlaybookCreateTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if t.manager == nil {
		return nil, fmt.Errorf("playbook: manager not initialized")
	}
	yamlContent, _ := input["yaml_content"].(string)
	if yamlContent == "" {
		return nil, fmt.Errorf("playbook: yaml_content is required")
	}

	var pb Playbook
	if err := yaml.Unmarshal([]byte(yamlContent), &pb); err != nil {
		return nil, fmt.Errorf("playbook: invalid YAML: %w", err)
	}
	if err := t.manager.Validate(&pb); err != nil {
		return nil, fmt.Errorf("playbook: validation failed: %w", err)
	}
	if err := t.manager.Save(&pb); err != nil {
		return nil, fmt.Errorf("playbook: save failed: %w", err)
	}
	return map[string]any{"created": true, "name": pb.Name}, nil
}

var defaultManager *Manager

// SetManager sets the default playbook manager.
func SetManager(m *Manager) {
	defaultManager = m
}

// DefaultManager returns the default playbook manager.
func DefaultManager() *Manager {
	return defaultManager
}

// RegisterAllTools registers all playbook tools with the global registry.
func RegisterAllTools() {
	m := defaultManager
	if m == nil {
		m = &Manager{}
	}
	tools.Register(&PlaybookExecuteTool{manager: m})
	tools.Register(&PlaybookListTool{manager: m})
	tools.Register(&PlaybookCreateTool{manager: m})
}
