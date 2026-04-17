package verifyfix

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type VerifierConfig struct {
	BuildCmd    string
	LintCmd     string
	TestCmd     string
	TestPattern string
	Timeout     time.Duration
	WorkingDir  string
}

func DefaultVerifierConfig() VerifierConfig {
	return VerifierConfig{
		BuildCmd: "go build ./...",
		Timeout:  120 * time.Second,
	}
}

type VerificationResult struct {
	Success     bool
	BuildErrors []BuildError
	LintErrors  []BuildError
	TestErrors  []BuildError
	AllErrors   []BuildError
	BuildOutput string
	LintOutput  string
	TestOutput  string
	Duration    time.Duration
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

func (v *Verifier) Verify(ctx context.Context) *VerificationResult {
	start := time.Now()
	result := &VerificationResult{}

	buildOk := v.runBuild(ctx, result)
	if !buildOk {
		result.Duration = time.Since(start)
		result.AllErrors = append(result.AllErrors, result.BuildErrors...)
		return result
	}

	if v.config.LintCmd != "" {
		v.runLint(ctx, result)
	}

	if v.config.TestCmd != "" {
		v.runTest(ctx, result)
	}

	result.Duration = time.Since(start)
	result.AllErrors = append(result.AllErrors, result.BuildErrors...)
	result.AllErrors = append(result.AllErrors, result.LintErrors...)
	result.AllErrors = append(result.AllErrors, result.TestErrors...)
	result.Success = len(result.AllErrors) == 0

	return result
}

func (v *Verifier) VerifyBuildOnly(ctx context.Context) *VerificationResult {
	start := time.Now()
	result := &VerificationResult{}

	v.runBuild(ctx, result)

	result.Duration = time.Since(start)
	result.AllErrors = append(result.AllErrors, result.BuildErrors...)
	result.Success = len(result.AllErrors) == 0

	return result
}

func (v *Verifier) VerifyFile(ctx context.Context, filePath string) *VerificationResult {
	start := time.Now()
	result := &VerificationResult{}

	pkg := derivePackageFromPath(filePath)
	buildCmd := fmt.Sprintf("go vet %s", pkg)
	if pkg == "" {
		buildCmd = v.config.BuildCmd
	}

	output, exitCode, _ := v.runCommand(ctx, buildCmd)
	result.BuildOutput = output

	if exitCode != 0 {
		result.BuildErrors = ParseGoBuildOutput(output)
	}

	result.Duration = time.Since(start)
	result.AllErrors = append(result.AllErrors, result.BuildErrors...)
	result.Success = len(result.AllErrors) == 0

	return result
}

func (v *Verifier) runBuild(ctx context.Context, result *VerificationResult) bool {
	output, exitCode, timedOut := v.runCommand(ctx, v.config.BuildCmd)
	result.BuildOutput = output

	if timedOut {
		result.BuildErrors = []BuildError{{
			Message:  fmt.Sprintf("build timed out after %s", v.config.Timeout),
			Severity: "error",
		}}
		return false
	}

	if exitCode != 0 {
		result.BuildErrors = ParseGoBuildOutput(output)
		return false
	}

	return true
}

func (v *Verifier) runLint(ctx context.Context, result *VerificationResult) {
	output, exitCode, timedOut := v.runCommand(ctx, v.config.LintCmd)
	result.LintOutput = output

	if timedOut {
		result.LintErrors = []BuildError{{
			Message:  fmt.Sprintf("lint timed out after %s", v.config.Timeout),
			Severity: "warning",
		}}
		return
	}

	if exitCode != 0 {
		result.LintErrors = ParseLintOutput(output)
	}
}

func (v *Verifier) runTest(ctx context.Context, result *VerificationResult) {
	testCmd := v.config.TestCmd
	if v.config.TestPattern != "" {
		testCmd = fmt.Sprintf("%s -run %s", testCmd, v.config.TestPattern)
	}

	output, exitCode, timedOut := v.runCommand(ctx, testCmd)
	result.TestOutput = output

	if timedOut {
		result.TestErrors = []BuildError{{
			Message:  fmt.Sprintf("tests timed out after %s", v.config.Timeout),
			Severity: "error",
		}}
		return
	}

	if exitCode != 0 {
		result.TestErrors = ParseGoTestOutput(output)
	}
}

func (v *Verifier) runCommand(ctx context.Context, cmdStr string) (output string, exitCode int, timedOut bool) {
	ctx, cancel := context.WithTimeout(ctx, v.config.Timeout)
	defer cancel()

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", 0, false
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if v.config.WorkingDir != "" {
		cmd.Dir = v.config.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String() + stderr.String(), -1, true
	}

	combined := stdout.String()
	if stderr.Len() > 0 {
		if combined != "" {
			combined += "\n"
		}
		combined += stderr.String()
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return combined, exitErr.ExitCode(), false
		}
		return combined, 1, false
	}

	return combined, 0, false
}

func derivePackageFromPath(filePath string) string {
	slash := strings.LastIndex(filePath, "/")
	if slash == -1 {
		return ""
	}
	return "./" + filePath[:slash] + "/..."
}
