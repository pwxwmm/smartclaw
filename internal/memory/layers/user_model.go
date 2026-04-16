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

type UserModel struct {
	Preferences        map[string]string
	CommunicationStyle string
	KnowledgeBg        []string
	CommonPatterns     []WorkPattern
	LastUpdated        time.Time
}

type WorkPattern struct {
	Pattern   string
	Frequency int
	LastSeen  time.Time
}

type UserModelingLayer struct {
	model     *UserModel
	store     *store.Store
	promptMem *PromptMemory
	mu        sync.RWMutex
	userID    string
}

func NewUserModelingLayer(s *store.Store, pm *PromptMemory) *UserModelingLayer {
	return &UserModelingLayer{
		model: &UserModel{
			Preferences: make(map[string]string),
		},
		store:     s,
		promptMem: pm,
	}
}

func NewUserModelingLayerForUser(s *store.Store, pm *PromptMemory, userID string) *UserModelingLayer {
	return &UserModelingLayer{
		model: &UserModel{
			Preferences: make(map[string]string),
		},
		store:     s,
		promptMem: pm,
		userID:    userID,
	}
}

func (uml *UserModelingLayer) UserID() string {
	return uml.userID
}

func (uml *UserModelingLayer) TrackPassive(ctx context.Context, messages []Observation) error {
	if uml.store == nil {
		return nil
	}

	userID := uml.userID
	if userID == "" {
		userID = "default"
	}

	for _, obs := range messages {
		if _, err := uml.store.DB().Exec(
			`INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			obs.Category, obs.Key, obs.Value, obs.Confidence, time.Now(), obs.SessionID, userID,
		); err != nil {
			slog.Warn("user modeling: failed to record observation", "error", err)
		}
	}

	return nil
}

func (uml *UserModelingLayer) UpdateProfile(ctx context.Context) error {
	if uml.store == nil || uml.promptMem == nil {
		return nil
	}

	uml.mu.Lock()
	defer uml.mu.Unlock()

	var rows *sql.Rows
	var err error

	if uml.userID != "" && uml.userID != "default" {
		rows, err = uml.store.DB().Query(`
			SELECT category, key, value, COUNT(*) as freq
			FROM user_observations
			WHERE user_id = ?
			GROUP BY category, key
			ORDER BY freq DESC
			LIMIT 50
		`, uml.userID)
	} else {
		rows, err = uml.store.DB().Query(`
			SELECT category, key, value, COUNT(*) as freq
			FROM user_observations
			GROUP BY category, key
			ORDER BY freq DESC
			LIMIT 50
		`)
	}
	if err != nil {
		return fmt.Errorf("user modeling: query: %w", err)
	}
	defer rows.Close()

	preferences := make(map[string]string)
	var knowledge []string
	var patterns []string

	for rows.Next() {
		var category, key, value string
		var freq int
		if err := rows.Scan(&category, &key, &value, &freq); err != nil {
			continue
		}

		switch category {
		case "preference":
			preferences[key] = value
		case "knowledge":
			knowledge = append(knowledge, value)
		case "pattern":
			patterns = append(patterns, value)
		}
	}

	var sb strings.Builder
	sb.WriteString("# User Profile\n\n")

	if len(preferences) > 0 {
		sb.WriteString("## Preferences\n")
		for k, v := range preferences {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
		}
		sb.WriteString("\n")
	}

	if len(knowledge) > 0 {
		sb.WriteString("## Knowledge Background\n")
		for _, k := range knowledge {
			sb.WriteString(fmt.Sprintf("- %s\n", k))
		}
		sb.WriteString("\n")
	}

	if len(patterns) > 0 {
		sb.WriteString("## Common Patterns\n")
		for _, p := range patterns {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	content := sb.String()
	if len(content) > MaxPromptMemoryChars/2 {
		content = content[:MaxPromptMemoryChars/2]
	}

	if err := uml.promptMem.UpdateUserProfile(content); err != nil {
		return fmt.Errorf("user modeling: update profile: %w", err)
	}

	uml.model.Preferences = preferences
	uml.model.KnowledgeBg = knowledge
	uml.model.LastUpdated = time.Now()

	slog.Info("user modeling: profile updated", "preferences", len(preferences), "knowledge", len(knowledge))
	return nil
}

func (uml *UserModelingLayer) GetModel() *UserModel {
	uml.mu.RLock()
	defer uml.mu.RUnlock()
	return uml.model
}

func (uml *UserModelingLayer) BuildStaticBlock() string {
	uml.mu.RLock()
	defer uml.mu.RUnlock()

	if uml.model == nil {
		return ""
	}

	hasContent := len(uml.model.Preferences) > 0 ||
		uml.model.CommunicationStyle != "" ||
		len(uml.model.KnowledgeBg) > 0 ||
		len(uml.model.CommonPatterns) > 0

	if !hasContent {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## User Model\n")

	if len(uml.model.Preferences) > 0 {
		sb.WriteString("Preferences:\n")
		for k, v := range uml.model.Preferences {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
		}
	}

	if uml.model.CommunicationStyle != "" {
		sb.WriteString(fmt.Sprintf("Communication: %s\n", uml.model.CommunicationStyle))
	}

	if len(uml.model.KnowledgeBg) > 0 {
		sb.WriteString("Knowledge:\n")
		for _, k := range uml.model.KnowledgeBg {
			sb.WriteString(fmt.Sprintf("- %s\n", k))
		}
	}

	if len(uml.model.CommonPatterns) > 0 {
		sb.WriteString("Patterns:\n")
		for _, p := range uml.model.CommonPatterns {
			sb.WriteString(fmt.Sprintf("- %s (freq: %d)\n", p.Pattern, p.Frequency))
		}
	}

	return sb.String()
}

type Observation struct {
	Category   string
	Key        string
	Value      string
	Confidence float64
	SessionID  string
}

func ExtractObservations(role, content string) []Observation {
	var observations []Observation

	if role != "user" {
		return observations
	}

	if len(content) > 200 {
		content = content[:200]
	}

	lower := strings.ToLower(content)

	if strings.Contains(lower, "always") || strings.Contains(lower, "prefer") || strings.Contains(lower, "never") {
		observations = append(observations, Observation{
			Category:   "preference",
			Key:        "communication",
			Value:      content,
			Confidence: 0.7,
		})
	}

	langPatterns := map[string]string{
		"go":         "go",
		"python":     "python",
		"rust":       "rust",
		"java":       "java",
		"typescript": "typescript",
	}
	for lang, label := range langPatterns {
		if strings.Contains(lower, lang) {
			observations = append(observations, Observation{
				Category:   "knowledge",
				Key:        "language",
				Value:      label,
				Confidence: 0.6,
			})
			break
		}
	}

	return observations
}
