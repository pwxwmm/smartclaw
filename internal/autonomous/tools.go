package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

// AutonomousExecuteTool runs an autonomous task loop.
type AutonomousExecuteTool struct{}

func (t *AutonomousExecuteTool) Name() string { return "autonomous_execute" }

func (t *AutonomousExecuteTool) Description() string {
	return "Execute an autonomous task that plans, implements, verifies, and fixes code changes automatically. The agent loops until the task is complete or max steps are reached."
}

func (t *AutonomousExecuteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_description": map[string]any{
				"type":        "string",
				"description": "Description of the task to execute autonomously",
			},
			"working_dir": map[string]any{
				"type":        "string",
				"description": "Project root directory (defaults to current directory)",
			},
			"max_steps": map[string]any{
				"type":        "integer",
				"description": "Maximum number of steps (default: 20)",
				"default":     20,
			},
			"verify": map[string]any{
				"type":        "boolean",
				"description": "Run build verification after each step (default: true)",
				"default":     true,
			},
			"create_pr": map[string]any{
				"type":        "boolean",
				"description": "Create a PR when done (default: false)",
				"default":     false,
			},
		},
		"required": []string{"task_description"},
	}
}

func (t *AutonomousExecuteTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	taskDesc, _ := input["task_description"].(string)
	if taskDesc == "" {
		return nil, fmt.Errorf("autonomous: task_description is required")
	}

	workingDir, _ := input["working_dir"].(string)
	if workingDir == "" {
		workingDir = "."
	}

	maxSteps := 20
	if f, ok := input["max_steps"].(float64); ok {
		maxSteps = int(f)
	}

	verify := true
	if v, ok := input["verify"].(bool); ok {
		verify = v
	}

	createPR := false
	if v, ok := input["create_pr"].(bool); ok {
		createPR = v
	}

	opts := []Option{
		WithMaxSteps(maxSteps),
		WithVerify(verify),
		WithCreatePR(createPR),
	}

	result, err := ExecuteTask(ctx, taskDesc, workingDir, opts...)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, nil
	}

	return map[string]any{
		"success":     result.Success,
		"steps_total": result.StepsTotal,
		"steps_done":  result.StepsDone,
		"duration":    result.Duration.String(),
		"errors":      result.Errors,
	}, nil
}

// AutonomousCheckpointTool saves or loads a checkpoint for an autonomous loop.
type AutonomousCheckpointTool struct{}

func (t *AutonomousCheckpointTool) Name() string { return "autonomous_checkpoint" }

func (t *AutonomousCheckpointTool) Description() string {
	return "Save or load a checkpoint for an autonomous task loop, enabling pause/resume of long-running tasks."
}

func (t *AutonomousCheckpointTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action to perform: 'save' or 'load'",
				"enum":        []string{"save", "load"},
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "Unique task identifier for the checkpoint",
			},
			"dir": map[string]any{
				"type":        "string",
				"description": "Directory to store checkpoints (default: .smartclaw/checkpoints)",
			},
		},
		"required": []string{"action", "task_id"},
	}
}

func (t *AutonomousCheckpointTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	action, _ := input["action"].(string)
	taskID, _ := input["task_id"].(string)
	dir, _ := input["dir"].(string)

	if action == "" || taskID == "" {
		return nil, fmt.Errorf("autonomous_checkpoint: action and task_id are required")
	}

	if dir == "" {
		dir = ".smartclaw/checkpoints"
	}

	switch action {
	case "save":
		// Build a checkpoint file using the same format as SaveCheckpoint.
		// SaveCheckpoint requires a *Loop, so we construct the file directly.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create checkpoint dir: %w", err)
		}
		cp := Checkpoint{
			ID: taskID,
			State: LoopState{
				Phase:     "checkpointed",
				Errors:    []string{},
				StartTime: time.Now(),
			},
			CreatedAt: time.Now(),
		}
		data, err := json.MarshalIndent(cp, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal checkpoint: %w", err)
		}
		path := filepath.Join(dir, fmt.Sprintf("autonomous-%s.json", taskID))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, fmt.Errorf("write checkpoint: %w", err)
		}
		return map[string]any{"saved": true, "task_id": taskID}, nil
	case "load":
		state, err := LoadCheckpoint(dir, taskID)
		if err != nil {
			return nil, fmt.Errorf("load checkpoint: %w", err)
		}
		return map[string]any{"loaded": true, "task_id": taskID, "phase": state.Phase, "step": state.Step}, nil
	default:
		return nil, fmt.Errorf("autonomous_checkpoint: unknown action %q", action)
	}
}

// RegisterAllTools registers all autonomous tools with the global registry.
func RegisterAllTools() {
	tools.Register(&AutonomousExecuteTool{})
	tools.Register(&AutonomousCheckpointTool{})
}
