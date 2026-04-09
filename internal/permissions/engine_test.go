package permissions

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")
	if engine == nil {
		t.Fatal("Expected non-nil engine")
	}

	if engine.mode != PermissionModeDefault {
		t.Errorf("Expected mode %s, got %s", PermissionModeDefault, engine.mode)
	}
}

func TestEngineCheckBypass(t *testing.T) {
	engine := NewEngine(PermissionModeBypassPermissions, "/tmp")

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAllow {
		t.Errorf("Expected allow for bypass mode, got %s", decision.Behavior)
	}
}

func TestEngineCheckDontAsk(t *testing.T) {
	engine := NewEngine(PermissionModeDontAsk, "/tmp")

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAllow {
		t.Errorf("Expected allow for dontAsk mode, got %s", decision.Behavior)
	}
}

func TestEngineCheckPlan(t *testing.T) {
	engine := NewEngine(PermissionModePlan, "/tmp")

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAsk {
		t.Errorf("Expected ask for plan mode, got %s", decision.Behavior)
	}
}

func TestPermissionMode(t *testing.T) {
	modes := []PermissionMode{
		PermissionModeDefault,
		PermissionModePlan,
		PermissionModeAcceptEdits,
		PermissionModeBypassPermissions,
		PermissionModeDontAsk,
		PermissionModeAuto,
	}

	for _, mode := range modes {
		if mode == "" {
			t.Error("Permission mode should not be empty")
		}
	}
}

func TestPermissionBehavior(t *testing.T) {
	behaviors := []PermissionBehavior{
		PermissionBehaviorAllow,
		PermissionBehaviorDeny,
		PermissionBehaviorAsk,
		PermissionBehaviorPassthrough,
	}

	for _, behavior := range behaviors {
		if behavior == "" {
			t.Error("Permission behavior should not be empty")
		}
	}
}

func TestPermissionResult(t *testing.T) {
	results := []PermissionResult{
		PermissionAllow,
		PermissionDeny,
		PermissionAsk,
	}

	for _, result := range results {
		if result == "" {
			t.Error("Permission result should not be empty")
		}
	}
}

func TestPermissionRuleSource(t *testing.T) {
	sources := []PermissionRuleSource{
		PermissionSourceProject,
		PermissionSourceUser,
		PermissionSourceLocal,
		PermissionSourceCliArg,
		PermissionSourceCommand,
		PermissionSourceSession,
	}

	for _, source := range sources {
		if source == "" {
			t.Error("Permission source should not be empty")
		}
	}
}

func TestPermissionRule(t *testing.T) {
	rule := PermissionRule{
		Source:       PermissionSourceProject,
		RuleBehavior: PermissionBehaviorAllow,
		RuleValue: PermissionRuleValue{
			ToolName:    "bash",
			RuleContent: "allow all bash commands",
		},
	}

	if rule.Source != PermissionSourceProject {
		t.Errorf("Expected source project, got %s", rule.Source)
	}

	if rule.RuleBehavior != PermissionBehaviorAllow {
		t.Errorf("Expected behavior allow, got %s", rule.RuleBehavior)
	}

	if rule.RuleValue.ToolName != "bash" {
		t.Errorf("Expected tool name bash, got %s", rule.RuleValue.ToolName)
	}
}

func TestPermissionDecision(t *testing.T) {
	decision := PermissionDecision{
		Behavior: PermissionBehaviorAllow,
		Reason: &PermissionDecisionReason{
			Type:   "test",
			Reason: "test reason",
		},
	}

	if decision.Behavior != PermissionBehaviorAllow {
		t.Errorf("Expected behavior allow, got %s", decision.Behavior)
	}

	if decision.Reason == nil {
		t.Error("Expected non-nil reason")
	}
}

func TestPermissionDecisionReason(t *testing.T) {
	reason := PermissionDecisionReason{
		Type:       "rule_match",
		Reason:     "Matched allow rule",
		Mode:       PermissionModeDefault,
		HookName:   "test-hook",
		Classifier: "auto",
	}

	if reason.Type != "rule_match" {
		t.Errorf("Expected type 'rule_match', got '%s'", reason.Type)
	}

	if reason.Mode != PermissionModeDefault {
		t.Errorf("Expected mode default, got %s", reason.Mode)
	}
}

func TestPermissionRuleValue(t *testing.T) {
	value := PermissionRuleValue{
		ToolName:    "read_file",
		RuleContent: "allow reading specific files",
	}

	if value.ToolName != "read_file" {
		t.Errorf("Expected tool 'read_file', got '%s'", value.ToolName)
	}
}
