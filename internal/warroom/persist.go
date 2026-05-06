package warroom

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type WarRoomStore interface {
	SaveSession(session *WarRoomSession) error
	LoadSession(id string) (*WarRoomSession, error)
	ListSessions() ([]*WarRoomSession, error)
	DeleteSession(id string) error
	SaveBlackboard(sessionID string, bb *Blackboard) error
	LoadBlackboard(sessionID string) (*Blackboard, error)
}

type SQLiteWarRoomStore struct {
	db *sql.DB
}

func NewSQLiteWarRoomStore(db *sql.DB) *SQLiteWarRoomStore {
	return &SQLiteWarRoomStore{db: db}
}

func (s *SQLiteWarRoomStore) SaveSession(session *WarRoomSession) error {
	if session == nil {
		return fmt.Errorf("warroom: cannot save nil session")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("warroom: begin tx: %w", err)
	}
	defer tx.Rollback()

	contextJSON, err := json.Marshal(session.Context)
	if err != nil {
		return fmt.Errorf("warroom: marshal context: %w", err)
	}

	var closedAt sql.NullString
	if session.ClosedAt != nil {
		closedAt = sql.NullString{String: session.ClosedAt.Format(time.RFC3339Nano), Valid: true}
	}

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO warroom_sessions (id, incident_id, title, description, status, context, created_at, closed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.IncidentID,
		session.Title,
		session.Description,
		string(session.Status),
		string(contextJSON),
		session.CreatedAt.Format(time.RFC3339Nano),
		closedAt,
	)
	if err != nil {
		return fmt.Errorf("warroom: save session: %w", err)
	}

	tx.Exec(`DELETE FROM warroom_agents WHERE session_id = ?`, session.ID)
	tx.Exec(`DELETE FROM warroom_findings WHERE session_id = ?`, session.ID)
	tx.Exec(`DELETE FROM warroom_timeline WHERE session_id = ?`, session.ID)

	for _, a := range session.Agents {
		_, err = tx.Exec(`
			INSERT INTO warroom_agents (session_id, agent_type, status, assigned_at, last_active)
			VALUES (?, ?, ?, ?, ?)`,
			session.ID,
			string(a.AgentType),
			string(a.Status),
			a.AssignedAt.Format(time.RFC3339Nano),
			a.LastActive.Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("warroom: save agent: %w", err)
		}
	}

	for _, f := range session.Findings {
		evidenceJSON, err := json.Marshal(f.Evidence)
		if err != nil {
			return fmt.Errorf("warroom: marshal evidence: %w", err)
		}
		xrefJSON, err := json.Marshal(f.CrossReferences)
		if err != nil {
			return fmt.Errorf("warroom: marshal cross_references: %w", err)
		}

		_, err = tx.Exec(`
			INSERT INTO warroom_findings (id, session_id, agent_type, category, title, description, confidence, evidence, cross_references, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.ID,
			session.ID,
			string(f.AgentType),
			f.Category,
			f.Title,
			f.Description,
			f.Confidence,
			string(evidenceJSON),
			string(xrefJSON),
			f.CreatedAt.Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("warroom: save finding: %w", err)
		}
	}

	for _, e := range session.Timeline {
		_, err = tx.Exec(`
			INSERT INTO warroom_timeline (session_id, agent_type, event, details, timestamp)
			VALUES (?, ?, ?, ?, ?)`,
			session.ID,
			string(e.AgentType),
			e.Event,
			e.Details,
			e.Timestamp.Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("warroom: save timeline: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("warroom: commit: %w", err)
	}

	return nil
}

func (s *SQLiteWarRoomStore) LoadSession(id string) (*WarRoomSession, error) {
	var session WarRoomSession
	var contextJSON string
	var createdAtStr string
	var closedAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, incident_id, title, description, status, context, created_at, closed_at
		FROM warroom_sessions WHERE id = ?`, id).
		Scan(&session.ID, &session.IncidentID, &session.Title, &session.Description,
			&session.Status, &contextJSON, &createdAtStr, &closedAt)
	if err != nil {
		return nil, fmt.Errorf("warroom: load session %s: %w", id, err)
	}

	if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
		session.CreatedAt = t
	}

	session.Context = make(map[string]any)
	if contextJSON != "" {
		json.Unmarshal([]byte(contextJSON), &session.Context)
	}

	if closedAt.Valid {
		if t, err := time.Parse(time.RFC3339Nano, closedAt.String); err == nil {
			session.ClosedAt = &t
		}
	}

	agentRows, err := s.db.Query(`
		SELECT agent_type, status, assigned_at, last_active
		FROM warroom_agents WHERE session_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("warroom: load agents: %w", err)
	}
	defer agentRows.Close()

	session.Agents = []AgentAssignment{}
	for agentRows.Next() {
		var a AgentAssignment
		var agentType, status, assignedAt, lastActive string
		if err := agentRows.Scan(&agentType, &status, &assignedAt, &lastActive); err != nil {
			return nil, fmt.Errorf("warroom: scan agent: %w", err)
		}
		a.AgentType = DomainAgentType(agentType)
		a.Status = AgentStatus(status)
		if t, err := time.Parse(time.RFC3339Nano, assignedAt); err == nil {
			a.AssignedAt = t
		}
		if t, err := time.Parse(time.RFC3339Nano, lastActive); err == nil {
			a.LastActive = t
		}
		a.Findings = []Finding{}
		session.Agents = append(session.Agents, a)
	}

	findingRows, err := s.db.Query(`
		SELECT id, agent_type, category, title, description, confidence, evidence, cross_references, created_at
		FROM warroom_findings WHERE session_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("warroom: load findings: %w", err)
	}
	defer findingRows.Close()

	session.Findings = []Finding{}
	for findingRows.Next() {
		var f Finding
		var agentType, evidenceJSON, xrefJSON, fCreatedAt string
		if err := findingRows.Scan(&f.ID, &agentType, &f.Category, &f.Title, &f.Description,
			&f.Confidence, &evidenceJSON, &xrefJSON, &fCreatedAt); err != nil {
			return nil, fmt.Errorf("warroom: scan finding: %w", err)
		}
		f.AgentType = DomainAgentType(agentType)
		json.Unmarshal([]byte(evidenceJSON), &f.Evidence)
		if f.Evidence == nil {
			f.Evidence = []string{}
		}
		json.Unmarshal([]byte(xrefJSON), &f.CrossReferences)
		if f.CrossReferences == nil {
			f.CrossReferences = []CrossReference{}
		}
		if t, err := time.Parse(time.RFC3339Nano, fCreatedAt); err == nil {
			f.CreatedAt = t
		}
		session.Findings = append(session.Findings, f)
	}

	for i := range session.Agents {
		for _, f := range session.Findings {
			if f.AgentType == session.Agents[i].AgentType {
				session.Agents[i].Findings = append(session.Agents[i].Findings, f)
			}
		}
	}

	timelineRows, err := s.db.Query(`
		SELECT agent_type, event, details, timestamp
		FROM warroom_timeline WHERE session_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("warroom: load timeline: %w", err)
	}
	defer timelineRows.Close()

	session.Timeline = []TimelineEntry{}
	for timelineRows.Next() {
		var e TimelineEntry
		var agentType, ts string
		if err := timelineRows.Scan(&agentType, &e.Event, &e.Details, &ts); err != nil {
			return nil, fmt.Errorf("warroom: scan timeline: %w", err)
		}
		e.AgentType = DomainAgentType(agentType)
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			e.Timestamp = t
		}
		session.Timeline = append(session.Timeline, e)
	}

	return &session, nil
}

func (s *SQLiteWarRoomStore) ListSessions() ([]*WarRoomSession, error) {
	rows, err := s.db.Query(`
		SELECT id, incident_id, title, description, status, context, created_at, closed_at
		FROM warroom_sessions ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("warroom: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*WarRoomSession
	for rows.Next() {
		var session WarRoomSession
		var contextJSON string
		var createdAtStr string
		var closedAt sql.NullString
		if err := rows.Scan(&session.ID, &session.IncidentID, &session.Title, &session.Description,
			&session.Status, &contextJSON, &createdAtStr, &closedAt); err != nil {
			return nil, fmt.Errorf("warroom: scan session: %w", err)
		}

		if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			session.CreatedAt = t
		}

		session.Context = make(map[string]any)
		if contextJSON != "" {
			json.Unmarshal([]byte(contextJSON), &session.Context)
		}

		if closedAt.Valid {
			if t, err := time.Parse(time.RFC3339Nano, closedAt.String); err == nil {
				session.ClosedAt = &t
			}
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (s *SQLiteWarRoomStore) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM warroom_sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("warroom: delete session %s: %w", id, err)
	}
	return nil
}

type blackboardEntryRow struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
}

func (s *SQLiteWarRoomStore) SaveBlackboard(sessionID string, bb *Blackboard) error {
	if bb == nil {
		return fmt.Errorf("warroom: cannot save nil blackboard")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("warroom: begin tx: %w", err)
	}
	defer tx.Rollback()

	tx.Exec(`DELETE FROM warroom_blackboard_entries WHERE session_id = ?`, sessionID)
	tx.Exec(`DELETE FROM warroom_hypotheses WHERE session_id = ?`, sessionID)
	tx.Exec(`DELETE FROM warroom_shared_facts WHERE session_id = ?`, sessionID)

	entries := bb.ReadEntries("")
	for _, e := range entries {
		rowJSON, err := json.Marshal(blackboardEntryRow{Key: e.Key, Value: e.Value})
		if err != nil {
			return fmt.Errorf("warroom: marshal blackboard entry: %w", err)
		}
		_, err = tx.Exec(`
			INSERT INTO warroom_blackboard_entries (session_id, agent_type, content, entry_type, timestamp)
			VALUES (?, ?, ?, ?, ?)`,
			sessionID,
			string(e.Author),
			string(rowJSON),
			e.Category,
			e.Timestamp.Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("warroom: save blackboard entry: %w", err)
		}
	}

	hypotheses := bb.GetHypotheses()
	for _, h := range hypotheses {
		supportingJSON, err := json.Marshal(h.SupportingEvidence)
		if err != nil {
			return fmt.Errorf("warroom: marshal hypothesis supporting evidence: %w", err)
		}
		contradictingJSON, err := json.Marshal(h.ContradictingEvidence)
		if err != nil {
			return fmt.Errorf("warroom: marshal hypothesis contradicting evidence: %w", err)
		}

		evidenceCombined := string(supportingJSON) + "\x00" + string(contradictingJSON)
		now := time.Now()
		_, err = tx.Exec(`
			INSERT INTO warroom_hypotheses (id, session_id, agent_type, content, status, confidence, evidence, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			h.ID,
			sessionID,
			string(h.ProposedBy),
			h.Description,
			h.Status,
			h.Confidence,
			evidenceCombined,
			now.Format(time.RFC3339Nano),
			now.Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("warroom: save hypothesis: %w", err)
		}
	}

	facts := bb.GetSharedFacts()
	for _, f := range facts {
		confirmingJSON, err := json.Marshal(f.ConfirmedBy)
		if err != nil {
			return fmt.Errorf("warroom: marshal confirming agents: %w", err)
		}

		_, err = tx.Exec(`
			INSERT INTO warroom_shared_facts (session_id, content, source_agent, confirmation_count, confirming_agents, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			sessionID,
			f.Content,
			string(f.Source),
			len(f.ConfirmedBy),
			string(confirmingJSON),
			time.Now().Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("warroom: save shared fact: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("warroom: commit blackboard: %w", err)
	}

	return nil
}

func (s *SQLiteWarRoomStore) LoadBlackboard(sessionID string) (*Blackboard, error) {
	bb := NewBlackboard(sessionID)

	entryRows, err := s.db.Query(`
		SELECT agent_type, content, entry_type, timestamp
		FROM warroom_blackboard_entries WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("warroom: load blackboard entries: %w", err)
	}
	defer entryRows.Close()

	for entryRows.Next() {
		var agentType, content, entryType, ts string
		if err := entryRows.Scan(&agentType, &content, &entryType, &ts); err != nil {
			return nil, fmt.Errorf("warroom: scan blackboard entry: %w", err)
		}

		var row blackboardEntryRow
		if err := json.Unmarshal([]byte(content), &row); err != nil {
			continue
		}

		var t time.Time
		if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			t = parsed
		}

		bb.entries[row.Key] = BlackboardEntry{
			Key:       row.Key,
			Value:     row.Value,
			Author:    DomainAgentType(agentType),
			Category:  entryType,
			Timestamp: t,
		}
	}

	hypRows, err := s.db.Query(`
		SELECT id, agent_type, content, status, confidence, evidence
		FROM warroom_hypotheses WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("warroom: load hypotheses: %w", err)
	}
	defer hypRows.Close()

	for hypRows.Next() {
		var h Hypothesis
		var agentType, evidenceCombined string
		if err := hypRows.Scan(&h.ID, &agentType, &h.Description, &h.Status, &h.Confidence, &evidenceCombined); err != nil {
			return nil, fmt.Errorf("warroom: scan hypothesis: %w", err)
		}
		h.ProposedBy = DomainAgentType(agentType)

		parts := splitNullSeparated(evidenceCombined)
		if len(parts) >= 1 {
			json.Unmarshal([]byte(parts[0]), &h.SupportingEvidence)
		}
		if len(parts) >= 2 {
			json.Unmarshal([]byte(parts[1]), &h.ContradictingEvidence)
		}
		if h.SupportingEvidence == nil {
			h.SupportingEvidence = []string{}
		}
		if h.ContradictingEvidence == nil {
			h.ContradictingEvidence = []string{}
		}

		bb.hypotheses = append(bb.hypotheses, h)
	}

	factRows, err := s.db.Query(`
		SELECT content, source_agent, confirmation_count, confirming_agents
		FROM warroom_shared_facts WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("warroom: load shared facts: %w", err)
	}
	defer factRows.Close()

	for factRows.Next() {
		var f SharedFact
		var sourceAgent, confirmingJSON string
		var confCount int
		if err := factRows.Scan(&f.Content, &sourceAgent, &confCount, &confirmingJSON); err != nil {
			return nil, fmt.Errorf("warroom: scan shared fact: %w", err)
		}
		f.Source = DomainAgentType(sourceAgent)
		json.Unmarshal([]byte(confirmingJSON), &f.ConfirmedBy)
		if f.ConfirmedBy == nil {
			f.ConfirmedBy = []DomainAgentType{}
		}
		f.Confidence = float64(confCount) / 2.0
		if f.Confidence > 1.0 {
			f.Confidence = 1.0
		}

		bb.sharedFacts = append(bb.sharedFacts, f)
	}

	return bb, nil
}

func splitNullSeparated(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\x00' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
