package operator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/instructkr/smartclaw/internal/autoremediation"
	"github.com/instructkr/smartclaw/internal/warroom"
)

type CronScheduler interface {
	ScheduleCron(id string, schedule string, fn func()) error
	UnscheduleCron(id string) error
}

type OperatorManager struct {
	mu              sync.RWMutex
	configs         map[string]*OperatorConfig
	statuses        map[string]*OperatorStatus
	checkResults    map[string][]HealthCheckResult
	events          map[string][]OperatorEvent
	escalationState map[string]map[int]bool

	healthChecker *HealthChecker

	stopChans map[string]chan struct{}
	running   bool

	cronScheduler   CronScheduler
	maxConfigs      int
	maxCheckResults int
	maxEvents       int
}

func NewOperatorManager() *OperatorManager {
	cfg := DefaultConfig()
	return &OperatorManager{
		configs:         make(map[string]*OperatorConfig),
		statuses:        make(map[string]*OperatorStatus),
		checkResults:    make(map[string][]HealthCheckResult),
		events:          make(map[string][]OperatorEvent),
		escalationState: make(map[string]map[int]bool),
		stopChans:       make(map[string]chan struct{}),
		maxConfigs:      100,
		maxCheckResults: cfg.MaxRecentResults,
		maxEvents:       cfg.MaxEvents,
	}
}

func (m *OperatorManager) SetHealthChecker(hc *HealthChecker) {
	m.healthChecker = hc
}

func (m *OperatorManager) SetCronScheduler(cs CronScheduler) {
	m.cronScheduler = cs
}

func (m *OperatorManager) Enable(ctx context.Context, config OperatorConfig) (*OperatorConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	config.Enabled = true

	if config.MaxAutoActions <= 0 {
		config.MaxAutoActions = 3
	}

	if config.AutonomyLevel == "" {
		config.AutonomyLevel = AutonomySuggest
	}

	if config.Schedule == "" {
		config.Schedule = "*/5 * * * *"
	}

	if config.EscalationPolicy.Levels == nil {
		config.EscalationPolicy = defaultEscalationPolicy()
	}

	if m.maxConfigs > 0 && len(m.configs) >= m.maxConfigs {
		var oldestID string
		var oldestTime time.Time
		for id, st := range m.statuses {
			if st.LastCheckCycle == nil || oldestID == "" || st.LastCheckCycle.Before(oldestTime) {
				oldestID = id
				if st.LastCheckCycle != nil {
					oldestTime = *st.LastCheckCycle
				}
			}
		}
		if oldestID != "" {
			_ = m.disableLocked(oldestID)
			delete(m.configs, oldestID)
			delete(m.statuses, oldestID)
			delete(m.checkResults, oldestID)
			delete(m.events, oldestID)
			delete(m.escalationState, oldestID)
		}
	}

	m.configs[config.ID] = &config

	m.statuses[config.ID] = &OperatorStatus{
		ConfigID:      config.ID,
		Enabled:       true,
		AutonomyLevel: config.AutonomyLevel,
		RecentResults: []HealthCheckResult{},
	}
	m.checkResults[config.ID] = []HealthCheckResult{}
	m.events[config.ID] = []OperatorEvent{}
	m.escalationState[config.ID] = make(map[int]bool)

	stopCh := make(chan struct{})
	m.stopChans[config.ID] = stopCh

	if m.cronScheduler != nil {
		cronID := "operator_main_" + config.ID
		cfgID := config.ID
		if err := m.cronScheduler.ScheduleCron(cronID, config.Schedule, func() {
			runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := m.RunCheckCycle(runCtx, cfgID); err != nil {
				slog.Warn("operator: check cycle failed", "config_id", cfgID, "error", err)
			}
		}); err != nil {
			slog.Warn("operator: failed to schedule main cron", "config_id", config.ID, "error", err)
		}

		for i := range config.HealthChecks {
			hc := &config.HealthChecks[i]
			if hc.Schedule != "" {
				checkCronID := fmt.Sprintf("operator_check_%s_%s", config.ID, hc.ID)
				checkID := hc.ID
				cID := config.ID
				if err := m.cronScheduler.ScheduleCron(checkCronID, hc.Schedule, func() {
					runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					if m.healthChecker != nil {
						result := m.healthChecker.ExecuteCheck(runCtx, *hc)
						if err := m.HandleCheckResult(cID, result); err != nil {
							slog.Warn("operator: handle check result failed", "config_id", cID, "check_id", checkID, "error", err)
						}
					}
				}); err != nil {
					slog.Warn("operator: failed to schedule check cron", "config_id", config.ID, "check_id", hc.ID, "error", err)
				}
			}
		}
	}

	go m.runSchedulerLoop(config.ID, config.Schedule, stopCh)

	m.appendEventLocked(config.ID, OperatorEvent{
		Timestamp: time.Now(),
		Type:      "operator_enabled",
		Details:   fmt.Sprintf("operator %s enabled with schedule %s", config.Name, config.Schedule),
		Severity:  "info",
	})

	metricOperatorActive.Inc()

	return &config, nil
}

func (m *OperatorManager) Disable(configID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.disableLocked(configID)
}

func (m *OperatorManager) disableLocked(configID string) error {
	config, ok := m.configs[configID]
	if !ok {
		return fmt.Errorf("operator config %q not found", configID)
	}

	if stopCh, exists := m.stopChans[configID]; exists {
		close(stopCh)
		delete(m.stopChans, configID)
	}

	if m.cronScheduler != nil {
		cronID := "operator_main_" + configID
		_ = m.cronScheduler.UnscheduleCron(cronID)

		if config != nil {
			for _, hc := range config.HealthChecks {
				if hc.Schedule != "" {
					checkCronID := fmt.Sprintf("operator_check_%s_%s", configID, hc.ID)
					_ = m.cronScheduler.UnscheduleCron(checkCronID)
				}
			}
		}
	}

	if config != nil {
		config.Enabled = false
	}
	if status, exists := m.statuses[configID]; exists {
		status.Enabled = false
	}

	m.appendEventLocked(configID, OperatorEvent{
		Timestamp: time.Now(),
		Type:      "operator_disabled",
		Details:   fmt.Sprintf("operator %s disabled", configID),
		Severity:  "info",
	})

	metricOperatorActive.Dec()

	return nil
}

func (m *OperatorManager) DisableAll() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var disabled []string
	for id := range m.configs {
		if err := m.disableLocked(id); err == nil {
			disabled = append(disabled, id)
		}
	}
	return disabled
}

func Shutdown() {
	defaultManagerMu.RLock()
	mgr := defaultManager
	defaultManagerMu.RUnlock()
	if mgr != nil {
		mgr.DisableAll()
	}
}

func (m *OperatorManager) RunCheckCycle(ctx context.Context, configID string) (*OperatorStatus, error) {
	m.mu.RLock()
	config, ok := m.configs[configID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("operator config %q not found", configID)
	}
	status := m.statuses[configID]
	m.mu.RUnlock()

	if !config.Enabled {
		return status, fmt.Errorf("operator %s is not enabled", configID)
	}

	m.AddEvent(configID, "check_cycle", "starting check cycle", "info")

	var results []HealthCheckResult
	passCount := 0
	failCount := 0

	for i := range config.HealthChecks {
		check := config.HealthChecks[i]

		var result HealthCheckResult
		if m.healthChecker != nil {
			result = m.healthChecker.ExecuteCheck(ctx, check)
		} else {
			result = HealthCheckResult{
				CheckID:   check.ID,
				CheckName: check.Name,
				Status:    CheckError,
				Message:   "health checker not configured",
				Timestamp: time.Now(),
			}
		}

		results = append(results, result)

		switch result.Status {
		case CheckPass:
			passCount++
		case CheckFail:
			failCount++
		}

		metricOperatorChecks.WithLabelValues(string(check.Type), string(result.Status)).Inc()

		if err := m.HandleCheckResult(configID, result); err != nil {
			slog.Warn("operator: handle check result error", "config_id", configID, "check_id", check.ID, "error", err)
		}
	}

	escalation, err := m.EvaluateEscalation(configID)
	if err != nil {
		slog.Warn("operator: escalation evaluation error", "config_id", configID, "error", err)
	}

	if escalation != nil {
		for _, action := range escalation.Actions {
			result, err := m.ExecuteAction(ctx, configID, action)
			if err != nil {
				slog.Warn("operator: action execution error", "config_id", configID, "action", action, "error", err)
			} else {
				m.AddEvent(configID, "action", fmt.Sprintf("executed action %s: %s", action, result), "info")
			}
		}
	}

	m.mu.Lock()
	now := time.Now()
	status.LastCheckCycle = &now
	status.TotalChecks = len(results)
	status.PassingChecks = passCount
	status.FailingChecks = failCount
	status.RecentResults = results

	if status.TotalChecks > 0 {
		status.Uptime = float64(passCount) / float64(status.TotalChecks) * 100.0
	}

	m.mu.Unlock()

	return m.GetStatus(configID), nil
}

func (m *OperatorManager) HandleCheckResult(configID string, result HealthCheckResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkResults[configID] = append(m.checkResults[configID], result)
	maxCR := m.maxCheckResults
	if maxCR <= 0 {
		maxCR = 100
	}
	if len(m.checkResults[configID]) > maxCR {
		m.checkResults[configID] = m.checkResults[configID][len(m.checkResults[configID])-maxCR:]
	}

	severity := "info"
	if result.Status == CheckFail {
		severity = "error"
	} else if result.Status == CheckWarn {
		severity = "warning"
	}

	m.appendEventLocked(configID, OperatorEvent{
		Timestamp: time.Now(),
		Type:      "check_result",
		Details:   fmt.Sprintf("check %s: %s - %s", result.CheckName, result.Status, result.Message),
		Severity:  severity,
	})

	return nil
}

func (m *OperatorManager) EvaluateEscalation(configID string) (*EscalationLevel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, ok := m.configs[configID]
	if !ok {
		return nil, fmt.Errorf("operator config %q not found", configID)
	}

	results := m.checkResults[configID]
	state := m.escalationState[configID]

	for _, level := range config.EscalationPolicy.Levels {
		if state[level.Level] {
			continue
		}

		if m.evaluateTrigger(level.Trigger, results) {
			state[level.Level] = true
			metricOperatorEscalations.Inc()
			m.appendEventLocked(configID, OperatorEvent{
				Timestamp: time.Now(),
				Type:      "escalation",
				Details:   fmt.Sprintf("escalation level %d triggered: %s", level.Level, level.Trigger),
				Severity:  "warning",
			})

			status := m.statuses[configID]
			if status != nil {
				status.ActiveEscalations++
			}

			return &level, nil
		}
	}

	return nil, nil
}

func (m *OperatorManager) evaluateTrigger(trigger string, results []HealthCheckResult) bool {
	switch {
	case trigger == "check_fail":
		for _, r := range results {
			if r.Status == CheckFail {
				return true
			}
		}
		return false

	case strings.HasPrefix(trigger, "slo_burn>"):
		thresholdStr := strings.TrimPrefix(trigger, "slo_burn>")
		threshold, err := parseFloatSuffix(thresholdStr)
		if err != nil {
			return false
		}
		for _, r := range results {
			if r.Value > threshold {
				return true
			}
		}
		return false

	case strings.HasPrefix(trigger, "alert_count>"):
		thresholdStr := strings.TrimPrefix(trigger, "alert_count>")
		threshold, err := parseFloatSuffix(thresholdStr)
		if err != nil {
			return false
		}
		for _, r := range results {
			if r.Value > threshold {
				return true
			}
		}
		return false

	default:
		return false
	}
}

func (m *OperatorManager) ExecuteAction(ctx context.Context, configID string, action string) (string, error) {
	m.mu.RLock()
	config, ok := m.configs[configID]
	if !ok {
		m.mu.RUnlock()
		return "", fmt.Errorf("operator config %q not found", configID)
	}
	autonomy := config.AutonomyLevel
	status := m.statuses[configID]
	m.mu.RUnlock()

	preApproved := map[string]bool{
		"notify":          true,
		"create_incident": true,
	}

	switch autonomy {
	case AutonomyObserve:
		msg := fmt.Sprintf("would execute: %s", action)
		m.AddEvent(configID, "action", msg, "info")
		return msg, nil

	case AutonomySuggest:
		msg := fmt.Sprintf("suggested action: %s (requires approval)", action)
		m.AddEvent(configID, "action", msg, "info")
		return msg, nil

	case AutonomyAuto:
		if preApproved[action] {
			return m.executeActionInternal(ctx, configID, action)
		}
		msg := fmt.Sprintf("suggested action: %s (requires approval - not pre-approved at auto level)", action)
		m.AddEvent(configID, "action", msg, "info")
		m.mu.Lock()
		if status != nil {
			status.AutoActionsToday++
		}
		m.mu.Unlock()
		return msg, nil

	case AutonomyFull:
		return m.executeActionInternal(ctx, configID, action)

	default:
		return "", fmt.Errorf("unknown autonomy level: %s", autonomy)
	}
}

func (m *OperatorManager) executeActionInternal(ctx context.Context, configID string, action string) (string, error) {
	m.mu.Lock()
	if status, ok := m.statuses[configID]; ok {
		status.AutoActionsToday++
	}
	config := m.configs[configID]
	m.mu.Unlock()

	switch action {
	case "auto_remediate":
		return m.executeAutoRemediate(ctx, configID, config)
	case "warroom":
		return m.executeWarRoom(ctx, configID, config)
	case "notify":
		result := "notification sent"
		m.AddEvent(configID, "action", result, "info")
		return result, nil
	case "create_incident":
		result := "incident created"
		m.AddEvent(configID, "action", result, "info")
		return result, nil
	default:
		result := fmt.Sprintf("executed: %s", action)
		m.AddEvent(configID, "action", result, "info")
		return result, nil
	}
}

func (m *OperatorManager) executeAutoRemediate(ctx context.Context, configID string, config *OperatorConfig) (string, error) {
	engine := autoremediation.DefaultRemediationEngine()
	if engine == nil {
		return "auto-remediation: engine not available", nil
	}

	service := config.Name
	action, err := engine.CreateAction("auto_remediate", service, "operator_escalation", autoremediation.AutonomyAuto)
	if err != nil {
		return fmt.Sprintf("auto-remediation failed: %v", err), err
	}

	result := fmt.Sprintf("auto-remediation action created: %s (status: %s)", action.ID, action.Status)
	m.AddEvent(configID, "action", result, "info")
	return result, nil
}

func (m *OperatorManager) executeWarRoom(ctx context.Context, configID string, config *OperatorConfig) (string, error) {
	coord := warroom.DefaultWarRoomCoordinator()
	if coord == nil {
		return "warroom: coordinator not available", nil
	}

	service := config.Name
	session, err := coord.StartWarRoom(ctx, warroom.WarRoomRequest{
		Title:       fmt.Sprintf("Operator escalation for service %s", service),
		Description: fmt.Sprintf("Operator escalation for service %s (config: %s)", service, configID),
	})
	if err != nil {
		return fmt.Sprintf("warroom failed: %v", err), err
	}

	result := fmt.Sprintf("warroom session started: %s (service: %s)", session.ID, service)
	m.AddEvent(configID, "action", result, "info")
	return result, nil
}

func (m *OperatorManager) GetStatus(configID string) *OperatorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, ok := m.statuses[configID]
	if !ok {
		return nil
	}

	cp := *status
	if len(status.RecentResults) > 0 {
		cp.RecentResults = make([]HealthCheckResult, len(status.RecentResults))
		copy(cp.RecentResults, status.RecentResults)
	} else {
		cp.RecentResults = []HealthCheckResult{}
	}

	return &cp
}

func (m *OperatorManager) ListOperators() []*OperatorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*OperatorConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	return result
}

func (m *OperatorManager) AddEvent(configID string, eventType string, details string, severity string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.appendEventLocked(configID, OperatorEvent{
		Timestamp: time.Now(),
		Type:      eventType,
		Details:   details,
		Severity:  severity,
	})
}

func (m *OperatorManager) appendEventLocked(configID string, event OperatorEvent) {
	m.events[configID] = append(m.events[configID], event)
	maxE := m.maxEvents
	if maxE <= 0 {
		maxE = 1000
	}
	if len(m.events[configID]) > maxE {
		m.events[configID] = m.events[configID][len(m.events[configID])-maxE:]
	}
}

func (m *OperatorManager) GetEvents(configID string) []OperatorEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events, ok := m.events[configID]
	if !ok {
		return nil
	}
	cp := make([]OperatorEvent, len(events))
	copy(cp, events)
	return cp
}

func (m *OperatorManager) GetCheckResults(configID string) []HealthCheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results, ok := m.checkResults[configID]
	if !ok {
		return nil
	}
	cp := make([]HealthCheckResult, len(results))
	copy(cp, results)
	return cp
}

func (m *OperatorManager) runSchedulerLoop(configID string, schedule string, stopCh chan struct{}) {
	slog.Info("operator: scheduler loop started", "config_id", configID)

	for {
		select {
		case <-stopCh:
			slog.Info("operator: scheduler loop stopped", "config_id", configID)
			return
		default:
		}

		select {
		case <-stopCh:
			slog.Info("operator: scheduler loop stopped", "config_id", configID)
			return
		case <-time.After(config.CheckInterval):
			m.mu.RLock()
			cfg, exists := m.configs[configID]
			m.mu.RUnlock()

			if !exists || !cfg.Enabled {
				return
			}
		}
	}
}

func (m *OperatorManager) ResetEscalationState(configID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.escalationState[configID] = make(map[int]bool)
	if status, ok := m.statuses[configID]; ok {
		status.ActiveEscalations = 0
	}
}

func defaultEscalationPolicy() EscalationPolicy {
	return EscalationPolicy{
		Levels: []EscalationLevel{
			{
				Level:    1,
				Trigger:  "check_fail",
				Actions:  []string{"notify"},
				WaitTime: 5 * time.Minute,
				Notify:   []string{"on-call"},
			},
			{
				Level:    2,
				Trigger:  "slo_burn>3",
				Actions:  []string{"notify", "create_incident"},
				WaitTime: 15 * time.Minute,
				Notify:   []string{"on-call", "team-lead"},
			},
		},
	}
}

func InitOperatorManager(hc *HealthChecker, cs CronScheduler) *OperatorManager {
	m := NewOperatorManager()
	if hc != nil {
		m.SetHealthChecker(hc)
	}
	if cs != nil {
		m.SetCronScheduler(cs)
	}
	SetOperatorManager(m)
	return m
}
