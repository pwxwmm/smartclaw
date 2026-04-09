package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestClientInitialize(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
}

func TestClientWithTransport(t *testing.T) {
	transport := NewSSETransport("http://localhost:8080")
	client := NewClientWithTransport(transport)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.transport == nil {
		t.Error("Expected transport to be set")
	}
}

func TestClientFromConfig(t *testing.T) {
	config := &McpServerConfig{
		Name:      "test-server",
		Transport: "sse",
		URL:       "http://localhost:8080",
	}

	client, err := NewClientFromConfig(context.Background(), config)
	if err == nil {
		t.Error("Expected error for unreachable server")
	}
	if client != nil {
		t.Error("Expected nil client for unreachable server")
	}
}

func TestJSONRPCRequest(t *testing.T) {
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
		},
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", req.JSONRPC)
	}

	if req.Method != "initialize" {
		t.Errorf("Expected method 'initialize', got '%s'", req.Method)
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Errorf("Failed to marshal request: %v", err)
	}

	if string(data) == "" {
		t.Error("Expected non-empty JSON")
	}
}

func TestJSONRPCResponse(t *testing.T) {
	respData := `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{}}}`

	resp, err := ParseJSONRPCResponse([]byte(respData))
	if err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}

	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %v", resp.ID)
	}
}

func TestJSONRPCResponseError(t *testing.T) {
	respData := `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`

	resp, err := ParseJSONRPCResponse([]byte(respData))
	if err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}

	if resp.Error.Code != -32600 {
		t.Errorf("Expected error code -32600, got %d", resp.Error.Code)
	}
}

func TestInitializeParams(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
		Capabilities: ClientCapabilities{
			Sampling: &SamplingCapability{},
		},
	}

	if params.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version '2024-11-05', got '%s'", params.ProtocolVersion)
	}

	if params.ClientInfo.Name != "test-client" {
		t.Errorf("Expected client name 'test-client', got '%s'", params.ClientInfo.Name)
	}
}

func TestInitializeResult(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
			Prompts:   &PromptsCapability{},
		},
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version '2024-11-05', got '%s'", result.ProtocolVersion)
	}

	if result.ServerInfo.Name != "test-server" {
		t.Errorf("Expected server name 'test-server', got '%s'", result.ServerInfo.Name)
	}
}

func TestListToolsResult(t *testing.T) {
	result := ListToolsResult{
		Tools: []McpTool{
			{Name: "read_file", Description: "Read a file"},
			{Name: "write_file", Description: "Write a file"},
		},
	}

	if len(result.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(result.Tools))
	}

	if result.Tools[0].Name != "read_file" {
		t.Errorf("Expected tool name 'read_file', got '%s'", result.Tools[0].Name)
	}
}

func TestCallToolParams(t *testing.T) {
	params := CallToolParams{
		Name: "read_file",
		Arguments: map[string]interface{}{
			"path": "/test/file.txt",
		},
	}

	if params.Name != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", params.Name)
	}

	if params.Arguments["path"] != "/test/file.txt" {
		t.Errorf("Expected path '/test/file.txt', got '%v'", params.Arguments["path"])
	}
}

func TestCallToolResult(t *testing.T) {
	result := CallToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "File content"},
		},
		IsError: false,
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(result.Content))
	}

	if result.IsError {
		t.Error("Expected IsError to be false")
	}
}

func TestListResourcesResult(t *testing.T) {
	result := ListResourcesResult{
		Resources: []McpResource{
			{URI: "file:///test.txt", Name: "test.txt"},
		},
	}

	if len(result.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(result.Resources))
	}

	if result.Resources[0].URI != "file:///test.txt" {
		t.Errorf("Expected URI 'file:///test.txt', got '%s'", result.Resources[0].URI)
	}
}

func TestReadResourceParams(t *testing.T) {
	params := ReadResourceParams{
		URI: "file:///test.txt",
	}

	if params.URI != "file:///test.txt" {
		t.Errorf("Expected URI 'file:///test.txt', got '%s'", params.URI)
	}
}

func TestReadResourceResult(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContents{
			{
				URI:      "file:///test.txt",
				MimeType: "text/plain",
				Text:     "Hello World",
			},
		},
	}

	if len(result.Contents) != 1 {
		t.Errorf("Expected 1 content, got %d", len(result.Contents))
	}

	if result.Contents[0].Text != "Hello World" {
		t.Errorf("Expected text 'Hello World', got '%s'", result.Contents[0].Text)
	}
}

func TestSSETransportStart(t *testing.T) {
	transport := NewSSETransport("http://localhost:8080")

	if err := transport.Start(); err != nil {
		t.Errorf("Start failed: %v", err)
	}

	if !transport.IsRunning() {
		t.Error("Expected transport to be running")
	}

	if err := transport.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	if transport.IsRunning() {
		t.Error("Expected transport to be stopped")
	}
}

func TestSSETransportSendNotReady(t *testing.T) {
	transport := NewSSETransport("http://localhost:8080")

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}

	err := transport.Send(req)
	if err == nil {
		t.Error("Expected error for transport not ready")
	}
}

func TestSSETransportReceiveNotSupported(t *testing.T) {
	transport := NewSSETransport("http://localhost:8080")
	transport.Start()

	_, err := transport.Receive()
	if err == nil {
		t.Error("Expected error for SSE transport Receive")
	}
}

func TestStdioTransportStart(t *testing.T) {
	config := &McpServerConfig{
		Name:    "test",
		Command: "echo",
		Args:    []string{"test"},
	}

	transport := NewStdioTransport(config)

	if transport == nil {
		t.Fatal("Expected non-nil transport")
	}
}

func TestStdioTransportSendNotReady(t *testing.T) {
	config := &McpServerConfig{
		Name:    "test",
		Command: "echo",
	}

	transport := NewStdioTransport(config)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}

	err := transport.Send(req)
	if err == nil {
		t.Error("Expected error for transport not ready")
	}
}

func TestStdioTransportReceiveNotReady(t *testing.T) {
	config := &McpServerConfig{
		Name:    "test",
		Command: "echo",
	}

	transport := NewStdioTransport(config)

	_, err := transport.Receive()
	if err == nil {
		t.Error("Expected error for transport not ready")
	}
}

func TestClientDisconnect(t *testing.T) {
	transport := NewSSETransport("http://localhost:8080")
	transport.Start()

	client := NewClientWithTransport(transport)

	err := client.Disconnect()
	if err != nil {
		t.Errorf("Disconnect failed: %v", err)
	}
}

func TestClientIsReady(t *testing.T) {
	transport := NewSSETransport("http://localhost:8080")

	client := NewClientWithTransport(transport)

	if client.IsReady() {
		t.Error("Expected client to not be ready before start")
	}

	transport.Start()

	if !client.IsReady() {
		t.Error("Expected client to be ready after start")
	}
}

func TestRPCError(t *testing.T) {
	err := &RPCError{
		Code:    -32600,
		Message: "Invalid Request",
	}

	if err.Code != -32600 {
		t.Errorf("Expected code -32600, got %d", err.Code)
	}

	if err.Error() == "" {
		t.Error("Expected error message")
	}
}

func TestNewJSONRPCRequest(t *testing.T) {
	req := NewJSONRPCRequest(1, "test", map[string]string{"key": "value"})

	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", req.JSONRPC)
	}

	if req.Method != "test" {
		t.Errorf("Expected method 'test', got '%s'", req.Method)
	}
}

func TestJSONRPCRequestMustMarshal(t *testing.T) {
	req := NewJSONRPCRequest(1, "initialize", nil)

	data := req.MustMarshal()

	if len(data) == 0 {
		t.Error("Expected non-empty data")
	}
}

func TestClientCapabilities(t *testing.T) {
	caps := ClientCapabilities{
		Roots:    &RootsCapability{ListChanged: true},
		Sampling: &SamplingCapability{},
	}

	if caps.Roots == nil {
		t.Error("Expected Roots capability")
	}

	if caps.Sampling == nil {
		t.Error("Expected Sampling capability")
	}
}

func TestServerCapabilities(t *testing.T) {
	caps := ServerCapabilities{
		Tools:     &ToolsCapability{ListChanged: true},
		Resources: &ResourcesCapability{Subscribe: true, ListChanged: true},
		Prompts:   &PromptsCapability{ListChanged: true},
	}

	if caps.Tools == nil {
		t.Error("Expected Tools capability")
	}

	if caps.Resources == nil {
		t.Error("Expected Resources capability")
	}

	if caps.Prompts == nil {
		t.Error("Expected Prompts capability")
	}
}

func TestResourceContents(t *testing.T) {
	content := ResourceContents{
		URI:      "file:///test.txt",
		MimeType: "text/plain",
		Text:     "Hello",
	}

	if content.URI != "file:///test.txt" {
		t.Errorf("Expected URI 'file:///test.txt', got '%s'", content.URI)
	}

	if content.MimeType != "text/plain" {
		t.Errorf("Expected mime type 'text/plain', got '%s'", content.MimeType)
	}
}

func TestContentBlock(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Hello World",
	}

	if block.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", block.Type)
	}

	if block.Text != "Hello World" {
		t.Errorf("Expected text 'Hello World', got '%s'", block.Text)
	}
}
