package verifyfix

import (
	"strings"
	"testing"
)

func TestParseGoBuildOutput_SyntaxError(t *testing.T) {
	output := "main.go:42: syntax error: unexpected semicolon"
	errors := ParseGoBuildOutput(output)

	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].File != "main.go" {
		t.Errorf("expected File=main.go, got %q", errors[0].File)
	}
	if errors[0].Line != 42 {
		t.Errorf("expected Line=42, got %d", errors[0].Line)
	}
	if errors[0].Message != "syntax error: unexpected semicolon" {
		t.Errorf("unexpected Message: %q", errors[0].Message)
	}
	if errors[0].Severity != "error" {
		t.Errorf("expected severity=error, got %q", errors[0].Severity)
	}
}

func TestParseGoBuildOutput_WithColumn(t *testing.T) {
	output := "pkg/handler.go:10:5: undefined: Foo"
	errors := ParseGoBuildOutput(output)

	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].File != "pkg/handler.go" {
		t.Errorf("expected File=pkg/handler.go, got %q", errors[0].File)
	}
	if errors[0].Line != 10 {
		t.Errorf("expected Line=10, got %d", errors[0].Line)
	}
	if errors[0].Col != 5 {
		t.Errorf("expected Col=5, got %d", errors[0].Col)
	}
	if errors[0].Severity != "error" {
		t.Errorf("expected severity=error for 'undefined', got %q", errors[0].Severity)
	}
}

func TestParseGoBuildOutput_MultipleLines(t *testing.T) {
	output := `file1.go:1: syntax error
file2.go:5:3: cannot use x as int
some noise line
file3.go:10: missing return at end of function
`
	errors := ParseGoBuildOutput(output)

	if len(errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(errors))
	}
	if errors[0].File != "file1.go" {
		t.Errorf("errors[0].File = %q", errors[0].File)
	}
	if errors[1].File != "file2.go" {
		t.Errorf("errors[1].File = %q", errors[1].File)
	}
	if errors[2].File != "file3.go" {
		t.Errorf("errors[2].File = %q", errors[2].File)
	}
}

func TestParseGoBuildOutput_Empty(t *testing.T) {
	errors := ParseGoBuildOutput("")
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestParseGoBuildOutput_VetWarning(t *testing.T) {
	output := "main.go:20: vet: possible formatting directive"
	errors := ParseGoBuildOutput(output)

	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].Severity != "warning" {
		t.Errorf("expected severity=warning for vet, got %q", errors[0].Severity)
	}
}

func TestParseGoTestOutput_FailLine(t *testing.T) {
	output := `--- FAIL: TestX (0.00s)
    handler_test.go:15: unexpected value
    handler_test.go:16: another failure
`
	errors := ParseGoTestOutput(output)

	if len(errors) == 0 {
		t.Fatal("expected at least 1 error")
	}

	foundFail := false
	foundFile := false
	for _, e := range errors {
		if strings.Contains(e.Message, "test failed: TestX") {
			foundFail = true
		}
		if e.File == "handler_test.go" && e.Line == 15 {
			foundFile = true
		}
	}
	if !foundFail {
		t.Error("expected a 'test failed: TestX' error")
	}
	if !foundFile {
		t.Error("expected handler_test.go:15 error")
	}
}

func TestParseGoTestOutput_MultipleFails(t *testing.T) {
	output := `--- FAIL: TestA (0.00s)
    a_test.go:10: fail a
--- FAIL: TestB (0.00s)
    b_test.go:20: fail b
`
	errors := ParseGoTestOutput(output)

	failCount := 0
	for _, e := range errors {
		if strings.HasPrefix(e.Message, "test failed:") {
			failCount++
		}
	}
	if failCount != 2 {
		t.Errorf("expected 2 test failures, got %d", failCount)
	}
}

func TestParseGoTestOutput_Empty(t *testing.T) {
	errors := ParseGoTestOutput("")
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestParseGoTestOutput_PassOnly(t *testing.T) {
	output := `--- PASS: TestOk (0.00s)
ok  github.com/example/pkg  0.123s
`
	errors := ParseGoTestOutput(output)
	if len(errors) != 0 {
		t.Errorf("expected 0 errors for passing tests, got %d", len(errors))
	}
}

func TestFormatErrors_WithAllFields(t *testing.T) {
	errors := []BuildError{
		{File: "main.go", Line: 42, Col: 5, Message: "syntax error", Severity: "error"},
	}
	result := FormatErrors(errors)

	if !strings.Contains(result, "main.go:42:5: syntax error") {
		t.Errorf("expected formatted line with file:line:col, got:\n%s", result)
	}
	if !strings.Contains(result, "errors were found after applying edits") {
		t.Errorf("expected header, got:\n%s", result)
	}
	if !strings.Contains(result, "Please fix these errors") {
		t.Errorf("expected footer, got:\n%s", result)
	}
}

func TestFormatErrors_LineOnly(t *testing.T) {
	errors := []BuildError{
		{File: "main.go", Line: 10, Message: "undefined: Foo", Severity: "error"},
	}
	result := FormatErrors(errors)

	if !strings.Contains(result, "main.go:10: undefined: Foo") {
		t.Errorf("expected formatted line with file:line, got:\n%s", result)
	}
}

func TestFormatErrors_FileOnly(t *testing.T) {
	errors := []BuildError{
		{File: "main.go", Message: "build failed", Severity: "error"},
	}
	result := FormatErrors(errors)

	if !strings.Contains(result, "main.go: build failed") {
		t.Errorf("expected formatted line with file only, got:\n%s", result)
	}
}

func TestFormatErrors_MessageOnly(t *testing.T) {
	errors := []BuildError{
		{Message: "test failed: TestX", Severity: "error"},
	}
	result := FormatErrors(errors)

	if !strings.Contains(result, "test failed: TestX") {
		t.Errorf("expected message-only line, got:\n%s", result)
	}
}

func TestFormatErrors_Empty(t *testing.T) {
	result := FormatErrors(nil)
	if result != "" {
		t.Errorf("expected empty string for nil errors, got %q", result)
	}
}

func TestFormatErrors_Multiple(t *testing.T) {
	errors := []BuildError{
		{File: "a.go", Line: 1, Col: 1, Message: "error1", Severity: "error"},
		{File: "b.go", Line: 2, Message: "error2", Severity: "error"},
	}
	result := FormatErrors(errors)

	if !strings.Contains(result, "a.go:1:1: error1") {
		t.Errorf("missing a.go line, got:\n%s", result)
	}
	if !strings.Contains(result, "b.go:2: error2") {
		t.Errorf("missing b.go line, got:\n%s", result)
	}
}

func TestParseLintOutput(t *testing.T) {
	output := "main.go:10:5: S1000: should use fmt.Println"
	errors := ParseLintOutput(output)

	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].File != "main.go" {
		t.Errorf("expected File=main.go, got %q", errors[0].File)
	}
	if errors[0].Line != 10 {
		t.Errorf("expected Line=10, got %d", errors[0].Line)
	}
	if errors[0].Col != 5 {
		t.Errorf("expected Col=5, got %d", errors[0].Col)
	}
	if errors[0].Severity != "warning" {
		t.Errorf("expected severity=warning for lint, got %q", errors[0].Severity)
	}
}

func TestClassifyBuildSeverity_SyntaxError(t *testing.T) {
	errors := ParseGoBuildOutput("x.go:1: syntax error: unexpected {")
	if len(errors) != 1 || errors[0].Severity != "error" {
		t.Errorf("syntax error should be severity=error")
	}
}

func TestClassifyBuildSeverity_ImportedNotUsed(t *testing.T) {
	errors := ParseGoBuildOutput("x.go:3: imported and not used: \"fmt\"")
	if len(errors) != 1 || errors[0].Severity != "error" {
		t.Errorf("imported and not used should be severity=error")
	}
}
