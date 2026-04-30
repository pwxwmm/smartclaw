package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ImprovedSkill represents a skill that has been analyzed and revised.
type ImprovedSkill struct {
	Name         string
	Description  string
	Triggers     []string
	Steps        []string
	Tools        []string
	Tags         []string
	Version      int
	ChangeSummary string
}

// SkillImprover analyzes failed/retried skill invocations and generates
// improved versions using LLM-guided analysis.
type SkillImprover struct {
	client LLMClient
}

const improvementThreshold = 0.5
const improvementWindowInvocations = 5

const improvementSystemPrompt = `You are a skill improvement assistant. Given a skill that has been failing, analyze the failure patterns and generate an improved version of the skill.

The improved skill should:
1. Address the specific failure modes described
2. Add error handling steps where appropriate
3. Include fallback strategies
4. Be more robust while remaining concise

Respond in JSON format:
{
  "name": "skill-name (same as original, kebab-case)",
  "description": "Updated one-line description",
  "triggers": ["/trigger-command", "keyword1"],
  "steps": ["Step 1: description", "Step 2: description", ...],
  "tools": ["tool1", "tool2"],
  "tags": ["tag1", "tag2"],
  "change_summary": "Brief description of what changed and why"
}

Rules:
- Name must remain the same kebab-case name
- Steps must be concrete and actionable
- Address each failure mode explicitly
- Tools should be actual tool names (bash, read_file, write_file, grep, etc.)
- Tags should be lowercase and descriptive`

// NewSkillImprover creates a SkillImprover with the given LLM client.
func NewSkillImprover(client LLMClient) *SkillImprover {
	return &SkillImprover{client: client}
}

// ShouldImprove checks if a skill's success rate has dropped below the
// improvement threshold over the last N invocations.
func (si *SkillImprover) ShouldImprove(tracker *SkillTracker, skillName string) bool {
	if tracker == nil || tracker.store == nil {
		return false
	}

	recentScore, err := tracker.getRecentSuccessRate(skillName, improvementWindowInvocations)
	if err != nil {
		slog.Debug("skill improver: could not get recent success rate", "skill", skillName, "error", err)
		return false
	}

	if recentScore.InvocationCount < 3 {
		return false
	}

	return recentScore.SuccessRate < improvementThreshold
}

// Improve uses LLM to analyze failure patterns and generate an improved skill version.
func (si *SkillImprover) Improve(ctx context.Context, skillName string, failureMessages []string, originalSkill *ExtractedSkill) (*ImprovedSkill, error) {
	if si.client == nil {
		return nil, fmt.Errorf("skill improver: no LLM client configured")
	}

	originalContent := formatSkillMarkdown(originalSkill)
	failuresStr := strings.Join(failureMessages, "\n- ")

	userPrompt := fmt.Sprintf(
		"Improve this failing skill:\n\n## Original Skill\n%s\n\n## Failure Messages\n- %s\n\nGenerate an improved version that addresses these failures.",
		originalContent, failuresStr,
	)

	response, err := si.client.CreateMessage(ctx, improvementSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("skill improver: LLM call: %w", err)
	}

	improved := &ImprovedSkill{}
	if err := parseImprovementResponse(response, improved); err != nil {
		slog.Warn("skill improver: failed to parse response, using fallback", "error", err)
		improved = fallbackImprovement(originalSkill, failureMessages)
	}

	improved.Version = detectSkillVersion(originalSkill.Name) + 1

	return improved, nil
}

// ApplyImprovement writes the improved skill, incrementing the version.
func (si *SkillImprover) ApplyImprovement(skillWriter *SkillWriter, improved *ImprovedSkill) error {
	extracted := &ExtractedSkill{
		Name:        improved.Name,
		Description: improved.Description,
		Triggers:    improved.Triggers,
		Steps:       improved.Steps,
		Tools:       improved.Tools,
		Tags:        improved.Tags,
	}

	if err := skillWriter.WriteSkill(extracted); err != nil {
		return fmt.Errorf("skill improver: write improved skill: %w", err)
	}

	slog.Info("skill improver: applied improvement",
		"skill", improved.Name,
		"version", improved.Version,
		"changes", improved.ChangeSummary,
	)

	return nil
}

func parseImprovementResponse(response string, improved *ImprovedSkill) error {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return fmt.Errorf("no JSON found in response")
	}

	var raw struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Triggers      []string `json:"triggers"`
		Steps         []string `json:"steps"`
		Tools         []string `json:"tools"`
		Tags          []string `json:"tags"`
		ChangeSummary string   `json:"change_summary"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return fmt.Errorf("JSON unmarshal: %w", err)
	}

	if raw.Name == "" {
		return fmt.Errorf("empty skill name in improvement response")
	}

	improved.Name = raw.Name
	improved.Description = raw.Description
	improved.Triggers = raw.Triggers
	improved.Steps = raw.Steps
	improved.Tools = raw.Tools
	improved.Tags = raw.Tags
	improved.ChangeSummary = raw.ChangeSummary

	return nil
}

func fallbackImprovement(original *ExtractedSkill, failureMessages []string) *ImprovedSkill {
	steps := make([]string, len(original.Steps))
	copy(steps, original.Steps)

	if len(failureMessages) > 0 {
		errorHandling := fmt.Sprintf("Before starting, check for common failure conditions: %s", truncate(failureMessages[0], 100))
		steps = append([]string{errorHandling}, steps...)
	}

	steps = append(steps, "If any step fails, try an alternative approach and report what went wrong")

	return &ImprovedSkill{
		Name:          original.Name,
		Description:   original.Description + " (improved)",
		Triggers:      original.Triggers,
		Steps:         steps,
		Tools:         original.Tools,
		Tags:          append(original.Tags, "improved"),
		Version:       0,
		ChangeSummary: "Added error handling and fallback steps based on failure analysis",
	}
}

// detectSkillVersion looks at the skills directory to determine the current version.
func detectSkillVersion(skillName string) int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	skillDir := filepath.Join(home, ".smartclaw", "skills", skillName)

	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return 0
	}

	version := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "SKILL.md" {
			if version == 0 {
				version = 1
			}
		}
		if name == "SKILL.md.bak" {
			version = 2
		}
	}

	return version
}

// RecentSuccessRate holds data about recent skill performance.
type RecentSuccessRate struct {
	InvocationCount int
	SuccessRate     float64
}

// getRecentSuccessRate calculates the success rate over the last N invocations.
func (st *SkillTracker) getRecentSuccessRate(skillID string, lastN int) (RecentSuccessRate, error) {
	result := RecentSuccessRate{}

	if st.store == nil {
		return result, nil
	}

	row := st.store.DB().QueryRow(
		`SELECT COUNT(*) FROM (
			SELECT outcome FROM skill_outcomes
			WHERE skill_id = ?
			ORDER BY recorded_at DESC
			LIMIT ?
		)`,
		skillID, lastN,
	)
	var total int
	if err := row.Scan(&total); err != nil {
		return result, fmt.Errorf("skill tracker: count recent outcomes: %w", err)
	}

	if total == 0 {
		return result, nil
	}

	row = st.store.DB().QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END), 0)
		FROM (
			SELECT outcome FROM skill_outcomes
			WHERE skill_id = ?
			ORDER BY recorded_at DESC
			LIMIT ?
		)`,
		skillID, lastN,
	)
	var successes int
	if err := row.Scan(&successes); err != nil {
		return result, fmt.Errorf("skill tracker: count recent successes: %w", err)
	}

	result.InvocationCount = total
	result.SuccessRate = float64(successes) / float64(total)
	return result, nil
}
