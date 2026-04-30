package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

// ObservationConflict represents a thesis-antithesis pair of conflicting
// observations about the user. The dialectic engine resolves these by
// selecting the most recent and highest-confidence observation.
type ObservationConflict struct {
	Category             string
	Key                  string
	Thesis               string
	ThesisConfidence     float64
	ThesisObservedAt     time.Time
	Antithesis           string
	AntithesisConfidence float64
	AntithesisObservedAt time.Time
	Resolved             bool
	Resolution           string
}

// UserModelSnapshot is the synthesized result of the dialectic modeling
// engine. It contains resolved preferences, communication style,
// knowledge background, and common work patterns.
type UserModelSnapshot struct {
	Preferences         map[string]string
	CommunicationStyle  string
	KnowledgeBackground []string
	TopPatterns         []layers.WorkPattern
	Conflicts           []ObservationConflict
	LastUpdated         time.Time
}

// UserModelingEngine implements the dialectic user modeling approach.
// New observations form a thesis, conflicting observations form an
// antithesis, and the engine synthesizes both into a nuanced model.
type UserModelingEngine struct {
	store     *store.Store
	promptMem *layers.PromptMemory
	userID    string

	mu       sync.RWMutex
	snapshot *UserModelSnapshot
}

// NewUserModelingEngine creates a new dialectic modeling engine for the
// given user. The engine reads observations from the user_observations
// table and synthesizes them into a coherent user model.
func NewUserModelingEngine(s *store.Store, pm *layers.PromptMemory, userID string) *UserModelingEngine {
	if userID == "" {
		userID = "default"
	}
	return &UserModelingEngine{
		store:     s,
		promptMem: pm,
		userID:    userID,
	}
}

// observation is an internal representation of a row from user_observations.
type observation struct {
	ID         int64
	Category   string
	Key        string
	Value      string
	Confidence float64
	ObservedAt time.Time
	SessionID  string
}

// SynthesizeModel loads all observations from the user_observations table,
// identifies conflicting observations (thesis-antithesis pairs), resolves
// them, and builds a UserModelSnapshot.
func (e *UserModelingEngine) SynthesizeModel(ctx context.Context) (*UserModelSnapshot, error) {
	if e.store == nil {
		return nil, fmt.Errorf("user modeling engine: store not available")
	}

	observations, err := e.loadObservations(ctx)
	if err != nil {
		return nil, fmt.Errorf("user modeling engine: load observations: %w", err)
	}

	// Group observations by (category, key).
	grouped := make(map[string]map[string][]observation) // category -> key -> []observation
	for _, obs := range observations {
		if grouped[obs.Category] == nil {
			grouped[obs.Category] = make(map[string][]observation)
		}
		grouped[obs.Category][obs.Key] = append(grouped[obs.Category][obs.Key], obs)
	}

	// Find conflicts and resolve them.
	var conflicts []ObservationConflict
	preferences := make(map[string]string)
	var knowledgeBg []string
	var patterns []layers.WorkPattern
	commStyle := ""

	for category, keys := range grouped {
		for key, obsGroup := range keys {
			if len(obsGroup) == 0 {
				continue
			}

			// Find conflicting observations: same category+key but different values
			// with confidence > 0.5.
			conflicting := e.findConflicts(category, key, obsGroup)
			conflicts = append(conflicts, conflicting...)

			// Resolve: pick the most recent + highest confidence observation.
			resolved := e.resolveObservations(obsGroup)

			// Distribute resolved value into the appropriate snapshot field.
			switch category {
			case "preference", "code_style":
				preferences[key] = resolved.Value
			case "communication_style":
				commStyle = resolved.Value
			case "knowledge":
				knowledgeBg = append(knowledgeBg, resolved.Value)
			case "pattern", "workflow_pattern":
				patterns = append(patterns, layers.WorkPattern{
					Pattern:   resolved.Value,
					Frequency: len(obsGroup),
					LastSeen:  resolved.ObservedAt,
				})
			}
		}
	}

	// Sort patterns by frequency (descending).
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})

	// Limit to top 10 patterns.
	if len(patterns) > 10 {
		patterns = patterns[:10]
	}

	// Deduplicate knowledge.
	knowledgeBg = dedupStrings(knowledgeBg)

	snapshot := &UserModelSnapshot{
		Preferences:         preferences,
		CommunicationStyle:  commStyle,
		KnowledgeBackground: knowledgeBg,
		TopPatterns:         patterns,
		Conflicts:           conflicts,
		LastUpdated:         time.Now(),
	}

	e.mu.Lock()
	e.snapshot = snapshot
	e.mu.Unlock()

	slog.Debug("user modeling engine: synthesized model",
		"preferences", len(preferences),
		"knowledge", len(knowledgeBg),
		"patterns", len(patterns),
		"conflicts", len(conflicts),
	)

	return snapshot, nil
}

// GetSnapshot returns the cached snapshot from the last SynthesizeModel call.
func (e *UserModelingEngine) GetSnapshot() *UserModelSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.snapshot
}

// RecordObservation writes a new observation to the user_observations table.
func (e *UserModelingEngine) RecordObservation(ctx context.Context, category, key, value string, confidence float64, sessionID string) error {
	if e.store == nil {
		return nil
	}

	_, err := e.store.DB().ExecContext(ctx,
		`INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		category, key, value, confidence, time.Now(), sessionID, e.userID,
	)
	if err != nil {
		return fmt.Errorf("user modeling engine: record observation: %w", err)
	}
	return nil
}

// GetConflicts returns all unresolved thesis/antithesis pairs from the
// last synthesis.
func (e *UserModelingEngine) GetConflicts() []ObservationConflict {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.snapshot == nil {
		return nil
	}

	var unresolved []ObservationConflict
	for _, c := range e.snapshot.Conflicts {
		if !c.Resolved {
			unresolved = append(unresolved, c)
		}
	}
	return unresolved
}

// loadObservations fetches all observations for this user from the store.
func (e *UserModelingEngine) loadObservations(ctx context.Context) ([]observation, error) {
	var rows *sql.Rows
	var err error

	if e.userID != "default" {
		rows, err = e.store.DB().QueryContext(ctx, `
			SELECT id, category, key, value, confidence, observed_at, session_id
			FROM user_observations
			WHERE user_id = ?
			ORDER BY observed_at ASC
		`, e.userID)
	} else {
		rows, err = e.store.DB().QueryContext(ctx, `
			SELECT id, category, key, value, confidence, observed_at, session_id
			FROM user_observations
			ORDER BY observed_at ASC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []observation
	for rows.Next() {
		var obs observation
		var observedAtStr string
		if err := rows.Scan(&obs.ID, &obs.Category, &obs.Key, &obs.Value,
			&obs.Confidence, &observedAtStr, &obs.SessionID); err != nil {
			continue
		}
		parsed, parseErr := time.Parse(time.RFC3339, observedAtStr)
		if parseErr != nil {
			// Try SQLite default format.
			parsed, parseErr = time.Parse("2006-01-02 15:04:05", observedAtStr)
		}
		if parseErr != nil {
			parsed = time.Now()
		}
		obs.ObservedAt = parsed
		result = append(result, obs)
	}
	return result, rows.Err()
}

// findConflicts identifies conflicting observations: same category+key with
// different values and confidence > 0.5.
func (e *UserModelingEngine) findConflicts(category, key string, obsGroup []observation) []ObservationConflict {
	// Collect distinct high-confidence values.
	type valueEntry struct {
		value      string
		confidence float64
		observedAt time.Time
	}
	valueMap := make(map[string]valueEntry)
	for _, obs := range obsGroup {
		if obs.Confidence <= 0.5 {
			continue
		}
		existing, exists := valueMap[obs.Value]
		if !exists || obs.Confidence > existing.confidence ||
			(obs.Confidence == existing.confidence && obs.ObservedAt.After(existing.observedAt)) {
			valueMap[obs.Value] = valueEntry{
				value:      obs.Value,
				confidence: obs.Confidence,
				observedAt: obs.ObservedAt,
			}
		}
	}

	if len(valueMap) <= 1 {
		return nil
	}

	// Sort values by confidence descending, then by recency.
	var entries []valueEntry
	for _, ve := range valueMap {
		entries = append(entries, ve)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].confidence != entries[j].confidence {
			return entries[i].confidence > entries[j].confidence
		}
		return entries[i].observedAt.After(entries[j].observedAt)
	})

	// The top entry is the thesis; each subsequent distinct value is an antithesis.
	var conflicts []ObservationConflict
	thesis := entries[0]
	for i := 1; i < len(entries); i++ {
		antithesis := entries[i]
		conflict := ObservationConflict{
			Category:             category,
			Key:                  key,
			Thesis:               thesis.value,
			ThesisConfidence:     thesis.confidence,
			ThesisObservedAt:     thesis.observedAt,
			Antithesis:           antithesis.value,
			AntithesisConfidence: antithesis.confidence,
			AntithesisObservedAt: antithesis.observedAt,
			Resolved:             true,
			Resolution:           thesis.value,
		}
		conflicts = append(conflicts, conflict)
	}
	return conflicts
}

// resolveObservations picks the winning observation for a group by selecting
// the most recent observation with the highest confidence.
func (e *UserModelingEngine) resolveObservations(obsGroup []observation) observation {
	if len(obsGroup) == 0 {
		return observation{}
	}

	best := obsGroup[0]
	for _, obs := range obsGroup[1:] {
		// Prefer higher confidence; break ties with recency.
		if obs.Confidence > best.Confidence ||
			(obs.Confidence == best.Confidence && obs.ObservedAt.After(best.ObservedAt)) {
			best = obs
		}
	}
	return best
}

// dedupStrings removes duplicate strings while preserving order.
func dedupStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
