package warroom

import (
	"time"
)

type WarRoomStatus string

const (
	WarRoomActive   WarRoomStatus = "active"
	WarRoomPaused   WarRoomStatus = "paused"
	WarRoomResolved WarRoomStatus = "resolved"
	WarRoomClosed   WarRoomStatus = "closed"
)

type AgentStatus string

const (
	AgentStatusSpawning AgentStatus = "spawning"
	AgentStatusRunning  AgentStatus = "running"
	AgentStatusWaiting  AgentStatus = "waiting"
	AgentStatusComplete AgentStatus = "complete"
	AgentStatusFailed   AgentStatus = "failed"
)

type DomainAgentType string

const (
	AgentNetwork   DomainAgentType = "network"
	AgentDatabase  DomainAgentType = "database"
	AgentInfra     DomainAgentType = "infra"
	AgentApp       DomainAgentType = "app"
	AgentSecurity  DomainAgentType = "security"
	AgentReasoning DomainAgentType = "reasoning"
	AgentTraining  DomainAgentType = "training"
	AgentInference DomainAgentType = "inference"
)

type MessageType string

const (
	MsgTask      MessageType = "task"      // coordinator → agent: here's your task
	MsgFinding   MessageType = "finding"   // agent → coordinator: I found something
	MsgAnswer    MessageType = "answer"    // coordinator → agent: here's more info
	MsgBroadcast MessageType = "broadcast" // coordinator → all agents: shared update
)

type WarRoomSession struct {
	ID          string            `json:"id"`
	IncidentID  string            `json:"incident_id,omitempty"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      WarRoomStatus     `json:"status"`
	Agents      []AgentAssignment `json:"agents"`
	Findings    []Finding         `json:"findings"`
	Timeline    []TimelineEntry   `json:"timeline"`
	CreatedAt   time.Time         `json:"created_at"`
	ClosedAt    *time.Time        `json:"closed_at,omitempty"`
	Context     map[string]any    `json:"context"`
}

type AgentAssignment struct {
	AgentType  DomainAgentType `json:"agent_type"`
	Status     AgentStatus     `json:"status"`
	AssignedAt time.Time       `json:"assigned_at"`
	LastActive time.Time       `json:"last_active"`
	Findings   []Finding       `json:"findings"`
}

type Finding struct {
	ID          string          `json:"id"`
	AgentType   DomainAgentType `json:"agent_type"`
	Category    string          `json:"category"` // root_cause, symptom, dependency, metric, log, config, hypothesis
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Confidence  float64         `json:"confidence"` // 0.0-1.0
	Evidence    []string        `json:"evidence"`
	CreatedAt   time.Time       `json:"created_at"`
}

type TimelineEntry struct {
	Timestamp time.Time       `json:"timestamp"`
	AgentType DomainAgentType `json:"agent_type,omitempty"`
	Event     string          `json:"event"`
	Details   string          `json:"details,omitempty"`
}

type AgentMessage struct {
	SessionID string          `json:"session_id"`
	AgentType DomainAgentType `json:"agent_type"`
	Type      MessageType     `json:"type"`
	Content   string          `json:"content"`
	Finding   *Finding        `json:"finding,omitempty"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
}

type InvestigationResult struct {
	SessionID       string                     `json:"session_id"`
	Summary         string                     `json:"summary"`
	RootCause       *Finding                   `json:"root_cause,omitempty"`
	AllFindings     []Finding                  `json:"all_findings"`
	AgentReports    map[DomainAgentType]string `json:"agent_reports"`
	Recommendations []string                   `json:"recommendations"`
	Duration        time.Duration              `json:"duration"`
}

type WarRoomRequest struct {
	IncidentID  string            `json:"incident_id,omitempty"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	AgentTypes  []DomainAgentType `json:"agent_types"` // which agents to include (default: all 5)
	Context     map[string]any    `json:"context"`
}
