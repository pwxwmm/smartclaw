package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SubagentStatus string

const (
	SubagentStatusPending   SubagentStatus = "pending"
	SubagentStatusRunning   SubagentStatus = "running"
	SubagentStatusCompleted SubagentStatus = "completed"
	SubagentStatusFailed    SubagentStatus = "failed"
	SubagentStatusCancelled SubagentStatus = "cancelled"
)

type SubagentTask struct {
	ID           string         `json:"id"`
	ParentID     string         `json:"parentId"`
	AgentType    string         `json:"agentType"`
	Prompt       string         `json:"prompt"`
	Status       SubagentStatus `json:"status"`
	Result       string         `json:"result,omitempty"`
	Error        string         `json:"error,omitempty"`
	StartTime    time.Time      `json:"startTime"`
	EndTime      time.Time      `json:"endTime,omitempty"`
	WorktreePath string         `json:"worktreePath,omitempty"`
	IsBackground bool           `json:"isBackground"`
	Progress     float64        `json:"progress"`
	Messages     []SubagentMsg  `json:"messages,omitempty"`
}

type SubagentMsg struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type SubagentManager struct {
	tasks       map[string]*SubagentTask
	mu          sync.RWMutex
	workDir     string
	agentMgr    *AgentManager
	maxParallel int
}

func NewSubagentManager(workDir string, agentMgr *AgentManager) *SubagentManager {
	return &SubagentManager{
		tasks:       make(map[string]*SubagentTask),
		workDir:     workDir,
		agentMgr:    agentMgr,
		maxParallel: 5,
	}
}

func (sm *SubagentManager) SpawnSubagent(ctx context.Context, agentType, prompt string, opts ...SpawnOption) (*SubagentTask, error) {
	task := &SubagentTask{
		ID:           generateTaskID(),
		AgentType:    agentType,
		Prompt:       prompt,
		Status:       SubagentStatusPending,
		StartTime:    time.Now(),
		IsBackground: false,
		Progress:     0,
		Messages:     make([]SubagentMsg, 0),
	}

	for _, opt := range opts {
		opt(task)
	}

	sm.mu.Lock()
	sm.tasks[task.ID] = task
	sm.mu.Unlock()

	go sm.executeTask(ctx, task)

	return task, nil
}

type SpawnOption func(*SubagentTask)

func WithBackground(isBackground bool) SpawnOption {
	return func(t *SubagentTask) {
		t.IsBackground = isBackground
	}
}

func WithParentID(parentID string) SpawnOption {
	return func(t *SubagentTask) {
		t.ParentID = parentID
	}
}

func WithWorktree(worktreePath string) SpawnOption {
	return func(t *SubagentTask) {
		t.WorktreePath = worktreePath
	}
}

func (sm *SubagentManager) executeTask(ctx context.Context, task *SubagentTask) {
	sm.updateTaskStatus(task.ID, SubagentStatusRunning)

	agent, err := sm.agentMgr.GetAgent(task.AgentType)
	if err != nil {
		sm.failTask(task.ID, fmt.Sprintf("Agent not found: %s", task.AgentType))
		return
	}

	workDir := sm.workDir
	if task.WorktreePath != "" {
		workDir = task.WorktreePath
	}

	task.Messages = append(task.Messages, SubagentMsg{
		Role:      "user",
		Content:   task.Prompt,
		Timestamp: time.Now(),
	})

	result := sm.simulateAgentExecution(ctx, agent, task.Prompt, workDir)

	if ctx.Err() != nil {
		sm.updateTaskStatus(task.ID, SubagentStatusCancelled)
		return
	}

	task.Messages = append(task.Messages, SubagentMsg{
		Role:      "assistant",
		Content:   result,
		Timestamp: time.Now(),
	})

	task.Result = result
	task.EndTime = time.Now()
	task.Progress = 1.0
	sm.updateTaskStatus(task.ID, SubagentStatusCompleted)
}

func (sm *SubagentManager) simulateAgentExecution(ctx context.Context, agent *AgentDefinition, prompt, workDir string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent: %s\n", agent.AgentType))
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", workDir))
	sb.WriteString(fmt.Sprintf("Prompt: %s\n\n", prompt))

	if len(agent.Tools) > 0 {
		sb.WriteString("Available tools:\n")
		for _, tool := range agent.Tools {
			sb.WriteString(fmt.Sprintf("  - %s\n", tool))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Execution result:\n")
	sb.WriteString(fmt.Sprintf("Task completed by %s agent.\n", agent.AgentType))

	return sb.String()
}

func (sm *SubagentManager) updateTaskStatus(taskID string, status SubagentStatus) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if task, exists := sm.tasks[taskID]; exists {
		task.Status = status
	}
}

func (sm *SubagentManager) failTask(taskID, errMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if task, exists := sm.tasks[taskID]; exists {
		task.Status = SubagentStatusFailed
		task.Error = errMsg
		task.EndTime = time.Now()
	}
}

func (sm *SubagentManager) GetTask(taskID string) (*SubagentTask, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	task, exists := sm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

func (sm *SubagentManager) ListTasks() []*SubagentTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var tasks []*SubagentTask
	for _, task := range sm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

func (sm *SubagentManager) ListTasksByStatus(status SubagentStatus) []*SubagentTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var tasks []*SubagentTask
	for _, task := range sm.tasks {
		if task.Status == status {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (sm *SubagentManager) CancelTask(taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, exists := sm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == SubagentStatusCompleted || task.Status == SubagentStatusFailed {
		return fmt.Errorf("cannot cancel task with status: %s", task.Status)
	}

	task.Status = SubagentStatusCancelled
	task.EndTime = time.Now()
	return nil
}

func (sm *SubagentManager) WaitForTask(ctx context.Context, taskID string) (*SubagentTask, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task, err := sm.GetTask(taskID)
			if err != nil {
				return nil, err
			}

			if task.Status == SubagentStatusCompleted ||
				task.Status == SubagentStatusFailed ||
				task.Status == SubagentStatusCancelled {
				return task, nil
			}
		}
	}
}

func (sm *SubagentManager) CreateWorktree(taskID string) (string, error) {
	task, err := sm.GetTask(taskID)
	if err != nil {
		return "", err
	}

	worktreeName := fmt.Sprintf("subagent-%s", taskID[:8])
	worktreePath := filepath.Join(sm.workDir, ".worktrees", worktreeName)

	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create worktree directory: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", worktreePath, "HEAD")
	cmd.Dir = sm.workDir
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(worktreePath)
		return "", fmt.Errorf("failed to create git worktree: %w, output: %s", err, string(output))
	}

	task.WorktreePath = worktreePath
	return worktreePath, nil
}

func (sm *SubagentManager) RemoveWorktree(taskID string) error {
	task, err := sm.GetTask(taskID)
	if err != nil {
		return err
	}

	if task.WorktreePath == "" {
		return nil
	}

	cmd := exec.Command("git", "worktree", "remove", "--force", task.WorktreePath)
	cmd.Dir = sm.workDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove git worktree: %w, output: %s", err, string(output))
	}

	task.WorktreePath = ""
	return nil
}

func (sm *SubagentManager) FormatTaskList() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("╭──────────────────────────────────────────────────────────╮\n")
	sb.WriteString("│                   🔄 子代理任务列表                        │\n")
	sb.WriteString("╰──────────────────────────────────────────────────────────╯\n\n")

	if len(sm.tasks) == 0 {
		sb.WriteString("  暂无子代理任务\n")
		return sb.String()
	}

	running := 0
	completed := 0
	failed := 0

	for _, task := range sm.tasks {
		switch task.Status {
		case SubagentStatusRunning, SubagentStatusPending:
			running++
		case SubagentStatusCompleted:
			completed++
		case SubagentStatusFailed:
			failed++
		}
	}

	sb.WriteString(fmt.Sprintf("  运行中: %d  已完成: %d  失败: %d\n\n", running, completed, failed))

	for _, task := range sm.tasks {
		statusIcon := getStatusIcon(task.Status)
		sb.WriteString(fmt.Sprintf("  %s [%s] %s\n", statusIcon, task.ID[:8], task.AgentType))
		sb.WriteString(fmt.Sprintf("      状态: %s  进度: %.0f%%\n", task.Status, task.Progress*100))
		if task.Result != "" {
			preview := task.Result
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			sb.WriteString(fmt.Sprintf("      结果: %s\n", preview))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("使用方法:\n")
	sb.WriteString("  /subagent spawn <agent-type> <prompt>  - 启动子代理\n")
	sb.WriteString("  /subagent status <task-id>             - 查看任务状态\n")
	sb.WriteString("  /subagent cancel <task-id>              - 取消任务\n")

	return sb.String()
}

func (sm *SubagentManager) FormatTaskInfo(task *SubagentTask) string {
	var sb strings.Builder

	sb.WriteString("╭─────────────────────────────────────────────────╮\n")
	sb.WriteString(fmt.Sprintf("│  子代理任务: %-35s │\n", task.ID[:8]))
	sb.WriteString("╰─────────────────────────────────────────────────╯\n\n")

	sb.WriteString(fmt.Sprintf("  Agent类型:     %s\n", task.AgentType))
	sb.WriteString(fmt.Sprintf("  状态:          %s\n", task.Status))
	sb.WriteString(fmt.Sprintf("  进度:          %.0f%%\n", task.Progress*100))
	sb.WriteString(fmt.Sprintf("  开始时间:      %s\n", task.StartTime.Format("2006-01-02 15:04:05")))

	if !task.EndTime.IsZero() {
		sb.WriteString(fmt.Sprintf("  结束时间:      %s\n", task.EndTime.Format("2006-01-02 15:04:05")))
		duration := task.EndTime.Sub(task.StartTime)
		sb.WriteString(fmt.Sprintf("  执行时长:      %v\n", duration.Round(time.Second)))
	}

	if task.WorktreePath != "" {
		sb.WriteString(fmt.Sprintf("  工作目录:      %s\n", task.WorktreePath))
	}

	sb.WriteString("\n  提示词:\n")
	sb.WriteString("  ─────────────────────────────────────────────\n")
	lines := strings.Split(task.Prompt, "\n")
	for _, line := range lines {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}

	if task.Result != "" {
		sb.WriteString("  ─────────────────────────────────────────────\n")
		sb.WriteString("\n  结果:\n")
		sb.WriteString("  ─────────────────────────────────────────────\n")
		resultLines := strings.Split(task.Result, "\n")
		for _, line := range resultLines {
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	if task.Error != "" {
		sb.WriteString("  ─────────────────────────────────────────────\n")
		sb.WriteString(fmt.Sprintf("\n  ❌ 错误: %s\n", task.Error))
	}

	return sb.String()
}

func getStatusIcon(status SubagentStatus) string {
	switch status {
	case SubagentStatusPending:
		return "⏳"
	case SubagentStatusRunning:
		return "🔄"
	case SubagentStatusCompleted:
		return "✅"
	case SubagentStatusFailed:
		return "❌"
	case SubagentStatusCancelled:
		return "🚫"
	default:
		return "❓"
	}
}

func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

func (sm *SubagentManager) ExportTask(taskID string) (string, error) {
	task, err := sm.GetTask(taskID)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (sm *SubagentManager) GetRunningCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, task := range sm.tasks {
		if task.Status == SubagentStatusRunning || task.Status == SubagentStatusPending {
			count++
		}
	}
	return count
}
