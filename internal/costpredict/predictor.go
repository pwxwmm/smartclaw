package costpredict

import (
	"fmt"
	"sync"
)

type PredictionInput struct {
	Model           string
	SystemPromptLen int
	HistoryLen      int
	QueryLen        int
	ToolCount       int
	HasVision       bool
	ImageCount      int
	IsStreaming     bool
}

type CostPrediction struct {
	EstimatedInputTokens  int
	EstimatedOutputTokens int
	EstimatedCostUSD      float64
	InputCostUSD          float64
	OutputCostUSD         float64
	Confidence            float64
	Model                 string
	RiskLevel             string
	Warning               string
	BudgetAfterCall       float64
	BudgetFractionAfter   float64
}

type HistoricalStats struct {
	AvgInputTokens   float64
	AvgOutputTokens  float64
	InputCharRatio   float64
	OutputInputRatio float64
	SampleCount      int
}

type PriceInfo struct {
	InputPricePer1M  float64
	OutputPricePer1M float64
}

type CostPredictor struct {
	mu          sync.RWMutex
	stats       map[string]*HistoricalStats
	prices      map[string]PriceInfo
	budgetLimit float64
	budgetUsed  float64
}

func NewCostPredictor() *CostPredictor {
	return &CostPredictor{
		stats: make(map[string]*HistoricalStats),
		prices: map[string]PriceInfo{
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
		},
	}
}

func (cp *CostPredictor) Predict(input PredictionInput) CostPrediction {
	inputTokens := cp.estimateInputTokens(input)
	outputTokens := cp.estimateOutputTokens(input, inputTokens)

	cp.mu.RLock()
	price, ok := cp.prices[input.Model]
	cp.mu.RUnlock()
	if !ok {
		price = PriceInfo{InputPricePer1M: 3.0, OutputPricePer1M: 15.0}
	}

	inputCost := float64(inputTokens) * price.InputPricePer1M / 1_000_000
	outputCost := float64(outputTokens) * price.OutputPricePer1M / 1_000_000
	totalCost := inputCost + outputCost

	confidence := cp.calculateConfidence(input)
	riskLevel, warning := cp.assessRisk(totalCost)

	cp.mu.RLock()
	budgetLimit := cp.budgetLimit
	budgetUsed := cp.budgetUsed
	cp.mu.RUnlock()

	budgetAfter := budgetLimit - budgetUsed - totalCost
	if budgetAfter < 0 {
		budgetAfter = 0
	}
	budgetFraction := (budgetUsed + totalCost) / budgetLimit
	if budgetLimit == 0 {
		budgetFraction = 0
	}

	return CostPrediction{
		EstimatedInputTokens:  inputTokens,
		EstimatedOutputTokens: outputTokens,
		EstimatedCostUSD:      totalCost,
		InputCostUSD:          inputCost,
		OutputCostUSD:         outputCost,
		Confidence:            confidence,
		Model:                 input.Model,
		RiskLevel:             riskLevel,
		Warning:               warning,
		BudgetAfterCall:       budgetAfter,
		BudgetFractionAfter:   budgetFraction,
	}
}

func (cp *CostPredictor) UpdateStats(model string, actualInputTokens, actualOutputTokens int) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	stats, ok := cp.stats[model]
	if !ok {
		stats = &HistoricalStats{
			InputCharRatio:   4.0,
			OutputInputRatio: 0.4,
		}
		cp.stats[model] = stats
	}

	alpha := 0.3 // EMA smoothing factor — weight recent observations more

	if stats.SampleCount == 0 {
		stats.AvgInputTokens = float64(actualInputTokens)
		stats.AvgOutputTokens = float64(actualOutputTokens)
	} else {
		stats.AvgInputTokens = alpha*float64(actualInputTokens) + (1-alpha)*stats.AvgInputTokens
		stats.AvgOutputTokens = alpha*float64(actualOutputTokens) + (1-alpha)*stats.AvgOutputTokens
	}

	if actualInputTokens > 0 {
		observedRatio := float64(actualOutputTokens) / float64(actualInputTokens)
		if stats.SampleCount == 0 {
			stats.OutputInputRatio = observedRatio
		} else {
			stats.OutputInputRatio = alpha*observedRatio + (1-alpha)*stats.OutputInputRatio
		}
	}

	stats.SampleCount++
}

func (cp *CostPredictor) SetBudget(limitUSD, usedUSD float64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.budgetLimit = limitUSD
	cp.budgetUsed = usedUSD
}

func (cp *CostPredictor) GetStats(model string) *HistoricalStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	stats, ok := cp.stats[model]
	if !ok {
		return nil
	}
	clone := *stats
	return &clone
}

func (cp *CostPredictor) estimateInputTokens(input PredictionInput) int {
	charRatio := 4.0 // ~4 chars per token for English
	cp.mu.RLock()
	if stats, ok := cp.stats[input.Model]; ok && stats.SampleCount >= 5 {
		charRatio = stats.InputCharRatio
	}
	cp.mu.RUnlock()

	promptChars := input.SystemPromptLen + input.QueryLen
	historyOverhead := input.HistoryLen * 50  // ~50 tokens per turn for message metadata
	toolOverhead := input.ToolCount * 20      // ~20 tokens per tool definition
	imageTokens := 0
	if input.HasVision {
		imageTokens = input.ImageCount * 1000
	}

	estimated := int(float64(promptChars)/charRatio) + historyOverhead + toolOverhead + imageTokens
	if estimated < 100 {
		estimated = 100
	}
	return estimated
}

func (cp *CostPredictor) estimateOutputTokens(input PredictionInput, inputTokens int) int {
	ratio := 0.4
	cp.mu.RLock()
	if stats, ok := cp.stats[input.Model]; ok && stats.SampleCount >= 5 {
		ratio = stats.OutputInputRatio
	}
	cp.mu.RUnlock()

	if input.ToolCount > 20 {
		ratio += 0.1
	}
	if input.HistoryLen > 10 {
		ratio += 0.1
	}

	estimated := int(float64(inputTokens) * ratio)
	if estimated < 50 {
		estimated = 50
	}
	if estimated > 4096 {
		estimated = 4096
	}
	return estimated
}

func (cp *CostPredictor) calculateConfidence(input PredictionInput) float64 {
	base := 0.3

	cp.mu.RLock()
	if stats, ok := cp.stats[input.Model]; ok {
		if stats.SampleCount >= 20 {
			base = 0.8
		} else if stats.SampleCount >= 10 {
			base = 0.7
		} else if stats.SampleCount >= 5 {
			base = 0.6
		} else if stats.SampleCount >= 1 {
			base = 0.5
		}
	}
	cp.mu.RUnlock()

	if input.ImageCount > 5 {
		base -= 0.1
	}
	if input.HistoryLen > 20 {
		base -= 0.05
	}

	if base < 0.1 {
		base = 0.1
	}
	return base
}

func (cp *CostPredictor) assessRisk(estimatedCost float64) (string, string) {
	cp.mu.RLock()
	budgetLimit := cp.budgetLimit
	cp.mu.RUnlock()

	if budgetLimit <= 0 {
		return "unknown", ""
	}

	fraction := estimatedCost / budgetLimit

	switch {
	case fraction > 0.5:
		return "extreme", fmt.Sprintf("This call may cost $%.4f (%.0f%% of budget). Consider using a cheaper model.", estimatedCost, fraction*100)
	case fraction > 0.2:
		return "high", fmt.Sprintf("This call may cost $%.4f (%.0f%% of budget).", estimatedCost, fraction*100)
	case fraction > 0.1:
		return "medium", ""
	default:
		return "low", ""
	}
}
