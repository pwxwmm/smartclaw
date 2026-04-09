package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

type TaskEvaluation struct {
	WorthKeeping  bool
	Reason        string
	KeySteps      []string
	SkillName     string
	SkillCategory string
}

type Evaluator struct {
	client LLMClient
}

func NewEvaluator(client LLMClient) *Evaluator {
	return &Evaluator{client: client}
}

const evaluationSystemPrompt = `You are a task evaluation assistant. Analyze the completed task and determine if the approach used is worth preserving as a reusable skill.

A task is worth keeping if:
1. It involved a multi-step process that could recur
2. The approach was effective and reusable
3. It is not a trivial or one-off task

Respond in JSON format:
{
  "worth_keeping": true/false,
  "reason": "why it is or isn't worth keeping",
  "key_steps": ["step1", "step2", ...],
  "skill_name": "suggested-skill-name",
  "skill_category": "category (e.g., debugging, deployment, refactoring)"
}

Be conservative: if unsure, set worth_keeping to false.`

func (e *Evaluator) Evaluate(ctx context.Context, messages []Message, result *TaskResult) (*TaskEvaluation, error) {
	conversation := formatMessagesForPrompt(messages)

	userPrompt := "Evaluate this completed task:\n\n" + conversation
	if result != nil {
		userPrompt += fmt.Sprintf("\n\nTask result: stop_reason=%s, duration=%v, cost=$%.4f, tokens=%d",
			result.StopReason, result.Duration, result.Cost, result.TokensUsed)
	}

	response, err := e.client.CreateMessage(ctx, evaluationSystemPrompt, userPrompt)
	if err != nil {
		return &TaskEvaluation{WorthKeeping: false, Reason: "evaluation failed: " + err.Error()}, nil
	}

	eval := &TaskEvaluation{}
	if err := parseEvaluationResponse(response, eval); err != nil {
		slog.Warn("learning evaluator: failed to parse response, defaulting to not worth keeping", "error", err)
		return &TaskEvaluation{WorthKeeping: false, Reason: "parse failed"}, nil
	}

	return eval, nil
}

func parseEvaluationResponse(response string, eval *TaskEvaluation) error {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return fmt.Errorf("no JSON found in response")
	}

	var raw struct {
		WorthKeeping  bool     `json:"worth_keeping"`
		Reason        string   `json:"reason"`
		KeySteps      []string `json:"key_steps"`
		SkillName     string   `json:"skill_name"`
		SkillCategory string   `json:"skill_category"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return fmt.Errorf("JSON unmarshal: %w", err)
	}

	eval.WorthKeeping = raw.WorthKeeping
	eval.Reason = raw.Reason
	eval.KeySteps = raw.KeySteps
	eval.SkillName = raw.SkillName
	eval.SkillCategory = raw.SkillCategory
	return nil
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func formatMessagesForPrompt(messages []Message) string {
	var parts []string
	totalTokens := 0
	maxTokens := 4000

	for i := len(messages) - 1; i >= 0 && totalTokens < maxTokens; i-- {
		msg := messages[i]
		line := fmt.Sprintf("[%s]: %s", msg.Role, truncate(msg.Content, 500))
		parts = append([]string{line}, parts...)
		totalTokens += msg.Tokens
		if totalTokens == 0 {
			totalTokens += len(msg.Content) / 4
		}
	}

	return strings.Join(parts, "\n")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
