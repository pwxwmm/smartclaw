package shadow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")

	initialFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test Project\n"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "initial commit")
}

func TestNewShadowWorkspace(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	sw, err := NewShadowWorkspace(dir)
	if err != nil {
		t.Fatalf("NewShadowWorkspace: %v", err)
	}
	defer sw.Cleanup()

	if !sw.created {
		t.Error("expected workspace to be created")
	}
	if sw.shadowPath == "" {
		t.Error("expected non-empty shadow path")
	}
	if _, err := os.Stat(sw.shadowPath); os.IsNotExist(err) {
		t.Error("shadow path directory should exist")
	}

	t.Cleanup(func() {
		cmd := exec.Command("git", "worktree", "prune")
		cmd.Dir = dir
		cmd.Run()
	})
}

func TestShadowWorkspace_ApplyChange(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	sw, err := NewShadowWorkspace(dir)
	if err != nil {
		t.Fatalf("NewShadowWorkspace: %v", err)
	}
	defer sw.Cleanup()

	err = sw.ApplyChange(FileChange{
		Path:    "internal/foo/handler.go",
		Content: "package foo\n\nfunc Handle() {}\n",
	})
	if err != nil {
		t.Fatalf("ApplyChange: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(sw.shadowPath, "internal", "foo", "handler.go"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(written) != "package foo\n\nfunc Handle() {}\n" {
		t.Errorf("unexpected content: %q", string(written))
	}
}

func TestShadowWorkspace_ApplyChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	sw, err := NewShadowWorkspace(dir)
	if err != nil {
		t.Fatalf("NewShadowWorkspace: %v", err)
	}
	defer sw.Cleanup()

	err = sw.ApplyChanges([]FileChange{
		{Path: "a.txt", Content: "file a"},
		{Path: "b/c.txt", Content: "file c"},
	})
	if err != nil {
		t.Fatalf("ApplyChanges: %v", err)
	}

	a, _ := os.ReadFile(filepath.Join(sw.shadowPath, "a.txt"))
	if string(a) != "file a" {
		t.Errorf("unexpected a.txt content: %q", string(a))
	}
	c, _ := os.ReadFile(filepath.Join(sw.shadowPath, "b", "c.txt"))
	if string(c) != "file c" {
		t.Errorf("unexpected c.txt content: %q", string(c))
	}
}

func TestShadowWorkspace_GetDiff(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	sw, err := NewShadowWorkspace(dir)
	if err != nil {
		t.Fatalf("NewShadowWorkspace: %v", err)
	}
	defer sw.Cleanup()

	err = sw.ApplyChange(FileChange{
		Path:    "README.md",
		Content: "# Modified Project\n",
	})
	if err != nil {
		t.Fatalf("ApplyChange: %v", err)
	}

	diff, err := sw.GetDiff()
	if err != nil {
		t.Fatalf("GetDiff: %v", err)
	}
	if diff == "" {
		t.Error("expected non-empty diff after applying change")
	}
	if !strings.Contains(diff, "README.md") {
		t.Errorf("expected diff to mention README.md, got:\n%s", diff)
	}
}

func TestShadowWorkspace_GetDiff_NoChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	sw, err := NewShadowWorkspace(dir)
	if err != nil {
		t.Fatalf("NewShadowWorkspace: %v", err)
	}
	defer sw.Cleanup()

	diff, err := sw.GetDiff()
	if err != nil {
		t.Fatalf("GetDiff: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff with no changes, got:\n%s", diff)
	}
}

func TestShadowWorkspace_Cleanup(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	sw, err := NewShadowWorkspace(dir)
	if err != nil {
		t.Fatalf("NewShadowWorkspace: %v", err)
	}

	shadowPath := sw.shadowPath

	err = sw.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if sw.created {
		t.Error("expected created=false after cleanup")
	}
	if _, err := os.Stat(shadowPath); !os.IsNotExist(err) {
		t.Error("shadow directory should be removed after cleanup")
	}
}

func TestShadowWorkspace_Cleanup_Idempotent(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	sw, err := NewShadowWorkspace(dir)
	if err != nil {
		t.Fatalf("NewShadowWorkspace: %v", err)
	}

	err = sw.Cleanup()
	if err != nil {
		t.Fatalf("first Cleanup: %v", err)
	}
	err = sw.Cleanup()
	if err != nil {
		t.Errorf("second Cleanup should be no-op, got: %v", err)
	}
}

func TestShadowWorkspace_ApplyChange_NotCreated(t *testing.T) {
	sw := &ShadowWorkspace{created: false}
	err := sw.ApplyChange(FileChange{Path: "x.go", Content: "x"})
	if err == nil {
		t.Error("expected error when workspace not created")
	}
}

func TestShadowWorkspace_GetDiff_NotCreated(t *testing.T) {
	sw := &ShadowWorkspace{created: false}
	_, err := sw.GetDiff()
	if err == nil {
		t.Error("expected error when workspace not created")
	}
}
