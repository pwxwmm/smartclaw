package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SandboxConfig controls the execution limits for sandboxed plugin runs.
type SandboxConfig struct {
	Timeout       time.Duration
	MaxMemoryMB   int
	AllowedPaths  []string
	NetworkAccess bool
	MaxOutputBytes int
}

// DefaultSandboxConfig returns sensible defaults:
// 30s timeout, 256MB memory, no network, 1MB output limit.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		Timeout:        30 * time.Second,
		MaxMemoryMB:    256,
		AllowedPaths:   nil,
		NetworkAccess:  false,
		MaxOutputBytes: 1024 * 1024,
	}
}

// ExecutionMetrics tracks performance data from a single plugin execution.
type ExecutionMetrics struct {
	Duration    time.Duration `json:"duration"`
	OutputBytes int           `json:"output_bytes"`
	PeakMemoryMB int          `json:"peak_memory_mb"`
	ExitCode    int           `json:"exit_code"`
}

// SandboxError wraps errors from sandboxed execution with additional context.
type SandboxError struct {
	Err             error
	Timeout         bool
	OutputTruncated bool
}

func (e *SandboxError) Error() string {
	if e.Timeout {
		return fmt.Sprintf("sandbox timeout: %v", e.Err)
	}
	if e.OutputTruncated {
		return fmt.Sprintf("sandbox output truncated: %v", e.Err)
	}
	return e.Err.Error()
}

func (e *SandboxError) Unwrap() error { return e.Err }

// Sandbox provides a controlled execution environment for plugins with
// configurable timeouts, memory limits, and output size constraints.
type Sandbox struct {
	config  SandboxConfig
	metrics map[string]*ExecutionMetrics
	mu      sync.RWMutex
}

// NewSandbox creates a Sandbox with the given configuration.
func NewSandbox(config SandboxConfig) *Sandbox {
	return &Sandbox{
		config:  config,
		metrics: make(map[string]*ExecutionMetrics),
	}
}

// Execute runs a function inside the sandbox with the configured constraints.
// It enforces timeout via context cancellation and tracks execution metrics.
func (s *Sandbox) Execute(ctx context.Context, plugin PluginInterface, fn func(ctx context.Context) (any, error)) (any, error) {
	name := plugin.Name()

	timeout := s.config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		value any
		err   error
	}

	ch := make(chan result, 1)

	start := time.Now()

	go func() {
		val, err := fn(ctx)
		ch <- result{value: val, err: err}
	}()

	select {
	case <-ctx.Done():
		m := &ExecutionMetrics{
			Duration: time.Since(start),
			ExitCode: -1,
		}
		s.recordMetrics(name, m)

		return nil, &SandboxError{
			Err:     ctx.Err(),
			Timeout: true,
		}
	case r := <-ch:
		m := &ExecutionMetrics{
			Duration: time.Since(start),
			ExitCode: 0,
		}

		if r.err != nil {
			m.ExitCode = 1
			s.recordMetrics(name, m)
			return nil, r.err
		}

		outputBytes, truncated, truncatedResult := s.enforceOutputLimit(r.value)
		m.OutputBytes = outputBytes

		s.recordMetrics(name, m)

		if truncated {
			return truncatedResult, &SandboxError{
				Err:             fmt.Errorf("output exceeded %d bytes", s.config.MaxOutputBytes),
				OutputTruncated: true,
			}
		}

		return truncatedResult, nil
	}
}

// ExecuteTool wraps a ToolPlugin.ExecuteTool call inside the sandbox,
// enforces output limits, and records execution metrics.
func (s *Sandbox) ExecuteTool(ctx context.Context, plugin ToolPlugin, input map[string]any) (any, error) {
	result, err := s.Execute(ctx, plugin, func(ctx context.Context) (any, error) {
		return plugin.ExecuteTool(ctx, input)
	})

	if err != nil {
		if sandboxErr, ok := err.(*SandboxError); ok && sandboxErr.OutputTruncated {
			return result, err
		}
		return nil, err
	}

	return result, nil
}

// GetMetrics returns the last recorded execution metrics for a plugin.
func (s *Sandbox) GetMetrics(pluginName string) *ExecutionMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics[pluginName]
}

func (s *Sandbox) recordMetrics(name string, m *ExecutionMetrics) {
	s.mu.Lock()
	s.metrics[name] = m
	s.mu.Unlock()
}

func (s *Sandbox) enforceOutputLimit(value any) (int, bool, any) {
	if s.config.MaxOutputBytes <= 0 {
		return 0, false, value
	}

	data, err := json.Marshal(value)
	if err != nil {
		return 0, false, value
	}

	outputLen := len(data)
	if outputLen <= s.config.MaxOutputBytes {
		return outputLen, false, value
	}

	truncated := bytes.TrimSpace(data[:s.config.MaxOutputBytes])

	var result any
	if json.Unmarshal(truncated, &result) != nil {
		result = map[string]any{
			"output":       string(truncated),
			"truncated":    true,
			"original_size": outputLen,
		}
	}

	return s.config.MaxOutputBytes, true, result
}
