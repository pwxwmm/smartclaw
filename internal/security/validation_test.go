package security

import (
	"strings"
	"testing"
)

func assertBlocked(t *testing.T, cmd string, expectedCode string) {
	t.Helper()
	result := ValidateCommandSecurity(cmd)
	if result.Allowed {
		t.Errorf("expected command to be blocked: %q", cmd)
	}
	if expectedCode != "" && result.ErrorCode != expectedCode {
		t.Errorf("expected error code %q for %q, got %q (%s)", expectedCode, cmd, result.ErrorCode, result.Reason)
	}
}

func assertAllowed(t *testing.T, cmd string) {
	t.Helper()
	result := ValidateCommandSecurity(cmd)
	if !result.Allowed {
		t.Errorf("expected command to be allowed: %q, got blocked: %s (%s)", cmd, result.Reason, result.ErrorCode)
	}
}

func TestCommandSubstitution(t *testing.T) {
	tests := []struct {
		cmd  string
		code string
	}{
		{"$(rm -rf /)", "DANGEROUS_PATTERN"},
		{"echo $(cat /etc/passwd)", "DANGEROUS_PATTERN"},
		{"$($(cmd))", "DANGEROUS_PATTERN"},
		{"$(cat /etc/passwd)", "DANGEROUS_PATTERN"},
		{"ls $(pwd)", "DANGEROUS_PATTERN"},
		{"echo $(whoami)", "DANGEROUS_PATTERN"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assertBlocked(t, tt.cmd, tt.code)
		})
	}
}

func TestProcessSubstitution(t *testing.T) {
	tests := []struct {
		cmd  string
		code string
	}{
		{"<(cmd)", "DANGEROUS_PATTERN"},
		{">(cmd)", "DANGEROUS_PATTERN"},
		{"cat <(ls)", "DANGEROUS_PATTERN"},
		{"tee >(grep foo)", "DANGEROUS_PATTERN"},
		{"diff <(sort a.txt) <(sort b.txt)", "DANGEROUS_PATTERN"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assertBlocked(t, tt.cmd, tt.code)
		})
	}
}

func TestParameterExpansion(t *testing.T) {
	tests := []struct {
		cmd  string
		code string
	}{
		{"${IFS}", "DANGEROUS_PATTERN"},
		{"${PATH}", "DANGEROUS_PATTERN"},
		{"${LD_PRELOAD}", "DANGEROUS_PATTERN"},
		{"${HOME}/.ssh", "DANGEROUS_PATTERN"},
		{"echo ${BASH_VERSION}", "DANGEROUS_PATTERN"},
		{"${USER}", "DANGEROUS_PATTERN"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assertBlocked(t, tt.cmd, tt.code)
		})
	}
}

func TestDangerousVariableAssignments(t *testing.T) {
	tests := []struct {
		cmd  string
		code string
	}{
		{"IFS=...", "DANGEROUS_VARIABLE"},
		{"LD_PRELOAD=...", "DANGEROUS_VARIABLE"},
		{"PATH=...", "DANGEROUS_VARIABLE"},
		{"LD_LIBRARY_PATH=...", "DANGEROUS_VARIABLE"},
		{"SHELL=/bin/bash", "DANGEROUS_VARIABLE"},
		{"BASH_ENV=...", "DANGEROUS_VARIABLE"},
		{"ENV=...", "DANGEROUS_VARIABLE"},
		{"PYTHONPATH=...", "DANGEROUS_VARIABLE"},
		{"NODE_PATH=...", "DANGEROUS_VARIABLE"},
		{"PERL5LIB=...", "DANGEROUS_VARIABLE"},
		{"RUBYLIB=...", "DANGEROUS_VARIABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assertBlocked(t, tt.cmd, tt.code)
		})
	}
}

func TestDangerousVariableReferences(t *testing.T) {
	tests := []struct {
		cmd  string
		code string
	}{
		{"echo $IFS", "DANGEROUS_VARIABLE"},
		{"echo $LD_PRELOAD", "DANGEROUS_VARIABLE"},
		{"echo $PATH", "DANGEROUS_VARIABLE"},
		{"echo $LD_LIBRARY_PATH", "DANGEROUS_VARIABLE"},
		{"echo $SHELL", "DANGEROUS_VARIABLE"},
		{"echo $BASH_ENV", "DANGEROUS_VARIABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assertBlocked(t, tt.cmd, tt.code)
		})
	}
}

func TestZshDangerousCommands(t *testing.T) {
	commands := []string{
		"zmodload", "emulate", "sysopen", "sysread", "syswrite",
		"sysseek", "zpty", "ztcp", "zsocket", "mapfile",
		"zf_rm", "zf_mv", "zf_ln", "zf_chmod", "zf_chown",
		"zf_mkdir", "zf_rmdir", "zf_chgrp",
	}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			assertBlocked(t, cmd, "ZSH_DANGEROUS_COMMAND")
		})
	}
}

func TestZshDangerousCommandsWithArgs(t *testing.T) {
	tests := []struct {
		cmd  string
		code string
	}{
		{"zmodload zsh/net/tcp", "ZSH_DANGEROUS_COMMAND"},
		{"zpty -w daemon cmd", "ZSH_DANGEROUS_COMMAND"},
		{"sysopen -r /dev/null", "ZSH_DANGEROUS_COMMAND"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assertBlocked(t, tt.cmd, tt.code)
		})
	}
}

func TestCompoundCommandsWithDangerousSubcommands(t *testing.T) {
	tests := []struct {
		cmd  string
		code string
	}{
		{"ls && zmodload", "ZSH_DANGEROUS_COMPOUND"},
		{"true || zpty start", "ZSH_DANGEROUS_COMPOUND"},
		{"ls ; sysopen /dev/null", "ZSH_DANGEROUS_COMPOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			assertBlocked(t, tt.cmd, tt.code)
		})
	}
}

func TestCompoundCommandsSafe(t *testing.T) {
	assertAllowed(t, "ls && pwd")
	assertAllowed(t, "echo hello ; echo world")
}

func TestPipes(t *testing.T) {
	assertBlocked(t, "cat file | $(rm -rf /)", "DANGEROUS_PATTERN")
	assertAllowed(t, "ls | grep foo")
	assertAllowed(t, "cat file.txt | head -5")
}

func TestSafeCommands(t *testing.T) {
	commands := []string{
		"ls", "cat file.txt", "echo hello", "go build",
		"git status", "cd /tmp", "pwd", "make test",
		"docker ps", "kubectl get pods", "python script.py",
		"node server.js", "npm install", "cargo build", "rustc main.rs",
	}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			assertAllowed(t, cmd)
		})
	}
}

func TestEmptyCommand(t *testing.T) {
	assertBlocked(t, "", "EMPTY_COMMAND")
}

func TestWhitespaceOnly(t *testing.T) {
	assertBlocked(t, "   ", "EMPTY_COMMAND")
}

// Control chars pattern: [\x00-\x08\x0b\x0c\x0e-\x1f\x7f]
func TestControlCharacters(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"null_byte", "ls\x00rm"},
		{"bell", "echo\x07hello"},
		{"backspace", "cat\b\b"},
		{"vertical_tab", "ls\x0brm"},
		{"form_feed", "echo\x0cworld"},
		{"escape", "ls\x1b[31m"},
		{"del", "rm\x7f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBlocked(t, tt.cmd, "CONTROL_CHARS")
		})
	}
}

func TestNewlineNotControlChar(t *testing.T) {
	result := ValidateCommandSecurity("ls\nrm")
	if result.ErrorCode == "CONTROL_CHARS" {
		t.Error("newline (0x0a) should not trigger CONTROL_CHARS")
	}
}

func TestTabNotControlChar(t *testing.T) {
	result := ValidateCommandSecurity("ls\trm")
	if result.ErrorCode == "CONTROL_CHARS" {
		t.Error("tab (0x09) should not trigger CONTROL_CHARS")
	}
}

func TestVeryLongCommand(t *testing.T) {
	assertAllowed(t, strings.Repeat("a", 10000))
}

func TestQuotedDangerousContent(t *testing.T) {
	assertBlocked(t, `"$(cmd)"`, "DANGEROUS_PATTERN")
	assertBlocked(t, `'$(rm -rf /)'`, "DANGEROUS_PATTERN")
	assertAllowed(t, `echo 'rm -rf /'`)
	assertBlocked(t, `"${IFS}"`, "DANGEROUS_PATTERN")
}

func TestLegacyArithmeticExpansion(t *testing.T) {
	assertBlocked(t, "$[1+2]", "DANGEROUS_PATTERN")
}

func TestZshEqualsExpansion(t *testing.T) {
	assertBlocked(t, "=ls", "DANGEROUS_PATTERN")
	assertBlocked(t, "=cat", "DANGEROUS_PATTERN")
}

func TestZshProcessSubEquals(t *testing.T) {
	assertBlocked(t, "=(cmd)", "DANGEROUS_PATTERN")
}

func TestZshTildeParameterExpansion(t *testing.T) {
	assertBlocked(t, "~[param]", "DANGEROUS_PATTERN")
}

func TestZshGlobQualifiers(t *testing.T) {
	assertBlocked(t, "(e:cmd:)", "DANGEROUS_PATTERN")
	assertBlocked(t, "(+cmd)", "DANGEROUS_PATTERN")
}

func TestZshAlwaysBlock(t *testing.T) {
	assertBlocked(t, "} always {", "DANGEROUS_PATTERN")
}

func TestPowerShellComment(t *testing.T) {
	assertBlocked(t, "<# comment #>", "DANGEROUS_PATTERN")
}

func TestExtractBaseCommand(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ls -la", "ls"},
		{"git status", "git"},
		{"  echo hello  ", "echo"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		got := extractBaseCommand(tt.input)
		if got != tt.want {
			t.Errorf("extractBaseCommand(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitCommandWithOperators(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"simple", "ls && pwd", []string{"ls", "&&", "pwd"}},
		{"or", "ls || pwd", []string{"ls", "||", "pwd"}},
		{"semicolon", "ls ; pwd", []string{"ls", ";", "pwd"}},
		{"mixed", "a && b || c", []string{"a", "&&", "b", "||", "c"}},
		{"single", "ls", []string{"ls"}},
		{"quoted_and", `echo "a && b"`, []string{`echo "a && b"`}},
		{"single_quoted_semi", `echo 'a ; b'`, []string{`echo 'a ; b'`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCommandWithOperators(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitCommandWithOperators(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCommandWithOperators(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsOperator(t *testing.T) {
	if !isOperator("&&") {
		t.Error("&& should be operator")
	}
	if !isOperator("||") {
		t.Error("|| should be operator")
	}
	if !isOperator(";") {
		t.Error("; should be operator")
	}
	if isOperator("ls") {
		t.Error("ls should not be operator")
	}
	if isOperator("|") {
		t.Error("| should not be operator")
	}
}

func TestSecurityValidationResultFields(t *testing.T) {
	result := ValidateCommandSecurity("$(rm -rf /)")
	if result.Allowed != false {
		t.Error("should be blocked")
	}
	if result.Reason == "" {
		t.Error("reason should not be empty")
	}
	if result.ErrorCode == "" {
		t.Error("error code should not be empty")
	}
}

func TestSafeCommandsThatLookDangerous(t *testing.T) {
	assertAllowed(t, "echo PATH")
	assertAllowed(t, "grep LD_PRELOAD")
	assertAllowed(t, "ls IFS")
}

func TestCompoundCommandWithDangerousPattern(t *testing.T) {
	assertBlocked(t, "ls && $(rm -rf /)", "DANGEROUS_PATTERN")
	assertBlocked(t, "ls ; ${IFS}", "DANGEROUS_PATTERN")
}

func TestBacktickSubstitution_CurrentlyAllowed(t *testing.T) {
	assertAllowed(t, "`rm -rf /`")
	assertAllowed(t, "echo `whoami`")
}

func TestBuiltinAndExec_NotInDangerousMap(t *testing.T) {
	assertAllowed(t, "builtin cd /tmp")
	assertAllowed(t, "exec ls")
}

func TestCompoundCommands_NonZshDangerousSubcommands(t *testing.T) {
	assertAllowed(t, "ls && rm -rf /")
	assertAllowed(t, "ls ; cat /etc/passwd")
	assertAllowed(t, "true || rm file.txt")
}

func TestQuotedCommandSubstitution(t *testing.T) {
	assertBlocked(t, `"$(cmd)"`, "DANGEROUS_PATTERN")
	assertBlocked(t, `'$(rm -rf /)'`, "DANGEROUS_PATTERN")
	assertAllowed(t, `echo 'rm -rf /'`)
}

func TestDangerousVariableDirectReference(t *testing.T) {
	assertBlocked(t, "$IFS", "DANGEROUS_VARIABLE")
	assertBlocked(t, "$LD_PRELOAD", "DANGEROUS_VARIABLE")
	assertBlocked(t, "$PATH", "DANGEROUS_VARIABLE")
}

func TestDollarSignInPrice_IsSafe(t *testing.T) {
	assertAllowed(t, "echo $5.00")
}

func TestValidateCommandSecurity_EmptyResult(t *testing.T) {
	result := ValidateCommandSecurity("ls")
	if !result.Allowed {
		t.Error("safe command should be allowed")
	}
	if result.Reason != "" {
		t.Errorf("allowed command should have empty reason, got %q", result.Reason)
	}
	if result.ErrorCode != "" {
		t.Errorf("allowed command should have empty error code, got %q", result.ErrorCode)
	}
}
