package tools

import (
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ToolsetConfig defines a named set of tools with a selection weight and optional condition.
type ToolsetConfig struct {
	Name      string   // unique name of this toolset
	Tools     []string // tool names belonging to this set
	Weight    float64  // probability weight (default: 1.0)
	Condition string   // optional condition expression, e.g. "complexity > 0.7"
}

// ToolsetDistribution implements probability-weighted toolset selection with
// configurable distributions and seed control.
type ToolsetDistribution struct {
	sets map[string]*ToolsetConfig
	seed int64
	rng  *rand.Rand
	mu   sync.Mutex
}

// NewToolsetDistribution creates a new ToolsetDistribution. If seed is 0, a
// time-based seed is used for non-deterministic selection.
func NewToolsetDistribution(seed int64) *ToolsetDistribution {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &ToolsetDistribution{
		sets: make(map[string]*ToolsetConfig),
		seed: seed,
		rng:  rand.New(rand.NewPCG(uint64(seed), uint64(seed>>32))),
	}
}

// RegisterSet registers a toolset with the given name, tools, and weight.
func (d *ToolsetDistribution) RegisterSet(name string, tools []string, weight float64) {
	d.RegisterSetWithCondition(name, tools, weight, "")
}

// RegisterSetWithCondition registers a toolset with an additional condition expression.
func (d *ToolsetDistribution) RegisterSetWithCondition(name string, tools []string, weight float64, condition string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if weight <= 0 {
		weight = 1.0
	}
	d.sets[name] = &ToolsetConfig{
		Name:      name,
		Tools:     tools,
		Weight:    weight,
		Condition: condition,
	}
}

// SelectSet selects a toolset based on weighted probability and condition evaluation.
// The complexity parameter is passed to condition evaluation.
func (d *ToolsetDistribution) SelectSet(ctx context.Context, complexity float64) (*ToolsetConfig, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.sets) == 0 {
		return nil, fmt.Errorf("no toolsets registered")
	}

	// Filter eligible sets by condition.
	var eligible []*ToolsetConfig
	var totalWeight float64
	for _, set := range d.sets {
		ok, err := evaluateCondition(set.Condition, complexity, "", 0)
		if err != nil {
			return nil, fmt.Errorf("condition evaluation failed for set %q: %w", set.Name, err)
		}
		if ok {
			eligible = append(eligible, set)
			totalWeight += set.Weight
		}
	}

	if len(eligible) == 0 {
		return nil, fmt.Errorf("no eligible toolsets for complexity=%.2f", complexity)
	}

	if len(eligible) == 1 {
		result := *eligible[0]
		return &result, nil
	}

	// Sort eligible sets by name for deterministic selection order.
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].Name < eligible[j].Name
	})

	// Weighted random selection.
	r := d.rng.Float64() * totalWeight
	var cumulative float64
	for _, set := range eligible {
		cumulative += set.Weight
		if r <= cumulative {
			result := *set
			return &result, nil
		}
	}

	// Fallback to last eligible set (floating-point edge case).
	result := *eligible[len(eligible)-1]
	return &result, nil
}

// SelectTools selects a toolset and returns matching tools from the available registry.
// If the selected set has an empty Tools list, all available tools are returned.
func (d *ToolsetDistribution) SelectTools(ctx context.Context, complexity float64, available map[string]Tool) ([]Tool, error) {
	set, err := d.SelectSet(ctx, complexity)
	if err != nil {
		return nil, err
	}

	// Empty tools list means "all tools".
	if len(set.Tools) == 0 {
		all := make([]Tool, 0, len(available))
		for _, tool := range available {
			all = append(all, tool)
		}
		return all, nil
	}

	var result []Tool
	for _, name := range set.Tools {
		if tool, ok := available[name]; ok {
			result = append(result, tool)
		}
	}
	return result, nil
}

// ListSets returns all registered toolsets.
func (d *ToolsetDistribution) ListSets() []ToolsetConfig {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make([]ToolsetConfig, 0, len(d.sets))
	for _, set := range d.sets {
		result = append(result, *set)
	}
	return result
}

// GetSet returns a specific toolset by name, or nil if not found.
func (d *ToolsetDistribution) GetSet(name string) *ToolsetConfig {
	d.mu.Lock()
	defer d.mu.Unlock()

	if set, ok := d.sets[name]; ok {
		result := *set
		return &result
	}
	return nil
}

// RemoveSet removes a toolset by name.
func (d *ToolsetDistribution) RemoveSet(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sets, name)
}

// SetSeed changes the RNG seed, useful for reproducibility.
func (d *ToolsetDistribution) SetSeed(seed int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seed = seed
	d.rng = rand.New(rand.NewPCG(uint64(seed), uint64(seed>>32)))
}

// DefaultToolsets returns the predefined toolset configurations.
func DefaultToolsets() []ToolsetConfig {
	return []ToolsetConfig{
		{Name: "core", Tools: []string{"bash", "read_file", "write_file", "edit_file", "glob", "grep"}, Weight: 1.0, Condition: "always"},
		{Name: "web", Tools: []string{"web_fetch", "web_search", "browser_navigate", "browser_screenshot"}, Weight: 0.3, Condition: "complexity > 0.3"},
		{Name: "code", Tools: []string{"lsp", "ast_grep", "code_search", "index"}, Weight: 0.5, Condition: "complexity > 0.5"},
		{Name: "sre", Tools: []string{"sopa_agent_list", "sopa_inventory_nodes_list", "sopa_fault_tracking_list"}, Weight: 0.2, Condition: "always"},
		{Name: "full", Tools: []string{}, Weight: 0.1, Condition: "complexity > 0.9"},
	}
}

var (
	conditionRe = regexp.MustCompile(`^\s*(\w+)\s*(>=|<=|!=|==|>|<)\s*(.+?)\s*$`)
)

// evaluateCondition evaluates a simple condition expression.
// Supported variables: complexity (float64), mode (string), tool_count (int).
// Supported operators: >, <, >=, <=, ==, !=
// Empty string or "always" always returns true.
func evaluateCondition(cond string, complexity float64, mode string, toolCount int) (bool, error) {
	cond = strings.TrimSpace(cond)
	if cond == "" || strings.EqualFold(cond, "always") {
		return true, nil
	}

	matches := conditionRe.FindStringSubmatch(cond)
	if matches == nil {
		return false, fmt.Errorf("invalid condition expression: %q", cond)
	}

	varName := matches[1]
	op := matches[2]
	valStr := strings.TrimSpace(matches[3])

	switch varName {
	case "complexity":
		threshold, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return false, fmt.Errorf("invalid complexity value %q: %w", valStr, err)
		}
		return compareFloat(complexity, op, threshold), nil

	case "mode":
		valStr = strings.Trim(valStr, `"`)
		return compareString(mode, op, valStr), nil

	case "tool_count":
		threshold, err := strconv.Atoi(valStr)
		if err != nil {
			return false, fmt.Errorf("invalid tool_count value %q: %w", valStr, err)
		}
		return compareInt(toolCount, op, threshold), nil

	default:
		return false, fmt.Errorf("unknown variable in condition: %q", varName)
	}
}

func compareFloat(val float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return val > threshold
	case "<":
		return val < threshold
	case ">=":
		return val >= threshold
	case "<=":
		return val <= threshold
	case "==":
		return val == threshold
	case "!=":
		return val != threshold
	default:
		return false
	}
}

func compareString(val, op, threshold string) bool {
	switch op {
	case "==":
		return val == threshold
	case "!=":
		return val != threshold
	default:
		return false
	}
}

func compareInt(val int, op string, threshold int) bool {
	switch op {
	case ">":
		return val > threshold
	case "<":
		return val < threshold
	case ">=":
		return val >= threshold
	case "<=":
		return val <= threshold
	case "==":
		return val == threshold
	case "!=":
		return val != threshold
	default:
		return false
	}
}
