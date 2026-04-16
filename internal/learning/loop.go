package learning

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
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
	EnforceLimit() error
}

type LearningLoop struct {
	evaluator    *Evaluator
	extractor    *Extractor
	skillWriter  *SkillWriter
	skillAuditor *SkillAuditor
	skillTracker *SkillTracker
	nudgeEngine  *NudgeEngine
	promptMem    PromptMemoryWriter
	enabled      bool
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

func (l *LearningLoop) SetSkillAuditor(auditor *SkillAuditor) {
	l.skillAuditor = auditor
}

func (l *LearningLoop) SetSkillTracker(tracker *SkillTracker) {
	l.skillTracker = tracker
}

func (l *LearningLoop) SetStore(s *store.Store) {
	if l.skillAuditor == nil && s != nil {
		l.skillAuditor = NewSkillAuditor(s, l.skillWriter.GetSkillsDir())
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

	if l.skillAuditor != nil {
		l.skillAuditor.RecordSkillUse(skill.Name)
	}

	if l.skillTracker != nil {
		if err := l.skillTracker.RecordInvocation(skill.Name, sessionID); err != nil {
			slog.Warn("learning loop: failed to record skill invocation", "error", err)
		}
		if err := l.skillTracker.RecordOutcome(skill.Name, OutcomeSuccess, sessionID); err != nil {
			slog.Warn("learning loop: failed to record skill outcome", "error", err)
		}
	}

	if l.promptMem != nil {
		if err := l.promptMem.AppendToSection("Learned Patterns",
			fmt.Sprintf("- %s: %s", skill.Name, skill.Description)); err != nil {
			slog.Error("learning loop: failed to update MEMORY.md", "error", err)
		}
		if err := l.promptMem.EnforceLimit(); err != nil {
			slog.Warn("learning loop: EnforceLimit failed", "error", err)
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

	if l.skillAuditor != nil {
		auditResult, err := l.skillAuditor.AuditStaleSkills(DefaultAuditConfig())
		if err != nil {
			slog.Warn("learning loop: skill audit failed", "error", err)
		} else if len(auditResult.Evicted) > 0 {
			slog.Info("learning loop: evicted stale skills", "count", len(auditResult.Evicted), "names", auditResult.Evicted)
			if l.promptMem != nil {
				if err := l.promptMem.EnforceLimit(); err != nil {
					slog.Warn("failed to enforce prompt memory limit", "error", err)
				}
			}
		}
	}

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
			return fmt.Errorf("learning loop nudge write skill: %w", err)
		}

		if l.skillAuditor != nil {
			l.skillAuditor.RecordSkillUse(skill.Name)
		}

		if l.skillTracker != nil {
			if err := l.skillTracker.RecordInvocation(skill.Name, sessionID); err != nil {
				slog.Warn("learning loop: failed to record nudge skill invocation", "error", err)
			}
			if err := l.skillTracker.RecordOutcome(skill.Name, OutcomeSuccess, sessionID); err != nil {
				slog.Warn("learning loop: failed to record nudge skill outcome", "error", err)
			}
		}

		if l.promptMem != nil {
			if err := l.promptMem.AppendToSection("Learned Patterns",
				fmt.Sprintf("- %s: %s", skill.Name, skill.Description)); err != nil {
				slog.Warn("failed to append learned pattern to prompt memory", "error", err)
			}
		}
	}

	if l.promptMem != nil {
		if err := l.promptMem.EnforceLimit(); err != nil {
			slog.Warn("learning loop: nudge EnforceLimit failed", "error", err)
		}
	}

	return nil
}

func (l *LearningLoop) MaybeNudge(currentTurn int) *NudgePrompt {
	return l.nudgeEngine.MaybeNudge(currentTurn)
}
