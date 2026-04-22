package coordinator

import (
	"os"
	"strings"
	"testing"
)

func TestNewCoordinatorService_EnvOne(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	svc := NewCoordinatorService()
	if svc.GetMode() != ModeCoordinator {
		t.Errorf("expected ModeCoordinator, got %v", svc.GetMode())
	}
}

func TestNewCoordinatorService_EnvTrue(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "true")
	svc := NewCoordinatorService()
	if svc.GetMode() != ModeCoordinator {
		t.Errorf("expected ModeCoordinator, got %v", svc.GetMode())
	}
}

func TestNewCoordinatorService_EnvEmpty(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")
	svc := NewCoordinatorService()
	if svc.GetMode() != ModeNormal {
		t.Errorf("expected ModeNormal, got %v", svc.GetMode())
	}
}

func TestNewCoordinatorService_EnvOtherValue(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "false")
	svc := NewCoordinatorService()
	if svc.GetMode() != ModeNormal {
		t.Errorf("expected ModeNormal for value 'false', got %v", svc.GetMode())
	}
}

func TestGetMode(t *testing.T) {
	svc := &CoordinatorService{mode: ModeNormal}
	if svc.GetMode() != ModeNormal {
		t.Errorf("expected ModeNormal, got %v", svc.GetMode())
	}
	svc.mode = ModeCoordinator
	if svc.GetMode() != ModeCoordinator {
		t.Errorf("expected ModeCoordinator, got %v", svc.GetMode())
	}
}

func TestIsCoordinator(t *testing.T) {
	svc := &CoordinatorService{mode: ModeNormal}
	if svc.IsCoordinator() {
		t.Error("expected IsCoordinator() to be false for ModeNormal")
	}
	svc.mode = ModeCoordinator
	if !svc.IsCoordinator() {
		t.Error("expected IsCoordinator() to be true for ModeCoordinator")
	}
}

func TestSetMode_Coordinator(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")
	svc := &CoordinatorService{mode: ModeNormal}
	svc.SetMode(ModeCoordinator)
	if svc.GetMode() != ModeCoordinator {
		t.Errorf("expected ModeCoordinator after SetMode, got %v", svc.GetMode())
	}
	if os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") != "1" {
		t.Errorf("expected env CLAUDE_CODE_COORDINATOR_MODE=1, got %q", os.Getenv("CLAUDE_CODE_COORDINATOR_MODE"))
	}
}

func TestSetMode_Normal(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	svc := &CoordinatorService{mode: ModeCoordinator}
	svc.SetMode(ModeNormal)
	if svc.GetMode() != ModeNormal {
		t.Errorf("expected ModeNormal after SetMode, got %v", svc.GetMode())
	}
	if os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") != "" {
		t.Errorf("expected env CLAUDE_CODE_COORDINATOR_MODE to be unset, got %q", os.Getenv("CLAUDE_CODE_COORDINATOR_MODE"))
	}
}

func TestMatchSessionMode_SameMode(t *testing.T) {
	svc := &CoordinatorService{mode: ModeNormal}
	switched, msg := svc.MatchSessionMode(ModeNormal)
	if switched {
		t.Error("expected no switch when modes are same")
	}
	if msg != "" {
		t.Errorf("expected empty message, got %q", msg)
	}
}

func TestMatchSessionMode_DifferentModes_ToCoordinator(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")
	svc := &CoordinatorService{mode: ModeNormal}
	switched, msg := svc.MatchSessionMode(ModeCoordinator)
	if !switched {
		t.Error("expected switch when modes differ")
	}
	if !strings.Contains(msg, "coordinator") {
		t.Errorf("expected message to mention coordinator, got %q", msg)
	}
	if svc.GetMode() != ModeCoordinator {
		t.Errorf("expected mode to be switched to ModeCoordinator, got %v", svc.GetMode())
	}
}

func TestMatchSessionMode_DifferentModes_ToNormal(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	svc := &CoordinatorService{mode: ModeCoordinator}
	switched, msg := svc.MatchSessionMode(ModeNormal)
	if !switched {
		t.Error("expected switch when modes differ")
	}
	if !strings.Contains(msg, "Exited") {
		t.Errorf("expected message to mention exiting, got %q", msg)
	}
	if svc.GetMode() != ModeNormal {
		t.Errorf("expected mode to be switched to ModeNormal, got %v", svc.GetMode())
	}
}

func TestMatchSessionMode_EmptySessionMode(t *testing.T) {
	svc := &CoordinatorService{mode: ModeNormal}
	switched, msg := svc.MatchSessionMode("")
	if switched {
		t.Error("expected no switch when session mode is empty")
	}
	if msg != "" {
		t.Errorf("expected empty message, got %q", msg)
	}
}

func TestGetWorkerToolsContext_NotCoordinator(t *testing.T) {
	svc := &CoordinatorService{mode: ModeNormal}
	result := svc.GetWorkerToolsContext([]string{"mcp1"}, "/tmp/scratch")
	if result != "" {
		t.Errorf("expected empty string when not coordinator, got %q", result)
	}
}

func TestGetWorkerToolsContext_Coordinator(t *testing.T) {
	svc := &CoordinatorService{mode: ModeCoordinator}
	result := svc.GetWorkerToolsContext(nil, "")
	if !strings.Contains(result, "bash") {
		t.Error("expected result to list worker tools like bash")
	}
	if !strings.Contains(result, "Workers spawned") {
		t.Error("expected result to contain worker context intro")
	}
}

func TestGetWorkerToolsContext_WithMCPClients(t *testing.T) {
	svc := &CoordinatorService{mode: ModeCoordinator}
	result := svc.GetWorkerToolsContext([]string{"github", "filesystem"}, "")
	if !strings.Contains(result, "MCP tools") {
		t.Error("expected result to mention MCP tools")
	}
	if !strings.Contains(result, "github") || !strings.Contains(result, "filesystem") {
		t.Error("expected result to list MCP client names")
	}
}

func TestGetWorkerToolsContext_WithScratchpadDir(t *testing.T) {
	svc := &CoordinatorService{mode: ModeCoordinator}
	result := svc.GetWorkerToolsContext(nil, "/tmp/scratchpad")
	if !strings.Contains(result, "Scratchpad directory") {
		t.Error("expected result to mention scratchpad directory")
	}
	if !strings.Contains(result, "/tmp/scratchpad") {
		t.Error("expected result to contain scratchpad path")
	}
}

func TestGetWorkerToolsContext_Full(t *testing.T) {
	svc := &CoordinatorService{mode: ModeCoordinator}
	result := svc.GetWorkerToolsContext([]string{"mcpA"}, "/tmp/scratch")
	if !strings.Contains(result, "Workers spawned") {
		t.Error("expected worker tools intro")
	}
	if !strings.Contains(result, "MCP tools") {
		t.Error("expected MCP tools section")
	}
	if !strings.Contains(result, "Scratchpad directory") {
		t.Error("expected scratchpad section")
	}
}

func TestGetCoordinatorSystemPrompt_NotCoordinator(t *testing.T) {
	svc := &CoordinatorService{mode: ModeNormal}
	result := svc.GetCoordinatorSystemPrompt()
	if result != "" {
		t.Errorf("expected empty string when not coordinator, got %q", result)
	}
}

func TestGetCoordinatorSystemPrompt_Coordinator(t *testing.T) {
	svc := &CoordinatorService{mode: ModeCoordinator}
	result := svc.GetCoordinatorSystemPrompt()
	if result == "" {
		t.Error("expected non-empty prompt when coordinator")
	}
	if !strings.Contains(result, "coordinator") {
		t.Error("expected prompt to mention coordinator role")
	}
	if !strings.Contains(result, "agent") {
		t.Error("expected prompt to mention agent tool")
	}
}

func TestGetAllowedTools_NotCoordinator(t *testing.T) {
	svc := &CoordinatorService{mode: ModeNormal}
	tools := svc.GetAllowedTools()
	if tools != nil {
		t.Errorf("expected nil when not coordinator, got %v", tools)
	}
}

func TestGetAllowedTools_Coordinator(t *testing.T) {
	svc := &CoordinatorService{mode: ModeCoordinator}
	tools := svc.GetAllowedTools()
	if len(tools) != 7 {
		t.Errorf("expected 7 tools, got %d", len(tools))
	}
	expectedTools := map[string]bool{
		"agent": true, "send_message": true, "task_stop": true,
		"task_list": true, "task_get": true,
		"subscribe_pr_activity": true, "unsubscribe_pr_activity": true,
	}
	for _, tool := range tools {
		if !expectedTools[tool] {
			t.Errorf("unexpected tool %q", tool)
		}
	}
}
