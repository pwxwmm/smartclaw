package verifyfix

import (
	"fmt"
	"regexp"
	"strings"
)

type BuildError struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Col      int    `json:"col,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

var goBuildRe = regexp.MustCompile(`^([\w./-]+\.go):(\d+)(?::(\d+))?:\s+(.+)$`) // file.go:line:col: msg or file.go:line: msg
var goTestFailRe = regexp.MustCompile(`^--- FAIL:\s+(\S+)`)                     // --- FAIL: TestName
var goTestFileRe = regexp.MustCompile(`^([\w./-]+_test\.go):(\d+):\s+(.+)$`) // file_test.go:line: msg
var lintRe = regexp.MustCompile(`^([\w./-]+\.go):(\d+):(\d+):\s+(.+)$`)         // file.go:line:col: msg

func ParseGoBuildOutput(output string) []BuildError {
	var errors []BuildError
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := goBuildRe.FindStringSubmatch(line); m != nil {
			be := BuildError{
				File:    m[1],
				Message: m[4],
			}
			be.Line = atoi(m[2])
			be.Col = atoi(m[3])
			be.Severity = classifyBuildSeverity(m[4])
			errors = append(errors, be)
		}
	}
	return errors
}

func ParseGoTestOutput(output string) []BuildError {
	var errors []BuildError
	var currentTest string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if m := goTestFailRe.FindStringSubmatch(line); m != nil {
			currentTest = m[1]
			errors = append(errors, BuildError{
				Message:  fmt.Sprintf("test failed: %s", m[1]),
				Severity: "error",
			})
			continue
		}

		if m := goTestFileRe.FindStringSubmatch(line); m != nil {
			be := BuildError{
				File:     m[1],
				Line:     atoi(m[2]),
				Message:  m[3],
				Severity: "error",
			}
			if currentTest != "" {
				be.Message = fmt.Sprintf("%s: %s", currentTest, be.Message)
			}
			errors = append(errors, be)
		}
	}
	return errors
}

func ParseLintOutput(output string) []BuildError {
	var errors []BuildError
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := lintRe.FindStringSubmatch(line); m != nil {
			be := BuildError{
				File:     m[1],
				Line:     atoi(m[2]),
				Col:      atoi(m[3]),
				Message:  m[4],
				Severity: classifyLintSeverity(m[4]),
			}
			errors = append(errors, be)
		}
	}
	return errors
}

func FormatErrors(errors []BuildError) string {
	if len(errors) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("The following errors were found after applying edits:\n\n")

	for _, e := range errors {
		switch {
		case e.File != "" && e.Line > 0 && e.Col > 0:
			sb.WriteString(fmt.Sprintf("%s:%d:%d: %s\n", e.File, e.Line, e.Col, e.Message))
		case e.File != "" && e.Line > 0:
			sb.WriteString(fmt.Sprintf("%s:%d: %s\n", e.File, e.Line, e.Message))
		case e.File != "":
			sb.WriteString(fmt.Sprintf("%s: %s\n", e.File, e.Message))
		default:
			sb.WriteString(fmt.Sprintf("%s\n", e.Message))
		}
	}

	sb.WriteString("\nPlease fix these errors by editing the relevant files.")
	return sb.String()
}

func classifyBuildSeverity(msg string) string {
	lower := strings.ToLower(msg)
	if strings.HasPrefix(lower, "undefined:") ||
		strings.HasPrefix(lower, "cannot use") ||
		strings.HasPrefix(lower, "too many errors") ||
		strings.HasPrefix(lower, "syntax error") ||
		strings.HasPrefix(lower, "missing return") ||
		strings.HasPrefix(lower, "imported and not used") ||
		strings.HasPrefix(lower, "declared but not used") ||
		strings.Contains(lower, "cannot refer to unexported") {
		return "error"
	}
	if strings.Contains(lower, "vet:") {
		return "warning"
	}
	if strings.HasPrefix(lower, "note:") {
		return "note"
	}
	return "error"
}

func classifyLintSeverity(msg string) string {
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") {
		return "error"
	}
	return "warning"
}

func atoi(s string) int {
	if s == "" {
		return 0
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
