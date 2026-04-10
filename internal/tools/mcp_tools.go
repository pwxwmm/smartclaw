package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/mcp"
)

type MCPClientRegistry struct {
	clients map[string]*mcp.McpClient
	mu      sync.RWMutex
}

var defaultMCPRegistry = &MCPClientRegistry{
	clients: make(map[string]*mcp.McpClient),
}

func GetMCPRegistry() *MCPClientRegistry {
	return defaultMCPRegistry
}

func (r *MCPClientRegistry) Connect(ctx context.Context, name string, config *mcp.McpServerConfig) (*mcp.McpClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.clients[name]; ok && existing.IsReady() {
		return existing, nil
	}

	client, err := mcp.NewClientFromConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
	}

	r.clients[name] = client
	return client, nil
}

func (r *MCPClientRegistry) Get(name string) (*mcp.McpClient, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.clients[name]
	return client, ok
}

func (r *MCPClientRegistry) Disconnect(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, ok := r.clients[name]
	if !ok {
		return fmt.Errorf("server not connected: %s", name)
	}

	delete(r.clients, name)
	return client.Disconnect()
}

func (r *MCPClientRegistry) DisconnectAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []string
	for name, client := range r.clients {
		if err := client.Disconnect(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
		delete(r.clients, name)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors disconnecting: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (r *MCPClientRegistry) ListConnected() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.clients))
	for name, client := range r.clients {
		if client.IsReady() {
			names = append(names, name)
		}
	}
	return names
}

type McpExecuteTool struct{}

func (t *McpExecuteTool) Name() string { return "mcp" }
func (t *McpExecuteTool) Description() string {
	return "Execute a tool on an MCP server. Connects to the server if not already connected."
}

func (t *McpExecuteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server":    map[string]interface{}{"type": "string", "description": "MCP server name"},
			"tool":      map[string]interface{}{"type": "string", "description": "Tool name on the server"},
			"arguments": map[string]interface{}{"type": "object", "description": "Tool arguments"},
		},
		"required": []string{"server", "tool"},
	}
}

func (t *McpExecuteTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	tool, _ := input["tool"].(string)
	if server == "" || tool == "" {
		return nil, ErrRequiredField("server and tool")
	}

	args, _ := input["arguments"].(map[string]interface{})
	if args == nil {
		args = make(map[string]interface{})
	}

	registry := GetMCPRegistry()
	client, ok := registry.Get(server)
	if !ok || !client.IsReady() {
		return nil, fmt.Errorf("MCP server '%s' not connected. Use /mcp connect first", server)
	}

	result, err := client.InvokeTool(ctx, tool, args)
	if err != nil {
		return nil, fmt.Errorf("MCP tool execution failed: %w", err)
	}

	return map[string]interface{}{
		"server": server,
		"tool":   tool,
		"result": result,
	}, nil
}

type ListMcpResourcesTool struct{}

func (t *ListMcpResourcesTool) Name() string { return "list_mcp_resources" }
func (t *ListMcpResourcesTool) Description() string {
	return "List resources available on an MCP server"
}

func (t *ListMcpResourcesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server": map[string]interface{}{"type": "string", "description": "MCP server name"},
		},
		"required": []string{"server"},
	}
}

func (t *ListMcpResourcesTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	if server == "" {
		return nil, ErrRequiredField("server")
	}

	registry := GetMCPRegistry()
	client, ok := registry.Get(server)
	if !ok || !client.IsReady() {
		return nil, fmt.Errorf("MCP server '%s' not connected", server)
	}

	resources, err := client.ListResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(resources))
	for _, r := range resources {
		result = append(result, map[string]interface{}{
			"uri":         r.URI,
			"name":        r.Name,
			"description": r.Description,
			"mimeType":    r.MimeType,
		})
	}

	return map[string]interface{}{
		"server":    server,
		"resources": result,
		"count":     len(result),
	}, nil
}

type ReadMcpResourceTool struct{}

func (t *ReadMcpResourceTool) Name() string        { return "read_mcp_resource" }
func (t *ReadMcpResourceTool) Description() string { return "Read a resource from an MCP server" }

func (t *ReadMcpResourceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server": map[string]interface{}{"type": "string", "description": "MCP server name"},
			"uri":    map[string]interface{}{"type": "string", "description": "Resource URI"},
		},
		"required": []string{"server", "uri"},
	}
}

func (t *ReadMcpResourceTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	uri, _ := input["uri"].(string)
	if server == "" {
		return nil, ErrRequiredField("server")
	}
	if uri == "" {
		return nil, ErrRequiredField("uri")
	}

	registry := GetMCPRegistry()
	client, ok := registry.Get(server)
	if !ok || !client.IsReady() {
		return nil, fmt.Errorf("MCP server '%s' not connected", server)
	}

	content, err := client.ReadResource(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	return map[string]interface{}{
		"server":  server,
		"uri":     uri,
		"content": content,
	}, nil
}

type McpAuthTool struct{}

func (t *McpAuthTool) Name() string        { return "mcp_auth" }
func (t *McpAuthTool) Description() string { return "Authenticate with an MCP server using OAuth" }

func (t *McpAuthTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server":     map[string]interface{}{"type": "string", "description": "MCP server name"},
			"auth_url":   map[string]interface{}{"type": "string", "description": "OAuth authorization URL"},
			"token":      map[string]interface{}{"type": "string", "description": "OAuth token (after user completes flow)"},
			"expires_in": map[string]interface{}{"type": "integer", "description": "Token expiry in seconds"},
		},
		"required": []string{"server"},
	}
}

func (t *McpAuthTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	if server == "" {
		return nil, ErrRequiredField("server")
	}

	authManager := mcp.NewMCPAuthManager()

	if token, _ := input["token"].(string); token != "" {
		expiresIn := 3600
		if exp, ok := input["expires_in"].(float64); ok && exp > 0 {
			expiresIn = int(exp)
		}

		if err := authManager.CompleteFlow(server, token, time.Now().Add(time.Duration(expiresIn)*time.Second)); err != nil {
			return nil, fmt.Errorf("failed to complete auth: %w", err)
		}

		return map[string]interface{}{
			"server":  server,
			"status":  "authenticated",
			"message": "OAuth authentication completed successfully",
		}, nil
	}

	authURL, _ := input["auth_url"].(string)
	if authURL == "" {
		authURL = fmt.Sprintf("https://%s.example.com/oauth/authorize", server)
	}

	flow := authManager.StartFlow(server, authURL)

	return map[string]interface{}{
		"status":  "auth_required",
		"server":  server,
		"authUrl": flow.AuthURL,
		"message": "Please complete authentication. Use mcp_auth with token parameter after completing the flow.",
	}, nil
}
