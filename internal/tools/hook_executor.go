package tools

import (
	"context"
	"fmt"

	"github.com/instructkr/smartclaw/internal/hooks"
)

// HookAwareExecutor wraps tool execution with hook lifecycle
type HookAwareExecutor struct {
	registry    *ToolRegistry
	hookManager *hooks.HookManager
}

// NewHookAwareExecutor creates a hook-aware tool executor
func NewHookAwareExecutor(registry *ToolRegistry, hookManager *hooks.HookManager) *HookAwareExecutor {
	return &HookAwareExecutor{
		registry:    registry,
		hookManager: hookManager,
	}
}

// ExecuteWithHooks runs a tool with PreToolUse and PostToolUse hooks
func (e *HookAwareExecutor) ExecuteWithHooks(ctx context.Context, name string, input map[string]any) (any, error) {
	// Get the tool first
	tool := e.registry.Get(name)
	if tool == nil {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	// Variable to track if execution should proceed
	shouldExecute := true
	var modifiedInput map[string]any = input

	// Execute PreToolUse hooks if hook manager exists
	if e.hookManager != nil {
		results, err := e.hookManager.ExecutePreToolUse(ctx, name, input)
		if err != nil {
			// Hook blocked execution
			return map[string]any{
				"error":   err.Error(),
				"blocked": true,
			}, err
		}

		// Process hook results
		for _, result := range results {
			if result.Output != nil {
				// Check if hook wants to block
				if result.Output.Decision == "block" {
					return map[string]any{
						"error":      result.Output.Reason,
						"blocked":    true,
						"hook_name":  result.HookName,
						"hook_event": string(result.Event),
					}, fmt.Errorf("hook blocked: %s", result.Output.Reason)
				}

				// Check if hook modified input
				if result.Output.UpdatedInput != nil {
					modifiedInput = result.Output.UpdatedInput
				}

				// Check if hook wants to suppress execution
				if result.Output.Decision == "suppress" {
					shouldExecute = false
				}
			}
		}
	}

	// Execute the tool if not suppressed
	var output any
	var execErr error

	if shouldExecute {
		output, execErr = tool.Execute(ctx, modifiedInput)
	} else {
		output = map[string]any{
			"suppressed": true,
			"message":    "Tool execution suppressed by hook",
		}
	}

	// Execute PostToolUse hooks if hook manager exists
	if e.hookManager != nil {
		if execErr != nil {
			// Execute PostToolUseFailure for errors
			e.hookManager.ExecutePostToolUseFailure(ctx, name, modifiedInput, execErr.Error())
		} else {
			results := e.hookManager.ExecutePostToolUse(ctx, name, modifiedInput, output)

			for _, result := range results {
				if result.Output != nil {
					if result.Output.Decision == "modify" && result.Output.Stdout != "" {
						output = map[string]any{
							"modified": true,
							"result":   result.Output.Stdout,
						}
					}
				}
			}
		}
	}

	return output, execErr
}

// Execute runs a tool (without hooks, for backward compatibility)
func (e *HookAwareExecutor) Execute(ctx context.Context, name string, input map[string]any) (any, error) {
	return e.ExecuteWithHooks(ctx, name, input)
}

// ExecuteWithoutHooks runs a tool bypassing all hooks
func (e *HookAwareExecutor) ExecuteWithoutHooks(ctx context.Context, name string, input map[string]any) (any, error) {
	return e.registry.Execute(ctx, name, input)
}

// GetRegistry returns the underlying tool registry
func (e *HookAwareExecutor) GetRegistry() *ToolRegistry {
	return e.registry
}

// GetHookManager returns the hook manager
func (e *HookAwareExecutor) GetHookManager() *hooks.HookManager {
	return e.hookManager
}

// SetHookManager updates the hook manager
func (e *HookAwareExecutor) SetHookManager(hookManager *hooks.HookManager) {
	e.hookManager = hookManager
}

// ToolInfo returns information about a tool
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ListTools returns all registered tools
func (e *HookAwareExecutor) ListTools() []ToolInfo {
	tools := e.registry.All()
	result := make([]ToolInfo, len(tools))
	for i, t := range tools {
		result[i] = ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		}
	}
	return result
}

// Global hook-aware executor for convenience
var defaultHookAwareExecutor *HookAwareExecutor

// InitHookAwareExecutor initializes the global hook-aware executor
func InitHookAwareExecutor(hookManager *hooks.HookManager) {
	defaultHookAwareExecutor = NewHookAwareExecutor(GetRegistry(), hookManager)
}

// GetHookAwareExecutor returns the global hook-aware executor
func GetHookAwareExecutor() *HookAwareExecutor {
	if defaultHookAwareExecutor == nil {
		// Initialize without hooks if not already set
		defaultHookAwareExecutor = NewHookAwareExecutor(GetRegistry(), nil)
	}
	return defaultHookAwareExecutor
}

// ExecuteWithGlobalHooks executes a tool using the global hook-aware executor
func ExecuteWithGlobalHooks(ctx context.Context, name string, input map[string]any) (any, error) {
	return GetHookAwareExecutor().ExecuteWithHooks(ctx, name, input)
}
