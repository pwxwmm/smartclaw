package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type McpClient struct {
	mu      sync.Mutex
	conn    *McpConnection
	client  *sdk.Client
	session *sdk.ClientSession
	ready   bool
}

func NewClient() *McpClient {
	return &McpClient{}
}

func NewClientFromConfig(ctx context.Context, config *McpServerConfig) (*McpClient, error) {
	client := sdk.NewClient(&sdk.Implementation{Name: "smartclaw", Version: "1.0.0"}, nil)

	var transport sdk.Transport

	switch config.Transport {
	case "sse", "http":
		transport = &sdk.StreamableClientTransport{
			Endpoint: config.URL,
		}
	case "stdio":
		fallthrough
	default:
		cmd := exec.Command(config.Command, config.Args...)
		if len(config.Env) > 0 {
			env := make([]string, 0, len(config.Env))
			for k, v := range config.Env {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
			cmd.Env = env
		}
		transport = &sdk.CommandTransport{Command: cmd}
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	mcpClient := &McpClient{
		client:  client,
		session: session,
		conn: &McpConnection{
			Config: config,
		},
		ready: true,
	}

	tools, err := mcpClient.ListTools(ctx)
	if err == nil {
		mcpClient.conn.Tools = tools
	}

	resources, err := mcpClient.ListResources(ctx)
	if err == nil {
		mcpClient.conn.Resources = resources
	}

	return mcpClient, nil
}

func (c *McpClient) ListTools(ctx context.Context) ([]McpTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready || c.session == nil {
		return nil, fmt.Errorf("not connected")
	}

	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}

	return convertSDKTools(result.Tools), nil
}

func (c *McpClient) InvokeTool(ctx context.Context, name string, args map[string]any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready || c.session == nil {
		return nil, fmt.Errorf("not connected")
	}

	result, err := c.session.CallTool(ctx, &sdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *McpClient) ListResources(ctx context.Context) ([]McpResource, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready || c.session == nil {
		return nil, fmt.Errorf("not connected")
	}

	result, err := c.session.ListResources(ctx, nil)
	if err != nil {
		return nil, err
	}

	return convertSDKResources(result.Resources), nil
}

func (c *McpClient) ReadResource(ctx context.Context, uri string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready || c.session == nil {
		return "", fmt.Errorf("not connected")
	}

	result, err := c.session.ReadResource(ctx, &sdk.ReadResourceParams{URI: uri})
	if err != nil {
		return "", err
	}

	if len(result.Contents) == 0 {
		return "", fmt.Errorf("no content")
	}

	return result.Contents[0].Text, nil
}

func (c *McpClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ready || c.session == nil {
		return nil
	}

	c.ready = false
	err := c.session.Close()
	if err != nil {
		return err
	}

	return nil
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
