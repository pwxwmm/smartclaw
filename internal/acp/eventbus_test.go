package acp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := NewEventBus()
	var received atomic.Value

	bus.Subscribe(EventToolCall, func(_ context.Context, e Event) error {
		received.Store(e)
		return nil
	})

	ev := Event{
		ID:        "e1",
		Type:      EventToolCall,
		Source:    "agent",
		Timestamp: time.Now(),
		Data:      map[string]any{"tool": "read_file"},
	}

	if err := bus.Publish(context.Background(), ev); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	got, ok := received.Load().(Event)
	if !ok {
		t.Fatal("handler was not called")
	}
	if got.ID != "e1" {
		t.Errorf("got ID %q, want %q", got.ID, "e1")
	}
	if got.Data["tool"] != "read_file" {
		t.Errorf("got tool %v, want read_file", got.Data["tool"])
	}
}

func TestEventBus_MultipleHandlers(t *testing.T) {
	bus := NewEventBus()
	var count atomic.Int32

	bus.Subscribe(EventNotification, func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})
	bus.Subscribe(EventNotification, func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})

	ev := Event{ID: "n1", Type: EventNotification, Source: "user", Timestamp: time.Now()}
	if err := bus.Publish(context.Background(), ev); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if got := count.Load(); got != 2 {
		t.Errorf("called %d handlers, want 2", got)
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	var count atomic.Int32

	subID := bus.Subscribe(EventToolResult, func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})

	ev := Event{ID: "r1", Type: EventToolResult, Source: "tool", Timestamp: time.Now()}
	bus.Publish(context.Background(), ev)
	if got := count.Load(); got != 1 {
		t.Fatalf("before unsubscribe: %d, want 1", got)
	}

	bus.Unsubscribe(EventToolResult, subID)
	bus.Publish(context.Background(), ev)
	if got := count.Load(); got != 1 {
		t.Errorf("after unsubscribe: %d, want still 1", got)
	}
}

func TestEventBus_PublishStopsOnError(t *testing.T) {
	bus := NewEventBus()
	var secondCalled atomic.Bool

	bus.Subscribe(EventError, func(_ context.Context, _ Event) error {
		return fmt.Errorf("boom")
	})
	bus.Subscribe(EventError, func(_ context.Context, _ Event) error {
		secondCalled.Store(true)
		return nil
	})

	ev := Event{ID: "e1", Type: EventError, Source: "tool", Timestamp: time.Now()}
	err := bus.Publish(context.Background(), ev)
	if err == nil {
		t.Fatal("expected error from Publish")
	}

	if secondCalled.Load() {
		t.Error("second handler should not have been called after first error")
	}
}

func TestEventBus_PublishAsync(t *testing.T) {
	bus := NewEventBus()
	var count atomic.Int32

	bus.Subscribe(EventMemoryUpdate, func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})

	ev := Event{ID: "m1", Type: EventMemoryUpdate, Source: "memory", Timestamp: time.Now()}
	bus.PublishAsync(context.Background(), ev)

	time.Sleep(50 * time.Millisecond)

	if got := count.Load(); got != 1 {
		t.Errorf("async handler called %d times, want 1", got)
	}
}

func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := NewEventBus()
	var count atomic.Int32

	bus.Subscribe(EventSessionStart, func(_ context.Context, _ Event) error {
		count.Add(1)
		return nil
	})

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ev := Event{
				ID:        fmt.Sprintf("s%d", i),
				Type:      EventSessionStart,
				Source:    "user",
				Timestamp: time.Now(),
			}
			bus.Publish(context.Background(), ev)
		}()
	}
	wg.Wait()

	if got := count.Load(); got != 100 {
		t.Errorf("concurrent publish: %d calls, want 100", got)
	}
}
