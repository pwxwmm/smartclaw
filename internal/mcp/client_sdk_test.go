package mcp

import (
	"context"
	"testing"
)

func TestNewClientFromConfigInvalidURL(t *testing.T) {
	config := &McpServerConfig{
		Name:      "test-server",
		Transport: "sse",
		URL:       "http://localhost:1",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()

	client, err := NewClientFromConfig(ctx, config)
	if err == nil {
		if client != nil {
			client.Disconnect()
		}
		t.Error("Expected error for unreachable server")
	}
	if client != nil {
		t.Error("Expected nil client for unreachable server")
	}
}

func TestNewClientFromConfigInvalidCommand(t *testing.T) {
	config := &McpServerConfig{
		Name:      "test-server",
		Transport: "stdio",
		Command:   "nonexistent_command_that_does_not_exist",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()

	client, err := NewClientFromConfig(ctx, config)
	if err == nil {
		if client != nil {
			client.Disconnect()
		}
		t.Error("Expected error for nonexistent command")
	}
	if client != nil {
		t.Error("Expected nil client for nonexistent command")
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.IsReady() {
		t.Error("Expected client to not be ready before connection")
	}
}

func TestClientDisconnectNotConnected(t *testing.T) {
	client := NewClient()
	err := client.Disconnect()
	if err != nil {
		t.Errorf("Disconnect on unconnected client should not error, got: %v", err)
	}
}

func TestConvertSDKTools(t *testing.T) {
	tools := convertSDKTools(nil)
	if tools == nil {
		t.Error("Expected non-nil slice for nil input")
	}
	if len(tools) != 0 {
		t.Errorf("Expected empty slice for nil input, got %d", len(tools))
	}
}

func TestConvertSDKResources(t *testing.T) {
	resources := convertSDKResources(nil)
	if resources == nil {
		t.Error("Expected non-nil slice for nil input")
	}
	if len(resources) != 0 {
		t.Errorf("Expected empty slice for nil input, got %d", len(resources))
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

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

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

	if registry.Get("nonexistent") != nil {
		t.Error("Expected nil for nonexistent connection")
	}

	registry.Remove("test-server")
	if registry.Get("test-server") != nil {
		t.Error("Expected nil after removal")
	}
}

func TestMcpToolInputSchemaAny(t *testing.T) {
	tool := McpTool{
		Name:        "read_file",
		Description: "Read a file",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
		},
	}

	if tool.Name != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", tool.Name)
	}
}
