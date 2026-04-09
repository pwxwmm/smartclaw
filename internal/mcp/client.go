package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

type McpClient struct {
	mu        sync.Mutex
	conn      *McpConnection
	transport Transport
	requestID uint64
	ready     bool
}

func NewClient() *McpClient {
	return &McpClient{}
}

func NewClientWithTransport(transport Transport) *McpClient {
	return &McpClient{
		transport: transport,
	}
}

func NewClientFromConfig(ctx context.Context, config *McpServerConfig) (*McpClient, error) {
	client := NewClient()

	var transport Transport
	switch config.Transport {
	case "sse", "http":
		transport = NewSSETransport(config.URL)
	case "stdio":
		fallthrough
	default:
		transport = NewStdioTransport(config)
	}

	client.transport = transport

	if err := transport.Start(); err != nil {
		return nil, fmt.Errorf("failed to start transport: %w", err)
	}

	client.conn = &McpConnection{
		Config: config,
	}

	if err := client.initialize(ctx); err != nil {
		_ = transport.Stop()
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	tools, err := client.ListTools(ctx)
	if err == nil {
		client.conn.Tools = tools
	}

	resources, err := client.ListResources(ctx)
	if err == nil {
		client.conn.Resources = resources
	}

	client.ready = true
	return client, nil
}

func (c *McpClient) initialize(ctx context.Context) error {
	params := &InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: ClientInfo{
			Name:    "smartcode",
			Version: "1.0.0",
		},
		Capabilities: ClientCapabilities{},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return err
	}

	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	_, _ = c.sendRequest(ctx, "notifications/initialized", nil)

	return nil
}

func (c *McpClient) ListTools(ctx context.Context) ([]McpTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready && c.transport == nil {
		return nil, fmt.Errorf("not connected")
	}

	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result ListToolsResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	return result.Tools, nil
}

func (c *McpClient) InvokeTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready && c.transport == nil {
		return nil, fmt.Errorf("not connected")
	}

	params := &CallToolParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	return result, nil
}

func (c *McpClient) ListResources(ctx context.Context) ([]McpResource, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready && c.transport == nil {
		return nil, fmt.Errorf("not connected")
	}

	resp, err := c.sendRequest(ctx, "resources/list", nil)
	if err != nil {
		return nil, err
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result ListResourcesResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources list: %w", err)
	}

	return result.Resources, nil
}

func (c *McpClient) ReadResource(ctx context.Context, uri string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready && c.transport == nil {
		return "", fmt.Errorf("not connected")
	}

	params := &ReadResourceParams{
		URI: uri,
	}

	resp, err := c.sendRequest(ctx, "resources/read", params)
	if err != nil {
		return "", err
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return "", err
	}

	var result ReadResourceResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return "", fmt.Errorf("failed to parse resource result: %w", err)
	}

	if len(result.Contents) == 0 {
		return "", fmt.Errorf("no content")
	}

	return result.Contents[0].Text, nil
}

func (c *McpClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready && c.transport == nil {
		return nil
	}

	c.ready = false
	return c.transport.Stop()
}

func (c *McpClient) GetConnection() *McpConnection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

func (c *McpClient) IsReady() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ready
}

func (c *McpClient) sendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	id := atomic.AddUint64(&c.requestID, 1)

	req := NewJSONRPCRequest(id, method, params)

	if err := c.transport.Send(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	resp, err := c.transport.Receive()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp, nil
}
