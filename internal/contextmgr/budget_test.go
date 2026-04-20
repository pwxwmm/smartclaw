package contextmgr

import (
	"testing"
)

func TestNewTokenBudget(t *testing.T) {
	tb := NewTokenBudget(10000)
	if tb.Total != 10000 {
		t.Errorf("expected Total=10000, got %d", tb.Total)
	}
	if tb.Used != 0 {
		t.Errorf("expected Used=0, got %d", tb.Used)
	}
	if tb.BySource == nil {
		t.Error("expected BySource to be initialized")
	}
}

func TestTokenBudget_Remaining(t *testing.T) {
	tb := NewTokenBudget(5000)
	if r := tb.Remaining(); r != 5000 {
		t.Errorf("expected Remaining=5000, got %d", r)
	}
	tb.Record("src1", 1500)
	if r := tb.Remaining(); r != 3500 {
		t.Errorf("expected Remaining=3500, got %d", r)
	}
}

func TestTokenBudget_Record(t *testing.T) {
	tb := NewTokenBudget(10000)
	tb.Record("files", 100)
	tb.Record("symbols", 200)
	tb.Record("files", 50)

	if tb.Used != 350 {
		t.Errorf("expected Used=350, got %d", tb.Used)
	}
	if tb.BySource["files"] != 150 {
		t.Errorf("expected BySource[files]=150, got %d", tb.BySource["files"])
	}
	if tb.BySource["symbols"] != 200 {
		t.Errorf("expected BySource[symbols]=200, got %d", tb.BySource["symbols"])
	}
}

func TestAllocate_BasicPriority(t *testing.T) {
	sources := []SourcePriority{
		{Source: "high", Weight: 1.0, MinTokens: 0, MaxTokens: 1000},
		{Source: "medium", Weight: 0.5, MinTokens: 0, MaxTokens: 1000},
		{Source: "low", Weight: 0.1, MinTokens: 0, MaxTokens: 1000},
	}

	result := Allocate(1000, sources)

	total := 0
	for _, v := range result {
		total += v
	}

	if total > 1000 {
		t.Errorf("allocated more than total: %d", total)
	}

	// High-weight source should get more than low-weight
	if result["high"] <= result["low"] {
		t.Errorf("expected high > low, got high=%d low=%d", result["high"], result["low"])
	}
}

func TestAllocate_RespectsMinTokens(t *testing.T) {
	sources := []SourcePriority{
		{Source: "system", Weight: 1.0, MinTokens: 500, MaxTokens: 2000},
		{Source: "conversation", Weight: 0.5, MinTokens: 200, MaxTokens: 5000},
		{Source: "files", Weight: 0.3, MinTokens: 0, MaxTokens: 5000},
	}

	result := Allocate(1000, sources)

	if result["system"] < 500 {
		t.Errorf("system should get at least MinTokens=500, got %d", result["system"])
	}
	if result["conversation"] < 200 {
		t.Errorf("conversation should get at least MinTokens=200, got %d", result["conversation"])
	}
}

func TestAllocate_RespectsMaxTokens(t *testing.T) {
	sources := []SourcePriority{
		{Source: "tiny", Weight: 1.0, MinTokens: 0, MaxTokens: 100},
		{Source: "big", Weight: 0.5, MinTokens: 0, MaxTokens: 5000},
	}

	result := Allocate(10000, sources)

	if result["tiny"] > 100 {
		t.Errorf("tiny should not exceed MaxTokens=100, got %d", result["tiny"])
	}
}

func TestAllocate_ZeroBudget(t *testing.T) {
	sources := []SourcePriority{
		{Source: "a", Weight: 1.0, MinTokens: 0, MaxTokens: 1000},
		{Source: "b", Weight: 0.5, MinTokens: 0, MaxTokens: 1000},
	}

	result := Allocate(0, sources)

	for src, v := range result {
		if v != 0 {
			t.Errorf("expected 0 allocation for %s, got %d", src, v)
		}
	}
}

func TestAllocate_SingleSource(t *testing.T) {
	sources := []SourcePriority{
		{Source: "only", Weight: 1.0, MinTokens: 100, MaxTokens: 5000},
	}

	result := Allocate(3000, sources)

	if result["only"] != 3000 {
		t.Errorf("single source should get all tokens, got %d", result["only"])
	}
}

func TestAllocate_OversubscribedMinTokens(t *testing.T) {
	// When total minimums exceed budget, should still not exceed total
	sources := []SourcePriority{
		{Source: "a", Weight: 1.0, MinTokens: 500, MaxTokens: 5000},
		{Source: "b", Weight: 0.5, MinTokens: 400, MaxTokens: 5000},
		{Source: "c", Weight: 0.3, MinTokens: 300, MaxTokens: 5000},
	}

	result := Allocate(800, sources)

	total := 0
	for _, v := range result {
		total += v
	}
	if total > 800 {
		t.Errorf("total allocation %d exceeds budget 800", total)
	}
}

func TestAllocate_DefaultPriorities(t *testing.T) {
	sources := DefaultSourcePriorities()
	result := Allocate(20000, sources)

	// System prompt and conversation should always get their min tokens
	if result["system_prompt"] < 500 {
		t.Errorf("system_prompt should get min 500, got %d", result["system_prompt"])
	}
	if result["conversation"] < 1000 {
		t.Errorf("conversation should get min 1000, got %d", result["conversation"])
	}

	// Total should not exceed budget
	total := 0
	for _, v := range result {
		total += v
	}
	if total > 20000 {
		t.Errorf("total allocation %d exceeds budget", total)
	}

	// High-weight sources should get more than low-weight
	if result["system_prompt"] <= result["git"] {
		t.Errorf("system_prompt should get more than git; got system=%d git=%d",
			result["system_prompt"], result["git"])
	}
}

func TestAllocate_ProportionalDistribution(t *testing.T) {
	// Two sources with same min/max but different weights
	sources := []SourcePriority{
		{Source: "heavy", Weight: 0.8, MinTokens: 0, MaxTokens: 10000},
		{Source: "light", Weight: 0.2, MinTokens: 0, MaxTokens: 10000},
	}

	result := Allocate(1000, sources)

	// heavy should get roughly 4x what light gets
	ratio := float64(result["heavy"]) / float64(result["light"])
	if ratio < 3.0 || ratio > 5.0 {
		t.Errorf("expected ratio ~4.0, got %.2f (heavy=%d light=%d)",
			ratio, result["heavy"], result["light"])
	}
}
