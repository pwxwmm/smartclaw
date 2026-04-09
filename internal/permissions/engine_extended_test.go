package permissions

import (
	"testing"
)

func TestEngineCheckDefaultReadOnly(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	readOnlyTools := []string{"read_file", "glob", "grep", "web_fetch", "web_search", "lsp"}
	for _, tool := range readOnlyTools {
		decision := engine.Check(tool, map[string]interface{}{})
		if decision.Behavior != PermissionBehaviorAllow {
			t.Errorf("Expected allow for read-only tool %s in default mode, got %s", tool, decision.Behavior)
		}
	}
}

func TestEngineCheckDefaultWrite(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	writeTools := []string{"bash", "write_file", "edit_file"}
	for _, tool := range writeTools {
		decision := engine.Check(tool, map[string]interface{}{})
		if decision.Behavior != PermissionBehaviorAsk {
			t.Errorf("Expected ask for write tool %s in default mode, got %s", tool, decision.Behavior)
		}
	}
}

func TestEngineCheckAutoMode(t *testing.T) {
	engine := NewEngine(PermissionModeAuto, "/tmp")

	decision := engine.Check("read_file", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAllow {
		t.Errorf("Expected allow for read_file in auto mode, got %s", decision.Behavior)
	}

	decision = engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAsk {
		t.Errorf("Expected ask for bash in auto mode, got %s", decision.Behavior)
	}
}

func TestEngineCheckAcceptEditsMode(t *testing.T) {
	engine := NewEngine(PermissionModeAcceptEdits, "/tmp")

	editTools := []string{"edit_file", "write_file", "bash"}
	for _, tool := range editTools {
		decision := engine.Check(tool, map[string]interface{}{})
		if decision.Behavior != PermissionBehaviorAllow {
			t.Errorf("Expected allow for edit tool %s in acceptEdits mode, got %s", tool, decision.Behavior)
		}
	}

	decision := engine.Check("read_file", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAllow {
		t.Errorf("Expected allow for read_file in acceptEdits mode, got %s", decision.Behavior)
	}
}

func TestEngineAddRuleAllow(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	rule := PermissionRule{
		Source:       PermissionSourceUser,
		RuleBehavior: PermissionBehaviorAllow,
		RuleValue: PermissionRuleValue{
			ToolName: "bash",
		},
	}

	engine.AddRule(PermissionSourceUser, rule)

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAllow {
		t.Errorf("Expected allow for bash with allow rule, got %s", decision.Behavior)
	}
}

func TestEngineAddRuleDeny(t *testing.T) {
	engine := NewEngine(PermissionModeBypassPermissions, "/tmp")

	rule := PermissionRule{
		Source:       PermissionSourceUser,
		RuleBehavior: PermissionBehaviorDeny,
		RuleValue: PermissionRuleValue{
			ToolName: "bash",
		},
	}

	engine.AddRule(PermissionSourceUser, rule)

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorDeny {
		t.Errorf("Expected deny for bash with deny rule, got %s", decision.Behavior)
	}
}

func TestEngineAddRuleAsk(t *testing.T) {
	engine := NewEngine(PermissionModeBypassPermissions, "/tmp")

	rule := PermissionRule{
		Source:       PermissionSourceUser,
		RuleBehavior: PermissionBehaviorAsk,
		RuleValue: PermissionRuleValue{
			ToolName: "bash",
		},
	}

	engine.AddRule(PermissionSourceUser, rule)

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAsk {
		t.Errorf("Expected ask for bash with ask rule, got %s", decision.Behavior)
	}
}

func TestEngineRecordDecision(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	engine.RecordDecision("bash", PermissionAllow)

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAllow {
		t.Errorf("Expected allow for bash with recorded decision, got %s", decision.Behavior)
	}
}

func TestEngineClearSessionDecisions(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	engine.RecordDecision("bash", PermissionAllow)
	engine.ClearSessionDecisions()

	decision := engine.Check("bash", map[string]interface{}{})
	if decision.Behavior != PermissionBehaviorAsk {
		t.Errorf("Expected ask for bash after clearing decisions, got %s", decision.Behavior)
	}
}

func TestEngineSetMode(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	engine.SetMode(PermissionModePlan)

	if engine.GetMode() != PermissionModePlan {
		t.Errorf("Expected mode plan, got %s", engine.GetMode())
	}
}

func TestEngineGetAllowRules(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	rule := PermissionRule{
		Source:       PermissionSourceUser,
		RuleBehavior: PermissionBehaviorAllow,
		RuleValue: PermissionRuleValue{
			ToolName: "bash",
		},
	}

	engine.AddRule(PermissionSourceUser, rule)

	rules := engine.GetAllowRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 allow rule, got %d", len(rules))
	}
}

func TestEngineGetDenyRules(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	rule := PermissionRule{
		Source:       PermissionSourceUser,
		RuleBehavior: PermissionBehaviorDeny,
		RuleValue: PermissionRuleValue{
			ToolName: "bash",
		},
	}

	engine.AddRule(PermissionSourceUser, rule)

	rules := engine.GetDenyRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 deny rule, got %d", len(rules))
	}
}

func TestMatchRuleExact(t *testing.T) {
	rule := PermissionRule{
		RuleValue: PermissionRuleValue{
			ToolName: "bash",
		},
	}

	if !matchRule(rule, "bash", nil) {
		t.Error("Expected match for exact tool name")
	}

	if matchRule(rule, "read_file", nil) {
		t.Error("Expected no match for different tool name")
	}
}

func TestMatchRuleWildcard(t *testing.T) {
	rule := PermissionRule{
		RuleValue: PermissionRuleValue{
			ToolName: "*",
		},
	}

	if !matchRule(rule, "bash", nil) {
		t.Error("Expected match for wildcard")
	}

	if !matchRule(rule, "read_file", nil) {
		t.Error("Expected match for wildcard")
	}
}

func TestMatchRulePrefix(t *testing.T) {
	rule := PermissionRule{
		RuleValue: PermissionRuleValue{
			ToolName: "lsp*",
		},
	}

	if !matchRule(rule, "lsp", nil) {
		t.Error("Expected match for prefix wildcard")
	}

	if !matchRule(rule, "lsp_goto_definition", nil) {
		t.Error("Expected match for prefix wildcard")
	}

	if matchRule(rule, "read_file", nil) {
		t.Error("Expected no match for different prefix")
	}
}

func TestMatchRuleSuffix(t *testing.T) {
	rule := PermissionRule{
		RuleValue: PermissionRuleValue{
			ToolName: "*_file",
		},
	}

	if !matchRule(rule, "read_file", nil) {
		t.Error("Expected match for suffix wildcard")
	}

	if !matchRule(rule, "write_file", nil) {
		t.Error("Expected match for suffix wildcard")
	}

	if matchRule(rule, "bash", nil) {
		t.Error("Expected no match for different suffix")
	}
}

func TestIsReadOnlyTool(t *testing.T) {
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

	for tool, expected := range readOnlyTools {
		result := isReadOnlyTool(tool)
		if result != expected {
			t.Errorf("Expected isReadOnlyTool(%s) = %v, got %v", tool, expected, result)
		}
	}

	if isReadOnlyTool("bash") {
		t.Error("Expected bash to not be read-only")
	}
}

func TestIsEditTool(t *testing.T) {
	editTools := map[string]bool{
		"edit_file":  true,
		"write_file": true,
		"bash":       true,
		"powershell": true,
	}

	for tool, expected := range editTools {
		result := isEditTool(tool)
		if result != expected {
			t.Errorf("Expected isEditTool(%s) = %v, got %v", tool, expected, result)
		}
	}

	if isEditTool("read_file") {
		t.Error("Expected read_file to not be edit tool")
	}
}

func TestSandboxConfig(t *testing.T) {
	config := NewSandboxConfig("/workspace")

	if !config.Enabled {
		t.Error("Expected sandbox to be enabled")
	}

	if config.FilesystemMode != "workspace-only" {
		t.Errorf("Expected filesystem mode 'workspace-only', got '%s'", config.FilesystemMode)
	}
}

func TestSandboxConfigValidatePath(t *testing.T) {
	config := NewSandboxConfig("/workspace")

	err := config.ValidatePath("/workspace/test.txt")
	if err != nil {
		t.Errorf("Expected valid path, got error: %v", err)
	}

	err = config.ValidatePath("/outside/workspace/test.txt")
	if err == nil {
		t.Error("Expected error for path outside workspace")
	}
}

func TestSandboxConfigIsPathAllowed(t *testing.T) {
	config := NewSandboxConfig("/workspace")

	if !config.IsPathAllowed("/workspace/test.txt") {
		t.Error("Expected path to be allowed")
	}

	if config.IsPathAllowed("/outside/workspace/test.txt") {
		t.Error("Expected path to not be allowed")
	}
}

func TestSandboxConfigDisabled(t *testing.T) {
	config := NewSandboxConfig("/workspace")
	config.Enabled = false

	err := config.ValidatePath("/any/path/test.txt")
	if err != nil {
		t.Errorf("Expected no error when sandbox disabled, got: %v", err)
	}
}

func TestSandboxConfigOffMode(t *testing.T) {
	config := NewSandboxConfig("/workspace")
	config.FilesystemMode = "off"

	err := config.ValidatePath("/any/path/test.txt")
	if err != nil {
		t.Errorf("Expected no error in off mode, got: %v", err)
	}
}

func TestSandboxConfigAllowList(t *testing.T) {
	config := NewSandboxConfig("/workspace")
	config.FilesystemMode = "allow-list"
	config.AllowedPaths = []string{"/allowed"}

	err := config.ValidatePath("/allowed/test.txt")
	if err != nil {
		t.Errorf("Expected valid path in allow list, got error: %v", err)
	}

	err = config.ValidatePath("/not-allowed/test.txt")
	if err == nil {
		t.Error("Expected error for path not in allow list")
	}
}

func TestParsePermissionRuleValue(t *testing.T) {
	value := ParsePermissionRuleValue("bash")

	if value.ToolName != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", value.ToolName)
	}

	if value.RuleContent != "" {
		t.Errorf("Expected empty rule content, got '%s'", value.RuleContent)
	}
}

func TestParsePermissionRuleValueWithContent(t *testing.T) {
	value := ParsePermissionRuleValue("bash(safe commands)")

	if value.ToolName != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", value.ToolName)
	}

	if value.RuleContent != "safe commands" {
		t.Errorf("Expected rule content 'safe commands', got '%s'", value.RuleContent)
	}
}

func TestPermissionRuleValueString(t *testing.T) {
	value := PermissionRuleValue{
		ToolName: "bash",
	}

	if value.String() != "bash" {
		t.Errorf("Expected string 'bash', got '%s'", value.String())
	}

	value.RuleContent = "safe commands"
	if value.String() != "bash(safe commands)" {
		t.Errorf("Expected string 'bash(safe commands)', got '%s'", value.String())
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		expect  bool
	}{
		{"*", "anything", true},
		{"bash", "bash", true},
		{"bash", "read_file", false},
		{"lsp*", "lsp", true},
		{"lsp*", "lsp_goto_definition", true},
		{"lsp*", "read_file", false},
		{"*_file", "read_file", true},
		{"*_file", "write_file", true},
		{"*_file", "bash", false},
	}

	for _, test := range tests {
		result := matchPattern(test.pattern, test.name)
		if result != test.expect {
			t.Errorf("matchPattern(%s, %s) = %v, expected %v", test.pattern, test.name, result, test.expect)
		}
	}
}

func TestPermissionDecisionReasonMode(t *testing.T) {
	engine := NewEngine(PermissionModePlan, "/tmp")

	decision := engine.Check("bash", map[string]interface{}{})

	if decision.Reason == nil {
		t.Fatal("Expected non-nil reason")
	}

	if decision.Reason.Type != "mode" {
		t.Errorf("Expected reason type 'mode', got '%s'", decision.Reason.Type)
	}

	if decision.Reason.Mode != PermissionModePlan {
		t.Errorf("Expected reason mode 'plan', got '%s'", decision.Reason.Mode)
	}
}

func TestPermissionDecisionReasonRule(t *testing.T) {
	engine := NewEngine(PermissionModeDefault, "/tmp")

	rule := PermissionRule{
		Source:       PermissionSourceUser,
		RuleBehavior: PermissionBehaviorAllow,
		RuleValue: PermissionRuleValue{
			ToolName: "bash",
		},
	}
	engine.AddRule(PermissionSourceUser, rule)

	decision := engine.Check("bash", map[string]interface{}{})

	if decision.Reason == nil {
		t.Fatal("Expected non-nil reason")
	}

	if decision.Reason.Type != "rule" {
		t.Errorf("Expected reason type 'rule', got '%s'", decision.Reason.Type)
	}

	if decision.Reason.Rule == nil {
		t.Fatal("Expected non-nil rule in reason")
	}

	if decision.Reason.Rule.RuleValue.ToolName != "bash" {
		t.Errorf("Expected rule tool name 'bash', got '%s'", decision.Reason.Rule.RuleValue.ToolName)
	}
}
