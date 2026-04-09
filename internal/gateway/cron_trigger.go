package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type CronTask struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Instruction string `json:"instruction"`
	Schedule    string `json:"schedule"`
	Platform    string `json:"platform"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	LastRunAt   string `json:"last_run_at,omitempty"`
}

type CronTrigger struct {
	store   *store.Store
	gateway *Gateway
	cronDir string
	stopCh  chan struct{}
	running bool
	mu      sync.Mutex
}

func NewCronTrigger(s *store.Store, gw *Gateway) *CronTrigger {
	home, _ := os.UserHomeDir()
	cronDir := filepath.Join(home, ".smartclaw", "cron")

	return &CronTrigger{
		store:   s,
		gateway: gw,
		cronDir: cronDir,
		stopCh:  make(chan struct{}),
	}
}

func (ct *CronTrigger) ScheduleCron(taskID, userID, instruction, schedule, platform string) error {
	if err := os.MkdirAll(ct.cronDir, 0755); err != nil {
		return fmt.Errorf("cron: mkdir: %w", err)
	}

	task := &CronTask{
		ID:          taskID,
		UserID:      userID,
		Instruction: instruction,
		Schedule:    schedule,
		Platform:    platform,
		Enabled:     true,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("cron: marshal: %w", err)
	}

	path := filepath.Join(ct.cronDir, taskID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cron: write: %w", err)
	}

	slog.Info("cron: scheduled task", "id", taskID, "schedule", schedule)
	return nil
}

func (ct *CronTrigger) Start() {
	ct.mu.Lock()
	if ct.running {
		ct.mu.Unlock()
		return
	}
	ct.running = true
	ct.mu.Unlock()

	go ct.runLoop()
	slog.Info("cron: trigger started")
}

func (ct *CronTrigger) Stop() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if ct.running {
		ct.stopCh <- struct{}{}
		ct.running = false
	}
}

func (ct *CronTrigger) runLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ct.stopCh:
			slog.Info("cron: trigger stopped")
			return
		case <-ticker.C:
			ct.tick()
		}
	}
}

func (ct *CronTrigger) tick() {
	tasks, err := ct.loadDueTasks()
	if err != nil {
		slog.Warn("cron: failed to load tasks", "error", err)
		return
	}

	for _, task := range tasks {
		if !task.Enabled {
			continue
		}

		slog.Info("cron: executing task", "id", task.ID)

		if ct.gateway != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			_, err := ct.gateway.HandleMessage(ctx, task.UserID, task.Platform, task.Instruction)
			cancel()

			if err != nil {
				slog.Warn("cron: task execution failed", "id", task.ID, "error", err)
			}
		}

		task.LastRunAt = time.Now().Format(time.RFC3339)
		ct.saveTask(task)
	}
}

func (ct *CronTrigger) loadDueTasks() ([]*CronTask, error) {
	entries, err := os.ReadDir(ct.cronDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cron dir: %w", err)
	}

	var tasks []*CronTask
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(ct.cronDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		task := &CronTask{}
		if err := json.Unmarshal(data, task); err != nil {
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (ct *CronTrigger) saveTask(task *CronTask) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("cron: marshal: %w", err)
	}

	path := filepath.Join(ct.cronDir, task.ID+".json")
	return os.WriteFile(path, data, 0644)
}

func (ct *CronTrigger) ListTasks() ([]*CronTask, error) {
	return ct.loadDueTasks()
}

func (ct *CronTrigger) DeleteTask(taskID string) error {
	path := filepath.Join(ct.cronDir, taskID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cron: delete: %w", err)
	}
	slog.Info("cron: deleted task", "id", taskID)
	return nil
}

func (ct *CronTrigger) DisableTask(taskID string) error {
	tasks, err := ct.loadDueTasks()
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task.ID == taskID {
			task.Enabled = false
			return ct.saveTask(task)
		}
	}

	return fmt.Errorf("cron: task %q not found", taskID)
}
