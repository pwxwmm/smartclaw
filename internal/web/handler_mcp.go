package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/instructkr/smartclaw/internal/mcp"
)

type MCPCatalogEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Type        string   `json:"type"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Popular     bool     `json:"popular"`
	Installed   bool     `json:"installed"`
}

var mcpCatalog = []MCPCatalogEntry{
	{Name: "filesystem", Description: "File system operations with access controls", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}, Popular: true},
	{Name: "github", Description: "GitHub API - repos, issues, PRs, search", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"}, Popular: true},
	{Name: "postgres", Description: "PostgreSQL database queries and schema", Category: "Data", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-postgres"}, Popular: true},
	{Name: "sqlite", Description: "SQLite database exploration and queries", Category: "Data", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-sqlite"}},
	{Name: "fetch", Description: "Web content fetching and search", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-fetch"}, Popular: true},
	{Name: "brave-search", Description: "Web search via Brave Search API", Category: "AI", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-brave-search"}},
	{Name: "memory", Description: "Knowledge graph and persistent memory", Category: "AI", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-memory"}, Popular: true},
	{Name: "puppeteer", Description: "Browser automation via Puppeteer", Category: "Operations", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-puppeteer"}},
	{Name: "slack", Description: "Slack messaging and channel management", Category: "Communication", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-slack"}},
	{Name: "google-maps", Description: "Google Maps directions, places, geocoding", Category: "Productivity", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-google-maps"}},
	{Name: "sequential-thinking", Description: "Structured problem-solving and reasoning", Category: "AI", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-sequential-thinking"}},
	{Name: "everything", Description: "MCP test server with all features", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-everything"}},
}

func getCatalogWithInstalledStatus(registry *mcp.MCPServerRegistry) []MCPCatalogEntry {
	installed := make(map[string]bool)
	if registry != nil {
		for _, s := range registry.ListServers() {
			installed[s.Name] = true
		}
	}

	result := make([]MCPCatalogEntry, len(mcpCatalog))
	for i, entry := range mcpCatalog {
		result[i] = entry
		result[i].Installed = installed[entry.Name]
	}
	return result
}

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

func (h *Handler) handleMCPCatalogWS(client *Client) {
	catalog := getCatalogWithInstalledStatus(h.mcpRegistry)
	h.sendToClient(client, WSResponse{Type: "mcp_catalog", Data: catalog})
}

func (s *WebServer) handleMCPCatalogAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	catalog := getCatalogWithInstalledStatus(s.handler.mcpRegistry)
	writeJSON(w, http.StatusOK, catalog)
}

func (s *WebServer) handleMCPStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.handler.mcpRegistry == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	name := r.URL.Query().Get("name")
	if name != "" {
		running := s.handler.mcpRegistry.IsServerRunning(name)
		writeJSON(w, http.StatusOK, map[string]any{
			"name":    name,
			"running": running,
		})
		return
	}

	servers := s.handler.mcpRegistry.ListServers()
	statuses := make([]map[string]any, 0, len(servers))
	for _, srv := range servers {
		statuses = append(statuses, map[string]any{
			"name":       srv.Name,
			"running":    s.handler.mcpRegistry.IsServerRunning(srv.Name),
			"type":       srv.Type,
			"auto_start": srv.AutoStart,
		})
	}
	writeJSON(w, http.StatusOK, statuses)
}

func (s *WebServer) handleMCPToolsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.handler.mcpRegistry == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name query parameter is required"})
		return
	}

	client, exists := s.handler.mcpRegistry.GetClient(name)
	if !exists || client == nil || !client.IsReady() {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":  name,
			"tools": []any{},
			"error": "server not running",
		})
		return
	}

	tools, err := client.ListTools(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":  name,
			"tools": []any{},
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"name":  name,
		"tools": tools,
	})
}

func (s *WebServer) handleMCPResourcesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.handler.mcpRegistry == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name query parameter is required"})
		return
	}

	client, exists := s.handler.mcpRegistry.GetClient(name)
	if !exists || client == nil || !client.IsReady() {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":      name,
			"resources": []any{},
			"error":     "server not running",
		})
		return
	}

	resources, err := client.ListResources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":      name,
			"resources": []any{},
			"error":     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"name":      name,
		"resources": resources,
	})
}

func (h *Handler) handleMCPToolsWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid MCP tools request")
		return
	}
	name, _ := data["name"].(string)
	if name == "" {
		h.sendError(client, "Server name is required")
		return
	}
	if h.mcpRegistry == nil {
		h.sendToClient(client, WSResponse{Type: "mcp_tools", Data: map[string]any{
			"name": name, "tools": []any{}, "error": "registry not available",
		}})
		return
	}

	mcpClient, exists := h.mcpRegistry.GetClient(name)
	if !exists || mcpClient == nil || !mcpClient.IsReady() {
		h.sendToClient(client, WSResponse{Type: "mcp_tools", Data: map[string]any{
			"name": name, "tools": []any{}, "error": "server not running",
		}})
		return
	}

	tools, err := mcpClient.ListTools(context.Background())
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "mcp_tools", Data: map[string]any{
			"name": name, "tools": []any{}, "error": err.Error(),
		}})
		return
	}

	h.sendToClient(client, WSResponse{Type: "mcp_tools", Data: map[string]any{
		"name": name, "tools": tools,
	}})
}

func (h *Handler) handleMCPResourcesWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid MCP resources request")
		return
	}
	name, _ := data["name"].(string)
	if name == "" {
		h.sendError(client, "Server name is required")
		return
	}
	if h.mcpRegistry == nil {
		h.sendToClient(client, WSResponse{Type: "mcp_resources", Data: map[string]any{
			"name": name, "resources": []any{}, "error": "registry not available",
		}})
		return
	}

	mcpClient, exists := h.mcpRegistry.GetClient(name)
	if !exists || mcpClient == nil || !mcpClient.IsReady() {
		h.sendToClient(client, WSResponse{Type: "mcp_resources", Data: map[string]any{
			"name": name, "resources": []any{}, "error": "server not running",
		}})
		return
	}

	resources, err := mcpClient.ListResources(context.Background())
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "mcp_resources", Data: map[string]any{
			"name": name, "resources": []any{}, "error": err.Error(),
		}})
		return
	}

	h.sendToClient(client, WSResponse{Type: "mcp_resources", Data: map[string]any{
		"name": name, "resources": resources,
	}})
}
