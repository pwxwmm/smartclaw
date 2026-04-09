package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

type McpServer struct {
	name         string
	version      string
	tools        map[string]McpTool
	resources    map[string]McpResource
	prompts      map[string]McpPrompt
	capabilities ServerCapabilities
	mu           sync.RWMutex
	toolHandler  func(name string, arguments map[string]interface{}) (interface{}, error)
	stdin        io.Reader
	stdout       io.Writer
}

func NewMcpServer(name, version string) *McpServer {
	return &McpServer{
		name:      name,
		version:   version,
		tools:     make(map[string]McpTool),
		resources: make(map[string]McpResource),
		prompts:   make(map[string]McpPrompt),
		capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{ListChanged: true},
			Resources: &ResourcesCapability{Subscribe: true, ListChanged: true},
			Prompts:   &PromptsCapability{ListChanged: true},
		},
		stdin:  os.Stdin,
		stdout: os.Stdout,
	}
}

func (s *McpServer) SetIO(stdin io.Reader, stdout io.Writer) {
	s.stdin = stdin
	s.stdout = stdout
}

func (s *McpServer) RegisterTool(tool McpTool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
}

func (s *McpServer) RegisterResource(resource McpResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[resource.URI] = resource
}

func (s *McpServer) RegisterPrompt(prompt McpPrompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[prompt.Name] = prompt
}

func (s *McpServer) SetToolHandler(handler func(name string, arguments map[string]interface{}) (interface{}, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolHandler = handler
}

func (s *McpServer) Run(ctx context.Context) error {
	reader := bufio.NewReader(s.stdin)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var length int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				continue
			}

			if line == "\r\n" {
				break
			}

			var n int
			if _, err := fmt.Sscanf(line, "Content-Length: %d\r\n", &n); err == nil {
				length = n
			}
		}

		if length == 0 {
			continue
		}

		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			continue
		}

		var request JSONRPCRequest
		if err := json.Unmarshal(data, &request); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		go s.handleRequest(ctx, &request)
	}
}

func (s *McpServer) handleRequest(ctx context.Context, request *JSONRPCRequest) {
	var result interface{}
	var err error

	switch request.Method {
	case "initialize":
		result, err = s.handleInitialize(request)
	case "tools/list":
		result, err = s.handleToolsList()
	case "tools/call":
		result, err = s.handleToolsCall(request)
	case "resources/list":
		result, err = s.handleResourcesList()
	case "resources/read":
		result, err = s.handleResourcesRead(request)
	case "prompts/list":
		result, err = s.handlePromptsList()
	case "prompts/get":
		result, err = s.handlePromptsGet(request)
	case "notifications/initialized":
		return
	default:
		s.sendError(request.ID, -32601, "Method not found")
		return
	}

	if err != nil {
		s.sendError(request.ID, -32603, err.Error())
		return
	}

	s.sendResult(request.ID, result)
}

func (s *McpServer) handleInitialize(request *JSONRPCRequest) (*InitializeResult, error) {
	return &InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
		Capabilities: s.capabilities,
	}, nil
}

func (s *McpServer) handleToolsList() (*ListToolsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]McpTool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}

	return &ListToolsResult{Tools: tools}, nil
}

func (s *McpServer) handleToolsCall(request *JSONRPCRequest) (*CallToolResult, error) {
	s.mu.RLock()
	handler := s.toolHandler
	s.mu.RUnlock()

	if handler == nil {
		return nil, fmt.Errorf("no tool handler configured")
	}

	params, ok := request.Params.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid params")
	}

	name, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	result, err := handler(name, arguments)
	if err != nil {
		return &CallToolResult{
			IsError: true,
			Content: []ContentBlock{
				{Type: "text", Text: err.Error()},
			},
		}, nil
	}

	content := []ContentBlock{}
	switch v := result.(type) {
	case string:
		content = append(content, ContentBlock{Type: "text", Text: v})
	case []ContentBlock:
		content = v
	case map[string]interface{}:
		data, _ := json.Marshal(v)
		content = append(content, ContentBlock{Type: "text", Text: string(data)})
	default:
		data, _ := json.Marshal(result)
		content = append(content, ContentBlock{Type: "text", Text: string(data)})
	}

	return &CallToolResult{
		Content: content,
		IsError: false,
	}, nil
}

func (s *McpServer) handleResourcesList() (*ListResourcesResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make([]McpResource, 0, len(s.resources))
	for _, resource := range s.resources {
		resources = append(resources, resource)
	}

	return &ListResourcesResult{Resources: resources}, nil
}

func (s *McpServer) handleResourcesRead(request *JSONRPCRequest) (*ReadResourceResult, error) {
	params, ok := request.Params.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid params")
	}

	uri, _ := params["uri"].(string)

	s.mu.RLock()
	resource, exists := s.resources[uri]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	return &ReadResourceResult{
		Contents: []ResourceContents{
			{
				URI:      resource.URI,
				MimeType: resource.MimeType,
				Text:     resource.Description,
			},
		},
	}, nil
}

func (s *McpServer) handlePromptsList() (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prompts := make([]McpPrompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		prompts = append(prompts, prompt)
	}

	return map[string]interface{}{
		"prompts": prompts,
	}, nil
}

func (s *McpServer) handlePromptsGet(request *JSONRPCRequest) (map[string]interface{}, error) {
	params, ok := request.Params.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid params")
	}

	name, _ := params["name"].(string)

	s.mu.RLock()
	prompt, exists := s.prompts[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("prompt not found: %s", name)
	}

	return map[string]interface{}{
		"description": prompt.Description,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": map[string]interface{}{
					"type": "text",
					"text": prompt.Description,
				},
			},
		},
	}, nil
}

func (s *McpServer) sendResult(id interface{}, result interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	s.sendResponse(&response)
}

func (s *McpServer) sendError(id interface{}, code int, message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}

	s.sendResponse(&response)
}

func (s *McpServer) sendResponse(response *JSONRPCResponse) {
	data, err := json.Marshal(response)
	if err != nil {
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	s.stdout.Write([]byte(header))
	s.stdout.Write(data)
}

func (s *McpServer) SendNotification(method string, params interface{}) {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(request)
	if err != nil {
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	s.stdout.Write([]byte(header))
	s.stdout.Write(data)
}

func (s *McpServer) NotifyToolsListChanged() {
	s.SendNotification("notifications/tools/list_changed", nil)
}

func (s *McpServer) NotifyResourcesListChanged() {
	s.SendNotification("notifications/resources/list_changed", nil)
}

func (s *McpServer) NotifyPromptsListChanged() {
	s.SendNotification("notifications/prompts/list_changed", nil)
}

type McpServerBuilder struct {
	server *McpServer
}

func NewMcpServerBuilder(name, version string) *McpServerBuilder {
	return &McpServerBuilder{
		server: NewMcpServer(name, version),
	}
}

func (b *McpServerBuilder) WithTool(tool McpTool) *McpServerBuilder {
	b.server.RegisterTool(tool)
	return b
}

func (b *McpServerBuilder) WithResource(resource McpResource) *McpServerBuilder {
	b.server.RegisterResource(resource)
	return b
}

func (b *McpServerBuilder) WithPrompt(prompt McpPrompt) *McpServerBuilder {
	b.server.RegisterPrompt(prompt)
	return b
}

func (b *McpServerBuilder) WithToolHandler(handler func(name string, arguments map[string]interface{}) (interface{}, error)) *McpServerBuilder {
	b.server.SetToolHandler(handler)
	return b
}

func (b *McpServerBuilder) Build() *McpServer {
	return b.server
}
