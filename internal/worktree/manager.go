package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	branchPrefix = "smartclaw/"
	worktreeBase = ".smartclaw/worktrees"
)

type Worktree struct {
	Name      string
	Path      string
	Branch    string
	BaseRef   string
	CreatedAt time.Time
}

type WorktreeStatus struct {
	Name       string
	HasChanges bool
	Staged     int
	Unstaged   int
	Untracked  int
}

type Manager struct {
	repoRoot    string
	worktreeDir string
}

func NewManager(repoRoot string) *Manager {
	return &Manager{
		repoRoot:    repoRoot,
		worktreeDir: filepath.Join(repoRoot, worktreeBase),
	}
}

func (m *Manager) Create(ctx context.Context, name, ref string) (string, error) {
	wtPath := filepath.Join(m.worktreeDir, name)
	branch := branchPrefix + name

	if _, err := os.Stat(wtPath); err == nil {
		return "", fmt.Errorf("worktree %q already exists at %s", name, wtPath)
	}

	if _, err := runGit(ctx, m.repoRoot, "worktree", "add", wtPath, "-b", branch, ref); err != nil {
		return "", fmt.Errorf("create worktree %q: %w", name, err)
	}

	return wtPath, nil
}

func (m *Manager) Remove(ctx context.Context, name string) error {
	wtPath := filepath.Join(m.worktreeDir, name)
	branch := branchPrefix + name

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree %q not found at %s", name, wtPath)
	}

	if _, err := runGit(ctx, m.repoRoot, "worktree", "remove", wtPath, "--force"); err != nil {
		return fmt.Errorf("remove worktree %q: %w", name, err)
	}

	if _, err := runGit(ctx, m.repoRoot, "branch", "-D", branch); err != nil {
		return fmt.Errorf("delete branch %q: %w", branch, err)
	}

	return nil
}

func (m *Manager) List(ctx context.Context) ([]*Worktree, error) {
	output, err := runGit(ctx, m.repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	return m.parseWorktreeList(output)
}

func (m *Manager) Get(ctx context.Context, name string) (*Worktree, error) {
	worktrees, err := m.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		if wt.Name == name {
			return wt, nil
		}
	}

	return nil, fmt.Errorf("worktree %q not found", name)
}

func (m *Manager) Merge(ctx context.Context, name, strategy string) error {
	branch := branchPrefix + name

	mainBranch, err := m.resolveMainBranch(ctx)
	if err != nil {
		return err
	}

	if _, err := runGit(ctx, m.repoRoot, "checkout", mainBranch); err != nil {
		return fmt.Errorf("checkout %q: %w", mainBranch, err)
	}

	switch strategy {
	case "squash":
		if _, err := runGit(ctx, m.repoRoot, "merge", "--squash", branch); err != nil {
			return fmt.Errorf("squash merge %q: %w", branch, err)
		}
	case "rebase":
		if _, err := runGit(ctx, m.repoRoot, "rebase", branch); err != nil {
			return fmt.Errorf("rebase %q: %w", branch, err)
		}
	default:
		if _, err := runGit(ctx, m.repoRoot, "merge", branch); err != nil {
			return fmt.Errorf("merge %q: %w", branch, err)
		}
	}

	return nil
}

func (m *Manager) Diff(ctx context.Context, name string) ([]byte, error) {
	branch := branchPrefix + name

	baseRef, err := m.mergeBase(ctx, branch)
	if err != nil {
		return nil, fmt.Errorf("find merge base for %q: %w", branch, err)
	}

	output, err := runGit(ctx, m.repoRoot, "diff", baseRef+".."+branch)
	if err != nil {
		return nil, fmt.Errorf("diff %s..%s: %w", baseRef, branch, err)
	}

	return []byte(output), nil
}

func (m *Manager) Status(ctx context.Context, name string) (*WorktreeStatus, error) {
	wtPath := filepath.Join(m.worktreeDir, name)

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("worktree %q not found at %s", name, wtPath)
	}

	output, err := runGit(ctx, wtPath, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("status worktree %q: %w", name, err)
	}

	status := &WorktreeStatus{Name: name}

	if output == "" {
		return status, nil
	}

	for line := range strings.SplitSeq(output, "\n") {
		if len(line) < 3 {
			continue
		}

		x := line[0] // staging area indicator
		y := line[1] // working tree indicator

		if x == '?' && y == '?' {
			status.Untracked++
		} else {
			if x != ' ' && x != '?' {
				status.Staged++
			}
			if y != ' ' && y != '?' {
				status.Unstaged++
			}
		}
	}

	status.HasChanges = status.Staged > 0 || status.Unstaged > 0 || status.Untracked > 0

	return status, nil
}

func (m *Manager) HasChanges(ctx context.Context, name string) (bool, error) {
	status, err := m.Status(ctx, name)
	if err != nil {
		return false, err
	}
	return status.HasChanges, nil
}

func (m *Manager) Commit(ctx context.Context, name, message string) error {
	wtPath := filepath.Join(m.worktreeDir, name)

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree %q not found at %s", name, wtPath)
	}

	if _, err := runGit(ctx, wtPath, "add", "-A"); err != nil {
		return fmt.Errorf("stage changes in worktree %q: %w", name, err)
	}

	if _, err := runGit(ctx, wtPath, "commit", "-m", message); err != nil {
		return fmt.Errorf("commit in worktree %q: %w", name, err)
	}

	return nil
}

func (m *Manager) resolveMainBranch(ctx context.Context) (string, error) {
	output, err := runGit(ctx, m.repoRoot, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil && strings.Contains(output, "main") {
		return "main", nil
	}

	if _, err := runGit(ctx, m.repoRoot, "rev-parse", "--verify", "main"); err == nil {
		return "main", nil
	}

	if _, err := runGit(ctx, m.repoRoot, "rev-parse", "--verify", "master"); err == nil {
		return "master", nil
	}

	return "", fmt.Errorf("cannot determine main branch")
}

func (m *Manager) mergeBase(ctx context.Context, branch string) (string, error) {
	mainBranch, err := m.resolveMainBranch(ctx)
	if err != nil {
		return "", err
	}

	output, err := runGit(ctx, m.repoRoot, "merge-base", mainBranch, branch)
	if err != nil {
		return "", err
	}

	return output, nil
}

func (m *Manager) parseWorktreeList(output string) ([]*Worktree, error) {
	var worktrees []*Worktree

	type parsedWorktree struct {
		path       string
		branch     string
		isBare     bool
		committish string
	}

	var current *parsedWorktree

	flush := func() {
		if current == nil || current.isBare || current.branch == "" {
			return
		}

		branchName, _ := strings.CutPrefix(current.branch, "refs/heads/")

		name, ok := strings.CutPrefix(branchName, branchPrefix)
		if !ok {
			return
		}

		worktrees = append(worktrees, &Worktree{
			Name:      name,
			Path:      current.path,
			Branch:    branchName,
			BaseRef:   current.committish,
			CreatedAt: time.Time{},
		})
	}

	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			flush()
			current = nil
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 1 {
			continue
		}

		switch parts[0] {
		case "worktree":
			if current != nil {
				flush()
			}
			current = &parsedWorktree{}
			if len(parts) > 1 {
				current.path = parts[1]
			}
		case "branch":
			if current != nil && len(parts) > 1 {
				current.branch = parts[1]
			}
		case "bare":
			if current != nil {
				current.isBare = true
			}
		case "HEAD":
			if current != nil && len(parts) > 1 {
				current.committish = parts[1]
			}
		}
	}

	flush()

	return worktrees, nil
}
