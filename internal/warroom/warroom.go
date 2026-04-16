package warroom

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type AgentRunner interface {
	RunAgent(ctx context.Context, agentType DomainAgentType, task string, tools []string) (string, error)
}

type WarRoomCoordinator struct {
	mu       sync.RWMutex
	sessions map[string]*WarRoomSession
	channels map[string]map[DomainAgentType]chan AgentMessage
	findings map[string]chan AgentMessage
	cancels  map[string]context.CancelFunc
	runner   AgentRunner
}

func NewWarRoomCoordinator() *WarRoomCoordinator {
	return &WarRoomCoordinator{
		sessions: make(map[string]*WarRoomSession),
		channels: make(map[string]map[DomainAgentType]chan AgentMessage),
		findings: make(map[string]chan AgentMessage),
		cancels:  make(map[string]context.CancelFunc),
		runner:   nil,
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
	c.mu.Unlock()

	metricWarRoomSessionsActive.Inc()

	for _, at := range agentTypes {
		go c.runAgent(ctx, sessionID, at)
	}

	go c.processFindings(ctx, sessionID)

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
	c.mu.Unlock()

	metricWarRoomSessionsActive.Dec()

	result := c.buildInvestigationResult(session)
	return result, nil
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

					result, err := runner.RunAgent(ctx, agentType, taskPrompt, agent.Tools)

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
