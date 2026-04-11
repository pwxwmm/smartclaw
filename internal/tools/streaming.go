package tools

import (
	"context"
	"sync"
	"time"
)

// StreamChunk represents an intermediate result from a long-running tool.
type StreamChunk struct {
	ToolName string    `json:"tool_name"`
	Data     string    `json:"data"`
	Progress float64   `json:"progress,omitempty"` // 0.0 to 1.0
	Done     bool      `json:"done"`
	Error    string    `json:"error,omitempty"`
	Time     time.Time `json:"time"`
}

// StreamCallback is invoked for each intermediate chunk produced by a streaming tool.
type StreamCallback func(chunk StreamChunk)

// StreamingTool extends the Tool interface with streaming output support.
type StreamingTool interface {
	Tool
	ExecuteStreaming(ctx context.Context, input map[string]any, callback StreamCallback) (any, error)
}

// StreamManager coordinates streaming tool output, allowing consumers to subscribe.
type StreamManager struct {
	mu        sync.RWMutex
	subs      map[string][]StreamCallback
	active    map[string]bool
	chunkBuf  map[string][]StreamChunk
	maxBuffer int
}

// NewStreamManager creates a new StreamManager.
func NewStreamManager() *StreamManager {
	return &StreamManager{
		subs:      make(map[string][]StreamCallback),
		active:    make(map[string]bool),
		chunkBuf:  make(map[string][]StreamChunk),
		maxBuffer: 100,
	}
}

// Subscribe registers a callback for streaming output of a specific execution.
func (sm *StreamManager) Subscribe(executionID string, cb StreamCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.subs[executionID] = append(sm.subs[executionID], cb)

	// Replay buffered chunks
	if chunks, ok := sm.chunkBuf[executionID]; ok {
		go func() {
			for _, chunk := range chunks {
				cb(chunk)
			}
		}()
	}
}

// Unsubscribe removes a callback.
func (sm *StreamManager) Unsubscribe(executionID string, cb StreamCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if subs, ok := sm.subs[executionID]; ok {
		for i, s := range subs {
			if &s == &cb {
				sm.subs[executionID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// Emit sends a chunk to all subscribers of the given execution.
func (sm *StreamManager) Emit(executionID string, chunk StreamChunk) {
	sm.mu.Lock()
	// Buffer the chunk
	sm.chunkBuf[executionID] = append(sm.chunkBuf[executionID], chunk)
	if len(sm.chunkBuf[executionID]) > sm.maxBuffer {
		sm.chunkBuf[executionID] = sm.chunkBuf[executionID][1:]
	}
	subs := make([]StreamCallback, len(sm.subs[executionID]))
	copy(subs, sm.subs[executionID])
	sm.mu.Unlock()

	for _, cb := range subs {
		cb(chunk)
	}
}

// Start marks an execution as active.
func (sm *StreamManager) Start(executionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.active[executionID] = true
}

// Finish marks an execution as complete and cleans up subscribers.
func (sm *StreamManager) Finish(executionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.active, executionID)

	// Keep buffer for late subscribers for a short time, then clean
	go func() {
		time.Sleep(5 * time.Second)
		sm.mu.Lock()
		delete(sm.chunkBuf, executionID)
		delete(sm.subs, executionID)
		sm.mu.Unlock()
	}()
}

// IsActive checks if an execution is still running.
func (sm *StreamManager) IsActive(executionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.active[executionID]
}

// ExecuteWithStreaming runs a tool with streaming support.
// If the tool implements StreamingTool, it uses ExecuteStreaming.
// Otherwise, it falls back to regular Execute and emits the result as a single chunk.
func ExecuteWithStreaming(ctx context.Context, registry *ToolRegistry, toolName string, input map[string]any, executionID string, sm *StreamManager) (any, error) {
	tool := registry.Get(toolName)
	if tool == nil {
		return nil, ErrToolNotFound(toolName)
	}

	sm.Start(executionID)

	if st, ok := tool.(StreamingTool); ok {
		result, err := st.ExecuteStreaming(ctx, input, func(chunk StreamChunk) {
			chunk.ToolName = toolName
			sm.Emit(executionID, chunk)
		})
		sm.Finish(executionID)
		return result, err
	}

	// Fallback: run normally, emit result as a single chunk
	result, err := tool.Execute(ctx, input)

	chunk := StreamChunk{
		ToolName: toolName,
		Progress: 1.0,
		Done:     true,
		Time:     time.Now(),
	}
	if err != nil {
		chunk.Error = err.Error()
	}

	sm.Emit(executionID, chunk)
	sm.Finish(executionID)

	return result, err
}
