package resilience

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_InitialTokens(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(50, time.Second)
	if rl.tokens != 50 {
		t.Errorf("initial tokens = %d, want 50", rl.tokens)
	}
	if rl.maxTokens != 50 {
		t.Errorf("maxTokens = %d, want 50", rl.maxTokens)
	}
}

func TestRateLimiter_WaitAcquiresToken(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(5, time.Second)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Errorf("Wait(%d) failed: %v", i, err)
		}
	}
}

func TestRateLimiter_WaitContextCancellation(t *testing.T) {
	rl := NewRateLimiter(1, 100*time.Millisecond)
	ctx := context.Background()

	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first Wait failed: %v", err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(cancelCtx)
	if err == nil {
		t.Error("Wait with cancelled context should return error")
	}
}

func TestRateLimiter_WaitContextTimeout(t *testing.T) {
	rl := NewRateLimiter(1, 10*time.Second)
	ctx := context.Background()

	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first Wait failed: %v", err)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(timeoutCtx)
	if err == nil {
		t.Error("Wait with expired context should return error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	ctx := context.Background()

	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait 1 failed: %v", err)
	}
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait 2 failed: %v", err)
	}

	rl.mu.Lock()
	if rl.tokens != 0 {
		t.Errorf("tokens after exhaustion = %d, want 0", rl.tokens)
	}
	rl.mu.Unlock()

	time.Sleep(120 * time.Millisecond)

	if err := rl.Wait(ctx); err != nil {
		t.Errorf("Wait after refill failed: %v", err)
	}
}

func TestRateLimiter_ConcurrentWait(t *testing.T) {
	rl := NewRateLimiter(50, time.Second)

	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := rl.Wait(ctx); err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if got := successCount.Load(); got != 50 {
		t.Errorf("successful Wait calls = %d, want 50", got)
	}
}

func TestRateLimiter_ConcurrentWaitExceedsCapacity(t *testing.T) {
	rl := NewRateLimiter(10, 50*time.Millisecond)

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var timeoutCount atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			if err := rl.Wait(ctx); err == nil {
				successCount.Add(1)
			} else {
				timeoutCount.Add(1)
			}
		}()
	}

	wg.Wait()

	total := successCount.Load() + timeoutCount.Load()
	if total != 20 {
		t.Errorf("total outcomes = %d, want 20", total)
	}
	if successCount.Load() < 10 {
		t.Errorf("successCount = %d, want at least 10", successCount.Load())
	}
}

func TestRateLimiter_RefillDoesNotExceedMax(t *testing.T) {
	rl := NewRateLimiter(5, 10*time.Millisecond)
	ctx := context.Background()

	rl.Wait(ctx)
	rl.Wait(ctx)

	time.Sleep(100 * time.Millisecond)

	rl.mu.Lock()
	tokens := rl.tokens
	rl.mu.Unlock()

	if tokens > 5 {
		t.Errorf("tokens after refill = %d, want <= 5 (maxTokens)", tokens)
	}
}

func TestRateLimiter_NewRateLimiterSetsFields(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(100, 500*time.Millisecond)

	if rl.maxTokens != 100 {
		t.Errorf("maxTokens = %d, want 100", rl.maxTokens)
	}
	if rl.refillRate != 500*time.Millisecond {
		t.Errorf("refillRate = %v, want 500ms", rl.refillRate)
	}
	if rl.tokens != 100 {
		t.Errorf("tokens = %d, want 100", rl.tokens)
	}
	if rl.lastRefill.IsZero() {
		t.Error("lastRefill should be set to current time")
	}
}
