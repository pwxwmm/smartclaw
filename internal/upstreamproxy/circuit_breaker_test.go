package upstreamproxy

import (
	"testing"
	"time"
)

func TestNewCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(3, 30*time.Second)

	if cb.GetState() != CircuitClosed {
		t.Errorf("expected initial state closed, got %s", cb.GetState())
	}
	if !cb.Allow() {
		t.Error("expected Allow()=true in closed state")
	}
}

func TestCircuitBreaker_StayClosedUnderThreshold(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)

	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != CircuitClosed {
		t.Errorf("expected closed state with 4 failures (< 5 max), got %s", cb.GetState())
	}
	if !cb.Allow() {
		t.Error("expected Allow()=true when under failure threshold")
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 30*time.Second)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != CircuitOpen {
		t.Errorf("expected open state after 3 failures, got %s", cb.GetState())
	}
	if cb.Allow() {
		t.Error("expected Allow()=false in open state")
	}
}

func TestCircuitBreaker_OpenRejectsRequests(t *testing.T) {
	cb := NewCircuitBreaker(1, 30*time.Second)

	cb.RecordFailure()

	if cb.GetState() != CircuitOpen {
		t.Errorf("expected open state, got %s", cb.GetState())
	}
	for i := 0; i < 10; i++ {
		if cb.Allow() {
			t.Error("expected Allow()=false in open state")
			break
		}
	}
}

func TestCircuitBreaker_TransitionToHalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	if cb.GetState() != CircuitOpen {
		t.Errorf("expected open state, got %s", cb.GetState())
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Error("expected Allow()=true after timeout (half-open)")
	}
	if cb.GetState() != CircuitHalfOpen {
		t.Errorf("expected half-open state after timeout, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenAllowsRequests(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Error("expected Allow()=true in half-open state")
	}
}

func TestCircuitBreaker_HalfOpenToClosedOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.Allow()
	cb.RecordSuccess()

	if cb.GetState() != CircuitClosed {
		t.Errorf("expected closed state after success in half-open, got %s", cb.GetState())
	}
	if !cb.Allow() {
		t.Error("expected Allow()=true after recovery")
	}
}

func TestCircuitBreaker_HalfOpenBackToOpenOnFailure(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.Allow()
	cb.RecordFailure()

	if cb.GetState() != CircuitOpen {
		t.Errorf("expected open state after failure in half-open, got %s", cb.GetState())
	}
	if cb.Allow() {
		t.Error("expected Allow()=false after half-open failure")
	}
}

func TestCircuitBreaker_FullTransitionCycle(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	if cb.GetState() != CircuitClosed {
		t.Errorf("step 1: expected closed, got %s", cb.GetState())
	}

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.GetState() != CircuitOpen {
		t.Errorf("step 2: expected open, got %s", cb.GetState())
	}

	time.Sleep(60 * time.Millisecond)
	cb.Allow()
	if cb.GetState() != CircuitHalfOpen {
		t.Errorf("step 3: expected half-open, got %s", cb.GetState())
	}

	cb.RecordSuccess()
	if cb.GetState() != CircuitClosed {
		t.Errorf("step 4: expected closed after recovery, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(1, 30*time.Second)

	cb.RecordFailure()
	if cb.GetState() != CircuitOpen {
		t.Errorf("expected open state before reset, got %s", cb.GetState())
	}

	cb.Reset()

	if cb.GetState() != CircuitClosed {
		t.Errorf("expected closed state after reset, got %s", cb.GetState())
	}
	if !cb.Allow() {
		t.Error("expected Allow()=true after reset")
	}
}

func TestCircuitBreaker_SuccessInClosedStateNoOp(t *testing.T) {
	cb := NewCircuitBreaker(3, 30*time.Second)

	cb.RecordSuccess()

	if cb.GetState() != CircuitClosed {
		t.Errorf("expected closed state after success in closed, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_GetStats(t *testing.T) {
	cb := NewCircuitBreaker(3, 30*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()

	stats := cb.GetStats()

	if stats["state"] != "closed" {
		t.Errorf("expected state=closed, got %v", stats["state"])
	}
	if stats["failures"] != 2 {
		t.Errorf("expected failures=2, got %v", stats["failures"])
	}
	if stats["max_failures"] != 3 {
		t.Errorf("expected max_failures=3, got %v", stats["max_failures"])
	}
}

func TestCircuitBreaker_TimeoutNotYetExpired(t *testing.T) {
	cb := NewCircuitBreaker(1, 5*time.Second)

	cb.RecordFailure()

	if cb.Allow() {
		t.Error("expected Allow()=false immediately after opening (timeout not expired)")
	}
}

func TestCircuitBreaker_MultipleFailuresResetOnRecovery(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.GetState() != CircuitOpen {
		t.Errorf("expected open, got %s", cb.GetState())
	}

	time.Sleep(60 * time.Millisecond)
	cb.Allow()
	cb.RecordSuccess()

	stats := cb.GetStats()
	if stats["failures"] != 0 {
		t.Errorf("expected failures=0 after recovery, got %v", stats["failures"])
	}

	cb.RecordFailure()
	if cb.GetState() != CircuitClosed {
		t.Errorf("expected still closed after 1 failure (< threshold), got %s", cb.GetState())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.expected {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}
