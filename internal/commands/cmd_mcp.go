package commands

import (
	"context"
	"fmt"

	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/tools"
)

func init() {
	Register(Command{
		Name:    "mcp",
		Summary: "Manage MCP servers",
	}, mcpHandler)

	Register(Command{
		Name:    "mcp-add",
		Summary: "Add MCP server",
	}, mcpAddHandler)

	Register(Command{
		Name:    "mcp-remove",
		Summary: "Remove MCP server",
	}, mcpRemoveHandler)

	Register(Command{
		Name:    "mcp-list",
		Summary: "List MCP servers",
	}, mcpListHandler)

	Register(Command{
		Name:    "mcp-start",
		Summary: "Start MCP server",
	}, mcpStartHandler)

	Register(Command{
		Name:    "mcp-stop",
		Summary: "Stop MCP server",
	}, mcpStopHandler)
}

func mcpHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         MCP Servers                 │")
	fmt.Println("└─────────────────────────────────────┘")

	registry := mcp.NewMCPServerRegistry()
	servers := registry.ListServers()
	clientRegistry := tools.GetMCPRegistry()

	if len(servers) == 0 {
		fmt.Println("  No MCP servers configured")
		fmt.Println("\n  Edit ~/.smartclaw/mcp/servers.json to add servers")
		fmt.Println("  Then use /mcp-start <name> to connect")
		return nil
	}

	for _, s := range servers {
		status := "stopped"
		toolCount := 0
		if client, ok := clientRegistry.Get(s.Name); ok && client.IsReady() {
			status = "connected"
			if mcpTools, err := client.ListTools(context.Background()); err == nil {
				toolCount = len(mcpTools)
			}
		}

		fmt.Printf("\n  %s:\n", s.Name)
		fmt.Printf("    Type: %s\n", s.Type)
		if s.Command != "" {
			fmt.Printf("    Command: %s\n", s.Command)
		}
		if s.URL != "" {
			fmt.Printf("    URL: %s\n", s.URL)
		}
		fmt.Printf("    Status: %s\n", status)
		if toolCount > 0 {
			fmt.Printf("    Tools: %d\n", toolCount)
		}
	}

	fmt.Println()
	return nil
}

func mcpAddHandler(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: /mcp-add <name> <command> [args...]")
		fmt.Println("\nExample: /mcp-add filesystem npx -y @modelcontextprotocol/server-filesystem /path")
		return nil
	}

	name := args[0]
	commandParts := args[1:]

	registry := mcp.NewMCPServerRegistry()
	config := &mcp.ServerConfig{
		Name:    name,
		Type:    "local",
		Command: commandParts[0],
	}

	if len(commandParts) > 1 {
		config.Args = commandParts[1:]
	}

	if err := registry.AddServer(config); err != nil {
		fmt.Printf("✗ Failed to add server: %v\n", err)
		return nil
	}

	fmt.Printf("✓ MCP server added: %s\n", name)
	fmt.Println("  Use /mcp-start " + name + " to connect")
	return nil
}

func mcpRemoveHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /mcp-remove <name>")
		return nil
	}

	name := args[0]

	registry := mcp.NewMCPServerRegistry()
	if err := registry.RemoveServer(name); err != nil {
		fmt.Printf("✗ %v\n", err)
		return nil
	}

	tools.GetMCPRegistry().Disconnect(name)
	fmt.Printf("✓ MCP server removed: %s\n", name)
	return nil
}

func mcpListHandler(args []string) error {
	return mcpHandler(args)
}

func mcpStartHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /mcp-start <server-name>")
		fmt.Println("\nAvailable servers:")
		registry := mcp.NewMCPServerRegistry()
		for _, s := range registry.ListServers() {
			status := "stopped"
			if client, ok := tools.GetMCPRegistry().Get(s.Name); ok && client.IsReady() {
				status = "running"
			}
			fmt.Printf("  %s (%s) - %s\n", s.Name, s.Type, status)
		}
		return nil
	}

	name := args[0]
	registry := mcp.NewMCPServerRegistry()
	serverConfig, ok := registry.GetServer(name)
	if !ok {
		fmt.Printf("✗ MCP server '%s' not found in config\n", name)
		fmt.Println("  Use /mcp-add to add a server, or edit ~/.smartclaw/mcp/servers.json")
		return nil
	}

	if _, ok := tools.GetMCPRegistry().Get(name); ok {
		fmt.Printf("✓ MCP server '%s' is already connected\n", name)
		return nil
	}

	mcpConfig := &mcp.McpServerConfig{
		Name:      serverConfig.Name,
		Transport: "stdio",
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Env:       serverConfig.Env,
	}

	if serverConfig.Type == "sse" || serverConfig.Type == "http" {
		mcpConfig.Transport = "sse"
		mcpConfig.URL = serverConfig.URL
	}

	fmt.Printf("Connecting to MCP server '%s'...\n", name)

	client, err := tools.GetMCPRegistry().Connect(context.Background(), name, mcpConfig)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		return nil
	}

	mcpTools, _ := client.ListTools(context.Background())
	mcpResources, _ := client.ListResources(context.Background())
	fmt.Printf("✓ Connected to '%s' (%d tools, %d resources)\n", name, len(mcpTools), len(mcpResources))
	return nil
}

func mcpStopHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /mcp-stop <server-name>")
		return nil
	}

	name := args[0]
	registry := tools.GetMCPRegistry()
	if _, ok := registry.Get(name); !ok {
		fmt.Printf("✗ MCP server '%s' is not connected\n", name)
		return nil
	}

	if err := registry.Disconnect(name); err != nil {
		fmt.Printf("✗ Failed to disconnect: %v\n", err)
		return nil
	}

	fmt.Printf("✓ Disconnected from MCP server '%s'\n", name)
	return nil
}
