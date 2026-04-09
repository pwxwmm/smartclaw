package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Task struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Priority    string                 `json:"priority"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Assignee    string                 `json:"assignee,omitempty"`
	Labels      []string               `json:"labels,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type TaskManager struct {
	tasks    map[string]*Task
	mu       sync.RWMutex
	dataFile string
}

func NewTaskManager(dataDir string) (*TaskManager, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(home, ".smartclaw", "tasks")
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	tm := &TaskManager{
		tasks:    make(map[string]*Task),
		dataFile: filepath.Join(dataDir, "tasks.json"),
	}

	tm.load()
	return tm, nil
}

func (tm *TaskManager) load() error {
	data, err := os.ReadFile(tm.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	for _, task := range tasks {
		tm.tasks[task.ID] = task
	}

	return nil
}

func (tm *TaskManager) save() error {
	tasks := make([]*Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tasks = append(tasks, task)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tm.dataFile, data, 0644)
}

func (tm *TaskManager) Create(title, description string, priority string) *Task {
	if priority == "" {
		priority = "medium"
	}

	task := &Task{
		ID:          fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Title:       title,
		Description: description,
		Status:      "pending",
		Priority:    priority,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Labels:      []string{},
		Metadata:    make(map[string]interface{}),
	}

	tm.mu.Lock()
	tm.tasks[task.ID] = task
	tm.mu.Unlock()

	tm.save()
	return task
}

func (tm *TaskManager) Get(id string) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tasks[id]
}

func (tm *TaskManager) List(filter map[string]interface{}) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range tm.tasks {
		if filter == nil {
			result = append(result, task)
			continue
		}

		match := true
		if status, ok := filter["status"].(string); ok && status != "" {
			match = match && task.Status == status
		}
		if priority, ok := filter["priority"].(string); ok && priority != "" {
			match = match && task.Priority == priority
		}

		if match {
			result = append(result, task)
		}
	}

	return result
}

func (tm *TaskManager) Update(id string, updates map[string]interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	if status, ok := updates["status"].(string); ok {
		task.Status = status
	}
	if title, ok := updates["title"].(string); ok {
		task.Title = title
	}
	if description, ok := updates["description"].(string); ok {
		task.Description = description
	}
	if priority, ok := updates["priority"].(string); ok {
		task.Priority = priority
	}

	task.UpdatedAt = time.Now()
	tm.save()

	return nil
}

func (tm *TaskManager) Stop(id string) error {
	return tm.Update(id, map[string]interface{}{"status": "stopped"})
}

type TaskCreateTool struct {
	manager *TaskManager
}

func NewTaskCreateTool(tm *TaskManager) *TaskCreateTool {
	return &TaskCreateTool{manager: tm}
}

func (t *TaskCreateTool) Name() string        { return "task_create" }
func (t *TaskCreateTool) Description() string { return "Create a new task" }

func (t *TaskCreateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title":       map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
			"priority":    map[string]interface{}{"type": "string"},
		},
		"required": []string{"title"},
	}
}

func (t *TaskCreateTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	title, _ := input["title"].(string)
	if title == "" {
		return nil, ErrRequiredField("title")
	}

	description, _ := input["description"].(string)
	priority, _ := input["priority"].(string)

	task := t.manager.Create(title, description, priority)
	return task, nil
}

type TaskGetTool struct {
	manager *TaskManager
}

func NewTaskGetTool(tm *TaskManager) *TaskGetTool {
	return &TaskGetTool{manager: tm}
}

func (t *TaskGetTool) Name() string        { return "task_get" }
func (t *TaskGetTool) Description() string { return "Get a task by ID" }

func (t *TaskGetTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"},
		},
		"required": []string{"id"},
	}
}

func (t *TaskGetTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	id, _ := input["id"].(string)
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	task := t.manager.Get(id)
	if task == nil {
		return nil, &Error{Code: "NOT_FOUND", Message: "task not found"}
	}

	return task, nil
}

type TaskListTool struct {
	manager *TaskManager
}

func NewTaskListTool(tm *TaskManager) *TaskListTool {
	return &TaskListTool{manager: tm}
}

func (t *TaskListTool) Name() string        { return "task_list" }
func (t *TaskListTool) Description() string { return "List all tasks" }

func (t *TaskListTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status":   map[string]interface{}{"type": "string"},
			"priority": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *TaskListTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	tasks := t.manager.List(input)
	return map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	}, nil
}

type TaskUpdateTool struct {
	manager *TaskManager
}

func NewTaskUpdateTool(tm *TaskManager) *TaskUpdateTool {
	return &TaskUpdateTool{manager: tm}
}

func (t *TaskUpdateTool) Name() string        { return "task_update" }
func (t *TaskUpdateTool) Description() string { return "Update a task" }

func (t *TaskUpdateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":          map[string]interface{}{"type": "string"},
			"title":       map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
			"status":      map[string]interface{}{"type": "string"},
			"priority":    map[string]interface{}{"type": "string"},
		},
		"required": []string{"id"},
	}
}

func (t *TaskUpdateTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	id, _ := input["id"].(string)
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	updates := make(map[string]interface{})
	for k, v := range input {
		if k != "id" {
			updates[k] = v
		}
	}

	if err := t.manager.Update(id, updates); err != nil {
		return nil, &Error{Code: "UPDATE_ERROR", Message: err.Error()}
	}

	return t.manager.Get(id), nil
}

type TaskStopTool struct {
	manager *TaskManager
}

func NewTaskStopTool(tm *TaskManager) *TaskStopTool {
	return &TaskStopTool{manager: tm}
}

func (t *TaskStopTool) Name() string        { return "task_stop" }
func (t *TaskStopTool) Description() string { return "Stop a task" }

func (t *TaskStopTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"},
		},
		"required": []string{"id"},
	}
}

func (t *TaskStopTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	id, _ := input["id"].(string)
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	if err := t.manager.Stop(id); err != nil {
		return nil, &Error{Code: "STOP_ERROR", Message: err.Error()}
	}

	return map[string]interface{}{
		"id":     id,
		"status": "stopped",
	}, nil
}

type TaskOutputTool struct {
	manager *TaskManager
}

func NewTaskOutputTool(tm *TaskManager) *TaskOutputTool {
	return &TaskOutputTool{manager: tm}
}

func (t *TaskOutputTool) Name() string        { return "task_output" }
func (t *TaskOutputTool) Description() string { return "Get task output" }

func (t *TaskOutputTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"},
		},
		"required": []string{"id"},
	}
}

func (t *TaskOutputTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	id, _ := input["id"].(string)
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	task := t.manager.Get(id)
	if task == nil {
		return nil, &Error{Code: "NOT_FOUND", Message: "task not found"}
	}

	return map[string]interface{}{
		"id":          id,
		"status":      task.Status,
		"title":       task.Title,
		"description": task.Description,
	}, nil
}
