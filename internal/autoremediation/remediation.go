package autoremediation

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SLOProvider interface {
	GetSLOStatus(service string) (*SLOInfo, error)
}

type Commander interface {
	ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (string, error)
	ExecuteTool(ctx context.Context, toolName string, params map[string]any) (any, error)
}

type RemediationEngine struct {
	mu          sync.RWMutex
	runbooks    map[string]*Runbook
	actions     map[string]*RemediationAction
	history     []RemediationAction
	runbookDir  string
	sloProvider SLOProvider
	commander   Commander
}

func NewRemediationEngine(runbookDir string) *RemediationEngine {
	return &RemediationEngine{
		runbooks:   make(map[string]*Runbook),
		actions:    make(map[string]*RemediationAction),
		history:    make([]RemediationAction, 0, config.MaxHistorySize),
		runbookDir: runbookDir,
	}
}

func Shutdown() {
}

func (e *RemediationEngine) SetSLOProvider(sp SLOProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sloProvider = sp
}

func (e *RemediationEngine) SetCommander(c Commander) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.commander = c
}

func (e *RemediationEngine) LoadRunbooks() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	runbooks, err := LoadRunbooksFromDir(e.runbookDir)
	if err != nil {
		return err
	}

	EnsureBuiltInRunbooks(e.runbookDir, runbooks)

	// Reload after ensuring built-ins
	runbooks, err = LoadRunbooksFromDir(e.runbookDir)
	if err != nil {
		return err
	}

	e.runbooks = runbooks
	metricRemediationRunbooksLoaded.Set(float64(len(runbooks)))
	return nil
}

func (e *RemediationEngine) SaveRunbook(runbook *Runbook) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := SaveRunbookToDir(e.runbookDir, runbook); err != nil {
		return err
	}

	e.runbooks[runbook.ID] = runbook
	return nil
}

func BurnRateToAutonomy(burnRate float64, errorBudgetLeft float64) AutonomyLevel {
	if burnRate > 14.0 && errorBudgetLeft < 0.05 {
		return AutonomySpeculative
	}
	if burnRate > 6.0 && errorBudgetLeft < 0.10 {
		return AutonomyAuto
	}
	if burnRate > 3.0 && errorBudgetLeft < 0.20 {
		return AutonomyPreApproved
	}
	return AutonomySuggest
}

func (e *RemediationEngine) AssessSLO(service string) (*SLOAssessment, error) {
	e.mu.RLock()
	sp := e.sloProvider
	e.mu.RUnlock()

	if sp == nil {
		return nil, fmt.Errorf("autoremediation: SLO provider not configured")
	}

	sloInfo, err := sp.GetSLOStatus(service)
	if err != nil {
		return nil, fmt.Errorf("autoremediation: assess SLO: %w", err)
	}

	autonomy := BurnRateToAutonomy(sloInfo.BurnRate, sloInfo.ErrorBudgetRemaining)

	var recommended []string
	e.mu.RLock()
	for _, rb := range e.runbooks {
		if rb.Trigger.Type == "slo_burn" && sloInfo.BurnRate >= rb.Trigger.BurnRate {
			if rb.Service == "*" || rb.Service == service {
				recommended = append(recommended, rb.ID)
			}
		}
	}
	e.mu.RUnlock()

	return &SLOAssessment{
		Service:             service,
		SLOName:             sloInfo.SLOName,
		BurnRate:            sloInfo.BurnRate,
		ErrorBudgetLeft:     sloInfo.ErrorBudgetRemaining,
		AutonomyLevel:       autonomy,
		RecommendedRunbooks: recommended,
	}, nil
}

func (e *RemediationEngine) SuggestRemediation(service string, trigger string) ([]*Runbook, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var matches []*Runbook
	for _, rb := range e.runbooks {
		if rb.Service != "*" && rb.Service != service {
			continue
		}
		if trigger != "" && rb.Trigger.Type != trigger {
			continue
		}
		matches = append(matches, rb)
	}

	return matches, nil
}

func (e *RemediationEngine) CreateAction(runbookID string, service string, trigger string, autonomy AutonomyLevel) (*RemediationAction, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	rb, ok := e.runbooks[runbookID]
	if !ok {
		return nil, fmt.Errorf("autoremediation: runbook not found: %s", runbookID)
	}

	action := &RemediationAction{
		ID:        uuid.New().String(),
		RunbookID: runbookID,
		Service:   service,
		Trigger:   trigger,
		Autonomy:  autonomy,
		Status:    ActionPending,
		Steps:     make([]StepResult, 0, len(rb.Steps)),
		StartedAt: time.Now().UTC(),
	}

	e.actions[action.ID] = action
	metricRemediationActions.WithLabelValues(string(ActionPending)).Inc()
	return action, nil
}

func (e *RemediationEngine) ApproveAction(actionID string, approver string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	action, ok := e.actions[actionID]
	if !ok {
		return fmt.Errorf("autoremediation: action not found: %s", actionID)
	}

	if action.Status != ActionPending {
		return fmt.Errorf("autoremediation: action %s is %s, cannot approve (must be pending)", actionID, action.Status)
	}

	action.Status = ActionApproved
	action.ApprovedBy = approver
	return nil
}

func (e *RemediationEngine) ExecuteAction(ctx context.Context, actionID string) (*RemediationResult, error) {
	// Get action and runbook under read lock first
	e.mu.RLock()
	action, ok := e.actions[actionID]
	if !ok {
		e.mu.RUnlock()
		return nil, fmt.Errorf("autoremediation: action not found: %s", actionID)
	}
	rb, rbOk := e.runbooks[action.RunbookID]
	commander := e.commander
	e.mu.RUnlock()

	if !rbOk {
		return nil, fmt.Errorf("autoremediation: runbook not found: %s", action.RunbookID)
	}

	if !autonomySufficient(action.Autonomy, rb.Autonomy) {
		return nil, fmt.Errorf("autoremediation: insufficient autonomy: action has %s but runbook requires %s", action.Autonomy, rb.Autonomy)
	}

	// Check action status — must be pending or approved to execute
	e.mu.Lock()
	if action.Status != ActionPending && action.Status != ActionApproved {
		e.mu.Unlock()
		return nil, fmt.Errorf("autoremediation: action %s is %s, cannot execute (must be pending or approved)", actionID, action.Status)
	}
	wasApproved := action.Status == ActionApproved
	action.Status = ActionRunning
	e.mu.Unlock()

	startTime := time.Now().UTC()
	needsRollback := false
	var lastFailedStep *RunbookStep

	for _, step := range rb.Steps {
		if step.Type == StepApproval && action.Autonomy != AutonomyAuto && action.Autonomy != AutonomySpeculative {
			if !wasApproved {
				sr := StepResult{
					StepID:    step.ID,
					StepName:  step.Name,
					Status:    ActionPending,
					Output:    "waiting for approval",
					StartedAt: time.Now().UTC(),
				}
				e.mu.Lock()
				action.Steps = append(action.Steps, sr)
				e.mu.Unlock()
				break
			}
		}

		sr := e.executeStep(ctx, step, action, commander)

		e.mu.Lock()
		action.Steps = append(action.Steps, sr)
		e.mu.Unlock()

		if sr.Status == ActionFailed {
			lastFailedStep = &step
			switch step.OnFailure {
			case FailureStop:
				e.mu.Lock()
				now := time.Now().UTC()
				action.Status = ActionFailed
				action.CompletedAt = &now
				e.mu.Unlock()
				return e.buildResult(action, startTime), nil

			case FailureRollback:
				needsRollback = true
				e.rollbackAction(ctx, action, rb)
				e.mu.Lock()
				now := time.Now().UTC()
				action.Status = ActionRolledBack
				action.CompletedAt = &now
				e.mu.Unlock()
				return e.buildResult(action, startTime), nil

			case FailureContinue:
				continue

			case FailureSkip:
				continue
			}
		}
	}

	_ = lastFailedStep

	e.mu.Lock()
	now := time.Now().UTC()
	action.CompletedAt = &now
	if !needsRollback {
		action.Status = ActionSuccess
	}
	e.mu.Unlock()

	metricRemediationActions.WithLabelValues(string(action.Status)).Inc()

	e.addToHistory(*action)

	return e.buildResult(action, startTime), nil
}

func (e *RemediationEngine) CancelAction(actionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	action, ok := e.actions[actionID]
	if !ok {
		return fmt.Errorf("autoremediation: action not found: %s", actionID)
	}

	if action.Status != ActionPending && action.Status != ActionApproved && action.Status != ActionRunning {
		return fmt.Errorf("autoremediation: action %s is %s, cannot cancel", actionID, action.Status)
	}

	action.Status = ActionCancelled
	now := time.Now().UTC()
	action.CompletedAt = &now
	return nil
}

func (e *RemediationEngine) GetAction(actionID string) *RemediationAction {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.actions[actionID]
}

func (e *RemediationEngine) ListActions(status ActionStatus) []*RemediationAction {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*RemediationAction
	for _, a := range e.actions {
		if status == "" || a.Status == status {
			result = append(result, a)
		}
	}
	return result
}

func (e *RemediationEngine) executeStep(ctx context.Context, step RunbookStep, _ *RemediationAction, commander Commander) StepResult {
	start := time.Now().UTC()
	sr := StepResult{
		StepID:    step.ID,
		StepName:  step.Name,
		StartedAt: start,
	}

	if commander == nil {
		sr.Status = ActionFailed
		sr.Error = "commander not configured"
		sr.CompletedAt = ptrTime(time.Now().UTC())
		sr.Duration = time.Since(start)
		return sr
	}

	stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
	defer cancel()

	switch step.Type {
	case StepCommand:
		out, err := commander.ExecuteCommand(stepCtx, step.Action, step.Timeout)
		if err != nil {
			sr.Status = ActionFailed
			sr.Error = err.Error()
		} else {
			sr.Status = ActionSuccess
			sr.Output = out
		}

	case StepAPI:
		out, err := commander.ExecuteCommand(stepCtx, step.Action, step.Timeout)
		if err != nil {
			sr.Status = ActionFailed
			sr.Error = err.Error()
		} else {
			sr.Status = ActionSuccess
			sr.Output = out
		}

	case StepTool:
		result, err := commander.ExecuteTool(stepCtx, step.Action, step.Parameters)
		if err != nil {
			sr.Status = ActionFailed
			sr.Error = err.Error()
		} else {
			sr.Status = ActionSuccess
			sr.Output = fmt.Sprintf("%v", result)
		}

	case StepPrompt:
		result, err := commander.ExecuteTool(stepCtx, step.Action, step.Parameters)
		if err != nil {
			sr.Status = ActionFailed
			sr.Error = err.Error()
		} else {
			sr.Status = ActionSuccess
			sr.Output = fmt.Sprintf("%v", result)
		}

	case StepApproval:
		sr.Status = ActionSuccess
		sr.Output = "approved"

	default:
		sr.Status = ActionFailed
		sr.Error = fmt.Sprintf("unknown step type: %s", step.Type)
	}

	sr.CompletedAt = ptrTime(time.Now().UTC())
	sr.Duration = time.Since(start)
	return sr
}

func (e *RemediationEngine) rollbackAction(ctx context.Context, action *RemediationAction, rb *Runbook) {
	e.mu.RLock()
	commander := e.commander
	e.mu.RUnlock()

	if commander == nil {
		slog.Warn("autoremediation: cannot rollback, commander not configured", "action_id", action.ID)
		return
	}

	// Execute rollback steps in reverse order
	for i := len(rb.Steps) - 1; i >= 0; i-- {
		step := rb.Steps[i]
		if step.Rollback == "" {
			continue
		}

		// Only rollback steps that completed successfully
		var completed bool
		e.mu.RLock()
		for _, sr := range action.Steps {
			if sr.StepID == step.ID && sr.Status == ActionSuccess {
				completed = true
				break
			}
		}
		e.mu.RUnlock()

		if !completed {
			continue
		}

		rollbackCtx, cancel := context.WithTimeout(ctx, step.Timeout)
		_, err := commander.ExecuteCommand(rollbackCtx, step.Rollback, step.Timeout)
		cancel()

		if err != nil {
			slog.Warn("autoremediation: rollback step failed",
				"action_id", action.ID,
				"step_id", step.ID,
				"error", err,
			)
		}
	}
}

func (e *RemediationEngine) addToHistory(action RemediationAction) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.history) >= config.MaxHistorySize {
		e.history = e.history[1:]
	}
	e.history = append(e.history, action)
}

func (e *RemediationEngine) buildResult(action *RemediationAction, startTime time.Time) *RemediationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var stepsCopy []StepResult
	if len(action.Steps) > 0 {
		stepsCopy = make([]StepResult, len(action.Steps))
		copy(stepsCopy, action.Steps)
	}

	summary := fmt.Sprintf("Action %s completed with status %s (%d steps)", action.ID, action.Status, len(action.Steps))

	return &RemediationResult{
		ActionID:       action.ID,
		Status:         action.Status,
		Steps:          stepsCopy,
		Summary:        summary,
		RollbackNeeded: action.Status == ActionRolledBack,
		Duration:       time.Since(startTime),
	}
}

func autonomySufficient(current, required AutonomyLevel) bool {
	levels := map[AutonomyLevel]int{
		AutonomySuggest:     0,
		AutonomyPreApproved: 1,
		AutonomyAuto:        2,
		AutonomySpeculative: 3,
	}
	return levels[current] >= levels[required]
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
