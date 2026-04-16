package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/alertengine"
	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/autoremediation"
	"github.com/instructkr/smartclaw/internal/changerisk"
	"github.com/instructkr/smartclaw/internal/fingerprint"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/operator"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/timetravel"
	"github.com/instructkr/smartclaw/internal/tools"
	"github.com/instructkr/smartclaw/internal/topology"
	"github.com/instructkr/smartclaw/internal/warroom"
)

type TopologyAdapter struct {
	Graph *topology.TopologyGraph
}

func (a TopologyAdapter) GetNeighbors(serviceID string, depth int) ([]string, error) {
	if a.Graph == nil {
		return nil, fmt.Errorf("topology not initialized")
	}
	nodes, _ := a.Graph.GetNeighbors(serviceID, depth)
	services := make([]string, len(nodes))
	for i, n := range nodes {
		services[i] = n.ID
	}
	return services, nil
}

func (a TopologyAdapter) GetNodeHealth(serviceID string) (string, error) {
	if a.Graph == nil {
		return "unknown", fmt.Errorf("topology not initialized")
	}
	node := a.Graph.GetNode(serviceID)
	if node == nil {
		return "unknown", nil
	}
	return string(node.Health), nil
}

type IncidentAdapter struct {
	IM *layers.IncidentMemory
}

func (a IncidentAdapter) GetRecentIncidents(service string, since time.Time) ([]changerisk.IncidentInfo, error) {
	if a.IM == nil {
		return nil, nil
	}
	incidents, err := a.IM.ListIncidentsByService(service)
	if err != nil {
		return nil, err
	}
	var result []changerisk.IncidentInfo
	for _, inc := range incidents {
		if inc.StartedAt.After(since) {
			result = append(result, changerisk.IncidentInfo{
				ID:        inc.ID,
				Title:     inc.Title,
				Severity:  inc.Severity,
				Service:   inc.Service,
				Status:    inc.Status,
				StartedAt: inc.StartedAt,
			})
		}
	}
	return result, nil
}

func (a IncidentAdapter) GetSLOStatus(service string) (*changerisk.SLOInfo, error) {
	if a.IM == nil {
		return nil, nil
	}
	statuses, err := a.IM.GetSLOStatuses()
	if err != nil {
		return nil, err
	}
	for _, s := range statuses {
		if s.Service == service {
			return &changerisk.SLOInfo{
				Service:              s.Service,
				SLOName:              s.SLOName,
				Target:               s.Target,
				Current:              s.Current,
				ErrorBudgetRemaining: s.ErrorBudgetRemaining,
				BurnRate:             s.BurnRate,
			}, nil
		}
	}
	return nil, nil
}

type SLOProviderAdapter struct {
	IM *layers.IncidentMemory
}

func (a SLOProviderAdapter) GetSLOStatus(service string) (*autoremediation.SLOInfo, error) {
	if a.IM == nil {
		return nil, nil
	}
	statuses, err := a.IM.GetSLOStatuses()
	if err != nil {
		return nil, err
	}
	for _, s := range statuses {
		if s.Service == service {
			return &autoremediation.SLOInfo{
				Service:              s.Service,
				SLOName:              s.SLOName,
				Target:               s.Target,
				Current:              s.Current,
				ErrorBudgetRemaining: s.ErrorBudgetRemaining,
				BurnRate:             s.BurnRate,
			}, nil
		}
	}
	return nil, nil
}

type CommanderAdapter struct{}

func (a CommanderAdapter) ExecuteCommand(_ context.Context, command string, _ time.Duration) (string, error) {
	return fmt.Sprintf("command queued: %s", command), nil
}

func (a CommanderAdapter) ExecuteTool(_ context.Context, toolName string, params map[string]any) (any, error) {
	return fmt.Sprintf("tool %s queued with params: %v", toolName, params), nil
}

type CronSchedulerAdapter struct {
	entries map[string]func()
}

func NewCronSchedulerAdapter() *CronSchedulerAdapter {
	return &CronSchedulerAdapter{entries: make(map[string]func())}
}

func (a *CronSchedulerAdapter) ScheduleCron(id string, _ string, fn func()) error {
	a.entries[id] = fn
	return nil
}

func (a *CronSchedulerAdapter) UnscheduleCron(id string) error {
	delete(a.entries, id)
	return nil
}

type AlertProviderAdapter struct {
	Engine *alertengine.AlertEngine
}

func (a AlertProviderAdapter) GetActiveAlertCount(service string) (int, error) {
	if a.Engine == nil {
		return 0, nil
	}
	results := a.Engine.Query(service, "", time.Time{})
	return len(results), nil
}

type RecordingStoreAdapter struct {
	Store *store.Store
}

func (a RecordingStoreAdapter) LoadRecording(path string) ([]timetravel.RecordingEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []timetravel.RecordingEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (a RecordingStoreAdapter) ListRecordings() ([]string, error) {
	home, _ := os.UserHomeDir()
	recordingsDir := filepath.Join(home, ".smartclaw", "recordings")
	entries, err := os.ReadDir(recordingsDir)
	if err != nil {
		return nil, nil
	}
	var result []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			result = append(result, filepath.Join(recordingsDir, e.Name()))
		}
	}
	return result, nil
}

type TTIncidentStoreAdapter struct {
	IM *layers.IncidentMemory
}

func (a TTIncidentStoreAdapter) GetIncidentTimeline(incidentID string) ([]timetravel.TimelineEvent, error) {
	if a.IM == nil {
		return nil, nil
	}
	inc, err := a.IM.GetIncident(incidentID)
	if err != nil {
		return nil, err
	}
	if inc == nil {
		return nil, nil
	}
	var result []timetravel.TimelineEvent
	result = append(result, timetravel.TimelineEvent{
		Timestamp: inc.StartedAt,
		Type:      "incident_start",
		Content:   inc.Title,
		Source:    inc.AlertSource,
	})
	for _, ev := range inc.TimelineEvents {
		result = append(result, timetravel.TimelineEvent{
			Timestamp: ev.Timestamp,
			Type:      ev.Type,
			Content:   ev.Content,
			Source:    ev.Source,
		})
	}
	return result, nil
}

func (a TTIncidentStoreAdapter) GetIncident(incidentID string) (*timetravel.IncidentInfo, error) {
	if a.IM == nil {
		return nil, nil
	}
	inc, err := a.IM.GetIncident(incidentID)
	if err != nil {
		return nil, err
	}
	if inc == nil {
		return nil, nil
	}
	return &timetravel.IncidentInfo{
		ID:         inc.ID,
		Title:      inc.Title,
		Severity:   inc.Severity,
		Status:     inc.Status,
		Service:    inc.Service,
		StartedAt:  inc.StartedAt,
		ResolvedAt: inc.ResolvedAt,
	}, nil
}

type FPIncidentStoreAdapter struct {
	IM *layers.IncidentMemory
}

func (a FPIncidentStoreAdapter) GetIncident(id string) (*fingerprint.IncidentBrief, error) {
	if a.IM == nil {
		return nil, nil
	}
	inc, err := a.IM.GetIncident(id)
	if err != nil {
		return nil, err
	}
	if inc == nil {
		return nil, nil
	}
	return &fingerprint.IncidentBrief{
		ID:       inc.ID,
		Title:    inc.Title,
		Severity: inc.Severity,
		Service:  inc.Service,
	}, nil
}

func (a FPIncidentStoreAdapter) ListIncidents(limit int) ([]fingerprint.IncidentBrief, error) {
	if a.IM == nil {
		return nil, nil
	}
	incidents, err := a.IM.ListActiveIncidents()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(incidents) > limit {
		incidents = incidents[:limit]
	}
	var result []fingerprint.IncidentBrief
	for _, inc := range incidents {
		result = append(result, fingerprint.IncidentBrief{
			ID:       inc.ID,
			Title:    inc.Title,
			Severity: inc.Severity,
			Service:  inc.Service,
		})
	}
	return result, nil
}

type AgentRunnerAdapter struct {
	API *api.Client
}

func (a AgentRunnerAdapter) RunAgent(ctx context.Context, agentType warroom.DomainAgentType, task string, _ []string) (string, error) {
	if a.API == nil {
		return "", fmt.Errorf("API client not configured")
	}
	messages := []api.MessageParam{
		{Role: "user", Content: task},
	}
	system := fmt.Sprintf("You are a %s domain expert. Analyze the following and provide findings.", agentType)
	resp, err := a.API.CreateMessageCtx(ctx, messages, system)
	if err != nil {
		return "", err
	}
	if len(resp.Content) > 0 {
		return resp.Content[0].Text, nil
	}
	return "", nil
}

func InitInnovationPackages(mm *memory.MemoryManager, apiClient *api.Client) {
	var topoAdpt TopologyAdapter
	var healthChecker *operator.HealthChecker

	if mm != nil {
		topo := topology.InitTopology(mm.GetStore())
		topology.SetDefaultTopology(topo)

		topoAdpt = TopologyAdapter{Graph: topo}
		alertEngine := alertengine.InitAlertEngine(topoAdpt)
		alertengine.SetAlertEngine(alertEngine)

		incAdpt := IncidentAdapter{IM: mm.GetIncidentMemory()}
		riskChecker := changerisk.InitChangeRiskChecker(topoAdpt, incAdpt)
		changerisk.SetChangeRiskChecker(riskChecker)

		fpEngine, fpErr := fingerprint.InitFingerprintEngine(mm.GetStore().DB(), FPIncidentStoreAdapter{IM: mm.GetIncidentMemory()})
		if fpErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: fingerprint engine init failed: %v\n", fpErr)
		} else {
			fingerprint.SetFingerprintEngine(fpEngine)
		}

		healthChecker = operator.NewHealthChecker()
		healthChecker.SetTopologyProvider(topoAdpt)
		healthChecker.SetAlertProvider(AlertProviderAdapter{Engine: alertEngine})

		home, _ := os.UserHomeDir()
		runbookDir := filepath.Join(home, ".smartclaw", "runbooks")
		remediationEngine := autoremediation.InitRemediationEngine(runbookDir, SLOProviderAdapter{IM: mm.GetIncidentMemory()}, CommanderAdapter{})
		autoremediation.SetRemediationEngine(remediationEngine)

		ttEngine := timetravel.InitTimeTravelEngine(RecordingStoreAdapter{Store: mm.GetStore()}, TTIncidentStoreAdapter{IM: mm.GetIncidentMemory()})
		timetravel.SetTimeTravelEngine(ttEngine)
	} else {
		ttEngine := timetravel.InitTimeTravelEngine(nil, nil)
		timetravel.SetTimeTravelEngine(ttEngine)

		home, _ := os.UserHomeDir()
		runbookDir := filepath.Join(home, ".smartclaw", "runbooks")
		remediationEngine := autoremediation.InitRemediationEngine(runbookDir, nil, nil)
		autoremediation.SetRemediationEngine(remediationEngine)

		healthChecker = operator.NewHealthChecker()
	}

	warRoomCoord := warroom.InitWarRoom(AgentRunnerAdapter{API: apiClient})
	warroom.SetWarRoomCoordinator(warRoomCoord)

	opManager := operator.InitOperatorManager(healthChecker, NewCronSchedulerAdapter())
	operator.SetOperatorManager(opManager)

	topology.RegisterAllTools()
	alertengine.RegisterAllTools()
	changerisk.RegisterTools(tools.GetRegistry())
	operator.RegisterOperatorTools(tools.GetRegistry())
	fingerprint.RegisterTools(tools.GetRegistry())
	warroom.RegisterAllTools()
	autoremediation.RegisterAllTools()
	timetravel.RegisterAllTools()
}

func ShutdownInnovationPackages() {
	topology.StopAutoSnapshot()
	alertengine.Shutdown()
	changerisk.Shutdown()
	warroom.Shutdown()
	operator.Shutdown()
	autoremediation.Shutdown()
	timetravel.Shutdown()
	fingerprint.Shutdown()
}
