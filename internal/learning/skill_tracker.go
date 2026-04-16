package learning

import (
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
