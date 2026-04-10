package learning

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

type Suggestion struct {
	Text       string
	Confidence float64
	SkillID    string
}

type Action struct {
	Type    string
	Content string
	Tool    string
}

type ProactiveEngine struct {
	patterns    []WorkflowPattern
	userHistory []Action
	mu          sync.RWMutex
	maxHistory  int
}

type WorkflowPattern struct {
	Name       string
	Sequence   []string
	Suggest    string
	SkillID    string
	Confidence float64
}

func NewProactiveEngine() *ProactiveEngine {
	return &ProactiveEngine{
		patterns:    defaultWorkflowPatterns(),
		maxHistory:  50,
		userHistory: make([]Action, 0, 50),
	}
}

func defaultWorkflowPatterns() []WorkflowPattern {
	return []WorkflowPattern{
		{
			Name:       "edit-then-test",
			Sequence:   []string{"edit"},
			Suggest:    "Run tests for the changed files?",
			SkillID:    "test-generator",
			Confidence: 0.7,
		},
		{
			Name:       "fix-then-regression",
			Sequence:   []string{"debug", "fix"},
			Suggest:    "Write a regression test for this fix?",
			SkillID:    "test-generator",
			Confidence: 0.6,
		},
		{
			Name:       "new-file-then-gitignore",
			Sequence:   []string{"write:new"},
			Suggest:    "Add this file to .gitignore if needed?",
			SkillID:    "",
			Confidence: 0.4,
		},
		{
			Name:       "refactor-then-verify",
			Sequence:   []string{"refactor"},
			Suggest:    "Verify the refactored code still works?",
			SkillID:    "verify",
			Confidence: 0.65,
		},
		{
			Name:       "deploy-after-test",
			Sequence:   []string{"test:pass"},
			Suggest:    "Ready to deploy?",
			SkillID:    "deployment",
			Confidence: 0.5,
		},
	}
}

func (pe *ProactiveEngine) RecordAction(action Action) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pe.userHistory = append(pe.userHistory, action)

	if len(pe.userHistory) > pe.maxHistory {
		pe.userHistory = pe.userHistory[len(pe.userHistory)-pe.maxHistory:]
	}
}

func (pe *ProactiveEngine) MaybeSuggest(lastAction Action) *Suggestion {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	history := append(pe.userHistory, lastAction)

	var bestMatch *Suggestion

	for _, pattern := range pe.patterns {
		if matchesSequence(history, pattern.Sequence) {
			confidence := pattern.Confidence

			adjustedConf := adjustConfidence(confidence, lastAction)

			suggestion := &Suggestion{
				Text:       pattern.Suggest,
				Confidence: adjustedConf,
				SkillID:    pattern.SkillID,
			}

			if bestMatch == nil || adjustedConf > bestMatch.Confidence {
				bestMatch = suggestion
			}
		}
	}

	if bestMatch != nil && bestMatch.Confidence >= 0.4 {
		slog.Debug("proactive: suggestion generated",
			"text", bestMatch.Text,
			"confidence", fmt.Sprintf("%.2f", bestMatch.Confidence),
			"skill", bestMatch.SkillID,
		)
		return bestMatch
	}

	return nil
}

func matchesSequence(history []Action, sequence []string) bool {
	if len(history) < len(sequence) {
		return false
	}

	recentCount := len(sequence)
	recent := history[len(history)-recentCount:]

	for i, expected := range sequence {
		actual := classifyAction(recent[i])
		if !actionMatches(actual, expected) {
			return false
		}
	}

	return true
}

func classifyAction(action Action) string {
	tool := strings.ToLower(action.Tool)
	content := strings.ToLower(action.Content)

	switch {
	case tool == "edit_file" || tool == "write_file":
		if strings.Contains(content, "new") || strings.Contains(content, "create") {
			return "write:new"
		}
		return "edit"
	case tool == "bash" && (strings.Contains(content, "test") || strings.Contains(content, "go test") || strings.Contains(content, "pytest")):
		if strings.Contains(content, "pass") || !strings.Contains(content, "fail") {
			return "test:pass"
		}
		return "test:fail"
	case tool == "bash" && (strings.Contains(content, "debug") || strings.Contains(content, "dlv")):
		return "debug"
	case tool == "bash" && strings.Contains(content, "fix"):
		return "fix"
	case tool == "bash" && (strings.Contains(content, "refactor") || strings.Contains(content, "restructure")):
		return "refactor"
	case tool == "bash" && (strings.Contains(content, "deploy") || strings.Contains(content, "ship")):
		return "deploy"
	default:
		return action.Type
	}
}

func actionMatches(actual, expected string) bool {
	if actual == expected {
		return true
	}

	parts := strings.Split(expected, ":")
	if len(parts) == 2 && parts[0] == actual {
		return true
	}

	parts = strings.Split(actual, ":")
	if len(parts) == 2 && parts[0] == expected {
		return true
	}

	return false
}

func adjustConfidence(base float64, lastAction Action) float64 {
	adjusted := base

	content := strings.ToLower(lastAction.Content)
	if strings.Contains(content, "error") || strings.Contains(content, "fail") {
		adjusted -= 0.1
	}
	if strings.Contains(content, "success") || strings.Contains(content, "pass") {
		adjusted += 0.1
	}

	if adjusted < 0 {
		adjusted = 0
	}
	if adjusted > 1 {
		adjusted = 1
	}

	return adjusted
}

func (pe *ProactiveEngine) AddPattern(pattern WorkflowPattern) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.patterns = append(pe.patterns, pattern)
}

func (pe *ProactiveEngine) GetPatterns() []WorkflowPattern {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	result := make([]WorkflowPattern, len(pe.patterns))
	copy(result, pe.patterns)
	return result
}
