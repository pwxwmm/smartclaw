package layers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type ReasoningLevel string

const (
	ReasoningQuick    ReasoningLevel = "quick"    // single-pass analysis
	ReasoningStandard ReasoningLevel = "standard" // analyze then synthesize
	ReasoningDeep     ReasoningLevel = "deep"     // analyze, critique, synthesize
)

type DialecticConfig struct {
	Cadence         int            // trigger every N user messages (default: 10)
	ReasoningLevel  ReasoningLevel // depth of reasoning (default: "standard")
	MaxObservations int            // max observations per cycle (default: 50)
	AutoUpdate      bool           // auto-update USER.md after reasoning (default: true)
}

func DefaultDialecticConfig() DialecticConfig {
	return DialecticConfig{
		Cadence:         10,
		ReasoningLevel:  ReasoningStandard,
		MaxObservations: 50,
		AutoUpdate:      true,
	}
}

type DialecticRound struct {
	RoundNumber int
	Phase       string    // "analyze", "critique", "synthesize"
	Input       string
	Reasoning   string
	Insights    []string
	Timestamp   time.Time
}

type DialecticLLMFunc func(ctx context.Context, prompt string, maxTokens int) (string, error)

type DialecticUserModel struct {
	config       DialecticConfig
	store        *store.Store
	promptMem    *PromptMemory
	messageCount int
	rounds       []DialecticRound
	mu           sync.Mutex
	userID       string
	llmFunc      DialecticLLMFunc
}

func NewDialecticUserModel(s *store.Store, pm *PromptMemory, config DialecticConfig, llmFunc DialecticLLMFunc) *DialecticUserModel {
	if config.Cadence <= 0 {
		config.Cadence = 10
	}
	if config.ReasoningLevel == "" {
		config.ReasoningLevel = ReasoningStandard
	}
	if config.MaxObservations <= 0 {
		config.MaxObservations = 50
	}
	return &DialecticUserModel{
		config:    config,
		store:     s,
		promptMem: pm,
		rounds:    make([]DialecticRound, 0),
		llmFunc:   llmFunc,
	}
}

func (d *DialecticUserModel) SetUserID(userID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.userID = userID
}

func (d *DialecticUserModel) ProcessMessage(ctx context.Context, role, content string) error {
	if role != "user" {
		return nil
	}

	d.mu.Lock()
	d.messageCount++
	shouldTrigger := d.messageCount >= d.config.Cadence
	d.mu.Unlock()

	if shouldTrigger {
		return d.RunDialecticCycle(ctx)
	}
	return nil
}

func (d *DialecticUserModel) ShouldTrigger() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.messageCount >= d.config.Cadence
}

func (d *DialecticUserModel) GetInsights() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var all []string
	for _, r := range d.rounds {
		all = append(all, r.Insights...)
	}
	return all
}

func (d *DialecticUserModel) GetRounds() []DialecticRound {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make([]DialecticRound, len(d.rounds))
	copy(result, d.rounds)
	return result
}

func (d *DialecticUserModel) ResetCadence() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.messageCount = 0
}

func (d *DialecticUserModel) GetMessageCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.messageCount
}

func (d *DialecticUserModel) RunDialecticCycle(ctx context.Context) error {
	if d.llmFunc == nil {
		return fmt.Errorf("dialectic: LLM function not configured")
	}

	observations, err := d.gatherObservations(ctx)
	if err != nil {
		return fmt.Errorf("dialectic: gather observations: %w", err)
	}

	if len(observations) == 0 {
		slog.Debug("dialectic: no observations to analyze, skipping cycle")
		d.ResetCadence()
		return nil
	}

	existingProfile := ""
	if d.promptMem != nil {
		existingProfile = d.promptMem.GetUserContent()
	}

	roundNumber := len(d.rounds) + 1

	// Phase 1: Analyze (all levels)
	analyzeInput := d.buildAnalyzePrompt(observations, existingProfile)
	analyzeReasoning, err := d.llmFunc(ctx, analyzeInput, 1024)
	if err != nil {
		return fmt.Errorf("dialectic: analyze phase: %w", err)
	}
	analyzeInsights := extractInsights(analyzeReasoning)

	analyzeRound := DialecticRound{
		RoundNumber: roundNumber,
		Phase:       "analyze",
		Input:       truncateForStorage(analyzeInput, 2000),
		Reasoning:   analyzeReasoning,
		Insights:    analyzeInsights,
		Timestamp:   time.Now(),
	}
	d.addRound(analyzeRound)

	// Phase 2: Critique (deep only)
	var critiqueReasoning string
	if d.config.ReasoningLevel == ReasoningDeep {
		roundNumber++
		critiqueInput := d.buildCritiquePrompt(analyzeReasoning, observations)
		critiqueReasoning, err = d.llmFunc(ctx, critiqueInput, 1024)
		if err != nil {
			return fmt.Errorf("dialectic: critique phase: %w", err)
		}
		critiqueInsights := extractInsights(critiqueReasoning)

		critiqueRound := DialecticRound{
			RoundNumber: roundNumber,
			Phase:       "critique",
			Input:       truncateForStorage(critiqueInput, 2000),
			Reasoning:   critiqueReasoning,
			Insights:    critiqueInsights,
			Timestamp:   time.Now(),
		}
		d.addRound(critiqueRound)
	}

	// Phase 3: Synthesize (standard and deep)
	var finalInsights []string
	if d.config.ReasoningLevel == ReasoningStandard || d.config.ReasoningLevel == ReasoningDeep {
		roundNumber++
		synthesizeInput := d.buildSynthesizePrompt(analyzeReasoning, critiqueReasoning, existingProfile)
		synthesizeReasoning, err := d.llmFunc(ctx, synthesizeInput, 1024)
		if err != nil {
			return fmt.Errorf("dialectic: synthesize phase: %w", err)
		}
		finalInsights = extractInsights(synthesizeReasoning)

		synthesizeRound := DialecticRound{
			RoundNumber: roundNumber,
			Phase:       "synthesize",
			Input:       truncateForStorage(synthesizeInput, 2000),
			Reasoning:   synthesizeReasoning,
			Insights:    finalInsights,
			Timestamp:   time.Now(),
		}
		d.addRound(synthesizeRound)
	}

	if d.config.AutoUpdate && d.promptMem != nil && len(finalInsights) > 0 {
		if err := d.updateUserProfile(ctx, finalInsights); err != nil {
			slog.Warn("dialectic: failed to auto-update USER.md", "error", err)
		}
	}

	d.ResetCadence()

	slog.Info("dialectic: cycle completed",
		"level", d.config.ReasoningLevel,
		"observations", len(observations),
		"insights", len(finalInsights),
	)
	return nil
}

func (d *DialecticUserModel) gatherObservations(ctx context.Context) ([]string, error) {
	if d.store == nil {
		return nil, nil
	}

	userID := d.userID
	if userID == "" {
		userID = "default"
	}

	var rows *sql.Rows
	var err error

	if userID != "default" {
		rows, err = d.store.DB().QueryContext(ctx, `
			SELECT category, key, value, confidence, observed_at
			FROM user_observations
			WHERE user_id = ?
			ORDER BY observed_at DESC
			LIMIT ?
		`, userID, d.config.MaxObservations)
	} else {
		rows, err = d.store.DB().QueryContext(ctx, `
			SELECT category, key, value, confidence, observed_at
			FROM user_observations
			ORDER BY observed_at DESC
			LIMIT ?
		`, d.config.MaxObservations)
	}
	if err != nil {
		return nil, fmt.Errorf("query observations: %w", err)
	}
	defer rows.Close()

	var observations []string
	for rows.Next() {
		var category, key, value string
		var confidence float64
		var observedAt string
		if err := rows.Scan(&category, &key, &value, &confidence, &observedAt); err != nil {
			continue
		}
		obs := fmt.Sprintf("[%s] %s=%s (confidence: %.2f, at: %s)", category, key, value, confidence, observedAt)
		observations = append(observations, obs)
	}
	return observations, nil
}

func (d *DialecticUserModel) buildAnalyzePrompt(observations []string, existingProfile string) string {
	var sb strings.Builder

	sb.WriteString("You are analyzing user behavior observations to build a deeper understanding of this user.\n\n")

	if existingProfile != "" {
		sb.WriteString("## Existing User Profile\n")
		sb.WriteString(existingProfile)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Recent Observations\n")
	for _, obs := range observations {
		sb.WriteString("- ")
		sb.WriteString(obs)
		sb.WriteString("\n")
	}

	sb.WriteString("\n## Task\n")
	sb.WriteString("Based on these observations, what can we infer about this user? Please identify:\n")
	sb.WriteString("1. **Preferences**: What does the user prefer or dislike?\n")
	sb.WriteString("2. **Communication Style**: How does the user communicate? (formal, casual, technical, brief, verbose)\n")
	sb.WriteString("3. **Knowledge Gaps**: What areas might the user need more help with?\n")
	sb.WriteString("4. **Work Patterns**: What recurring patterns or workflows does the user follow?\n\n")
	sb.WriteString("Provide each insight as a separate point starting with '- '.\n")

	return sb.String()
}

func (d *DialecticUserModel) buildCritiquePrompt(analysis string, observations []string) string {
	var sb strings.Builder

	sb.WriteString("You are critically reviewing an analysis of user behavior.\n\n")

	sb.WriteString("## Original Analysis\n")
	sb.WriteString(analysis)
	sb.WriteString("\n\n")

	sb.WriteString("## Source Observations (sample)\n")
	maxObs := 20
	if len(observations) > maxObs {
		observations = observations[:maxObs]
	}
	for _, obs := range observations {
		sb.WriteString("- ")
		sb.WriteString(obs)
		sb.WriteString("\n")
	}

	sb.WriteString("\n## Task\n")
	sb.WriteString("What might be wrong or missing in this analysis? Consider:\n")
	sb.WriteString("1. Are there biases or assumptions not supported by the data?\n")
	sb.WriteString("2. Are there patterns in the observations that were overlooked?\n")
	sb.WriteString("3. Could any insights be misinterpretations?\n")
	sb.WriteString("4. What additional context would improve the analysis?\n\n")
	sb.WriteString("Provide each critique point as a separate point starting with '- '.\n")

	return sb.String()
}

func (d *DialecticUserModel) buildSynthesizePrompt(analysis, critique, existingProfile string) string {
	var sb strings.Builder

	sb.WriteString("You are synthesizing an analysis and its critique into final insights about a user.\n\n")

	sb.WriteString("## Analysis\n")
	sb.WriteString(analysis)
	sb.WriteString("\n\n")

	sb.WriteString("## Critique\n")
	sb.WriteString(critique)
	sb.WriteString("\n\n")

	if existingProfile != "" {
		sb.WriteString("## Existing User Profile\n")
		sb.WriteString(existingProfile)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Task\n")
	sb.WriteString("Merge the analysis with the critique into a refined set of insights. ")
	sb.WriteString("Keep insights that survive critique, modify those that were challenged, and add any missing observations. ")
	sb.WriteString("Focus on actionable, specific insights rather than vague generalizations.\n\n")
	sb.WriteString("Provide each final insight as a separate point starting with '- '.\n")

	return sb.String()
}

func (d *DialecticUserModel) updateUserProfile(ctx context.Context, insights []string) error {
	if d.promptMem == nil {
		return nil
	}

	existing := d.promptMem.GetUserContent()

	var sb strings.Builder
	sb.WriteString("# User Profile\n\n")

	if existing != "" {
		sb.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Dialectic Insights\n")
	for _, insight := range insights {
		sb.WriteString("- ")
		sb.WriteString(insight)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	content := sb.String()
	if len(content) > MaxPromptMemoryChars/2 {
		content = content[:MaxPromptMemoryChars/2]
	}

	if err := d.promptMem.UpdateUserProfile(content); err != nil {
		return fmt.Errorf("dialectic: update user profile: %w", err)
	}
	return nil
}

func (d *DialecticUserModel) addRound(round DialecticRound) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rounds = append(d.rounds, round)
}

func extractInsights(text string) []string {
	var insights []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			insight := strings.TrimSpace(trimmed[2:])
			if insight != "" {
				insights = append(insights, insight)
			}
		}
	}
	if len(insights) == 0 && strings.TrimSpace(text) != "" {
		insights = append(insights, strings.TrimSpace(text))
	}
	return insights
}

func truncateForStorage(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
