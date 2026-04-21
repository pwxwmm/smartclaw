package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/tools"
)

const (
	protocolVersion = "2025-03-26"
	serverName      = "smartclaw"
	serverVersion   = "1.0.0"
	serverAuthor    = "weimengmeng 天气晴"
	serverEmail     = "1300042631@qq.com"
)

// ACPServer implements the Agent Communication Protocol over stdio JSON-RPC.
// IDEs (VS Code, Zed, JetBrains) launch `smartclaw acp` and communicate
// via stdin/stdout using the same Content-Length framing as MCP.
type ACPServer struct {
	registry    *tools.ToolRegistry
	eventBus    *EventBus
	permissions *PermissionModel
	services    *ServiceRegistry
}

// NewACPServer creates an ACP server backed by the given tool registry.
func NewACPServer(registry *tools.ToolRegistry) *ACPServer {
	pm := NewPermissionModel()
	for _, r := range DefaultPermissions() {
		pm.AddRule(r)
	}

	return &ACPServer{
		registry:    registry,
		eventBus:    NewEventBus(),
		permissions: pm,
		services:    NewServiceRegistry(),
	}
}

func (s *ACPServer) GetEventBus() *EventBus       { return s.eventBus }
func (s *ACPServer) GetPermissions() *PermissionModel { return s.permissions }
func (s *ACPServer) GetServices() *ServiceRegistry    { return s.services }

// Serve starts the ACP server loop, reading JSON-RPC from reader and
// writing responses to writer. Blocks until ctx is cancelled or the
// client sends a shutdown request.
func (s *ACPServer) Serve(ctx context.Context, reader io.Reader, writer io.Writer) error {
	slog.Info("acp: server starting")

	bufReader := newBufferedReader(reader)

	for {
		select {
		case <-ctx.Done():
			slog.Info("acp: server stopped by context")
			return nil
		default:
		}

		req, err := readRequest(bufReader)
		if err != nil {
			if err == io.EOF {
				slog.Info("acp: client disconnected")
				return nil
			}
			slog.Warn("acp: failed to read request", "error", err)
			continue
		}

		resp := s.handleRequest(ctx, req)

		if err := writeResponse(writer, resp); err != nil {
			slog.Warn("acp: failed to write response", "error", err)
			return fmt.Errorf("acp: write: %w", err)
		}

		if req.Method == "shutdown" {
			slog.Info("acp: shutdown requested")
			return nil
		}
	}
}

func (s *ACPServer) handleRequest(ctx context.Context, req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return &mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "events/subscribe":
		return s.handleEventsSubscribe(req)
	case "events/publish":
		return s.handleEventsPublish(ctx, req)
	case "permissions/check":
		return s.handlePermissionsCheck(req)
	case "services/register":
		return s.handleServicesRegister(req)
	case "services/list":
		return s.handleServicesList(req)
	case "services/find":
		return s.handleServicesFind(req)
	case "shutdown":
		return &mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	default:
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *ACPServer) handleInitialize(req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	result := mcp.InitializeResult{
		ProtocolVersion: protocolVersion,
		ServerInfo: mcp.ServerInfo{
			Name:    serverName,
			Version: serverVersion,
		},
		Capabilities: mcp.ServerCapabilities{
			Tools: &mcp.ToolsCapability{ListChanged: false},
		},
	}
	return &mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *ACPServer) handleToolsList(req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	toolList := s.registry.All()
	mcpTools := make([]mcp.McpTool, 0, len(toolList))
	for _, t := range toolList {
		mcpTools = append(mcpTools, mcp.McpTool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcp.ListToolsResult{Tools: mcpTools},
	}
}

func (s *ACPServer) handleToolsCall(ctx context.Context, req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	var params mcp.CallToolParams
	if err := parseParams(req.Params, &params); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	s.eventBus.PublishAsync(ctx, Event{
		ID:        fmt.Sprintf("tc-%d", time.Now().UnixNano()),
		Type:      EventToolCall,
		Source:    "agent",
		Timestamp: time.Now(),
		Data:      map[string]any{"tool": params.Name, "arguments": params.Arguments},
	})

	result, err := s.registry.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		s.eventBus.PublishAsync(ctx, Event{
			ID:        fmt.Sprintf("tr-%d", time.Now().UnixNano()),
			Type:      EventError,
			Source:    "tool",
			Timestamp: time.Now(),
			Data:      map[string]any{"tool": params.Name, "error": err.Error()},
		})
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcp.CallToolResult{
				Content: []mcp.ContentBlock{
					{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			},
		}
	}

	text := formatResult(result)

	s.eventBus.PublishAsync(ctx, Event{
		ID:        fmt.Sprintf("tr-%d", time.Now().UnixNano()),
		Type:      EventToolResult,
		Source:    "tool",
		Timestamp: time.Now(),
		Data:      map[string]any{"tool": params.Name},
	})

	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcp.CallToolResult{
			Content: []mcp.ContentBlock{
				{Type: "text", Text: text},
			},
		},
	}
}

func parseParams(raw any, target any) error {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func formatResult(result any) string {
	switch v := result.(type) {
	case string:
		return v
	default:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

// --- stdio framing (Content-Length header, same as MCP) ---

type bufferedReader struct {
	reader *json.Decoder
}

func newBufferedReader(r io.Reader) *bufferedReader {
	return &bufferedReader{reader: json.NewDecoder(r)}
}

func readRequest(br *bufferedReader) (*mcp.JSONRPCRequest, error) {
	var req mcp.JSONRPCRequest
	if err := br.reader.Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func writeResponse(w io.Writer, resp *mcp.JSONRPCResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := w.Write([]byte(header)); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	// Flush if writer supports it
	if flusher, ok := w.(interface{ Flush() }); ok {
		flusher.Flush()
	}

	return nil
}
