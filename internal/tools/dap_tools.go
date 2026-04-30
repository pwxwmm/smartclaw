package tools

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/instructkr/smartclaw/internal/dap"
)

type DapTool struct{ BaseTool }

func (t *DapTool) Name() string { return "dap" }
func (t *DapTool) Description() string {
	return "DAP debugger: start/stop debug sessions, set breakpoints, step through code, inspect variables"
}

func (t *DapTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{
					"start", "stop", "set_breakpoint", "continue",
					"step_over", "step_into", "step_out",
					"inspect", "evaluate", "stack_trace",
				},
			},
			"session_id": map[string]any{"type": "string"},
			"file":       map[string]any{"type": "string"},
			"line":       map[string]any{"type": "integer"},
			"expression": map[string]any{"type": "string"},
			"condition":  map[string]any{"type": "string"},
		},
		"required": []string{"operation"},
	}
}

func (t *DapTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	operation, _ := input["operation"].(string)

	if operation != "start" {
		if _, err := exec.LookPath("dlv"); err != nil {
			return nil, fmt.Errorf("delve (dlv) is not installed or not in PATH")
		}
	}

	switch operation {
	case "start":
		return t.startSession(input)
	case "stop":
		return t.stopSession(input)
	case "set_breakpoint":
		return t.setBreakpoint(input)
	case "continue":
		return t.continueExec(input)
	case "step_over":
		return t.stepOver(input)
	case "step_into":
		return t.stepInto(input)
	case "step_out":
		return t.stepOut(input)
	case "inspect":
		return t.inspect(input)
	case "evaluate":
		return t.evaluate(input)
	case "stack_trace":
		return t.stackTrace(input)
	default:
		return nil, &Error{Code: "INVALID_OPERATION", Message: "Unknown operation: " + operation}
	}
}

func (t *DapTool) startSession(input map[string]any) (any, error) {
	programPath, _ := input["file"].(string)
	if programPath == "" {
		return nil, ErrRequiredField("file")
	}

	if _, err := exec.LookPath("dlv"); err != nil {
		return nil, fmt.Errorf("delve (dlv) is not installed or not in PATH")
	}

	session, err := dapMgr.CreateSession(programPath)
	if err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Failed to start debug session: %v", err)}
	}

	return map[string]any{
		"session_id":   session.ID,
		"program":      session.ProgramPath,
		"status":       "started",
		"created_at":   session.CreatedAt,
	}, nil
}

func (t *DapTool) stopSession(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}

	if err := dapMgr.CloseSession(sessionID); err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Failed to stop session: %v", err)}
	}

	return map[string]any{
		"session_id": sessionID,
		"status":     "stopped",
	}, nil
}

func (t *DapTool) setBreakpoint(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	file, _ := input["file"].(string)
	line, _ := input["line"].(int)
	condition, _ := input["condition"].(string)

	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}
	if file == "" {
		return nil, ErrRequiredField("file")
	}
	if line == 0 {
		return nil, ErrRequiredField("line")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	source := dap.Source{Name: file, Path: file}
	breakpoints, err := session.Client.SetBreakpoints(source, []int{line})
	if err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Failed to set breakpoint: %v", err)}
	}

	if len(breakpoints) > 0 {
		if condition != "" {
			breakpoints[0].Condition = condition
		}
		session.ActiveBreakpoints[breakpoints[0].ID] = breakpoints[0]
	}

	return map[string]any{
		"session_id":  sessionID,
		"breakpoints": breakpoints,
		"count":       len(breakpoints),
	}, nil
}

func (t *DapTool) continueExec(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	if err := session.Client.Continue(); err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Continue failed: %v", err)}
	}

	return map[string]any{
		"session_id": sessionID,
		"status":     "running",
	}, nil
}

func (t *DapTool) stepOver(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	if err := session.Client.Next(); err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Step over failed: %v", err)}
	}

	return map[string]any{
		"session_id": sessionID,
		"status":     "paused",
	}, nil
}

func (t *DapTool) stepInto(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	if err := session.Client.StepIn(); err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Step into failed: %v", err)}
	}

	return map[string]any{
		"session_id": sessionID,
		"status":     "paused",
	}, nil
}

func (t *DapTool) stepOut(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	if err := session.Client.StepOut(); err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Step out failed: %v", err)}
	}

	return map[string]any{
		"session_id": sessionID,
		"status":     "paused",
	}, nil
}

func (t *DapTool) inspect(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	frames, err := session.Client.GetStackTrace(1)
	if err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Stack trace failed: %v", err)}
	}

	if len(frames) == 0 {
		return map[string]any{
			"session_id": sessionID,
			"variables":  []any{},
			"message":    "no stack frames available (program may not be paused)",
		}, nil
	}

	session.CurrentFrame = &frames[0]

	scopes, err := session.Client.GetScopes(frames[0].ID)
	if err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Get scopes failed: %v", err)}
	}

	var allVars []dap.Variable
	for _, scope := range scopes {
		vars, err := session.Client.GetVariables(scope.VariablesReference)
		if err != nil {
			continue
		}
		allVars = append(allVars, vars...)
	}

	return map[string]any{
		"session_id":   sessionID,
		"frame":        frames[0],
		"scopes":       scopes,
		"variables":    allVars,
		"var_count":    len(allVars),
	}, nil
}

func (t *DapTool) evaluate(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	expression, _ := input["expression"].(string)

	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}
	if expression == "" {
		return nil, ErrRequiredField("expression")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	frameID := 0
	if session.CurrentFrame != nil {
		frameID = session.CurrentFrame.ID
	}

	result, err := session.Client.Evaluate(expression, frameID)
	if err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Evaluate failed: %v", err)}
	}

	return map[string]any{
		"session_id": sessionID,
		"expression": expression,
		"result":     result,
	}, nil
}

func (t *DapTool) stackTrace(input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, ErrRequiredField("session_id")
	}

	session, ok := dapMgr.GetSession(sessionID)
	if !ok {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Session %s not found", sessionID)}
	}

	frames, err := session.Client.GetStackTrace(1)
	if err != nil {
		return nil, &Error{Code: "DAP_ERROR", Message: fmt.Sprintf("Stack trace failed: %v", err)}
	}

	return map[string]any{
		"session_id":   sessionID,
		"stack_frames": frames,
		"frame_count":  len(frames),
	}, nil
}

var dapMgr = dap.DefaultSessionManager
