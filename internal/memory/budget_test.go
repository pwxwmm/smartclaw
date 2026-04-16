package memory

import (
	"strings"
	"testing"
)

func TestDefaultContextBudget(t *testing.T) {
	cb := DefaultContextBudget()

	if cb.MaxChars <= 0 {
		t.Error("expected positive max chars")
	}
	if len(cb.Layers) != 8 {
		t.Errorf("expected 8 budget layers, got %d", len(cb.Layers))
	}
}

func TestAllocate_AllLayersPresent(t *testing.T) {
	cb := ContextBudget{
		MaxChars: 1000,
		Layers: []BudgetLayer{
			{Name: LayerSOUL, Weight: 0.5, MinChars: 0, MaxChars: 1000},
			{Name: LayerMemory, Weight: 0.5, MinChars: 0, MaxChars: 1000},
		},
	}

	contents := []LayerContent{
		{Name: LayerSOUL, Content: strings.Repeat("a", 600)},
		{Name: LayerMemory, Content: strings.Repeat("b", 600)},
	}

	result := cb.Allocate(contents)

	if len(result) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(result))
	}

	totalChars := 0
	for _, alloc := range result {
		totalChars += alloc.Chars
	}

	if totalChars > cb.MaxChars+10 {
		t.Errorf("total chars %d exceeds budget %d", totalChars, cb.MaxChars)
	}
}

func TestAllocate_Truncation(t *testing.T) {
	cb := ContextBudget{
		MaxChars: 100,
		Layers: []BudgetLayer{
			{Name: LayerSOUL, Weight: 1.0, MinChars: 0, MaxChars: 100},
		},
	}

	longContent := strings.Repeat("x", 500)
	contents := []LayerContent{
		{Name: LayerSOUL, Content: longContent},
	}

	result := cb.Allocate(contents)

	if len(result) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(result))
	}

	if !result[0].Truncated {
		t.Error("expected truncation")
	}

	if result[0].Chars > 100 {
		t.Errorf("chars %d exceeds budget 100", result[0].Chars)
	}

	if !strings.HasSuffix(result[0].Content, "...") {
		t.Error("truncated content should end with ...")
	}
}

func TestAllocate_EmptyContent(t *testing.T) {
	cb := DefaultContextBudget()

	contents := []LayerContent{
		{Name: LayerSOUL, Content: ""},
		{Name: LayerMemory, Content: "some memory"},
	}

	result := cb.Allocate(contents)

	for _, alloc := range result {
		if alloc.Name == LayerSOUL {
			t.Error("empty content should not be allocated")
		}
	}
}

func TestAllocate_ProportionalDistribution(t *testing.T) {
	cb := ContextBudget{
		MaxChars: 1000,
		Layers: []BudgetLayer{
			{Name: LayerSOUL, Weight: 0.75, MinChars: 0, MaxChars: 1000},
			{Name: LayerMemory, Weight: 0.25, MinChars: 0, MaxChars: 1000},
		},
	}

	contents := []LayerContent{
		{Name: LayerSOUL, Content: strings.Repeat("a", 800)},
		{Name: LayerMemory, Content: strings.Repeat("b", 800)},
	}

	result := cb.Allocate(contents)

	soulAlloc := findLayer(result, LayerSOUL)
	memoryAlloc := findLayer(result, LayerMemory)

	if soulAlloc == nil || memoryAlloc == nil {
		t.Fatal("expected both layers to be allocated")
	}

	if soulAlloc.Budget <= memoryAlloc.Budget {
		t.Errorf("soul (weight 0.75) should get more budget than memory (weight 0.25), got %d vs %d",
			soulAlloc.Budget, memoryAlloc.Budget)
	}
}

func TestAllocate_MaxCharsCap(t *testing.T) {
	cb := ContextBudget{
		MaxChars: 5000,
		Layers: []BudgetLayer{
			{Name: LayerUserModel, Weight: 1.0, MinChars: 0, MaxChars: 500},
		},
	}

	contents := []LayerContent{
		{Name: LayerUserModel, Content: strings.Repeat("x", 3000)},
	}

	result := cb.Allocate(contents)

	if len(result) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(result))
	}

	if result[0].Chars > 500 {
		t.Errorf("chars %d exceeds MaxChars cap 500", result[0].Chars)
	}
}

func TestAllocate_MinCharsFloor(t *testing.T) {
	cb := ContextBudget{
		MaxChars: 50,
		Layers: []BudgetLayer{
			{Name: LayerSOUL, Weight: 1.0, MinChars: 30, MaxChars: 500},
		},
	}

	contents := []LayerContent{
		{Name: LayerSOUL, Content: strings.Repeat("x", 200)},
	}

	result := cb.Allocate(contents)

	if len(result) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(result))
	}

	if result[0].Budget < 30 {
		t.Errorf("budget %d below MinChars floor 30", result[0].Budget)
	}
}

func findLayer(layers []AllocatedLayer, name LayerName) *AllocatedLayer {
	for _, l := range layers {
		if l.Name == name {
			return &l
		}
	}
	return nil
}
