package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/hooks"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
)

type QueryEngine struct {
	client        *api.Client
	state         *QueryState
	config        QueryConfig
	mu            sync.RWMutex
	tools         *tools.ToolRegistry
	hookExecutor  *tools.HookAwareExecutor
	hookManager   *hooks.HookManager
	learningLoop  *learning.LearningLoop
	memoryManager *memory.MemoryManager
	dataStore     *store.Store
	sessionID     string
}

func NewQueryEngine(client *api.Client, config QueryConfig) *QueryEngine {
	registry := tools.GetRegistry()
	return &QueryEngine{
		client:       client,
		state:        NewQueryState(),
		config:       config,
		tools:        registry,
		hookExecutor: tools.NewHookAwareExecutor(registry, nil),
	}
}

// NewQueryEngineWithHooks creates a QueryEngine with hook support
func NewQueryEngineWithHooks(client *api.Client, config QueryConfig, hookManager *hooks.HookManager) *QueryEngine {
	registry := tools.GetRegistry()
	return &QueryEngine{
		client:       client,
		state:        NewQueryState(),
		config:       config,
		tools:        registry,
		hookExecutor: tools.NewHookAwareExecutor(registry, hookManager),
		hookManager:  hookManager,
	}
}

// SetHookManager updates the hook manager for this engine
func (e *QueryEngine) SetHookManager(hookManager *hooks.HookManager) {
	e.hookManager = hookManager
	e.hookExecutor.SetHookManager(hookManager)
}

// GetHookManager returns the current hook manager
func (e *QueryEngine) GetHookManager() *hooks.HookManager {
	return e.hookManager
}

func (e *QueryEngine) SetLearningLoop(loop *learning.LearningLoop) {
	e.learningLoop = loop
}

func (e *QueryEngine) SetMemoryManager(mm *memory.MemoryManager) {
	e.memoryManager = mm
	if mm != nil {
		e.dataStore = mm.GetStore()
	}
}

func (e *QueryEngine) SetStore(s *store.Store) {
	e.dataStore = s
}

func (e *QueryEngine) SetSessionID(id string) {
	e.sessionID = id
}

func (e *QueryEngine) Query(ctx context.Context, input string) (*QueryResult, error) {
	e.state.IncrementTurn()

	if e.memoryManager != nil && e.config.SystemPrompt == "" {
		memCtx := e.memoryManager.BuildSystemContext(ctx, input)
		if memCtx != "" {
			e.config.SystemPrompt = memCtx
		}
	}

	userMsg := Message{
		Role:      "user",
		Content:   input,
		Timestamp: time.Now(),
	}
	e.state.AddMessage(userMsg)

	e.persistMessage("user", input)

	messages := e.prepareMessages()

	startTime := time.Now()

	resp, err := e.client.CreateMessage(messages, e.config.SystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	duration := time.Since(startTime)

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content = block.Text
			break
		}
	}

	assistantMsg := Message{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	}
	e.state.AddMessage(assistantMsg)

	e.persistMessage("assistant", content)

	apiUsage := resp.Usage
	e.state.UpdateUsage(apiUsage)

	result := &QueryResult{
		Message:    assistantMsg,
		Usage:      apiUsage,
		StopReason: StopReason(resp.StopReason),
		Duration:   duration,
		Cost:       CalculateCost(apiUsage),
	}

	e.triggerLearningIfNeeded(ctx, result)
	e.trackUserObservations(input)

	return result, nil
}

func (e *QueryEngine) QueryStream(ctx context.Context, input string, handler StreamHandler) error {
	return fmt.Errorf("StreamMessage not implemented")
}

func (e *QueryEngine) ExecuteTool(ctx context.Context, toolName string, input map[string]interface{}) (interface{}, error) {
	if e.hookManager != nil && e.state.GetTurnCount() == 1 {
		e.hookManager.ExecuteSessionStart(ctx)
	}

	result, err := e.hookExecutor.ExecuteWithHooks(ctx, toolName, input)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	toolResultMsg := Message{
		Role:      "tool",
		Content:   result,
		Timestamp: time.Now(),
	}
	e.state.AddMessage(toolResultMsg)

	return result, nil
}

// ExecuteToolWithoutHooks bypasses hook system for internal operations
func (e *QueryEngine) ExecuteToolWithoutHooks(ctx context.Context, toolName string, input map[string]interface{}) (interface{}, error) {
	result, err := e.tools.Execute(ctx, toolName, input)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	toolResultMsg := Message{
		Role:      "tool",
		Content:   result,
		Timestamp: time.Now(),
	}
	e.state.AddMessage(toolResultMsg)

	return result, nil
}

func (e *QueryEngine) prepareMessages() []api.Message {
	msgs := e.state.GetMessages()
	result := make([]api.Message, len(msgs))
	for i, m := range msgs {
		result[i] = api.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}

func (e *QueryEngine) prepareToolSpecs() []api.ToolDefinition {
	toolList := e.tools.All()
	specs := make([]api.ToolDefinition, 0, len(toolList))

	for _, t := range toolList {
		specs = append(specs, api.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}

	return specs
}

func (e *QueryEngine) GetState() *QueryState {
	return e.state
}

func (e *QueryEngine) SetSystemPrompt(prompt string) {
	e.config.SystemPrompt = prompt
}

func (e *QueryEngine) AddTool(tool tools.Tool) {
	e.tools.Register(tool)
}

func CalculateCost(usage api.Usage) float64 {
	inputPrice := 0.000015
	outputPrice := 0.000075

	cost := float64(usage.InputTokens)*inputPrice + float64(usage.OutputTokens)*outputPrice
	return cost
}

func (e *QueryEngine) CompactIfNeeded() {
	if e.config.MaxTokens > 0 && e.state.GetUsage().InputTokens > e.config.MaxTokens {
		if e.hookManager != nil {
			e.hookManager.ExecutePreCompact(context.Background())
		}

		turnCount := e.state.GetTurnCount()
		if e.learningLoop != nil {
			if nudge := e.learningLoop.MaybeNudge(turnCount); nudge != nil {
				e.state.AddMessage(Message{
					Role:    "system",
					Content: nudge.Content,
				})
				slog.Info("learning loop: nudge injected before compaction", "turn", turnCount)
			}
		}

		e.state.AddMessage(Message{
			Role:    "system",
			Content: "Context compacted",
		})

		if e.hookManager != nil {
			e.hookManager.ExecutePostCompact(context.Background())
		}
	}
}

// Close cleans up the engine and fires session end hooks
func (e *QueryEngine) Close() error {
	if e.hookManager != nil {
		e.hookManager.ExecuteSessionEnd(context.Background())
	}
	return nil
}

// ExecuteUserPromptHook fires the UserPromptSubmit hook before processing user input
func (e *QueryEngine) ExecuteUserPromptHook(ctx context.Context, message string) []hooks.HookResult {
	if e.hookManager == nil {
		return nil
	}
	return e.hookManager.ExecuteUserPromptSubmit(ctx, message)
}

// ExecuteStopHook fires the Stop hook when the session stops
func (e *QueryEngine) ExecuteStopHook(ctx context.Context, message string) []hooks.HookResult {
	if e.hookManager == nil {
		return nil
	}
	return e.hookManager.ExecuteStop(ctx, message)
}

func (e *QueryEngine) triggerLearningIfNeeded(ctx context.Context, result *QueryResult) {
	if e.learningLoop == nil || !e.learningLoop.IsEnabled() {
		return
	}

	if string(result.StopReason) != "end_turn" && string(result.StopReason) != "stop_sequence" {
		return
	}

	messages := e.state.GetMessages()
	learningMessages := convertToLearningMessages(messages)

	turnCount := e.state.GetTurnCount()
	if nudge := e.learningLoop.MaybeNudge(turnCount); nudge != nil {
		slog.Info("learning loop: nudge triggered", "turn", turnCount)

		e.state.AddMessage(Message{
			Role:    "system",
			Content: nudge.Content,
		})

		go func() {
			allMessages := e.state.GetMessages()
			learningMessages := convertToLearningMessages(allMessages)
			if err := e.learningLoop.OnNudge(ctx, "auto", learningMessages); err != nil {
				slog.Error("learning loop: OnNudge failed", "error", err)
			}
		}()
	}

	go func() {
		learningResult := &learning.TaskResult{
			StopReason: string(result.StopReason),
			Duration:   result.Duration,
			Cost:       result.Cost,
			TokensUsed: result.Usage.InputTokens + result.Usage.OutputTokens,
		}
		if err := e.learningLoop.OnTaskComplete(ctx, "auto", learningMessages, learningResult); err != nil {
			slog.Error("learning loop: OnTaskComplete failed", "error", err)
		}
	}()
}

func convertToLearningMessages(messages []Message) []learning.Message {
	result := make([]learning.Message, 0, len(messages))
	for _, m := range messages {
		content, ok := m.Content.(string)
		if !ok {
			continue
		}
		result = append(result, learning.Message{
			Role:      m.Role,
			Content:   content,
			Timestamp: m.Timestamp,
		})
	}
	return result
}

func (e *QueryEngine) persistMessage(role, content string) {
	if e.dataStore == nil || e.sessionID == "" {
		return
	}

	msg := &store.Message{
		SessionID: e.sessionID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	if _, err := e.dataStore.InsertMessage(msg); err != nil {
		slog.Warn("engine: failed to persist message", "error", err)
	}
}

func (e *QueryEngine) trackUserObservations(input string) {
	if e.memoryManager == nil {
		return
	}

	userModel := e.memoryManager.GetUserModel()
	if userModel == nil {
		return
	}

	observations := layers.ExtractObservations("user", input)
	if len(observations) > 0 {
		if err := userModel.TrackPassive(context.Background(), observations); err != nil {
			slog.Warn("engine: failed to track user observations", "error", err)
		}
	}
}
