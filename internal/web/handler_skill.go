package web

import (
	"encoding/json"
	"os"

	"github.com/instructkr/smartclaw/internal/skills"
)

func (h *Handler) handleSkillListWS(client *Client) {
	sm := skills.GetSkillManager()
	if sm == nil {
		h.sendError(client, "Skill manager not available")
		return
	}
	skillList := sm.List()
	h.sendToClient(client, WSResponse{Type: "skill_list", Data: skillList})
}

func (h *Handler) handleSkillDetailWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid skill detail request")
		return
	}
	name, _ := data["name"].(string)
	sm := skills.GetSkillManager()
	if sm == nil {
		h.sendError(client, "Skill manager not available")
		return
	}
	skill := sm.Get(name)
	if skill == nil {
		h.sendError(client, "Skill not found: "+name)
		return
	}
	h.sendToClient(client, WSResponse{Type: "skill_detail", Data: skill})
}

func (h *Handler) handleSkillToggleWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid skill toggle request")
		return
	}
	name, _ := data["name"].(string)
	action, _ := data["action"].(string)
	sm := skills.GetSkillManager()
	if sm == nil {
		h.sendError(client, "Skill manager not available")
		return
	}
	switch action {
	case "enable":
		sm.Enable(name)
		h.sendToClient(client, WSResponse{Type: "skill_toggle", Data: map[string]string{"name": name, "status": "enabled"}})
	case "disable":
		sm.Disable(name)
		h.sendToClient(client, WSResponse{Type: "skill_toggle", Data: map[string]string{"name": name, "status": "disabled"}})
	default:
		h.sendError(client, "Invalid action: "+action)
	}
}

func (h *Handler) handleSkillSearchWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid skill search request")
		return
	}
	query, _ := data["query"].(string)
	sm := skills.GetSkillManager()
	if sm == nil {
		h.sendError(client, "Skill manager not available")
		return
	}
	results := sm.Search(query)
	h.sendToClient(client, WSResponse{Type: "skill_search", Data: results})
}

func (h *Handler) handleSkillCreateWS(client *Client, msg WSMessage) {
	var data struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Version     string   `json:"version"`
		Tags        []string `json:"tags"`
		Tools       []string `json:"tools"`
		Triggers    []string `json:"triggers"`
		Body        string   `json:"body"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid skill create request")
		return
	}

	if data.Name == "" || data.Description == "" {
		h.sendError(client, "Name and description are required")
		return
	}

	if data.Version == "" {
		data.Version = "1.0"
	}

	schema := &skills.SkillSchema{
		Name:        data.Name,
		Description: data.Description,
		Version:     data.Version,
		Tags:        data.Tags,
		Tools:       data.Tools,
		Triggers:    data.Triggers,
	}

	sm := skills.GetSkillManager()
	if sm == nil {
		h.sendError(client, "Skill manager not available")
		return
	}

	if err := sm.CreateSkill(schema, data.Body); err != nil {
		h.sendToClient(client, WSResponse{Type: "skill_create", Data: map[string]any{
			"success": false,
			"error":   err.Error(),
		}})
		return
	}

	h.sendToClient(client, WSResponse{Type: "skill_create", Data: map[string]any{
		"success": true,
		"name":    schema.Name,
	}})

	skillList := sm.List()
	h.sendToClient(client, WSResponse{Type: "skill_list", Data: skillList})
}

func (h *Handler) handleSkillEditWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid skill edit request")
		return
	}

	name, _ := data["name"].(string)
	content, _ := data["content"].(string)
	if name == "" {
		h.sendError(client, "Skill name is required")
		return
	}
	if content == "" {
		h.sendError(client, "Skill content must not be empty")
		return
	}

	sm := skills.GetSkillManager()
	if sm == nil {
		h.sendError(client, "Skill manager not available")
		return
	}

	skill := sm.Get(name)
	if skill == nil {
		h.sendError(client, "Skill not found: "+name)
		return
	}

	if skill.Source == "bundled" {
		h.sendError(client, "Cannot edit bundled skill: "+name)
		return
	}

	if skill.Path == "" {
		h.sendError(client, "Skill has no file path: "+name)
		return
	}

	if err := os.WriteFile(skill.Path, []byte(content), 0644); err != nil {
		h.sendError(client, "Failed to write skill file: "+err.Error())
		return
	}

	if err := sm.Reload(name); err != nil {
		h.sendError(client, "Failed to reload skill: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{Type: "skill_edit", Data: map[string]any{
		"success": true,
		"name":    name,
	}})

	skillList := sm.List()
	h.sendToClient(client, WSResponse{Type: "skill_list", Data: skillList})
}
