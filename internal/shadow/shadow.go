package shadow

import (
	"context"
	"fmt"
)

type Result struct {
	Success      bool
	Diff         string
	Verification *VerificationResult
}

func VerifyAndApply(ctx context.Context, projectRoot string, changes []FileChange) (*Result, error) {
	sw, err := NewShadowWorkspace(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("creating shadow workspace: %w", err)
	}

	cleanupOnError := func() {
		sw.Cleanup()
	}

	if err := sw.ApplyChanges(changes); err != nil {
		cleanupOnError()
		return nil, fmt.Errorf("applying changes to shadow: %w", err)
	}

	verifier := NewVerifier(VerifierConfig{
		WorkingDir: sw.shadowPath,
	})

	vr := verifier.Verify(ctx)

	if !vr.Success {
		cleanupOnError()
		return &Result{
			Success:      false,
			Verification: vr,
		}, nil
	}

	diff, err := sw.GetDiff()
	if err != nil {
		cleanupOnError()
		return nil, fmt.Errorf("getting diff: %w", err)
	}

	return &Result{
		Success:      true,
		Diff:         diff,
		Verification: vr,
	}, nil
}
