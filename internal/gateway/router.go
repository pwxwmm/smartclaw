package gateway

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type SessionRouter struct {
	store *store.Store
}

func NewSessionRouter(s *store.Store) *SessionRouter {
	return &SessionRouter{store: s}
}

type RoutedSession struct {
	ID     string
	UserID string
	Source string
}

func (sr *SessionRouter) Route(userID string) *RoutedSession {
	if sr.store != nil {
		sessions, err := sr.store.ListSessions(userID, 1)
		if err == nil && len(sessions) > 0 {
			latest := sessions[0]
			if time.Since(latest.UpdatedAt) < 30*time.Minute {
				return &RoutedSession{
					ID:     latest.ID,
					UserID: latest.UserID,
					Source: latest.Source,
				}
			}
		}
	}

	sessionID := generateSessionID(userID)

	if sr.store != nil {
		session := &store.Session{
			ID:        sessionID,
			UserID:    userID,
			Source:    "gateway",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := sr.store.UpsertSession(session); err != nil {
			slog.Warn("router: failed to persist session", "error", err)
		}
	}

	return &RoutedSession{
		ID:     sessionID,
		UserID: userID,
		Source: "gateway",
	}
}

func generateSessionID(userID string) string {
	return fmt.Sprintf("sess_%s_%s", userID, time.Now().Format("20060102_150405"))
}
