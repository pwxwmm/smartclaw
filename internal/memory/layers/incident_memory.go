package layers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

// Incident represents an active or resolved operational incident.
type Incident struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	Severity         string          `json:"severity"`
	Status           string          `json:"status"`
	Service          string          `json:"service"`
	Description      string          `json:"description"`
	RootCause        string          `json:"root_cause,omitempty"`
	Remediation      string          `json:"remediation,omitempty"`
	StartedAt        time.Time       `json:"started_at"`
	MitigatedAt      *time.Time      `json:"mitigated_at,omitempty"`
	ResolvedAt       *time.Time      `json:"resolved_at,omitempty"`
	AlertSource      string          `json:"alert_source,omitempty"`
	AffectedServices []string        `json:"affected_services,omitempty"`
	TimelineEvents   []TimelineEvent `json:"timeline_events,omitempty"`
}

// TimelineEvent represents a single event in an incident's timeline.
type TimelineEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
}

// SLOStatus represents the current SLO status for a service.
type SLOStatus struct {
	Service              string  `json:"service"`
	SLOName              string  `json:"slo_name"`
	Target               float64 `json:"target"`
	Current              float64 `json:"current"`
	ErrorBudgetRemaining float64 `json:"error_budget_remaining"`
	BurnRate             float64 `json:"burn_rate"`
	Status               string  `json:"status"`
}

// Postmortem represents a post-incident review document.
type Postmortem struct {
	ID             string    `json:"id"`
	IncidentID     string    `json:"incident_id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	RootCause      string    `json:"root_cause"`
	Contributing   []string  `json:"contributing_factors"`
	ActionItems    []string  `json:"action_items"`
	LessonsLearned []string  `json:"lessons_learned"`
	CreatedAt      time.Time `json:"created_at"`
}

const incidentSchemaSQL = `
CREATE TABLE IF NOT EXISTS incidents (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    severity          TEXT NOT NULL DEFAULT 'medium',
    status            TEXT NOT NULL DEFAULT 'active',
    service           TEXT NOT NULL DEFAULT '',
    description       TEXT DEFAULT '',
    root_cause        TEXT DEFAULT '',
    remediation       TEXT DEFAULT '',
    started_at        DATETIME NOT NULL,
    mitigated_at      DATETIME,
    resolved_at       DATETIME,
    alert_source      TEXT DEFAULT '',
    affected_services TEXT DEFAULT '[]',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS incident_timeline (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    timestamp   DATETIME NOT NULL,
    type        TEXT NOT NULL,
    content     TEXT NOT NULL,
    source      TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS slo_statuses (
    service               TEXT NOT NULL,
    slo_name              TEXT NOT NULL,
    target                REAL NOT NULL DEFAULT 0.0,
    current               REAL NOT NULL DEFAULT 0.0,
    error_budget_remaining REAL NOT NULL DEFAULT 0.0,
    burn_rate             REAL NOT NULL DEFAULT 0.0,
    status                TEXT NOT NULL DEFAULT 'healthy',
    updated_at            DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (service, slo_name)
);

CREATE TABLE IF NOT EXISTS postmortems (
    id                  TEXT PRIMARY KEY,
    incident_id         TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    summary             TEXT DEFAULT '',
    root_cause          TEXT DEFAULT '',
    contributing        TEXT DEFAULT '[]',
    action_items        TEXT DEFAULT '[]',
    lessons_learned     TEXT DEFAULT '[]',
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_service ON incidents(service);
CREATE INDEX IF NOT EXISTS idx_incidents_started_at ON incidents(started_at);
CREATE INDEX IF NOT EXISTS idx_incident_timeline_incident ON incident_timeline(incident_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_postmortems_incident ON postmortems(incident_id);
CREATE INDEX IF NOT EXISTS idx_slo_statuses_service ON slo_statuses(service);
`

const maxIncidentPromptChars = 2000

// IncidentMemory implements L5: real-time SRE context for incident response.
type IncidentMemory struct {
	store *store.Store
	mu    sync.RWMutex
}

// NewIncidentMemory creates a new incident memory layer backed by SQLite.
func NewIncidentMemory(s *store.Store) *IncidentMemory {
	im := &IncidentMemory{store: s}
	im.initSchema()
	return im
}

func (im *IncidentMemory) initSchema() {
	if im.store == nil {
		return
	}
	db := im.store.DB()
	if db == nil {
		return
	}
	if _, err := db.Exec(incidentSchemaSQL); err != nil {
		slog.Warn("incident memory: schema init failed", "error", err)
	}
}

// CreateIncident inserts a new incident into the database.
func (im *IncidentMemory) CreateIncident(ctx context.Context, incident *Incident) error {
	if im.store == nil {
		return nil
	}

	affectedJSON, _ := json.Marshal(incident.AffectedServices)
	if incident.StartedAt.IsZero() {
		incident.StartedAt = time.Now().UTC()
	}

	return im.store.WriteWithRetry(ctx,
		`INSERT INTO incidents (id, title, severity, status, service, description, root_cause, remediation, started_at, mitigated_at, resolved_at, alert_source, affected_services)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		incident.ID, incident.Title, incident.Severity, incident.Status, incident.Service,
		incident.Description, incident.RootCause, incident.Remediation,
		incident.StartedAt, incident.MitigatedAt, incident.ResolvedAt,
		incident.AlertSource, string(affectedJSON),
	)
}

// UpdateIncident updates specific fields of an incident.
func (im *IncidentMemory) UpdateIncident(ctx context.Context, id string, updates map[string]any) error {
	if im.store == nil || len(updates) == 0 {
		return nil
	}

	allowedCols := map[string]bool{
		"title": true, "severity": true, "status": true, "service": true,
		"description": true, "root_cause": true, "remediation": true,
		"mitigated_at": true, "resolved_at": true, "alert_source": true,
		"affected_services": true,
	}

	var setClauses []string
	var args []any
	for col, val := range updates {
		if !allowedCols[col] {
			continue
		}
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, id)

	query := "UPDATE incidents SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	return im.store.WriteWithRetry(ctx, query, args...)
}

// GetIncident retrieves a single incident by ID, including its timeline events.
func (im *IncidentMemory) GetIncident(id string) (*Incident, error) {
	if im.store == nil {
		return nil, nil
	}

	db := im.store.DB()
	row := db.QueryRow(
		`SELECT id, title, severity, status, service, description, root_cause, remediation,
		        started_at, mitigated_at, resolved_at, alert_source, affected_services
		 FROM incidents WHERE id = ?`, id,
	)

	var inc Incident
	var affectedJSON string
	var mitigatedAt, resolvedAt sql.NullTime

	if err := row.Scan(
		&inc.ID, &inc.Title, &inc.Severity, &inc.Status, &inc.Service,
		&inc.Description, &inc.RootCause, &inc.Remediation,
		&inc.StartedAt, &mitigatedAt, &resolvedAt, &inc.AlertSource, &affectedJSON,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("incident memory: get incident: %w", err)
	}

	if mitigatedAt.Valid {
		inc.MitigatedAt = &mitigatedAt.Time
	}
	if resolvedAt.Valid {
		inc.ResolvedAt = &resolvedAt.Time
	}
	if err := json.Unmarshal([]byte(affectedJSON), &inc.AffectedServices); err != nil {
		slog.Warn("failed to unmarshal affected services", "error", err)
	}

	// Load timeline events
	events, err := im.loadTimelineEvents(db, id)
	if err == nil {
		inc.TimelineEvents = events
	}

	return &inc, nil
}

// ListActiveIncidents returns all incidents that are not resolved.
func (im *IncidentMemory) ListActiveIncidents() ([]*Incident, error) {
	if im.store == nil {
		return nil, nil
	}

	db := im.store.DB()
	rows, err := db.Query(
		`SELECT id, title, severity, status, service, description, root_cause, remediation,
		        started_at, mitigated_at, resolved_at, alert_source, affected_services
		 FROM incidents WHERE status != 'resolved'
		 ORDER BY CASE severity
		     WHEN 'critical' THEN 0
		     WHEN 'high' THEN 1
		     WHEN 'medium' THEN 2
		     WHEN 'low' THEN 3
		     ELSE 4
		 END, started_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("incident memory: list active: %w", err)
	}
	defer rows.Close()

	return im.scanIncidents(rows)
}

// ListIncidentsByService returns all incidents for a given service.
func (im *IncidentMemory) ListIncidentsByService(service string) ([]*Incident, error) {
	if im.store == nil {
		return nil, nil
	}

	db := im.store.DB()
	rows, err := db.Query(
		`SELECT id, title, severity, status, service, description, root_cause, remediation,
		        started_at, mitigated_at, resolved_at, alert_source, affected_services
		 FROM incidents WHERE service = ?
		 ORDER BY started_at DESC`, service,
	)
	if err != nil {
		return nil, fmt.Errorf("incident memory: list by service: %w", err)
	}
	defer rows.Close()

	return im.scanIncidents(rows)
}

// AddTimelineEvent appends a timeline event to an incident.
func (im *IncidentMemory) AddTimelineEvent(ctx context.Context, incidentID string, event TimelineEvent) error {
	if im.store == nil {
		return nil
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	return im.store.WriteWithRetry(ctx,
		`INSERT INTO incident_timeline (incident_id, timestamp, type, content, source)
		 VALUES (?, ?, ?, ?, ?)`,
		incidentID, event.Timestamp, event.Type, event.Content, event.Source,
	)
}

// ResolveIncident resolves an incident with root cause and remediation details.
func (im *IncidentMemory) ResolveIncident(ctx context.Context, id string, rootCause, remediation string) error {
	if im.store == nil {
		return nil
	}

	now := time.Now().UTC()
	return im.store.WriteWithRetry(ctx,
		`UPDATE incidents SET status = 'resolved', root_cause = ?, remediation = ?, resolved_at = ?, updated_at = ?
		 WHERE id = ?`,
		rootCause, remediation, now, now, id,
	)
}

// CreatePostmortem stores a post-incident review.
func (im *IncidentMemory) CreatePostmortem(ctx context.Context, pm *Postmortem) error {
	if im.store == nil {
		return nil
	}

	contribJSON, _ := json.Marshal(pm.Contributing)
	actionJSON, _ := json.Marshal(pm.ActionItems)
	lessonsJSON, _ := json.Marshal(pm.LessonsLearned)

	if pm.CreatedAt.IsZero() {
		pm.CreatedAt = time.Now().UTC()
	}

	return im.store.WriteWithRetry(ctx,
		`INSERT INTO postmortems (id, incident_id, title, summary, root_cause, contributing, action_items, lessons_learned, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pm.ID, pm.IncidentID, pm.Title, pm.Summary, pm.RootCause,
		string(contribJSON), string(actionJSON), string(lessonsJSON), pm.CreatedAt,
	)
}

// GetPostmortem retrieves a postmortem by incident ID.
func (im *IncidentMemory) GetPostmortem(incidentID string) (*Postmortem, error) {
	if im.store == nil {
		return nil, nil
	}

	db := im.store.DB()
	row := db.QueryRow(
		`SELECT id, incident_id, title, summary, root_cause, contributing, action_items, lessons_learned, created_at
		 FROM postmortems WHERE incident_id = ?`, incidentID,
	)

	var pm Postmortem
	var contribJSON, actionJSON, lessonsJSON string

	if err := row.Scan(
		&pm.ID, &pm.IncidentID, &pm.Title, &pm.Summary, &pm.RootCause,
		&contribJSON, &actionJSON, &lessonsJSON, &pm.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("incident memory: get postmortem: %w", err)
	}

	if err := json.Unmarshal([]byte(contribJSON), &pm.Contributing); err != nil {
		slog.Warn("failed to unmarshal contributing factors", "error", err)
	}
	if err := json.Unmarshal([]byte(actionJSON), &pm.ActionItems); err != nil {
		slog.Warn("failed to unmarshal action items", "error", err)
	}
	if err := json.Unmarshal([]byte(lessonsJSON), &pm.LessonsLearned); err != nil {
		slog.Warn("failed to unmarshal lessons learned", "error", err)
	}

	return &pm, nil
}

// ListPostmortems returns the most recent postmortems up to the given limit.
func (im *IncidentMemory) ListPostmortems(limit int) ([]*Postmortem, error) {
	if im.store == nil {
		return nil, nil
	}

	if limit <= 0 {
		limit = 10
	}

	db := im.store.DB()
	rows, err := db.Query(
		`SELECT id, incident_id, title, summary, root_cause, contributing, action_items, lessons_learned, created_at
		 FROM postmortems ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("incident memory: list postmortems: %w", err)
	}
	defer rows.Close()

	var pms []*Postmortem
	for rows.Next() {
		var pm Postmortem
		var contribJSON, actionJSON, lessonsJSON string

		if err := rows.Scan(
			&pm.ID, &pm.IncidentID, &pm.Title, &pm.Summary, &pm.RootCause,
			&contribJSON, &actionJSON, &lessonsJSON, &pm.CreatedAt,
		); err != nil {
			continue
		}

		if err := json.Unmarshal([]byte(contribJSON), &pm.Contributing); err != nil {
			slog.Warn("failed to unmarshal contributing factors", "error", err)
		}
		if err := json.Unmarshal([]byte(actionJSON), &pm.ActionItems); err != nil {
			slog.Warn("failed to unmarshal action items", "error", err)
		}
		if err := json.Unmarshal([]byte(lessonsJSON), &pm.LessonsLearned); err != nil {
			slog.Warn("failed to unmarshal lessons learned", "error", err)
		}

		pms = append(pms, &pm)
	}

	return pms, nil
}

// SetSLOStatus upserts an SLO status record.
func (im *IncidentMemory) SetSLOStatus(slo *SLOStatus) error {
	if im.store == nil {
		return nil
	}

	return im.store.WriteWithRetry(context.Background(),
		`INSERT INTO slo_statuses (service, slo_name, target, current, error_budget_remaining, burn_rate, status, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(service, slo_name) DO UPDATE SET
		     target = excluded.target,
		     current = excluded.current,
		     error_budget_remaining = excluded.error_budget_remaining,
		     burn_rate = excluded.burn_rate,
		     status = excluded.status,
		     updated_at = excluded.updated_at`,
		slo.Service, slo.SLOName, slo.Target, slo.Current,
		slo.ErrorBudgetRemaining, slo.BurnRate, slo.Status, time.Now().UTC(),
	)
}

// GetSLOStatuses returns all SLO status records.
func (im *IncidentMemory) GetSLOStatuses() ([]*SLOStatus, error) {
	if im.store == nil {
		return nil, nil
	}

	db := im.store.DB()
	rows, err := db.Query(
		`SELECT service, slo_name, target, current, error_budget_remaining, burn_rate, status
		 FROM slo_statuses ORDER BY status DESC, burn_rate DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("incident memory: get slo statuses: %w", err)
	}
	defer rows.Close()

	var statuses []*SLOStatus
	for rows.Next() {
		var s SLOStatus
		if err := rows.Scan(&s.Service, &s.SLOName, &s.Target, &s.Current,
			&s.ErrorBudgetRemaining, &s.BurnRate, &s.Status); err != nil {
			continue
		}
		statuses = append(statuses, &s)
	}

	return statuses, nil
}

// BuildIncidentPrompt constructs the context block for injection into the system prompt.
// The output is capped at ~2000 characters.
func (im *IncidentMemory) BuildIncidentPrompt() string {
	if im.store == nil {
		return ""
	}

	im.mu.RLock()
	defer im.mu.RUnlock()

	var sb strings.Builder

	// Active Incidents section
	incidents, err := im.ListActiveIncidents()
	if err != nil {
		slog.Warn("incident memory: failed to list active incidents", "error", err)
	}

	if len(incidents) > 0 {
		sb.WriteString("=== Active Incidents ===\n")
		for _, inc := range incidents {
			line := fmt.Sprintf("[%s] %s: %s (service: %s)\n",
				strings.ToUpper(inc.Severity), inc.ID, inc.Title, inc.Service)
			sb.WriteString(line)

			sb.WriteString(fmt.Sprintf("  Started: %s | Status: %s\n",
				inc.StartedAt.UTC().Format("2006-01-02 15:04 UTC"), inc.Status))

			if len(inc.AffectedServices) > 0 {
				sb.WriteString(fmt.Sprintf("  Affected: %s\n",
					strings.Join(inc.AffectedServices, ", ")))
			}

			if inc.Remediation != "" {
				sb.WriteString(fmt.Sprintf("  Remediation: %s\n", inc.Remediation))
			}

			eventCount := len(inc.TimelineEvents)
			if eventCount > 0 {
				sb.WriteString(fmt.Sprintf("  Timeline: %d events\n", eventCount))
			}
		}
	}

	// SLO Status section
	sloStatuses, err := im.GetSLOStatuses()
	if err != nil {
		slog.Warn("incident memory: failed to get SLO statuses", "error", err)
	}

	if len(sloStatuses) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("=== SLO Status ===\n")
		for _, s := range sloStatuses {
			icon := "✅"
			extra := ""
			switch s.Status {
			case "critical":
				icon = "🔴"
				if s.BurnRate > 0 {
					extra = fmt.Sprintf(" — error budget exhausted in %.1fh", s.ErrorBudgetRemaining)
				}
			case "at_risk":
				icon = "⚠️"
				extra = fmt.Sprintf(" — error budget: %.0f%% remaining", s.ErrorBudgetRemaining*100)
			}
			line := fmt.Sprintf("%s: %.1f%% (target: %.1f%%) %s %s%s\n",
				s.Service, s.Current*100, s.Target*100, icon, s.Status, extra)
			sb.WriteString(line)
		}
	}

	// Recent Postmortems section (last 7 days)
	postmortems, err := im.ListPostmortems(5)
	if err != nil {
		slog.Warn("incident memory: failed to list postmortems", "error", err)
	}

	sevenDaysAgo := time.Now().UTC().Add(-7 * 24 * time.Hour)
	var recentPMs []*Postmortem
	for _, pm := range postmortems {
		if pm.CreatedAt.After(sevenDaysAgo) {
			recentPMs = append(recentPMs, pm)
		}
	}

	if len(recentPMs) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("=== Recent Postmortems (last 7 days) ===\n")
		for _, pm := range recentPMs {
			sb.WriteString(fmt.Sprintf("- %s: %s → root cause: %s\n",
				pm.IncidentID, pm.Title, pm.RootCause))
		}
	}

	result := sb.String()
	if len(result) > maxIncidentPromptChars {
		result = result[:maxIncidentPromptChars-3] + "..."
	}

	return result
}

// UpdateIncidentFromToolResult parses SOPA tool results and auto-creates or updates incidents.
// This method is called by the tool execution layer to feed real-time SRE data into the memory system.
func (im *IncidentMemory) UpdateIncidentFromToolResult(toolName string, result map[string]any) error {
	if im.store == nil {
		return nil
	}

	switch toolName {
	case "sopa_list_faults", "sopa_get_fault":
		return im.ingestAlertEvents(result)
	default:
		return nil
	}
}

// ingestAlertEvents processes alert event results and creates/updates incidents.
func (im *IncidentMemory) ingestAlertEvents(result map[string]any) error {
	eventsRaw, ok := result["data"]
	if !ok {
		eventsRaw, ok = result["events"]
	}
	if !ok {
		eventsRaw, ok = result["items"]
	}
	if !ok {
		return nil
	}

	events, ok := eventsRaw.([]any)
	if !ok {
		return nil
	}

	for _, ev := range events {
		evMap, ok := ev.(map[string]any)
		if !ok {
			continue
		}

		severity := "medium"
		if sev, ok := evMap["severity"].(string); ok {
			severity = sev
		}

		status := "active"
		if st, ok := evMap["status"].(string); ok {
			switch st {
			case "firing", "active", "open":
				status = "active"
			case "investigating", "acknowledged":
				status = "investigating"
			case "resolved", "closed":
				status = "resolved"
			default:
				status = st
			}
		}

		id, _ := evMap["id"].(string)
		if id == "" {
			id = fmt.Sprintf("ALERT-%d", time.Now().UnixNano())
		}

		title, _ := evMap["title"].(string)
		if title == "" {
			title, _ = evMap["name"].(string)
		}

		service, _ := evMap["service"].(string)
		if service == "" {
			if labels, ok := evMap["labels"].(map[string]any); ok {
				service, _ = labels["service"].(string)
			}
		}

		description, _ := evMap["description"].(string)
		if description == "" {
			description, _ = evMap["message"].(string)
		}

		alertSource, _ := evMap["source"].(string)
		if alertSource == "" {
			alertSource, _ = evMap["alert_source"].(string)
		}

		// Check if incident already exists
		existing, err := im.GetIncident(id)
		if err != nil {
			slog.Warn("incident memory: error checking existing incident", "id", id, "error", err)
			continue
		}

		if existing != nil {
			// Update status if changed
			if existing.Status != status {
				updates := map[string]any{"status": status}
				if status == "resolved" {
					now := time.Now().UTC()
					updates["resolved_at"] = now
				}
				if err := im.UpdateIncident(context.Background(), id, updates); err != nil {
					slog.Warn("incident memory: failed to update incident from alert", "id", id, "error", err)
				}
			}
		} else if status != "resolved" {
			incident := &Incident{
				ID:          id,
				Title:       title,
				Severity:    severity,
				Status:      status,
				Service:     service,
				Description: description,
				AlertSource: alertSource,
				StartedAt:   time.Now().UTC(),
			}
			if err := im.CreateIncident(context.Background(), incident); err != nil {
				slog.Warn("incident memory: failed to create incident from alert", "id", id, "error", err)
			}
		}
	}

	return nil
}

// ingestFaultTrackings processes fault tracking results and creates/updates incidents.
func (im *IncidentMemory) ingestFaultTrackings(result map[string]any) error {
	itemsRaw, ok := result["items"]
	if !ok {
		itemsRaw, ok = result["faults"]
	}
	if !ok {
		return nil
	}

	items, ok := itemsRaw.([]any)
	if !ok {
		return nil
	}

	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		id, _ := itemMap["id"].(string)
		if id == "" {
			id = fmt.Sprintf("FAULT-%d", time.Now().UnixNano())
		}

		title, _ := itemMap["title"].(string)
		if title == "" {
			title, _ = itemMap["name"].(string)
		}

		severity := "medium"
		if sev, ok := itemMap["severity"].(string); ok {
			severity = sev
		}

		service, _ := itemMap["node_name"].(string)
		if service == "" {
			service, _ = itemMap["service"].(string)
		}

		status := "active"
		if st, ok := itemMap["status"].(string); ok {
			switch st {
			case "open", "active":
				status = "active"
			case "investigating", "in_progress":
				status = "investigating"
			case "mitigated", "resolved", "closed":
				status = "resolved"
			default:
				status = st
			}
		}

		description, _ := itemMap["description"].(string)
		if description == "" {
			description, _ = itemMap["summary"].(string)
		}

		rootCause, _ := itemMap["root_cause"].(string)
		remediation, _ := itemMap["remediation"].(string)

		existing, err := im.GetIncident(id)
		if err != nil {
			slog.Warn("incident memory: error checking existing fault", "id", id, "error", err)
			continue
		}

		if existing != nil {
			updates := map[string]any{"status": status}
			if rootCause != "" {
				updates["root_cause"] = rootCause
			}
			if remediation != "" {
				updates["remediation"] = remediation
			}
			if status == "resolved" {
				now := time.Now().UTC()
				updates["resolved_at"] = now
			}
			if err := im.UpdateIncident(context.Background(), id, updates); err != nil {
				slog.Warn("incident memory: failed to update incident from fault", "id", id, "error", err)
			}
		} else if status != "resolved" {
			incident := &Incident{
				ID:          id,
				Title:       title,
				Severity:    severity,
				Status:      status,
				Service:     service,
				Description: description,
				RootCause:   rootCause,
				Remediation: remediation,
				StartedAt:   time.Now().UTC(),
			}
			if err := im.CreateIncident(context.Background(), incident); err != nil {
				slog.Warn("incident memory: failed to create incident from fault", "id", id, "error", err)
			}
		}
	}

	return nil
}

// scanIncidents is a helper to scan multiple incident rows from a query result.
func (im *IncidentMemory) scanIncidents(rows *sql.Rows) ([]*Incident, error) {
	var incidents []*Incident
	for rows.Next() {
		var inc Incident
		var affectedJSON string
		var mitigatedAt, resolvedAt sql.NullTime

		if err := rows.Scan(
			&inc.ID, &inc.Title, &inc.Severity, &inc.Status, &inc.Service,
			&inc.Description, &inc.RootCause, &inc.Remediation,
			&inc.StartedAt, &mitigatedAt, &resolvedAt, &inc.AlertSource, &affectedJSON,
		); err != nil {
			continue
		}

		if mitigatedAt.Valid {
			inc.MitigatedAt = &mitigatedAt.Time
		}
		if resolvedAt.Valid {
			inc.ResolvedAt = &resolvedAt.Time
		}
		if err := json.Unmarshal([]byte(affectedJSON), &inc.AffectedServices); err != nil {
			slog.Warn("failed to unmarshal affected services", "error", err)
		}

		incidents = append(incidents, &inc)
	}

	return incidents, nil
}

// loadTimelineEvents loads timeline events for a given incident.
func (im *IncidentMemory) loadTimelineEvents(db *sql.DB, incidentID string) ([]TimelineEvent, error) {
	rows, err := db.Query(
		`SELECT timestamp, type, content, source
		 FROM incident_timeline WHERE incident_id = ?
		 ORDER BY timestamp ASC`, incidentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []TimelineEvent
	for rows.Next() {
		var ev TimelineEvent
		if err := rows.Scan(&ev.Timestamp, &ev.Type, &ev.Content, &ev.Source); err != nil {
			continue
		}
		events = append(events, ev)
	}

	return events, nil
}
