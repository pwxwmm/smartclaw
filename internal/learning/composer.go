package learning

import (
	"fmt"
	"log/slog"
	"strings"
)

type SkillStep struct {
	SkillName   string
	Description string
	InputHints  map[string]string
}

type Workflow struct {
	Name         string
	Steps        []WorkflowStep
	Dependencies map[string][]string
}

type WorkflowStep struct {
	Name       string
	SkillName  string
	InputHints map[string]string
	OutputKey  string
}

type SkillComposer struct {
	skillTracker *SkillTracker
}

func NewSkillComposer(tracker *SkillTracker) *SkillComposer {
	return &SkillComposer{
		skillTracker: tracker,
	}
}

func (sc *SkillComposer) Compose(steps []SkillStep) *Workflow {
	if len(steps) == 0 {
		return nil
	}

	name := steps[0].SkillName
	for _, s := range steps[1:] {
		name += "_" + s.SkillName
	}

	workflow := &Workflow{
		Name:         name,
		Steps:        make([]WorkflowStep, len(steps)),
		Dependencies: make(map[string][]string),
	}

	for i, step := range steps {
		workflow.Steps[i] = WorkflowStep{
			Name:       fmt.Sprintf("step_%d_%s", i+1, step.SkillName),
			SkillName:  step.SkillName,
			InputHints: step.InputHints,
			OutputKey:  fmt.Sprintf("step_%d_output", i+1),
		}

		if i > 0 {
			prevStep := workflow.Steps[i-1].Name
			workflow.Dependencies[workflow.Steps[i].Name] = []string{prevStep}
		}
	}

	slog.Info("skill composer: composed workflow", "name", name, "steps", len(steps))
	return workflow
}

func (sc *SkillComposer) Decompose(goal string) []SkillStep {
	var steps []SkillStep

	lower := strings.ToLower(goal)

	type goalPattern struct {
		keywords []string
		steps    []SkillStep
	}

	patterns := []goalPattern{
		{
			keywords: []string{"refactor", "deploy"},
			steps: []SkillStep{
				{SkillName: "refactoring", Description: "Refactor the code", InputHints: map[string]string{"scope": "identified_code"}},
				{SkillName: "test-generator", Description: "Generate tests for refactored code", InputHints: map[string]string{"target": "refactored_output"}},
				{SkillName: "deployment", Description: "Deploy the changes", InputHints: map[string]string{"artifact": "tested_output"}},
			},
		},
		{
			keywords: []string{"fix", "deploy"},
			steps: []SkillStep{
				{SkillName: "debugger", Description: "Debug and fix the issue", InputHints: map[string]string{"error": "identified_error"}},
				{SkillName: "test-generator", Description: "Write regression test", InputHints: map[string]string{"fix": "fixed_output"}},
				{SkillName: "deployment", Description: "Deploy the fix", InputHints: map[string]string{"artifact": "tested_output"}},
			},
		},
		{
			keywords: []string{"new", "feature", "test"},
			steps: []SkillStep{
				{SkillName: "api-designer", Description: "Design the feature API", InputHints: map[string]string{"requirements": "feature_spec"}},
				{SkillName: "documentation", Description: "Document the feature", InputHints: map[string]string{"api": "designed_api"}},
				{SkillName: "test-generator", Description: "Generate tests", InputHints: map[string]string{"feature": "documented_feature"}},
			},
		},
	}

	for _, pattern := range patterns {
		allMatch := true
		for _, kw := range pattern.keywords {
			if !strings.Contains(lower, kw) {
				allMatch = false
				break
			}
		}
		if allMatch {
			steps = pattern.steps
			break
		}
	}

	if len(steps) == 0 {
		keywords := extractKeywords(lower)
		for _, kw := range keywords {
			steps = append(steps, SkillStep{
				SkillName:   kw,
				Description: fmt.Sprintf("Execute %s skill", kw),
				InputHints:  map[string]string{"goal": goal},
			})
		}
	}

	return steps
}

func extractKeywords(text string) []string {
	skillKeywords := map[string]bool{
		"refactor": true, "test": true, "deploy": true,
		"debug": true, "document": true, "review": true,
		"security": true, "performance": true, "simplify": true,
	}

	var found []string
	words := strings.Fields(text)
	for _, w := range words {
		if skillKeywords[w] {
			skill := mapKeywordToSkill(w)
			found = append(found, skill)
		}
	}

	if len(found) == 0 {
		found = append(found, "code-review")
	}

	return found
}

func mapKeywordToSkill(keyword string) string {
	mapping := map[string]string{
		"refactor":    "refactoring",
		"test":        "test-generator",
		"deploy":      "deployment",
		"debug":       "debugger",
		"document":    "documentation",
		"review":      "code-review",
		"security":    "security",
		"performance": "performance",
		"simplify":    "simplify",
	}
	if skill, ok := mapping[keyword]; ok {
		return skill
	}
	return keyword
}
