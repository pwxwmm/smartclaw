package costguard

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCostGuard_DefaultConfig(t *testing.T) {
	cfg := DefaultBudgetConfig()
	cg := NewCostGuard(cfg)

	if !cg.IsEnabled() {
		t.Error("expected CostGuard to be enabled by default")
	}
	if cfg.DailyLimitUSD != 50.0 {
		t.Errorf("expected DailyLimitUSD=50.0, got %f", cfg.DailyLimitUSD)
	}
	if cfg.SessionLimitUSD != 10.0 {
		t.Errorf("expected SessionLimitUSD=10.0, got %f", cfg.SessionLimitUSD)
	}
	if cfg.WarningThreshold != 0.7 {
		t.Errorf("expected WarningThreshold=0.7, got %f", cfg.WarningThreshold)
	}
	if cfg.DowngradeThreshold != 0.9 {
		t.Errorf("expected DowngradeThreshold=0.9, got %f", cfg.DowngradeThreshold)
	}
	if cfg.DowngradeModel != "claude-3-5-haiku-20241022" {
		t.Errorf("expected DowngradeModel=claude-3-5-haiku-20241022, got %s", cfg.DowngradeModel)
	}

	snap := cg.Snapshot("test-model")
	if snap.SessionCostUSD != 0 {
		t.Errorf("expected initial session cost 0, got %f", snap.SessionCostUSD)
	}
	if snap.SessionInputTokens != 0 {
		t.Errorf("expected initial input tokens 0, got %d", snap.SessionInputTokens)
	}
	if snap.SessionOutputTokens != 0 {
		t.Errorf("expected initial output tokens 0, got %d", snap.SessionOutputTokens)
	}
	if snap.ShouldBlock {
		t.Error("expected ShouldBlock=false initially")
	}
	if snap.ShouldWarn {
		t.Error("expected ShouldWarn=false initially")
	}
	if snap.ShouldDowngrade {
		t.Error("expected ShouldDowngrade=false initially")
	}
}

func TestRecordUsage_TracksTokensAndCost(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	snap := cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)

	if snap.SessionInputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", snap.SessionInputTokens)
	}
	if snap.SessionOutputTokens != 500 {
		t.Errorf("expected 500 output tokens, got %d", snap.SessionOutputTokens)
	}

	expectedInputCost := 1000.0 * 3.0 / 1_000_000
	expectedOutputCost := 500.0 * 15.0 / 1_000_000
	expectedCost := expectedInputCost + expectedOutputCost

	if snap.SessionCostUSD < expectedCost*0.99 || snap.SessionCostUSD > expectedCost*1.01 {
		t.Errorf("expected session cost ~%f, got %f", expectedCost, snap.SessionCostUSD)
	}
	if snap.DailyInputTokens != 1000 {
		t.Errorf("expected 1000 daily input tokens, got %d", snap.DailyInputTokens)
	}
	if snap.DailyOutputTokens != 500 {
		t.Errorf("expected 500 daily output tokens, got %d", snap.DailyOutputTokens)
	}
}

func TestRecordUsage_CumulativeTracking(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)
	snap := cg.RecordUsage("claude-3-5-sonnet-20241022", 2000, 1000, 0, 0)

	if snap.SessionInputTokens != 3000 {
		t.Errorf("expected 3000 cumulative input tokens, got %d", snap.SessionInputTokens)
	}
	if snap.SessionOutputTokens != 1500 {
		t.Errorf("expected 1500 cumulative output tokens, got %d", snap.SessionOutputTokens)
	}
}

func TestRecordUsage_CacheTokens(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	snapNoCache := cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)
	costNoCache := snapNoCache.SessionCostUSD

	cg.ResetSession()
	snapWithCache := cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 500, 200)
	costWithCache := snapWithCache.SessionCostUSD

	if costWithCache <= costNoCache {
		t.Errorf("expected cost with cache tokens (%f) > cost without cache (%f)", costWithCache, costNoCache)
	}
}

func TestCheckBudget_AllowsWhenUnderLimit(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	snap := cg.Snapshot("claude-3-5-sonnet-20241022")
	if snap.ShouldBlock {
		t.Error("expected ShouldBlock=false when well under limit")
	}
	if snap.BudgetFraction >= 0.7 {
		t.Errorf("expected budget fraction < 0.7, got %f", snap.BudgetFraction)
	}
}

func TestCheckBudget_BlocksWhenOverLimit(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    0.001, // very low limit
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	snap := cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)

	if !snap.ShouldBlock {
		t.Error("expected ShouldBlock=true when budget exceeded")
	}
	if snap.BudgetRemaining != 0 {
		t.Errorf("expected BudgetRemaining=0 when over limit, got %f", snap.BudgetRemaining)
	}
}

func TestWarningThreshold(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	var warnCalled atomic.Int32
	cg.OnWarning(func(s CostSnapshot) {
		warnCalled.Add(1)
	})

	// Use enough to cross 70% of $10 = $7.00
	// claude-3-5-sonnet: input=$3/1M, output=$15/1M
	// Need: inputTokens*3/1M + outputTokens*15/1M >= 7.00
	// e.g. input=100000, output=400000 => 0.3 + 6.0 = 6.3 < 7.0
	// Try input=200000, output=400000 => 0.6 + 6.0 = 6.6 < 7.0
	// Try input=0, output=500000 => 0 + 7.5 = 7.5 > 7.0 ✓
	snap := cg.RecordUsage("claude-3-5-sonnet-20241022", 0, 500000, 0, 0)
	time.Sleep(10 * time.Millisecond)

	if !snap.ShouldWarn {
		t.Error("expected ShouldWarn=true when spending >= 70%")
	}
	if snap.ShouldDowngrade {
		t.Error("expected ShouldDowngrade=false when spending < 90%")
	}
	if warnCalled.Load() == 0 {
		t.Error("expected warning callback to be called")
	}
}

func TestDowngradeThreshold(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	var downgradeCalled atomic.Int32
	cg.OnDowngrade(func(s CostSnapshot) {
		downgradeCalled.Add(1)
	})

	// Need spending >= 90% of $10 = $9.00
	// output=600000 => 9.0 exactly
	snap := cg.RecordUsage("claude-3-5-sonnet-20241022", 0, 600000, 0, 0)
	time.Sleep(10 * time.Millisecond)

	if !snap.ShouldWarn {
		t.Error("expected ShouldWarn=true when spending >= 70%")
	}
	if !snap.ShouldDowngrade {
		t.Error("expected ShouldDowngrade=true when spending >= 90%")
	}
	if snap.ShouldBlock {
		t.Error("expected ShouldBlock=false when spending < 100%")
	}
	if downgradeCalled.Load() == 0 {
		t.Error("expected downgrade callback to be called")
	}
}

func TestBlockThreshold(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	var blockCalled atomic.Int32
	cg.OnBlock(func(s CostSnapshot) {
		blockCalled.Add(1)
	})

	// Need spending >= 100% of $10 = $10.00
	// output=700000 => 10.5 > 10.0 ✓
	snap := cg.RecordUsage("claude-3-5-sonnet-20241022", 0, 700000, 0, 0)
	time.Sleep(10 * time.Millisecond)

	if !snap.ShouldBlock {
		t.Error("expected ShouldBlock=true when budget exceeded")
	}
	if blockCalled.Load() == 0 {
		t.Error("expected block callback to be called")
	}
}

func TestRecommendedModel_NoDowngradeWhenBudgetOK(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	model := cg.RecommendedModel("claude-opus-4-20250514")
	if model != "claude-opus-4-20250514" {
		t.Errorf("expected original model, got %s", model)
	}
}

func TestRecommendedModel_DowngradesWhenOverThreshold(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    0.001, // very low limit
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	// Record enough usage to trigger downgrade
	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)

	model := cg.RecommendedModel("claude-opus-4-20250514")
	if model != "claude-3-5-haiku-20241022" {
		t.Errorf("expected downgrade model, got %s", model)
	}
}

func TestRecommendedModel_DisabledGuard(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    0.001,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            false,
	}
	cg := NewCostGuard(cfg)

	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)

	model := cg.RecommendedModel("claude-opus-4-20250514")
	if model != "claude-opus-4-20250514" {
		t.Errorf("expected original model when disabled, got %s", model)
	}
}

func TestRecommendedModel_NoDowngradeModel(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    0.001,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)

	model := cg.RecommendedModel("claude-opus-4-20250514")
	if model != "claude-opus-4-20250514" {
		t.Errorf("expected original model when no downgrade model set, got %s", model)
	}
}

func TestResetSession(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)
	cg.ResetSession()

	snap := cg.Snapshot("test")
	if snap.SessionCostUSD != 0 {
		t.Errorf("expected session cost 0 after reset, got %f", snap.SessionCostUSD)
	}
	if snap.SessionInputTokens != 0 {
		t.Errorf("expected session input tokens 0 after reset, got %d", snap.SessionInputTokens)
	}
	if snap.SessionOutputTokens != 0 {
		t.Errorf("expected session output tokens 0 after reset, got %d", snap.SessionOutputTokens)
	}
}

func TestSetEnabled(t *testing.T) {
	cfg := DefaultBudgetConfig()
	cg := NewCostGuard(cfg)

	if !cg.IsEnabled() {
		t.Error("expected enabled initially")
	}

	cg.SetEnabled(false)
	if cg.IsEnabled() {
		t.Error("expected disabled after SetEnabled(false)")
	}

	cg.SetEnabled(true)
	if !cg.IsEnabled() {
		t.Error("expected enabled after SetEnabled(true)")
	}
}

func TestGetSetConfig(t *testing.T) {
	cfg := DefaultBudgetConfig()
	cg := NewCostGuard(cfg)

	newCfg := BudgetConfig{
		DailyLimitUSD:      200.0,
		SessionLimitUSD:    20.0,
		WarningThreshold:   0.5,
		DowngradeThreshold: 0.8,
		DowngradeModel:     "gpt-4o-mini",
		Enabled:            false,
	}
	cg.SetConfig(newCfg)

	got := cg.GetConfig()
	if got.DailyLimitUSD != 200.0 {
		t.Errorf("expected DailyLimitUSD=200.0, got %f", got.DailyLimitUSD)
	}
	if got.SessionLimitUSD != 20.0 {
		t.Errorf("expected SessionLimitUSD=20.0, got %f", got.SessionLimitUSD)
	}
	if got.DowngradeModel != "gpt-4o-mini" {
		t.Errorf("expected DowngradeModel=gpt-4o-mini, got %s", got.DowngradeModel)
	}
}

func TestCalculateCost(t *testing.T) {
	cfg := DefaultBudgetConfig()
	cg := NewCostGuard(cfg)

	cost, breakdown := cg.CalculateCost("claude-3-5-sonnet-20241022", 1000, 500)

	if breakdown.InputCost <= 0 {
		t.Errorf("expected positive input cost, got %f", breakdown.InputCost)
	}
	if breakdown.OutputCost <= 0 {
		t.Errorf("expected positive output cost, got %f", breakdown.OutputCost)
	}
	if cost != breakdown.InputCost+breakdown.OutputCost {
		t.Errorf("expected cost=%f to equal input+output=%f", cost, breakdown.InputCost+breakdown.OutputCost)
	}
	if breakdown.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected model in breakdown, got %s", breakdown.Model)
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	cfg := DefaultBudgetConfig()
	cg := NewCostGuard(cfg)

	cost, _ := cg.CalculateCost("unknown-model", 1000, 500)
	if cost <= 0 {
		t.Errorf("expected positive cost for unknown model (uses default pricing), got %f", cost)
	}
}

func TestGetPricingTier(t *testing.T) {
	cfg := DefaultBudgetConfig()
	cg := NewCostGuard(cfg)

	tier, ok := cg.GetPricingTier("claude-3-5-sonnet-20241022")
	if !ok {
		t.Error("expected to find pricing tier for claude-3-5-sonnet-20241022")
	}
	if tier.InputPricePer1M != 3.0 {
		t.Errorf("expected InputPricePer1M=3.0, got %f", tier.InputPricePer1M)
	}

	_, ok = cg.GetPricingTier("nonexistent-model")
	if ok {
		t.Error("expected not to find pricing tier for nonexistent model")
	}
}

func TestSnapshot_BudgetFractionCapped(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    0.001,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)
	snap := cg.Snapshot("test")

	if snap.BudgetFraction > 1.0 {
		t.Errorf("expected BudgetFraction capped at 1.0, got %f", snap.BudgetFraction)
	}
	if snap.BudgetRemaining != 0 {
		t.Errorf("expected BudgetRemaining=0 when over limit, got %f", snap.BudgetRemaining)
	}
}

func TestSnapshot_DisabledGuard(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    0.001,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            false,
	}
	cg := NewCostGuard(cfg)

	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)
	snap := cg.Snapshot("test")

	if snap.ShouldWarn {
		t.Error("expected ShouldWarn=false when guard is disabled")
	}
	if snap.ShouldDowngrade {
		t.Error("expected ShouldDowngrade=false when guard is disabled")
	}
	if snap.ShouldBlock {
		t.Error("expected ShouldBlock=false when guard is disabled")
	}
}

func TestThresholdCallbackOrder(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    0.001,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	var blockCalled atomic.Int32
	cg.OnBlock(func(s CostSnapshot) {
		blockCalled.Add(1)
	})

	// When blocked, only the block callback should fire (not warn/downgrade)
	cg.RecordUsage("claude-3-5-sonnet-20241022", 1000, 500, 0, 0)
	time.Sleep(10 * time.Millisecond)

	if blockCalled.Load() == 0 {
		t.Error("expected block callback when budget exceeded")
	}
}

func TestDifferentModelsDifferentPricing(t *testing.T) {
	cfg := BudgetConfig{
		DailyLimitUSD:      100.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   0.7,
		DowngradeThreshold: 0.9,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
	cg := NewCostGuard(cfg)

	_, sonnetBreakdown := cg.CalculateCost("claude-3-5-sonnet-20241022", 1000, 500)
	_, haikuBreakdown := cg.CalculateCost("claude-3-5-haiku-20241022", 1000, 500)

	if sonnetBreakdown.InputCost <= haikuBreakdown.InputCost {
		t.Error("expected sonnet input cost > haiku input cost")
	}
	if sonnetBreakdown.OutputCost <= haikuBreakdown.OutputCost {
		t.Error("expected sonnet output cost > haiku output cost")
	}
}
