package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/instructkr/smartclaw/internal/permissions"
)

func TestNewBashTool(t *testing.T) {
	tool := NewBashTool("/tmp")
	if tool.WorkDir != "/tmp" {
		t.Errorf("Expected WorkDir '/tmp', got '%s'", tool.WorkDir)
	}
	if tool.Timeout != 120000 {
		t.Errorf("Expected default timeout 120000, got %d", tool.Timeout)
	}
}

func TestBashToolExecuteEcho(t *testing.T) {
	tool := NewBashTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(*BashToolResult)
	if !ok {
		t.Fatalf("Expected *BashToolResult, got %T", result)
	}

	if !strings.Contains(resultMap.Stdout, "hello") {
		t.Errorf("Expected stdout to contain 'hello', got %q", resultMap.Stdout)
	}
	if resultMap.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", resultMap.ExitCode)
	}
}

func TestBashToolExecuteWithWorkdir(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewBashTool(tmpDir)
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "pwd",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(*BashToolResult)
	if !strings.Contains(resultMap.Stdout, tmpDir) {
		t.Errorf("Expected pwd to contain %q, got %q", tmpDir, resultMap.Stdout)
	}
}

func TestBashToolExecuteWithWorkdirOverride(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewBashTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "pwd",
		"workdir": tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(*BashToolResult)
	if !strings.Contains(resultMap.Stdout, tmpDir) {
		t.Errorf("Expected pwd to contain %q, got %q", tmpDir, resultMap.Stdout)
	}
}

func TestBashToolExecuteInvalidCommand(t *testing.T) {
	tool := NewBashTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "nonexistent_command_xyz123",
	})
	if err != nil {
		t.Fatalf("Execute should not return error for command failures: %v", err)
	}

	resultMap := result.(*BashToolResult)
	if resultMap.ExitCode == 0 {
		t.Error("Expected non-zero exit code for invalid command")
	}
}

func TestBashToolExecuteMissingCommand(t *testing.T) {
	tool := NewBashTool("")
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing command")
	}
}

func TestBashToolExecuteEmptyCommand(t *testing.T) {
	tool := NewBashTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"command": "",
	})
	if err == nil {
		t.Error("Expected error for empty command")
	}
}

func TestBashToolExecuteTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	tool := NewBashTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "sleep 60",
		"timeout": 500,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(*BashToolResult)
	if !resultMap.TimedOut {
		t.Error("Expected TimedOut=true for command exceeding timeout")
	}
	if !resultMap.Interrupted {
		t.Error("Expected Interrupted=true for timed out command")
	}
}

func TestBashToolExecuteTimeoutCapped(t *testing.T) {
	tool := NewBashTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo test",
		"timeout": 700000,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(*BashToolResult)
	if resultMap.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", resultMap.ExitCode)
	}
}

func TestBashToolExecuteStderr(t *testing.T) {
	tool := NewBashTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo error_msg >&2 && exit 1",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(*BashToolResult)
	if resultMap.ExitCode == 0 {
		t.Error("Expected non-zero exit code")
	}
	if !strings.Contains(resultMap.Stderr, "error_msg") {
		t.Errorf("Expected stderr to contain 'error_msg', got %q", resultMap.Stderr)
	}
}

func TestBashToolCheckPermissionsRead(t *testing.T) {
	tool := &BashTool{}
	result := tool.CheckPermissions("cat file.txt", nil)
	if result == nil || *result != permissions.PermissionAllow {
		t.Error("Read command should have Allow permission")
	}
}

func TestBashToolCheckPermissionsSearch(t *testing.T) {
	tool := &BashTool{}
	result := tool.CheckPermissions("grep pattern file.go", nil)
	if result == nil || *result != permissions.PermissionAllow {
		t.Error("Search command should have Allow permission")
	}
}

func TestBashToolCheckPermissionsDestructive(t *testing.T) {
	tool := &BashTool{}
	result := tool.CheckPermissions("rm -rf /tmp/test", nil)
	if result == nil || *result != permissions.PermissionAsk {
		t.Error("Destructive command should have Ask permission")
	}
}

func TestBashToolCheckPermissionsGit(t *testing.T) {
	tool := &BashTool{}
	result := tool.CheckPermissions("git status", nil)
	if result == nil || *result != permissions.PermissionAsk {
		t.Error("Git command should have Ask permission")
	}
}

func TestBashToolGetTimeoutForCommand(t *testing.T) {
	tool := &BashTool{Timeout: 120000}

	searchTimeout := tool.GetTimeoutForCommand("grep pattern file")
	if searchTimeout != 60000 {
		t.Errorf("Expected 60000 for search command, got %d", searchTimeout)
	}

	gitTimeout := tool.GetTimeoutForCommand("git status")
	if gitTimeout != 30000 {
		t.Errorf("Expected 30000 for git command, got %d", gitTimeout)
	}

	defaultTimeout := tool.GetTimeoutForCommand("echo hello")
	if defaultTimeout != 120000 {
		t.Errorf("Expected 120000 for default command, got %d", defaultTimeout)
	}
}

func TestBashToolValidateWorkingDir(t *testing.T) {
	tool := &BashTool{}

	if err := tool.ValidateWorkingDir(""); err != nil {
		t.Error("Empty workdir should be valid")
	}

	if err := tool.ValidateWorkingDir("/nonexistent/path/xyz"); err == nil {
		t.Error("Non-existent workdir should be invalid")
	}

	tmpDir := t.TempDir()
	if err := tool.ValidateWorkingDir(tmpDir); err != nil {
		t.Errorf("Existing dir should be valid: %v", err)
	}

	tmpFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(tmpFile, []byte("test"), 0644)
	if err := tool.ValidateWorkingDir(tmpFile); err == nil {
		t.Error("File path should not be valid as workdir")
	}
}

func TestBashToolExpandEnvVars(t *testing.T) {
	tool := &BashTool{}
	os.Setenv("TEST_BASH_VAR", "expanded")
	defer os.Unsetenv("TEST_BASH_VAR")

	result := tool.ExpandEnvVars("echo $TEST_BASH_VAR")
	if !strings.Contains(result, "expanded") {
		t.Errorf("Expected expanded var, got %q", result)
	}
}

func TestBashToolSanitizeOutput(t *testing.T) {
	tool := &BashTool{}

	normalOutput := tool.SanitizeOutput("line1\r\nline2\r\n")
	if strings.Contains(normalOutput, "\r\n") {
		t.Error("Should convert CRLF to LF")
	}

	longOutput := strings.Repeat("line\n", 2000)
	sanitized := tool.SanitizeOutput(longOutput)
	if strings.Count(sanitized, "\n") >= 2000 {
		t.Error("Should truncate long output")
	}
}

func TestBashToolBackgroundExecution(t *testing.T) {
	tool := NewBashTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"command":          "echo background",
		"run_in_background": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(*BashToolResult)
	if resultMap.BackgroundTaskID == "" {
		t.Error("Expected background task ID")
	}
}
