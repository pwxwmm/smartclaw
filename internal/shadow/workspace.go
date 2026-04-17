package shadow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ShadowWorkspace manages an isolated git worktree for verifying code changes
// before applying them to the real project.
type ShadowWorkspace struct {
	originalPath string
	shadowPath   string
	worktreeName string
	created      bool
}

// FileChange represents a single file modification to apply.
type FileChange struct {
	Path    string
	Content string
}

// NewShadowWorkspace creates a git worktree named shadow-verify-{timestamp}
// from the current HEAD of the project at projectRoot.
func NewShadowWorkspace(projectRoot string) (*ShadowWorkspace, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving project root: %w", err)
	}

	worktreeName := fmt.Sprintf("shadow-verify-%d", time.Now().UnixMilli())
	shadowPath := filepath.Join(absRoot, worktreeName)

	cmd := exec.Command("git", "worktree", "add", "--detach", shadowPath)
	cmd.Dir = absRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("creating git worktree: %w\n%s", err, out)
	}

	return &ShadowWorkspace{
		originalPath: absRoot,
		shadowPath:   shadowPath,
		worktreeName: worktreeName,
		created:      true,
	}, nil
}

// ApplyChange writes a single file change into the shadow workspace.
func (sw *ShadowWorkspace) ApplyChange(fc FileChange) error {
	if !sw.created {
		return fmt.Errorf("shadow workspace not created")
	}

	fullPath := filepath.Join(sw.shadowPath, fc.Path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(fc.Content), 0o644); err != nil {
		return fmt.Errorf("writing file %s: %w", fc.Path, err)
	}

	return nil
}

// ApplyChanges writes multiple file changes into the shadow workspace.
func (sw *ShadowWorkspace) ApplyChanges(changes []FileChange) error {
	for _, fc := range changes {
		if err := sw.ApplyChange(fc); err != nil {
			return err
		}
	}
	return nil
}

// GetDiff returns the git diff between the shadow workspace and the original
// branch HEAD.
func (sw *ShadowWorkspace) GetDiff() (string, error) {
	if !sw.created {
		return "", fmt.Errorf("shadow workspace not created")
	}

	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = sw.shadowPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff: %w\n%s", err, out)
	}
	return string(out), nil
}

// Cleanup removes the git worktree and its directory.
func (sw *ShadowWorkspace) Cleanup() error {
	if !sw.created {
		return nil
	}

	cmd := exec.Command("git", "worktree", "remove", "--force", sw.shadowPath)
	cmd.Dir = sw.originalPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing worktree: %w\n%s", err, out)
	}

	sw.created = false
	return nil
}
