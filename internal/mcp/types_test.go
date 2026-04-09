package mcp

import (
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.connections == nil {
		t.Error("Expected connections map to be initialized")
	}
}

func TestRegistryAdd(t *testing.T) {
	registry := NewRegistry()

	conn := &McpConnection{
		Config: &McpServerConfig{
			Name:    "test-server",
			Command: "test",
		},
	}

	registry.Add("test-server", conn)

	if len(registry.connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(registry.connections))
	}
}

func TestRegistryGet(t *testing.T) {
	registry := NewRegistry()

	conn := &McpConnection{
		Config: &McpServerConfig{
			Name:    "test-server",
			Command: "test",
		},
	}

	registry.Add("test-server", conn)

	retrieved := registry.Get("test-server")
	if retrieved == nil {
		t.Error("Expected to retrieve connection")
	}

	if retrieved.Config.Name != "test-server" {
		t.Errorf("Expected name 'test-server', got '%s'", retrieved.Config.Name)
	}
}

func TestRegistryGetNonexistent(t *testing.T) {
	registry := NewRegistry()

	retrieved := registry.Get("nonexistent")
	if retrieved != nil {
		t.Error("Expected nil for nonexistent connection")
	}
}

func TestRegistryRemove(t *testing.T) {
	registry := NewRegistry()

	conn := &McpConnection{
		Config: &McpServerConfig{
			Name:    "test-server",
			Command: "test",
		},
	}

	registry.Add("test-server", conn)
	registry.Remove("test-server")

	if len(registry.connections) != 0 {
		t.Errorf("Expected 0 connections after remove, got %d", len(registry.connections))
	}
}

func TestRegistryList(t *testing.T) {
	registry := NewRegistry()

	conn1 := &McpConnection{
		Config: &McpServerConfig{Name: "server1"},
	}
	conn2 := &McpConnection{
		Config: &McpServerConfig{Name: "server2"},
	}

	registry.Add("server1", conn1)
	registry.Add("server2", conn2)

	list := registry.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(list))
	}
}

func TestMcpServerConfig(t *testing.T) {
	config := McpServerConfig{
		Name:      "test-server",
		Transport: "stdio",
		Command:   "/usr/bin/node",
		Args:      []string{"server.js"},
		Env: map[string]string{
			"NODE_ENV": "test",
		},
		Timeout: 30,
	}

	if config.Name != "test-server" {
		t.Errorf("Expected name 'test-server', got '%s'", config.Name)
	}

	if config.Transport != "stdio" {
		t.Errorf("Expected transport 'stdio', got '%s'", config.Transport)
	}

	if len(config.Args) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(config.Args))
	}
}

func TestMcpTool(t *testing.T) {
	tool := McpTool{
		Name:        "read_file",
		Description: "Read a file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
			},
		},
	}

	if tool.Name != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", tool.Name)
	}

	if tool.Description != "Read a file" {
		t.Errorf("Expected description 'Read a file', got '%s'", tool.Description)
	}
}

func TestMcpResource(t *testing.T) {
	resource := McpResource{
		URI:         "file:///test.txt",
		Name:        "test.txt",
		Description: "A test file",
		MimeType:    "text/plain",
	}

	if resource.URI != "file:///test.txt" {
		t.Errorf("Expected URI 'file:///test.txt', got '%s'", resource.URI)
	}

	if resource.MimeType != "text/plain" {
		t.Errorf("Expected mime type 'text/plain', got '%s'", resource.MimeType)
	}
}

func TestMcpPrompt(t *testing.T) {
	prompt := McpPrompt{
		Name:        "greeting",
		Description: "A greeting prompt",
		Arguments: []PromptArgument{
			{Name: "name", Description: "The name", Required: true},
		},
	}

	if prompt.Name != "greeting" {
		t.Errorf("Expected name 'greeting', got '%s'", prompt.Name)
	}

	if len(prompt.Arguments) != 1 {
		t.Errorf("Expected 1 argument, got %d", len(prompt.Arguments))
	}
}

func TestPromptArgument(t *testing.T) {
	arg := PromptArgument{
		Name:        "path",
		Description: "File path",
		Required:    true,
	}

	if arg.Name != "path" {
		t.Errorf("Expected name 'path', got '%s'", arg.Name)
	}

	if !arg.Required {
		t.Error("Expected required to be true")
	}
}

func TestToolAnnotations(t *testing.T) {
	annotations := ToolAnnotations{
		ReadOnly:    true,
		Idempotent:  true,
		SideEffects: false,
	}

	if !annotations.ReadOnly {
		t.Error("Expected ReadOnly to be true")
	}

	if !annotations.Idempotent {
		t.Error("Expected Idempotent to be true")
	}

	if annotations.SideEffects {
		t.Error("Expected SideEffects to be false")
	}
}

func TestMcpConnection(t *testing.T) {
	conn := &McpConnection{
		Config: &McpServerConfig{
			Name:    "test",
			Command: "test",
		},
		Tools: []McpTool{
			{Name: "tool1"},
			{Name: "tool2"},
		},
		Resources: []McpResource{
			{URI: "resource1"},
		},
		Prompts: []McpPrompt{
			{Name: "prompt1"},
		},
		ready: false,
	}

	if conn.Config.Name != "test" {
		t.Errorf("Expected config name 'test', got '%s'", conn.Config.Name)
	}

	if len(conn.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(conn.Tools))
	}

	if len(conn.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(conn.Resources))
	}

	if len(conn.Prompts) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(conn.Prompts))
	}
}

func TestMcpOAuthConfig(t *testing.T) {
	oauth := McpOAuthConfig{
		ClientID:     "client123",
		ClientSecret: "secret123",
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		Scopes:       []string{"read", "write"},
	}

	if oauth.ClientID != "client123" {
		t.Errorf("Expected client ID 'client123', got '%s'", oauth.ClientID)
	}

	if len(oauth.Scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(oauth.Scopes))
	}
}
