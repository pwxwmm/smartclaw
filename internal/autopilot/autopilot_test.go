package autopilot

import "testing"

func assertAction(t *testing.T, got, want Action, toolName string) {
	t.Helper()
	if got != want {
		t.Errorf("Decide(%q): action = %q, want %q", toolName, got, want)
	}
}

func TestTrustLevelOff_AllToolsRequireAsking(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelOff, "/workspace")

	tools := []string{"read_file", "write_file", "bash", "agent", "mcp"}
	for _, tool := range tools {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAsk, tool)
		if dec.RiskScore != 1.0 {
			t.Errorf("Decide(%q): RiskScore = %f, want 1.0", tool, dec.RiskScore)
		}
	}

	stats := ae.GetStats()
	if stats.EscalatedToUser != len(tools) {
		t.Errorf("EscalatedToUser = %d, want %d", stats.EscalatedToUser, len(tools))
	}
	if stats.AutoApproved != 0 {
		t.Errorf("AutoApproved = %d, want 0", stats.AutoApproved)
	}
}

func TestTrustLevelRead_ReadOnlyToolsApproved(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelRead, "/workspace")

	readTools := []string{"read_file", "glob", "grep", "web_search", "web_fetch", "think", "lsp", "ast_grep", "tool_search", "memory", "skill", "git_status", "git_diff", "git_log"}
	for _, tool := range readTools {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAllow, tool)
	}

	stats := ae.GetStats()
	if stats.AutoApproved != len(readTools) {
		t.Errorf("AutoApproved = %d, want %d", stats.AutoApproved, len(readTools))
	}
}

func TestTrustLevelRead_WriteToolsDenied(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelRead, "/workspace")

	writeTools := []string{"write_file", "edit_file", "todowrite", "session", "bash", "execute_code", "docker_exec", "agent", "mcp"}
	for _, tool := range writeTools {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAsk, tool)
	}
}

func TestTrustLevelWrite_ReadAndWriteApproved(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelWrite, "/workspace")

	approved := []string{"read_file", "glob", "grep", "write_file", "edit_file", "todowrite", "session"}
	for _, tool := range approved {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAllow, tool)
	}
}

func TestTrustLevelWrite_ExecuteToolsAsk(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelWrite, "/workspace")

	executeTools := []string{"bash", "execute_code", "docker_exec", "agent", "mcp"}
	for _, tool := range executeTools {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAsk, tool)
	}
}

func TestTrustLevelExecute_ReadWriteExecuteApproved(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelExecute, "/workspace")

	approved := []string{"read_file", "glob", "write_file", "edit_file", "todowrite", "session", "bash", "execute_code", "docker_exec", "git_ai"}
	for _, tool := range approved {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAllow, tool)
	}
}

func TestTrustLevelExecute_DangerousToolsAsk(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelExecute, "/workspace")

	dangerous := []string{"agent", "mcp", "powershell"}
	for _, tool := range dangerous {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAsk, tool)
	}
}

func TestTrustLevelFull_EverythingApprovedExceptBrowser(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelFull, "/workspace")

	approved := []string{"read_file", "glob", "grep", "write_file", "edit_file", "bash", "execute_code", "agent", "mcp", "powershell", "docker_exec", "git_ai"}
	for _, tool := range approved {
		dec := ae.Decide(tool, nil)
		assertAction(t, dec.Action, ActionAllow, tool)
	}

	dec := ae.Decide("browser_navigate", nil)
	assertAction(t, dec.Action, ActionDeny, "browser_navigate")
}

func TestDenyTool_OverridesTrustLevel(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelFull, "/workspace")
	ae.DenyTool("bash")

	dec := ae.Decide("bash", nil)
	assertAction(t, dec.Action, ActionDeny, "bash")

	dec = ae.Decide("read_file", nil)
	assertAction(t, dec.Action, ActionAllow, "read_file")
}

func TestDenyPath_BlocksFileOperations(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelWrite, "/workspace")
	ae.DenyPath("/etc/")

	dec := ae.Decide("write_file", map[string]any{"path": "/etc/passwd"})
	assertAction(t, dec.Action, ActionDeny, "write_file")

	dec = ae.Decide("write_file", map[string]any{"path": "/workspace/file.go"})
	assertAction(t, dec.Action, ActionAllow, "write_file")

	dec = ae.Decide("read_file", map[string]any{"path": "/etc/hosts"})
	assertAction(t, dec.Action, ActionDeny, "read_file")
}

func TestDenyPath_NoPathInInput(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelExecute, "/workspace")
	ae.DenyPath("/etc/")

	dec := ae.Decide("bash", map[string]any{"command": "ls /etc"})
	assertAction(t, dec.Action, ActionAllow, "bash")
}

func TestMaxAutoActions_TriggersEscalation(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelRead, "/workspace")
	ae.SetMaxAutoActions(2)

	dec := ae.Decide("read_file", nil)
	assertAction(t, dec.Action, ActionAllow, "read_file")

	dec = ae.Decide("glob", nil)
	assertAction(t, dec.Action, ActionAllow, "glob")

	dec = ae.Decide("read_file", nil)
	assertAction(t, dec.Action, ActionAsk, "read_file")
	if dec.Reason != "reached max auto-actions (2), requiring confirmation" {
		t.Errorf("unexpected reason: %s", dec.Reason)
	}
}

func TestAddRule_CustomRule(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelRead, "/workspace")

	dec := ae.Decide("custom_tool", nil)
	assertAction(t, dec.Action, ActionAsk, "custom_tool")

	ae.AddRule(AutoRule{
		ToolName:  "custom_tool",
		Action:    ActionAllow,
		Condition: "custom tool auto-approval",
		TrustMin:  TrustLevelRead,
		RiskScore: 0.2,
	})

	dec = ae.Decide("custom_tool", nil)
	assertAction(t, dec.Action, ActionAllow, "custom_tool")
	if dec.RiskScore != 0.2 {
		t.Errorf("RiskScore = %f, want 0.2", dec.RiskScore)
	}
}

func TestAddRule_CustomDenyRule(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelFull, "/workspace")

	ae.AddRule(AutoRule{
		ToolName:  "dangerous_tool",
		Action:    ActionDeny,
		Condition: "explicitly denied by policy",
		TrustMin:  TrustLevelFull,
		RiskScore: 1.0,
	})

	dec := ae.Decide("dangerous_tool", nil)
	assertAction(t, dec.Action, ActionDeny, "dangerous_tool")
}

func TestGetStats_TracksCorrectMetrics(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelWrite, "/workspace")

	ae.Decide("read_file", nil)
	ae.Decide("write_file", nil)
	ae.Decide("agent", nil)
	ae.Decide("bash", nil)

	stats := ae.GetStats()
	if stats.TotalDecisions != 4 {
		t.Errorf("TotalDecisions = %d, want 4", stats.TotalDecisions)
	}
	if stats.AutoApproved != 2 {
		t.Errorf("AutoApproved = %d, want 2", stats.AutoApproved)
	}
	if stats.EscalatedToUser != 2 {
		t.Errorf("EscalatedToUser = %d, want 2", stats.EscalatedToUser)
	}
	if stats.PromptsSaved != 2 {
		t.Errorf("PromptsSaved = %d, want 2", stats.PromptsSaved)
	}
}

func TestNoMatchingRule_DefaultsToAsk(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelRead, "/workspace")

	dec := ae.Decide("unknown_tool_xyz", nil)
	assertAction(t, dec.Action, ActionAsk, "unknown_tool_xyz")
	if dec.RiskScore != 0.5 {
		t.Errorf("RiskScore = %f, want 0.5", dec.RiskScore)
	}
}

func TestSessionPolicy_TracksTotalSaved(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelWrite, "/workspace")

	for i := 0; i < 5; i++ {
		ae.Decide("read_file", nil)
	}

	if ae.GetTotalSaved() != 5 {
		t.Errorf("totalSaved = %d, want 5", ae.GetTotalSaved())
	}
}

func TestSetTrustLevel(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelOff, "/workspace")

	dec := ae.Decide("read_file", nil)
	assertAction(t, dec.Action, ActionAsk, "read_file")

	ae.SetTrustLevel(TrustLevelRead)
	if ae.GetTrustLevel() != TrustLevelRead {
		t.Errorf("GetTrustLevel() = %d, want %d", ae.GetTrustLevel(), TrustLevelRead)
	}

	dec = ae.Decide("read_file", nil)
	assertAction(t, dec.Action, ActionAllow, "read_file")
}

func TestAllowPath(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelWrite, "/workspace")
	ae.AllowPath("/tmp/safe")

	if len(ae.policy.AllowedPaths) != 1 || ae.policy.AllowedPaths[0] != "/tmp/safe" {
		t.Errorf("AllowedPaths = %v, want [/tmp/safe]", ae.policy.AllowedPaths)
	}
}

func TestTrustLevelString(t *testing.T) {
	tests := map[TrustLevel]string{
		TrustLevelOff:     "off",
		TrustLevelRead:    "read",
		TrustLevelWrite:   "write",
		TrustLevelExecute: "execute",
		TrustLevelFull:    "full",
		TrustLevel(99):    "unknown",
	}
	for level, want := range tests {
		if got := level.String(); got != want {
			t.Errorf("TrustLevel(%d).String() = %q, want %q", level, got, want)
		}
	}
}

func TestAutoDecision_Fields(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelRead, "/workspace")

	dec := ae.Decide("read_file", nil)
	if dec.ToolName != "read_file" {
		t.Errorf("ToolName = %q, want %q", dec.ToolName, "read_file")
	}
	if dec.TrustUsed != TrustLevelRead {
		t.Errorf("TrustUsed = %d, want %d", dec.TrustUsed, TrustLevelRead)
	}
	if dec.Reason != "read-only operation" {
		t.Errorf("Reason = %q, want %q", dec.Reason, "read-only operation")
	}
}

func TestDenyTool_CumulativeDenials(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelFull, "/workspace")
	ae.DenyTool("bash")
	ae.DenyTool("powershell")
	ae.DenyTool("agent")

	stats := ae.GetStats()
	ae.Decide("bash", nil)
	ae.Decide("powershell", nil)
	ae.Decide("agent", nil)

	stats = ae.GetStats()
	if stats.AutoDenied != 3 {
		t.Errorf("AutoDenied = %d, want 3", stats.AutoDenied)
	}
}

func TestMaxAutoActions_ZeroMeansUnlimited(t *testing.T) {
	ae := NewAutopilotEngine(TrustLevelRead, "/workspace")
	ae.SetMaxAutoActions(0)

	for i := 0; i < 100; i++ {
		dec := ae.Decide("read_file", nil)
		assertAction(t, dec.Action, ActionAllow, "read_file")
	}
}

func TestBrowserNavigate_AlwaysDenied(t *testing.T) {
	for _, level := range []TrustLevel{TrustLevelOff, TrustLevelRead, TrustLevelWrite, TrustLevelExecute, TrustLevelFull} {
		ae := NewAutopilotEngine(level, "/workspace")
		dec := ae.Decide("browser_navigate", nil)

		if level == TrustLevelOff {
			assertAction(t, dec.Action, ActionAsk, "browser_navigate")
		} else {
			assertAction(t, dec.Action, ActionDeny, "browser_navigate")
		}
	}
}
