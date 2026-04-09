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
)

// Executor executes tools
type Executor struct {
	WorkDir string
	Timeout time.Duration
}

// NewExecutor creates a new tool executor
func NewExecutor(workDir string) *Executor {
	return &Executor{
		WorkDir: workDir,
		Timeout: 120 * time.Second,
	}
}

// Execute runs a tool by name
func (e *Executor) Execute(ctx context.Context, name string, input map[string]interface{}) (interface{}, error) {
	switch name {
	case "bash":
		return e.executeBash(ctx, input)
	case "read_file":
		return e.readFile(ctx, input)
	case "write_file":
		return e.writeFile(ctx, input)
	case "edit_file":
		return e.editFile(ctx, input)
	case "glob":
		return e.glob(ctx, input)
	case "grep":
		return e.grep(ctx, input)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// BashOutput is the output of a bash command
type BashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func (e *Executor) executeBash(ctx context.Context, input map[string]interface{}) (*BashOutput, error) {
	command, ok := input["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command is required")
	}

	timeout := e.Timeout
	if t, ok := input["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = e.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return &BashOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// FileOutput is the output of a file operation
type FileOutput struct {
	Content string `json:"content,omitempty"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists,omitempty"`
}

func (e *Executor) readFile(ctx context.Context, input map[string]interface{}) (*FileOutput, error) {
	path, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	// Make path absolute if relative
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.WorkDir, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &FileOutput{
		Content: string(content),
		Path:    path,
	}, nil
}

func (e *Executor) writeFile(ctx context.Context, input map[string]interface{}) (*FileOutput, error) {
	path, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	content, ok := input["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required")
	}

	// Make path absolute if relative
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.WorkDir, path)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &FileOutput{
		Path:   path,
		Exists: true,
	}, nil
}

func (e *Executor) editFile(ctx context.Context, input map[string]interface{}) (*FileOutput, error) {
	// TODO: Implement file editing with string replacement
	return nil, fmt.Errorf("edit_file not yet implemented")
}

// GlobOutput is the output of a glob search
type GlobOutput struct {
	Files []string `json:"files"`
}

func (e *Executor) glob(ctx context.Context, input map[string]interface{}) (*GlobOutput, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern is required")
	}

	matches, err := filepath.Glob(filepath.Join(e.WorkDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to glob: %w", err)
	}

	// Make paths relative to workdir
	relMatches := make([]string, len(matches))
	for i, match := range matches {
		rel, err := filepath.Rel(e.WorkDir, match)
		if err != nil {
			relMatches[i] = match
		} else {
			relMatches[i] = rel
		}
	}

	return &GlobOutput{
		Files: relMatches,
	}, nil
}

// GrepOutput is the output of a grep search
type GrepOutput struct {
	Matches []GrepMatch `json:"matches"`
}

// GrepMatch represents a single grep match
type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func (e *Executor) grep(ctx context.Context, input map[string]interface{}) (*GrepOutput, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern is required")
	}

	// TODO: Implement grep
	_ = pattern
	_ = ctx

	return &GrepOutput{
		Matches: []GrepMatch{},
	}, nil
}

// Glob is a helper function for glob search
func Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// ReadFile is a helper function for reading files
func ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// WriteFile is a helper function for writing files
func WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// RunCommand runs a shell command
func RunCommand(command string, workDir string) (string, string, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir checks if a path is a directory
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir ensures a directory exists
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// ExpandHome expands ~ to home directory
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
