package timetravel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type RecordingStore interface {
	LoadRecording(path string) ([]RecordingEntry, error)
	ListRecordings() ([]string, error)
}

type IncidentStore interface {
	GetIncidentTimeline(incidentID string) ([]TimelineEvent, error)
	GetIncident(incidentID string) (*IncidentInfo, error)
}

type TimeTravelEngine struct {
	mu       sync.RWMutex
	sessions map[string]*ReplaySession

	recordingStore RecordingStore
	incidentStore  IncidentStore
}

func NewTimeTravelEngine() *TimeTravelEngine {
	return &TimeTravelEngine{
		sessions: make(map[string]*ReplaySession),
	}
}

func Shutdown() {
	defaultEngineMu.Lock()
	if defaultEngine != nil {
		defaultEngine.sessions = make(map[string]*ReplaySession)
	}
	defaultEngineMu.Unlock()
}

func (e *TimeTravelEngine) sessionCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.sessions)
}

func (e *TimeTravelEngine) SetRecordingStore(rs RecordingStore) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recordingStore = rs
}

func (e *TimeTravelEngine) SetIncidentStore(is IncidentStore) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.incidentStore = is
}

func (e *TimeTravelEngine) ReplayIncident(ctx context.Context, incidentID string) (*ReplaySession, error) {
	if e.sessionCount() >= config.MaxSessions {
		return nil, fmt.Errorf("max replay sessions (%d) reached", config.MaxSessions)
	}

	e.mu.RLock()
	is := e.incidentStore
	e.mu.RUnlock()

	if is == nil {
		return nil, fmt.Errorf("incident store not configured")
	}

	incident, err := is.GetIncident(incidentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get incident %s: %w", incidentID, err)
	}
	if incident == nil {
		return nil, fmt.Errorf("incident %s not found", incidentID)
	}

	timeline, err := is.GetIncidentTimeline(incidentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline for incident %s: %w", incidentID, err)
	}

	events := e.ExtractEventsFromTimeline(timeline, incident)

	session := &ReplaySession{
		ID:         generateSessionID(),
		IncidentID: incidentID,
		SourceType: ReplayIncidentMemory,
		SourceID:   incidentID,
		Status:     ReplayComplete,
		Events:     events,
		Summary:    e.AnalyzeTimeline(events),
		CreatedAt:  time.Now().UTC(),
	}
	now := time.Now().UTC()
	session.CompletedAt = &now

	e.mu.Lock()
	e.sessions[session.ID] = session
	if len(e.sessions) > config.MaxSessions {
		var oldestID string
		var oldestTime time.Time
		first := true
		for id, s := range e.sessions {
			if first || s.CreatedAt.Before(oldestTime) {
				oldestID = id
				oldestTime = s.CreatedAt
				first = false
			}
		}
		delete(e.sessions, oldestID)
	}
	e.mu.Unlock()

	metricTimetravelReplays.Inc()

	return session, nil
}

func (e *TimeTravelEngine) ReplayRecording(ctx context.Context, recordingPath string) (*ReplaySession, error) {
	e.mu.RLock()
	rs := e.recordingStore
	e.mu.RUnlock()

	if rs == nil {
		return nil, fmt.Errorf("recording store not configured")
	}

	entries, err := rs.LoadRecording(recordingPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load recording %s: %w", recordingPath, err)
	}

	events := e.ExtractEventsFromRecording(entries)

	session := &ReplaySession{
		ID:         generateSessionID(),
		SourceType: ReplayRecording,
		SourceID:   recordingPath,
		Status:     ReplayComplete,
		Events:     events,
		Summary:    e.AnalyzeTimeline(events),
		CreatedAt:  time.Now().UTC(),
	}
	now := time.Now().UTC()
	session.CompletedAt = &now

	e.mu.Lock()
	e.sessions[session.ID] = session
	if len(e.sessions) > config.MaxSessions {
		var oldestID string
		var oldestTime time.Time
		first := true
		for id, s := range e.sessions {
			if first || s.CreatedAt.Before(oldestTime) {
				oldestID = id
				oldestTime = s.CreatedAt
				first = false
			}
		}
		delete(e.sessions, oldestID)
	}
	e.mu.Unlock()

	return session, nil
}

func (e *TimeTravelEngine) GetSession(sessionID string) *ReplaySession {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessions[sessionID]
}

func (e *TimeTravelEngine) AnalyzeTimeline(events []ReplayEvent) *ReplaySummary {
	if len(events) == 0 {
		return &ReplaySummary{
			Timeline:   []TimelinePhase{},
			KeyMoments: []KeyMoment{},
		}
	}

	sorted := make([]ReplayEvent, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	phases := detectPhases(sorted)
	keyMoments := identifyKeyMoments(sorted, phases)
	rootCauseHint := findRootCauseHint(sorted, phases)

	var duration time.Duration
	if len(sorted) > 1 {
		duration = sorted[len(sorted)-1].Timestamp.Sub(sorted[0].Timestamp)
	}

	return &ReplaySummary{
		TotalEvents:   len(sorted),
		Duration:      duration,
		KeyMoments:    keyMoments,
		Timeline:      phases,
		RootCauseHint: rootCauseHint,
	}
}

func (e *TimeTravelEngine) WhatIf(sessionID string, eventIndex int, change string) (*WhatIfScenario, error) {
	e.mu.RLock()
	session := e.sessions[sessionID]
	e.mu.RUnlock()

	if session == nil {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	if eventIndex < 0 || eventIndex >= len(session.Events) {
		return nil, fmt.Errorf("event index %d out of range (0-%d)", eventIndex, len(session.Events)-1)
	}

	evt := session.Events[eventIndex]
	subsequentCount := len(session.Events) - eventIndex - 1
	outcome, confidence := projectWhatIf(change, subsequentCount)

	metricTimetravelWhatIf.Inc()

	return &WhatIfScenario{
		ID:               fmt.Sprintf("whatif-%d", time.Now().UnixNano()),
		Description:      fmt.Sprintf("If at %s we had: %s", evt.Timestamp.Format(time.RFC3339), change),
		ChangePoint:      evt.Timestamp,
		Change:           change,
		ProjectedOutcome: outcome,
		Confidence:       confidence,
	}, nil
}

func (e *TimeTravelEngine) ExtractEventsFromRecording(entries []RecordingEntry) []ReplayEvent {
	events := make([]ReplayEvent, 0, len(entries))
	for _, entry := range entries {
		evt := recordingEntryToReplayEvent(entry)
		events = append(events, evt)
	}
	return events
}

func (e *TimeTravelEngine) ExtractEventsFromTimeline(timeline []TimelineEvent, incident *IncidentInfo) []ReplayEvent {
	events := make([]ReplayEvent, 0, len(timeline))
	for _, te := range timeline {
		evt := timelineEventToReplayEvent(te, incident)
		events = append(events, evt)
	}
	return events
}

func recordingEntryToReplayEvent(entry RecordingEntry) ReplayEvent {
	evt := ReplayEvent{
		Timestamp: entry.Timestamp,
		Type:      mapRecordingType(entry.Type),
		Metadata:  entry.Data,
	}

	switch entry.Type {
	case "tool_call":
		if tool, ok := entry.Data["tool"].(string); ok {
			evt.Actor = "agent"
			evt.Action = "called " + tool
		}
		if input, ok := entry.Data["input"]; ok {
			evt.Metadata = map[string]any{"input": input}
		}
	case "tool_result":
		if tool, ok := entry.Data["tool"].(string); ok {
			evt.Actor = "system"
			evt.Action = tool + " returned"
		}
		if result, ok := entry.Data["result"]; ok {
			evt.Result = fmt.Sprintf("%v", result)
		}
	case "message":
		if role, ok := entry.Data["role"].(string); ok {
			evt.Actor = role
		}
		if content, ok := entry.Data["content"].(string); ok {
			evt.Action = content
		}
	default:
		evt.Actor = "system"
		evt.Action = entry.Type
	}

	return evt
}

func mapRecordingType(t string) string {
	switch t {
	case "tool_call":
		return "tool_call"
	case "tool_result":
		return "tool_result"
	case "message":
		return "message"
	default:
		return t
	}
}

func timelineEventToReplayEvent(te TimelineEvent, incident *IncidentInfo) ReplayEvent {
	evt := ReplayEvent{
		Timestamp: te.Timestamp,
		Type:      mapTimelineType(te.Type),
		Actor:     te.Source,
		Action:    te.Content,
	}

	if incident != nil {
		evt.Service = incident.Service
		evt.Severity = incident.Severity
	}

	return evt
}

func mapTimelineType(t string) string {
	switch t {
	case "alert", "hypothesis", "evidence", "escalation", "mitigation", "resolution":
		return t
	default:
		return "incident"
	}
}

func detectPhases(events []ReplayEvent) []TimelinePhase {
	if len(events) == 0 {
		return []TimelinePhase{}
	}

	phaseOrder := []string{"detection", "triage", "investigation", "mitigation", "resolution"}
	phaseBounds := make(map[string]*phaseBound)

	for i, evt := range events {
		phase := classifyEventPhase(evt, phaseBounds)
		if _, exists := phaseBounds[phase]; !exists {
			phaseBounds[phase] = &phaseBound{startIdx: i, endIdx: i}
		} else {
			phaseBounds[phase].endIdx = i
		}
	}

	var phases []TimelinePhase
	for _, name := range phaseOrder {
		b, exists := phaseBounds[name]
		if !exists {
			continue
		}
		phases = append(phases, TimelinePhase{
			Name:       name,
			Start:      events[b.startIdx].Timestamp,
			End:        events[b.endIdx].Timestamp,
			EventCount: b.endIdx - b.startIdx + 1,
		})
	}

	return phases
}

type phaseBound struct {
	startIdx int
	endIdx   int
}

func classifyEventPhase(evt ReplayEvent, existing map[string]*phaseBound) string {
	if isAlertOrIncident(evt) && !phaseExists(existing, "detection") {
		return "detection"
	}

	if isResolution(evt) {
		return "resolution"
	}

	if isMitigationAction(evt) {
		return "mitigation"
	}

	if isToolCallOrInvestigation(evt) {
		if !phaseExists(existing, "triage") {
			return "triage"
		}
		return "investigation"
	}

	if phaseExists(existing, "investigation") {
		if isMitigationAction(evt) || phaseExists(existing, "mitigation") {
			if phaseExists(existing, "resolution") {
				return "resolution"
			}
			return "mitigation"
		}
		return "investigation"
	}
	if phaseExists(existing, "triage") {
		return "investigation"
	}
	if phaseExists(existing, "detection") {
		return "triage"
	}
	return "detection"
}

func phaseExists(existing map[string]*phaseBound, name string) bool {
	_, ok := existing[name]
	return ok
}

func isAlertOrIncident(evt ReplayEvent) bool {
	return evt.Type == "alert" || evt.Type == "incident" || evt.Type == "slo_change"
}

func isToolCallOrInvestigation(evt ReplayEvent) bool {
	if evt.Type == "tool_call" {
		return true
	}
	action := strings.ToLower(evt.Action)
	return strings.Contains(action, "search") || strings.Contains(action, "grep") ||
		strings.Contains(action, "read") || strings.Contains(action, "investigate") ||
		strings.Contains(action, "hypothesis") || strings.Contains(action, "evidence")
}

func isMitigationAction(evt ReplayEvent) bool {
	if evt.Type == "mitigation" || evt.Type == "escalation" {
		return true
	}
	action := strings.ToLower(evt.Action)
	return strings.Contains(action, "restart") || strings.Contains(action, "scale") ||
		strings.Contains(action, "rollback") || strings.Contains(action, "deploy") ||
		strings.Contains(action, "fix") || strings.Contains(action, "mitigate") ||
		strings.Contains(action, "kubectl") || strings.Contains(action, "write")
}

func isResolution(evt ReplayEvent) bool {
	return evt.Type == "resolution"
}

func identifyKeyMoments(events []ReplayEvent, phases []TimelinePhase) []KeyMoment {
	var moments []KeyMoment

	phaseFirstEvent := make(map[string]int)
	for _, p := range phases {
		for i, evt := range events {
			if evt.Timestamp.Equal(p.Start) {
				phaseFirstEvent[p.Name] = i
				break
			}
		}
	}

	phaseImportance := map[string]float64{
		"detection":     0.9,
		"triage":        0.7,
		"investigation": 0.5,
		"mitigation":    0.8,
		"resolution":    0.9,
	}

	for _, p := range phases {
		idx, ok := phaseFirstEvent[p.Name]
		if !ok || idx >= len(events) {
			continue
		}
		evt := events[idx]
		moments = append(moments, KeyMoment{
			Timestamp:   evt.Timestamp,
			Description: fmt.Sprintf("Phase transition: %s — %s", p.Name, evt.Action),
			Type:        phaseTransitionType(p.Name),
			Importance:  phaseImportance[p.Name],
		})
	}

	for _, evt := range events {
		action := strings.ToLower(evt.Action)
		result := strings.ToLower(evt.Result)
		if strings.Contains(action, "root cause") || strings.Contains(result, "root cause") ||
			strings.Contains(action, "found") || strings.Contains(result, "found") ||
			strings.Contains(action, "caused by") || strings.Contains(result, "caused by") {
			moments = append(moments, KeyMoment{
				Timestamp:   evt.Timestamp,
				Description: evt.Action,
				Type:        "root_cause",
				Importance:  0.95,
			})
		}
	}

	prevSeverity := ""
	for _, evt := range events {
		if evt.Severity != "" && evt.Severity != prevSeverity {
			if prevSeverity != "" && severityRank(evt.Severity) > severityRank(prevSeverity) {
				moments = append(moments, KeyMoment{
					Timestamp:   evt.Timestamp,
					Description: fmt.Sprintf("Severity escalated: %s → %s", prevSeverity, evt.Severity),
					Type:        "escalation",
					Importance:  0.85,
				})
			}
			prevSeverity = evt.Severity
		}
	}

	for _, evt := range events {
		if evt.Type == "tool_result" && evt.Result != "" {
			result := strings.ToLower(evt.Result)
			if strings.Contains(result, "error") || strings.Contains(result, "fail") ||
				strings.Contains(result, "cause") || strings.Contains(result, "found") {
				moments = append(moments, KeyMoment{
					Timestamp:   evt.Timestamp,
					Description: fmt.Sprintf("Actionable finding: %s", truncate(evt.Result, 100)),
					Type:        "escalation",
					Importance:  0.6,
				})
			}
		}
	}

	sort.Slice(moments, func(i, j int) bool {
		return moments[i].Timestamp.Before(moments[j].Timestamp)
	})

	return moments
}

func phaseTransitionType(phase string) string {
	switch phase {
	case "detection":
		return "trigger"
	case "mitigation":
		return "mitigation"
	case "resolution":
		return "resolution"
	default:
		return "escalation"
	}
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func findRootCauseHint(events []ReplayEvent, phases []TimelinePhase) string {
	var lastInvestigationEvent *ReplayEvent
	mitigationStarted := false

	for i := range phases {
		if phases[i].Name == "mitigation" {
			mitigationStarted = true
			break
		}
	}

	for i := len(events) - 1; i >= 0; i-- {
		evt := &events[i]
		if mitigationStarted && evt.Timestamp.Before(phases[len(phases)-1].Start) {
		}
		action := strings.ToLower(evt.Action)
		result := strings.ToLower(evt.Result)
		if strings.Contains(action, "root cause") || strings.Contains(result, "root cause") ||
			strings.Contains(action, "caused by") || strings.Contains(result, "caused by") ||
			strings.Contains(action, "found") || strings.Contains(result, "found") {
			if evt.Action != "" {
				return evt.Action
			}
			return truncate(evt.Result, 200)
		}
	}

	if mitigationStarted {
		for i := len(events) - 1; i >= 0; i-- {
			evt := &events[i]
			if isToolCallOrInvestigation(*evt) {
				lastInvestigationEvent = evt
				break
			}
		}
	}
	if lastInvestigationEvent != nil {
		return truncate(lastInvestigationEvent.Action, 200)
	}

	return ""
}

func projectWhatIf(change string, subsequentCount int) (string, float64) {
	changeLower := strings.ToLower(change)

	if strings.Contains(changeLower, "earlier") || strings.Contains(changeLower, "faster") ||
		strings.Contains(changeLower, "detect") || strings.Contains(changeLower, "alert") {
		confidence := 0.7
		if subsequentCount > 10 {
			confidence -= 0.1
		}
		return "Earlier detection would have reduced the time to triage and investigation, potentially cutting total incident duration by 30-50%.", confidence
	}

	if strings.Contains(changeLower, "automat") || strings.Contains(changeLower, "auto-remediat") ||
		strings.Contains(changeLower, "runbook") || strings.Contains(changeLower, "self-heal") {
		confidence := 0.8
		return "Automated remediation would have resolved the incident near-instantly, bypassing manual investigation and mitigation phases entirely.", confidence
	}

	if strings.Contains(changeLower, "different tool") || strings.Contains(changeLower, "other") ||
		strings.Contains(changeLower, "alternative") {
		confidence := 0.4
		if subsequentCount > 5 {
			confidence -= 0.15
		}
		return "A different tool might have led to a different investigation path, potentially faster or slower depending on the tool's diagnostic capabilities.", confidence
	}

	confidence := 0.5
	if subsequentCount > 5 {
		confidence -= 0.1
	}
	if subsequentCount > 10 {
		confidence -= 0.1
	}
	return fmt.Sprintf("The proposed change would affect %d subsequent events, potentially altering the investigation and mitigation timeline.", subsequentCount), confidence
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func generateSessionID() string {
	return fmt.Sprintf("replay-%d", time.Now().UnixNano())
}
