package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewTaskManager(t *testing.T) {
	tmpDir := t.TempDir()
	tm, err := NewTaskManager(tmpDir)
	if err != nil {
		t.Fatalf("NewTaskManager failed: %v", err)
	}
	if tm == nil {
		t.Fatal("TaskManager should not be nil")
	}
}

func TestNewTaskManagerEmptyDir(t *testing.T) {
	tm, err := NewTaskManager("")
	if err != nil {
		t.Fatalf("NewTaskManager with empty dir should use default: %v", err)
	}
	if tm == nil {
		t.Fatal("TaskManager should not be nil")
	}
}

func TestTaskManagerCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)

	task := tm.Create("Test Task", "A description", "high")
	if task == nil {
		t.Fatal("Create should return a task")
	}
	if task.Title != "Test Task" {
		t.Errorf("Expected title 'Test Task', got %s", task.Title)
	}
	if task.Status != "pending" {
		t.Errorf("Expected status 'pending', got %s", task.Status)
	}
	if task.Priority != "high" {
		t.Errorf("Expected priority 'high', got %s", task.Priority)
	}

	got := tm.Get(task.ID)
	if got == nil {
		t.Fatal("Get should return the task")
	}
	if got.Title != "Test Task" {
		t.Error("Get should return same task")
	}

	gotNil := tm.Get("nonexistent")
	if gotNil != nil {
		t.Error("Get nonexistent should return nil")
	}
}

func TestTaskManagerDefaultPriority(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)

	task := tm.Create("Task", "desc", "")
	if task.Priority != "medium" {
		t.Errorf("Expected default priority 'medium', got %s", task.Priority)
	}
}

func TestTaskManagerList(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)

	tm.Create("Task1", "", "high")
	tm.Create("Task2", "", "low")

	all := tm.List(nil)
	if len(all) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(all))
	}

	highOnly := tm.List(map[string]any{"priority": "high"})
	if len(highOnly) != 1 {
		t.Errorf("Expected 1 high priority task, got %d", len(highOnly))
	}

	pendingOnly := tm.List(map[string]any{"status": "pending"})
	if len(pendingOnly) != 2 {
		t.Errorf("Expected 2 pending tasks, got %d", len(pendingOnly))
	}
}

func TestTaskManagerUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)

	task := tm.Create("Original", "desc", "medium")

	err := tm.Update(task.ID, map[string]any{
		"title":       "Updated",
		"description": "new desc",
		"status":      "in_progress",
		"priority":    "high",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated := tm.Get(task.ID)
	if updated.Title != "Updated" {
		t.Errorf("Expected title 'Updated', got %s", updated.Title)
	}
	if updated.Status != "in_progress" {
		t.Errorf("Expected status 'in_progress', got %s", updated.Status)
	}
	if updated.Priority != "high" {
		t.Errorf("Expected priority 'high', got %s", updated.Priority)
	}
}

func TestTaskManagerUpdateNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)

	err := tm.Update("nonexistent", map[string]any{"status": "done"})
	if err == nil {
		t.Error("Expected error for nonexistent task")
	}
}

func TestTaskManagerStop(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)

	task := tm.Create("Task", "", "medium")
	err := tm.Stop(task.ID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stopped := tm.Get(task.ID)
	if stopped.Status != "stopped" {
		t.Errorf("Expected status 'stopped', got %s", stopped.Status)
	}
}

func TestTaskManagerLoadFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	tm1, _ := NewTaskManager(tmpDir)
	tm1.Create("Persisted Task", "should survive restart", "high")

	tm2, _ := NewTaskManager(tmpDir)
	tasks := tm2.List(nil)
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task after reload, got %d", len(tasks))
	}
	if tasks[0].Title != "Persisted Task" {
		t.Errorf("Expected 'Persisted Task', got %s", tasks[0].Title)
	}
}

func TestTaskManagerLoadInvalidData(t *testing.T) {
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "tasks.json")
	os.WriteFile(dataFile, []byte("invalid json"), 0644)

	tm, err := NewTaskManager(tmpDir)
	if err != nil {
		t.Fatalf("NewTaskManager should handle invalid data: %v", err)
	}
	if tm == nil {
		t.Fatal("TaskManager should not be nil even with bad data")
	}
}

func TestTaskCreateToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskCreateTool(tm)

	result, err := tool.Execute(context.Background(), map[string]any{
		"title":       "Test Task",
		"description": "A test",
		"priority":    "high",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	task := result.(*Task)
	if task.Title != "Test Task" {
		t.Errorf("Expected 'Test Task', got %s", task.Title)
	}
}

func TestTaskCreateToolMissingTitle(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskCreateTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing title")
	}
}

func TestTaskGetToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskGetTool(tm)

	task := tm.Create("Find Me", "", "medium")
	result, err := tool.Execute(context.Background(), map[string]any{
		"id": task.ID,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	got := result.(*Task)
	if got.Title != "Find Me" {
		t.Errorf("Expected 'Find Me', got %s", got.Title)
	}
}

func TestTaskGetToolMissingID(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskGetTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestTaskGetToolNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskGetTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{
		"id": "nonexistent",
	})
	if err == nil {
		t.Error("Expected error for not found")
	}
}

func TestTaskListToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskListTool(tm)

	tm.Create("Task1", "", "high")
	tm.Create("Task2", "", "low")

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["count"].(int) != 2 {
		t.Errorf("Expected count=2, got %d", m["count"])
	}
}

func TestTaskUpdateToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskUpdateTool(tm)

	task := tm.Create("Original", "", "medium")

	result, err := tool.Execute(context.Background(), map[string]any{
		"id":     task.ID,
		"title":  "Updated",
		"status": "done",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	updated := result.(*Task)
	if updated.Title != "Updated" {
		t.Errorf("Expected 'Updated', got %s", updated.Title)
	}
}

func TestTaskUpdateToolMissingID(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskUpdateTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestTaskUpdateToolNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskUpdateTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{
		"id":     "nonexistent",
		"status": "done",
	})
	if err == nil {
		t.Error("Expected error for nonexistent task")
	}
}

func TestTaskStopToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskStopTool(tm)

	task := tm.Create("Stop Me", "", "medium")

	result, err := tool.Execute(context.Background(), map[string]any{
		"id": task.ID,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["status"] != "stopped" {
		t.Errorf("Expected status 'stopped', got %v", m["status"])
	}
}

func TestTaskStopToolMissingID(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskStopTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestTaskOutputToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskOutputTool(tm)

	task := tm.Create("Output Task", "desc", "medium")

	result, err := tool.Execute(context.Background(), map[string]any{
		"id": task.ID,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["title"] != "Output Task" {
		t.Errorf("Expected title 'Output Task', got %v", m["title"])
	}
}

func TestTaskOutputToolMissingID(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskOutputTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestTaskOutputToolNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)
	tool := NewTaskOutputTool(tm)

	_, err := tool.Execute(context.Background(), map[string]any{
		"id": "nonexistent",
	})
	if err == nil {
		t.Error("Expected error for not found")
	}
}

func TestTaskToolSchemas(t *testing.T) {
	tmpDir := t.TempDir()
	tm, _ := NewTaskManager(tmpDir)

	tools := []Tool{
		NewTaskCreateTool(tm),
		NewTaskGetTool(tm),
		NewTaskListTool(tm),
		NewTaskUpdateTool(tm),
		NewTaskStopTool(tm),
		NewTaskOutputTool(tm),
	}
	for _, tool := range tools {
		if tool.Name() == "" {
			t.Error("Tool name should not be empty")
		}
		if tool.InputSchema() == nil {
			t.Errorf("Tool %s: InputSchema should not be nil", tool.Name())
		}
	}
}
