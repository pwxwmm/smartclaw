package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

type ExtractedSkill struct {
	Name        string
	Description string
	Triggers    []string
	Steps       []string
	Tools       []string
	Tags        []string
}

type Extractor struct {
	client LLMClient
}

func NewExtractor(client LLMClient) *Extractor {
	return &Extractor{client: client}
}

const extractionSystemPrompt = `You are a skill extraction assistant. Given a completed task and its evaluation, extract a reusable skill.

Create a skill that captures the METHOD, not the specific data. The skill should be general enough to apply to similar tasks.

Respond in JSON format:
{
  "name": "skill-name (kebab-case)",
  "description": "One-line description of what this skill does",
  "triggers": ["/trigger-command", "keyword1"],
  "steps": ["Step 1: description", "Step 2: description", ...],
  "tools": ["tool1", "tool2"],
  "tags": ["tag1", "tag2"]
}

Rules:
- Name must be kebab-case (e.g., "debug-go-tests")
- Triggers should include a slash command and relevant keywords
- Steps must be concrete and actionable
- Tools should be actual tool names (bash, read_file, write_file, grep, etc.)
- Tags should be lowercase and descriptive`

func (ex *Extractor) Extract(ctx context.Context, messages []Message, evaluation *TaskEvaluation) (*ExtractedSkill, error) {
	conversation := formatMessagesForPrompt(messages)

	userPrompt := fmt.Sprintf("Extract a reusable skill from this task.\n\nEvaluation:\n- Worth keeping: %v\n- Reason: %s\n- Key steps: %s\n- Suggested name: %s\n- Category: %s\n\nConversation:\n%s",
		evaluation.WorthKeeping,
		evaluation.Reason,
		strings.Join(evaluation.KeySteps, "; "),
		evaluation.SkillName,
		evaluation.SkillCategory,
		conversation,
	)

	response, err := ex.client.CreateMessage(ctx, extractionSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("extractor LLM call: %w", err)
	}

	skill := &ExtractedSkill{}
	if err := parseExtractionResponse(response, skill); err != nil {
		slog.Warn("learning extractor: failed to parse response, using evaluation fallback", "error", err)
		skill = fallbackSkill(evaluation)
	}

	return skill, nil
}

func parseExtractionResponse(response string, skill *ExtractedSkill) error {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return fmt.Errorf("no JSON found in response")
	}

	var raw struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Triggers    []string `json:"triggers"`
		Steps       []string `json:"steps"`
		Tools       []string `json:"tools"`
		Tags        []string `json:"tags"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return fmt.Errorf("JSON unmarshal: %w", err)
	}

	skill.Name = raw.Name
	skill.Description = raw.Description
	skill.Triggers = raw.Triggers
	skill.Steps = raw.Steps
	skill.Tools = raw.Tools
	skill.Tags = raw.Tags

	if skill.Name == "" {
		return fmt.Errorf("empty skill name")
	}

	return nil
}

func fallbackSkill(evaluation *TaskEvaluation) *ExtractedSkill {
	name := evaluation.SkillName
	if name == "" {
		name = "learned-skill"
	}

	return &ExtractedSkill{
		Name:        name,
		Description: evaluation.Reason,
		Triggers:    []string{"/" + name},
		Steps:       evaluation.KeySteps,
		Tools:       []string{"bash", "read_file"},
		Tags:        []string{evaluation.SkillCategory, "learned"},
	}
}
