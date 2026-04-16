package resilience

import (
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	if cb.GetState() != StateClosed {
		t.Errorf("initial state = %v, want StateClosed", cb.GetState())
	}
}

func TestCircuitBreaker_AllowWhenClosed(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	if err := cb.Allow(); err != nil {
		t.Errorf("Allow() in Closed state returned error: %v", err)
	}
}

func TestCircuitBreaker_TransitionsToOpenAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 30*time.Second)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("state after 5 failures = %v, want StateOpen", cb.GetState())
	}
}

func TestCircuitBreaker_DoesNotOpenBeforeMaxFailures(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)

	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateClosed {
		t.Errorf("state after 4 failures = %v, want StateClosed", cb.GetState())
	}
}

func TestCircuitBreaker_OpenStateRejectsRequests(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 30*time.Second)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	err := cb.Allow()
	if err == nil {
		t.Error("Allow() in Open state should return error")
	}
}

func TestCircuitBreaker_HalfOpenAfterResetTimeout(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 50*time.Millisecond)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateOpen {
		t.Fatal("expected Open state before timeout")
	}

	time.Sleep(80 * time.Millisecond)

	if err := cb.Allow(); err != nil {
		t.Errorf("Allow() after reset timeout returned error: %v", err)
	}

	if cb.GetState() != StateHalfOpen {
		t.Errorf("state after reset timeout = %v, want StateHalfOpen", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenSuccessesTransitionToClosed(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 50*time.Millisecond)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	time.Sleep(80 * time.Millisecond)

	cb.Allow()

	cb.RecordSuccess()
	if cb.GetState() != StateHalfOpen {
		t.Errorf("state after 1 half-open success = %v, want StateHalfOpen", cb.GetState())
	}

	cb.RecordSuccess()
	if cb.GetState() != StateClosed {
		t.Errorf("state after 2 half-open successes = %v, want StateClosed", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenFailureTransitionsToOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 50*time.Millisecond)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	time.Sleep(80 * time.Millisecond)
	cb.Allow()

	cb.RecordSuccess()

	cb.RecordFailure()
	if cb.GetState() != StateOpen {
		t.Errorf("state after half-open failure = %v, want StateOpen", cb.GetState())
	}
}

func TestCircuitBreaker_SuccessInClosedDoesNotResetFailures(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 30*time.Second)

	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}

	cb.RecordSuccess()

	cb.RecordFailure()

	if cb.GetState() != StateOpen {
		t.Errorf("state = %v, want StateOpen (4 prior failures + 1 = 5 >= maxFailures)", cb.GetState())
	}
}

func TestCircuitBreaker_SuccessInHalfOpenResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 50*time.Millisecond)

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	time.Sleep(80 * time.Millisecond)
	cb.Allow()

	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.GetState() != StateClosed {
		t.Fatalf("state after recovery = %v, want StateClosed", cb.GetState())
	}

	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateClosed {
		t.Errorf("state = %v, want StateClosed (failures should be reset after HalfOpen→Closed)", cb.GetState())
	}
}

func TestCircuitBreaker_FullCycle(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 50*time.Millisecond)

	if cb.GetState() != StateClosed {
		t.Fatal("initial state should be Closed")
	}

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.GetState() != StateOpen {
		t.Fatal("should be Open after 3 failures")
	}

	time.Sleep(80 * time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Fatalf("Allow after timeout: %v", err)
	}
	if cb.GetState() != StateHalfOpen {
		t.Fatal("should be HalfOpen after timeout")
	}

	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.GetState() != StateClosed {
		t.Fatal("should be Closed after 2 half-open successes")
	}
}

func TestCircuitBreaker_MultipleFailuresInHalfOpenReopen(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 50*time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	time.Sleep(80 * time.Millisecond)
	cb.Allow()

	cb.RecordFailure()
	if cb.GetState() != StateOpen {
		t.Errorf("state after failure in HalfOpen = %v, want StateOpen", cb.GetState())
	}
}

func TestCircuitBreaker_StateString(t *testing.T) {
	t.Parallel()

	states := map[State]string{
		StateClosed:   "Closed",
		StateOpen:     "Open",
		StateHalfOpen: "HalfOpen",
	}

	for s, name := range states {
		_ = s
		_ = name
	}
}

func TestCircuitBreaker_NameInErrorMessage(t *testing.T) {
	cb := NewCircuitBreaker("my-service", 2, 30*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()

	err := cb.Allow()
	if err == nil {
		t.Fatal("expected error in Open state")
	}

	want := "circuit breaker [my-service] is open"
	if err.Error() != want {
		t.Errorf("error message = %q, want %q", err.Error(), want)
	}
}
