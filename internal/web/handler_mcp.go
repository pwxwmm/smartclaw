package web

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/instructkr/smartclaw/internal/mcp"
)

func (h *Handler) handleMCPListWS(client *Client) {
	if h.mcpRegistry == nil {
		h.sendToClient(client, WSResponse{Type: "mcp_list", Data: []any{}})
		return
	}
	servers := h.mcpRegistry.ListServers()
	h.sendToClient(client, WSResponse{Type: "mcp_list", Data: servers})
}

func (h *Handler) handleMCPAddWS(client *Client, msg WSMessage) {
	var data struct {
		Name        string            `json:"name"`
		Type        string            `json:"type"`
		Command     string            `json:"command"`
		Args        []string          `json:"args"`
		URL         string            `json:"url"`
		Env         map[string]string `json:"env"`
		AutoStart   bool              `json:"auto_start"`
		Description string            `json:"description"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid MCP add request")
		return
	}
	if data.Name == "" {
		h.sendError(client, "Server name is required")
		return
	}
	if h.mcpRegistry == nil {
		h.sendError(client, "MCP registry not available")
		return
	}

	config := &mcp.ServerConfig{
		Name:        data.Name,
		Type:        data.Type,
		Command:     data.Command,
		Args:        data.Args,
		URL:         data.URL,
		Env:         data.Env,
		AutoStart:   data.AutoStart,
		Description: data.Description,
	}

	if err := h.mcpRegistry.AddServer(config); err != nil {
		h.sendToClient(client, WSResponse{Type: "mcp_add", Data: map[string]any{
			"success": false,
			"error":   err.Error(),
		}})
		return
	}

	h.sendToClient(client, WSResponse{Type: "mcp_add", Data: map[string]any{
		"success": true,
		"name":    data.Name,
	}})

	servers := h.mcpRegistry.ListServers()
	h.sendToClient(client, WSResponse{Type: "mcp_list", Data: servers})
}

func (h *Handler) handleMCPRemoveWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid MCP remove request")
		return
	}
	name, _ := data["name"].(string)
	if name == "" {
		h.sendError(client, "Server name is required")
		return
	}
	if h.mcpRegistry == nil {
		h.sendError(client, "MCP registry not available")
		return
	}

	if err := h.mcpRegistry.RemoveServer(name); err != nil {
		h.sendToClient(client, WSResponse{Type: "mcp_remove", Data: map[string]any{
			"success": false,
			"error":   err.Error(),
		}})
		return
	}

	h.sendToClient(client, WSResponse{Type: "mcp_remove", Data: map[string]any{
		"success": true,
		"name":    name,
	}})

	servers := h.mcpRegistry.ListServers()
	h.sendToClient(client, WSResponse{Type: "mcp_list", Data: servers})
}

func (h *Handler) handleMCPStartWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid MCP start request")
		return
	}
	name, _ := data["name"].(string)
	if name == "" {
		h.sendError(client, "Server name is required")
		return
	}
	if h.mcpRegistry == nil {
		h.sendError(client, "MCP registry not available")
		return
	}

	_, exists := h.mcpRegistry.GetServer(name)
	if !exists {
		h.sendError(client, "Server not found: "+name)
		return
	}

	if err := h.mcpRegistry.StartServer(context.Background(), name); err != nil {
		h.sendToClient(client, WSResponse{Type: "mcp_start", Data: map[string]any{
			"success": false,
			"name":    name,
			"error":   err.Error(),
		}})
		return
	}

	slog.Info("MCP server started", "name", name)

	h.sendToClient(client, WSResponse{Type: "mcp_start", Data: map[string]any{
		"success": true,
		"name":    name,
	}})

	servers := h.mcpRegistry.ListServers()
	h.sendToClient(client, WSResponse{Type: "mcp_list", Data: servers})
}

func (h *Handler) handleMCPStopWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid MCP stop request")
		return
	}
	name, _ := data["name"].(string)
	if name == "" {
		h.sendError(client, "Server name is required")
		return
	}
	if h.mcpRegistry == nil {
		h.sendError(client, "MCP registry not available")
		return
	}

	if err := h.mcpRegistry.StopServer(name); err != nil {
		h.sendToClient(client, WSResponse{Type: "mcp_stop", Data: map[string]any{
			"success": false,
			"name":    name,
			"error":   err.Error(),
		}})
		return
	}

	slog.Info("MCP server stopped", "name", name)

	h.sendToClient(client, WSResponse{Type: "mcp_stop", Data: map[string]any{
		"success": true,
		"name":    name,
	}})

	servers := h.mcpRegistry.ListServers()
	h.sendToClient(client, WSResponse{Type: "mcp_list", Data: servers})
}
