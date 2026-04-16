package security

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
