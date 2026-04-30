package onboarding

type OnboardingState struct {
	UserID    string `json:"user_id"`
	Step      int    `json:"step"`       // 0=not started, 1-3=steps, 4=completed
	StartedAt int64  `json:"started_at"`
	DoneAt    int64  `json:"done_at,omitempty"`
}

type OnboardingStep struct {
	Step        int    `json:"step"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`       // What to tell the user to try
	SkillName   string `json:"skill_name"`    // Skill that will be created
	Insight     string `json:"insight"`       // What the user learns
}
