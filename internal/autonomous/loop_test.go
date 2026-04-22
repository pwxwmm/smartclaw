package autonomous

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewLoop(t *testing.T) {
	t.Parallel()

	l := NewLoop(LoopConfig{})
	if l.config.MaxSteps != 20 {
		t.Errorf("default MaxSteps = %d, want 20", l.config.MaxSteps)
	}
	if l.config.MaxRetries != 3 {
		t.Errorf("default MaxRetries = %d, want 3", l.config.MaxRetries)
	}
	state := l.GetState()
	if state.Phase != "planning" {
		t.Errorf("initial phase = %q, want %q", state.Phase, "planning")
	}
}

func TestNewLoopCustomConfig(t *testing.T) {
	t.Parallel()

	l := NewLoop(LoopConfig{MaxSteps: 5, MaxRetries: 1, TaskDescription: "test task"})
	if l.config.MaxSteps != 5 {
		t.Errorf("MaxSteps = %d, want 5", l.config.MaxSteps)
	}
	if l.config.MaxRetries != 1 {
		t.Errorf("MaxRetries = %d, want 1", l.config.MaxRetries)
	}
	if l.config.TaskDescription != "test task" {
		t.Errorf("TaskDescription = %q, want %q", l.config.TaskDescription, "test task")
	}
}

func TestLoopCancel(t *testing.T) {
	l := NewLoop(LoopConfig{
		TaskDescription: "long task",
		MaxSteps:        100,
		VerifyAfterStep: false,
	})

	// Run in background and cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	go func() {
		result, err := l.Run(ctx)
		_ = result
		_ = err
		close(done)
	}()

	// Cancel after a short delay
	time.Sleep(50 * time.Millisecond)
	l.Cancel()

	select {
	case <-done:
		// Loop stopped as expected
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not stop after Cancel()")
	}
}

func TestLoopPlanFallback(t *testing.T) {
	t.Parallel()

	// Ensure no API client so plan falls back
	origClient := defaultAPIClient
	defaultAPIClient = nil
	defer func() { defaultAPIClient = origClient }()

	l := NewLoop(LoopConfig{TaskDescription: "do something"})
	ctx := context.Background()

	if err := l.plan(ctx); err != nil {
		t.Fatalf("plan() error: %v", err)
	}

	if len(l.state.Plan) != 1 {
		t.Fatalf("len(Plan) = %d, want 1", len(l.state.Plan))
	}
	if l.state.Plan[0].Description != "do something" {
		t.Errorf("Plan[0].Description = %q, want %q", l.state.Plan[0].Description, "do something")
	}
	if l.state.Plan[0].Status != "pending" {
		t.Errorf("Plan[0].Status = %q, want %q", l.state.Plan[0].Status, "pending")
	}
}

func TestLoopRunSuccess(t *testing.T) {
	// Ensure no API client so plan falls back to single step and implementStep returns "skipped"
	origClient := defaultAPIClient
	defaultAPIClient = nil
	defer func() { defaultAPIClient = origClient }()

	l := NewLoop(LoopConfig{
		TaskDescription: "simple task",
		MaxSteps:        5,
		VerifyAfterStep: false,
		CreatePR:        false,
	})

	result, err := l.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !result.Success {
		t.Error("result.Success = false, want true")
	}
	if result.StepsTotal != 1 {
		t.Errorf("StepsTotal = %d, want 1", result.StepsTotal)
	}
	if result.StepsDone != 1 {
		t.Errorf("StepsDone = %d, want 1", result.StepsDone)
	}
	if result.Duration == 0 {
		t.Error("Duration = 0, want non-zero")
	}
}

func TestLoopRunWithVerify(t *testing.T) {
	// Use workingDir = "." which should compile the project
	origClient := defaultAPIClient
	defaultAPIClient = nil
	defer func() { defaultAPIClient = origClient }()

	l := NewLoop(LoopConfig{
		TaskDescription: "verify task",
		WorkingDir:      ".",
		MaxSteps:        5,
		VerifyAfterStep: true,
		CreatePR:        false,
	})

	result, err := l.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !result.Success {
		t.Errorf("result.Success = false, want true; errors: %v", result.Errors)
	}
}

func TestLoopGetState(t *testing.T) {
	t.Parallel()

	l := NewLoop(LoopConfig{TaskDescription: "state test"})
	state := l.GetState()

	if state.Phase != "planning" {
		t.Errorf("Phase = %q, want %q", state.Phase, "planning")
	}
	if state.Step != 0 {
		t.Errorf("Step = %d, want 0", state.Step)
	}
	if len(state.Errors) != 0 {
		t.Errorf("Errors = %v, want empty", state.Errors)
	}
	if state.StartTime.IsZero() {
		t.Error("StartTime is zero, want non-zero")
	}
}

func TestCheckpointSaveLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a loop and save its checkpoint
	l := NewLoop(LoopConfig{
		TaskDescription: "checkpoint test",
		MaxSteps:        10,
	})

	if err := SaveCheckpoint(dir, l); err != nil {
		t.Fatalf("SaveCheckpoint() error: %v", err)
	}

	// Load the checkpoint back
	state := l.GetState()
	loaded, err := LoadCheckpoint(dir, stateID(state))
	if err != nil {
		t.Fatalf("LoadCheckpoint() error: %v", err)
	}

	if loaded.Phase != state.Phase {
		t.Errorf("loaded Phase = %q, want %q", loaded.Phase, state.Phase)
	}
	if loaded.Step != state.Step {
		t.Errorf("loaded Step = %d, want %d", loaded.Step, state.Step)
	}
}

func TestCheckpointListAndDelete(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	l1 := NewLoop(LoopConfig{TaskDescription: "first"})
	if err := SaveCheckpoint(dir, l1); err != nil {
		t.Fatalf("SaveCheckpoint(1) error: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // ensure different timestamps

	l2 := NewLoop(LoopConfig{TaskDescription: "second"})
	if err := SaveCheckpoint(dir, l2); err != nil {
		t.Fatalf("SaveCheckpoint(2) error: %v", err)
	}

	checkpoints, err := ListCheckpoints(dir)
	if err != nil {
		t.Fatalf("ListCheckpoints() error: %v", err)
	}
	if len(checkpoints) != 2 {
		t.Fatalf("len(checkpoints) = %d, want 2", len(checkpoints))
	}

	// Should be sorted newest first
	if !checkpoints[0].CreatedAt.After(checkpoints[1].CreatedAt) {
		t.Error("checkpoints not sorted newest first")
	}

	// Delete first checkpoint
	state1 := l1.GetState()
	if err := DeleteCheckpoint(dir, stateID(state1)); err != nil {
		t.Fatalf("DeleteCheckpoint() error: %v", err)
	}

	checkpoints, err = ListCheckpoints(dir)
	if err != nil {
		t.Fatalf("ListCheckpoints() after delete error: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("len(checkpoints) after delete = %d, want 1", len(checkpoints))
	}
}

func TestListCheckpointsEmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	checkpoints, err := ListCheckpoints(dir)
	if err != nil {
		t.Fatalf("ListCheckpoints() on empty dir error: %v", err)
	}
	if len(checkpoints) != 0 {
		t.Errorf("len(checkpoints) = %d, want 0", len(checkpoints))
	}
}

func TestLoadCheckpointNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := LoadCheckpoint(dir, "nonexistent")
	if err == nil {
		t.Error("LoadCheckpoint() with nonexistent ID should return error")
	}
}

// helper to generate checkpoint ID matching SaveCheckpoint format
func stateID(s LoopState) string {
	return fmt.Sprintf("%d", s.StartTime.UnixMilli())
}
