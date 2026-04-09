package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitStatus struct {
	Branch      string   `json:"branch"`
	Ahead       int      `json:"ahead"`
	Behind      int      `json:"behind"`
	Staged      []string `json:"staged"`
	Unstaged    []string `json:"unstaged"`
	Untracked   []string `json:"untracked"`
	Conflicted  []string `json:"conflicted"`
	IsRepo      bool     `json:"is_repo"`
	HasUpstream bool     `json:"has_upstream"`
	WorkingDir  string   `json:"working_dir"`
}

type GitCommit struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

type GitBranch struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
	IsRemote  bool   `json:"is_remote"`
	Upstream  string `json:"upstream"`
}

type GitManager struct {
	workDir string
}

func NewGitManager(workDir string) *GitManager {
	return &GitManager{workDir: workDir}
}

func (gm *GitManager) IsGitRepo() bool {
	gitDir := filepath.Join(gm.workDir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (gm *GitManager) runGit(args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = gm.workDir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (gm *GitManager) GetStatus() (*GitStatus, error) {
	if !gm.IsGitRepo() {
		return &GitStatus{IsRepo: false}, nil
	}

	status := &GitStatus{
		IsRepo:     true,
		WorkingDir: gm.workDir,
		Staged:     []string{},
		Unstaged:   []string{},
		Untracked:  []string{},
		Conflicted: []string{},
	}

	// Get branch
	branch, _, err := gm.runGit("rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		status.Branch = strings.TrimSpace(branch)
	}

	// Get ahead/behind
	status.HasUpstream = false
	upstream, _, err := gm.runGit("rev-parse", "--abbrev-ref", "@{upstream}")
	if err == nil && upstream != "" {
		status.HasUpstream = true
		upstream = strings.TrimSpace(upstream)

		// Ahead
		ahead, _, _ := gm.runGit("rev-list", "--count", upstream+"..HEAD")
		fmt.Sscanf(strings.TrimSpace(ahead), "%d", &status.Ahead)

		// Behind
		behind, _, _ := gm.runGit("rev-list", "--count", "HEAD.."+upstream)
		fmt.Sscanf(strings.TrimSpace(behind), "%d", &status.Behind)
	}

	// Get status
	output, _, err := gm.runGit("status", "--porcelain")
	if err != nil {
		return status, err
	}

	for _, line := range strings.Split(output, "\n") {
		if len(line) < 2 {
			continue
		}

		code := line[:2]
		file := strings.TrimSpace(line[3:])

		switch {
		case strings.Contains(code, "UU"):
			status.Conflicted = append(status.Conflicted, file)
		case code[0] != ' ' && code[0] != '?':
			status.Staged = append(status.Staged, file)
		case code[1] != ' ':
			status.Unstaged = append(status.Unstaged, file)
		case code[0] == '?':
			status.Untracked = append(status.Untracked, file)
		}
	}

	return status, nil
}

func (gm *GitManager) GetBranches() ([]GitBranch, error) {
	if !gm.IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	var branches []GitBranch

	// Get current branch
	currentBranch, _, _ := gm.runGit("rev-parse", "--abbrev-ref", "HEAD")
	currentBranch = strings.TrimSpace(currentBranch)

	// Get local branches
	output, _, err := gm.runGit("branch", "--list")
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		isCurrent := strings.HasPrefix(line, "* ")
		name := strings.TrimPrefix(line, "* ")
		name = strings.TrimSpace(name)

		if name != "" {
			branches = append(branches, GitBranch{
				Name:      name,
				IsCurrent: isCurrent,
				IsRemote:  false,
			})
		}
	}

	// Get remote branches
	output, _, _ = gm.runGit("branch", "-r")
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "->") {
			continue
		}

		branches = append(branches, GitBranch{
			Name:      line,
			IsCurrent: false,
			IsRemote:  true,
		})
	}

	return branches, nil
}

func (gm *GitManager) GetLog(limit int) ([]GitCommit, error) {
	if !gm.IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	if limit == 0 {
		limit = 20
	}

	output, _, err := gm.runGit("log", fmt.Sprintf("-%d", limit), "--pretty=format:%H|%an|%ad|%s", "--date=short")
	if err != nil {
		return nil, err
	}

	var commits []GitCommit
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			commits = append(commits, GitCommit{
				Hash:    parts[0][:7],
				Author:  parts[1],
				Date:    parts[2],
				Message: parts[3],
			})
		}
	}

	return commits, nil
}

func (gm *GitManager) Diff(cached bool) (string, error) {
	if !gm.IsGitRepo() {
		return "", fmt.Errorf("not a git repository")
	}

	args := []string{"diff"}
	if cached {
		args = append(args, "--cached")
	}

	diff, _, err := gm.runGit(args...)
	return diff, err
}

func (gm *GitManager) Add(files []string) error {
	if !gm.IsGitRepo() {
		return fmt.Errorf("not a git repository")
	}

	args := append([]string{"add"}, files...)
	_, _, err := gm.runGit(args...)
	return err
}

func (gm *GitManager) Commit(message string) error {
	if !gm.IsGitRepo() {
		return fmt.Errorf("not a git repository")
	}

	_, _, err := gm.runGit("commit", "-m", message)
	return err
}

func (gm *GitManager) Push(setUpstream bool, branch string) error {
	if !gm.IsGitRepo() {
		return fmt.Errorf("not a git repository")
	}

	args := []string{"push"}
	if setUpstream {
		args = append(args, "-u", "origin", branch)
	}

	_, _, err := gm.runGit(args...)
	return err
}

func (gm *GitManager) Pull() error {
	if !gm.IsGitRepo() {
		return fmt.Errorf("not a git repository")
	}

	_, _, err := gm.runGit("pull")
	return err
}

func (gm *GitManager) Checkout(branch string) error {
	if !gm.IsGitRepo() {
		return fmt.Errorf("not a git repository")
	}

	_, _, err := gm.runGit("checkout", branch)
	return err
}

func (gm *GitManager) CreateBranch(name string) error {
	if !gm.IsGitRepo() {
		return fmt.Errorf("not a git repository")
	}

	_, _, err := gm.runGit("checkout", "-b", name)
	return err
}

func (gm *GitManager) FormatStatus(status *GitStatus) string {
	if !status.IsRepo {
		return "❌ Not a git repository"
	}

	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("🌿 Git Status\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	sb.WriteString(fmt.Sprintf("📍 Branch: %s\n", status.Branch))

	if status.HasUpstream {
		if status.Ahead > 0 {
			sb.WriteString(fmt.Sprintf("  ↑ Ahead: %d commits\n", status.Ahead))
		}
		if status.Behind > 0 {
			sb.WriteString(fmt.Sprintf("  ↓ Behind: %d commits\n", status.Behind))
		}
	} else {
		sb.WriteString("  ⚠️  No upstream set\n")
	}

	if len(status.Staged) > 0 {
		sb.WriteString("\n✅ Staged:\n")
		for _, file := range status.Staged {
			sb.WriteString(fmt.Sprintf("   + %s\n", file))
		}
	}

	if len(status.Unstaged) > 0 {
		sb.WriteString("\n📝 Modified:\n")
		for _, file := range status.Unstaged {
			sb.WriteString(fmt.Sprintf("   M %s\n", file))
		}
	}

	if len(status.Untracked) > 0 {
		sb.WriteString("\n❓ Untracked:\n")
		for _, file := range status.Untracked {
			sb.WriteString(fmt.Sprintf("   ? %s\n", file))
		}
	}

	if len(status.Conflicted) > 0 {
		sb.WriteString("\n⚠️  Conflicted:\n")
		for _, file := range status.Conflicted {
			sb.WriteString(fmt.Sprintf("   ! %s\n", file))
		}
	}

	if len(status.Staged) == 0 && len(status.Unstaged) == 0 && len(status.Untracked) == 0 {
		sb.WriteString("\n✨ Working tree clean\n")
	}

	return sb.String()
}

func (gm *GitManager) FormatLog(commits []GitCommit) string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📜 Git Log\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	for _, commit := range commits {
		sb.WriteString(fmt.Sprintf("%s %s\n", commit.Hash, commit.Message))
		sb.WriteString(fmt.Sprintf("   by %s on %s\n\n", commit.Author, commit.Date))
	}

	return sb.String()
}

func (gm *GitManager) FormatBranches(branches []GitBranch) string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("🌿 Branches\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	sb.WriteString("Local:\n")
	for _, branch := range branches {
		if !branch.IsRemote {
			current := ""
			if branch.IsCurrent {
				current = " ✓"
			}
			sb.WriteString(fmt.Sprintf("  %s%s\n", branch.Name, current))
		}
	}

	sb.WriteString("\nRemote:\n")
	for _, branch := range branches {
		if branch.IsRemote {
			sb.WriteString(fmt.Sprintf("  %s\n", branch.Name))
		}
	}

	return sb.String()
}
