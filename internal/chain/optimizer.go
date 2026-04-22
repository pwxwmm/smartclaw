package chain

import "sort"

// ChainStep represents a single step in a tool-use chain.
type ChainStep struct {
	ToolName string
	Input    map[string]any
	Result   any
}

// Chain represents a sequence of steps.
type Chain struct {
	Steps  []ChainStep
	Query  string // the original user query
	Merged bool   // whether this chain was merged
}

// MergeSuggestion represents a suggestion to merge steps.
type MergeSuggestion struct {
	Steps      []int    // indices of steps to merge
	Reason     string   // why they can be merged
	Savings    int      // estimated API calls saved
	ToolName   string   // the merged tool name (e.g., "execute_code")
	Confidence float64  // 0.0-1.0 confidence in merge safety
}

// MergePattern defines a pattern of sequential tools that can be merged.
type MergePattern struct {
	Sequence []string // tool names in order
	MergedAs string   // what they become when merged
	Reason   string   // why this merge is safe
	Savings  int      // API calls saved
}

// ChainOptimizer detects mergeable tool sequences.
type ChainOptimizer struct {
	patterns []MergePattern
	enabled  bool
}

// NewChainOptimizer creates a ChainOptimizer with default patterns enabled.
func NewChainOptimizer() *ChainOptimizer {
	return &ChainOptimizer{
		patterns: defaultPatterns(),
		enabled:  true,
	}
}

// AnalyzeChain examines a chain of tool calls and returns merge suggestions.
// It uses greedy matching: once a subsequence is matched, those indices are
// consumed and cannot overlap with later suggestions.
// Results are sorted by savings descending.
func (co *ChainOptimizer) AnalyzeChain(chain *Chain) []MergeSuggestion {
	if chain == nil || len(chain.Steps) < 2 {
		return nil
	}

	var suggestions []MergeSuggestion
	used := make(map[int]bool)

	for i := 0; i < len(chain.Steps); i++ {
		if used[i] {
			continue
		}
		for _, pat := range co.patterns {
			if !co.matchesPattern(chain.Steps, i, pat, used) {
				continue
			}
			indices := co.patternIndices(i, len(pat.Sequence))
			confidence := co.computeConfidence(chain.Steps, indices)

			suggestions = append(suggestions, MergeSuggestion{
				Steps:      indices,
				Reason:     pat.Reason,
				Savings:    pat.Savings,
				ToolName:   pat.MergedAs,
				Confidence: confidence,
			})

			for _, idx := range indices {
				used[idx] = true
			}
			break
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Savings > suggestions[j].Savings
	})

	return suggestions
}

// ShouldMerge returns whether the optimizer recommends merging for this query.
func (co *ChainOptimizer) ShouldMerge(query string, history []ChainStep) bool {
	if !co.enabled || len(history) < 2 {
		return false
	}
	chain := &Chain{Steps: history, Query: query}
	suggestions := co.AnalyzeChain(chain)
	for _, s := range suggestions {
		if s.Confidence >= 0.7 {
			return true
		}
	}
	return false
}

// EstimateSavings returns estimated API calls and cost savings from merging.
func (co *ChainOptimizer) EstimateSavings(chain *Chain) (callsSaved int, confidence float64) {
	if chain == nil || len(chain.Steps) < 2 {
		return 0, 0
	}
	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) == 0 {
		return 0, 0
	}

	totalSaved := 0
	var weightedConf float64
	for _, s := range suggestions {
		totalSaved += s.Savings
		weightedConf += s.Confidence * float64(s.Savings)
	}
	totalWeight := float64(totalSaved)
	if totalWeight > 0 {
		confidence = weightedConf / totalWeight
	} else {
		confidence = 0
	}
	return totalSaved, confidence
}

// IsEnabled returns whether chain optimization is active.
func (co *ChainOptimizer) IsEnabled() bool {
	return co.enabled
}

// SetEnabled toggles chain optimization.
func (co *ChainOptimizer) SetEnabled(enabled bool) {
	co.enabled = enabled
}

// AddPattern registers a custom merge pattern.
func (co *ChainOptimizer) AddPattern(pattern MergePattern) {
	co.patterns = append(co.patterns, pattern)
}

// matchesPattern checks whether the chain steps starting at position i
// match the given pattern, skipping any indices already used.
func (co *ChainOptimizer) matchesPattern(steps []ChainStep, start int, pat MergePattern, used map[int]bool) bool {
	if start+len(pat.Sequence) > len(steps) {
		return false
	}
	for j, name := range pat.Sequence {
		idx := start + j
		if used[idx] {
			return false
		}
		if steps[idx].ToolName != name {
			return false
		}
	}
	return true
}

// patternIndices returns a slice of consecutive indices [start, start+n).
func (co *ChainOptimizer) patternIndices(start, n int) []int {
	indices := make([]int, n)
	for i := range n {
		indices[i] = start + i
	}
	return indices
}

// computeConfidence determines merge confidence based on step inputs.
//
// Rules:
//   - read_file + edit_file on the same file → 0.9
//   - read_file + edit_file on different files → 0.5
//   - bash + bash with no shared state → 0.7
//   - glob/grep + read_file on same path → 0.9
//   - glob/grep + read_file on different paths → 0.5
//   - edit_file + bash (edit-then-test) → 0.8
//   - bash + edit_file → 0.6
//   - Default for other patterns → 0.7
func (co *ChainOptimizer) computeConfidence(steps []ChainStep, indices []int) float64 {
	if len(indices) < 2 {
		return 0.7
	}

	first := steps[indices[0]]
	second := steps[indices[1]]

	firstFile := extractFilePath(first.Input)
	secondFile := extractFilePath(second.Input)

	switch {
	case first.ToolName == "read_file" && second.ToolName == "edit_file":
		if firstFile != "" && firstFile == secondFile {
			return 0.9
		}
		return 0.5

	case first.ToolName == "edit_file" && second.ToolName == "bash":
		return 0.8

	case first.ToolName == "bash" && second.ToolName == "edit_file":
		return 0.6

	case first.ToolName == "bash" && second.ToolName == "bash":
		return 0.7

	case (first.ToolName == "glob" || first.ToolName == "grep") && second.ToolName == "read_file":
		if firstFile != "" && firstFile == secondFile {
			return 0.9
		}
		if firstFile != "" && secondFile != "" && sameDirectory(firstFile, secondFile) {
			return 0.8
		}
		return 0.5

	case first.ToolName == "read_file" && second.ToolName == "read_file":
		if firstFile != "" && firstFile == secondFile {
			return 0.9
		}
		return 0.5

	default:
		return 0.7
	}
}

// extractFilePath tries to extract a file path from a step's input map.
// It checks common key names: "path", "file_path", "filePath", "file",
// "filename", "directory", "dir", "pattern", "query".
func extractFilePath(input map[string]any) string {
	if input == nil {
		return ""
	}
	for _, key := range []string{"path", "file_path", "filePath", "file", "filename", "directory", "dir", "pattern", "query"} {
		if v, ok := input[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// sameDirectory checks whether two file paths share the same directory prefix.
func sameDirectory(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	i := lastSlash(a)
	j := lastSlash(b)
	if i < 0 || j < 0 {
		return false
	}
	return a[:i] == b[:j]
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
