package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetContextNonGitRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "non-git-repo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, err := GetContext(tmpDir)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	if ctx.IsRepo {
		t.Error("Expected IsRepo to be false for non-git directory")
	}
}

func TestGetContextGitRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-repo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.email", "test@test.com")
	runGit(tmpDir, "config", "user.name", "Test")

	ctx, err := GetContext(tmpDir)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !ctx.IsRepo {
		t.Error("Expected IsRepo to be true for git directory")
	}

	if ctx.RootDir == "" {
		t.Error("Expected non-empty root directory")
	}
}

func TestGetDiff(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-repo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.email", "test@test.com")
	runGit(tmpDir, "config", "user.name", "Test")

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	runGit(tmpDir, "add", "test.txt")
	runGit(tmpDir, "commit", "-m", "initial")

	os.WriteFile(testFile, []byte("modified content"), 0644)

	diff, err := GetDiff(tmpDir, false)
	if err != nil {
		t.Fatalf("Expected no error getting diff, got %v", err)
	}

	if diff == "" {
		t.Error("Expected non-empty diff for modified file")
	}
}

func TestGetStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-repo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.email", "test@test.com")
	runGit(tmpDir, "config", "user.name", "Test")

	testFile := filepath.Join(tmpDir, "untracked.txt")
	os.WriteFile(testFile, []byte("untracked"), 0644)

	status, err := GetStatus(tmpDir)
	if err != nil {
		t.Fatalf("Expected no error getting status, got %v", err)
	}

	if status == "" {
		t.Error("Expected non-empty status for repo with untracked files")
	}
}

func TestGetLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-repo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.email", "test@test.com")
	runGit(tmpDir, "config", "user.name", "Test")

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	runGit(tmpDir, "add", "test.txt")
	runGit(tmpDir, "commit", "-m", "initial commit")

	log, err := GetLog(tmpDir, 5)
	if err != nil {
		t.Fatalf("Expected no error getting log, got %v", err)
	}

	if log == "" {
		t.Error("Expected non-empty log for repo with commits")
	}
}

func TestContextString(t *testing.T) {
	ctx := &Context{
		RootDir:      "/test/path",
		Branch:       "main",
		IsRepo:       true,
		HasChanges:   true,
		StagedFiles:  []string{"file1.txt"},
		ChangedFiles: []string{"file2.txt"},
	}

	str := ctx.String()

	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestContextStringNonRepo(t *testing.T) {
	ctx := &Context{
		IsRepo: false,
	}

	str := ctx.String()

	if str != "Not a git repository" {
		t.Errorf("Expected 'Not a git repository', got '%s'", str)
	}
}

func TestContext(t *testing.T) {
	ctx := &Context{
		RootDir:      "/path/to/repo",
		Branch:       "feature-branch",
		IsRepo:       true,
		HasChanges:   true,
		StagedFiles:  []string{"file1.txt", "file2.txt"},
		ChangedFiles: []string{"file3.txt"},
	}

	if ctx.RootDir != "/path/to/repo" {
		t.Errorf("Expected root '/path/to/repo', got '%s'", ctx.RootDir)
	}

	if ctx.Branch != "feature-branch" {
		t.Errorf("Expected branch 'feature-branch', got '%s'", ctx.Branch)
	}

	if len(ctx.StagedFiles) != 2 {
		t.Errorf("Expected 2 staged files, got %d", len(ctx.StagedFiles))
	}
}

func TestParseLines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	lines := parseLines(input)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line1" {
		t.Errorf("Expected first line 'line1', got '%s'", lines[0])
	}
}

func TestParseLinesEmpty(t *testing.T) {
	lines := parseLines("")

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for empty input, got %d", len(lines))
	}
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
