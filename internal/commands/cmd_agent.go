package commands

import (
	"fmt"
	"os"
	"strings"
)

func init() {
	Register(Command{
		Name:    "agent",
		Summary: "Manage AI agents",
	}, agentHandler)

	Register(Command{
		Name:    "agent-list",
		Summary: "List available agents",
	}, agentListHandler)

	Register(Command{
		Name:    "agent-switch",
		Summary: "Switch to an agent",
	}, agentSwitchHandler)

	Register(Command{
		Name:    "agent-create",
		Summary: "Create custom agent",
	}, agentCreateHandler)

	Register(Command{
		Name:    "agent-delete",
		Summary: "Delete custom agent",
	}, agentDeleteHandler)

	Register(Command{
		Name:    "agent-info",
		Summary: "Show agent info",
	}, agentInfoHandler)

	Register(Command{
		Name:    "agent-export",
		Summary: "Export agent",
	}, agentExportHandler)

	Register(Command{
		Name:    "agent-import",
		Summary: "Import agent",
	}, agentImportHandler)

	Register(Command{
		Name:    "subagent",
		Summary: "Spawn subagent",
	}, subagentHandler)

	Register(Command{
		Name:    "agents",
		Summary: "List available agents",
	}, agentsHandler)
}

var globalAgentManager interface {
	GetCurrentAgent() *AgentInfo
	SetCurrentAgent(agentType string) error
	GetAgent(agentType string) (*AgentInfo, error)
	ListAgents() []*AgentInfo
	CreateCustomAgent(agent *AgentInfo) error
	DeleteCustomAgent(agentType string) error
	ExportAgent(agentType string, format string) (string, error)
	ImportAgent(data string, format string) error
	FormatAgentInfo(*AgentInfo) string
	FormatAgentList() string
}

type AgentInfo struct {
	AgentType       string
	WhenToUse       string
	Tools           []string
	DisallowedTools []string
	Skills          []string
	Model           string
	PermissionMode  string
	Color           string
	MaxTurns        int
	Memory          string
	Background      bool
	Source          string
	SystemPrompt    string
}

func SetGlobalAgentManager(am interface {
	GetCurrentAgent() *AgentInfo
	SetCurrentAgent(agentType string) error
	GetAgent(agentType string) (*AgentInfo, error)
	ListAgents() []*AgentInfo
	CreateCustomAgent(agent *AgentInfo) error
	DeleteCustomAgent(agentType string) error
	ExportAgent(agentType string, format string) (string, error)
	ImportAgent(data string, format string) error
	FormatAgentInfo(*AgentInfo) string
	FormatAgentList() string
}) {
	globalAgentManager = am
}

func agentHandler(args []string) error {
	if len(args) == 0 {
		return agentListHandler(args)
	}
	switch args[0] {
	case "list":
		return agentListHandler(args[1:])
	case "switch":
		return agentSwitchHandler(args[1:])
	case "create":
		return agentCreateHandler(args[1:])
	case "delete":
		return agentDeleteHandler(args[1:])
	case "info":
		return agentInfoHandler(args[1:])
	case "export":
		return agentExportHandler(args[1:])
	case "import":
		return agentImportHandler(args[1:])
	default:
		fmt.Printf("Unknown agent subcommand: %s\n", args[0])
		fmt.Println("Usage: /agent [list|switch|create|delete|info|export|import]")
		return nil
	}
}

func agentListHandler(args []string) error {
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	fmt.Print(globalAgentManager.FormatAgentList())
	return nil
}

func agentSwitchHandler(args []string) error {
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	if len(args) == 0 {
		fmt.Println("Usage: /agent switch <agent-name>")
		fmt.Println("\nAvailable agents:")
		fmt.Print(globalAgentManager.FormatAgentList())
		return nil
	}
	agentName := args[0]
	if err := globalAgentManager.SetCurrentAgent(agentName); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	agent, _ := globalAgentManager.GetAgent(agentName)
	if agent != nil {
		fmt.Printf("✓ Switched to agent: %s\n%s\n", agent.AgentType, agent.WhenToUse)
	} else {
		fmt.Printf("✓ Switched to agent: %s\n", agentName)
	}
	return nil
}

func agentCreateHandler(args []string) error {
	if len(args) < 3 {
		fmt.Println("Usage: /agent create <name> <description> <system-prompt>")
		fmt.Println("Example: /agent create myagent \"My custom agent\" \"You are a helpful assistant...\"")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	agent := &AgentInfo{
		AgentType:    args[0],
		WhenToUse:    args[1],
		SystemPrompt: strings.Join(args[2:], " "),
	}
	if err := globalAgentManager.CreateCustomAgent(agent); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Created custom agent: %s\n", args[0])
	return nil
}

func agentDeleteHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /agent delete <agent-name>")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	if err := globalAgentManager.DeleteCustomAgent(args[0]); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Deleted agent: %s\n", args[0])
	return nil
}

func agentInfoHandler(args []string) error {
	if len(args) == 0 {
		if globalAgentManager != nil {
			agent := globalAgentManager.GetCurrentAgent()
			if agent != nil {
				fmt.Print(globalAgentManager.FormatAgentInfo(agent))
				return nil
			}
		}
		fmt.Println("Usage: /agent info <agent-name>")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	agent, err := globalAgentManager.GetAgent(args[0])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Print(globalAgentManager.FormatAgentInfo(agent))
	return nil
}

func agentExportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /agent export <agent-name> [json|md]")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	agentName := args[0]
	format := "md"
	if len(args) > 1 {
		format = args[1]
	}
	content, err := globalAgentManager.ExportAgent(agentName, format)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("Exported agent:\n%s\n", content)
	return nil
}

func agentImportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /agent import <file-path>")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return nil
	}
	format := "md"
	if strings.HasSuffix(filePath, ".json") {
		format = "json"
	}
	if err := globalAgentManager.ImportAgent(string(data), format); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Imported agent from: %s\n", filePath)
	return nil
}

func subagentHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /subagent <type> <prompt>")
		fmt.Println("\nAvailable types:")
		fmt.Println("  explore - Explore codebase")
		fmt.Println("  deep    - Deep research")
		fmt.Println("  verify  - Verify implementation")
		return nil
	}
	fmt.Printf("Spawning subagent: %s\n", args[0])
	return nil
}

func agentsHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Available Agents            │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	agents := []struct {
		name string
		desc string
	}{
		{"explore", "Explores codebase to find patterns"},
		{"verification", "Verifies implementations"},
		{"deep-research", "Deep research agent"},
	}

	for _, a := range agents {
		fmt.Printf("  %-15s %s\n", a.name, a.desc)
	}

	fmt.Println("\n  Use /agent spawn <type> <prompt> to launch")
	return nil
}
