package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type stubTool struct {
	BaseTool
}

func (s *stubTool) Execute(_ context.Context, _ map[string]any) (any, error) {
	return nil, nil
}

func newStubTool(name string) *stubTool {
	return &stubTool{BaseTool: NewBaseTool(name, "", nil)}
}

func TestNewToolsetDistribution(t *testing.T) {
	d := NewToolsetDistribution(42)
	if d == nil {
		t.Fatal("expected non-nil distribution")
	}
	if len(d.sets) != 0 {
		t.Errorf("expected 0 sets, got %d", len(d.sets))
	}
	if d.seed != 42 {
		t.Errorf("expected seed 42, got %d", d.seed)
	}
}

func TestNewToolsetDistributionZeroSeed(t *testing.T) {
	d := NewToolsetDistribution(0)
	if d == nil {
		t.Fatal("expected non-nil distribution")
	}
	if d.seed == 0 {
		t.Error("expected non-zero seed when seed=0 passed")
	}
}

func TestRegisterSetAndListSets(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSet("core", []string{"bash", "read_file"}, 1.0)
	d.RegisterSet("web", []string{"web_fetch"}, 0.5)

	sets := d.ListSets()
	if len(sets) != 2 {
		t.Fatalf("expected 2 sets, got %d", len(sets))
	}

	names := map[string]bool{}
	for _, s := range sets {
		names[s.Name] = true
	}
	if !names["core"] || !names["web"] {
		t.Errorf("expected core and web sets, got %v", names)
	}
}

func TestRegisterSetZeroWeight(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSet("zero", []string{"bash"}, 0)
	s := d.GetSet("zero")
	if s.Weight != 1.0 {
		t.Errorf("expected default weight 1.0 for zero weight, got %f", s.Weight)
	}
}

func TestRegisterSetWithCondition(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSetWithCondition("code", []string{"lsp"}, 0.5, "complexity > 0.7")

	s := d.GetSet("code")
	if s == nil {
		t.Fatal("expected set 'code' to exist")
	}
	if s.Condition != "complexity > 0.7" {
		t.Errorf("expected condition 'complexity > 0.7', got %q", s.Condition)
	}
}

func TestSelectSetSingleSet(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSet("only", []string{"bash"}, 1.0)

	for i := 0; i < 10; i++ {
		set, err := d.SelectSet(context.Background(), 0.5)
		if err != nil {
			t.Fatalf("SelectSet failed: %v", err)
		}
		if set.Name != "only" {
			t.Errorf("expected 'only', got %q", set.Name)
		}
	}
}

func TestSelectSetNoSets(t *testing.T) {
	d := NewToolsetDistribution(1)
	_, err := d.SelectSet(context.Background(), 0.5)
	if err == nil {
		t.Fatal("expected error with no sets registered")
	}
}

func TestSelectSetWeightedDistribution(t *testing.T) {
	d := NewToolsetDistribution(99)
	d.RegisterSet("a", []string{"bash"}, 3.0)
	d.RegisterSet("b", []string{"read_file"}, 1.0)

	counts := map[string]int{"a": 0, "b": 0}
	n := 10000
	for i := 0; i < n; i++ {
		set, err := d.SelectSet(context.Background(), 0.5)
		if err != nil {
			t.Fatalf("SelectSet failed: %v", err)
		}
		counts[set.Name]++
	}

	ratio := float64(counts["a"]) / float64(counts["b"])
	if ratio < 2.0 || ratio > 4.0 {
		t.Errorf("expected ratio ~3.0, got %.2f (a=%d, b=%d)", ratio, counts["a"], counts["b"])
	}
}

func TestSelectSetWithCondition(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSetWithCondition("high", []string{"lsp"}, 1.0, "complexity > 0.7")
	d.RegisterSetWithCondition("low", []string{"bash"}, 1.0, "always")

	set, err := d.SelectSet(context.Background(), 0.9)
	if err != nil {
		t.Fatalf("SelectSet failed: %v", err)
	}
	if set.Name != "high" && set.Name != "low" {
		t.Errorf("unexpected set %q", set.Name)
	}

	set, err = d.SelectSet(context.Background(), 0.1)
	if err != nil {
		t.Fatalf("SelectSet failed: %v", err)
	}
	if set.Name != "low" {
		t.Errorf("expected 'low' for complexity 0.1, got %q", set.Name)
	}
}

func TestSelectSetNoEligibleSets(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSetWithCondition("high", []string{"lsp"}, 1.0, "complexity > 0.9")

	_, err := d.SelectSet(context.Background(), 0.1)
	if err == nil {
		t.Fatal("expected error when no sets are eligible")
	}
}

func TestSelectTools(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSet("core", []string{"bash", "read_file"}, 1.0)

	available := map[string]Tool{
		"bash":      newStubTool("bash"),
		"read_file": newStubTool("read_file"),
		"grep":      newStubTool("grep"),
	}

	tools, err := d.SelectTools(context.Background(), 0.5, available)
	if err != nil {
		t.Fatalf("SelectTools failed: %v", err)
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	if !names["bash"] || !names["read_file"] {
		t.Errorf("expected bash and read_file, got %v", names)
	}
	if names["grep"] {
		t.Error("grep should not be included in 'core' set")
	}
}

func TestSelectToolsEmptyToolsAllTools(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSetWithCondition("full", []string{}, 1.0, "always")

	available := map[string]Tool{
		"bash":      newStubTool("bash"),
		"read_file": newStubTool("read_file"),
	}

	tools, err := d.SelectTools(context.Background(), 0.5, available)
	if err != nil {
		t.Fatalf("SelectTools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools for empty set (all tools), got %d", len(tools))
	}
}

func TestSetSeedReproducibility(t *testing.T) {
	d := NewToolsetDistribution(42)
	d.RegisterSet("a", []string{"bash"}, 1.0)
	d.RegisterSet("b", []string{"read_file"}, 1.0)

	d.SetSeed(42)
	var first []string
	for i := 0; i < 100; i++ {
		set, _ := d.SelectSet(context.Background(), 0.5)
		first = append(first, set.Name)
	}

	d.SetSeed(42)
	var second []string
	for i := 0; i < 100; i++ {
		set, _ := d.SelectSet(context.Background(), 0.5)
		second = append(second, set.Name)
	}

	for i := range first {
		if first[i] != second[i] {
			t.Errorf("reproducibility broken at index %d: %q != %q", i, first[i], second[i])
			return
		}
	}
}

func TestRemoveSet(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSet("a", []string{"bash"}, 1.0)
	d.RegisterSet("b", []string{"read_file"}, 1.0)

	d.RemoveSet("a")

	if d.GetSet("a") != nil {
		t.Error("expected 'a' to be removed")
	}
	if d.GetSet("b") == nil {
		t.Error("expected 'b' to still exist")
	}
}

func TestGetSetNotFound(t *testing.T) {
	d := NewToolsetDistribution(1)
	if s := d.GetSet("nonexistent"); s != nil {
		t.Error("expected nil for nonexistent set")
	}
}

func TestDefaultToolsets(t *testing.T) {
	sets := DefaultToolsets()
	if len(sets) != 5 {
		t.Fatalf("expected 5 default toolsets, got %d", len(sets))
	}

	names := map[string]bool{}
	for _, s := range sets {
		names[s.Name] = true
	}
	expected := []string{"core", "web", "code", "sre", "full"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing default toolset %q", name)
		}
	}

	for _, s := range sets {
		if s.Weight <= 0 {
			t.Errorf("toolset %q has non-positive weight %f", s.Name, s.Weight)
		}
	}
}

func TestConditionEvaluation(t *testing.T) {
	tests := []struct {
		cond        string
		complexity  float64
		mode        string
		toolCount   int
		expect      bool
		expectError bool
	}{
		{"", 0.5, "", 0, true, false},
		{"always", 0.5, "", 0, true, false},
		{"ALWAYS", 0.5, "", 0, true, false},
		{"complexity > 0.7", 0.8, "", 0, true, false},
		{"complexity > 0.7", 0.5, "", 0, false, false},
		{"complexity < 0.3", 0.1, "", 0, true, false},
		{"complexity >= 0.5", 0.5, "", 0, true, false},
		{"complexity <= 0.5", 0.5, "", 0, true, false},
		{"complexity == 0.5", 0.5, "", 0, true, false},
		{"complexity != 0.5", 0.6, "", 0, true, false},
		{`mode == "expert"`, 0.5, "expert", 0, true, false},
		{`mode == "expert"`, 0.5, "novice", 0, false, false},
		{`mode != "expert"`, 0.5, "novice", 0, true, false},
		{"tool_count > 3", 0.5, "", 5, true, false},
		{"tool_count >= 3", 0.5, "", 3, true, false},
		{"tool_count < 3", 0.5, "", 2, true, false},
		{"invalid_expr", 0.5, "", 0, false, true},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("cond_%d", i), func(t *testing.T) {
			ok, err := evaluateCondition(tt.cond, tt.complexity, tt.mode, tt.toolCount)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for condition %q", tt.cond)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for condition %q: %v", tt.cond, err)
				return
			}
			if ok != tt.expect {
				t.Errorf("condition %q: expected %v, got %v (complexity=%.2f, mode=%q, toolCount=%d)",
					tt.cond, tt.expect, ok, tt.complexity, tt.mode, tt.toolCount)
			}
		})
	}
}

func TestConditionUnknownVariable(t *testing.T) {
	_, err := evaluateCondition("unknown > 5", 0.5, "", 0)
	if err == nil {
		t.Error("expected error for unknown variable")
	}
}

func TestConditionInvalidComplexityValue(t *testing.T) {
	_, err := evaluateCondition("complexity > abc", 0.5, "", 0)
	if err == nil {
		t.Error("expected error for invalid complexity value")
	}
}

func TestConcurrentSelectSet(t *testing.T) {
	d := NewToolsetDistribution(1)
	d.RegisterSet("a", []string{"bash"}, 1.0)
	d.RegisterSet("b", []string{"read_file"}, 1.0)
	d.RegisterSet("c", []string{"grep"}, 1.0)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := d.SelectSet(context.Background(), 0.5)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent SelectSet error: %v", err)
	}
}

func TestRegistrySetDistribution(t *testing.T) {
	r := NewRegistryWithoutCache()
	if dist := r.GetDistribution(); dist != nil {
		t.Error("expected nil distribution initially")
	}

	d := NewToolsetDistribution(1)
	r.SetDistribution(d)
	if got := r.GetDistribution(); got != d {
		t.Error("expected distribution to be set")
	}
}

func TestRegistrySelectToolsetNoDistribution(t *testing.T) {
	r := NewRegistryWithoutCache()
	tools := r.SelectToolset(context.Background(), 0.5)
	if len(tools) != 0 {
		t.Fatalf("expected empty tools when no tools registered and no distribution, got %d", len(tools))
	}
}

func TestRegistrySelectToolset(t *testing.T) {
	r := NewRegistryWithoutCache()
	r.Register(newStubTool("bash"))
	r.Register(newStubTool("read_file"))
	r.Register(newStubTool("grep"))

	d := NewToolsetDistribution(1)
	d.RegisterSet("core", []string{"bash", "read_file"}, 1.0)
	r.SetDistribution(d)

	tools := r.SelectToolset(context.Background(), 0.5)
	if len(tools) == 0 {
		t.Fatalf("SelectToolset returned no tools")
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	if !names["bash"] || !names["read_file"] {
		t.Errorf("expected bash and read_file, got %v", names)
	}
	if names["grep"] {
		t.Error("grep should not be in core set")
	}
}
