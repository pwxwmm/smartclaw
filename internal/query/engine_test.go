package query

import (
	"context"
	"errors"
	"testing"
)

func TestNewTokenBudget(t *testing.T) {
	b := NewTokenBudget(1000)
	if b.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %d, want 1000", b.MaxTokens)
	}
	if b.UsedTokens != 0 {
		t.Errorf("UsedTokens = %d, want 0", b.UsedTokens)
	}
	if b.ReservedTokens != 0 {
		t.Errorf("ReservedTokens = %d, want 0", b.ReservedTokens)
	}
}

func TestTokenBudget_Allocate_Success(t *testing.T) {
	b := NewTokenBudget(100)
	if !b.Allocate(40) {
		t.Fatal("Allocate(40) should succeed")
	}
	if b.GetUsed() != 40 {
		t.Errorf("GetUsed() = %d, want 40", b.GetUsed())
	}
	if b.Available() != 60 {
		t.Errorf("Available() = %d, want 60", b.Available())
	}
}

func TestTokenBudget_Allocate_Fail(t *testing.T) {
	b := NewTokenBudget(100)
	b.Allocate(80)
	if b.Allocate(30) {
		t.Fatal("Allocate(30) should fail when only 20 available")
	}
	if b.GetUsed() != 80 {
		t.Errorf("GetUsed() = %d, want 80", b.GetUsed())
	}
}

func TestTokenBudget_Reserve_Success(t *testing.T) {
	b := NewTokenBudget(100)
	if !b.Reserve(30) {
		t.Fatal("Reserve(30) should succeed")
	}
	if b.GetUsed() != 0 {
		t.Errorf("GetUsed() = %d, want 0 after Reserve only", b.GetUsed())
	}
	if b.Available() != 70 {
		t.Errorf("Available() = %d, want 70", b.Available())
	}
}

func TestTokenBudget_Reserve_Fail(t *testing.T) {
	b := NewTokenBudget(100)
	b.Allocate(50)
	if b.Reserve(60) {
		t.Fatal("Reserve(60) should fail, only 50 available")
	}
}

func TestTokenBudget_Commit(t *testing.T) {
	b := NewTokenBudget(100)
	b.Reserve(30)
	b.Commit(20)
	if b.GetUsed() != 20 {
		t.Errorf("GetUsed() = %d, want 20 after Commit", b.GetUsed())
	}
	if b.Available() != 70 { // 100 - 20 used - 10 remaining reserved
		t.Errorf("Available() = %d, want 70", b.Available())
	}
}

func TestTokenBudget_Commit_ExceedsReserved(t *testing.T) {
	b := NewTokenBudget(100)
	b.Reserve(10)
	b.Commit(20)
	if b.GetUsed() != 0 {
		t.Errorf("GetUsed() = %d, want 0 (commit ignored when tokens > reserved)", b.GetUsed())
	}
}

func TestTokenBudget_Release(t *testing.T) {
	b := NewTokenBudget(100)
	b.Reserve(50)
	b.Release(20)
	if b.Available() != 70 { // 100 - 0 used - 30 reserved
		t.Errorf("Available() = %d, want 70", b.Available())
	}
}

func TestTokenBudget_Release_ExceedsReserved(t *testing.T) {
	b := NewTokenBudget(100)
	b.Reserve(10)
	b.Release(50)
	avail := b.Available()
	if avail != 90 { 
		t.Errorf("Available() = %d, want 90", avail)
	}
}

func TestTokenBudget_Reset(t *testing.T) {
	b := NewTokenBudget(100)
	b.Allocate(40)
	b.Reserve(20)
	b.Reset()
	if b.GetUsed() != 0 {
		t.Errorf("GetUsed() = %d after Reset, want 0", b.GetUsed())
	}
	if b.Available() != 100 {
		t.Errorf("Available() = %d after Reset, want 100", b.Available())
	}
}

func TestTokenBudget_SetMaxTokens(t *testing.T) {
	b := NewTokenBudget(100)
	b.Allocate(30)
	b.SetMaxTokens(200)
	if b.Available() != 170 { // 200 - 30 used - 0 reserved
		t.Errorf("Available() = %d, want 170", b.Available())
	}
}

func TestDefaultQueryConfig(t *testing.T) {
	cfg := DefaultQueryConfig()
	if cfg.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want %q", cfg.Model, "claude-sonnet-4-5")
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", cfg.MaxTokens)
	}
	if cfg.Temperature != 1.0 {
		t.Errorf("Temperature = %f, want 1.0", cfg.Temperature)
	}
	if cfg.TopP != 1.0 {
		t.Errorf("TopP = %f, want 1.0", cfg.TopP)
	}
	if !cfg.EnableCaching {
		t.Error("EnableCaching = false, want true")
	}
	if cfg.StopSequences == nil {
		t.Error("StopSequences = nil, want empty slice")
	}
}

func TestNewQueryDeps(t *testing.T) {
	cfg := &QueryConfig{
		Model:     "test-model",
		MaxTokens: 2048,
	}
	deps := NewQueryDeps(cfg)
	if deps.Config != cfg {
		t.Error("Config not set correctly")
	}
	if deps.TokenBudget == nil {
		t.Fatal("TokenBudget is nil")
	}
	if deps.TokenBudget.MaxTokens != 2048 {
		t.Errorf("TokenBudget.MaxTokens = %d, want 2048", deps.TokenBudget.MaxTokens)
	}
}

func TestNewQueryDeps_NilConfig(t *testing.T) {
	deps := NewQueryDeps(nil)
	if deps.Config == nil {
		t.Fatal("Config should default to DefaultQueryConfig")
	}
	if deps.Config.Model != "claude-sonnet-4-5" {
		t.Errorf("Default model = %q, want %q", deps.Config.Model, "claude-sonnet-4-5")
	}
	if deps.TokenBudget.MaxTokens != 4096 {
		t.Errorf("Default MaxTokens = %d, want 4096", deps.TokenBudget.MaxTokens)
	}
}

func TestStopHooks_Add(t *testing.T) {
	sh := NewStopHooks()
	called := false
	sh.Add(func(ctx context.Context, reason string) error {
		called = true
		return nil
	})
	if err := sh.Execute(context.Background(), "test"); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !called {
		t.Error("Hook was not called")
	}
}

func TestStopHooks_Execute_Sequential(t *testing.T) {
	sh := NewStopHooks()
	var order []int
	sh.Add(func(ctx context.Context, reason string) error {
		order = append(order, 1)
		return nil
	})
	sh.Add(func(ctx context.Context, reason string) error {
		order = append(order, 2)
		return nil
	})
	sh.Add(func(ctx context.Context, reason string) error {
		order = append(order, 3)
		return nil
	})
	sh.Execute(context.Background(), "test")
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("Hooks executed in wrong order: %v", order)
	}
}

func TestStopHooks_Execute_FirstError(t *testing.T) {
	sh := NewStopHooks()
	sh.Add(func(ctx context.Context, reason string) error {
		return errors.New("hook1 error")
	})
	sh.Add(func(ctx context.Context, reason string) error {
		return errors.New("hook2 error")
	})
	err := sh.Execute(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected error from Execute")
	}
	if err.Error() != "hook1 error" {
		t.Errorf("Error = %q, want %q", err.Error(), "hook1 error")
	}
}

func TestStopHooks_Clear(t *testing.T) {
	sh := NewStopHooks()
	called := false
	sh.Add(func(ctx context.Context, reason string) error {
		called = true
		return nil
	})
	sh.Clear()
	sh.Execute(context.Background(), "test")
	if called {
		t.Error("Hook should not be called after Clear")
	}
}

func TestStopHooks_Execute_Empty(t *testing.T) {
	sh := NewStopHooks()
	if err := sh.Execute(context.Background(), "test"); err != nil {
		t.Errorf("Execute on empty hooks should return nil, got %v", err)
	}
}
