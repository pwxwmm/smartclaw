package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/hooks"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/routing"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
)

type QueryEngine struct {
	client              *api.Client
	cacheClient         *api.CacheAwareClient
	state               *QueryState
	config              QueryConfig
	mu                  sync.RWMutex
	tools               *tools.ToolRegistry
	hookExecutor        *tools.HookAwareExecutor
	hookManager         *hooks.HookManager
	learningLoop        *learning.LearningLoop
	memoryManager       *memory.MemoryManager
	dataStore           *store.Store
	sessionID           string
	llmCompactor        *LLMCompactor
	router              *routing.ModelRouter
	proactive           *learning.ProactiveEngine
	speculativeExecutor *routing.SpeculativeExecutor
	costGuard           *costguard.CostGuard
	thinkingManager     *ThinkingManager
	prefetcher          *tools.PredictivePrefetcher
	shutdownCh          chan struct{}
	shutdownOnce        sync.Once
}

func NewQueryEngine(client *api.Client, config QueryConfig) *QueryEngine {
	registry := tools.GetRegistry()
	e := &QueryEngine{
		client:          client,
		cacheClient:     api.NewCacheAwareClient(client),
		state:           NewQueryState(),
		config:          config,
		tools:           registry,
		hookExecutor:    tools.NewHookAwareExecutor(registry, nil),
		thinkingManager: NewThinkingManager(),
		shutdownCh:      make(chan struct{}),
	}

	if client != nil && config.EnableLLMCompaction {
		e.llmCompactor = NewLLMCompactor(e.createCompactionFunc())
	}

	return e
}

// NewQueryEngineWithHooks creates a QueryEngine with hook support
func NewQueryEngineWithHooks(client *api.Client, config QueryConfig, hookManager *hooks.HookManager) *QueryEngine {
	registry := tools.GetRegistry()
	return &QueryEngine{
		client:          client,
		cacheClient:     api.NewCacheAwareClient(client),
		state:           NewQueryState(),
		config:          config,
		tools:           registry,
		hookExecutor:    tools.NewHookAwareExecutor(registry, hookManager),
		hookManager:     hookManager,
		thinkingManager: NewThinkingManager(),
		shutdownCh:      make(chan struct{}),
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

func (e *QueryEngine) SetRouter(r *routing.ModelRouter) {
	e.router = r
}

func (e *QueryEngine) GetRouter() *routing.ModelRouter {
	return e.router
}

func (e *QueryEngine) SetProactiveEngine(pe *learning.ProactiveEngine) {
	e.proactive = pe
}

func (e *QueryEngine) GetProactiveEngine() *learning.ProactiveEngine {
	return e.proactive
}

func (e *QueryEngine) SetSpeculativeExecutor(se *routing.SpeculativeExecutor) {
	e.speculativeExecutor = se
}

func (e *QueryEngine) GetSpeculativeExecutor() *routing.SpeculativeExecutor {
	return e.speculativeExecutor
}

func (e *QueryEngine) SetCostGuard(cg *costguard.CostGuard) {
	e.costGuard = cg
}

func (e *QueryEngine) GetCostGuard() *costguard.CostGuard {
	return e.costGuard
}

func (e *QueryEngine) SetThinkingManager(tm *ThinkingManager) {
	e.thinkingManager = tm
}

func (e *QueryEngine) GetThinkingManager() *ThinkingManager {
	return e.thinkingManager
}

func (e *QueryEngine) SetPrefetcher(pp *tools.PredictivePrefetcher) {
	e.prefetcher = pp
}

func (e *QueryEngine) GetPrefetcher() *tools.PredictivePrefetcher {
	return e.prefetcher
}

func (e *QueryEngine) EnableConversationTree() {
	e.state.EnableTree()
}

func (e *QueryEngine) Branch(fromTurnID string) (string, error) {
	if !e.state.IsTreeEnabled() {
		return "", fmt.Errorf("conversation tree not enabled")
	}
	return e.state.tree.Branch(fromTurnID)
}

func (e *QueryEngine) Checkout(branchOrNodeID string) error {
	if !e.state.IsTreeEnabled() {
		return fmt.Errorf("conversation tree not enabled")
	}
	return e.state.tree.Checkout(branchOrNodeID)
}

func (e *QueryEngine) GetBranches() map[string]string {
	if !e.state.IsTreeEnabled() {
		return nil
	}
	return e.state.tree.GetBranches()
}

func (e *QueryEngine) GetConversationHeadID() string {
	if !e.state.IsTreeEnabled() {
		return ""
	}
	return e.state.tree.GetHeadID()
}

func (e *QueryEngine) Query(ctx context.Context, input string) (*QueryResult, error) {
	ctx, querySpan := observability.StartSpan(ctx, "query", observability.Attr("input_length", len(input)))
	defer observability.EndSpan(querySpan)

	e.state.IncrementTurn()

	if e.memoryManager != nil && e.config.SystemPrompt == "" {
		_, memSpan := observability.StartSpan(ctx, "memory.build")
		memCtx := e.memoryManager.BuildSystemContext(ctx, input)
		observability.EndSpan(memSpan)

		if memCtx != "" {
			e.config.SystemPrompt = memCtx
		}
	}

	// Invalidate cache when system prompt changes
	if e.cacheClient != nil {
		prevSystem := e.state.GetLastSystemPrompt()
		if prevSystem != e.config.SystemPrompt {
			e.cacheClient.OnMemoryChanged()
			e.state.SetLastSystemPrompt(e.config.SystemPrompt)
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

	if e.router != nil && e.router.IsEnabled() {
		_, routeSpan := observability.StartSpan(ctx, "routing.select")
		signal := routing.ComplexitySignal{
			MessageLength:    len(input),
			ToolCallCount:    0,
			HistoryTurnCount: e.state.GetTurnCount(),
			HasCodeContent:   containsCode(input),
			SkillMatched:     false,
		}
		selectedModel := e.router.Route(input, signal)
		observability.EndSpan(routeSpan)

		if selectedModel != "" && selectedModel != e.client.Model {
			e.client.SetModel(selectedModel)
			slog.Info("routing: model selected", "model", selectedModel)
		}

		if e.speculativeExecutor != nil && e.speculativeExecutor.IsEnabled() && routing.ShouldSpeculate(input, e.assessComplexity(input, signal)) {
			_, specSpan := observability.StartSpan(ctx, "speculative.execute")
			specResult, specErr := e.speculativeExecutor.Execute(ctx, messages, e.config.SystemPrompt)
			observability.EndSpan(specSpan)

			if specErr == nil && specResult != nil {
				var chosenResp *api.MessageResponse
				if specResult.UsedModel == "fast" {
					chosenResp = specResult.FastResult
				} else {
					chosenResp = specResult.SlowResult
				}

				if chosenResp != nil {
					slog.Info("speculative: result chosen", "model", specResult.UsedModel, "similarity", fmt.Sprintf("%.2f", specResult.Similarity))
					duration := specResult.FastDuration
					if specResult.UsedModel == "slow" {
						duration = specResult.SlowDuration
					}

					observability.RecordQueryDuration(duration, specResult.UsedModel)
					observability.RecordTokenUsage(chosenResp.Usage.InputTokens, chosenResp.Usage.OutputTokens, chosenResp.Usage.CacheRead, chosenResp.Usage.CacheCreation, specResult.UsedModel)

					if e.router != nil {
						success := string(StopReason(chosenResp.StopReason)) == "end_turn" || string(StopReason(chosenResp.StopReason)) == "stop_sequence"
						e.router.RecordOutcome(specResult.UsedModel, 0, success, false, duration)
					}

					return e.buildResultFromResponse(chosenResp, duration), nil
				}
			}
		}
	}

	startTime := time.Now()

	systemPrompt := e.config.SystemPrompt
	if e.prefetcher != nil && e.prefetcher.IsEnabled() {
		lastUserMsg := extractLastUserMessage(messages)
		if lastUserMsg != "" {
			prefetchResult := e.prefetcher.Prefetch(lastUserMsg, 3)
			if len(prefetchResult.Files) > 0 {
				var prefetchCtx strings.Builder
				prefetchCtx.WriteString("\n\n## Prefetched Context\n")
				for f, content := range prefetchResult.Files {
					prefetchCtx.WriteString(fmt.Sprintf("\n### %s\n```\n%s\n```\n", f, content))
				}
				systemPrompt += prefetchCtx.String()
			}
		}
	}

	if e.thinkingManager != nil && e.thinkingManager.IsEnabled() {
		e.client.Thinking = &api.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: e.thinkingManager.GetBudget(),
		}
	} else {
		e.client.Thinking = nil
	}

	_, apiSpan := observability.StartSpan(ctx, "api.call")
	var resp *api.MessageResponse
	var err error
	if e.cacheClient != nil {
		resp, err = e.cacheClient.CreateMessage(ctx, messages, systemPrompt)
	} else {
		resp, err = e.client.CreateMessage(messages, systemPrompt)
	}
	observability.EndSpan(apiSpan)

	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	duration := time.Since(startTime)

	observability.RecordQueryDuration(duration, e.client.Model)
	observability.RecordTokenUsage(resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheRead, resp.Usage.CacheCreation, e.client.Model)

	if e.costGuard != nil && e.costGuard.IsEnabled() {
		snapshot := e.costGuard.RecordUsage(e.client.Model, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheRead, resp.Usage.CacheCreation)
		if snapshot.ShouldDowngrade && snapshot.DowngradeModel != "" {
			slog.Warn("cost guard: downgrading model", "from", e.client.Model, "to", snapshot.DowngradeModel, "budget_used", fmt.Sprintf("%.0f%%", snapshot.BudgetFraction*100))
			e.client.SetModel(snapshot.DowngradeModel)
		}
	}

	if e.cacheClient != nil {
		observability.RecordCacheHit(resp.Usage.CacheRead > 0)
	}

	if e.router != nil && e.router.IsEnabled() {
		success := string(StopReason(resp.StopReason)) == "end_turn" || string(StopReason(resp.StopReason)) == "stop_sequence"
		e.router.RecordOutcome(e.client.Model, 0, success, false, duration)
	}

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

	if querySpan != nil {
		querySpan.SetAttribute("model", e.client.Model)
		querySpan.SetAttribute("duration_ms", duration.Milliseconds())
		querySpan.SetAttribute("input_tokens", apiUsage.InputTokens)
		querySpan.SetAttribute("output_tokens", apiUsage.OutputTokens)
	}

	_, learnSpan := observability.StartSpan(ctx, "learning.trigger")
	e.triggerLearningIfNeeded(ctx, result)
	observability.EndSpan(learnSpan)

	e.trackUserObservations(ctx, input)

	if e.proactive != nil {
		lastAction := learning.Action{
			Type:    "query",
			Content: input,
		}
		e.proactive.RecordAction(lastAction)
		if suggestion := e.proactive.MaybeSuggest(lastAction); suggestion != nil {
			slog.Info("proactive: suggestion available", "text", suggestion.Text, "confidence", fmt.Sprintf("%.2f", suggestion.Confidence))
		}
	}

	return result, nil
}

func (e *QueryEngine) QueryStream(ctx context.Context, input string, handler StreamHandler) error {
	ctx, streamSpan := observability.StartSpan(ctx, "query_stream", observability.Attr("input_length", len(input)))
	defer observability.EndSpan(streamSpan)

	e.state.IncrementTurn()

	if e.memoryManager != nil && e.config.SystemPrompt == "" {
		memCtx := e.memoryManager.BuildSystemContext(ctx, input)
		if memCtx != "" {
			e.config.SystemPrompt = memCtx
		}
	}

	if e.cacheClient != nil {
		prevSystem := e.state.GetLastSystemPrompt()
		if prevSystem != e.config.SystemPrompt {
			e.cacheClient.OnMemoryChanged()
			e.state.SetLastSystemPrompt(e.config.SystemPrompt)
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

	if e.router != nil && e.router.IsEnabled() {
		signal := routing.ComplexitySignal{
			MessageLength:    len(input),
			ToolCallCount:    0,
			HistoryTurnCount: e.state.GetTurnCount(),
			HasCodeContent:   containsCode(input),
			SkillMatched:     false,
		}
		selectedModel := e.router.Route(input, signal)
		if selectedModel != "" && selectedModel != e.client.Model {
			e.client.SetModel(selectedModel)
		}
	}

	startTime := time.Now()

	req := &api.MessageRequest{
		Model:     e.client.Model,
		MaxTokens: 4096,
		Messages:  messages,
		System:    e.config.SystemPrompt,
		Stream:    true,
	}

	if e.thinkingManager != nil && e.thinkingManager.IsEnabled() {
		req.Thinking = &api.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: e.thinkingManager.GetBudget(),
		}
		req.MaxTokens = max(req.MaxTokens, req.Thinking.BudgetTokens+4096)
	}

	if req.System != nil {
		if sysStr, ok := req.System.(string); ok && sysStr != "" {
			req.System = []api.SystemBlock{
				{
					Type: "text",
					Text: sysStr,
					CacheControl: &api.CacheControl{
						Type: "ephemeral",
					},
				},
			}
		}
	}

	parser := api.NewStreamMessageParser()

	if handler != nil {
		handler(QueryEvent{Type: "start", Timestamp: time.Now()})
	}

	err := e.client.StreamMessageSSE(ctx, req, func(event string, data []byte) error {
		result, parseErr := parser.HandleEvent(event, data)
		if parseErr != nil {
			return parseErr
		}

		if handler != nil {
			switch {
			case result.TextDelta != "":
				handler(QueryEvent{
					Type:      "text_delta",
					Data:      result.TextDelta,
					Timestamp: time.Now(),
				})
			case result.ThinkingDelta != "":
				handler(QueryEvent{
					Type:      "thinking_delta",
					Data:      result.ThinkingDelta,
					Timestamp: time.Now(),
				})
			case result.MessageStop:
				handler(QueryEvent{
					Type:      "done",
					Timestamp: time.Now(),
				})
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("stream failed: %w", err)
	}

	duration := time.Since(startTime)

	msg := parser.GetMessage()
	if msg != nil {
		var content string
		for _, block := range msg.Content {
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
		e.state.UpdateUsage(msg.Usage)

		observability.RecordQueryDuration(duration, e.client.Model)
		observability.RecordTokenUsage(msg.Usage.InputTokens, msg.Usage.OutputTokens, msg.Usage.CacheRead, msg.Usage.CacheCreation, e.client.Model)

		result := &QueryResult{
			Message:    assistantMsg,
			Usage:      msg.Usage,
			StopReason: StopReason(msg.StopReason),
			Duration:   duration,
			Cost:       CalculateCost(msg.Usage),
		}

		e.triggerLearningIfNeeded(ctx, result)
		e.trackUserObservations(ctx, input)
	}

	return nil
}

func (e *QueryEngine) ExecuteTool(ctx context.Context, toolName string, input map[string]any) (any, error) {
	if e.hookManager != nil && e.state.GetTurnCount() == 1 {
		e.hookManager.ExecuteSessionStart(ctx)
	}

	startTime := time.Now()
	result, err := e.hookExecutor.ExecuteWithHooks(ctx, toolName, input)
	duration := time.Since(startTime)

	observability.RecordToolExecution(toolName, duration, err == nil)

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
func (e *QueryEngine) ExecuteToolWithoutHooks(ctx context.Context, toolName string, input map[string]any) (any, error) {
	startTime := time.Now()
	result, err := e.tools.Execute(ctx, toolName, input)
	duration := time.Since(startTime)

	observability.RecordToolExecution(toolName, duration, err == nil)

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

func (e *QueryEngine) GetCacheClient() *api.CacheAwareClient {
	return e.cacheClient
}

func (e *QueryEngine) SetLLMCompactor(compactor *LLMCompactor) {
	e.llmCompactor = compactor
}

func (e *QueryEngine) createCompactionFunc() func(systemPrompt, userPrompt string) (string, error) {
	client := e.client
	return func(systemPrompt, userPrompt string) (string, error) {
		messages := []api.Message{
			{Role: "user", Content: userPrompt},
		}
		resp, err := client.CreateMessage(messages, systemPrompt)
		if err != nil {
			return "", err
		}
		return extractResponseText(resp), nil
	}
}

func extractResponseText(resp *api.MessageResponse) string {
	if resp == nil {
		return ""
	}
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text
		}
	}
	return ""
}

func CalculateCost(usage api.Usage) float64 {
	inputPrice := 0.000015
	outputPrice := 0.000075

	cost := float64(usage.InputTokens)*inputPrice + float64(usage.OutputTokens)*outputPrice
	return cost
}

func (e *QueryEngine) CompactIfNeeded() {
	if e.config.MaxTokens <= 0 || e.state.GetUsage().InputTokens <= e.config.MaxTokens {
		return
	}

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

	var compacted []Message
	if e.llmCompactor != nil {
		compacted = e.llmCompactor.CompactWithLLM(e.state.GetMessages(), e.config.MaxTokens)
	} else {
		compacted = Compact(e.state.GetMessages(), e.config.MaxTokens)
	}

	e.state.ReplaceMessages(compacted)
	slog.Info("engine: context compacted", "turn", turnCount, "messages_after", len(compacted))

	if e.hookManager != nil {
		e.hookManager.ExecutePostCompact(context.Background())
	}
}

// Shutdown signals all background goroutines to stop.
// Safe to call multiple times.
func (e *QueryEngine) Shutdown() {
	e.shutdownOnce.Do(func() {
		close(e.shutdownCh)
	})
}

// Close cleans up the engine and fires session end hooks
func (e *QueryEngine) Close() error {
	if e.hookManager != nil {
		e.hookManager.ExecuteSessionEnd(context.Background())
	}
	e.Shutdown()
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
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			select {
			case <-e.shutdownCh:
				return
			default:
			}

			allMessages := e.state.GetMessages()
			learningMessages := convertToLearningMessages(allMessages)
			if err := e.learningLoop.OnNudge(bgCtx, "auto", learningMessages); err != nil {
				slog.Error("learning loop: OnNudge failed", "error", err)
			}
		}()

		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			select {
			case <-e.shutdownCh:
				return
			default:
			}

			learningResult := &learning.TaskResult{
				StopReason: string(result.StopReason),
				Duration:   result.Duration,
				Cost:       result.Cost,
				TokensUsed: result.Usage.InputTokens + result.Usage.OutputTokens,
			}
			if err := e.learningLoop.OnTaskComplete(bgCtx, "auto", learningMessages, learningResult); err != nil {
				slog.Error("learning loop: OnTaskComplete failed", "error", err)
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

	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}
		if _, err := e.dataStore.InsertMessage(msg); err != nil {
			lastErr = err
			slog.Warn("engine: failed to persist message", "attempt", attempt+1, "error", err)
			continue
		}
		return
	}
	slog.Error("engine: persist message failed after retries", "role", role, "error", lastErr)
}

func (e *QueryEngine) trackUserObservations(ctx context.Context, input string) {
	if e.memoryManager == nil {
		return
	}

	userModel := e.memoryManager.GetUserModel()
	if userModel == nil {
		return
	}

	observations := layers.ExtractObservations("user", input)
	if len(observations) > 0 {
		if err := userModel.TrackPassive(ctx, observations); err != nil {
			slog.Warn("engine: failed to track user observations", "error", err)
		}
	}
}

func (e *QueryEngine) assessComplexity(input string, signal routing.ComplexitySignal) float64 {
	if e.router == nil {
		return 0
	}
	return e.router.AssessComplexity(input, signal)
}

func (e *QueryEngine) buildResultFromResponse(resp *api.MessageResponse, duration time.Duration) *QueryResult {
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
	e.state.UpdateUsage(resp.Usage)

	result := &QueryResult{
		Message:    assistantMsg,
		Usage:      resp.Usage,
		StopReason: StopReason(resp.StopReason),
		Duration:   duration,
		Cost:       CalculateCost(resp.Usage),
	}

	e.triggerLearningIfNeeded(context.Background(), result)
	e.trackUserObservations(context.Background(), content)

	return result
}

func containsCode(s string) bool {
	codeIndicators := []string{"func ", "func(", "import ", "package ", "class ", "def ", "fn ", "pub fn", "{", "};", "=>", "->", "==", "!=", "return "}
	lower := strings.ToLower(s)
	for _, indicator := range codeIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func extractLastUserMessage(messages []api.MessageParam) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			if str, ok := messages[i].Content.(string); ok {
				return str
			}
		}
	}
	return ""
}
