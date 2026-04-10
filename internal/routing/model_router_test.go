package routing

import (
	"testing"
)

func TestNewModelRouter(t *testing.T) {
	cfg := DefaultRoutingConfig()
	r := NewModelRouter(cfg)

	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if r.IsEnabled() {
		t.Error("expected router to be disabled by default")
	}
}

func TestRoute_Disabled(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = false
	r := NewModelRouter(cfg)

	model := r.Route("hello", ComplexitySignal{})
	if model != cfg.DefaultModel {
		t.Errorf("expected default model when disabled, got %s", model)
	}
}

func TestRoute_SimpleQuery(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = true
	cfg.Strategy = StrategyBalanced
	r := NewModelRouter(cfg)

	model := r.Route("list files in current directory", ComplexitySignal{})
	if model != cfg.FastModel {
		t.Errorf("expected fast model for simple query, got %s", model)
	}
}

func TestRoute_ComplexQuery(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = true
	cfg.Strategy = StrategyBalanced
	r := NewModelRouter(cfg)

	signal := ComplexitySignal{
		MessageLength:    3000,
		ToolCallCount:    5,
		HistoryTurnCount: 12,
		HasCodeContent:   true,
		RetryCount:       1,
	}

	model := r.Route("architect a distributed caching system", signal)
	if model != cfg.HeavyModel {
		t.Errorf("expected heavy model for complex query, got %s", model)
	}
}

func TestRoute_MediumQuery(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = true
	cfg.Strategy = StrategyBalanced
	r := NewModelRouter(cfg)

	signal := ComplexitySignal{
		MessageLength:    800,
		ToolCallCount:    2,
		HistoryTurnCount: 3,
		HasCodeContent:   true,
	}

	model := r.Route("fix the bug in auth handler", signal)
	if model != cfg.DefaultModel {
		t.Errorf("expected default model for medium query, got %s", model)
	}
}

func TestRoute_SkillMatchedDowngrades(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = true
	cfg.Strategy = StrategyBalanced
	r := NewModelRouter(cfg)

	signal := ComplexitySignal{
		MessageLength: 600,
		ToolCallCount: 1,
		SkillMatched:  true,
	}

	model := r.Route("run tests", signal)
	if model != cfg.FastModel {
		t.Errorf("expected fast model when skill matched, got %s", model)
	}
}

func TestComplexityScore_Bounds(t *testing.T) {
	cfg := DefaultRoutingConfig()
	r := NewModelRouter(cfg)

	score := r.assessComplexity("simple query", ComplexitySignal{})
	if score < 0 || score > 1 {
		t.Errorf("complexity score out of bounds: %f", score)
	}

	signal := ComplexitySignal{
		MessageLength:    5000,
		ToolCallCount:    10,
		HistoryTurnCount: 20,
		HasCodeContent:   true,
		RetryCount:       3,
	}
	score = r.assessComplexity("architect design debug investigate optimize analyze", signal)
	if score != 1.0 {
		t.Errorf("expected capped score 1.0, got %f", score)
	}
}

func TestCostFirstStrategy(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = true
	cfg.Strategy = StrategyCostFirst
	r := NewModelRouter(cfg)

	model := r.Route("explain this code", ComplexitySignal{})
	if model != cfg.FastModel {
		t.Errorf("cost-first should prefer fast model for medium query, got %s", model)
	}
}

func TestQualityFirstStrategy(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = true
	cfg.Strategy = StrategyQualityFirst
	r := NewModelRouter(cfg)

	signal := ComplexitySignal{MessageLength: 500, HasCodeContent: true}
	model := r.Route("explain this code", signal)
	if model != cfg.DefaultModel {
		t.Errorf("quality-first should prefer default model for medium query, got %s", model)
	}
}

func TestRecordOutcome(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Enabled = true
	r := NewModelRouter(cfg)

	r.RecordOutcome("claude-3-5-haiku-20241022", 0.2, true, false, 1000)
	r.RecordOutcome("claude-sonnet-4-5", 0.6, true, false, 3000)
	r.RecordOutcome("claude-sonnet-4-5", 0.8, false, true, 5000)

	stats := r.GetStats()

	fastStats, ok := stats.ByModel["claude-3-5-haiku-20241022"]
	if !ok || fastStats.TotalCalls != 1 || fastStats.Successes != 1 {
		t.Errorf("unexpected fast model stats: %+v", fastStats)
	}

	defaultStats, ok := stats.ByModel["claude-sonnet-4-5"]
	if !ok || defaultStats.TotalCalls != 2 || defaultStats.Retries != 1 {
		t.Errorf("unexpected default model stats: %+v", defaultStats)
	}
}

func TestRecordOutcome_HistoryLimit(t *testing.T) {
	cfg := DefaultRoutingConfig()
	r := NewModelRouter(cfg)

	for i := 0; i < 150; i++ {
		r.RecordOutcome("claude-sonnet-4-5", 0.5, true, false, 1000)
	}

	if len(r.history) > 100 {
		t.Errorf("history should be capped at 100, got %d", len(r.history))
	}
}

func TestSetEnabled(t *testing.T) {
	cfg := DefaultRoutingConfig()
	r := NewModelRouter(cfg)

	if r.IsEnabled() {
		t.Error("expected disabled by default")
	}

	r.SetEnabled(true)
	if !r.IsEnabled() {
		t.Error("expected enabled after SetEnabled(true)")
	}
}

func TestSetConfig(t *testing.T) {
	cfg := DefaultRoutingConfig()
	r := NewModelRouter(cfg)

	newCfg := RoutingConfig{
		Enabled:      true,
		Strategy:     StrategyCostFirst,
		FastModel:    "custom-fast",
		DefaultModel: "custom-default",
		HeavyModel:   "custom-heavy",
	}
	r.SetConfig(newCfg)

	got := r.GetConfig()
	if got.FastModel != "custom-fast" {
		t.Errorf("expected custom-fast, got %s", got.FastModel)
	}
}
