package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// BlameInfo represents a single line's blame information
type BlameInfo struct {
	Commit  string // abbreviated commit hash
	Author  string
	Date    string
	Line    int
	Content string
}

// FileLogEntry represents a commit that touched a file
type FileLogEntry struct {
	Hash    string
	Author  string
	Date    string
	Subject string
}

// GetBlame returns blame information for a specific file.
// maxLines limits the number of lines returned (0 = no limit).
func GetBlame(dir string, file string, maxLines int) ([]BlameInfo, error) {
	args := []string{"blame", "--porcelain", file}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame failed: %w", err)
	}

	return ParseBlamePorcelain(string(output), maxLines), nil
}

// GetFileLog returns commit history for a specific file.
// count limits the number of log entries returned.
func GetFileLog(dir string, file string, count int) ([]FileLogEntry, error) {
	args := []string{"log", "--follow", "--format=%h|%an|%ai|%s", fmt.Sprintf("-%d", count), "--", file}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return ParseFileLog(string(output)), nil
}

// ParseBlamePorcelain parses git blame --porcelain output into structured BlameInfo.
func ParseBlamePorcelain(output string, maxLines int) []BlameInfo {
	var results []BlameInfo
	lines := strings.Split(output, "\n")

	var current BlameInfo
	inChunk := false

	for _, line := range lines {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "\t") {
			current.Content = strings.TrimPrefix(line, "\t")
			if maxLines == 0 || len(results) < maxLines {
				results = append(results, current)
			}
			current = BlameInfo{}
			inChunk = false
			continue
		}

		if !inChunk {
			parts := strings.Fields(line)
			if len(parts) >= 1 && len(parts[0]) >= 7 {
				current.Commit = parts[0]
				if len(parts) >= 3 {
					if n, err := strconv.Atoi(parts[2]); err == nil {
						current.Line = n
					}
				}
				inChunk = true
			}
			continue
		}

		if strings.HasPrefix(line, "author ") {
			current.Author = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-time ") {
			current.Date = strings.TrimPrefix(line, "author-time ")
		}
	}

	return results
}

// ParseFileLog parses git log --format="%h|%an|%ai|%s" output into FileLogEntry slice.
func ParseFileLog(output string) []FileLogEntry {
	var entries []FileLogEntry
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, FileLogEntry{
			Hash:    strings.TrimSpace(parts[0]),
			Author:  strings.TrimSpace(parts[1]),
			Date:    strings.TrimSpace(parts[2]),
			Subject: strings.TrimSpace(parts[3]),
		})
	}

	return entries
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
