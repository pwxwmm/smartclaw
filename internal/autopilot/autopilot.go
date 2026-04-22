package autopilot

import (
	"fmt"
	"strings"
)

// TrustLevel represents the level of autonomous operation allowed
type TrustLevel int

const (
	TrustLevelOff     TrustLevel = iota // No auto-execution, ask for everything
	TrustLevelRead                      // Auto-approve read-only operations
	TrustLevelWrite                     // Auto-approve read + write within workspace
	TrustLevelExecute                   // Auto-approve read + write + shell (non-destructive)
	TrustLevelFull                      // Auto-approve everything (dangerous)
)

// String returns a human-readable name for the trust level
func (t TrustLevel) String() string {
	switch t {
	case TrustLevelOff:
		return "off"
	case TrustLevelRead:
		return "read"
	case TrustLevelWrite:
		return "write"
	case TrustLevelExecute:
		return "execute"
	case TrustLevelFull:
		return "full"
	default:
		return "unknown"
	}
}

// Action represents what the autopilot decides
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionAsk   Action = "ask"
)

// AutoDecision represents the autopilot's decision on a tool call
type AutoDecision struct {
	Action    Action     // allow, deny, ask
	Reason    string     // human-readable explanation
	TrustUsed TrustLevel // which trust level justified this decision
	ToolName  string     // tool that was evaluated
	RiskScore float64    // 0.0-1.0 risk assessment
}

// AutoRule defines a rule for auto-approving or auto-denying a tool
type AutoRule struct {
	ToolName  string     // tool to match ("" = wildcard)
	Action    Action     // allow or deny
	Condition string     // when this rule applies (e.g., "path within workspace")
	TrustMin  TrustLevel // minimum trust level required
	RiskScore float64    // risk score assigned (0.0=safe, 1.0=dangerous)
}

// SessionPolicy tracks session-level autopilot decisions
type SessionPolicy struct {
	TrustLevel      TrustLevel
	WorkspaceRoot   string          // workspace root for path checks
	AllowedPaths    []string        // paths where writes are allowed
	DeniedPaths     []string        // paths where operations are denied
	DeniedTools     map[string]bool // tools explicitly denied this session
	MaxAutoActions  int             // max auto-approved actions before requiring confirmation
	autoActionCount int             // count of auto-approved actions
	totalSaved      int             // total permission prompts saved
}

// AutoStats tracks autopilot performance metrics
type AutoStats struct {
	TotalDecisions  int
	AutoApproved    int
	AutoDenied      int
	EscalatedToUser int
	PromptsSaved    int
}

// AutopilotEngine makes autonomous execution decisions
type AutopilotEngine struct {
	rules  []AutoRule
	policy SessionPolicy
	stats  AutoStats
}

// NewAutopilotEngine creates a new autopilot engine with the given trust level and workspace root
func NewAutopilotEngine(trustLevel TrustLevel, workspaceRoot string) *AutopilotEngine {
	return &AutopilotEngine{
		rules: defaultRules(),
		policy: SessionPolicy{
			TrustLevel:   trustLevel,
			WorkspaceRoot: workspaceRoot,
			DeniedTools:  make(map[string]bool),
		},
		stats: AutoStats{},
	}
}

// Decide determines whether a tool call should be auto-approved
func (ae *AutopilotEngine) Decide(toolName string, input map[string]any) AutoDecision {
	// Step 1: Check if trust level is Off — always ask
	if ae.policy.TrustLevel == TrustLevelOff {
		ae.stats.TotalDecisions++
		ae.stats.EscalatedToUser++
		return AutoDecision{
			Action:    ActionAsk,
			Reason:    "autopilot is off",
			TrustUsed: TrustLevelOff,
			ToolName:  toolName,
			RiskScore: 1.0,
		}
	}

	// Step 2: Check denied tools list
	if ae.policy.DeniedTools[toolName] {
		ae.stats.TotalDecisions++
		ae.stats.AutoDenied++
		return AutoDecision{
			Action:    ActionDeny,
			Reason:    fmt.Sprintf("tool %s is denied for this session", toolName),
			TrustUsed: ae.policy.TrustLevel,
			ToolName:  toolName,
			RiskScore: 1.0,
		}
	}

	// Step 3: Check denied paths (for file operations)
	if path, ok := input["path"].(string); ok {
		for _, denied := range ae.policy.DeniedPaths {
			if strings.HasPrefix(path, denied) {
				ae.stats.TotalDecisions++
				ae.stats.AutoDenied++
				return AutoDecision{
					Action:    ActionDeny,
					Reason:    fmt.Sprintf("path %s is denied", path),
					TrustUsed: ae.policy.TrustLevel,
					ToolName:  toolName,
					RiskScore: 1.0,
				}
			}
		}
	}

	// Step 4: Check rules (most specific first — prefer tool-specific over wildcard)
	bestMatch := AutoRule{}
	matched := false
	for _, rule := range ae.rules {
		if rule.ToolName == toolName || rule.ToolName == "" {
			ruleApplies := ae.policy.TrustLevel >= rule.TrustMin || rule.Action == ActionDeny
			if ruleApplies {
				if !matched || rule.ToolName != "" {
					bestMatch = rule
					matched = true
				}
			}
		}
	}

	if matched {
		// Step 5: Check max auto actions
		if ae.policy.MaxAutoActions > 0 && ae.policy.autoActionCount >= ae.policy.MaxAutoActions {
			ae.stats.TotalDecisions++
			ae.stats.EscalatedToUser++
			return AutoDecision{
				Action:    ActionAsk,
				Reason:    fmt.Sprintf("reached max auto-actions (%d), requiring confirmation", ae.policy.MaxAutoActions),
				TrustUsed: ae.policy.TrustLevel,
				ToolName:  toolName,
				RiskScore: bestMatch.RiskScore,
			}
		}

		ae.stats.TotalDecisions++
		ae.policy.autoActionCount++
		ae.policy.totalSaved++
		ae.stats.PromptsSaved++

		if bestMatch.Action == ActionAllow {
			ae.stats.AutoApproved++
		} else {
			ae.stats.AutoDenied++
		}

		return AutoDecision{
			Action:    bestMatch.Action,
			Reason:    bestMatch.Condition,
			TrustUsed: bestMatch.TrustMin,
			ToolName:  toolName,
			RiskScore: bestMatch.RiskScore,
		}
	}

	// Step 6: No rule matched — default to ask
	ae.stats.TotalDecisions++
	ae.stats.EscalatedToUser++
	return AutoDecision{
		Action:    ActionAsk,
		Reason:    "no autopilot rule matched",
		TrustUsed: ae.policy.TrustLevel,
		ToolName:  toolName,
		RiskScore: 0.5,
	}
}

// SetTrustLevel changes the current trust level
func (ae *AutopilotEngine) SetTrustLevel(level TrustLevel) {
	ae.policy.TrustLevel = level
}

// GetTrustLevel returns the current trust level
func (ae *AutopilotEngine) GetTrustLevel() TrustLevel {
	return ae.policy.TrustLevel
}

// GetStats returns autopilot performance metrics
func (ae *AutopilotEngine) GetStats() AutoStats {
	return ae.stats
}

// AddRule adds a custom auto-rule
func (ae *AutopilotEngine) AddRule(rule AutoRule) {
	ae.rules = append(ae.rules, rule)
}

// SetMaxAutoActions sets the maximum auto-approved actions before requiring confirmation
func (ae *AutopilotEngine) SetMaxAutoActions(max int) {
	ae.policy.MaxAutoActions = max
}

// DenyTool explicitly denies a tool for this session
func (ae *AutopilotEngine) DenyTool(toolName string) {
	ae.policy.DeniedTools[toolName] = true
}

// AllowPath adds a path to the allowed list
func (ae *AutopilotEngine) AllowPath(path string) {
	ae.policy.AllowedPaths = append(ae.policy.AllowedPaths, path)
}

// DenyPath adds a path to the denied list
func (ae *AutopilotEngine) DenyPath(path string) {
	ae.policy.DeniedPaths = append(ae.policy.DeniedPaths, path)
}

// GetTotalSaved returns the total number of permission prompts saved
func (ae *AutopilotEngine) GetTotalSaved() int {
	return ae.policy.totalSaved
}
