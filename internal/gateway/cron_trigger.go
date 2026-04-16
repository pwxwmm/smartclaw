package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type CronConfig struct {
	Timeout      time.Duration
	JSONLDir     string
	TickInterval time.Duration
}

func defaultCronConfig() CronConfig {
	return CronConfig{
		Timeout:      5 * time.Minute,
		TickInterval: 1 * time.Minute,
	}
}

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
	store     *store.Store
	gateway   *Gateway
	cronDir   string
	lockDir   string
	stopCh    chan struct{}
	running   bool
	mu        sync.Mutex
	lockFiles map[string]*os.File
	config    CronConfig
}

func NewCronTrigger(s *store.Store, gw *Gateway) *CronTrigger {
	return NewCronTriggerWithConfig(s, gw, defaultCronConfig())
}

func NewCronTriggerWithConfig(s *store.Store, gw *Gateway, cfg CronConfig) *CronTrigger {
	home, _ := os.UserHomeDir()
	cronDir := filepath.Join(home, ".smartclaw", "cron")

	if s == nil {
		slog.Warn("cron: SQLite unavailable, cron tasks use JSON file persistence only")
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultCronConfig().Timeout
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = defaultCronConfig().TickInterval
	}

	return &CronTrigger{
		store:     s,
		gateway:   gw,
		cronDir:   cronDir,
		lockDir:   filepath.Join(home, ".smartclaw", "cron", ".locks"),
		stopCh:    make(chan struct{}),
		lockFiles: make(map[string]*os.File),
		config:    cfg,
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
	ticker := time.NewTicker(ct.config.TickInterval)
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
		if !ct.acquireLock(task.ID) {
			slog.Info("cron: skipping task, another instance is running", "id", task.ID)
			continue
		}

		slog.Info("cron: executing task", "id", task.ID)

		if ct.gateway != nil {
			sessionID := fmt.Sprintf("cron_%s_%s", task.ID, time.Now().Format("20060102_150405"))
			ctx, cancel := context.WithTimeout(context.Background(), ct.config.Timeout)
			resp, err := ct.gateway.HandleMessageWithSession(ctx, task.UserID, task.Platform, task.Instruction, sessionID)
			cancel()

			if err != nil {
				slog.Warn("cron: task execution failed", "id", task.ID, "error", err)
			} else if resp != nil {
				ct.deliverResult(task, resp)
			}
		}

		task.LastRunAt = time.Now().Format(time.RFC3339)
		ct.saveTask(task)
		ct.releaseLock(task.ID)
	}
}

func (ct *CronTrigger) acquireLock(taskID string) bool {
	os.MkdirAll(ct.lockDir, 0755)

	lockPath := filepath.Join(ct.lockDir, taskID+".lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		slog.Warn("cron: failed to open lock file", "id", taskID, "error", err)
		return false
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return false
	}

	ct.mu.Lock()
	ct.lockFiles[taskID] = f
	ct.mu.Unlock()

	return true
}

func (ct *CronTrigger) releaseLock(taskID string) {
	ct.mu.Lock()
	f, ok := ct.lockFiles[taskID]
	if ok {
		delete(ct.lockFiles, taskID)
	}
	ct.mu.Unlock()

	if !ok || f == nil {
		return
	}

	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}

func (ct *CronTrigger) deliverResult(task *CronTask, resp *GatewayResponse) {
	if ct.gateway == nil {
		return
	}
	ct.gateway.GetDelivery().Deliver(task.UserID, task.Platform, resp)
}

func (ct *CronTrigger) loadDueTasks() ([]*CronTask, error) {
	entries, err := os.ReadDir(ct.cronDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cron dir: %w", err)
	}

	now := time.Now()
	var due []*CronTask
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

		if !task.Enabled {
			continue
		}

		if isScheduleDue(task.Schedule, now, task.LastRunAt) {
			due = append(due, task)
		}
	}

	return due, nil
}

func (ct *CronTrigger) loadAllTasks() ([]*CronTask, error) {
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
	return ct.loadAllTasks()
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
	tasks, err := ct.loadAllTasks()
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
