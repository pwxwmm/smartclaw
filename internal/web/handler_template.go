package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

// Preset templates built into the system.
var presetTemplates = []map[string]any{
	{
		"id":          "preset-code-review",
		"name":        "Code Review",
		"description": "Review code for bugs, performance, and best practices",
		"category":    "Code Quality",
		"content":     "Review this code for bugs, performance issues, and best practices:\n\n{{file}}\n\nFocus on: error handling, security, readability",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-bug-fix",
		"name":        "Bug Fix",
		"description": "Analyze and fix bugs in code",
		"category":    "Debugging",
		"content":     "Analyze and fix the bug in this code:\n\n{{file}}\n\nThe issue is: ",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-doc-generation",
		"name":        "Doc Generation",
		"description": "Generate documentation for code",
		"category":    "Documentation",
		"content":     "Generate documentation for:\n\n{{file}}\n\nInclude: function descriptions, parameters, return values, usage examples",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-test-writing",
		"name":        "Test Writing",
		"description": "Write comprehensive tests for code",
		"category":    "Testing",
		"content":     "Write tests for:\n\n{{file}}\n\nCover: edge cases, error paths, happy path",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-refactor",
		"name":        "Refactor",
		"description": "Refactor code for better readability and maintainability",
		"category":    "Code Quality",
		"content":     "Refactor this code for better readability and maintainability:\n\n{{file}}\n\nKeep the same functionality",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-explain-code",
		"name":        "Explain Code",
		"description": "Explain what code does step by step",
		"category":    "Learning",
		"content":     "Explain what this code does step by step:\n\n{{file}}",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-security-audit",
		"name":        "Security Audit",
		"description": "Perform a security audit on code",
		"category":    "Security",
		"content":     "Perform a security audit on:\n\n{{file}}\n\nCheck for: injection, auth issues, data exposure, misconfigurations",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-performance-review",
		"name":        "Performance Review",
		"description": "Review performance of code",
		"category":    "Performance",
		"content":     "Review performance of:\n\n{{file}}\n\nIdentify: bottlenecks, unnecessary allocations, O(n²) patterns",
		"variables":   []string{"file"},
		"isPreset":    true,
	},
	{
		"id":          "preset-git-commit",
		"name":        "Git Commit Message",
		"description": "Generate a commit message for changes",
		"category":    "Git",
		"content":     "Generate a commit message for these changes:\n\n{{selection}}",
		"variables":   []string{"selection"},
		"isPreset":    true,
	},
	{
		"id":          "preset-readme-generator",
		"name":        "README Generator",
		"description": "Generate a README.md for a project",
		"category":    "Documentation",
		"content":     "Generate a README.md for this project:\n\nKey files: {{file}}\nLanguage: {{language}}",
		"variables":   []string{"file", "language"},
		"isPreset":    true,
	},
}

func (h *Handler) handleTemplateListWS(client *Client) {
	result := make([]map[string]any, 0, len(presetTemplates))
	result = append(result, presetTemplates...)

	if h.dataStore != nil {
		custom, err := h.loadCustomTemplates()
		if err == nil {
			result = append(result, custom...)
		}
	}

	h.sendToClient(client, WSResponse{Type: "template_list", Data: result})
}

func (h *Handler) handleTemplateCreateWS(client *Client, msg WSMessage) {
	var data struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Category    string   `json:"category"`
		Content     string   `json:"content"`
		Variables   []string `json:"variables"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid template create request")
		return
	}

	if data.Name == "" || data.Content == "" {
		h.sendError(client, "Name and content are required")
		return
	}

	if data.ID == "" {
		data.ID = "custom-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}

	if data.Category == "" {
		data.Category = "Custom"
	}

	if len(data.Variables) == 0 {
		data.Variables = extractVariables(data.Content)
	}

	if h.dataStore == nil {
		h.sendError(client, "Database not available")
		return
	}

	variablesJSON, _ := json.Marshal(data.Variables)

	_, err := h.dataStore.DB().Exec(
		`INSERT OR REPLACE INTO prompt_templates (id, user_id, name, description, category, content, variables, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		data.ID, "default", data.Name, data.Description, data.Category, data.Content, string(variablesJSON),
	)
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "template_create", Data: map[string]any{
			"success": false,
			"error":   err.Error(),
		}})
		return
	}

	h.sendToClient(client, WSResponse{Type: "template_create", Data: map[string]any{
		"success": true,
		"id":      data.ID,
		"name":    data.Name,
	}})

	h.handleTemplateListWS(client)
}

func (h *Handler) handleTemplateUpdateWS(client *Client, msg WSMessage) {
	var data struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Category    string   `json:"category"`
		Content     string   `json:"content"`
		Variables   []string `json:"variables"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid template update request")
		return
	}

	if data.ID == "" {
		h.sendError(client, "Template ID is required")
		return
	}

	if data.Name == "" || data.Content == "" {
		h.sendError(client, "Name and content are required")
		return
	}

	if len(data.Variables) == 0 {
		data.Variables = extractVariables(data.Content)
	}

	if h.dataStore == nil {
		h.sendError(client, "Database not available")
		return
	}

	variablesJSON, _ := json.Marshal(data.Variables)

	result, err := h.dataStore.DB().Exec(
		`UPDATE prompt_templates SET name=?, description=?, category=?, content=?, variables=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=?`,
		data.Name, data.Description, data.Category, data.Content, string(variablesJSON), data.ID, "default",
	)
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "template_update", Data: map[string]any{
			"success": false,
			"error":   err.Error(),
		}})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		h.sendError(client, "Template not found or is a preset")
		return
	}

	h.sendToClient(client, WSResponse{Type: "template_update", Data: map[string]any{
		"success": true,
		"id":      data.ID,
		"name":    data.Name,
	}})

	h.handleTemplateListWS(client)
}

func (h *Handler) handleTemplateDeleteWS(client *Client, msg WSMessage) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid template delete request")
		return
	}

	if data.ID == "" {
		h.sendError(client, "Template ID is required")
		return
	}

	// Don't allow deleting preset templates
	if len(data.ID) >= 7 && data.ID[:7] == "preset-" {
		h.sendError(client, "Cannot delete preset templates")
		return
	}

	if h.dataStore == nil {
		h.sendError(client, "Database not available")
		return
	}

	result, err := h.dataStore.DB().Exec(`DELETE FROM prompt_templates WHERE id=? AND user_id=?`, data.ID, "default")
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "template_delete", Data: map[string]any{
			"success": false,
			"error":   err.Error(),
		}})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		h.sendError(client, "Template not found")
		return
	}

	h.sendToClient(client, WSResponse{Type: "template_delete", Data: map[string]any{
		"success": true,
		"id":      data.ID,
	}})

	h.handleTemplateListWS(client)
}

func (h *Handler) loadCustomTemplates() ([]map[string]any, error) {
	if h.dataStore == nil {
		return nil, fmt.Errorf("database not available")
	}

	rows, err := h.dataStore.DB().Query(
		`SELECT id, user_id, name, description, category, content, variables, created_at, updated_at FROM prompt_templates WHERE user_id=? ORDER BY updated_at DESC`,
		"default",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []map[string]any
	for rows.Next() {
		var id, userID, name, description, category, content, variablesJSON string
		var createdAt, updatedAt string
		if err := rows.Scan(&id, &userID, &name, &description, &category, &content, &variablesJSON, &createdAt, &updatedAt); err != nil {
			continue
		}

		var variables []string
		if err := json.Unmarshal([]byte(variablesJSON), &variables); err != nil {
			variables = extractVariables(content)
		}

		templates = append(templates, map[string]any{
			"id":          id,
			"name":        name,
			"description": description,
			"category":    category,
			"content":     content,
			"variables":   variables,
			"isPreset":    false,
			"createdAt":   createdAt,
			"updatedAt":   updatedAt,
		})
	}

	return templates, nil
}

// HandleTemplatesAPI is the REST handler for /api/templates
func (s *WebServer) handleTemplatesAPI(w http.ResponseWriter, r *http.Request) {
	_ = store.Store{}
	switch r.Method {
	case http.MethodGet:
		result := make([]map[string]any, 0, len(presetTemplates))
		result = append(result, presetTemplates...)

		if s.handler.dataStore != nil {
			custom, err := s.handler.loadCustomTemplates()
			if err == nil {
				result = append(result, custom...)
			}
		}

		writeJSON(w, http.StatusOK, result)

	case http.MethodPost:
		var req struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Category    string   `json:"category"`
			Content     string   `json:"content"`
			Variables   []string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		if req.Name == "" || req.Content == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and content are required"})
			return
		}

		if req.ID == "" {
			req.ID = "custom-" + fmt.Sprintf("%d", time.Now().UnixNano())
		}
		if req.Category == "" {
			req.Category = "Custom"
		}
		if len(req.Variables) == 0 {
			req.Variables = extractVariables(req.Content)
		}

		if s.handler.dataStore == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		variablesJSON, _ := json.Marshal(req.Variables)
		_, err := s.handler.dataStore.DB().Exec(
			`INSERT OR REPLACE INTO prompt_templates (id, user_id, name, description, category, content, variables, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			req.ID, "default", req.Name, req.Description, req.Category, req.Content, string(variablesJSON),
		)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"success": true,
			"id":      req.ID,
			"name":    req.Name,
		})

	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}
		if len(req.ID) >= 7 && req.ID[:7] == "preset-" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete preset templates"})
			return
		}

		if s.handler.dataStore == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		result, err := s.handler.dataStore.DB().Exec(`DELETE FROM prompt_templates WHERE id=? AND user_id=?`, req.ID, "default")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		affected, _ := result.RowsAffected()
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "affected": affected})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// extractVariables finds all {{variable}} placeholders in template content.
func extractVariables(content string) []string {
	var vars []string
	seen := map[string]bool{}
	start := 0
	for {
		idx := indexOfDoubleBrace(content, start)
		if idx == -1 {
			break
		}
		end := indexOfDoubleBraceClose(content, idx+2)
		if end == -1 {
			break
		}
		name := content[idx+2 : end]
		if name != "" && !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
		start = end + 2
	}
	return vars
}

func indexOfDoubleBrace(s string, start int) int {
	for i := start; i <= len(s)-2; i++ {
		if s[i] == '{' && s[i+1] == '{' {
			return i
		}
	}
	return -1
}

func indexOfDoubleBraceClose(s string, start int) int {
	for i := start; i <= len(s)-2; i++ {
		if s[i] == '}' && s[i+1] == '}' {
			return i
		}
	}
	return -1
}
