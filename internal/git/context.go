package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Context represents the git context of the workspace
type Context struct {
	RootDir      string
	Branch       string
	IsRepo       bool
	HasChanges   bool
	StagedFiles  []string
	ChangedFiles []string
}

// GetContext returns the git context for a directory
func GetContext(dir string) (*Context, error) {
	ctx := &Context{
		RootDir: dir,
	}

	// Check if it's a git repo
	if !isGitRepo(dir) {
		return ctx, nil
	}

	ctx.IsRepo = true

	// Get git root
	root, err := getGitRoot(dir)
	if err == nil {
		ctx.RootDir = root
	}

	// Get current branch
	branch, err := getBranch(dir)
	if err == nil {
		ctx.Branch = branch
	}

	// Check for changes
	ctx.HasChanges = hasChanges(dir)

	// Get staged and changed files
	ctx.StagedFiles, _ = getStagedFiles(dir)
	ctx.ChangedFiles, _ = getChangedFiles(dir)

	return ctx, nil
}

func isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func getGitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getBranch(dir string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func hasChanges(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(output) > 0
}

func getStagedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseLines(string(output)), nil
}

func getChangedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseLines(string(output)), nil
}

func parseLines(s string) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	result := []string{}
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// GetDiff returns the git diff
func GetDiff(dir string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w\n%s", err, stderr.String())
	}

	return stdout.String(), nil
}

// GetStatus returns the git status
func GetStatus(dir string) (string, error) {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// GetLog returns recent git commits
func GetLog(dir string, count int) (string, error) {
	cmd := exec.Command("git", "log", "--oneline", fmt.Sprintf("-%d", count))
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// String returns a human-readable context
func (c *Context) String() string {
	if !c.IsRepo {
		return "Not a git repository"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Git Context:\n"))
	sb.WriteString(fmt.Sprintf("  Root: %s\n", c.RootDir))
	sb.WriteString(fmt.Sprintf("  Branch: %s\n", c.Branch))
	sb.WriteString(fmt.Sprintf("  Has Changes: %v\n", c.HasChanges))

	if len(c.StagedFiles) > 0 {
		sb.WriteString(fmt.Sprintf("  Staged: %d files\n", len(c.StagedFiles)))
	}

	if len(c.ChangedFiles) > 0 {
		sb.WriteString(fmt.Sprintf("  Changed: %d files\n", len(c.ChangedFiles)))
	}

	return sb.String()
}
