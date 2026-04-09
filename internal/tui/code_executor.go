package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Language string

const (
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageGo         Language = "go"
	LanguageBash       Language = "bash"
	LanguageShell      Language = "shell"
)

type ExecutionResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out"`
	Language string `json:"language"`
	Duration int64  `json:"duration_ms"`
	TempFile string `json:"temp_file,omitempty"`
}

type CodeExecutor struct {
	WorkDir      string
	Timeout      time.Duration
	MaxOutputLen int
}

func NewCodeExecutor(workDir string) *CodeExecutor {
	return &CodeExecutor{
		WorkDir:      workDir,
		Timeout:      10 * time.Second,
		MaxOutputLen: 10000,
	}
}

func (ce *CodeExecutor) DetectLanguage(code string) Language {
	if strings.HasPrefix(strings.TrimSpace(code), "#!") {
		firstLine := strings.Split(code, "\n")[0]
		if strings.Contains(firstLine, "python") {
			return LanguagePython
		}
		if strings.Contains(firstLine, "node") || strings.Contains(firstLine, "bash") {
			return LanguageBash
		}
	}

	if strings.Contains(code, "def ") || strings.Contains(code, "import ") || strings.Contains(code, "print(") {
		return LanguagePython
	}

	if strings.Contains(code, "func main()") || strings.Contains(code, "package main") {
		return LanguageGo
	}

	if strings.Contains(code, "function ") || strings.Contains(code, "const ") || strings.Contains(code, "console.log") {
		return LanguageJavaScript
	}

	return LanguageBash
}

func (ce *CodeExecutor) Execute(ctx context.Context, language Language, code string) (*ExecutionResult, error) {
	switch language {
	case LanguagePython:
		return ce.executePython(ctx, code)
	case LanguageJavaScript:
		return ce.executeJavaScript(ctx, code)
	case LanguageGo:
		return ce.executeGo(ctx, code)
	case LanguageBash, LanguageShell:
		return ce.executeBash(ctx, code)
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

func (ce *CodeExecutor) executePython(ctx context.Context, code string) (*ExecutionResult, error) {
	if !ce.isCommandAvailable("python3") && !ce.isCommandAvailable("python") {
		return nil, fmt.Errorf("Python is not installed. Please install Python 3")
	}

	pythonCmd := "python3"
	if !ce.isCommandAvailable("python3") {
		pythonCmd = "python"
	}

	tmpFile, err := ce.createTempFile(code, ".py")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	return ce.runCommand(ctx, pythonCmd, tmpFile)
}

func (ce *CodeExecutor) executeJavaScript(ctx context.Context, code string) (*ExecutionResult, error) {
	if !ce.isCommandAvailable("node") {
		return nil, fmt.Errorf("Node.js is not installed. Please install Node.js")
	}

	tmpFile, err := ce.createTempFile(code, ".js")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	return ce.runCommand(ctx, "node", tmpFile)
}

func (ce *CodeExecutor) executeGo(ctx context.Context, code string) (*ExecutionResult, error) {
	if !ce.isCommandAvailable("go") {
		return nil, fmt.Errorf("Go is not installed. Please install Go")
	}

	tmpFile, err := ce.createTempFile(code, ".go")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	return ce.runCommand(ctx, "go", "run", tmpFile)
}

func (ce *CodeExecutor) executeBash(ctx context.Context, code string) (*ExecutionResult, error) {
	if !ce.isCommandAvailable("bash") {
		return nil, fmt.Errorf("Bash is not installed")
	}

	return ce.runCommandWithInput(ctx, "bash", "-c", code)
}

func (ce *CodeExecutor) runCommand(ctx context.Context, name string, args ...string) (*ExecutionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, ce.Timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = ce.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	timedOut := false
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			timedOut = true
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	stdoutStr := ce.truncateOutput(stdout.String())
	stderrStr := ce.truncateOutput(stderr.String())

	return &ExecutionResult{
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		ExitCode: exitCode,
		TimedOut: timedOut,
		Language: name,
		Duration: duration,
	}, nil
}

func (ce *CodeExecutor) runCommandWithInput(ctx context.Context, name string, args ...string) (*ExecutionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, ce.Timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = ce.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	timedOut := false
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			timedOut = true
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	stdoutStr := ce.truncateOutput(stdout.String())
	stderrStr := ce.truncateOutput(stderr.String())

	return &ExecutionResult{
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		ExitCode: exitCode,
		TimedOut: timedOut,
		Language: name,
		Duration: duration,
	}, nil
}

func (ce *CodeExecutor) createTempFile(content string, ext string) (string, error) {
	tmpDir := filepath.Join(ce.WorkDir, ".smartclaw_tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(tmpDir, "code_*"+ext)
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func (ce *CodeExecutor) isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func (ce *CodeExecutor) truncateOutput(output string) string {
	if len(output) > ce.MaxOutputLen {
		return output[:ce.MaxOutputLen] + "\n... (output truncated)"
	}
	return output
}

func (ce *CodeExecutor) FormatResult(result *ExecutionResult) string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("📦 Language: %s\n", result.Language))
	sb.WriteString(fmt.Sprintf("⏱️  Duration: %dms\n", result.Duration))

	if result.TimedOut {
		sb.WriteString("⏰ Status: TIMED OUT\n")
	} else if result.ExitCode != 0 {
		sb.WriteString(fmt.Sprintf("❌ Exit Code: %d\n", result.ExitCode))
	} else {
		sb.WriteString("✅ Status: SUCCESS\n")
	}

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if result.Stdout != "" {
		sb.WriteString("\n📤 Output:\n")
		sb.WriteString(result.Stdout)
	}

	if result.Stderr != "" {
		sb.WriteString("\n⚠️  Error:\n")
		sb.WriteString(result.Stderr)
	}

	return sb.String()
}
