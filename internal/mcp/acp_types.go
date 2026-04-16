package mcp

import (
	"encoding/json"
	"fmt"
)

// ACP types: These types are used by the ACP server (internal/acp/server.go)
// and the MCP server (internal/mcp/server.go) for JSON-RPC communication.
// The official Go MCP SDK has its own versions for client-side use.
// These are kept for backward compatibility with the ACP protocol.

const jsonRPCVersion = "2.0"

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
	Capabilities    ClientCapabilities `json:"capabilities"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ClientCapabilities struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type SamplingCapability struct{}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ListToolsResult struct {
	Tools []McpTool `json:"tools"`
}

type ListResourcesResult struct {
	Resources []McpResource `json:"resources"`
}

type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

type ReadResourceParams struct {
	URI string `json:"uri"`
}

type ReadResourceResult struct {
	Contents []ResourceContents `json:"contents"`
}

type ResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

func NewJSONRPCRequest(id any, method string, params any) *JSONRPCRequest {
	return &JSONRPCRequest{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

func ParseJSONRPCResponse(data []byte) (*JSONRPCResponse, error) {
	var resp JSONRPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}
	return &resp, nil
}

func (r *JSONRPCRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *JSONRPCRequest) MustMarshal() ([]byte, error) {
	return json.Marshal(r)
}
