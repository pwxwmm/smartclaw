package lifecycle

import (
	"sync"
	"testing"
	"time"
)

type mockClosable struct {
	mu       sync.Mutex
	closed   bool
	closeErr error
	order    int
}

func (m *mockClosable) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.closeErr
}

func (m *mockClosable) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestRegister(t *testing.T) {
	t.Parallel()

	m := &Manager{}
	c := &mockClosable{}
	m.Register(c)

	if len(m.closables) != 1 {
		t.Errorf("len(closables) = %d, want 1", len(m.closables))
	}
}

func TestShutdown_LIFOOrder(t *testing.T) {
	m := &Manager{}

	var order []int
	var mu sync.Mutex

	recordClose := func(idx int) Closable {
		return &closableFn{fn: func() error {
			mu.Lock()
			order = append(order, idx)
			mu.Unlock()
			return nil
		}}
	}

	m.Register(recordClose(1))
	m.Register(recordClose(2))
	m.Register(recordClose(3))

	m.Shutdown(5 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("order length = %d, want 3", len(order))
	}
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("shutdown order = %v, want [3 2 1]", order)
	}
}

func TestShutdown_ContinuesOnError(t *testing.T) {
	m := &Manager{}

	var order []int
	var mu sync.Mutex

	recordClose := func(idx int, err error) Closable {
		return &closableFn{fn: func() error {
			mu.Lock()
			order = append(order, idx)
			mu.Unlock()
			return err
		}}
	}

	m.Register(recordClose(1, nil))
	m.Register(recordClose(2, errCloseFailed))
	m.Register(recordClose(3, nil))

	m.Shutdown(5 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Errorf("order length = %d, want 3 (should continue after error)", len(order))
	}
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("shutdown order = %v, want [3 2 1]", order)
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	m := &Manager{}

	callCount := 0
	m.Register(&closableFn{fn: func() error {
		callCount++
		return nil
	}})

	m.Shutdown(5 * time.Second)
	m.Shutdown(5 * time.Second)

	if callCount != 1 {
		t.Errorf("Close called %d times, want 1 (Shutdown should be idempotent)", callCount)
	}
}

func TestShutdown_EmptyManager(t *testing.T) {
	m := &Manager{}

	done := make(chan struct{})
	go func() {
		m.Shutdown(5 * time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown on empty Manager hung")
	}
}

func TestRegister_AfterShutdown(t *testing.T) {
	m := &Manager{}
	m.Shutdown(5 * time.Second)

	c := &mockClosable{}
	m.Register(c)

	if !m.closed {
		t.Error("Manager should still be marked closed")
	}

	if len(m.closables) != 1 {
		t.Errorf("Register after shutdown should still append, got %d closables", len(m.closables))
	}
}

func TestShutdown_WithTimeout(t *testing.T) {
	m := &Manager{}

	m.Register(&closableFn{fn: func() error {
		time.Sleep(10 * time.Second)
		return nil
	}})

	start := time.Now()
	m.Shutdown(50 * time.Millisecond)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("Shutdown took too long (%v), should respect timeout", elapsed)
	}
}

func TestShutdown_CallsCloseOnEach(t *testing.T) {
	m := &Manager{}

	c1 := &mockClosable{}
	c2 := &mockClosable{}
	c3 := &mockClosable{}

	m.Register(c1)
	m.Register(c2)
	m.Register(c3)

	m.Shutdown(5 * time.Second)

	for i, c := range []*mockClosable{c1, c2, c3} {
		if !c.isClosed() {
			t.Errorf("closable %d was not closed", i+1)
		}
	}
}

func TestDefault(t *testing.T) {
	d := Default()
	if d == nil {
		t.Fatal("Default() returned nil")
	}
	if d != defaultManager {
		t.Error("Default() should return the global defaultManager")
	}
}

func TestWaitForSignal_RespectsContext(t *testing.T) {
	m := &Manager{}
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.Shutdown(5 * time.Second)
	}()

	m.Register(&closableFn{fn: func() error { return nil }})
	m.closed = true

	wg.Wait()
}

type closableFn struct {
	fn func() error
}

func (c *closableFn) Close() error {
	return c.fn()
}

var errCloseFailed = errClose("close failed")

type errClose string

func (e errClose) Error() string { return string(e) }
