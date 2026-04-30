package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/instructkr/smartclaw/internal/gateway"
)

func (h *Handler) handleCronListWS(client *Client) {
	if h.cronTrigger == nil {
		h.sendToClient(client, WSResponse{Type: "cron_list", Data: map[string]any{
			"tasks": []any{},
		}})
		return
	}

	tasks, err := h.cronTrigger.ListTasks()
	if err != nil {
		h.sendError(client, "Failed to list cron tasks: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{Type: "cron_list", Data: map[string]any{
		"tasks": enrichTasks(tasks),
	}})
}

func (h *Handler) handleCronCreateWS(client *Client, msg WSMessage) {
	var req struct {
		Instruction string `json:"instruction"`
		Schedule    string `json:"schedule"`
		Platform    string `json:"platform"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.sendError(client, "Invalid cron create request")
		return
	}

	if req.Instruction == "" {
		h.sendError(client, "instruction is required")
		return
	}
	if req.Schedule == "" {
		h.sendError(client, "schedule is required")
		return
	}
	if req.Platform == "" {
		req.Platform = "web"
	}

	expr, err := gateway.ParseNaturalLanguage(req.Schedule)
	if err != nil {
		h.sendError(client, "Cannot parse schedule: "+err.Error())
		return
	}

	if err := gateway.ValidateCronExpression(expr); err != nil {
		h.sendError(client, "Invalid schedule: "+err.Error())
		return
	}

	if h.cronTrigger == nil {
		h.sendError(client, "Cron system not available")
		return
	}

	taskID := uuid.New().String()[:8]
	userID := client.UserID

	if err := h.cronTrigger.ScheduleCron(taskID, userID, req.Instruction, expr, req.Platform); err != nil {
		h.sendError(client, "Failed to create cron task: "+err.Error())
		return
	}

	task, _ := h.cronTrigger.GetTask(taskID)
	h.sendToClient(client, WSResponse{Type: "cron_created", Data: map[string]any{
		"success":   true,
		"task":      enrichTask(task),
		"parsed":    expr,
		"humanized": gateway.DescribeCronExpression(expr),
	}})
}

func (h *Handler) handleCronDeleteWS(client *Client, msg WSMessage) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.sendError(client, "Invalid cron delete request")
		return
	}
	if req.ID == "" {
		h.sendError(client, "id is required")
		return
	}

	if h.cronTrigger == nil {
		h.sendError(client, "Cron system not available")
		return
	}

	if err := h.cronTrigger.DeleteTask(req.ID); err != nil {
		h.sendError(client, "Failed to delete cron task: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{Type: "cron_deleted", Data: map[string]any{
		"success": true,
		"id":      req.ID,
	}})
}

func (h *Handler) handleCronToggleWS(client *Client, msg WSMessage) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.sendError(client, "Invalid cron toggle request")
		return
	}
	if req.ID == "" {
		h.sendError(client, "id is required")
		return
	}

	if h.cronTrigger == nil {
		h.sendError(client, "Cron system not available")
		return
	}

	task, err := h.cronTrigger.GetTask(req.ID)
	if err != nil {
		h.sendError(client, "Task not found: "+err.Error())
		return
	}

	if task.Enabled {
		err = h.cronTrigger.DisableTask(req.ID)
	} else {
		err = h.cronTrigger.EnableTask(req.ID)
	}
	if err != nil {
		h.sendError(client, "Failed to toggle task: "+err.Error())
		return
	}

	task, _ = h.cronTrigger.GetTask(req.ID)
	h.sendToClient(client, WSResponse{Type: "cron_toggled", Data: map[string]any{
		"success": true,
		"task":    enrichTask(task),
	}})
}

func (h *Handler) handleCronRunWS(client *Client, msg WSMessage) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.sendError(client, "Invalid cron run request")
		return
	}
	if req.ID == "" {
		h.sendError(client, "id is required")
		return
	}

	if h.cronTrigger == nil {
		h.sendError(client, "Cron system not available")
		return
	}

	task, err := h.cronTrigger.GetTask(req.ID)
	if err != nil {
		h.sendError(client, "Task not found: "+err.Error())
		return
	}

	task.LastRunAt = time.Now().Format(time.RFC3339)
	h.cronTrigger.ScheduleCron(task.ID, task.UserID, task.Instruction, task.Schedule, task.Platform)

	h.sendToClient(client, WSResponse{Type: "cron_triggered", Data: map[string]any{
		"success": true,
		"id":      req.ID,
		"message": "Task execution triggered",
	}})
}

func (s *WebServer) handleCronTasksAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleCronTaskListAPI(w, r)
	case http.MethodPost:
		s.handleCronTaskCreateAPI(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *WebServer) handleCronTaskListAPI(w http.ResponseWriter, r *http.Request) {
	if s.handler.cronTrigger == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tasks": []any{}})
		return
	}

	tasks, err := s.handler.cronTrigger.ListTasks()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"tasks": enrichTasks(tasks)})
}

func (s *WebServer) handleCronTaskCreateAPI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Instruction string `json:"instruction"`
		Schedule    string `json:"schedule"`
		Platform    string `json:"platform"`
		UserID      string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Instruction == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instruction is required"})
		return
	}
	if req.Schedule == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule is required"})
		return
	}
	if req.Platform == "" {
		req.Platform = "web"
	}
	if req.UserID == "" {
		req.UserID = "default"
	}

	expr, err := gateway.ParseNaturalLanguage(req.Schedule)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot parse schedule: " + err.Error()})
		return
	}

	if err := gateway.ValidateCronExpression(expr); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid schedule: " + err.Error()})
		return
	}

	if s.handler.cronTrigger == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	taskID := uuid.New().String()[:8]
	if err := s.handler.cronTrigger.ScheduleCron(taskID, req.UserID, req.Instruction, expr, req.Platform); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	task, _ := s.handler.cronTrigger.GetTask(taskID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"success":   true,
		"task":      enrichTask(task),
		"parsed":    expr,
		"humanized": gateway.DescribeCronExpression(expr),
	})
}

func (s *WebServer) handleCronTaskDetailAPI(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/cron/tasks/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task id is required"})
		return
	}

	if s.handler.cronTrigger == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := s.handler.cronTrigger.GetTask(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, enrichTask(task))

	case http.MethodDelete:
		if err := s.handler.cronTrigger.DeleteTask(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": id})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *WebServer) handleCronTaskToggleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/cron/tasks/")
	id = strings.TrimSuffix(id, "/")
	id = strings.TrimSuffix(id, "/toggle")

	if s.handler.cronTrigger == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	task, err := s.handler.cronTrigger.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	if task.Enabled {
		err = s.handler.cronTrigger.DisableTask(id)
	} else {
		err = s.handler.cronTrigger.EnableTask(id)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	task, _ = s.handler.cronTrigger.GetTask(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"task":    enrichTask(task),
	})
}

func (s *WebServer) handleCronTaskRunAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/cron/tasks/")
	id = strings.TrimSuffix(id, "/")
	id = strings.TrimSuffix(id, "/run")

	if s.handler.cronTrigger == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	task, err := s.handler.cronTrigger.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	task.LastRunAt = time.Now().Format(time.RFC3339)
	s.handler.cronTrigger.ScheduleCron(task.ID, task.UserID, task.Instruction, task.Schedule, task.Platform)

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"id":      id,
		"message": "Task execution triggered",
	})
}

type cronTaskJSON struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Instruction string `json:"instruction"`
	Schedule    string `json:"schedule"`
	Humanized   string `json:"humanized"`
	Platform    string `json:"platform"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	LastRunAt   string `json:"last_run_at,omitempty"`
}

func enrichTask(t *gateway.CronTask) cronTaskJSON {
	if t == nil {
		return cronTaskJSON{}
	}
	humanized := gateway.DescribeCronExpression(t.Schedule)
	return cronTaskJSON{
		ID:          t.ID,
		UserID:      t.UserID,
		Instruction: t.Instruction,
		Schedule:    t.Schedule,
		Humanized:   humanized,
		Platform:    t.Platform,
		Enabled:     t.Enabled,
		CreatedAt:   t.CreatedAt,
		LastRunAt:   t.LastRunAt,
	}
}

func enrichTasks(tasks []*gateway.CronTask) []cronTaskJSON {
	result := make([]cronTaskJSON, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, enrichTask(t))
	}
	return result
}

// cronRouteID extracts the task ID from a /api/cron/tasks/{id}/... path.
func cronRouteID(path, suffix string) string {
	prefix := "/api/cron/tasks/"
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimSuffix(s, "/")
	if suffix != "" {
		s = strings.TrimSuffix(s, "/"+suffix)
	}
	return s
}
