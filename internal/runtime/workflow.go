package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/instructkr/smartclaw/internal/learning"
)

type WorkflowResult struct {
	WorkflowName string
	StepResults  map[string]interface{}
	Completed    int
	Failed       int
	Skipped      int
}

type WorkflowExecutor struct {
	engine *QueryEngine
}

func NewWorkflowExecutor(engine *QueryEngine) *WorkflowExecutor {
	return &WorkflowExecutor{engine: engine}
}

func (we *WorkflowExecutor) Execute(ctx context.Context, workflow *learning.Workflow) (*WorkflowResult, error) {
	if workflow == nil || len(workflow.Steps) == 0 {
		return nil, fmt.Errorf("workflow: empty workflow")
	}

	result := &WorkflowResult{
		WorkflowName: workflow.Name,
		StepResults:  make(map[string]interface{}),
	}

	completed := make(map[string]bool)

	for {
		progress := false
		for _, step := range workflow.Steps {
			if completed[step.Name] {
				continue
			}

			if !we.depsSatisfied(step.Name, workflow.Dependencies, completed) {
				continue
			}

			slog.Info("workflow: executing step", "workflow", workflow.Name, "step", step.Name, "skill", step.SkillName)

			stepResult, err := we.executeStep(ctx, step)
			if err != nil {
				slog.Warn("workflow: step failed", "step", step.Name, "error", err)
				result.Failed++
				result.StepResults[step.Name] = map[string]interface{}{
					"error": err.Error(),
				}
				completed[step.Name] = true
				progress = true
				continue
			}

			result.StepResults[step.Name] = stepResult
			result.Completed++
			completed[step.Name] = true
			progress = true
		}

		if !progress {
			break
		}

		if result.Completed+result.Failed >= len(workflow.Steps) {
			break
		}
	}

	result.Skipped = len(workflow.Steps) - result.Completed - result.Failed

	return result, nil
}

func (we *WorkflowExecutor) depsSatisfied(stepName string, deps map[string][]string, completed map[string]bool) bool {
	stepDeps, ok := deps[stepName]
	if !ok || len(stepDeps) == 0 {
		return true
	}

	for _, dep := range stepDeps {
		if !completed[dep] {
			return false
		}
	}
	return true
}

func (we *WorkflowExecutor) executeStep(ctx context.Context, step learning.WorkflowStep) (interface{}, error) {
	input := fmt.Sprintf("Execute skill '%s'", step.SkillName)

	for key, hint := range step.InputHints {
		input += fmt.Sprintf("\n%s: %s", key, hint)
	}

	queryResult, err := we.engine.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("step %s: %w", step.Name, err)
	}

	return queryResult.Message.Content, nil
}
