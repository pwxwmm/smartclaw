package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func initTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "smartclaw-worktree-test")
	if err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, tmpDir, "init", "-b", "main")
	mustRunGit(t, tmpDir, "config", "user.email", "test@test.com")
	mustRunGit(t, tmpDir, "config", "user.name", "Test")

	testFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, tmpDir, "add", "hello.txt")
	mustRunGit(t, tmpDir, "commit", "-m", "initial commit")

	return tmpDir
}

func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestNewManager(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	m := NewManager("/tmp/test-repo")
	if m.repoRoot != "/tmp/test-repo" {
		t.Errorf("expected repoRoot /tmp/test-repo, got %s", m.repoRoot)
	}
	expected := filepath.Join("/tmp/test-repo", worktreeBase)
	if m.worktreeDir != expected {
		t.Errorf("expected worktreeDir %s, got %s", expected, m.worktreeDir)
	}
}

func TestCreateAndRemove(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	wtPath, err := m.Create(ctx, "task-1", "HEAD")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	expectedPath := filepath.Join(repoDir, worktreeBase, "task-1")
	if wtPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, wtPath)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree directory %s does not exist", wtPath)
	}

	if _, err := os.Stat(filepath.Join(wtPath, "hello.txt")); os.IsNotExist(err) {
		t.Errorf("worktree should contain hello.txt from base commit")
	}

	if err := m.Remove(ctx, "task-1"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := os.Stat(wtPath); err == nil {
		t.Errorf("worktree directory should be removed after Remove()")
	}
}

func TestCreateDuplicate(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	_, err := m.Create(ctx, "dup", "HEAD")
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	defer m.Remove(ctx, "dup")

	_, err = m.Create(ctx, "dup", "HEAD")
	if err == nil {
		t.Error("expected error creating duplicate worktree")
	}
}

func TestRemoveNonexistent(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	err := m.Remove(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error removing nonexistent worktree")
	}
}

func TestList(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	worktrees, err := m.List(ctx)
	if err != nil {
		t.Fatalf("List on empty manager failed: %v", err)
	}
	if len(worktrees) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(worktrees))
	}

	_, err = m.Create(ctx, "list-test-1", "HEAD")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer m.Remove(ctx, "list-test-1")

	worktrees, err = m.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, wt := range worktrees {
		if wt.Name == "list-test-1" {
			found = true
			if wt.Branch != "smartclaw/list-test-1" {
				t.Errorf("expected branch smartclaw/list-test-1, got %s", wt.Branch)
			}
			if wt.Path == "" {
				t.Error("expected non-empty path")
			}
		}
	}
	if !found {
		t.Error("expected to find worktree 'list-test-1' in list")
	}
}

func TestGet(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	_, err := m.Get(ctx, "missing")
	if err == nil {
		t.Error("expected error getting nonexistent worktree")
	}

	_, err = m.Create(ctx, "get-test", "HEAD")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer m.Remove(ctx, "get-test")

	wt, err := m.Get(ctx, "get-test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if wt.Name != "get-test" {
		t.Errorf("expected name get-test, got %s", wt.Name)
	}
	if wt.Branch != "smartclaw/get-test" {
		t.Errorf("expected branch smartclaw/get-test, got %s", wt.Branch)
	}
}

func TestDiff(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	wtPath, err := m.Create(ctx, "diff-test", "HEAD")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer m.Remove(ctx, "diff-test")

	newFile := filepath.Join(wtPath, "new_file.txt")
	if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, wtPath, "add", "new_file.txt")
	mustRunGit(t, wtPath, "commit", "-m", "add new file")

	diff, err := m.Diff(ctx, "diff-test")
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(diff) == 0 {
		t.Error("expected non-empty diff")
	}
}

func TestDiffNoChanges(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	_, err := m.Create(ctx, "diff-empty", "HEAD")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer m.Remove(ctx, "diff-empty")

	diff, err := m.Diff(ctx, "diff-empty")
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(diff) != 0 {
		t.Errorf("expected empty diff for unchanged worktree, got %s", string(diff))
	}
}

func TestMergeSquash(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	wtPath, err := m.Create(ctx, "merge-test", "HEAD")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer m.Remove(ctx, "merge-test")

	newFile := filepath.Join(wtPath, "merged_file.txt")
	if err := os.WriteFile(newFile, []byte("merged content"), 0644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, wtPath, "add", "merged_file.txt")
	mustRunGit(t, wtPath, "commit", "-m", "add merged file")

	err = m.Merge(ctx, "merge-test", "squash")
	if err != nil {
		t.Fatalf("Merge squash failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "merged_file.txt")); os.IsNotExist(err) {
		t.Error("expected merged_file.txt to exist after squash merge")
	}
}

func TestMergeDefault(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	m := NewManager(repoDir)
	ctx := context.Background()

	wtPath, err := m.Create(ctx, "merge-default", "HEAD")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer m.Remove(ctx, "merge-default")

	newFile := filepath.Join(wtPath, "default_merged.txt")
	if err := os.WriteFile(newFile, []byte("default merge content"), 0644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, wtPath, "add", "default_merged.txt")
	mustRunGit(t, wtPath, "commit", "-m", "add default merged file")

	err = m.Merge(ctx, "merge-default", "merge")
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "default_merged.txt")); os.IsNotExist(err) {
		t.Error("expected default_merged.txt to exist after merge")
	}
}

func TestRunGitError(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	ctx := context.Background()
	_, err := runGit(ctx, "/nonexistent/path", "status")
	if err == nil {
		t.Error("expected error running git in nonexistent directory")
	}
}

func TestCreateFromBranch(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := initTestRepo(t)
	defer os.RemoveAll(repoDir)

	mustRunGit(t, repoDir, "checkout", "-b", "feature-branch")
	extraFile := filepath.Join(repoDir, "feature.txt")
	if err := os.WriteFile(extraFile, []byte("feature content"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repoDir, "add", "feature.txt")
	mustRunGit(t, repoDir, "commit", "-m", "add feature")
	mustRunGit(t, repoDir, "checkout", "main")

	m := NewManager(repoDir)
	ctx := context.Background()

	wtPath, err := m.Create(ctx, "from-branch", "feature-branch")
	if err != nil {
		t.Fatalf("Create from branch failed: %v", err)
	}
	defer m.Remove(ctx, "from-branch")

	if _, err := os.Stat(filepath.Join(wtPath, "feature.txt")); os.IsNotExist(err) {
		t.Error("worktree based on feature-branch should contain feature.txt")
	}
}
