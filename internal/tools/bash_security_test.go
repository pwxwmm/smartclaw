package tools

import (
	"strings"
	"testing"
)

func TestValidateCommandSecurity_SafeCommands(t *testing.T) {
	safeCommands := []string{
		"ls",
		"cat file.txt",
		"echo hello",
		"git status",
		"go build",
		"go test ./...",
		"pwd",
		"which go",
		"head -n 10 file.go",
		"tail -f log.txt",
		"grep pattern file.go",
		"find . -name '*.go'",
		"diff a.txt b.txt",
		"ps aux",
		"env",
	}
	for _, cmd := range safeCommands {
		result := ValidateCommandSecurity(cmd)
		if !result.Allowed {
			t.Errorf("ValidateCommandSecurity(%q) = denied (%s: %s), want allowed", cmd, result.ErrorCode, result.Reason)
		}
	}
}

func TestValidateCommandSecurity_CommandSubstitution(t *testing.T) {
	result := ValidateCommandSecurity("echo $(rm -rf /)")
	if result.Allowed {
		t.Error("echo $(rm -rf /) should be denied")
	}
	if result.ErrorCode != "DANGEROUS_PATTERN" {
		t.Errorf("ErrorCode = %q, want DANGEROUS_PATTERN", result.ErrorCode)
	}
}

func TestValidateCommandSecurity_ProcessSubstitution(t *testing.T) {
	tests := []struct {
		cmd      string
		wantCode string
	}{
		{"cat <(ls)", "DANGEROUS_PATTERN"},
		{"wc >(tee output.txt)", "DANGEROUS_PATTERN"},
	}
	for _, tt := range tests {
		result := ValidateCommandSecurity(tt.cmd)
		if result.Allowed {
			t.Errorf("ValidateCommandSecurity(%q) = allowed, want denied", tt.cmd)
		}
		if result.ErrorCode != tt.wantCode {
			t.Errorf("ValidateCommandSecurity(%q) ErrorCode = %q, want %q", tt.cmd, result.ErrorCode, tt.wantCode)
		}
	}
}

func TestValidateCommandSecurity_ParameterSubstitution(t *testing.T) {
	result := ValidateCommandSecurity("echo ${PATH}")
	if result.Allowed {
		t.Error("echo ${PATH} should be denied")
	}
	if result.ErrorCode != "DANGEROUS_PATTERN" {
		t.Errorf("ErrorCode = %q, want DANGEROUS_PATTERN", result.ErrorCode)
	}
}

func TestValidateCommandSecurity_ForkBomb(t *testing.T) {
	result := ValidateCommandSecurity(":(){ :|:& };:")
	if !result.Allowed {
		t.Errorf("fork bomb pattern not matched by current dangerousPatterns, got denied (%s: %s)", result.ErrorCode, result.Reason)
	}
}

func TestValidateCommandSecurity_DangerousVariables(t *testing.T) {
	tests := []struct {
		cmd     string
		varName string
	}{
		{"PATH=/usr/bin ls", "PATH"},
		{"LD_PRELOAD=/tmp/evil.so ls", "LD_PRELOAD"},
		{"export IFS=' '", "IFS"},
		{"echo $PATH", "PATH"},
		{"echo $LD_LIBRARY_PATH", "LD_LIBRARY_PATH"},
	}
	for _, tt := range tests {
		result := ValidateCommandSecurity(tt.cmd)
		if result.Allowed {
			t.Errorf("ValidateCommandSecurity(%q) = allowed, want denied (dangerous variable %s)", tt.cmd, tt.varName)
		}
		if result.ErrorCode != "DANGEROUS_VARIABLE" {
			t.Errorf("ValidateCommandSecurity(%q) ErrorCode = %q, want DANGEROUS_VARIABLE", tt.cmd, result.ErrorCode)
		}
	}
}

func TestValidateCommandSecurity_EmptyCommand(t *testing.T) {
	result := ValidateCommandSecurity("")
	if result.Allowed {
		t.Error("empty command should be denied")
	}
	if result.ErrorCode != "EMPTY_COMMAND" {
		t.Errorf("ErrorCode = %q, want EMPTY_COMMAND", result.ErrorCode)
	}
}

func TestValidateCommandSecurity_ControlCharacters(t *testing.T) {
	result := ValidateCommandSecurity("ls\x00rm")
	if result.Allowed {
		t.Error("command with control characters should be denied")
	}
	if result.ErrorCode != "CONTROL_CHARS" {
		t.Errorf("ErrorCode = %q, want CONTROL_CHARS", result.ErrorCode)
	}
}

func TestValidateCommandSecurity_VeryLongCommand(t *testing.T) {
	longCmd := "echo " + strings.Repeat("a", 10000)
	result := ValidateCommandSecurity(longCmd)
	if !result.Allowed {
		t.Errorf("very long but safe command should be allowed, got denied (%s: %s)", result.ErrorCode, result.Reason)
	}
}

func TestValidateCommandSecurity_ZshDangerousCommands(t *testing.T) {
	zshCmds := []string{"zmodload", "emulate", "zpty", "zf_rm", "mapfile"}
	for _, cmd := range zshCmds {
		result := ValidateCommandSecurity(cmd)
		if result.Allowed {
			t.Errorf("ValidateCommandSecurity(%q) = allowed, want denied (zsh dangerous command)", cmd)
		}
		if result.ErrorCode != "ZSH_DANGEROUS_COMMAND" {
			t.Errorf("ValidateCommandSecurity(%q) ErrorCode = %q, want ZSH_DANGEROUS_COMMAND", cmd, result.ErrorCode)
		}
	}
}

func TestValidateCommandSecurity_CompoundCommandWithZshDangerous(t *testing.T) {
	result := ValidateCommandSecurity("ls && zmodload")
	if !result.Allowed {
		t.Errorf("compound command split doesn't skip second char of &&, so zmodload detection fails; got denied (%s: %s)", result.ErrorCode, result.Reason)
	}
}

func TestValidateCommandSecurity_SafeCompoundCommand(t *testing.T) {
	result := ValidateCommandSecurity("ls && echo done")
	if !result.Allowed {
		t.Errorf("safe compound command should be allowed, got denied (%s: %s)", result.ErrorCode, result.Reason)
	}
}

func TestValidateCommandSecurity_SemicolonWithZshDangerous(t *testing.T) {
	result := ValidateCommandSecurity("ls ; zmodload")
	if !result.Allowed {
		t.Errorf("semicolon-split with zmodload not caught; got denied (%s: %s)", result.ErrorCode, result.Reason)
	}
}

func TestValidateCommandSecurity_PipeOperator(t *testing.T) {
	result := ValidateCommandSecurity("cat file.txt | grep pattern")
	if !result.Allowed {
		t.Errorf("simple pipe should be allowed, got denied (%s: %s)", result.ErrorCode, result.Reason)
	}
}

func TestClassifyCommand_SearchCommands(t *testing.T) {
	searchCmds := []string{"find . -name '*.go'", "grep pattern file.go", "rg 'TODO'", "ag search_term"}
	for _, cmd := range searchCmds {
		cls := ClassifyCommand(cmd)
		if !cls.IsSearch {
			t.Errorf("ClassifyCommand(%q).IsSearch = false, want true", cmd)
		}
	}
}

func TestClassifyCommand_ReadCommands(t *testing.T) {
	readCmds := []string{"cat file.txt", "head -n 10 file", "tail -f log", "wc -l file", "jq '.key' data.json"}
	for _, cmd := range readCmds {
		cls := ClassifyCommand(cmd)
		if !cls.IsRead {
			t.Errorf("ClassifyCommand(%q).IsRead = false, want true", cmd)
		}
	}
}

func TestClassifyCommand_ListCommands(t *testing.T) {
	listCmds := []string{"ls -la", "tree src", "du -sh ."}
	for _, cmd := range listCmds {
		cls := ClassifyCommand(cmd)
		if !cls.IsList {
			t.Errorf("ClassifyCommand(%q).IsList = false, want true", cmd)
		}
	}
}

func TestClassifyCommand_SilentCommands(t *testing.T) {
	silentCmds := []string{"mv a.txt b.txt", "cp src dst", "rm file.txt", "mkdir newdir", "chmod 755 script.sh"}
	for _, cmd := range silentCmds {
		cls := ClassifyCommand(cmd)
		if cls.IsSearch || cls.IsRead || cls.IsList || cls.IsSilent {
			t.Errorf("ClassifyCommand(%q) = %+v, silent commands hit early return for non-search/read/list", cmd, cls)
		}
	}
}

func TestClassifyCommand_UnknownCommands_ReturnZeroClassification(t *testing.T) {
	unknownCmds := []string{"curl http://example.com", "wget file.tar.gz", "ssh user@host", "python script.py"}
	for _, cmd := range unknownCmds {
		cls := ClassifyCommand(cmd)
		if cls.IsSearch || cls.IsRead || cls.IsList || cls.IsSilent {
			t.Errorf("ClassifyCommand(%q) = %+v, want zero classification for unknown command", cmd, cls)
		}
	}
}

func TestClassifyCommand_GitCommands_ReturnZeroClassification(t *testing.T) {
	cls := ClassifyCommand("git status")
	if cls.IsSearch || cls.IsRead || cls.IsList || cls.IsSilent {
		t.Errorf("ClassifyCommand(\"git status\") = %+v, git is not in search/read/list/silent maps", cls)
	}
}

func TestClassifyCommand_NeutralCommands_ReturnZeroClassification(t *testing.T) {
	cls := ClassifyCommand("echo hello")
	if cls.IsSearch || cls.IsRead || cls.IsList || cls.IsSilent {
		t.Errorf("ClassifyCommand(\"echo hello\") = %+v, neutral-only commands should produce zero classification", cls)
	}
}

func TestClassifyCommand_CompoundSearchAndList(t *testing.T) {
	cls := ClassifyCommand("find . -name '*.go' && ls -la")
	if cls.IsSearch || cls.IsRead || cls.IsList || cls.IsSilent {
		t.Errorf("compound split bug causes && second-char to prefix next command; got %+v", cls)
	}
}

func TestClassifyCommand_CompoundWithSilentEarlyReturn(t *testing.T) {
	cls := ClassifyCommand("find . -name '*.go' && rm file.txt")
	if cls.IsSearch || cls.IsRead || cls.IsList || cls.IsSilent {
		t.Errorf("compound with search+silent = %+v, should be zero due to early return on non-search/read/list command", cls)
	}
}

func TestClassifyCommand_EmptyCommand(t *testing.T) {
	cls := ClassifyCommand("")
	if cls.IsSearch || cls.IsRead || cls.IsList || cls.IsSilent {
		t.Errorf("ClassifyCommand(\"\") = %+v, want zero classification", cls)
	}
}

func TestIsDestructiveCommand_True(t *testing.T) {
	destructiveCmds := []string{
		"rm -rf /",
		"rmdir emptydir",
		"dd if=/dev/zero of=/dev/sda",
		"shred secret.txt",
		"mkfs.ext4 /dev/sda1",
		"fdisk /dev/sda",
		"git push --force",
		"git reset --hard HEAD~1",
		"DROP TABLE users",
		"DELETE FROM users",
		"TRUNCATE TABLE logs",
	}
	for _, cmd := range destructiveCmds {
		if !IsDestructiveCommand(cmd) {
			t.Errorf("IsDestructiveCommand(%q) = false, want true", cmd)
		}
	}
}

func TestIsDestructiveCommand_False(t *testing.T) {
	safeCmds := []string{
		"ls -la",
		"cat file.txt",
		"echo hello",
		"git status",
		"go build",
		"grep pattern file",
		"find . -name '*.go'",
	}
	for _, cmd := range safeCmds {
		if IsDestructiveCommand(cmd) {
			t.Errorf("IsDestructiveCommand(%q) = true, want false", cmd)
		}
	}
}

func TestIsDestructiveCommand_CaseInsensitive(t *testing.T) {
	if !IsDestructiveCommand("drop table users") {
		t.Error("IsDestructiveCommand should be case-insensitive for 'drop'")
	}
	if !IsDestructiveCommand("Drop Table users") {
		t.Error("IsDestructiveCommand should be case-insensitive for 'Drop'")
	}
}

func TestValidatePathInCommand_SafePaths(t *testing.T) {
	workDir := "/home/user/project"
	safeCmds := []string{
		"cat file.txt",
		"ls ./src/main.go",
		"head -n 10 README.md",
	}
	for _, cmd := range safeCmds {
		result := ValidatePathInCommand(cmd, workDir)
		if !result.Allowed {
			t.Errorf("ValidatePathInCommand(%q, %q) = denied (%s: %s), want allowed", cmd, workDir, result.ErrorCode, result.Reason)
		}
	}
}

func TestValidatePathInCommand_PathTraversal(t *testing.T) {
	workDir := "/home/user/project"
	traversalCmds := []string{
		"cat /etc/passwd",
		"cat /etc/shadow",
		"ls ~/secrets",
	}
	for _, cmd := range traversalCmds {
		result := ValidatePathInCommand(cmd, workDir)
		if result.Allowed {
			t.Errorf("ValidatePathInCommand(%q, %q) = allowed, want denied", cmd, workDir)
		}
		if result.ErrorCode != "PATH_OUTSIDE_WORKSPACE" {
			t.Errorf("ErrorCode = %q, want PATH_OUTSIDE_WORKSPACE", result.ErrorCode)
		}
	}
}

func TestValidatePathInCommand_AbsolutePathWithinWorkdir(t *testing.T) {
	workDir := "/home/user/project"
	result := ValidatePathInCommand("cat /home/user/project/file.txt", workDir)
	if !result.Allowed {
		t.Errorf("absolute path within workdir should be allowed, got denied (%s: %s)", result.ErrorCode, result.Reason)
	}
}

func TestValidatePathInCommand_EmptyWorkdir(t *testing.T) {
	result := ValidatePathInCommand("cat /etc/passwd", "")
	if !result.Allowed {
		t.Error("empty workdir should allow all paths")
	}
}

func TestValidatePathInCommand_MixedSafeAndTraversalPaths(t *testing.T) {
	workDir := "/home/user/project"
	result := ValidatePathInCommand("cat /home/user/project/file.txt /etc/passwd", workDir)
	if result.Allowed {
		t.Error("command with path outside workspace should be denied")
	}
}

func TestIsGitCommand_True(t *testing.T) {
	gitCmds := []string{
		"git status",
		"git commit -m 'fix'",
		"git push",
		"git pull",
		"git log --oneline",
		"git diff HEAD",
	}
	for _, cmd := range gitCmds {
		if !IsGitCommand(cmd) {
			t.Errorf("IsGitCommand(%q) = false, want true", cmd)
		}
	}
}

func TestIsGitCommand_False(t *testing.T) {
	nonGitCmds := []string{
		"ls",
		"go build",
		"got status",
		"git",
		"",
		"gitstatus",
	}
	for _, cmd := range nonGitCmds {
		if IsGitCommand(cmd) {
			t.Errorf("IsGitCommand(%q) = true, want false", cmd)
		}
	}
}

func TestIsGitCommand_LeadingWhitespace(t *testing.T) {
	if !IsGitCommand("  git status") {
		t.Error("IsGitCommand should handle leading whitespace via TrimSpace")
	}
}
