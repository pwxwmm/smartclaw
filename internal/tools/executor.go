package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
func (e *Executor) Execute(ctx context.Context, name string, input map[string]any) (any, error) {
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

func (e *Executor) executeBash(ctx context.Context, input map[string]any) (*BashOutput, error) {
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

	if validationResult := ValidateCommandSecurity(command); !validationResult.Allowed {
		return nil, fmt.Errorf("command rejected by security policy: %s", validationResult.Reason)
	}

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

func (e *Executor) readFile(ctx context.Context, input map[string]any) (*FileOutput, error) {
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

func (e *Executor) writeFile(ctx context.Context, input map[string]any) (*FileOutput, error) {
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

func (e *Executor) editFile(ctx context.Context, input map[string]any) (*FileOutput, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return nil, ErrRequiredField("path")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(e.WorkDir, path)
	}

	operation, _ := input["operation"].(string)
	if operation == "" {
		operation = "replace"
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	original := string(content)

	var result string
	switch operation {
	case "replace":
		oldText, _ := input["old_text"].(string)
		newText, _ := input["new_text"].(string)
		if oldText == "" {
			return nil, ErrRequiredField("old_text")
		}
		count := strings.Count(original, oldText)
		if count == 0 {
			return nil, fmt.Errorf("old_text not found in file")
		}
		replaceAll := false
		if r, ok := input["replace_all"].(bool); ok {
			replaceAll = r
		}
		if count > 1 && !replaceAll {
			return nil, fmt.Errorf("old_text found %d times; set replace_all=true to replace all", count)
		}
		if replaceAll {
			result = strings.ReplaceAll(original, oldText, newText)
		} else {
			result = strings.Replace(original, oldText, newText, 1)
		}

	case "insert":
		lineNum := 0
		if ln, ok := input["line"].(int); ok {
			lineNum = ln
		} else if ln, ok := input["line"].(float64); ok {
			lineNum = int(ln)
		}
		newText, _ := input["new_text"].(string)
		if newText == "" {
			return nil, ErrRequiredField("new_text")
		}
		lines := strings.Split(original, "\n")
		if lineNum < 0 {
			lineNum = 0
		}
		if lineNum > len(lines) {
			lineNum = len(lines)
		}
		newLines := strings.Split(newText, "\n")
		rear := make([]string, len(lines[lineNum:]))
		copy(rear, lines[lineNum:])
		lines = append(lines[:lineNum], newLines...)
		lines = append(lines, rear...)
		result = strings.Join(lines, "\n")

	case "delete":
		lineStart := 0
		lineEnd := 0
		if ls, ok := input["line_start"].(int); ok {
			lineStart = ls
		} else if ls, ok := input["line_start"].(float64); ok {
			lineStart = int(ls)
		}
		if le, ok := input["line_end"].(int); ok {
			lineEnd = le
		} else if le, ok := input["line_end"].(float64); ok {
			lineEnd = int(le)
		}
		if lineEnd <= lineStart {
			lineEnd = lineStart + 1
		}
		lines := strings.Split(original, "\n")
		if lineStart < 0 {
			lineStart = 0
		}
		if lineEnd > len(lines) {
			lineEnd = len(lines)
		}
		result = strings.Join(append(lines[:lineStart], lines[lineEnd:]...), "\n")

	default:
		return nil, fmt.Errorf("unknown operation: %s (valid: replace, insert, delete)", operation)
	}

	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &FileOutput{
		Path:    path,
		Content: result,
		Exists:  true,
	}, nil
}

// GlobOutput is the output of a glob search
type GlobOutput struct {
	Files []string `json:"files"`
}

func (e *Executor) glob(ctx context.Context, input map[string]any) (*GlobOutput, error) {
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

func (e *Executor) grep(ctx context.Context, input map[string]any) (*GrepOutput, error) {
	pattern, ok := input["pattern"].(string)
	if !ok || pattern == "" {
		return nil, ErrRequiredField("pattern")
	}

	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}

	includePattern, _ := input["include"].(string)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var matches []GrepMatch
	maxMatches := 50

	err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "bin" || name == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}
		if len(matches) >= maxMatches {
			return nil
		}

		if includePattern != "" {
			matched, _ := filepath.Match(includePattern, d.Name())
			if !matched {
				return nil
			}
		}

		file, err := os.Open(filePath)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if len(matches) >= maxMatches {
				return nil
			}
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, GrepMatch{
					File:    filePath,
					Line:    lineNum,
					Content: line,
				})
			}
		}
		return nil
	})

	if err != nil && ctx.Err() == nil {
		return nil, fmt.Errorf("grep search failed: %w", err)
	}

	if matches == nil {
		matches = []GrepMatch{}
	}

	return &GrepOutput{Matches: matches}, nil
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
