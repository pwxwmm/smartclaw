package routing

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type RoutingStrategy string

const (
	StrategyCostFirst    RoutingStrategy = "cost-first"
	StrategyQualityFirst RoutingStrategy = "quality-first"
	StrategyBalanced     RoutingStrategy = "balanced"
)

type ModelTier string

const (
	TierFast    ModelTier = "fast"
	TierDefault ModelTier = "default"
	TierHeavy   ModelTier = "heavy"
)

type ModelEntry struct {
	ID      string
	Tier    ModelTier
	Aliases []string
}

type ComplexitySignal struct {
	MessageLength    int
	ToolCallCount    int
	HistoryTurnCount int
	HasCodeContent   bool
	RetryCount       int
	SkillMatched     bool
}

type RoutingConfig struct {
	Enabled      bool            `yaml:"enabled"`
	Strategy     RoutingStrategy `yaml:"strategy"`
	FastModel    string          `yaml:"fast_model"`
	DefaultModel string          `yaml:"default_model"`
	HeavyModel   string          `yaml:"heavy_model"`
}

type RoutingRecord struct {
	Model      string
	Complexity float64
	Success    bool
	Retried    bool
	Duration   time.Duration
	RecordedAt time.Time
}

type ModelRouter struct {
	config  RoutingConfig
	models  map[ModelTier]ModelEntry
	history []RoutingRecord
	mu      sync.RWMutex
}

func DefaultRoutingConfig() RoutingConfig {
	return RoutingConfig{
		Enabled:      false,
		Strategy:     StrategyBalanced,
		FastModel:    "claude-3-5-haiku-20241022",
		DefaultModel: "claude-sonnet-4-5",
		HeavyModel:   "claude-sonnet-4-5",
	}
}

func NewModelRouter(cfg RoutingConfig) *ModelRouter {
	r := &ModelRouter{
		config: cfg,
		models: map[ModelTier]ModelEntry{
			TierFast: {
				ID:      cfg.FastModel,
				Tier:    TierFast,
				Aliases: []string{"haiku", "fast", "cheap"},
			},
			TierDefault: {
				ID:      cfg.DefaultModel,
				Tier:    TierDefault,
				Aliases: []string{"sonnet", "default", "balanced"},
			},
			TierHeavy: {
				ID:      cfg.HeavyModel,
				Tier:    TierHeavy,
				Aliases: []string{"opus", "heavy", "quality"},
			},
		},
		history: make([]RoutingRecord, 0, 100),
	}

	return r
}

func (mr *ModelRouter) Route(query string, signal ComplexitySignal) string {
	if !mr.config.Enabled {
		return mr.models[TierDefault].ID
	}

	score := mr.assessComplexity(query, signal)

	tier := mr.complexityToTier(score)

	model := mr.models[tier].ID

	slog.Debug("routing: model selected",
		"complexity", fmt.Sprintf("%.2f", score),
		"tier", string(tier),
		"model", model,
	)

	return model
}

func (mr *ModelRouter) AssessComplexity(query string, signal ComplexitySignal) float64 {
	return mr.assessComplexity(query, signal)
}

func (mr *ModelRouter) assessComplexity(query string, signal ComplexitySignal) float64 {
	score := 0.0

	if signal.MessageLength > 2000 {
		score += 0.3
	} else if signal.MessageLength > 500 {
		score += 0.1
	}

	if signal.ToolCallCount > 3 {
		score += 0.3
	} else if signal.ToolCallCount > 1 {
		score += 0.15
	}

	if signal.HistoryTurnCount > 10 {
		score += 0.2
	} else if signal.HistoryTurnCount > 5 {
		score += 0.1
	}

	if signal.RetryCount > 0 {
		score += 0.25
	}

	if signal.HasCodeContent {
		score += 0.15
	}

	if signal.SkillMatched {
		score -= 0.2
	}

	lower := strings.ToLower(query)

	complexIndicators := []string{
		"architect", "design", "refactor", "debug", "investigate",
		"optimize", "analyze", "explain", "compare", "evaluate",
		"why does", "how does", "what if",
	}
	for _, indicator := range complexIndicators {
		if strings.Contains(lower, indicator) {
			score += 0.15
			break
		}
	}

	simpleIndicators := []string{
		"list", "show", "cat", "read", "grep", "find",
		"what is", "define", "format", "convert",
	}
	for _, indicator := range simpleIndicators {
		if strings.Contains(lower, indicator) {
			score -= 0.15
			break
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

func (mr *ModelRouter) complexityToTier(score float64) ModelTier {
	switch mr.config.Strategy {
	case StrategyCostFirst:
		if score < 0.5 {
			return TierFast
		}
		if score < 0.8 {
			return TierDefault
		}
		return TierHeavy

	case StrategyQualityFirst:
		if score < 0.15 {
			return TierFast
		}
		return TierDefault

	default: // StrategyBalanced
		if score < 0.3 {
			return TierFast
		}
		if score < 0.65 {
			return TierDefault
		}
		return TierHeavy
	}
}

func (mr *ModelRouter) RecordOutcome(model string, complexity float64, success bool, retried bool, duration time.Duration) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	record := RoutingRecord{
		Model:      model,
		Complexity: complexity,
		Success:    success,
		Retried:    retried,
		Duration:   duration,
		RecordedAt: time.Now(),
	}

	mr.history = append(mr.history, record)

	if len(mr.history) > 100 {
		mr.history = mr.history[len(mr.history)-100:]
	}
}

func (mr *ModelRouter) GetStats() RoutingStats {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	stats := RoutingStats{
		ByModel: make(map[string]ModelStats),
	}

	for tier, entry := range mr.models {
		stats.ByModel[entry.ID] = ModelStats{Tier: string(tier)}
	}

	for _, record := range mr.history {
		ms, ok := stats.ByModel[record.Model]
		if !ok {
			ms = ModelStats{}
		}
		ms.TotalCalls++
		if record.Success {
			ms.Successes++
		}
		if record.Retried {
			ms.Retries++
		}
		ms.TotalDuration += record.Duration
		stats.ByModel[record.Model] = ms
	}

	return stats
}

func (mr *ModelRouter) IsEnabled() bool {
	return mr.config.Enabled
}

func (mr *ModelRouter) SetEnabled(enabled bool) {
	mr.config.Enabled = enabled
}

func (mr *ModelRouter) GetConfig() RoutingConfig {
	return mr.config
}

func (mr *ModelRouter) SetConfig(cfg RoutingConfig) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.config = cfg

	mr.models[TierFast] = ModelEntry{ID: cfg.FastModel, Tier: TierFast}
	mr.models[TierDefault] = ModelEntry{ID: cfg.DefaultModel, Tier: TierDefault}
	mr.models[TierHeavy] = ModelEntry{ID: cfg.HeavyModel, Tier: TierHeavy}
}

type ModelStats struct {
	Tier          string
	TotalCalls    int
	Successes     int
	Retries       int
	TotalDuration time.Duration
}

type RoutingStats struct {
	ByModel map[string]ModelStats
}
