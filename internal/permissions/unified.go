package permissions

import (
	"fmt"
	"sync"
)

type UnifiedPermissionEngine struct {
	mu           sync.RWMutex
	approvalGate *ApprovalGate
	permEngine   *PermissionEngine
	mode         UnifiedMode
}

type UnifiedMode string

const (
	UnifiedModeApprovalGate UnifiedMode = "approval_gate"
	UnifiedModeEngine       UnifiedMode = "engine"
	UnifiedModeBoth         UnifiedMode = "both"
)

type UnifiedDecision struct {
	Allowed bool
	Reason  string
	Source  string
}

func NewUnifiedPermissionEngine(approvalGate *ApprovalGate, permEngine *PermissionEngine) *UnifiedPermissionEngine {
	mode := UnifiedModeApprovalGate
	if permEngine != nil {
		mode = UnifiedModeBoth
	}
	if approvalGate == nil {
		approvalGate = NewApprovalGate()
	}
	return &UnifiedPermissionEngine{
		approvalGate: approvalGate,
		permEngine:   permEngine,
		mode:         mode,
	}
}

func NewUnifiedPermissionEngineWithMode(approvalGate *ApprovalGate, permEngine *PermissionEngine, mode UnifiedMode) *UnifiedPermissionEngine {
	if approvalGate == nil {
		approvalGate = NewApprovalGate()
	}
	return &UnifiedPermissionEngine{
		approvalGate: approvalGate,
		permEngine:   permEngine,
		mode:         mode,
	}
}

func (u *UnifiedPermissionEngine) CheckPermission(toolName string, input map[string]any, context map[string]any) (bool, string) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	switch u.mode {
	case UnifiedModeApprovalGate:
		needsApproval, reason := u.approvalGate.NeedsApproval(toolName, input)
		return !needsApproval, reason

	case UnifiedModeEngine:
		if u.permEngine == nil {
			return false, "PermissionEngine not configured"
		}
		decision := u.permEngine.Check(toolName, input)
		switch decision.Behavior {
		case PermissionBehaviorAllow:
			return true, ""
		case PermissionBehaviorDeny:
			reason := "Denied by permission engine"
			if decision.Reason != nil {
				reason = decision.Reason.Reason
			}
			return false, reason
		case PermissionBehaviorAsk:
			reason := fmt.Sprintf("Tool %q requires approval", toolName)
			if decision.Reason != nil {
				reason = decision.Reason.Reason
			}
			return false, reason
		default:
			return false, fmt.Sprintf("Unknown behavior: %s", decision.Behavior)
		}

	case UnifiedModeBoth:
		if u.permEngine != nil {
			decision := u.permEngine.Check(toolName, input)
			switch decision.Behavior {
			case PermissionBehaviorDeny:
				reason := "Denied by permission engine"
				if decision.Reason != nil {
					reason = decision.Reason.Reason
				}
				return false, reason
			case PermissionBehaviorAllow:
				needsApproval, reason := u.approvalGate.NeedsApproval(toolName, input)
				if needsApproval {
					return false, reason
				}
				return true, ""
			case PermissionBehaviorAsk:
				needsApproval, gateReason := u.approvalGate.NeedsApproval(toolName, input)
				if needsApproval {
					return false, gateReason
				}
				reason := fmt.Sprintf("Tool %q requires approval", toolName)
				if decision.Reason != nil {
					reason = decision.Reason.Reason
				}
				return false, reason
			}
		}
		needsApproval, reason := u.approvalGate.NeedsApproval(toolName, input)
		return !needsApproval, reason

	default:
		needsApproval, reason := u.approvalGate.NeedsApproval(toolName, input)
		return !needsApproval, reason
	}
}

func (u *UnifiedPermissionEngine) SetMode(mode UnifiedMode) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.mode = mode
}

func (u *UnifiedPermissionEngine) GetMode() UnifiedMode {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.mode
}

func (u *UnifiedPermissionEngine) GetApprovalGate() *ApprovalGate {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.approvalGate
}

func (u *UnifiedPermissionEngine) GetPermissionEngine() *PermissionEngine {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.permEngine
}

func (u *UnifiedPermissionEngine) SetPermissionEngine(engine *PermissionEngine) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.permEngine = engine
	if engine != nil && u.mode == UnifiedModeApprovalGate {
		u.mode = UnifiedModeBoth
	}
}

func (u *UnifiedPermissionEngine) LoadApprovalConfig(path string) error {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.approvalGate.LoadConfig(path)
}

func (u *UnifiedPermissionEngine) LoadApprovalConfigFromDefaultPath() error {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.approvalGate.LoadConfigFromDefaultPath()
}

func (u *UnifiedPermissionEngine) SetSREMode(enabled bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.approvalGate.SetSREMode(enabled)
}

func (u *UnifiedPermissionEngine) AddToolPolicy(toolName string, policy ApprovalPolicy) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.approvalGate.AddToolPolicy(toolName, policy)
}

func (u *UnifiedPermissionEngine) AddRule(source PermissionRuleSource, rule PermissionRule) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if u.permEngine != nil {
		u.permEngine.AddRule(source, rule)
	}
}

func (u *UnifiedPermissionEngine) RecordDecision(toolName string, result PermissionResult) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if u.permEngine != nil {
		u.permEngine.RecordDecision(toolName, result)
	}
}

func (u *UnifiedPermissionEngine) GetConfig() ApprovalConfig {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.approvalGate.GetConfig()
}
