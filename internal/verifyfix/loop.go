package verifyfix

import (
	"context"
	"fmt"
	"os"
	"time"
)

type FileEdit struct {
	Path    string
	Content string
}

type FixLoopConfig struct {
	Verifier    *Verifier
	MaxRetries  int
	OnAttempt   func(attempt int, result *VerificationResult)
	GenerateFix func(ctx context.Context, errors []BuildError) ([]FileEdit, error)
}

type FixLoopResult struct {
	Success       bool
	Attempts      int
	FinalResult   *VerificationResult
	AppliedEdits  [][]FileEdit
	TotalDuration time.Duration
}

type FixLoop struct {
	config FixLoopConfig
}

func NewFixLoop(config FixLoopConfig) *FixLoop {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	return &FixLoop{config: config}
}

func (fl *FixLoop) Run(ctx context.Context) (*FixLoopResult, error) {
	result := fl.config.Verifier.Verify(ctx)
	if result.Success {
		return &FixLoopResult{
			Success:     true,
			Attempts:    1,
			FinalResult: result,
		}, nil
	}

	return fl.runFromErrors(ctx, result.AllErrors)
}

func (fl *FixLoop) RunFromErrors(ctx context.Context, initialErrors []BuildError) (*FixLoopResult, error) {
	return fl.runFromErrors(ctx, initialErrors)
}

func (fl *FixLoop) runFromErrors(ctx context.Context, errors []BuildError) (*FixLoopResult, error) {
	start := time.Now()
	flResult := &FixLoopResult{
		Attempts: 0,
	}

	currentErrors := errors

	for attempt := 1; attempt <= fl.config.MaxRetries; attempt++ {
		flResult.Attempts = attempt

		if fl.config.GenerateFix == nil {
			flResult.FinalResult = &VerificationResult{
				Success:   false,
				AllErrors: currentErrors,
			}
			flResult.TotalDuration = time.Since(start)
			return flResult, fmt.Errorf("no GenerateFix callback provided; cannot auto-fix %d errors", len(currentErrors))
		}

		edits, err := fl.config.GenerateFix(ctx, currentErrors)
		if err != nil {
			flResult.FinalResult = &VerificationResult{
				Success:   false,
				AllErrors: currentErrors,
			}
			flResult.TotalDuration = time.Since(start)
			return flResult, fmt.Errorf("GenerateFix failed on attempt %d: %w", attempt, err)
		}

		if len(edits) == 0 {
			flResult.FinalResult = &VerificationResult{
				Success:   false,
				AllErrors: currentErrors,
			}
			flResult.TotalDuration = time.Since(start)
			return flResult, fmt.Errorf("GenerateFix returned no edits on attempt %d", attempt)
		}

		if err := applyEdits(edits); err != nil {
			flResult.FinalResult = &VerificationResult{
				Success:   false,
				AllErrors: currentErrors,
			}
			flResult.TotalDuration = time.Since(start)
			return flResult, fmt.Errorf("failed to apply edits on attempt %d: %w", attempt, err)
		}

		flResult.AppliedEdits = append(flResult.AppliedEdits, edits)

		verifyResult := fl.config.Verifier.Verify(ctx)

		if fl.config.OnAttempt != nil {
			fl.config.OnAttempt(attempt, verifyResult)
		}

		if verifyResult.Success {
			flResult.Success = true
			flResult.FinalResult = verifyResult
			flResult.TotalDuration = time.Since(start)
			return flResult, nil
		}

		currentErrors = verifyResult.AllErrors
	}

	flResult.FinalResult = &VerificationResult{
		Success:   false,
		AllErrors: currentErrors,
	}
	flResult.TotalDuration = time.Since(start)
	return flResult, fmt.Errorf("fix loop exhausted %d retries; %d errors remain", fl.config.MaxRetries, len(currentErrors))
}

func applyEdits(edits []FileEdit) error {
	for _, edit := range edits {
		if edit.Path == "" {
			return fmt.Errorf("empty path in file edit")
		}
		if err := os.WriteFile(edit.Path, []byte(edit.Content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", edit.Path, err)
		}
	}
	return nil
}
