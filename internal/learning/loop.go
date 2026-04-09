package learning

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
	Tokens    int
}

type TaskResult struct {
	StopReason string
	Duration   time.Duration
	Cost       float64
	TokensUsed int
}

type LLMClient interface {
	CreateMessage(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type PromptMemoryWriter interface {
	AppendToSection(section, line string) error
	UpdateMemory(content string) error
	UpdateUserProfile(profile string) error
	AutoLoad() string
}

type LearningLoop struct {
	evaluator   *Evaluator
	extractor   *Extractor
	skillWriter *SkillWriter
	nudgeEngine *NudgeEngine
	promptMem   PromptMemoryWriter
	enabled     bool
}

func NewLearningLoop(llmClient LLMClient, promptMem PromptMemoryWriter, skillsDir string) *LearningLoop {
	if llmClient == nil {
		slog.Warn("learning loop: no LLM client, learning disabled")
		return &LearningLoop{enabled: false}
	}

	return &LearningLoop{
		evaluator:   NewEvaluator(llmClient),
		extractor:   NewExtractor(llmClient),
		skillWriter: NewSkillWriter(skillsDir),
		nudgeEngine: NewNudgeEngine(DefaultNudgeConfig()),
		promptMem:   promptMem,
		enabled:     true,
	}
}

func (l *LearningLoop) IsEnabled() bool {
	return l.enabled
}

func (l *LearningLoop) OnTaskComplete(ctx context.Context, sessionID string, messages []Message, result *TaskResult) error {
	if !l.enabled || len(messages) < 4 {
		return nil
	}

	slog.Info("learning loop: evaluating completed task", "session", sessionID, "messages", len(messages))

	evaluation, err := l.evaluator.Evaluate(ctx, messages, result)
	if err != nil {
		return fmt.Errorf("learning loop evaluate: %w", err)
	}

	if !evaluation.WorthKeeping {
		slog.Info("learning loop: task not worth keeping", "session", sessionID, "reason", evaluation.Reason)
		return nil
	}

	slog.Info("learning loop: task worth keeping", "session", sessionID, "skill", evaluation.SkillName)

	skill, err := l.extractor.Extract(ctx, messages, evaluation)
	if err != nil {
		return fmt.Errorf("learning loop extract: %w", err)
	}

	if err := l.skillWriter.WriteSkill(skill); err != nil {
		return fmt.Errorf("learning loop write skill: %w", err)
	}

	if l.promptMem != nil {
		if err := l.promptMem.AppendToSection("Learned Patterns",
			fmt.Sprintf("- %s: %s", skill.Name, skill.Description)); err != nil {
			slog.Error("learning loop: failed to update MEMORY.md", "error", err)
		}
	}

	slog.Info("learning loop: skill created", "name", skill.Name, "triggers", skill.Triggers)
	return nil
}

func (l *LearningLoop) OnNudge(ctx context.Context, sessionID string, messages []Message) error {
	if !l.enabled {
		return nil
	}

	slog.Info("learning loop: nudge triggered", "session", sessionID, "messages", len(messages))

	evaluation, err := l.evaluator.Evaluate(ctx, messages, nil)
	if err != nil {
		return fmt.Errorf("learning loop nudge evaluate: %w", err)
	}

	if evaluation.WorthKeeping {
		skill, err := l.extractor.Extract(ctx, messages, evaluation)
		if err != nil {
			return fmt.Errorf("learning loop nudge extract: %w", err)
		}

		if err := l.skillWriter.WriteSkill(skill); err != nil {
			return fmt.Errorf("learning loop nudge write: %w", err)
		}

		if l.promptMem != nil {
			_ = l.promptMem.AppendToSection("Learned Patterns",
				fmt.Sprintf("- %s: %s", skill.Name, skill.Description))
		}
	}

	return nil
}

func (l *LearningLoop) MaybeNudge(currentTurn int) *NudgePrompt {
	return l.nudgeEngine.MaybeNudge(currentTurn)
}
