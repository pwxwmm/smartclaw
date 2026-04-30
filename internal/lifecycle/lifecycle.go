package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Closable interface {
	Close() error
}

type Manager struct {
	mu        sync.Mutex
	closables []Closable
	closed    bool
}

var defaultManager = &Manager{}

// Default returns the global lifecycle Manager.
func Default() *Manager {
	return defaultManager
}

// Register adds a Closable to the global lifecycle Manager.
func Register(c Closable) {
	defaultManager.Register(c)
}

func (m *Manager) Register(c Closable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closables = append(m.closables, c)
}

func (m *Manager) Shutdown(timeout time.Duration) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	closables := make([]Closable, len(m.closables))
	copy(closables, m.closables)
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := len(closables) - 1; i >= 0; i-- {
			if err := closables[i].Close(); err != nil {
				slog.Error("lifecycle: close failed", "error", err, "index", i)
			} else {
				slog.Info("lifecycle: closed", "index", i)
			}
		}
	}()

	select {
	case <-done:
		slog.Info("lifecycle: shutdown complete")
	case <-ctx.Done():
		slog.Warn("lifecycle: shutdown timed out", "timeout", timeout)
	}
}

func WaitForSignal() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("lifecycle: received signal, shutting down", "signal", sig)
	Default().Shutdown(15 * time.Second)
	return sig
}
