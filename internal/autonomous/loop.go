package autonomous

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

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

			step.Result = "implementation placeholder"
			step.Status = "done"
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

	// Placeholder: create a single step from the task description.
	// In production, this would call an LLM to decompose the task.
	l.state.Plan = []Step{
		{
			Description: l.config.TaskDescription,
			Status:      "pending",
			Files:       []string{},
		},
	}
	return nil
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
