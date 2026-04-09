package permissions

import (
	"fmt"
	"path/filepath"
	"strings"
)

type PermissionMode string

const (
	PermissionModeDefault           PermissionMode = "default"
	PermissionModePlan              PermissionMode = "plan"
	PermissionModeAcceptEdits       PermissionMode = "acceptEdits"
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
	PermissionModeDontAsk           PermissionMode = "dontAsk"
	PermissionModeAuto              PermissionMode = "auto"
)

type PermissionBehavior string

const (
	PermissionBehaviorAllow       PermissionBehavior = "allow"
	PermissionBehaviorDeny        PermissionBehavior = "deny"
	PermissionBehaviorAsk         PermissionBehavior = "ask"
	PermissionBehaviorPassthrough PermissionBehavior = "passthrough"
)

type PermissionResult string

const (
	PermissionAllow PermissionResult = "allow"
	PermissionDeny  PermissionResult = "deny"
	PermissionAsk   PermissionResult = "ask"
)

type PermissionRuleSource string

const (
	PermissionSourceProject PermissionRuleSource = "project"
	PermissionSourceUser    PermissionRuleSource = "user"
	PermissionSourceLocal   PermissionRuleSource = "local"
	PermissionSourceCliArg  PermissionRuleSource = "cliArg"
	PermissionSourceCommand PermissionRuleSource = "command"
	PermissionSourceSession PermissionRuleSource = "session"
)

type PermissionRuleValue struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

type PermissionRule struct {
	Source       PermissionRuleSource `json:"source"`
	RuleBehavior PermissionBehavior   `json:"ruleBehavior"`
	RuleValue    PermissionRuleValue  `json:"ruleValue"`
}

type PermissionDecisionReason struct {
	Type       string          `json:"type"`
	Reason     string          `json:"reason,omitempty"`
	Rule       *PermissionRule `json:"rule,omitempty"`
	Mode       PermissionMode  `json:"mode,omitempty"`
	HookName   string          `json:"hookName,omitempty"`
	Classifier string          `json:"classifier,omitempty"`
}

type PermissionDecision struct {
	Behavior PermissionBehavior        `json:"behavior"`
	Reason   *PermissionDecisionReason `json:"reason,omitempty"`
}

type PermissionEngine struct {
	mode             PermissionMode
	alwaysAllowRules map[PermissionRuleSource][]PermissionRule
	alwaysDenyRules  map[PermissionRuleSource][]PermissionRule
	alwaysAskRules   map[PermissionRuleSource][]PermissionRule
	sessionDecisions map[string]PermissionResult
	workDir          string
}

func NewEngine(mode PermissionMode, workDir string) *PermissionEngine {
	return &PermissionEngine{
		mode:             mode,
		workDir:          workDir,
		alwaysAllowRules: make(map[PermissionRuleSource][]PermissionRule),
		alwaysDenyRules:  make(map[PermissionRuleSource][]PermissionRule),
		alwaysAskRules:   make(map[PermissionRuleSource][]PermissionRule),
		sessionDecisions: make(map[string]PermissionResult),
	}
}

func (e *PermissionEngine) Check(toolName string, input map[string]interface{}) *PermissionDecision {
	if e.mode == PermissionModeBypassPermissions || e.mode == PermissionModeDontAsk {
		return &PermissionDecision{Behavior: PermissionBehaviorAllow}
	}

	if e.mode == PermissionModePlan {
		return &PermissionDecision{
			Behavior: PermissionBehaviorAsk,
			Reason: &PermissionDecisionReason{
				Type:   "mode",
				Mode:   e.mode,
				Reason: "Plan mode requires approval",
			},
		}
	}

	if e.mode == PermissionModeAuto {
		return e.checkAutoMode(toolName, input)
	}

	if e.mode == PermissionModeAcceptEdits {
		if isEditTool(toolName) {
			return &PermissionDecision{Behavior: PermissionBehaviorAllow}
		}
	}

	if isReadOnlyTool(toolName) {
		return &PermissionDecision{Behavior: PermissionBehaviorAllow}
	}

	for _, rules := range e.alwaysAllowRules {
		for _, rule := range rules {
			if matchRule(rule, toolName, input) {
				return &PermissionDecision{
					Behavior: PermissionBehaviorAllow,
					Reason: &PermissionDecisionReason{
						Type: "rule",
						Rule: &rule,
					},
				}
			}
		}
	}

	for _, rules := range e.alwaysDenyRules {
		for _, rule := range rules {
			if matchRule(rule, toolName, input) {
				return &PermissionDecision{
					Behavior: PermissionBehaviorDeny,
					Reason: &PermissionDecisionReason{
						Type: "rule",
						Rule: &rule,
					},
				}
			}
		}
	}

	if cached, ok := e.sessionDecisions[toolName]; ok {
		return &PermissionDecision{Behavior: PermissionBehavior(cached)}
	}

	for _, rules := range e.alwaysAskRules {
		for _, rule := range rules {
			if matchRule(rule, toolName, input) {
				return &PermissionDecision{
					Behavior: PermissionBehaviorAsk,
					Reason: &PermissionDecisionReason{
						Type: "rule",
						Rule: &rule,
					},
				}
			}
		}
	}

	return &PermissionDecision{
		Behavior: PermissionBehaviorAsk,
		Reason: &PermissionDecisionReason{
			Type:   "mode",
			Mode:   e.mode,
			Reason: "Default permission mode requires approval",
		},
	}
}

func (e *PermissionEngine) checkAutoMode(toolName string, input map[string]interface{}) *PermissionDecision {
	if isReadOnlyTool(toolName) {
		return &PermissionDecision{Behavior: PermissionBehaviorAllow}
	}

	return &PermissionDecision{
		Behavior: PermissionBehaviorAsk,
		Reason: &PermissionDecisionReason{
			Type:   "mode",
			Mode:   e.mode,
			Reason: "Auto mode requires approval for write operations",
		},
	}
}

func (e *PermissionEngine) AddRule(source PermissionRuleSource, rule PermissionRule) {
	switch rule.RuleBehavior {
	case PermissionBehaviorAllow:
		e.alwaysAllowRules[source] = append(e.alwaysAllowRules[source], rule)
	case PermissionBehaviorDeny:
		e.alwaysDenyRules[source] = append(e.alwaysDenyRules[source], rule)
	case PermissionBehaviorAsk:
		e.alwaysAskRules[source] = append(e.alwaysAskRules[source], rule)
	}
}

func (e *PermissionEngine) RecordDecision(toolName string, result PermissionResult) {
	e.sessionDecisions[toolName] = result
}

func (e *PermissionEngine) SetMode(mode PermissionMode) {
	e.mode = mode
}

func (e *PermissionEngine) GetMode() PermissionMode {
	return e.mode
}

func (e *PermissionEngine) GetAllowRules() []PermissionRule {
	var rules []PermissionRule
	for _, sourceRules := range e.alwaysAllowRules {
		rules = append(rules, sourceRules...)
	}
	return rules
}

func (e *PermissionEngine) GetDenyRules() []PermissionRule {
	var rules []PermissionRule
	for _, sourceRules := range e.alwaysDenyRules {
		rules = append(rules, sourceRules...)
	}
	return rules
}

func (e *PermissionEngine) ClearSessionDecisions() {
	e.sessionDecisions = make(map[string]PermissionResult)
}

func isReadOnlyTool(name string) bool {
	readOnlyTools := map[string]bool{
		"read_file":           true,
		"glob":                true,
		"grep":                true,
		"web_fetch":           true,
		"web_search":          true,
		"list_mcp_resources":  true,
		"read_mcp_resource":   true,
		"lsp":                 true,
		"lsp_goto_definition": true,
		"lsp_find_references": true,
		"lsp_symbols":         true,
		"tool_search":         true,
	}
	return readOnlyTools[name]
}

func isEditTool(name string) bool {
	editTools := map[string]bool{
		"edit_file":  true,
		"write_file": true,
		"bash":       true,
		"powershell": true,
	}
	return editTools[name]
}

func matchRule(rule PermissionRule, toolName string, input map[string]interface{}) bool {
	if rule.RuleValue.ToolName == "*" {
		return true
	}
	if rule.RuleValue.ToolName == toolName {
		return true
	}
	if strings.HasSuffix(rule.RuleValue.ToolName, "*") {
		prefix := strings.TrimSuffix(rule.RuleValue.ToolName, "*")
		return strings.HasPrefix(toolName, prefix)
	}
	if strings.HasPrefix(rule.RuleValue.ToolName, "*") {
		suffix := strings.TrimPrefix(rule.RuleValue.ToolName, "*")
		return strings.HasSuffix(toolName, suffix)
	}
	return false
}

func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == name {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix)
	}
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(name, suffix)
	}
	return false
}

type SandboxConfig struct {
	Enabled            bool
	FilesystemMode     string
	AllowedPaths       []string
	NetworkIsolation   bool
	NamespaceIsolation bool
	WorkDir            string
}

func NewSandboxConfig(workDir string) *SandboxConfig {
	return &SandboxConfig{
		WorkDir:        workDir,
		FilesystemMode: "workspace-only",
		Enabled:        true,
	}
}

func (s *SandboxConfig) ValidatePath(path string) error {
	if !s.Enabled {
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absWorkDir, err := filepath.Abs(s.WorkDir)
	if err != nil {
		return fmt.Errorf("invalid workdir: %w", err)
	}

	switch s.FilesystemMode {
	case "off":
		return nil
	case "workspace-only":
		if !strings.HasPrefix(absPath, absWorkDir) {
			return fmt.Errorf("path outside workspace: %s", path)
		}
	case "allow-list":
		for _, allowed := range s.AllowedPaths {
			if strings.HasPrefix(absPath, allowed) {
				return nil
			}
			return fmt.Errorf("path not in allow list: %s", path)
		}
	}

	return nil
}

func (s *SandboxConfig) IsPathAllowed(path string) bool {
	return s.ValidatePath(path) == nil
}

func ParsePermissionRuleValue(s string) PermissionRuleValue {
	parts := strings.SplitN(s, "(", 2)
	toolName := parts[0]
	var ruleContent string
	if len(parts) > 1 {
		ruleContent = strings.TrimSuffix(parts[1], ")")
	}
	return PermissionRuleValue{
		ToolName:    toolName,
		RuleContent: ruleContent,
	}
}

func (v PermissionRuleValue) String() string {
	if v.RuleContent != "" {
		return fmt.Sprintf("%s(%s)", v.ToolName, v.RuleContent)
	}
	return v.ToolName
}
