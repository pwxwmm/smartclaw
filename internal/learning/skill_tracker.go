package learning

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type SkillOutcome string

const (
	OutcomeSuccess        SkillOutcome = "success"
	OutcomePartialSuccess SkillOutcome = "partial_success"
	OutcomeFailed         SkillOutcome = "failed"
	OutcomeUserOverride   SkillOutcome = "user_override"
)

const DecayThreshold = 0.3

type SkillHealth string

const (
	HealthHealthy   SkillHealth = "healthy"
	HealthDegraded  SkillHealth = "degraded"
	HealthFailing   SkillHealth = "failing"
	HealthUnused    SkillHealth = "unused"
)

type SkillTracker struct {
	store *store.Store
}

func NewSkillTracker(s *store.Store) *SkillTracker {
	return &SkillTracker{
		store: s,
	}
}

func (st *SkillTracker) RecordInvocation(skillID, sessionID string) error {
	if st.store == nil {
		return nil
	}

	_, err := st.store.DB().Exec(
		`INSERT INTO skill_invocations (skill_id, session_id, invoked_at) VALUES (?, ?, ?)`,
		skillID, sessionID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("skill tracker: record invocation: %w", err)
	}
	return nil
}

func (st *SkillTracker) RecordOutcome(skillID string, outcome SkillOutcome, sessionID string) error {
	if st.store == nil {
		return nil
	}

	_, err := st.store.DB().Exec(
		`INSERT INTO skill_outcomes (skill_id, session_id, outcome, recorded_at) VALUES (?, ?, ?, ?)`,
		skillID, sessionID, string(outcome), time.Now(),
	)
	if err != nil {
		return fmt.Errorf("skill tracker: record outcome: %w", err)
	}

	slog.Debug("skill tracker: recorded outcome", "skill", skillID, "outcome", string(outcome))
	return nil
}

func (st *SkillTracker) RecordOutcomeWithDetails(skillID string, outcome SkillOutcome, sessionID string, details string) error {
	if st.store == nil {
		return nil
	}

	_, err := st.store.DB().Exec(
		`INSERT INTO skill_outcomes (skill_id, session_id, outcome, details, recorded_at) VALUES (?, ?, ?, ?, ?)`,
		skillID, sessionID, string(outcome), details, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("skill tracker: record outcome with details: %w", err)
	}

	slog.Debug("skill tracker: recorded outcome", "skill", skillID, "outcome", string(outcome))
	return nil
}

func (st *SkillTracker) GetRecentFailures(ctx context.Context, skillID string, limit int) ([]string, error) {
	if st.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := st.store.DB().QueryContext(ctx,
		`SELECT COALESCE(details, 'failure in session ' || session_id) FROM skill_outcomes
		 WHERE skill_id = ? AND outcome = 'failed' ORDER BY recorded_at DESC LIMIT ?`,
		skillID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("skill tracker: get recent failures: %w", err)
	}
	defer rows.Close()
	var failures []string
	for rows.Next() {
		var detail string
		if err := rows.Scan(&detail); err != nil {
			continue
		}
		failures = append(failures, detail)
	}
	return failures, nil
}

type EffectivenessScore struct {
	SkillID          string
	TotalInvocations int
	Successes        int
	Failures         int
	UserOverrides    int
	Score            float64
}

func (st *SkillTracker) GetEffectivenessScore(skillID string) (EffectivenessScore, error) {
	score := EffectivenessScore{SkillID: skillID}

	if st.store == nil {
		score.Score = 0.5
		return score, nil
	}

	row := st.store.DB().QueryRow(
		`SELECT COUNT(*) FROM skill_invocations WHERE skill_id = ?`,
		skillID,
	)
	if err := row.Scan(&score.TotalInvocations); err != nil {
		return score, fmt.Errorf("skill tracker: count invocations: %w", err)
	}

	row = st.store.DB().QueryRow(
		`SELECT
			COALESCE(SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN outcome = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN outcome = 'user_override' THEN 1 ELSE 0 END), 0)
		FROM skill_outcomes WHERE skill_id = ?`,
		skillID,
	)
	if err := row.Scan(&score.Successes, &score.Failures, &score.UserOverrides); err != nil {
		return score, fmt.Errorf("skill tracker: count outcomes: %w", err)
	}

	if score.TotalInvocations == 0 {
		score.Score = 0.5
		return score, nil
	}

	totalOutcomes := score.Successes + score.Failures + score.UserOverrides
	if totalOutcomes == 0 {
		score.Score = 0.5
		return score, nil
	}

	weightedSuccess := float64(score.Successes) * 1.0
	weightedPartial := float64(score.UserOverrides) * 0.3

	score.Score = (weightedSuccess + weightedPartial) / float64(totalOutcomes)

	return score, nil
}

func (st *SkillTracker) GetDecayCandidates() ([]string, error) {
	if st.store == nil {
		return nil, nil
	}

	rows, err := st.store.DB().Query(
		`SELECT DISTINCT skill_id FROM skill_invocations`,
	)
	if err != nil {
		return nil, fmt.Errorf("skill tracker: list skills: %w", err)
	}
	defer rows.Close()

	var skillIDs []string
	for rows.Next() {
		var skillID string
		if err := rows.Scan(&skillID); err != nil {
			continue
		}
		skillIDs = append(skillIDs, skillID)
	}

	var candidates []string
	for _, skillID := range skillIDs {
		score, err := st.GetEffectivenessScore(skillID)
		if err != nil {
			slog.Warn("skill tracker: failed to get score", "skill", skillID, "error", err)
			continue
		}

		if score.Score < DecayThreshold && score.TotalInvocations >= 3 {
			candidates = append(candidates, skillID)
		}
	}

	return candidates, nil
}

func (st *SkillTracker) GetAllScores() (map[string]EffectivenessScore, error) {
	if st.store == nil {
		return nil, nil
	}

	rows, err := st.store.DB().Query(
		`SELECT DISTINCT skill_id FROM skill_invocations`,
	)
	if err != nil {
		return nil, fmt.Errorf("skill tracker: list skills: %w", err)
	}
	defer rows.Close()

	var skillIDs []string
	for rows.Next() {
		var skillID string
		if err := rows.Scan(&skillID); err != nil {
			continue
		}
		skillIDs = append(skillIDs, skillID)
	}

	scores := make(map[string]EffectivenessScore)
	for _, skillID := range skillIDs {
		score, err := st.GetEffectivenessScore(skillID)
		if err != nil {
			continue
		}
		scores[skillID] = score
	}

	return scores, nil
}

type SkillHealthEntry struct {
	SkillID          string
	SuccessRate      float64
	TotalInvocations int
	Trend            string
	LastUsed         *time.Time
	Health           SkillHealth
	Recommendation   string
}

type SkillHealthReport struct {
	Skills      []SkillHealthEntry
	GeneratedAt time.Time
	Healthy     int
	Degraded    int
	Failing     int
	Unused      int
}

type TrendingSkill struct {
	SkillID    string
	UsageCount int
	LastUsed   *time.Time
}

func (st *SkillTracker) GetHealthReport() (*SkillHealthReport, error) {
	if st.store == nil {
		return &SkillHealthReport{GeneratedAt: time.Now()}, nil
	}

	rows, err := st.store.DB().Query(
		`SELECT DISTINCT skill_id FROM skill_invocations`,
	)
	if err != nil {
		return nil, fmt.Errorf("skill tracker: list skills for health: %w", err)
	}
	defer rows.Close()

	var skillIDs []string
	for rows.Next() {
		var skillID string
		if err := rows.Scan(&skillID); err != nil {
			continue
		}
		skillIDs = append(skillIDs, skillID)
	}

	report := &SkillHealthReport{
		GeneratedAt: time.Now(),
	}

	for _, skillID := range skillIDs {
		entry := st.buildHealthEntry(skillID)
		report.Skills = append(report.Skills, entry)

		switch entry.Health {
		case HealthHealthy:
			report.Healthy++
		case HealthDegraded:
			report.Degraded++
		case HealthFailing:
			report.Failing++
		case HealthUnused:
			report.Unused++
		}
	}

	return report, nil
}

func (st *SkillTracker) buildHealthEntry(skillID string) SkillHealthEntry {
	entry := SkillHealthEntry{SkillID: skillID}

	score, err := st.GetEffectivenessScore(skillID)
	if err != nil {
		entry.Health = HealthUnused
		entry.Recommendation = "No data available"
		return entry
	}

	entry.SuccessRate = score.Score
	entry.TotalInvocations = score.TotalInvocations

	var lastUsed *time.Time
	row := st.store.DB().QueryRow(
		`SELECT MAX(invoked_at) FROM skill_invocations WHERE skill_id = ?`,
		skillID,
	)
	var lastUsedStr string
	if err := row.Scan(&lastUsedStr); err == nil {
		if t, err := time.Parse("2006-01-02 15:04:05", lastUsedStr); err == nil {
			lastUsed = &t
		}
	}
	entry.LastUsed = lastUsed

	entry.Trend = st.calculateTrend(skillID)

	if score.TotalInvocations < 2 {
		entry.Health = HealthUnused
		entry.Recommendation = "Insufficient data — use this skill more to assess health"
	} else if score.Score >= 0.7 {
		entry.Health = HealthHealthy
		entry.Recommendation = "Performing well — no action needed"
	} else if score.Score >= 0.4 {
		entry.Health = HealthDegraded
		entry.Recommendation = "Success rate declining — consider improving this skill"
	} else {
		entry.Health = HealthFailing
		entry.Recommendation = "Frequently failing — improve or retire this skill"
	}

	return entry
}

func (st *SkillTracker) calculateTrend(skillID string) string {
	if st.store == nil {
		return "stable"
	}

	row := st.store.DB().QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END), 0),
		        COALESCE(COUNT(*), 0)
		FROM (
			SELECT outcome FROM skill_outcomes
			WHERE skill_id = ?
			ORDER BY recorded_at DESC
			LIMIT 5
		)`,
		skillID,
	)
	var recentSuccesses, recentTotal int
	if err := row.Scan(&recentSuccesses, &recentTotal); err != nil || recentTotal == 0 {
		return "stable"
	}
	recentRate := float64(recentSuccesses) / float64(recentTotal)

	row = st.store.DB().QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END), 0),
		        COALESCE(COUNT(*), 0)
		FROM (
			SELECT outcome FROM skill_outcomes
			WHERE skill_id = ?
			ORDER BY recorded_at ASC
			LIMIT 5
		)`,
		skillID,
	)
	var earlySuccesses, earlyTotal int
	if err := row.Scan(&earlySuccesses, &earlyTotal); err != nil || earlyTotal == 0 {
		return "stable"
	}
	earlyRate := float64(earlySuccesses) / float64(earlyTotal)

	diff := recentRate - earlyRate
	switch {
	case diff > 0.15:
		return "improving"
	case diff < -0.15:
		return "declining"
	default:
		return "stable"
	}
}

func (st *SkillTracker) GetTrending(limit int) ([]TrendingSkill, error) {
	if st.store == nil {
		return nil, nil
	}

	if limit <= 0 {
		limit = 10
	}

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	rows, err := st.store.DB().Query(
		`SELECT skill_id, COUNT(*) as usage_count, MAX(invoked_at) as last_used
		FROM skill_invocations
		WHERE invoked_at > ?
		GROUP BY skill_id
		ORDER BY usage_count DESC
		LIMIT ?`,
		cutoff, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("skill tracker: get trending: %w", err)
	}
	defer rows.Close()

	var trending []TrendingSkill
	for rows.Next() {
		var ts TrendingSkill
		var lastUsedStr string
		if err := rows.Scan(&ts.SkillID, &ts.UsageCount, &lastUsedStr); err != nil {
			continue
		}
		if t, err := time.Parse("2006-01-02 15:04:05", lastUsedStr); err == nil {
			ts.LastUsed = &t
		}
		trending = append(trending, ts)
	}

	return trending, nil
}

func (st *SkillTracker) DetectFailingSkills() ([]string, error) {
	if st.store == nil {
		return nil, nil
	}

	rows, err := st.store.DB().Query(
		`SELECT DISTINCT skill_id FROM skill_invocations`,
	)
	if err != nil {
		return nil, fmt.Errorf("skill tracker: list skills for failure detection: %w", err)
	}
	defer rows.Close()

	var failing []string
	for rows.Next() {
		var skillID string
		if err := rows.Scan(&skillID); err != nil {
			continue
		}

		recent, err := st.getRecentSuccessRate(skillID, improvementWindowInvocations)
		if err != nil {
			continue
		}
		if recent.InvocationCount >= 3 && recent.SuccessRate < improvementThreshold {
			failing = append(failing, skillID)
		}
	}

	return failing, nil
}
