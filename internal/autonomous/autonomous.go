package autonomous

import "context"

// Option configures the autonomous loop.
type Option func(*LoopConfig)

// WithMaxSteps sets the maximum number of steps per task.
func WithMaxSteps(n int) Option {
	return func(c *LoopConfig) {
		c.MaxSteps = n
	}
}

// WithVerify enables or disables build verification after each step.
func WithVerify(v bool) Option {
	return func(c *LoopConfig) {
		c.VerifyAfterStep = v
	}
}

// WithCreatePR enables or disables PR creation upon completion.
func WithCreatePR(v bool) Option {
	return func(c *LoopConfig) {
		c.CreatePR = v
	}
}

// WithOnStep registers a callback invoked on each step/phase transition.
func WithOnStep(fn func(step int, phase string)) Option {
	return func(c *LoopConfig) {
		_onStepCallback = fn
	}
}

var _onStepCallback func(step int, phase string)

// ExecuteTask is the main entry point for autonomous task execution.
func ExecuteTask(ctx context.Context, taskDescription string, workingDir string, opts ...Option) (*LoopResult, error) {
	cfg := LoopConfig{
		MaxSteps:        20,
		MaxRetries:      3,
		VerifyAfterStep: true,
		CreatePR:        true,
		WorkingDir:      workingDir,
		TaskDescription: taskDescription,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	loop := NewLoop(cfg)
	loop.onStep = _onStepCallback
	_onStepCallback = nil

	return loop.Run(ctx)
}
