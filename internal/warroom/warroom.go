package warroom

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/instructkr/smartclaw/internal/api"
)

type AgentRunner interface {
	RunAgent(ctx context.Context, agentType DomainAgentType, task string, tools []string, opts ...RunAgentOptions) (string, error)
}

type WarRoomCoordinator struct {
	mu          sync.RWMutex
	sessions    map[string]*WarRoomSession
	channels    map[string]map[DomainAgentType]chan AgentMessage
	findings    map[string]chan AgentMessage
	cancels     map[string]context.CancelFunc
	runner      AgentRunner
	executor    *StagedExecutor
	dispatcher  *Dispatcher
	maxSessions int
	blackboards map[string]*Blackboard
	handoff     *HandoffManager
}

func NewWarRoomCoordinator() *WarRoomCoordinator {
	return &WarRoomCoordinator{
		sessions:    make(map[string]*WarRoomSession),
		channels:    make(map[string]map[DomainAgentType]chan AgentMessage),
		findings:    make(map[string]chan AgentMessage),
		cancels:     make(map[string]context.CancelFunc),
		runner:      nil,
		maxSessions: 50,
		blackboards: make(map[string]*Blackboard),
		handoff:     NewHandoffManager(),
	}
}

func Shutdown() {
	defaultCoordinatorMu.RLock()
	c := defaultCoordinator
	defaultCoordinatorMu.RUnlock()
	if c == nil {
		return
	}
	c.mu.Lock()
	ids := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		ids = append(ids, id)
	}
	c.mu.Unlock()
	for _, id := range ids {
		c.CloseSession(id)
	}
}

func (c *WarRoomCoordinator) SetAgentRunner(runner AgentRunner) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runner = runner
}

func (c *WarRoomCoordinator) getRunner() AgentRunner {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runner
}

func (c *WarRoomCoordinator) StartWarRoom(ctx context.Context, req WarRoomRequest) (*WarRoomSession, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if req.Description == "" {
		return nil, fmt.Errorf("description is required")
	}

	agentTypes := req.AgentTypes
	if len(agentTypes) == 0 {
		agentTypes = AllAgentTypes()
	}

	for _, at := range agentTypes {
		if _, ok := BuiltInAgents[at]; !ok {
			return nil, fmt.Errorf("unknown agent type: %s", at)
		}
	}

	c.mu.Lock()
	if c.maxSessions > 0 && len(c.sessions) >= c.maxSessions {
		var oldestID string
		var oldestTime time.Time
		for id, s := range c.sessions {
			if oldestID == "" || s.CreatedAt.Before(oldestTime) {
				oldestID = id
				oldestTime = s.CreatedAt
			}
		}
		if oldestID != "" {
			c.cleanupSessionLocked(oldestID)
		}
	}
	c.mu.Unlock()

	sessionID := uuid.New().String()
	now := time.Now()

	ctx, cancel := context.WithCancel(ctx)

	assignments := make([]AgentAssignment, 0, len(agentTypes))
	agentChs := make(map[DomainAgentType]chan AgentMessage, len(agentTypes))

	for _, at := range agentTypes {
		assignments = append(assignments, AgentAssignment{
			AgentType:  at,
			Status:     AgentStatusSpawning,
			AssignedAt: now,
			LastActive: now,
		})
		agentChs[at] = make(chan AgentMessage, config.ChannelBufferSize)
	}

	findingsCh := make(chan AgentMessage, config.ChannelBufferSize*len(agentTypes))

	session := &WarRoomSession{
		ID:          sessionID,
		IncidentID:  req.IncidentID,
		Title:       req.Title,
		Description: req.Description,
		Status:      WarRoomActive,
		Agents:      assignments,
		Findings:    []Finding{},
		Timeline: []TimelineEntry{
			{
				Timestamp: now,
				Event:     "war_room_started",
				Details:   fmt.Sprintf("War room started with %d agents: %s", len(agentTypes), joinAgentTypes(agentTypes)),
			},
		},
		CreatedAt: now,
		Context:   req.Context,
	}
	if session.Context == nil {
		session.Context = make(map[string]any)
	}

	c.mu.Lock()
	c.sessions[sessionID] = session
	c.channels[sessionID] = agentChs
	c.findings[sessionID] = findingsCh
	c.cancels[sessionID] = cancel
	c.blackboards[sessionID] = NewBlackboard(sessionID)
	c.handoff.CreateSession(sessionID)
	c.mu.Unlock()

	metricWarRoomSessionsActive.Inc()

	go c.processFindings(ctx, sessionID)

	if c.executor != nil {
		go c.executor.ExecuteStaged(ctx, sessionID, req.Description)
	} else {
		for _, at := range agentTypes {
			go c.runAgent(ctx, sessionID, at)
		}
	}

	return session, nil
}

func (c *WarRoomCoordinator) AssignTask(ctx context.Context, sessionID string, agentType DomainAgentType, task string) error {
	c.mu.RLock()
	agentChs, ok := c.channels[sessionID]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	ch, ok := agentChs[agentType]
	if !ok {
		return fmt.Errorf("agent %s not assigned to session %s", agentType, sessionID)
	}

	msg := AgentMessage{
		SessionID: sessionID,
		AgentType: agentType,
		Type:      MsgTask,
		Content:   task,
	}

	select {
	case ch <- msg:
		c.mu.Lock()
		if s, exists := c.sessions[sessionID]; exists {
			s.Timeline = append(s.Timeline, TimelineEntry{
				Timestamp: time.Now(),
				AgentType: agentType,
				Event:     "task_assigned",
				Details:   task,
			})
		}
		c.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *WarRoomCoordinator) Broadcast(ctx context.Context, sessionID string, message string) error {
	c.mu.RLock()
	agentChs, ok := c.channels[sessionID]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	for at, ch := range agentChs {
		msg := AgentMessage{
			SessionID: sessionID,
			AgentType: at,
			Type:      MsgBroadcast,
			Content:   message,
		}
		select {
		case ch <- msg:
		default:
		}
	}

	c.mu.Lock()
	if s, exists := c.sessions[sessionID]; exists {
		s.Timeline = append(s.Timeline, TimelineEntry{
			Timestamp: time.Now(),
			Event:     "broadcast",
			Details:   message,
		})
	}
	c.mu.Unlock()

	return nil
}

func (c *WarRoomCoordinator) SubmitFinding(sessionID string, agentType DomainAgentType, finding Finding) error {
	c.mu.Lock()
	s, exists := c.sessions[sessionID]
	if !exists {
		c.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}

	c.crossValidateFinding(s, &finding, agentType)

	s.Findings = append(s.Findings, finding)
	for i := range s.Agents {
		if s.Agents[i].AgentType == agentType {
			s.Agents[i].Findings = append(s.Agents[i].Findings, finding)
			s.Agents[i].LastActive = time.Now()
			break
		}
	}
	s.Timeline = append(s.Timeline, TimelineEntry{
		Timestamp: time.Now(),
		AgentType: agentType,
		Event:     "finding_submitted",
		Details:   finding.Title,
	})
	c.mu.Unlock()

	if bb, ok := c.GetBlackboard(sessionID); ok {
		bb.WriteEntry(BlackboardEntry{
			Key:      fmt.Sprintf("%s_%s", agentType, finding.Category),
			Value:    fmt.Sprintf("%s: %s", finding.Title, finding.Description),
			Author:   agentType,
			Category: finding.Category,
		})
	}

	metricWarRoomFindings.Inc()

	c.mu.RLock()
	findingsCh := c.findings[sessionID]
	c.mu.RUnlock()

	if findingsCh != nil {
		msg := AgentMessage{
			SessionID: sessionID,
			AgentType: agentType,
			Type:      MsgFinding,
			Finding:   &finding,
		}
		select {
		case findingsCh <- msg:
		default:
		}
	}

	return nil
}

func (c *WarRoomCoordinator) GetSession(sessionID string) *WarRoomSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessions[sessionID]
}

func (c *WarRoomCoordinator) GetBlackboard(sessionID string) (*Blackboard, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	bb, ok := c.blackboards[sessionID]
	return bb, ok
}

func (c *WarRoomCoordinator) RequestHandoff(ctx context.Context, sessionID string, req HandoffRequest) (*HandoffResponse, error) {
	return c.handoff.RequestHandoff(ctx, sessionID, req)
}

func (c *WarRoomCoordinator) SendHandoffResponse(sessionID string, resp HandoffResponse) error {
	return c.handoff.SendResponse(sessionID, resp)
}

func (c *WarRoomCoordinator) TryRecvHandoff(sessionID string) (HandoffRequest, bool) {
	return c.handoff.TryRecvRequest(sessionID)
}

func (c *WarRoomCoordinator) crossValidateFinding(s *WarRoomSession, finding *Finding, authorType DomainAgentType) {
	for i := range s.Findings {
		existing := &s.Findings[i]
		if existing.AgentType == authorType {
			continue
		}
		if existing.Category != finding.Category && !evidenceOverlap(existing.Evidence, finding.Evidence) {
			continue
		}

		agrees := evidenceOverlap(existing.Evidence, finding.Evidence) ||
			keywordOverlap(existing.Description, finding.Description)

		xref := CrossReference{
			FindingID:    existing.ID,
			ReferencedBy: authorType,
			Agrees:       agrees,
			Notes:        fmt.Sprintf("Cross-validated with %s finding: %s", existing.AgentType, existing.Title),
		}
		finding.CrossReferences = append(finding.CrossReferences, xref)

		reverseXref := CrossReference{
			FindingID:    finding.ID,
			ReferencedBy: existing.AgentType,
			Agrees:       agrees,
			Notes:        fmt.Sprintf("Cross-validated with %s finding: %s", authorType, finding.Title),
		}
		existing.CrossReferences = append(existing.CrossReferences, reverseXref)

		if agrees {
			delta := 0.05
			if finding.Confidence+delta > 0.95 {
				delta = 0.95 - finding.Confidence
			}
			finding.Confidence += delta

			existingDelta := 0.05
			if existing.Confidence+existingDelta > 0.95 {
				existingDelta = 0.95 - existing.Confidence
			}
			existing.Confidence += existingDelta
		} else {
			delta := 0.05
			if finding.Confidence-delta < 0.1 {
				delta = finding.Confidence - 0.1
			}
			finding.Confidence -= delta

			existingDelta := 0.05
			if existing.Confidence-existingDelta < 0.1 {
				existingDelta = existing.Confidence - 0.1
			}
			existing.Confidence -= existingDelta
		}
	}
}

func (c *WarRoomCoordinator) EvolveConfidence(sessionID string, findingID string, delta float64, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	s, exists := c.sessions[sessionID]
	if !exists {
		return
	}

	for i := range s.Findings {
		if s.Findings[i].ID == findingID {
			s.Findings[i].Confidence += delta
			if s.Findings[i].Confidence > 0.95 {
				s.Findings[i].Confidence = 0.95
			}
			if s.Findings[i].Confidence < 0.1 {
				s.Findings[i].Confidence = 0.1
			}
			s.Timeline = append(s.Timeline, TimelineEntry{
				Timestamp: time.Now(),
				Event:     "confidence_evolved",
				Details:   fmt.Sprintf("Finding %s confidence adjusted by %.2f: %s", findingID, delta, reason),
			})
			return
		}
	}
}

func evidenceOverlap(a, b []string) bool {
	for _, ea := range a {
		for _, eb := range b {
			if ea == eb {
				return true
			}
			if len(ea) > 10 && len(eb) > 10 {
				aLower := strings.ToLower(ea)
				bLower := strings.ToLower(eb)
				if strings.Contains(aLower, bLower) || strings.Contains(bLower, aLower) {
					return true
				}
			}
		}
	}
	return false
}

func keywordOverlap(a, b string) bool {
	aLower := strings.ToLower(a)
	bLower := strings.ToLower(b)

	keywords := []string{"error", "fail", "timeout", "crash", "oom", "latency", "down", "slow", "refused", "unavailable"}
	aMatches := 0
	bMatches := 0
	for _, kw := range keywords {
		if strings.Contains(aLower, kw) {
			aMatches++
		}
		if strings.Contains(bLower, kw) {
			bMatches++
		}
	}

	common := 0
	for _, kw := range keywords {
		if strings.Contains(aLower, kw) && strings.Contains(bLower, kw) {
			common++
		}
	}

	return common >= 2 || (aMatches > 0 && bMatches > 0 && common > 0)
}

func (c *WarRoomCoordinator) ListSessions() []*WarRoomSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*WarRoomSession, 0, len(c.sessions))
	for _, s := range c.sessions {
		result = append(result, s)
	}
	return result
}

func (c *WarRoomCoordinator) CloseSession(sessionID string) (*InvestigationResult, error) {
	c.mu.Lock()
	session, exists := c.sessions[sessionID]
	if !exists {
		c.mu.Unlock()
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if cancel, ok := c.cancels[sessionID]; ok {
		cancel()
		delete(c.cancels, sessionID)
	}

	now := time.Now()
	session.Status = WarRoomClosed
	session.ClosedAt = &now
	session.Timeline = append(session.Timeline, TimelineEntry{
		Timestamp: now,
		Event:     "war_room_closed",
	})

	if findingsCh, ok := c.findings[sessionID]; ok {
		close(findingsCh)
		delete(c.findings, sessionID)
	}

	for at, ch := range c.channels[sessionID] {
		close(ch)
		_ = at
	}
	delete(c.channels, sessionID)
	delete(c.blackboards, sessionID)
	c.handoff.CloseSession(sessionID)
	c.mu.Unlock()

	metricWarRoomSessionsActive.Dec()

	result := c.buildInvestigationResult(session)
	return result, nil
}

func (c *WarRoomCoordinator) cleanupSessionLocked(sessionID string) {
	if cancel, ok := c.cancels[sessionID]; ok {
		cancel()
		delete(c.cancels, sessionID)
	}

	if session, ok := c.sessions[sessionID]; ok {
		now := time.Now()
		session.Status = WarRoomClosed
		session.ClosedAt = &now
	}

	if findingsCh, ok := c.findings[sessionID]; ok {
		close(findingsCh)
		delete(c.findings, sessionID)
	}

	for at, ch := range c.channels[sessionID] {
		close(ch)
		_ = at
	}
	delete(c.channels, sessionID)
	delete(c.blackboards, sessionID)
	c.handoff.CloseSession(sessionID)
	delete(c.sessions, sessionID)

	metricWarRoomSessionsActive.Dec()
}

func (c *WarRoomCoordinator) buildInvestigationResult(session *WarRoomSession) *InvestigationResult {
	var rootCause *Finding
	for i := range session.Findings {
		f := &session.Findings[i]
		if f.Category == "root_cause" {
			if rootCause == nil || f.Confidence > rootCause.Confidence {
				rootCause = f
			}
		}
	}

	agentReports := make(map[DomainAgentType]string, len(session.Agents))
	for _, a := range session.Agents {
		if len(a.Findings) == 0 {
			agentReports[a.AgentType] = "No findings reported"
			continue
		}
		var sb strings.Builder
		for i, f := range a.Findings {
			if i > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(f.Title)
		}
		agentReports[a.AgentType] = sb.String()
	}

	recommendations := generateRecommendations(session.Findings)

	duration := time.Since(session.CreatedAt)
	if session.ClosedAt != nil {
		duration = session.ClosedAt.Sub(session.CreatedAt)
	}

	summary := fmt.Sprintf("War room '%s' completed with %d findings across %d agents",
		session.Title, len(session.Findings), len(session.Agents))
	if rootCause != nil {
		summary += fmt.Sprintf(". Root cause identified: %s (confidence: %.1f%%)",
			rootCause.Title, rootCause.Confidence*100)
	}

	return &InvestigationResult{
		SessionID:       session.ID,
		Summary:         summary,
		RootCause:       rootCause,
		AllFindings:     session.Findings,
		AgentReports:    agentReports,
		Recommendations: recommendations,
		Duration:        duration,
	}
}

func (c *WarRoomCoordinator) runAgent(ctx context.Context, sessionID string, agentType DomainAgentType) {
	agent, ok := BuiltInAgents[agentType]
	if !ok {
		return
	}

	c.mu.Lock()
	if s, exists := c.sessions[sessionID]; exists {
		for i := range s.Agents {
			if s.Agents[i].AgentType == agentType {
				s.Agents[i].Status = AgentStatusRunning
				break
			}
		}
		s.Timeline = append(s.Timeline, TimelineEntry{
			Timestamp: time.Now(),
			AgentType: agentType,
			Event:     "agent_started",
			Details:   agent.Name + " is now running",
		})
	}
	c.mu.Unlock()

	c.mu.RLock()
	agentChs := c.channels[sessionID]
	findingsCh := c.findings[sessionID]
	c.mu.RUnlock()

	if agentChs == nil {
		return
	}
	ch, ok := agentChs[agentType]
	if !ok {
		return
	}

	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			if s, exists := c.sessions[sessionID]; exists {
				for i := range s.Agents {
					if s.Agents[i].AgentType == agentType {
						s.Agents[i].Status = AgentStatusComplete
						break
					}
				}
			}
			c.mu.Unlock()
			return

		case msg, ok := <-ch:
			if !ok {
				return
			}

			switch msg.Type {
			case MsgTask, MsgBroadcast:
				c.mu.Lock()
				if s, exists := c.sessions[sessionID]; exists {
					for i := range s.Agents {
						if s.Agents[i].AgentType == agentType {
							s.Agents[i].Status = AgentStatusRunning
							s.Agents[i].LastActive = time.Now()
							break
						}
					}
				}
				c.mu.Unlock()

				runner := c.getRunner()
				if runner != nil && msg.Type == MsgTask {
					taskPrompt := fmt.Sprintf("Investigation for: %s\n\nSteps:\n%s\n\nContext: %s",
						msg.Content,
						strings.Join(agent.InvestigationSteps, "\n"),
						sessionDescription(c, sessionID),
					)

					result, err := runner.RunAgent(ctx, agentType, taskPrompt, agent.Tools, RunAgentOptions{
					SessionID:            sessionID,
					BlackboardSnapshotFn: func() string { return c.getBlackboardSnapshot(sessionID) },
				})

					c.mu.Lock()
					if s, exists := c.sessions[sessionID]; exists {
						if err != nil {
							for i := range s.Agents {
								if s.Agents[i].AgentType == agentType {
									s.Agents[i].Status = AgentStatusFailed
									break
								}
							}
							s.Timeline = append(s.Timeline, TimelineEntry{
								Timestamp: time.Now(),
								AgentType: agentType,
								Event:     "agent_failed",
								Details:   err.Error(),
							})
						} else if result != "" {
							finding := Finding{
								ID:          uuid.New().String(),
								AgentType:   agentType,
								Category:    "symptom",
								Title:       fmt.Sprintf("%s finding", agent.Name),
								Description: result,
								Confidence:  0.5,
								Evidence:    []string{result},
								CreatedAt:   time.Now(),
							}
							s.Findings = append(s.Findings, finding)
							for i := range s.Agents {
								if s.Agents[i].AgentType == agentType {
									s.Agents[i].Findings = append(s.Agents[i].Findings, finding)
									s.Agents[i].LastActive = time.Now()
									break
								}
							}
							s.Timeline = append(s.Timeline, TimelineEntry{
								Timestamp: time.Now(),
								AgentType: agentType,
								Event:     "finding_submitted",
								Details:   finding.Title,
							})
						}
					}
					c.mu.Unlock()

					if result != "" && findingsCh != nil {
						finding := Finding{
							ID:          uuid.New().String(),
							AgentType:   agentType,
							Category:    "symptom",
							Title:       fmt.Sprintf("%s finding", agent.Name),
							Description: result,
							Confidence:  0.5,
							Evidence:    []string{result},
							CreatedAt:   time.Now(),
						}
						findingsCh <- AgentMessage{
							SessionID: sessionID,
							AgentType: agentType,
							Type:      MsgFinding,
							Finding:   &finding,
						}
					}
				}

			case MsgAnswer:
				c.mu.Lock()
				if s, exists := c.sessions[sessionID]; exists {
					for i := range s.Agents {
						if s.Agents[i].AgentType == agentType {
							s.Agents[i].LastActive = time.Now()
							break
						}
					}
				}
				c.mu.Unlock()
			}
		}
	}
}

func (c *WarRoomCoordinator) processFindings(ctx context.Context, sessionID string) {
	c.mu.RLock()
	findingsCh, ok := c.findings[sessionID]
	c.mu.RUnlock()
	if !ok {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-findingsCh:
			if !ok {
				return
			}
			if msg.Finding == nil {
				continue
			}

			c.mu.RLock()
			agentChs := c.channels[sessionID]
			c.mu.RUnlock()

			if agentChs == nil {
				continue
			}

			for at, ch := range agentChs {
				if at == msg.AgentType {
					continue
				}
				broadcast := AgentMessage{
					SessionID: sessionID,
					AgentType: at,
					Type:      MsgBroadcast,
					Content:   fmt.Sprintf("New finding from %s: %s", msg.AgentType, msg.Finding.Title),
					Finding:   msg.Finding,
				}
				select {
				case ch <- broadcast:
				default:
				}
			}
		}
	}
}

func sessionDescription(c *WarRoomCoordinator, sessionID string) string {
	c.mu.RLock()
	s, exists := c.sessions[sessionID]
	c.mu.RUnlock()
	if !exists {
		return ""
	}
	return s.Description
}

func (c *WarRoomCoordinator) getBlackboardSnapshot(sessionID string) string {
	c.mu.RLock()
	bb, ok := c.blackboards[sessionID]
	c.mu.RUnlock()
	if !ok || bb == nil {
		return ""
	}
	return bb.GetSnapshot()
}

func joinAgentTypes(types []DomainAgentType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

func generateRecommendations(findings []Finding) []string {
	categorySet := make(map[string]bool)
	for _, f := range findings {
		categorySet[f.Category] = true
	}

	var recs []string
	if categorySet["root_cause"] {
		recs = append(recs, "Address the identified root cause as the highest priority")
	}
	if categorySet["symptom"] {
		recs = append(recs, "Monitor related symptoms to verify resolution after fix is applied")
	}
	if categorySet["dependency"] {
		recs = append(recs, "Review dependency health and consider adding circuit breakers or fallbacks")
	}
	if categorySet["config"] {
		recs = append(recs, "Audit configuration changes and validate against best practices")
	}
	if categorySet["metric"] {
		recs = append(recs, "Set up alerting on the identified metric anomalies")
	}
	if categorySet["log"] {
		recs = append(recs, "Improve log coverage for the affected components to aid future investigations")
	}
	if len(recs) == 0 {
		recs = append(recs, "Continue monitoring and gather more evidence if the issue persists")
	}
	return recs
}

func InitWarRoom(agentRunner AgentRunner) *WarRoomCoordinator {
	c := NewWarRoomCoordinator()
	if agentRunner != nil {
		c.SetAgentRunner(agentRunner)
	}
	SetWarRoomCoordinator(c)
	return c
}

func InitWarRoomWithLLM(client interface {
	CreateMessageWithSystem(ctx context.Context, messages []api.MessageParam, system any) (*api.MessageResponse, error)
	GetModel() string
}) *WarRoomCoordinator {
	apiClient := (*api.Client)(nil)
	if client != nil {
		if ac, ok := client.(*api.Client); ok {
			apiClient = ac
		}
	}

	c := NewWarRoomCoordinator()

	if apiClient != nil {
		runner := NewLLMAgentRunner(apiClient)
		c.SetAgentRunner(runner)
		dispatcher := NewDispatcher(runner)
		executor := NewStagedExecutor(dispatcher, c)
		c.dispatcher = dispatcher
		c.executor = executor
		runner.SetCoordinator(c)
	}

	RegisterAllTools()
	SetWarRoomCoordinator(c)
	return c
}
