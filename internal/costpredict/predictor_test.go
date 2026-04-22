package costpredict

import (
	"math"
	"strings"
	"testing"
)

func TestPredict_KnownModel(t *testing.T) {
	cp := NewCostPredictor()
	pred := cp.Predict(PredictionInput{
		Model:           "claude-sonnet-4-5",
		SystemPromptLen: 500,
		QueryLen:        200,
		HistoryLen:      3,
		ToolCount:       10,
	})

	if pred.EstimatedInputTokens <= 0 {
		t.Errorf("expected positive input tokens, got %d", pred.EstimatedInputTokens)
	}
	if pred.EstimatedOutputTokens <= 0 {
		t.Errorf("expected positive output tokens, got %d", pred.EstimatedOutputTokens)
	}
	if pred.EstimatedCostUSD <= 0 {
		t.Errorf("expected positive cost, got %f", pred.EstimatedCostUSD)
	}
	if pred.InputCostUSD <= 0 {
		t.Errorf("expected positive input cost, got %f", pred.InputCostUSD)
	}
	if pred.OutputCostUSD <= 0 {
		t.Errorf("expected positive output cost, got %f", pred.OutputCostUSD)
	}
	if math.Abs(pred.EstimatedCostUSD-(pred.InputCostUSD+pred.OutputCostUSD)) > 1e-10 {
		t.Errorf("expected total cost = input + output, got total=%f input=%f output=%f",
			pred.EstimatedCostUSD, pred.InputCostUSD, pred.OutputCostUSD)
	}
	if pred.Model != "claude-sonnet-4-5" {
		t.Errorf("expected model=claude-sonnet-4-5, got %s", pred.Model)
	}
}

func TestPredict_UnknownModel(t *testing.T) {
	cp := NewCostPredictor()
	pred := cp.Predict(PredictionInput{
		Model:           "totally-unknown-model",
		SystemPromptLen: 500,
		QueryLen:        200,
		HistoryLen:      3,
		ToolCount:       10,
	})

	if pred.EstimatedCostUSD <= 0 {
		t.Errorf("expected positive cost for unknown model (default pricing), got %f", pred.EstimatedCostUSD)
	}

	// Default pricing is sonnet: $3/1M input, $15/1M output
	// Should produce a non-zero cost
	inputCost := float64(pred.EstimatedInputTokens) * 3.0 / 1_000_000
	outputCost := float64(pred.EstimatedOutputTokens) * 15.0 / 1_000_000
	expectedTotal := inputCost + outputCost
	if math.Abs(pred.EstimatedCostUSD-expectedTotal) > 1e-10 {
		t.Errorf("expected cost %f for default pricing, got %f", expectedTotal, pred.EstimatedCostUSD)
	}
}

func TestPredict_MinimumTokenEstimates(t *testing.T) {
	cp := NewCostPredictor()
	pred := cp.Predict(PredictionInput{
		Model:           "claude-sonnet-4-5",
		SystemPromptLen: 0,
		QueryLen:        0,
		HistoryLen:      0,
		ToolCount:       0,
	})

	// Should floor at minimum estimates
	if pred.EstimatedInputTokens < 100 {
		t.Errorf("expected at least 100 input tokens (minimum), got %d", pred.EstimatedInputTokens)
	}
	if pred.EstimatedOutputTokens < 50 {
		t.Errorf("expected at least 50 output tokens (minimum), got %d", pred.EstimatedOutputTokens)
	}
}

func TestUpdateStats_AdjustsAverages(t *testing.T) {
	cp := NewCostPredictor()

	cp.UpdateStats("claude-sonnet-4-5", 1000, 500)
	stats := cp.GetStats("claude-sonnet-4-5")
	if stats == nil {
		t.Fatal("expected stats to exist after UpdateStats")
	}
	if stats.SampleCount != 1 {
		t.Errorf("expected SampleCount=1, got %d", stats.SampleCount)
	}
	if stats.AvgInputTokens != 1000 {
		t.Errorf("expected AvgInputTokens=1000, got %f", stats.AvgInputTokens)
	}
	if stats.AvgOutputTokens != 500 {
		t.Errorf("expected AvgOutputTokens=500, got %f", stats.AvgOutputTokens)
	}
	if stats.OutputInputRatio != 0.5 {
		t.Errorf("expected OutputInputRatio=0.5, got %f", stats.OutputInputRatio)
	}
}

func TestUpdateStats_MultipleCallsConverge(t *testing.T) {
	cp := NewCostPredictor()

	// Simulate consistent observations of 2000 input, 800 output
	for i := 0; i < 20; i++ {
		cp.UpdateStats("claude-sonnet-4-5", 2000, 800)
	}

	stats := cp.GetStats("claude-sonnet-4-5")
	if stats.SampleCount != 20 {
		t.Errorf("expected SampleCount=20, got %d", stats.SampleCount)
	}

	// EMA should converge toward the actual values
	tolerance := 100.0
	if math.Abs(stats.AvgInputTokens-2000) > tolerance {
		t.Errorf("expected AvgInputTokens near 2000, got %f", stats.AvgInputTokens)
	}
	if math.Abs(stats.AvgOutputTokens-800) > tolerance {
		t.Errorf("expected AvgOutputTokens near 800, got %f", stats.AvgOutputTokens)
	}
	if math.Abs(stats.OutputInputRatio-0.4) > 0.05 {
		t.Errorf("expected OutputInputRatio near 0.4, got %f", stats.OutputInputRatio)
	}
}

func TestConfidence_IncreasesWithObservations(t *testing.T) {
	cp := NewCostPredictor()
	input := PredictionInput{
		Model:           "claude-sonnet-4-5",
		SystemPromptLen: 100,
		QueryLen:        50,
	}

	pred0 := cp.Predict(input)
	if pred0.Confidence != 0.3 {
		t.Errorf("expected confidence 0.3 with no history, got %f", pred0.Confidence)
	}

	cp.UpdateStats("claude-sonnet-4-5", 1000, 500)
	pred1 := cp.Predict(input)
	if pred1.Confidence != 0.5 {
		t.Errorf("expected confidence 0.5 with 1 observation, got %f", pred1.Confidence)
	}

	for i := 0; i < 4; i++ {
		cp.UpdateStats("claude-sonnet-4-5", 1000, 500)
	}
	pred5 := cp.Predict(input)
	if pred5.Confidence != 0.6 {
		t.Errorf("expected confidence 0.6 with 5 observations, got %f", pred5.Confidence)
	}

	for i := 0; i < 5; i++ {
		cp.UpdateStats("claude-sonnet-4-5", 1000, 500)
	}
	pred10 := cp.Predict(input)
	if pred10.Confidence != 0.7 {
		t.Errorf("expected confidence 0.7 with 10 observations, got %f", pred10.Confidence)
	}

	for i := 0; i < 10; i++ {
		cp.UpdateStats("claude-sonnet-4-5", 1000, 500)
	}
	pred20 := cp.Predict(input)
	if pred20.Confidence != 0.8 {
		t.Errorf("expected confidence 0.8 with 20 observations, got %f", pred20.Confidence)
	}
}

func TestRiskAssessment_Levels(t *testing.T) {
	cp := NewCostPredictor()
	cp.SetBudget(1.0, 0)

	// Use haiku (cheap model) so we can precisely control cost via queryLen
	// haiku: $0.8/1M input, $4.0/1M output
	tests := []struct {
		name       string
		queryLen   int
		wantRisk   string
		wantWarn   bool
	}{
		{"low risk", 100, "low", false},
		{"medium risk", 500000, "medium", false},
		{"high risk", 1000000, "high", true},
		{"extreme risk", 2500000, "extreme", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := cp.Predict(PredictionInput{
				Model:       "claude-3-5-haiku-20241022",
				QueryLen:    tt.queryLen,
				ToolCount:   0,
				HistoryLen:  0,
			})
			if pred.RiskLevel != tt.wantRisk {
				t.Errorf("expected risk=%s, got %s (cost=%f, fraction=%f)",
					tt.wantRisk, pred.RiskLevel, pred.EstimatedCostUSD, pred.EstimatedCostUSD/1.0)
			}
			hasWarning := pred.Warning != ""
			if hasWarning != tt.wantWarn {
				t.Errorf("expected warning=%v, got %v (warning=%q)", tt.wantWarn, hasWarning, pred.Warning)
			}
		})
	}
}

func TestBudgetTracking(t *testing.T) {
	cp := NewCostPredictor()
	cp.SetBudget(10.0, 5.0) // $10 budget, $5 already used

	pred := cp.Predict(PredictionInput{
		Model:          "claude-sonnet-4-5",
		SystemPromptLen: 1000,
		QueryLen:        500,
	})

	if pred.BudgetAfterCall <= 0 {
		t.Errorf("expected positive BudgetAfterCall, got %f", pred.BudgetAfterCall)
	}
	if pred.BudgetFractionAfter <= 0.5 {
		t.Errorf("expected BudgetFractionAfter > 0.5 (since $5 used + new cost), got %f", pred.BudgetFractionAfter)
	}

	// BudgetAfterCall should be budgetLimit - budgetUsed - estimatedCost
	expectedAfter := 10.0 - 5.0 - pred.EstimatedCostUSD
	if math.Abs(pred.BudgetAfterCall-expectedAfter) > 1e-10 {
		t.Errorf("expected BudgetAfterCall=%f, got %f", expectedAfter, pred.BudgetAfterCall)
	}
}

func TestSetBudget(t *testing.T) {
	cp := NewCostPredictor()

	cp.SetBudget(20.0, 3.0)

	pred := cp.Predict(PredictionInput{
		Model:    "claude-sonnet-4-5",
		QueryLen: 100,
	})

	expectedFraction := (3.0 + pred.EstimatedCostUSD) / 20.0
	if math.Abs(pred.BudgetFractionAfter-expectedFraction) > 1e-6 {
		t.Errorf("expected BudgetFractionAfter=%f, got %f", expectedFraction, pred.BudgetFractionAfter)
	}
}

func TestZeroBudget_UnknownRisk(t *testing.T) {
	cp := NewCostPredictor()
	// No budget set — default is 0

	pred := cp.Predict(PredictionInput{
		Model:    "claude-sonnet-4-5",
		QueryLen: 100,
	})

	if pred.RiskLevel != "unknown" {
		t.Errorf("expected risk=unknown with zero budget, got %s", pred.RiskLevel)
	}
	if pred.Warning != "" {
		t.Errorf("expected no warning with zero budget, got %q", pred.Warning)
	}
}

func TestImageCount_IncreasesTokenEstimates(t *testing.T) {
	cp := NewCostPredictor()

	predNoImage := cp.Predict(PredictionInput{
		Model:       "claude-sonnet-4-5",
		QueryLen:    2000,
		HasVision:   false,
		ImageCount:  0,
	})

	predWithImage := cp.Predict(PredictionInput{
		Model:       "claude-sonnet-4-5",
		QueryLen:    2000,
		HasVision:   true,
		ImageCount:  3,
	})

	if predWithImage.EstimatedInputTokens <= predNoImage.EstimatedInputTokens {
		t.Errorf("expected more input tokens with images (%d) than without (%d)",
			predWithImage.EstimatedInputTokens, predNoImage.EstimatedInputTokens)
	}

	// 3 images * 1000 tokens per image = 3000 extra tokens
	diff := predWithImage.EstimatedInputTokens - predNoImage.EstimatedInputTokens
	if diff != 3000 {
		t.Errorf("expected 3000 extra tokens from 3 images, got %d", diff)
	}
}

func TestOutputTokenCap(t *testing.T) {
	cp := NewCostPredictor()

	// Very large input should hit the 4096 output cap
	pred := cp.Predict(PredictionInput{
		Model:           "claude-sonnet-4-5",
		SystemPromptLen: 100000,
		QueryLen:        100000,
		HistoryLen:      0,
		ToolCount:       0,
	})

	if pred.EstimatedOutputTokens > 4096 {
		t.Errorf("expected output tokens capped at 4096, got %d", pred.EstimatedOutputTokens)
	}
}

func TestHistoryAndToolOverhead(t *testing.T) {
	cp := NewCostPredictor()

	predSimple := cp.Predict(PredictionInput{
		Model:           "claude-sonnet-4-5",
		SystemPromptLen: 400,
		QueryLen:        100,
		HistoryLen:      0,
		ToolCount:       0,
	})

	predComplex := cp.Predict(PredictionInput{
		Model:           "claude-sonnet-4-5",
		SystemPromptLen: 400,
		QueryLen:        100,
		HistoryLen:      5,
		ToolCount:       15,
	})

	if predComplex.EstimatedInputTokens <= predSimple.EstimatedInputTokens {
		t.Errorf("expected more input tokens with history+tools (%d) than without (%d)",
			predComplex.EstimatedInputTokens, predSimple.EstimatedInputTokens)
	}

	// History: 5*50=250, Tools: 15*20=300, total overhead=550
	expectedDiff := 5*50 + 15*20
	actualDiff := predComplex.EstimatedInputTokens - predSimple.EstimatedInputTokens
	if actualDiff != expectedDiff {
		t.Errorf("expected %d overhead from history+tools, got %d", expectedDiff, actualDiff)
	}
}

func TestSreModelZeroCost(t *testing.T) {
	cp := NewCostPredictor()
	pred := cp.Predict(PredictionInput{
		Model:    "sre-model",
		QueryLen: 500,
	})

	if pred.EstimatedCostUSD != 0 {
		t.Errorf("expected zero cost for sre-model, got %f", pred.EstimatedCostUSD)
	}
}

func TestGetStats_NilForUnknownModel(t *testing.T) {
	cp := NewCostPredictor()
	stats := cp.GetStats("nonexistent-model")
	if stats != nil {
		t.Error("expected nil stats for unknown model")
	}
}

func TestConfidence_ReducedForManyImages(t *testing.T) {
	cp := NewCostPredictor()
	cp.UpdateStats("claude-sonnet-4-5", 1000, 500)

	predFewImages := cp.Predict(PredictionInput{
		Model:      "claude-sonnet-4-5",
		QueryLen:   100,
		ImageCount: 2,
		HasVision:  true,
	})

	predManyImages := cp.Predict(PredictionInput{
		Model:      "claude-sonnet-4-5",
		QueryLen:   100,
		ImageCount: 10,
		HasVision:  true,
	})

	if predManyImages.Confidence >= predFewImages.Confidence {
		t.Errorf("expected lower confidence with many images (%f) than few (%f)",
			predManyImages.Confidence, predFewImages.Confidence)
	}
}

func TestConfidence_ReducedForLongHistory(t *testing.T) {
	cp := NewCostPredictor()
	cp.UpdateStats("claude-sonnet-4-5", 1000, 500)

	predShortHistory := cp.Predict(PredictionInput{
		Model:      "claude-sonnet-4-5",
		QueryLen:   100,
		HistoryLen: 5,
	})

	predLongHistory := cp.Predict(PredictionInput{
		Model:      "claude-sonnet-4-5",
		QueryLen:   100,
		HistoryLen: 30,
	})

	if predLongHistory.Confidence >= predShortHistory.Confidence {
		t.Errorf("expected lower confidence with long history (%f) than short (%f)",
			predLongHistory.Confidence, predShortHistory.Confidence)
	}
}

func TestPricingMatchesCostguard(t *testing.T) {
	cp := NewCostPredictor()

	expectedPrices := map[string]PriceInfo{
		"claude-sonnet-4-5":          {InputPricePer1M: 3.0, OutputPricePer1M: 15.0},
		"claude-sonnet-4-20250514":   {InputPricePer1M: 3.0, OutputPricePer1M: 15.0},
		"claude-3-5-sonnet-20241022": {InputPricePer1M: 3.0, OutputPricePer1M: 15.0},
		"claude-opus-4-20250514":     {InputPricePer1M: 15.0, OutputPricePer1M: 75.0},
		"claude-3-5-haiku-20241022":  {InputPricePer1M: 0.8, OutputPricePer1M: 4.0},
		"gpt-4o":      {InputPricePer1M: 2.5, OutputPricePer1M: 10.0},
		"gpt-4o-mini": {InputPricePer1M: 0.15, OutputPricePer1M: 0.6},
		"gemini-2.5-pro":   {InputPricePer1M: 1.25, OutputPricePer1M: 10.0},
		"gemini-2.5-flash": {InputPricePer1M: 0.15, OutputPricePer1M: 0.6},
		"glm-4-plus": {InputPricePer1M: 2.0, OutputPricePer1M: 8.0},
		"sre-model":  {InputPricePer1M: 0.0, OutputPricePer1M: 0.0},
	}

	for model, expected := range expectedPrices {
		pred := cp.Predict(PredictionInput{
			Model:          model,
			SystemPromptLen: 4000,
			QueryLen:       1000,
		})
		inputTokens := float64(pred.EstimatedInputTokens)
		outputTokens := float64(pred.EstimatedOutputTokens)
		expectedCost := inputTokens*expected.InputPricePer1M/1_000_000 + outputTokens*expected.OutputPricePer1M/1_000_000
		if math.Abs(pred.EstimatedCostUSD-expectedCost) > 1e-10 {
			t.Errorf("model %s: expected cost %f, got %f", model, expectedCost, pred.EstimatedCostUSD)
		}
	}
}

func TestExtremeRiskWarning_ContainsModelSuggestion(t *testing.T) {
	cp := NewCostPredictor()
	cp.SetBudget(0.01, 0) // tiny budget

	pred := cp.Predict(PredictionInput{
		Model:       "claude-opus-4-20250514",
		QueryLen:    5000,
		HistoryLen:  5,
	})

	if pred.RiskLevel != "extreme" {
		t.Errorf("expected extreme risk, got %s", pred.RiskLevel)
	}
	if !strings.Contains(pred.Warning, "cheaper model") {
		t.Errorf("expected extreme warning to mention cheaper model, got %q", pred.Warning)
	}
}

func TestBudgetAfterCall_ZeroWhenExceeded(t *testing.T) {
	cp := NewCostPredictor()
	cp.SetBudget(0.001, 0)

	pred := cp.Predict(PredictionInput{
		Model:       "claude-sonnet-4-5",
		QueryLen:    5000,
	})

	if pred.BudgetAfterCall != 0 {
		t.Errorf("expected BudgetAfterCall=0 when budget exceeded, got %f", pred.BudgetAfterCall)
	}
}

func TestBudgetFractionAfter_ZeroWithNoBudget(t *testing.T) {
	cp := NewCostPredictor()
	// No budget set

	pred := cp.Predict(PredictionInput{
		Model:    "claude-sonnet-4-5",
		QueryLen: 100,
	})

	if pred.BudgetFractionAfter != 0 {
		t.Errorf("expected BudgetFractionAfter=0 with no budget, got %f", pred.BudgetFractionAfter)
	}
}
