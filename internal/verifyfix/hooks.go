package verifyfix

import (
	"context"
	"sync"
)

type EditHook struct {
	verifier *Verifier
	fixLoop  *FixLoop
	enabled  bool
	mu       sync.Mutex
}

func NewEditHook(verifier *Verifier, fixLoop *FixLoop) *EditHook {
	return &EditHook{
		verifier: verifier,
		fixLoop:  fixLoop,
		enabled:  true,
	}
}

func (h *EditHook) PostEdit(ctx context.Context, filePath string) *VerificationResult {
	if !h.isEnabled() {
		return nil
	}

	result := h.verifier.VerifyBuildOnly(ctx)
	if result.Success {
		return nil
	}

	return result
}

func (h *EditHook) PostEditAutoFix(ctx context.Context, filePath string, fixGenerator func(ctx context.Context, errors []BuildError) ([]FileEdit, error)) *FixLoopResult {
	if !h.isEnabled() {
		return nil
	}

	result := h.verifier.VerifyBuildOnly(ctx)
	if result.Success {
		return nil
	}

	loop := h.fixLoop
	if loop == nil {
		loop = NewFixLoop(FixLoopConfig{
			Verifier:    h.verifier,
			MaxRetries:  3,
			GenerateFix: fixGenerator,
		})
	} else {
		config := loop.config
		if config.GenerateFix == nil {
			config.GenerateFix = fixGenerator
		}
		loop = NewFixLoop(config)
	}

	fixResult, _ := loop.RunFromErrors(ctx, result.AllErrors)
	return fixResult
}

func (h *EditHook) Enable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = true
}

func (h *EditHook) Disable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = false
}

func (h *EditHook) IsEnabled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enabled
}

func (h *EditHook) isEnabled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enabled
}

var DefaultEditHook *EditHook

func InitDefaultHook(config VerifierConfig) {
	verifier := NewVerifier(config)
	DefaultEditHook = NewEditHook(verifier, nil)
}
