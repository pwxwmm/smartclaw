package tools

import (
	"fmt"
	"strings"
	"sync"
)

// ToolCall represents a recorded tool invocation for chain analysis.
type ToolCall struct {
	Name   string         `json:"name"`
	Input  map[string]any `json:"input"`
	Output any            `json:"output,omitempty"`
}

// ChainPattern defines a sequence of tool calls that can be optimized.
type ChainPattern struct {
	Name             string   `json:"name"`
	Sequence         []string `json:"sequence"`          // ordered tool names, e.g. ["read_file","edit_file"]
	EstimatedSavings float64  `json:"estimated_savings"` // fraction of tokens saved, e.g. 0.5 = 50%
	MergeFunc        func(calls []ToolCall) *MergedCall
}

// OptimizationSuggestion describes a detected optimization opportunity.
type OptimizationSuggestion struct {
	StartIdx         int     `json:"start_idx"`
	EndIdx           int     `json:"end_idx"`
	Pattern          string  `json:"pattern"`
	EstimatedSavings float64 `json:"estimated_savings"`
	Description      string  `json:"description"`
}

// MergedCall represents the result of merging multiple tool calls.
type MergedCall struct {
	Script        string   `json:"script"`
	Language      string   `json:"language"`       // "bash", "python", "javascript"
	OriginalCalls []string `json:"original_calls"` // names of merged tools
	Savings       float64  `json:"savings"`        // estimated token savings fraction
}

// ChainOptimizer analyzes tool call sequences and suggests merges.
type ChainOptimizer struct {
	mu       sync.RWMutex
	patterns []ChainPattern
	callLog  []ToolCall
	maxLog   int
	enabled  bool
}

// NewChainOptimizer creates a ChainOptimizer with built-in patterns.
func NewChainOptimizer() *ChainOptimizer {
	o := &ChainOptimizer{
		patterns: make([]ChainPattern, 0),
		callLog:  make([]ToolCall, 0),
		maxLog:   50,
		enabled:  false,
	}
	o.registerBuiltinPatterns()
	return o
}

// Enable turns on chain optimization logging and analysis.
func (o *ChainOptimizer) Enable() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.enabled = true
}

// Disable turns off chain optimization.
func (o *ChainOptimizer) Disable() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.enabled = false
}

// IsEnabled returns whether the optimizer is active.
func (o *ChainOptimizer) IsEnabled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.enabled
}

// RegisterPattern adds a new optimization pattern.
func (o *ChainOptimizer) RegisterPattern(pattern ChainPattern) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.patterns = append(o.patterns, pattern)
}

// RecordCall logs a tool call for chain analysis.
func (o *ChainOptimizer) RecordCall(name string, input map[string]any, output any) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.enabled {
		return
	}

	o.callLog = append(o.callLog, ToolCall{
		Name:   name,
		Input:  input,
		Output: output,
	})

	if len(o.callLog) > o.maxLog {
		o.callLog = o.callLog[len(o.callLog)-o.maxLog:]
	}
}

// Analyze inspects the recorded call log and returns optimization suggestions.
func (o *ChainOptimizer) Analyze() []OptimizationSuggestion {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.enabled || len(o.callLog) < 2 {
		return nil
	}

	var suggestions []OptimizationSuggestion

	for _, pat := range o.patterns {
		seqLen := len(pat.Sequence)
		if seqLen < 2 || len(o.callLog) < seqLen {
			continue
		}

		// Sliding window over callLog
		for i := 0; i <= len(o.callLog)-seqLen; i++ {
			match := true
			for j, expected := range pat.Sequence {
				if o.callLog[i+j].Name != expected {
					match = false
					break
				}
			}
			if match {
				suggestions = append(suggestions, OptimizationSuggestion{
					StartIdx:         i,
					EndIdx:           i + seqLen - 1,
					Pattern:          pat.Name,
					EstimatedSavings: pat.EstimatedSavings,
					Description:      fmt.Sprintf("Merge %s into a single script (saves ~%.0f%% tokens)", strings.Join(pat.Sequence, " → "), pat.EstimatedSavings*100),
				})
			}
		}
	}

	return suggestions
}

// AnalyzeCalls analyzes an explicit list of calls (not the internal log).
func (o *ChainOptimizer) AnalyzeCalls(calls []ToolCall) []OptimizationSuggestion {
	o.mu.RLock()
	patterns := o.patterns
	o.mu.RUnlock()

	if len(calls) < 2 {
		return nil
	}

	var suggestions []OptimizationSuggestion

	for _, pat := range patterns {
		seqLen := len(pat.Sequence)
		if seqLen < 2 || len(calls) < seqLen {
			continue
		}

		for i := 0; i <= len(calls)-seqLen; i++ {
			match := true
			for j, expected := range pat.Sequence {
				if calls[i+j].Name != expected {
					match = false
					break
				}
			}
			if match {
				suggestions = append(suggestions, OptimizationSuggestion{
					StartIdx:         i,
					EndIdx:           i + seqLen - 1,
					Pattern:          pat.Name,
					EstimatedSavings: pat.EstimatedSavings,
					Description:      fmt.Sprintf("Merge %s into a single script (saves ~%.0f%% tokens)", strings.Join(pat.Sequence, " → "), pat.EstimatedSavings*100),
				})
			}
		}
	}

	return suggestions
}

// Merge combines the calls identified by a suggestion into a single merged call.
func (o *ChainOptimizer) Merge(calls []ToolCall, suggestion OptimizationSuggestion) *MergedCall {
	o.mu.RLock()
	patterns := o.patterns
	o.mu.RUnlock()

	if suggestion.StartIdx < 0 || suggestion.EndIdx >= len(calls) {
		return nil
	}

	subset := calls[suggestion.StartIdx : suggestion.EndIdx+1]

	for _, pat := range patterns {
		if pat.Name != suggestion.Pattern || pat.MergeFunc == nil {
			continue
		}

		seqLen := len(pat.Sequence)
		if len(subset) != seqLen {
			continue
		}

		// Verify the subset matches the pattern
		match := true
		for j, expected := range pat.Sequence {
			if subset[j].Name != expected {
				match = false
				break
			}
		}
		if match {
			return pat.MergeFunc(subset)
		}
	}

	return nil
}

// ClearLog empties the recorded call log.
func (o *ChainOptimizer) ClearLog() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.callLog = o.callLog[:0]
}

// GetCallLog returns a copy of the recorded call log.
func (o *ChainOptimizer) GetCallLog() []ToolCall {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]ToolCall, len(o.callLog))
	copy(result, o.callLog)
	return result
}

// registerBuiltinPatterns adds the built-in optimization patterns.
func (o *ChainOptimizer) registerBuiltinPatterns() {
	// read_file + edit_file → single sed/python script
	o.patterns = append(o.patterns, ChainPattern{
		Name:             "read_edit",
		Sequence:         []string{"read_file", "edit_file"},
		EstimatedSavings: 0.4,
		MergeFunc: func(calls []ToolCall) *MergedCall {
			readPath, _ := calls[0].Input["path"].(string)
			editPath, _ := calls[1].Input["path"].(string)
			oldStr, _ := calls[1].Input["old_string"].(string)
			newStr, _ := calls[1].Input["new_string"].(string)

			// If same file, produce a single script
			if readPath == editPath || editPath == "" {
				editPath = readPath
			}

			escapedOld := escapeShell(oldStr)
			escapedNew := escapeShell(newStr)

			script := fmt.Sprintf(`python3 -c "
import sys
path = '%s'
with open(path, 'r') as f:
    content = f.read()
content = content.replace(%s, %s, 1)
with open(path, 'w') as f:
    f.write(content)
print('Edited', path)
"`, editPath, escapedOld, escapedNew)

			return &MergedCall{
				Script:        script,
				Language:      "bash",
				OriginalCalls: []string{"read_file", "edit_file"},
				Savings:       0.4,
			}
		},
	})

	// glob + grep → single find -exec grep
	o.patterns = append(o.patterns, ChainPattern{
		Name:             "glob_grep",
		Sequence:         []string{"glob", "grep"},
		EstimatedSavings: 0.5,
		MergeFunc: func(calls []ToolCall) *MergedCall {
			globPattern, _ := calls[0].Input["pattern"].(string)
			grepPattern, _ := calls[1].Input["pattern"].(string)
			basePath, _ := calls[1].Input["path"].(string)
			if basePath == "" {
				basePath = "."
			}

			script := fmt.Sprintf("find %s -name '%s' -exec grep -n '%s' {} +", basePath, globPattern, grepPattern)

			return &MergedCall{
				Script:        script,
				Language:      "bash",
				OriginalCalls: []string{"glob", "grep"},
				Savings:       0.5,
			}
		},
	})

	// grep + read_file → grep with context
	o.patterns = append(o.patterns, ChainPattern{
		Name:             "grep_read",
		Sequence:         []string{"grep", "read_file"},
		EstimatedSavings: 0.35,
		MergeFunc: func(calls []ToolCall) *MergedCall {
			grepPattern, _ := calls[0].Input["pattern"].(string)
			readPath, _ := calls[1].Input["path"].(string)

			script := fmt.Sprintf("grep -n -C 3 '%s' '%s'", grepPattern, readPath)

			return &MergedCall{
				Script:        script,
				Language:      "bash",
				OriginalCalls: []string{"grep", "read_file"},
				Savings:       0.35,
			}
		},
	})

	// bash + bash → single combined script
	o.patterns = append(o.patterns, ChainPattern{
		Name:             "bash_bash",
		Sequence:         []string{"bash", "bash"},
		EstimatedSavings: 0.3,
		MergeFunc: func(calls []ToolCall) *MergedCall {
			cmd1, _ := calls[0].Input["command"].(string)
			cmd2, _ := calls[1].Input["command"].(string)

			script := cmd1 + " && " + cmd2

			return &MergedCall{
				Script:        script,
				Language:      "bash",
				OriginalCalls: []string{"bash", "bash"},
				Savings:       0.3,
			}
		},
	})

	// write_file + bash (format/lint) → write && format
	o.patterns = append(o.patterns, ChainPattern{
		Name:             "write_format",
		Sequence:         []string{"write_file", "bash"},
		EstimatedSavings: 0.25,
		MergeFunc: func(calls []ToolCall) *MergedCall {
			writePath, _ := calls[0].Input["path"].(string)
			writeContent, _ := calls[0].Input["content"].(string)
			bashCmd, _ := calls[1].Input["command"].(string)

			// Produce a script that writes and then runs the format command
			escapedContent := escapeShell(writeContent)

			script := fmt.Sprintf("cat > '%s' << 'SMARTCLAW_EOF'\n%s\nSMARTCLAW_EOF\n%s", writePath, writeContent, bashCmd)

			_ = escapedContent // Use heredoc approach instead

			return &MergedCall{
				Script:        script,
				Language:      "bash",
				OriginalCalls: []string{"write_file", "bash"},
				Savings:       0.25,
			}
		},
	})
}

// escapeShell escapes a string for safe embedding in a shell command.
func escapeShell(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `'\''`)
	return fmt.Sprintf("'%s'", s)
}
