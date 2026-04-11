package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/permissions"
)

type BashTool struct {
	WorkDir string
	Timeout int
}

func NewBashTool(workDir string) *BashTool {
	return &BashTool{
		WorkDir: workDir,
		Timeout: 120000,
	}
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Execute a shell command in the current workspace" }

func (t *BashTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The command to execute",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in milliseconds (max 600000)",
				"maximum":     600000,
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Clear, concise description of what this command does",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Working directory for command execution",
			},
			"run_in_background": map[string]any{
				"type":        "boolean",
				"description": "Set to true to run this command in the background",
			},
		},
		"required": []string{"command"},
	}
}

type BashToolResult struct {
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	ExitCode         int    `json:"exit_code"`
	Interrupted      bool   `json:"interrupted"`
	BackgroundTaskID string `json:"background_task_id,omitempty"`
	TimedOut         bool   `json:"timed_out,omitempty"`
	WorkingDir       string `json:"working_dir,omitempty"`
}

func (t *BashTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	cmdStr, _ := input["command"].(string)
	if cmdStr == "" {
		return nil, ErrRequiredField("command")
	}

	securityResult := ValidateCommandSecurity(cmdStr)
	if !securityResult.Allowed {
		return nil, &Error{
			Code:    securityResult.ErrorCode,
			Message: securityResult.Reason,
		}
	}

	workdir, _ := input["workdir"].(string)
	if workdir == "" {
		workdir = t.WorkDir
	}
	if workdir == "" {
		if wd, err := os.Getwd(); err == nil {
			workdir = wd
		}
	}

	pathResult := ValidatePathInCommand(cmdStr, workdir)
	if !pathResult.Allowed {
		return nil, &Error{
			Code:    pathResult.ErrorCode,
			Message: pathResult.Reason,
		}
	}

	timeout := t.Timeout
	if t, ok := input["timeout"].(int); ok && t > 0 {
		if t > 600000 {
			t = 600000
		}
		timeout = t
	}

	runInBackground, _ := input["run_in_background"].(bool)
	if runInBackground {
		return t.executeBackground(ctx, cmdStr, workdir, timeout)
	}

	return t.executeForeground(ctx, cmdStr, workdir, timeout)
}

func (t *BashTool) executeForeground(ctx context.Context, cmdStr, workdir string, timeout int) (*BashToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	interrupted := false
	timedOut := false

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}

		if ctx.Err() == context.DeadlineExceeded {
			timedOut = true
			interrupted = true
		}
	}

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	maxOutput := 50000
	if len(stdoutStr) > maxOutput {
		stdoutStr = stdoutStr[:maxOutput] + "\n... (output truncated)"
	}
	if len(stderrStr) > maxOutput {
		stderrStr = stderrStr[:maxOutput] + "\n... (output truncated)"
	}

	return &BashToolResult{
		Stdout:      stdoutStr,
		Stderr:      stderrStr,
		ExitCode:    exitCode,
		Interrupted: interrupted,
		TimedOut:    timedOut,
		WorkingDir:  workdir,
	}, nil
}

func (t *BashTool) executeBackground(ctx context.Context, cmdStr, workdir string, timeout int) (*BashToolResult, error) {
	taskID := generateTaskID()

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, &Error{Code: "BACKGROUND_START_ERROR", Message: err.Error()}
	}

	go func() {
		cmd.Wait()
	}()

	return &BashToolResult{
		Stdout:           "",
		Stderr:           "",
		ExitCode:         0,
		BackgroundTaskID: taskID,
		WorkingDir:       workdir,
	}, nil
}

func generateTaskID() string {
	return fmt.Sprintf("bg_%d", time.Now().UnixNano())
}

func (t *BashTool) CheckPermissions(command string, permissionEngine any) *permissions.PermissionResult {
	classification := ClassifyCommand(command)

	if classification.IsRead || classification.IsSearch || classification.IsList {
		r := permissions.PermissionAllow
		return &r
	}

	if IsDestructiveCommand(command) {
		r := permissions.PermissionAsk
		return &r
	}

	if IsGitCommand(command) {
		r := permissions.PermissionAsk
		return &r
	}

	r := permissions.PermissionAsk
	return &r
}

func (t *BashTool) GetTimeoutForCommand(command string) int {
	classification := ClassifyCommand(command)

	if classification.IsSearch || classification.IsList {
		return 60000
	}

	if IsGitCommand(command) {
		return 30000
	}

	return t.Timeout
}

func (t *BashTool) ValidateWorkingDir(workdir string) error {
	if workdir == "" {
		return nil
	}

	absPath, err := filepath.Abs(workdir)
	if err != nil {
		return &Error{Code: "INVALID_WORKDIR", Message: err.Error()}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return &Error{Code: "WORKDIR_NOT_FOUND", Message: "working directory does not exist"}
	}

	if !info.IsDir() {
		return &Error{Code: "WORKDIR_NOT_DIR", Message: "working directory is not a directory"}
	}

	return nil
}

func (t *BashTool) ExpandEnvVars(command string) string {
	return os.ExpandEnv(command)
}

func (t *BashTool) SanitizeOutput(output string) string {
	output = strings.ReplaceAll(output, "\r\n", "\n")

	lines := strings.Split(output, "\n")
	if len(lines) > 1000 {
		lines = append(lines[:500], append([]string{"... (output truncated)"}, lines[len(lines)-500:]...)...)
	}

	return strings.Join(lines, "\n")
}
