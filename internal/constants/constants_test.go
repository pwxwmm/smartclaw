package constants

import "testing"

func TestCoreConstants(t *testing.T) {
	if Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", Version, "1.0.0")
	}
	if Name != "SmartClaw" {
		t.Errorf("Name = %q, want %q", Name, "SmartClaw")
	}
	if DefaultModel != "sre-model" {
		t.Errorf("DefaultModel = %q, want %q", DefaultModel, "sre-model")
	}
}

func TestNumericConstants(t *testing.T) {
	if MaxTokensDefault != 4096 {
		t.Errorf("MaxTokensDefault = %d, want 4096", MaxTokensDefault)
	}
	if MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", MaxRetries)
	}
}

func TestTokenPricing(t *testing.T) {
	if TokenPricingInput != 0.000015 {
		t.Errorf("TokenPricingInput = %v, want 0.000015", TokenPricingInput)
	}
	if TokenPricingOutput != 0.000075 {
		t.Errorf("TokenPricingOutput = %v, want 0.000075", TokenPricingOutput)
	}
}

func TestPermissionModes(t *testing.T) {
	if PermissionReadOnly != "read-only" {
		t.Errorf("PermissionReadOnly = %q, want %q", PermissionReadOnly, "read-only")
	}
	if PermissionWorkspaceWrite != "workspace-write" {
		t.Errorf("PermissionWorkspaceWrite = %q, want %q", PermissionWorkspaceWrite, "workspace-write")
	}
	if PermissionDangerFull != "danger-full-access" {
		t.Errorf("PermissionDangerFull = %q, want %q", PermissionDangerFull, "danger-full-access")
	}
}

func TestThresholds(t *testing.T) {
	if SpeculativeSimilarityThreshold != 0.3 {
		t.Errorf("SpeculativeSimilarityThreshold = %v, want 0.3", SpeculativeSimilarityThreshold)
	}
	if BudgetWarningThreshold != 0.7 {
		t.Errorf("BudgetWarningThreshold = %v, want 0.7", BudgetWarningThreshold)
	}
	if BudgetDowngradeThreshold != 0.9 {
		t.Errorf("BudgetDowngradeThreshold = %v, want 0.9", BudgetDowngradeThreshold)
	}
}

func TestMemoryCharLimit(t *testing.T) {
	if MemoryCharLimit != 3575 {
		t.Errorf("MemoryCharLimit = %d, want 3575", MemoryCharLimit)
	}
}
