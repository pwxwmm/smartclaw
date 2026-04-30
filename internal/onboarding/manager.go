package onboarding

import (
	"database/sql"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type Manager struct {
	store *store.Store
}

func NewManager(s *store.Store) *Manager {
	return &Manager{store: s}
}

func (m *Manager) GetState(userID string) (*OnboardingState, error) {
	if m.store == nil {
		return &OnboardingState{UserID: userID, Step: 0}, nil
	}

	row := m.store.DB().QueryRow(
		`SELECT user_id, step, started_at, done_at FROM onboarding_states WHERE user_id = ?`,
		userID,
	)

	var state OnboardingState
	var doneAt sql.NullInt64
	if err := row.Scan(&state.UserID, &state.Step, &state.StartedAt, &doneAt); err != nil {
		if err == sql.ErrNoRows {
			return &OnboardingState{UserID: userID, Step: 0}, nil
		}
		return nil, err
	}
	if doneAt.Valid {
		state.DoneAt = doneAt.Int64
	}
	return &state, nil
}

func (m *Manager) StartOnboarding(userID string) (*OnboardingState, error) {
	if m.store == nil {
		return &OnboardingState{UserID: userID, Step: 1, StartedAt: time.Now().Unix()}, nil
	}

	now := time.Now().Unix()
	_, err := m.store.DB().Exec(
		`INSERT INTO onboarding_states (user_id, step, started_at) VALUES (?, 1, ?)
		 ON CONFLICT(user_id) DO UPDATE SET step = 1, started_at = ?, done_at = 0`,
		userID, now, now,
	)
	if err != nil {
		return nil, err
	}

	return &OnboardingState{UserID: userID, Step: 1, StartedAt: now}, nil
}

func (m *Manager) AdvanceStep(userID string, skillCreated string) (*OnboardingState, *OnboardingStep, error) {
	state, err := m.GetState(userID)
	if err != nil {
		return nil, nil, err
	}

	if state.Step == 0 || state.Step >= 4 {
		return state, nil, nil
	}

	nextStep := state.Step + 1

	if nextStep > 3 {
		now := time.Now().Unix()
		if m.store != nil {
			_, err = m.store.DB().Exec(
				`UPDATE onboarding_states SET step = 4, done_at = ? WHERE user_id = ?`,
				now, userID,
			)
			if err != nil {
				return nil, nil, err
			}
		}
		return &OnboardingState{UserID: userID, Step: 4, StartedAt: state.StartedAt, DoneAt: now}, nil, nil
	}

	if m.store != nil {
		_, err = m.store.DB().Exec(
			`UPDATE onboarding_states SET step = ? WHERE user_id = ?`,
			nextStep, userID,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	step := GetStep(nextStep)
	return &OnboardingState{UserID: userID, Step: nextStep, StartedAt: state.StartedAt}, step, nil
}

func (m *Manager) CompleteOnboarding(userID string) error {
	if m.store == nil {
		return nil
	}

	now := time.Now().Unix()
	_, err := m.store.DB().Exec(
		`UPDATE onboarding_states SET step = 4, done_at = ? WHERE user_id = ?`,
		now, userID,
	)
	return err
}
