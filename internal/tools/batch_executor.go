package tools

import (
	"context"
	"log/slog"
	"sync"
)

// DeferredCall represents a tool call queued for batch execution.
type DeferredCall struct {
	Name   string         `json:"name"`
	Input  map[string]any `json:"input"`
	Result any            `json:"result,omitempty"`
	Error  error          `json:"error,omitempty"`
	Done   bool           `json:"done"`
}

// BatchExecutor manages deferred tool execution in lazy mode.
type BatchExecutor struct {
	mu      sync.Mutex
	queue   []DeferredCall
	lazy    bool
	enabled bool
}

// NewBatchExecutor creates a BatchExecutor.
func NewBatchExecutor() *BatchExecutor {
	return &BatchExecutor{
		queue: make([]DeferredCall, 0),
	}
}

// SetLazyMode enables or disables lazy/batch mode.
func (be *BatchExecutor) SetLazyMode(enabled bool) {
	be.mu.Lock()
	defer be.mu.Unlock()
	be.lazy = enabled
}

// IsLazyMode returns whether lazy mode is active.
func (be *BatchExecutor) IsLazyMode() bool {
	be.mu.Lock()
	defer be.mu.Unlock()
	return be.lazy
}

// Enqueue adds a tool call to the deferred queue.
// Returns true if the call was deferred, false if it should execute immediately.
func (be *BatchExecutor) Enqueue(name string, input map[string]any) bool {
	be.mu.Lock()
	defer be.mu.Unlock()

	if !be.lazy {
		return false
	}

	be.queue = append(be.queue, DeferredCall{
		Name:  name,
		Input: input,
	})
	slog.Debug("batch executor: deferred tool call", "tool", name, "queue_size", len(be.queue))
	return true
}

// Flush executes all queued tool calls and returns results.
func (be *BatchExecutor) Flush(ctx context.Context, registry *ToolRegistry) []DeferredCall {
	be.mu.Lock()
	queue := make([]DeferredCall, len(be.queue))
	copy(queue, be.queue)
	be.queue = be.queue[:0]
	be.mu.Unlock()

	for i := range queue {
		result, err := registry.Execute(ctx, queue[i].Name, queue[i].Input)
		queue[i].Result = result
		queue[i].Error = err
		queue[i].Done = true
	}

	return queue
}

// QueueSize returns the number of pending deferred calls.
func (be *BatchExecutor) QueueSize() int {
	be.mu.Lock()
	defer be.mu.Unlock()
	return len(be.queue)
}

// Peek returns the current queue without executing.
func (be *BatchExecutor) Peek() []DeferredCall {
	be.mu.Lock()
	defer be.mu.Unlock()
	result := make([]DeferredCall, len(be.queue))
	copy(result, be.queue)
	return result
}

// Clear empties the deferred queue without executing.
func (be *BatchExecutor) Clear() {
	be.mu.Lock()
	defer be.mu.Unlock()
	be.queue = be.queue[:0]
}
