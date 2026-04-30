package costguard

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/instructkr/smartclaw/internal/constants"
)

// PricingTier defines per-model token costs.
type PricingTier struct {
	InputPricePer1M  float64 // USD per 1M input tokens
	OutputPricePer1M float64 // USD per 1M output tokens
	CacheReadPer1M   float64 // USD per 1M cache read tokens
	CacheCreatePer1M float64 // USD per 1M cache create tokens
}

// BudgetConfig defines the spending limits and downgrade thresholds.
type BudgetConfig struct {
	DailyLimitUSD      float64 `json:"daily_limit_usd"`
	SessionLimitUSD    float64 `json:"session_limit_usd"`
	WarningThreshold   float64 `json:"warning_threshold"`   // fraction of budget (0.0-1.0)
	DowngradeThreshold float64 `json:"downgrade_threshold"` // fraction to trigger model downgrade
	DowngradeModel     string  `json:"downgrade_model"`     // cheaper model to fall back to
	Enabled            bool    `json:"enabled"`
	ShowPreEstimate    bool    `json:"show_pre_estimate"`
}

// DefaultBudgetConfig returns a sensible default configuration.
func DefaultBudgetConfig() BudgetConfig {
	return BudgetConfig{
		DailyLimitUSD:      50.0,
		SessionLimitUSD:    10.0,
		WarningThreshold:   constants.BudgetWarningThreshold,
		DowngradeThreshold: constants.BudgetDowngradeThreshold,
		DowngradeModel:     "claude-3-5-haiku-20241022",
		Enabled:            true,
	}
}

// CostSnapshot is a point-in-time view of spending.
type CostSnapshot struct {
	SessionInputTokens  int64   `json:"session_input_tokens"`
	SessionOutputTokens int64   `json:"session_output_tokens"`
	SessionCostUSD      float64 `json:"session_cost_usd"`
	DailyInputTokens    int64   `json:"daily_input_tokens"`
	DailyOutputTokens   int64   `json:"daily_output_tokens"`
	DailyCostUSD        float64 `json:"daily_cost_usd"`
	BudgetRemaining     float64 `json:"budget_remaining"`
	BudgetFraction      float64 `json:"budget_fraction"` // 0.0-1.0 of session budget
	ShouldWarn          bool    `json:"should_warn"`
	ShouldDowngrade     bool    `json:"should_downgrade"`
	ShouldBlock         bool    `json:"should_block"`
	CurrentModel        string  `json:"current_model"`
	DowngradeModel      string  `json:"downgrade_model,omitempty"`
}

// CostGuard tracks token usage and enforces budget limits.
type CostGuard struct {
	config BudgetConfig
	prices map[string]PricingTier
	db     *sql.DB

	sessionInputTokens  atomic.Int64
	sessionOutputTokens atomic.Int64
	sessionCostUSD      atomic.Int64 // stored as cents * 10000 for precision

	dailyMu           sync.Mutex
	dailyInputTokens  int64
	dailyOutputTokens int64
	dailyCostUSD      float64
	dailyResetDate    string

	onWarning   func(snapshot CostSnapshot)
	onDowngrade func(snapshot CostSnapshot)
	onBlock     func(snapshot CostSnapshot)
}

// NewCostGuard creates a CostGuard with the given config.
func NewCostGuard(config BudgetConfig) *CostGuard {
	cg := &CostGuard{
		config: config,
		prices: make(map[string]PricingTier),
	}
	cg.initPricing()
	return cg
}

// OnWarning registers a callback for when spending exceeds the warning threshold.
func (cg *CostGuard) OnWarning(fn func(CostSnapshot)) {
	cg.onWarning = fn
}

// OnDowngrade registers a callback for when spending triggers model downgrade.
func (cg *CostGuard) OnDowngrade(fn func(CostSnapshot)) {
	cg.onDowngrade = fn
}

// OnBlock registers a callback for when spending hits the hard limit.
func (cg *CostGuard) OnBlock(fn func(CostSnapshot)) {
	cg.onBlock = fn
}

// SetDB sets the database connection for cost analytics persistence.
func (cg *CostGuard) SetDB(db *sql.DB) {
	cg.db = db
}

// RecordUsage records token usage for a model and checks budget thresholds.
// Returns the current CostSnapshot after recording.
func (cg *CostGuard) RecordUsage(model string, inputTokens, outputTokens, cacheRead, cacheCreate int) CostSnapshot {
	cg.sessionInputTokens.Add(int64(inputTokens))
	cg.sessionOutputTokens.Add(int64(outputTokens))

	cost := cg.calculateCost(model, inputTokens, outputTokens, cacheRead, cacheCreate)
	cg.addSessionCost(cost)

	cg.dailyMu.Lock()
	cg.checkDailyReset()
	cg.dailyInputTokens += int64(inputTokens)
	cg.dailyOutputTokens += int64(outputTokens)
	cg.dailyCostUSD += cost
	cg.dailyMu.Unlock()

	snapshot := cg.Snapshot(model)

	if cg.config.Enabled {
		cg.checkThresholds(snapshot)
	}

	go RecordCostSnapshot(cg.db, model, int64(inputTokens), int64(outputTokens), int64(cacheRead), int64(cacheCreate), cost)

	return snapshot
}

// Snapshot returns the current spending state.
func (cg *CostGuard) Snapshot(currentModel string) CostSnapshot {
	sessInput := cg.sessionInputTokens.Load()
	sessOutput := cg.sessionOutputTokens.Load()
	sessCost := float64(cg.sessionCostUSD.Load()) / 10000.0

	cg.dailyMu.Lock()
	cg.checkDailyReset()
	dailyInput := cg.dailyInputTokens
	dailyOutput := cg.dailyOutputTokens
	dailyCost := cg.dailyCostUSD
	cg.dailyMu.Unlock()

	var budgetRemaining, budgetFraction float64
	var shouldWarn, shouldDowngrade, shouldBlock bool

	if cg.config.Enabled && cg.config.SessionLimitUSD > 0 {
		budgetRemaining = cg.config.SessionLimitUSD - sessCost
		budgetFraction = sessCost / cg.config.SessionLimitUSD

		shouldWarn = budgetFraction >= cg.config.WarningThreshold
		shouldDowngrade = budgetFraction >= cg.config.DowngradeThreshold
		shouldBlock = budgetFraction >= 1.0

		if budgetRemaining < 0 {
			budgetRemaining = 0
		}
		if budgetFraction > 1.0 {
			budgetFraction = 1.0
		}
	}

	return CostSnapshot{
		SessionInputTokens:  sessInput,
		SessionOutputTokens: sessOutput,
		SessionCostUSD:      sessCost,
		DailyInputTokens:    dailyInput,
		DailyOutputTokens:   dailyOutput,
		DailyCostUSD:        dailyCost,
		BudgetRemaining:     budgetRemaining,
		BudgetFraction:      budgetFraction,
		ShouldWarn:          shouldWarn,
		ShouldDowngrade:     shouldDowngrade,
		ShouldBlock:         shouldBlock,
		CurrentModel:        currentModel,
		DowngradeModel:      cg.config.DowngradeModel,
	}
}

// ResetSession clears the session-level counters (e.g., on new session).
func (cg *CostGuard) ResetSession() {
	cg.sessionInputTokens.Store(0)
	cg.sessionOutputTokens.Store(0)
	cg.sessionCostUSD.Store(0)
}

// IsEnabled returns whether the cost guard is active.
func (cg *CostGuard) IsEnabled() bool {
	return cg.config.Enabled
}

// SetEnabled toggles the cost guard.
func (cg *CostGuard) SetEnabled(enabled bool) {
	cg.config.Enabled = enabled
}

// GetConfig returns the current budget config.
func (cg *CostGuard) GetConfig() BudgetConfig {
	return cg.config
}

// SetConfig updates the budget config.
func (cg *CostGuard) SetConfig(config BudgetConfig) {
	cg.config = config
}

// RecommendedModel returns the model that should be used given current budget state.
// If we're above the downgrade threshold, it returns the cheaper model.
func (cg *CostGuard) RecommendedModel(currentModel string) string {
	if !cg.config.Enabled {
		return currentModel
	}
	snapshot := cg.Snapshot(currentModel)
	if snapshot.ShouldDowngrade && cg.config.DowngradeModel != "" {
		return cg.config.DowngradeModel
	}
	return currentModel
}

// CostBreakdown provides a detailed per-model cost itemization.
type CostBreakdown struct {
	InputCost  float64 `json:"inputCost"`
	OutputCost float64 `json:"outputCost"`
	Model      string  `json:"model"`
	InputRate  string  `json:"inputRate"`
	OutputRate string  `json:"outputRate"`
}

// CalculateCost returns the cost and breakdown for a model usage without recording it.
func (cg *CostGuard) CalculateCost(model string, inputTokens, outputTokens int) (float64, CostBreakdown) {
	tier, ok := cg.prices[model]
	if !ok {
		slog.Warn("cost guard: no pricing tier for model, using default", "model", model)
		tier = PricingTier{
			InputPricePer1M:  3.0,
			OutputPricePer1M: 15.0,
			CacheReadPer1M:   0.3,
			CacheCreatePer1M: 3.75,
		}
	}

	inputCost := float64(inputTokens) * tier.InputPricePer1M / 1_000_000
	outputCost := float64(outputTokens) * tier.OutputPricePer1M / 1_000_000
	total := inputCost + outputCost

	breakdown := CostBreakdown{
		InputCost:  inputCost,
		OutputCost: outputCost,
		Model:      model,
		InputRate:  fmt.Sprintf("$%.2f/1M", tier.InputPricePer1M),
		OutputRate: fmt.Sprintf("$%.2f/1M", tier.OutputPricePer1M),
	}

	return total, breakdown
}

// GetPricingTier returns the pricing tier for a model, or false if not found.
func (cg *CostGuard) GetPricingTier(model string) (PricingTier, bool) {
	tier, ok := cg.prices[model]
	return tier, ok
}

func (cg *CostGuard) addSessionCost(cost float64) {
	cents := int64(cost * 10000)
	cg.sessionCostUSD.Add(cents)
}

func (cg *CostGuard) calculateCost(model string, inputTokens, outputTokens, cacheRead, cacheCreate int) float64 {
	tier, ok := cg.prices[model]
	if !ok {
		// Default pricing (Claude Sonnet rates)
		tier = PricingTier{
			InputPricePer1M:  3.0,
			OutputPricePer1M: 15.0,
			CacheReadPer1M:   0.3,
			CacheCreatePer1M: 3.75,
		}
	}

	inputCost := float64(inputTokens) * tier.InputPricePer1M / 1_000_000
	outputCost := float64(outputTokens) * tier.OutputPricePer1M / 1_000_000
	cacheReadCost := float64(cacheRead) * tier.CacheReadPer1M / 1_000_000
	cacheCreateCost := float64(cacheCreate) * tier.CacheCreatePer1M / 1_000_000

	return inputCost + outputCost + cacheReadCost + cacheCreateCost
}

func (cg *CostGuard) checkThresholds(snapshot CostSnapshot) {
	if snapshot.ShouldBlock && cg.onBlock != nil {
		slog.Warn("cost guard: budget exceeded, blocking", "cost", fmt.Sprintf("$%.4f", snapshot.SessionCostUSD), "limit", fmt.Sprintf("$%.2f", cg.config.SessionLimitUSD))
		go cg.onBlock(snapshot)
		return
	}

	if snapshot.ShouldDowngrade && cg.onDowngrade != nil {
		slog.Warn("cost guard: downgrade threshold reached", "cost", fmt.Sprintf("$%.4f", snapshot.SessionCostUSD), "fraction", fmt.Sprintf("%.0f%%", snapshot.BudgetFraction*100))
		go cg.onDowngrade(snapshot)
	}

	if snapshot.ShouldWarn && cg.onWarning != nil {
		slog.Info("cost guard: warning threshold reached", "cost", fmt.Sprintf("$%.4f", snapshot.SessionCostUSD), "fraction", fmt.Sprintf("%.0f%%", snapshot.BudgetFraction*100))
		go cg.onWarning(snapshot)
	}
}

func (cg *CostGuard) checkDailyReset() {
	today := time.Now().Format("2006-01-02")
	if cg.dailyResetDate != today {
		cg.dailyInputTokens = 0
		cg.dailyOutputTokens = 0
		cg.dailyCostUSD = 0
		cg.dailyResetDate = today
	}
}

// PreTaskEstimate returns a quick cost estimate based on model pricing and estimated tokens.
func (cg *CostGuard) PreTaskEstimate(model string, estimatedTokens int) *TaskCostEstimate {
	tier, ok := cg.prices[model]
	if !ok {
		tier = PricingTier{
			InputPricePer1M:  3.0,
			OutputPricePer1M: 15.0,
		}
	}

	inputTokens := estimatedTokens * 60 / 100
	outputTokens := estimatedTokens * 40 / 100
	inputCost := float64(inputTokens) * tier.InputPricePer1M / 1_000_000
	outputCost := float64(outputTokens) * tier.OutputPricePer1M / 1_000_000
	estimatedCost := inputCost + outputCost

	haikuTier, haikuOk := cg.prices["claude-3-5-haiku-20241022"]
	if !haikuOk {
		haikuTier = PricingTier{InputPricePer1M: 0.8, OutputPricePer1M: 4.0}
	}
	haikuInputCost := float64(inputTokens) * haikuTier.InputPricePer1M / 1_000_000
	haikuOutputCost := float64(outputTokens) * haikuTier.OutputPricePer1M / 1_000_000
	haikuCost := haikuInputCost + haikuOutputCost

	return &TaskCostEstimate{
		TaskType:      "general",
		EstimatedCost: estimatedCost,
		Confidence:    0.6,
		CheaperAlt:    "claude-3-5-haiku-20241022",
		CheaperCost:   haikuCost,
	}
}

func (cg *CostGuard) initPricing() {
	cg.prices = map[string]PricingTier{
		// Claude models
		"claude-sonnet-4-20250514": {
			InputPricePer1M:  3.0,
			OutputPricePer1M: 15.0,
			CacheReadPer1M:   0.3,
			CacheCreatePer1M: 3.75,
		},
		"claude-sonnet-4-5": {
			InputPricePer1M:  3.0,
			OutputPricePer1M: 15.0,
			CacheReadPer1M:   0.3,
			CacheCreatePer1M: 3.75,
		},
		"claude-3-5-sonnet-20241022": {
			InputPricePer1M:  3.0,
			OutputPricePer1M: 15.0,
			CacheReadPer1M:   0.3,
			CacheCreatePer1M: 3.75,
		},
		"claude-3-5-haiku-20241022": {
			InputPricePer1M:  0.8,
			OutputPricePer1M: 4.0,
			CacheReadPer1M:   0.08,
			CacheCreatePer1M: 1.0,
		},
		"claude-opus-4-20250514": {
			InputPricePer1M:  15.0,
			OutputPricePer1M: 75.0,
			CacheReadPer1M:   1.5,
			CacheCreatePer1M: 18.75,
		},
		// GPT models
		"gpt-4o": {
			InputPricePer1M:  2.5,
			OutputPricePer1M: 10.0,
			CacheReadPer1M:   1.25,
			CacheCreatePer1M: 2.5,
		},
		"gpt-4o-mini": {
			InputPricePer1M:  0.15,
			OutputPricePer1M: 0.6,
			CacheReadPer1M:   0.075,
			CacheCreatePer1M: 0.15,
		},
		// Gemini models
		"gemini-2.5-pro": {
			InputPricePer1M:  1.25,
			OutputPricePer1M: 10.0,
			CacheReadPer1M:   0.31,
			CacheCreatePer1M: 1.25,
		},
		"gemini-2.5-flash": {
			InputPricePer1M:  0.15,
			OutputPricePer1M: 0.6,
			CacheReadPer1M:   0.0375,
			CacheCreatePer1M: 0.15,
		},
		// GLM models
		"glm-4-plus": {
			InputPricePer1M:  2.0,
			OutputPricePer1M: 8.0,
			CacheReadPer1M:   0.2,
			CacheCreatePer1M: 2.5,
		},
		// Internal
		"sre-model": {
			InputPricePer1M:  0.0,
			OutputPricePer1M: 0.0,
			CacheReadPer1M:   0.0,
			CacheCreatePer1M: 0.0,
		},
	}
}
