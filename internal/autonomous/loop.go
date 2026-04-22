package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

// defaultAPIClient is the API client used for LLM-driven planning.
var defaultAPIClient *api.Client

// SetAPIClient sets the API client for LLM-driven planning.
func SetAPIClient(client *api.Client) {
	defaultAPIClient = client
}

// LoopConfig configures the autonomous agent loop.
type LoopConfig struct {
	MaxSteps        int    // max steps per task (default: 20)
	MaxRetries      int    // max retries per step (default: 3)
	VerifyAfterStep bool   // run build verify after each step (default: true)
	CreatePR        bool   // create PR when done (default: true)
	WorkingDir      string // project root
	TaskDescription string // the task to accomplish
}

// LoopState tracks the current state of the autonomous loop.
type LoopState struct {
	Step        int
	Phase       string // "planning", "implementing", "verifying", "fixing", "done", "failed"
	Plan        []Step
	CurrentStep *Step
	Errors      []string
	StartTime   time.Time
}

// Step represents a single step in the plan.
type Step struct {
	Description string
	Status      string // "pending", "in_progress", "done", "failed", "skipped"
	Files       []string
	Result      string
}

// LoopResult holds the final result of the autonomous loop.
type LoopResult struct {
	Success    bool
	StepsTotal int
	StepsDone  int
	Duration   time.Duration
	PRURL      string // if CreatePR was true
	Diff       string
	Errors     []string
}

// Loop orchestrates autonomous task execution.
type Loop struct {
	config LoopConfig
	state  LoopState
	onStep func(step int, phase string)
	cancel context.CancelFunc
}

// NewLoop creates a new autonomous loop with the given configuration.
func NewLoop(config LoopConfig) *Loop {
	if config.MaxSteps <= 0 {
		config.MaxSteps = 20
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	return &Loop{
		config: config,
		state: LoopState{
			Phase:     "planning",
			Errors:    []string{},
			StartTime: time.Now(),
		},
	}
}

// GetState returns a copy of the current loop state.
func (l *Loop) GetState() LoopState {
	return l.state
}

// Cancel signals the loop to stop.
func (l *Loop) Cancel() {
	if l.cancel != nil {
		l.cancel()
	}
}

// Run executes the autonomous loop: plan → implement → verify → fix → done.
func (l *Loop) Run(ctx context.Context) (*LoopResult, error) {
	ctx, l.cancel = context.WithCancel(ctx)
	defer l.cancel()

	result := &LoopResult{
		Errors: []string{},
	}

	// Phase 1: Planning
	l.setPhase("planning")
	if err := l.plan(ctx); err != nil {
		return nil, fmt.Errorf("planning failed: %w", err)
	}
	result.StepsTotal = len(l.state.Plan)

	// Phase 2-4: Implement → Verify → Fix loop
	for i := range l.state.Plan {
		select {
		case <-ctx.Done():
			l.state.Errors = append(l.state.Errors, ctx.Err().Error())
			result.Errors = l.state.Errors
			return result, ctx.Err()
		default:
		}

		if i >= l.config.MaxSteps {
			l.state.Errors = append(l.state.Errors, "max steps exceeded")
			result.Errors = l.state.Errors
			return result, fmt.Errorf("max steps (%d) exceeded", l.config.MaxSteps)
		}

		step := &l.state.Plan[i]
		l.state.Step = i + 1
		l.state.CurrentStep = step

		retries := 0
		for retries <= l.config.MaxRetries {
			// Implement
			l.setPhase("implementing")
			step.Status = "in_progress"
			l.notify(i+1, "implementing")

			stepResult, implErr := l.implementStep(ctx, step)
			if implErr != nil {
				step.Result = fmt.Sprintf("implementation error: %v", implErr)
				step.Status = "failed"
			} else {
				step.Result = stepResult
				step.Status = "done"
			}
			l.notify(i+1, "implemented")

			// Verify
			if l.config.VerifyAfterStep {
				l.setPhase("verifying")
				l.notify(i+1, "verifying")

				if err := l.verify(ctx); err != nil {
					step.Status = "failed"
					l.state.Errors = append(l.state.Errors, fmt.Sprintf("step %d verify failed (attempt %d): %v", i+1, retries+1, err))

					retries++
					if retries > l.config.MaxRetries {
						l.setPhase("failed")
						l.notify(i+1, "failed")
						result.Errors = l.state.Errors
						return result, fmt.Errorf("step %d failed after %d retries: %w", i+1, l.config.MaxRetries, err)
					}

					// Fix
					l.setPhase("fixing")
					l.notify(i+1, "fixing")
					step.Status = "pending" // reset for retry
					continue
				}
			}

			break // step succeeded
		}

		result.StepsDone++
	}

	// Phase 5: Done
	l.setPhase("done")
	l.notify(result.StepsDone, "done")

	// PR creation placeholder
	if l.config.CreatePR {
		l.state.Phase = "creating_pr"
		l.notify(result.StepsDone, "creating_pr")
		// PR creation would be done by the LLM caller
	}

	result.Success = true
	result.Duration = time.Since(l.state.StartTime)
	result.Errors = l.state.Errors
	return result, nil
}

func (l *Loop) plan(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if defaultAPIClient == nil {
		l.state.Plan = []Step{
			{Description: l.config.TaskDescription, Status: "pending", Files: []string{}},
		}
		return nil
	}

	systemPrompt := `You are a task decomposition expert. Given a coding task, break it down into concrete, sequential steps.
Each step should be specific enough to execute with file operations or shell commands.
Return a JSON array of steps, where each step has:
- "description": what to do
- "files": list of files that will likely be modified (empty array if unknown)

Return ONLY the JSON array, no other text.`

	messages := []api.MessageParam{
		{Role: "user", Content: fmt.Sprintf("Task: %s\n\nWorking directory: %s\n\nDecompose this task into steps:", l.config.TaskDescription, l.config.WorkingDir)},
	}

	resp, err := defaultAPIClient.CreateMessageCtx(ctx, messages, systemPrompt)
	if err != nil {
		l.state.Plan = []Step{
			{Description: l.config.TaskDescription, Status: "pending", Files: []string{}},
		}
		return nil
	}

	if len(resp.Content) == 0 || resp.Content[0].Text == "" {
		l.state.Plan = []Step{
			{Description: l.config.TaskDescription, Status: "pending", Files: []string{}},
		}
		return nil
	}

	text := resp.Content[0].Text
	text = strings.TrimPrefix(text, "```json\n")
	text = strings.TrimPrefix(text, "```\n")
	text = strings.TrimSuffix(text, "\n```")
	text = strings.TrimSpace(text)

	var planSteps []struct {
		Description string   `json:"description"`
		Files       []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(text), &planSteps); err != nil || len(planSteps) == 0 {
		l.state.Plan = []Step{
			{Description: l.config.TaskDescription, Status: "pending", Files: []string{}},
		}
		return nil
	}

	l.state.Plan = make([]Step, len(planSteps))
	for i, ps := range planSteps {
		l.state.Plan[i] = Step{
			Description: ps.Description,
			Status:      "pending",
			Files:       ps.Files,
		}
		if l.state.Plan[i].Files == nil {
			l.state.Plan[i].Files = []string{}
		}
	}
	return nil
}

func (l *Loop) implementStep(ctx context.Context, step *Step) (string, error) {
	if defaultAPIClient == nil {
		return "skipped: no API client configured", nil
	}

	systemPrompt := fmt.Sprintf(`You are a coding assistant executing step %q of a larger task.
Working directory: %s
Files that may be modified: %v

Execute this step by calling the appropriate tools. Respond with a brief summary of what you did.`,
		step.Description, l.config.WorkingDir, step.Files)

	var prevContext []string
	for _, s := range l.state.Plan {
		if s.Status == "done" && s.Result != "" {
			prevContext = append(prevContext, fmt.Sprintf("- %s: %s", s.Description, s.Result))
		}
	}
	contextStr := ""
	if len(prevContext) > 0 {
		contextStr = fmt.Sprintf("\n\nPrevious steps completed:\n%s", strings.Join(prevContext, "\n"))
	}

	messages := []api.MessageParam{
		{Role: "user", Content: fmt.Sprintf("Execute this step: %s%s", step.Description, contextStr)},
	}

	resp, err := defaultAPIClient.CreateMessageCtx(ctx, messages, systemPrompt)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	if len(resp.Content) > 0 {
		return resp.Content[0].Text, nil
	}
	return "no response from LLM", nil
}

func (l *Loop) verify(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = l.config.WorkingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %s: %w", string(output), err)
	}
	return nil
}

func (l *Loop) setPhase(phase string) {
	l.state.Phase = phase
}

func (l *Loop) notify(step int, phase string) {
	if l.onStep != nil {
		l.onStep(step, phase)
	}
}
