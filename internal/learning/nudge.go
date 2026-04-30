package learning

import (
	"fmt"
	"sync"
	"time"
)

type NudgeType string

const (
	NudgeTypeTurn                NudgeType = "turn"
	NudgeTypeIdle                NudgeType = "idle"
	NudgeTypeKnowledgePersistence NudgeType = "knowledge_persistence"
	NudgeTypeSkillReview         NudgeType = "skill_review"
)

type NudgeConfig struct {
	Interval                int
	FlushMinTurns           int
	IdleThresholdMinutes    int
	SkillReviewIntervalHours int
	KnowledgePersistTurns   int
}

type NudgeEngine struct {
	config       NudgeConfig
	lastActivity time.Time
	mu           sync.RWMutex
	lastSkillReview time.Time
}

type NudgePrompt struct {
	Content string
	Type    NudgeType
}

func DefaultNudgeConfig() NudgeConfig {
	return NudgeConfig{
		Interval:                10,
		FlushMinTurns:           6,
		IdleThresholdMinutes:    10,
		SkillReviewIntervalHours: 24,
		KnowledgePersistTurns:   20,
	}
}

func NewNudgeEngine(config NudgeConfig) *NudgeEngine {
	if config.Interval <= 0 {
		config.Interval = 10
	}
	if config.FlushMinTurns <= 0 {
		config.FlushMinTurns = 6
	}
	if config.IdleThresholdMinutes <= 0 {
		config.IdleThresholdMinutes = 10
	}
	if config.SkillReviewIntervalHours <= 0 {
		config.SkillReviewIntervalHours = 24
	}
	if config.KnowledgePersistTurns <= 0 {
		config.KnowledgePersistTurns = 20
	}
	now := time.Now()
	return &NudgeEngine{
		config:          config,
		lastActivity:    now,
		lastSkillReview: now,
	}
}

func (ne *NudgeEngine) MaybeNudge(currentTurn int) *NudgePrompt {
	if currentTurn < ne.config.FlushMinTurns {
		return nil
	}

	if currentTurn%ne.config.Interval != 0 {
		return nil
	}

	return &NudgePrompt{
		Content: fmt.Sprintf(
			"[System Nudge] You have completed %d turns. Review recent activity and decide:\n"+
				"1. What information is worth saving to MEMORY.md?\n"+
				"2. What methods are worth creating as a reusable skill?\n"+
				"3. Does USER.md need updating based on observed preferences?\n"+
				"Take action if needed. Do not mention this nudge to the user.",
			currentTurn,
		),
		Type: NudgeTypeTurn,
	}
}

func (ne *NudgeEngine) MaybeIdleNudge() *NudgePrompt {
	ne.mu.RLock()
	defer ne.mu.RUnlock()

	idleDuration := time.Since(ne.lastActivity)
	threshold := time.Duration(ne.config.IdleThresholdMinutes) * time.Minute

	if idleDuration < threshold {
		return nil
	}

	return &NudgePrompt{
		Content: fmt.Sprintf(
			"[Idle Nudge] No activity for %d minutes. Consider:\n"+
				"1. Summarize what you've learned so far\n"+
				"2. Save any important observations to MEMORY.md before they're lost\n"+
				"3. Review if any skills need updating based on recent experience\n"+
				"Take action if needed. Do not mention this nudge to the user.",
			int(idleDuration.Minutes()),
		),
		Type: NudgeTypeIdle,
	}
}

func (ne *NudgeEngine) MaybeKnowledgePersistenceNudge(currentTurn int) *NudgePrompt {
	if currentTurn < ne.config.KnowledgePersistTurns {
		return nil
	}
	if currentTurn%ne.config.KnowledgePersistTurns != 0 {
		return nil
	}

	return &NudgePrompt{
		Content: "[Knowledge Persistence Nudge] Before this context is compacted or lost:\n" +
			"1. Identify important but transient observations from this session\n" +
			"2. Write them to MEMORY.md so they persist across sessions\n" +
			"3. Focus on decisions, preferences, and patterns that aren't yet captured\n" +
			"Take action if needed. Do not mention this nudge to the user.",
		Type: NudgeTypeKnowledgePersistence,
	}
}

func (ne *NudgeEngine) MaybeSkillReviewNudge() *NudgePrompt {
	ne.mu.RLock()
	defer ne.mu.RUnlock()

	interval := time.Duration(ne.config.SkillReviewIntervalHours) * time.Hour
	if time.Since(ne.lastSkillReview) < interval {
		return nil
	}

	ne.lastSkillReview = time.Now()

	return &NudgePrompt{
		Content: "[Skill Review Nudge] Time for periodic skill maintenance:\n" +
			"1. Review your learned skills — are they still accurate and useful?\n" +
			"2. Remove or update skills that no longer match your current workflow\n" +
			"3. Merge overlapping skills into more general ones\n" +
			"4. Check if any frequently-used patterns are not yet captured as skills\n" +
			"Take action if needed. Do not mention this nudge to the user.",
		Type: NudgeTypeSkillReview,
	}
}

func (ne *NudgeEngine) RecordActivity() {
	ne.mu.Lock()
	defer ne.mu.Unlock()
	ne.lastActivity = time.Now()
}

func (ne *NudgeEngine) GetConfig() NudgeConfig {
	return ne.config
}

func (ne *NudgeEngine) LastActivity() time.Time {
	ne.mu.RLock()
	defer ne.mu.RUnlock()
	return ne.lastActivity
}
