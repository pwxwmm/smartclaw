package tools

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"
)

// HealingStrategy defines how to fix a failed tool call.
type HealingStrategy int

const (
	HealNone          HealingStrategy = iota
	HealRetry                         // Simple retry
	HealPathFix                       // Fix path-related errors
	HealTimeoutExtend                 // Extend timeout
)

// HealResult represents the outcome of a self-healing attempt.
type HealResult struct {
	OriginalError error
	Strategy      HealingStrategy
	Retried       bool
	FixedInput    map[string]any
	Result        any
	Attempts      int
}

// SelfHealingExecutor wraps tool execution with automatic error recovery.
type SelfHealingExecutor struct {
	registry *ToolRegistry
	maxRetry int
	enabled  bool
}

// NewSelfHealingExecutor creates a self-healing executor.
func NewSelfHealingExecutor(registry *ToolRegistry) *SelfHealingExecutor {
	return &SelfHealingExecutor{
		registry: registry,
		maxRetry: 2,
		enabled:  true,
	}
}

// ExecuteWithHealing runs a tool and attempts to recover from errors.
func (she *SelfHealingExecutor) ExecuteWithHealing(ctx context.Context, name string, input map[string]any) *HealResult {
	result := &HealResult{
		FixedInput: input,
		Attempts:   0,
	}

	currentInput := copyInput(input)

	for attempt := 0; attempt <= she.maxRetry; attempt++ {
		result.Attempts = attempt + 1

		output, err := she.registry.Execute(ctx, name, currentInput)
		if err == nil {
			result.Result = output
			result.Retried = attempt > 0
			result.FixedInput = currentInput
			return result
		}

		result.OriginalError = err

		if !she.enabled || attempt >= she.maxRetry {
			return result
		}

		// Exponential backoff with jitter before retry
		delay := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		time.Sleep(delay + jitter)

		strategy := she.diagnose(name, err, currentInput)
		result.Strategy = strategy

		if strategy == HealNone {
			return result
		}

		fixedInput, fixed := she.applyHealing(strategy, name, err, currentInput)
		if !fixed {
			return result
		}

		slog.Info("self-healing: applying fix", "tool", name, "strategy", strategy, "attempt", attempt+1)
		currentInput = fixedInput
	}

	return result
}

// diagnose analyzes an error and suggests a healing strategy.
func (she *SelfHealingExecutor) diagnose(toolName string, err error, input map[string]any) HealingStrategy {
	errMsg := strings.ToLower(err.Error())

	if isPathError(errMsg) && hasPathParam(toolName, input) {
		return HealPathFix
	}

	if isTimeoutError(errMsg) {
		return HealTimeoutExtend
	}

	if isTransientError(errMsg) {
		return HealRetry
	}

	return HealNone
}

// applyHealing modifies the input based on the healing strategy.
func (she *SelfHealingExecutor) applyHealing(strategy HealingStrategy, toolName string, err error, input map[string]any) (map[string]any, bool) {
	switch strategy {
	case HealPathFix:
		return she.fixPath(toolName, input)
	case HealTimeoutExtend:
		return she.extendTimeout(input)
	case HealRetry:
		return input, true
	default:
		return input, false
	}
}

// fixPath attempts to find the correct path when a file is not found.
func (she *SelfHealingExecutor) fixPath(toolName string, input map[string]any) (map[string]any, bool) {
	pathKeys := []string{"path", "file_path", "filepath", "filename"}
	for _, key := range pathKeys {
		if pathVal, ok := input[key].(string); ok && pathVal != "" {
			if _, err := os.Stat(pathVal); err == nil {
				continue
			}

			// Try: filename only (search in cwd)
			baseName := filepath.Base(pathVal)
			if foundPath, err := findFile(baseName); err == nil {
				fixed := copyInput(input)
				fixed[key] = foundPath
				return fixed, true
			}

			// Try: with common prefixes
			absPath, _ := filepath.Abs(pathVal)
			if _, err := os.Stat(absPath); err == nil {
				fixed := copyInput(input)
				fixed[key] = absPath
				return fixed, true
			}
		}
	}

	return input, false
}

// extendTimeout doubles the timeout for timeout errors.
func (she *SelfHealingExecutor) extendTimeout(input map[string]any) (map[string]any, bool) {
	if timeout, ok := input["timeout"].(int); ok && timeout > 0 {
		fixed := copyInput(input)
		fixed["timeout"] = timeout * 2
		return fixed, true
	}
	return input, false
}

func (she *SelfHealingExecutor) IsEnabled() bool {
	return she.enabled
}

func (she *SelfHealingExecutor) SetEnabled(enabled bool) {
	she.enabled = enabled
}

func isPathError(errMsg string) bool {
	indicators := []string{
		"no such file", "not found", "does not exist",
		"cannot find", "not a directory", "no such file or directory",
		"stat ", "open ",
	}
	for _, ind := range indicators {
		if strings.Contains(errMsg, ind) {
			return true
		}
	}
	return false
}

func isTimeoutError(errMsg string) bool {
	indicators := []string{"timeout", "timed out", "deadline exceeded", "context deadline"}
	for _, ind := range indicators {
		if strings.Contains(errMsg, ind) {
			return true
		}
	}
	return false
}

func isTransientError(errMsg string) bool {
	indicators := []string{"connection refused", "connection reset", "temporary", "retry"}
	for _, ind := range indicators {
		if strings.Contains(errMsg, ind) {
			return true
		}
	}
	return false
}

func hasPathParam(toolName string, input map[string]any) bool {
	pathKeys := []string{"path", "file_path", "filepath", "filename"}
	for _, key := range pathKeys {
		if _, ok := input[key]; ok {
			return true
		}
	}
	return false
}

func findFile(name string) (string, error) {
	cwd, _ := os.Getwd()
	limit := 3
	result, _ := findFileInDir(cwd, name, limit)
	if result != "" {
		return result, nil
	}
	return "", fmt.Errorf("file not found: %s", name)
}

func findFileInDir(dir, name string, depth int) (string, error) {
	if depth <= 0 {
		return "", nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, e := range entries {
		if e.IsDir() {
			subdir := filepath.Join(dir, e.Name())
			skipDirs := map[string]bool{".git": true, "node_modules": true, "vendor": true, "dist": true}
			if skipDirs[e.Name()] {
				continue
			}
			if result, err := findFileInDir(subdir, name, depth-1); err == nil && result != "" {
				return result, nil
			}
		} else if e.Name() == name {
			return filepath.Join(dir, name), nil
		}
	}

	return "", nil
}

func copyInput(input map[string]any) map[string]any {
	copy := make(map[string]any, len(input))
	for k, v := range input {
		copy[k] = v
	}
	return copy
}
