package shadow

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type VerificationResult struct {
	Success     bool
	BuildOutput string
	LintOutput  string
	TestOutput  string
	Errors      []BuildError
	Duration    time.Duration
}

type BuildError struct {
	File     string
	Line     int
	Message  string
	Severity string
}

type VerifierConfig struct {
	BuildCmd   string
	LintCmd    string
	TestCmd    string
	Timeout    time.Duration
	WorkingDir string
}

type Verifier struct {
	config VerifierConfig
}

func NewVerifier(config VerifierConfig) *Verifier {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	if config.BuildCmd == "" {
		config.BuildCmd = "go build ./..."
	}
	return &Verifier{config: config}
}

var goErrorRe = regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*(.*)$`)

func parseGoErrors(output string) []BuildError {
	var errs []BuildError
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		m := goErrorRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var lineNum int
		fmt.Sscanf(m[2], "%d", &lineNum)
		severity := "error"
		if strings.Contains(m[4], "warning") || strings.HasPrefix(line, "warning:") {
			severity = "warning"
		}
		errs = append(errs, BuildError{
			File:     m[1],
			Line:     lineNum,
			Message:  m[4],
			Severity: severity,
		})
	}
	return errs
}

func (v *Verifier) runCommand(ctx context.Context, cmdStr string) (string, error) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", nil
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = v.config.WorkingDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (v *Verifier) Verify(ctx context.Context) *VerificationResult {
	start := time.Now()
	result := &VerificationResult{}

	ctx, cancel := context.WithTimeout(ctx, v.config.Timeout)
	defer cancel()

	buildOut, buildErr := v.runCommand(ctx, v.config.BuildCmd)
	result.BuildOutput = buildOut
	if buildErr != nil {
		result.Errors = parseGoErrors(buildOut)
		result.Duration = time.Since(start)
		return result
	}

	if v.config.LintCmd != "" {
		lintOut, _ := v.runCommand(ctx, v.config.LintCmd)
		result.LintOutput = lintOut
	}

	if v.config.TestCmd != "" {
		testOut, testErr := v.runCommand(ctx, v.config.TestCmd)
		result.TestOutput = testOut
		if testErr != nil {
			result.Errors = parseGoErrors(testOut)
			result.Duration = time.Since(start)
			return result
		}
	}

	result.Success = true
	result.Duration = time.Since(start)
	return result
}
