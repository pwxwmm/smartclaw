package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/instructkr/smartclaw/internal/sandbox"
)

// ExecuteCodeTool runs code in an RPC sandbox where the code can call
// SmartClaw tools (read_file, write_file, glob, grep, bash, web_search, web_fetch)
// via Unix socket RPC. Only stdout is returned to the LLM — intermediate
// tool results stay inside the sandbox, collapsing multi-turn workflows
// into a single turn.
type ExecuteCodeTool struct{}

func (t *ExecuteCodeTool) Name() string { return "execute_code" }
func (t *ExecuteCodeTool) Description() string {
	return "Execute code in a sandboxed environment with access to SmartClaw tools via RPC. " +
		"The code can call tools like read_file, write_file, glob, grep, bash, web_search, web_fetch directly. " +
		"Only stdout output returns to the conversation — intermediate tool results stay in the sandbox. " +
		"Supported languages: python (default), go (not yet supported). " +
		"Resource limits: 300s timeout, 50 tool calls max, 100KB output max."
}

func (t *ExecuteCodeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "The code to execute. For Python, you can import smartclaw_tools to access read_file, write_file, glob, grep, bash, web_search, web_fetch functions directly.",
			},
			"language": map[string]any{
				"type":        "string",
				"default":     "python",
				"description": "Programming language: python or go",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"default":     300,
				"description": "Maximum execution time in seconds (max 300)",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Working directory for code execution (defaults to temp dir)",
			},
		},
		"required": []string{"code"},
	}
}

func (t *ExecuteCodeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	code, _ := input["code"].(string)
	if code == "" {
		return nil, ErrRequiredField("code")
	}

	language, _ := input["language"].(string)
	if language == "" {
		language = "python"
	}

	timeoutSec := 300
	if ts, ok := input["timeout"].(int); ok && ts > 0 && ts <= 300 {
		timeoutSec = ts
	}

	workdir, _ := input["workdir"].(string)

	cfg := sandbox.DefaultCodeSandboxConfig()
	cfg.Timeout = time.Duration(timeoutSec) * time.Second
	cfg.WorkDir = workdir

	sb := sandbox.NewCodeSandbox(cfg)

	toolHandler := func(tool string, toolInput map[string]any) (any, error) {
		return GetRegistry().Execute(ctx, tool, toolInput)
	}

	result, err := sb.Execute(ctx, code, language, toolHandler)
	if err != nil {
		return nil, fmt.Errorf("code execution failed: %w", err)
	}

	return map[string]any{
		"stdout":     result.Stdout,
		"exit_code":  result.ExitCode,
		"duration":   result.Duration.String(),
		"tool_calls": result.ToolCalls,
		"truncated":  result.Truncated,
		"timed_out":  result.TimedOut,
		"language":   language,
	}, nil
}
