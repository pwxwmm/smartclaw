package learning

import "fmt"

type NudgeConfig struct {
	Interval      int
	FlushMinTurns int
}

type NudgeEngine struct {
	config NudgeConfig
}

type NudgePrompt struct {
	Content string
}

func DefaultNudgeConfig() NudgeConfig {
	return NudgeConfig{
		Interval:      10,
		FlushMinTurns: 6,
	}
}

func NewNudgeEngine(config NudgeConfig) *NudgeEngine {
	if config.Interval <= 0 {
		config.Interval = 10
	}
	if config.FlushMinTurns <= 0 {
		config.FlushMinTurns = 6
	}
	return &NudgeEngine{config: config}
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
	}
}

func (ne *NudgeEngine) GetConfig() NudgeConfig {
	return ne.config
}
