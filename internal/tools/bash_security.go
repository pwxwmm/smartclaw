package tools

import (
	"regexp"
	"strings"
)

type SecurityValidationResult struct {
	Allowed   bool
	Reason    string
	ErrorCode string
}

var dangerousPatterns = []struct {
	pattern *regexp.Regexp
	message string
}{
	{regexp.MustCompile(`<\(`), "process substitution <()"},
	{regexp.MustCompile(`>\(`), "process substitution >()"},
	{regexp.MustCompile(`=\(`), "Zsh process substitution =()"},
	{regexp.MustCompile(`(?:^|[\s;&|])=[a-zA-Z_]`), "Zsh equals expansion (=cmd)"},
	{regexp.MustCompile(`\$\(`), "$() command substitution"},
	{regexp.MustCompile(`\$\{`), "${} parameter substitution"},
	{regexp.MustCompile(`\$\[`), "$[] legacy arithmetic expansion"},
	{regexp.MustCompile(`~\[`), "Zsh-style parameter expansion"},
	{regexp.MustCompile(`\(e:`), "Zsh-style glob qualifiers"},
	{regexp.MustCompile(`\(\+`), "Zsh glob qualifier with command execution"},
	{regexp.MustCompile(`\}\s*always\s*\{`), "Zsh always block"},
	{regexp.MustCompile(`<#`), "PowerShell comment syntax"},
}

var zshDangerousCommands = map[string]bool{
	"zmodload": true,
	"emulate":  true,
	"sysopen":  true,
	"sysread":  true,
	"syswrite": true,
	"sysseek":  true,
	"zpty":     true,
	"ztcp":     true,
	"zsocket":  true,
	"mapfile":  true,
	"zf_rm":    true,
	"zf_mv":    true,
	"zf_ln":    true,
	"zf_chmod": true,
	"zf_chown": true,
	"zf_mkdir": true,
	"zf_rmdir": true,
	"zf_chgrp": true,
}

var dangerousVariables = []string{
	"IFS", "LD_PRELOAD", "LD_LIBRARY_PATH", "PATH", "SHELL", "BASH_ENV",
	"ENV", "PYTHONPATH", "NODE_PATH", "PERL5LIB", "RUBYLIB",
}

var controlCharsPattern = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)

func ValidateCommandSecurity(command string) *SecurityValidationResult {
	if command == "" {
		return &SecurityValidationResult{Allowed: false, Reason: "empty command", ErrorCode: "EMPTY_COMMAND"}
	}

	if controlCharsPattern.MatchString(command) {
		return &SecurityValidationResult{Allowed: false, Reason: "command contains control characters", ErrorCode: "CONTROL_CHARS"}
	}

	for _, dp := range dangerousPatterns {
		if dp.pattern.MatchString(command) {
			return &SecurityValidationResult{
				Allowed:   false,
				Reason:    "dangerous pattern detected: " + dp.message,
				ErrorCode: "DANGEROUS_PATTERN",
			}
		}
	}

	baseCmd := extractBaseCommand(command)
	if zshDangerousCommands[baseCmd] {
		return &SecurityValidationResult{
			Allowed:   false,
			Reason:    "Zsh dangerous command blocked: " + baseCmd,
			ErrorCode: "ZSH_DANGEROUS_COMMAND",
		}
	}

	for _, v := range dangerousVariables {
		if strings.Contains(command, v+"=") || strings.Contains(command, "$"+v) {
			return &SecurityValidationResult{
				Allowed:   false,
				Reason:    "dangerous variable assignment: " + v,
				ErrorCode: "DANGEROUS_VARIABLE",
			}
		}
	}

	if strings.Contains(command, "&&") || strings.Contains(command, "||") {
		result := validateCompoundCommand(command)
		if !result.Allowed {
			return result
		}
	}

	return &SecurityValidationResult{Allowed: true}
}

func extractBaseCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func validateCompoundCommand(command string) *SecurityValidationResult {
	parts := splitCommandWithOperators(command)
	for _, part := range parts {
		if isOperator(part) {
			continue
		}
		baseCmd := extractBaseCommand(part)
		if zshDangerousCommands[baseCmd] {
			return &SecurityValidationResult{
				Allowed:   false,
				Reason:    "Zsh dangerous command in compound: " + baseCmd,
				ErrorCode: "ZSH_DANGEROUS_COMPOUND",
			}
		}
	}
	return &SecurityValidationResult{Allowed: true}
}

func splitCommandWithOperators(command string) []string {
	var result []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i, ch := range command {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			current.WriteRune(ch)
			continue
		}

		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			current.WriteRune(ch)
			continue
		}

		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			current.WriteRune(ch)
			continue
		}

		if !inSingleQuote && !inDoubleQuote {
			if (ch == '&' && i+1 < len(command) && command[i+1] == '&') ||
				(ch == '|' && i+1 < len(command) && command[i+1] == '|') ||
				(ch == ';') {
				if current.Len() > 0 {
					result = append(result, strings.TrimSpace(current.String()))
					current.Reset()
				}
				if ch != ';' {
					result = append(result, string(ch)+string(command[i+1]))
				} else {
					result = append(result, string(ch))
				}
				continue
			}
		}

		current.WriteRune(ch)
	}

	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

func isOperator(s string) bool {
	return s == "&&" || s == "||" || s == ";"
}

type CommandClassification struct {
	IsSearch bool
	IsRead   bool
	IsList   bool
	IsSilent bool
}

var searchCommands = map[string]bool{
	"find": true, "grep": true, "rg": true, "ag": true,
	"ack": true, "locate": true, "which": true, "whereis": true,
}

var readCommands = map[string]bool{
	"cat": true, "head": true, "tail": true, "less": true,
	"more": true, "wc": true, "stat": true, "file": true,
	"strings": true, "jq": true, "awk": true, "cut": true,
	"sort": true, "uniq": true, "tr": true,
}

var listCommands = map[string]bool{
	"ls": true, "tree": true, "du": true,
}

var silentCommands = map[string]bool{
	"mv": true, "cp": true, "rm": true, "mkdir": true,
	"rmdir": true, "chmod": true, "chown": true, "chgrp": true,
	"touch": true, "ln": true, "cd": true, "export": true,
	"unset": true, "wait": true,
}

var semanticNeutralCommands = map[string]bool{
	"echo": true, "printf": true, "true": true, "false": true, ":": true,
}

func ClassifyCommand(command string) CommandClassification {
	parts := splitCommandWithOperators(command)
	result := CommandClassification{}
	hasNonNeutralCommand := false

	for _, part := range parts {
		if isOperator(part) {
			continue
		}

		baseCmd := extractBaseCommand(part)
		if baseCmd == "" {
			continue
		}

		if semanticNeutralCommands[baseCmd] {
			continue
		}

		hasNonNeutralCommand = true

		if searchCommands[baseCmd] {
			result.IsSearch = true
		}
		if readCommands[baseCmd] {
			result.IsRead = true
		}
		if listCommands[baseCmd] {
			result.IsList = true
		}
		if silentCommands[baseCmd] {
			result.IsSilent = true
		}

		if !searchCommands[baseCmd] && !readCommands[baseCmd] && !listCommands[baseCmd] {
			return CommandClassification{}
		}
	}

	if !hasNonNeutralCommand {
		return CommandClassification{}
	}

	return result
}

func ValidatePathInCommand(command, workDir string) *SecurityValidationResult {
	paths := extractPathsFromCommand(command)
	for _, path := range paths {
		if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~") {
			if !isPathWithinWorkdir(path, workDir) {
				return &SecurityValidationResult{
					Allowed:   false,
					Reason:    "path outside workspace: " + path,
					ErrorCode: "PATH_OUTSIDE_WORKSPACE",
				}
			}
		}
	}
	return &SecurityValidationResult{Allowed: true}
}

func extractPathsFromCommand(command string) []string {
	var paths []string
	parts := strings.Fields(command)
	for _, part := range parts {
		if strings.HasPrefix(part, "/") || strings.HasPrefix(part, "./") || strings.HasPrefix(part, "~") {
			cleanPath := strings.Trim(part, `'"`)
			paths = append(paths, cleanPath)
		}
	}
	return paths
}

func isPathWithinWorkdir(path, workDir string) bool {
	if workDir == "" {
		return true
	}
	return strings.HasPrefix(path, workDir)
}

func IsDestructiveCommand(command string) bool {
	destructiveCommands := []string{
		"rm", "rmdir", "dd", "shred", "wipe",
		"mkfs", "fdisk", "parted", "format",
		"git push --force", "git reset --hard",
		"DROP", "DELETE", "TRUNCATE",
	}

	lowerCmd := strings.ToLower(command)
	for _, dc := range destructiveCommands {
		if strings.Contains(lowerCmd, strings.ToLower(dc)) {
			return true
		}
	}
	return false
}

func IsGitCommand(command string) bool {
	return strings.HasPrefix(strings.TrimSpace(command), "git ")
}
