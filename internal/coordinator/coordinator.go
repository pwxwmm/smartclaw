package coordinator

import (
	"os"
	"strings"
)

type Mode string

const (
	ModeNormal      Mode = "normal"
	ModeCoordinator Mode = "coordinator"
)

type CoordinatorService struct {
	mode Mode
}

func NewCoordinatorService() *CoordinatorService {
	mode := ModeNormal
	if isCoordinatorMode() {
		mode = ModeCoordinator
	}
	return &CoordinatorService{mode: mode}
}

func isCoordinatorMode() bool {
	return os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") == "1" ||
		os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") == "true"
}

func (s *CoordinatorService) GetMode() Mode {
	return s.mode
}

func (s *CoordinatorService) IsCoordinator() bool {
	return s.mode == ModeCoordinator
}

func (s *CoordinatorService) SetMode(mode Mode) {
	s.mode = mode
	if mode == ModeCoordinator {
		os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	} else {
		os.Unsetenv("CLAUDE_CODE_COORDINATOR_MODE")
	}
}

func (s *CoordinatorService) MatchSessionMode(sessionMode Mode) (bool, string) {
	if sessionMode == "" {
		return false, ""
	}

	sessionIsCoordinator := sessionMode == ModeCoordinator
	currentIsCoordinator := s.IsCoordinator()

	if currentIsCoordinator == sessionIsCoordinator {
		return false, ""
	}

	s.SetMode(sessionMode)

	if sessionIsCoordinator {
		return true, "Entered coordinator mode to match resumed session."
	}
	return true, "Exited coordinator mode to match resumed session."
}

func (s *CoordinatorService) GetWorkerToolsContext(mcpClients []string, scratchpadDir string) string {
	if !s.IsCoordinator() {
		return ""
	}

	var sb strings.Builder

	workerTools := []string{
		"bash",
		"read_file",
		"write_file",
		"edit_file",
		"glob",
		"grep",
		"web_fetch",
		"web_search",
		"lsp",
	}

	sb.WriteString("Workers spawned via the agent tool have access to these tools: ")
	sb.WriteString(strings.Join(workerTools, ", "))

	if len(mcpClients) > 0 {
		sb.WriteString("\n\nWorkers also have access to MCP tools from connected MCP servers: ")
		sb.WriteString(strings.Join(mcpClients, ", "))
	}

	if scratchpadDir != "" {
		sb.WriteString("\n\nScratchpad directory: ")
		sb.WriteString(scratchpadDir)
		sb.WriteString("\nWorkers can read and write here without permission prompts.")
	}

	return sb.String()
}

func (s *CoordinatorService) GetCoordinatorSystemPrompt() string {
	if !s.IsCoordinator() {
		return ""
	}

	return `You are Claude Code, an AI assistant that orchestrates software engineering tasks across multiple workers.

## Your Role

You are a coordinator. Your job is to:
- Help the user achieve their goal
- Direct workers to research, implement and verify code changes
- Synthesize results and communicate with the user
- Answer questions directly when possible

## Your Tools

- **agent** - Spawn a new worker
- **send_message** - Continue an existing worker
- **task_stop** - Stop a running worker

Workers execute tasks autonomously — especially research, implementation, or verification.

## Workers

Workers have access to standard tools, MCP tools, and project skills.

## Task Workflow

Most tasks can be broken down into phases:
- Research: Workers investigate codebase (parallel)
- Synthesis: Coordinator understands findings
- Implementation: Workers make targeted changes
- Verification: Workers test changes

Parallelism is your superpower. Launch independent workers concurrently whenever possible.`
}

func (s *CoordinatorService) GetAllowedTools() []string {
	if !s.IsCoordinator() {
		return nil
	}

	return []string{
		"agent",
		"send_message",
		"task_stop",
		"task_list",
		"task_get",
		"subscribe_pr_activity",
		"unsubscribe_pr_activity",
	}
}
