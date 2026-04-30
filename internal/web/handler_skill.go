package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/instructkr/smartclaw/internal/learning"
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

func (h *Handler) handleSkillHealthWS(client *Client) {
	report := h.getSkillHealthReport()
	h.sendToClient(client, WSResponse{Type: "skill_health", Data: report})
}

func (h *Handler) handleSkillImproveWS(client *Client, msg WSMessage) {
	var data struct {
		Name            string   `json:"name"`
		FailureMessages []string `json:"failure_messages"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid skill improve request")
		return
	}

	if data.Name == "" {
		h.sendError(client, "Skill name is required")
		return
	}

	if h.dataStore == nil {
		h.sendError(client, "Store not available for skill improvement")
		return
	}

	apiClient := h.apiClient
	if apiClient == nil {
		h.sendError(client, "API client not configured for skill improvement")
		return
	}

	llmAdapter := learning.NewAPIClientAdapter(apiClient, "")
	improver := learning.NewSkillImprover(llmAdapter)
	writer := learning.NewSkillWriter("")

	originalSkill, err := loadSkillForImprovement(data.Name)
	if err != nil {
		h.sendError(client, "Failed to load skill: "+err.Error())
		return
	}

	failures := data.FailureMessages
	if len(failures) == 0 {
		failures = []string{"Manual improvement triggered"}
	}

	improved, err := improver.Improve(context.Background(), data.Name, failures, originalSkill)
	if err != nil {
		h.sendError(client, "Skill improvement failed: "+err.Error())
		return
	}

	if err := improver.ApplyImprovement(writer, improved); err != nil {
		h.sendError(client, "Failed to apply improvement: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{Type: "skill_improve", Data: map[string]any{
		"success":        true,
		"name":           improved.Name,
		"version":        improved.Version,
		"change_summary": improved.ChangeSummary,
	}})
}

func (h *Handler) getSkillHealthReport() any {
	if h.skillTracker == nil && h.dataStore == nil {
		return map[string]any{
			"skills":       []any{},
			"generated_at": time.Now().Format(time.RFC3339),
			"healthy":      0,
			"degraded":     0,
			"failing":      0,
			"unused":       0,
		}
	}

	tracker := h.skillTracker
	if tracker == nil {
		tracker = learning.NewSkillTracker(h.dataStore)
	}

	report, err := tracker.GetHealthReport()
	if err != nil {
		return map[string]any{
			"skills":       []any{},
			"generated_at": time.Now().Format(time.RFC3339),
			"error":        err.Error(),
		}
	}

	type healthEntry struct {
		SkillID          string   `json:"skill_id"`
		SuccessRate      float64  `json:"success_rate"`
		TotalInvocations int      `json:"total_invocations"`
		Trend            string   `json:"trend"`
		LastUsed         string   `json:"last_used,omitempty"`
		Health           string   `json:"health"`
		Recommendation   string   `json:"recommendation"`
	}

	entries := make([]healthEntry, 0, len(report.Skills))
	for _, s := range report.Skills {
		entry := healthEntry{
			SkillID:          s.SkillID,
			SuccessRate:      s.SuccessRate,
			TotalInvocations: s.TotalInvocations,
			Trend:            s.Trend,
			Health:           string(s.Health),
			Recommendation:   s.Recommendation,
		}
		if s.LastUsed != nil {
			entry.LastUsed = s.LastUsed.Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	return map[string]any{
		"skills":       entries,
		"generated_at": report.GeneratedAt.Format(time.RFC3339),
		"healthy":      report.Healthy,
		"degraded":     report.Degraded,
		"failing":      report.Failing,
		"unused":       report.Unused,
	}
}

func loadSkillForImprovement(name string) (*learning.ExtractedSkill, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	skillPath := home + "/.smartclaw/skills/" + name + "/SKILL.md"
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return &learning.ExtractedSkill{
			Name:        name,
			Description: "Skill loaded for improvement",
			Steps:       []string{"Execute the skill"},
			Tools:       []string{"bash"},
			Tags:        []string{"learned"},
		}, nil
	}

	return learning.ParseExistingSkill(name, string(data)), nil
}
