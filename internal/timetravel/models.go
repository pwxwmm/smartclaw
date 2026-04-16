package timetravel

import "time"

// ReplaySource indicates where the replay data originates from.
type ReplaySource string

const (
	ReplayRecording      ReplaySource = "recording"       // from session recorder JSONL
	ReplayIncidentMemory ReplaySource = "incident_memory" // from incident_memory L5
	ReplayTimeline       ReplaySource = "timeline"        // from incident timeline events
)

// ReplayStatus represents the current state of a replay session.
type ReplayStatus string

const (
	ReplayLoading  ReplayStatus = "loading"
	ReplayReady    ReplayStatus = "ready"
	ReplayPlaying  ReplayStatus = "playing"
	ReplayPaused   ReplayStatus = "paused"
	ReplayComplete ReplayStatus = "complete"
	ReplayError    ReplayStatus = "error"
)

// ReplaySession holds the full state of an investigation replay.
type ReplaySession struct {
	ID          string         `json:"id"`
	IncidentID  string         `json:"incident_id,omitempty"`
	SourceType  ReplaySource   `json:"source_type"`
	SourceID    string         `json:"source_id"`
	Status      ReplayStatus   `json:"status"`
	Events      []ReplayEvent  `json:"events"`
	Summary     *ReplaySummary `json:"summary,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// ReplayEvent represents a single event in a replay timeline.
type ReplayEvent struct {
	Timestamp time.Time      `json:"timestamp"`
	Type      string         `json:"type"` // tool_call, tool_result, message, alert, incident, slo_change, deployment
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Result    string         `json:"result"`
	Service   string         `json:"service,omitempty"`
	Severity  string         `json:"severity,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ReplaySummary contains the analysis results for a replay session.
type ReplaySummary struct {
	TotalEvents   int             `json:"total_events"`
	Duration      time.Duration   `json:"duration"`
	KeyMoments    []KeyMoment     `json:"key_moments"`
	Timeline      []TimelinePhase `json:"timeline"`
	RootCauseHint string          `json:"root_cause_hint,omitempty"`
}

// KeyMoment identifies an important point in the incident timeline.
type KeyMoment struct {
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	Type        string    `json:"type"`       // trigger, escalation, root_cause, mitigation, resolution
	Importance  float64   `json:"importance"` // 0.0-1.0
}

// TimelinePhase represents a named phase in the incident lifecycle.
type TimelinePhase struct {
	Name       string    `json:"name"` // detection, triage, investigation, mitigation, resolution
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	EventCount int       `json:"event_count"`
}

// WhatIfScenario represents a hypothetical divergence from the actual timeline.
type WhatIfScenario struct {
	ID               string    `json:"id"`
	Description      string    `json:"description"`
	ChangePoint      time.Time `json:"change_point"`
	Change           string    `json:"change"`
	ProjectedOutcome string    `json:"projected_outcome"`
	Confidence       float64   `json:"confidence"`
}

// RecordingEntry represents a single entry from a session recording file.
type RecordingEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data"`
}

// TimelineEvent represents an event from an incident timeline.
type TimelineEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
}

// IncidentInfo holds metadata about an incident for replay context.
type IncidentInfo struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Severity   string     `json:"severity"`
	Status     string     `json:"status"`
	Service    string     `json:"service"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}
