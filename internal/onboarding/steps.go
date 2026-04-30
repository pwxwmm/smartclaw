package onboarding

func GetSteps() []OnboardingStep {
	return []OnboardingStep{
		{
			Step:        1,
			Title:       "Fix a Bug",
			Description: "See how SmartClaw learns your debugging pattern and creates a reusable skill.",
			Prompt:      "Ask me to fix a simple bug",
			SkillName:   "bug-fix-workflow",
			Insight:     "I noticed your debugging pattern and created a skill. Next time, I'll use it automatically.",
		},
		{
			Step:        2,
			Title:       "Explain Code",
			Description: "Watch SmartClaw adapt to your preferred explanation style.",
			Prompt:      "Ask me to explain some code",
			SkillName:   "code-explanation",
			Insight:     "I'm learning you prefer concise explanations — I'll adjust my style.",
		},
		{
			Step:        3,
			Title:       "Run Tests",
			Description: "See how SmartClaw saves your test workflow for future reuse.",
			Prompt:      "Ask me to run the tests",
			SkillName:   "test-runner",
			Insight:     "I've saved your test workflow. SmartClaw now knows 3 things about how you work.",
		},
	}
}

func GetStep(n int) *OnboardingStep {
	for _, s := range GetSteps() {
		if s.Step == n {
			return &s
		}
	}
	return nil
}
