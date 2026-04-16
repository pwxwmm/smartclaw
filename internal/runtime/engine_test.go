package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/tools"
)

func newTestClient(serverURL string) *api.Client {
	return api.NewClientWithModel("test-key", serverURL, "claude-sonnet-4-5")
}

func TestNewQueryEngine(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	if engine == nil {
		t.Fatal("NewQueryEngine returned nil")
	}
	if engine.client == nil {
		t.Error("client should not be nil")
	}
	if engine.state == nil {
		t.Error("state should not be nil")
	}
	if engine.tools == nil {
		t.Error("tools should not be nil")
	}
	if engine.hookExecutor == nil {
		t.Error("hookExecutor should not be nil")
	}
	if engine.thinkingManager == nil {
		t.Error("thinkingManager should not be nil")
	}
	if engine.shutdownCh == nil {
		t.Error("shutdownCh should not be nil")
	}
	if engine.cacheClient == nil {
		t.Error("cacheClient should not be nil")
	}
}

func TestNewQueryEngineWithLLMCompaction(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{EnableLLMCompaction: true})

	if engine.llmCompactor == nil {
		t.Error("llmCompactor should not be nil when EnableLLMCompaction is true")
	}
}

func TestNewQueryEngineWithoutLLMCompaction(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{EnableLLMCompaction: false})

	if engine.llmCompactor != nil {
		t.Error("llmCompactor should be nil when EnableLLMCompaction is false")
	}
}

func TestNewQueryEngineNilClient(t *testing.T) {
	engine := NewQueryEngine(nil, QueryConfig{EnableLLMCompaction: true})

	if engine.llmCompactor != nil {
		t.Error("llmCompactor should be nil when client is nil")
	}
}

func TestQueryWithMockedAPI(t *testing.T) {
	expectedResponse := &api.MessageResponse{
		ID:    "msg_test",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-4-5",
		Content: []api.ContentBlock{
			{Type: "text", Text: "Hello from API"},
		},
		StopReason: "end_turn",
		Usage: api.Usage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	engine := NewQueryEngine(client, QueryConfig{})

	result, err := engine.Query(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result == nil {
		t.Fatal("Query returned nil result")
	}

	content, ok := result.Message.Content.(string)
	if !ok || content != "Hello from API" {
		t.Errorf("Expected content 'Hello from API', got %v", result.Message.Content)
	}

	if result.StopReason != StopReasonEndTurn {
		t.Errorf("Expected stop reason 'end_turn', got '%s'", result.StopReason)
	}

	if result.Usage.InputTokens != 10 {
		t.Errorf("Expected input tokens 10, got %d", result.Usage.InputTokens)
	}

	if result.Usage.OutputTokens != 5 {
		t.Errorf("Expected output tokens 5, got %d", result.Usage.OutputTokens)
	}

	if result.Duration <= 0 {
		t.Error("Expected positive duration")
	}

	if result.Cost <= 0 {
		t.Error("Expected positive cost")
	}

	state := engine.GetState()
	msgs := state.GetMessages()
	if len(msgs) < 2 {
		t.Errorf("Expected at least 2 messages (user + assistant), got %d", len(msgs))
	}

	if msgs[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got '%s'", msgs[0].Role)
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("Expected second message role 'assistant', got '%s'", msgs[1].Role)
	}
}

func TestQueryIncrementsTurn(t *testing.T) {
	expectedResponse := &api.MessageResponse{
		ID:    "msg_turn_test",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-4-5",
		Content: []api.ContentBlock{
			{Type: "text", Text: "Response"},
		},
		StopReason: "end_turn",
		Usage:      api.Usage{InputTokens: 5, OutputTokens: 3},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	engine := NewQueryEngine(client, QueryConfig{})

	if engine.GetState().GetTurnCount() != 0 {
		t.Error("Expected initial turn count 0")
	}

	_, err := engine.Query(context.Background(), "First query")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if engine.GetState().GetTurnCount() != 1 {
		t.Errorf("Expected turn count 1, got %d", engine.GetState().GetTurnCount())
	}

	_, err = engine.Query(context.Background(), "Second query")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if engine.GetState().GetTurnCount() != 2 {
		t.Errorf("Expected turn count 2, got %d", engine.GetState().GetTurnCount())
	}
}

func TestQueryContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	engine := NewQueryEngine(client, QueryConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := engine.Query(ctx, "This should be cancelled")
	if err == nil {
		t.Error("Expected error from cancelled context, got nil")
	}
}

func TestQueryAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	engine := NewQueryEngine(client, QueryConfig{})

	_, err := engine.Query(context.Background(), "Hello")
	if err == nil {
		t.Error("Expected error from 500 response, got nil")
	}
}

func TestExecuteTool(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	testTool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		result:      map[string]any{"status": "ok"},
	}
	engine.AddTool(testTool)

	result, err := engine.ExecuteTool(context.Background(), "test_tool", map[string]any{"arg": "value"})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	status, _ := resultMap["status"].(string)
	if status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", status)
	}

	state := engine.GetState()
	msgs := state.GetMessages()
	hasToolMsg := false
	for _, m := range msgs {
		if m.Role == "tool" {
			hasToolMsg = true
			break
		}
	}
	if !hasToolMsg {
		t.Error("Expected a tool message to be added to state")
	}
}

func TestExecuteToolUnknown(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	_, err := engine.ExecuteTool(context.Background(), "nonexistent_tool", nil)
	if err == nil {
		t.Error("Expected error for unknown tool, got nil")
	}
}

func TestExecuteToolError(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	testTool := &mockTool{
		name:        "failing_tool",
		description: "A failing tool",
		err:         fmt.Errorf("tool execution error"),
	}
	engine.AddTool(testTool)

	_, err := engine.ExecuteTool(context.Background(), "failing_tool", nil)
	if err == nil {
		t.Error("Expected error from failing tool, got nil")
	}
}

func TestExecuteToolWithoutHooks(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	testTool := &mockTool{
		name:        "direct_tool",
		description: "A direct tool",
		result:      "direct result",
	}
	engine.AddTool(testTool)

	result, err := engine.ExecuteToolWithoutHooks(context.Background(), "direct_tool", nil)
	if err != nil {
		t.Fatalf("ExecuteToolWithoutHooks failed: %v", err)
	}

	strResult, ok := result.(string)
	if !ok || strResult != "direct result" {
		t.Errorf("Expected 'direct result', got %v", result)
	}
}

func TestCompactIfNeeded(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{MaxTokens: 50})

	engine.GetState().AddMessage(Message{Role: "system", Content: "system prompt"})
	engine.GetState().AddMessage(Message{Role: "user", Content: "first user message"})
	engine.GetState().AddMessage(Message{Role: "assistant", Content: "first assistant response"})
	for i := 0; i < 15; i++ {
		engine.GetState().AddMessage(Message{Role: "user", Content: "middle user query number"})
		engine.GetState().AddMessage(Message{Role: "assistant", Content: "middle assistant answer number"})
	}
	engine.GetState().AddMessage(Message{Role: "user", Content: "final user question"})
	engine.GetState().AddMessage(Message{Role: "assistant", Content: "final assistant answer"})
	engine.GetState().UpdateUsage(Usage{InputTokens: 100, OutputTokens: 50})

	msgCountBefore := len(engine.GetState().GetMessages())
	engine.CompactIfNeeded()
	msgCountAfter := len(engine.GetState().GetMessages())

	if msgCountAfter >= msgCountBefore {
		t.Errorf("Expected fewer messages after compaction, got before=%d after=%d", msgCountBefore, msgCountAfter)
	}
}

func TestCompactIfNeededNoOp(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{MaxTokens: 10000})

	engine.GetState().AddMessage(Message{
		Role:    "user",
		Content: "short message",
	})

	msgCountBefore := len(engine.GetState().GetMessages())
	engine.CompactIfNeeded()
	msgCountAfter := len(engine.GetState().GetMessages())

	if msgCountAfter != msgCountBefore {
		t.Errorf("Expected same message count when no compaction needed, got before=%d after=%d", msgCountBefore, msgCountAfter)
	}
}

func TestCompactIfNeededZeroMaxTokens(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{MaxTokens: 0})

	for i := 0; i < 20; i++ {
		engine.GetState().AddMessage(Message{
			Role:    "user",
			Content: "test message with some words",
		})
	}
	engine.GetState().UpdateUsage(Usage{InputTokens: 100, OutputTokens: 50})

	msgCountBefore := len(engine.GetState().GetMessages())
	engine.CompactIfNeeded()
	msgCountAfter := len(engine.GetState().GetMessages())

	if msgCountAfter != msgCountBefore {
		t.Errorf("Expected no compaction with MaxTokens=0, got before=%d after=%d", msgCountBefore, msgCountAfter)
	}
}

func TestCompactIfNeededBelowThreshold(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{MaxTokens: 10000})

	for i := 0; i < 20; i++ {
		engine.GetState().AddMessage(Message{
			Role:    "user",
			Content: "test message with some words",
		})
	}
	engine.GetState().UpdateUsage(Usage{InputTokens: 50, OutputTokens: 25})

	msgCountBefore := len(engine.GetState().GetMessages())
	engine.CompactIfNeeded()
	msgCountAfter := len(engine.GetState().GetMessages())

	if msgCountAfter != msgCountBefore {
		t.Errorf("Expected no compaction when usage below threshold, got before=%d after=%d", msgCountBefore, msgCountAfter)
	}
}

func TestShutdown(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	engine.Shutdown()

	select {
	case <-engine.shutdownCh:
	default:
		t.Error("shutdownCh should be closed after Shutdown")
	}
}

func TestShutdownIdempotent(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	engine.Shutdown()
	engine.Shutdown()

	select {
	case <-engine.shutdownCh:
	default:
		t.Error("shutdownCh should be closed after Shutdown")
	}
}

func TestClose(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	err := engine.Close()
	if err != nil {
		t.Errorf("Close returned unexpected error: %v", err)
	}

	select {
	case <-engine.shutdownCh:
	default:
		t.Error("shutdownCh should be closed after Close")
	}
}

func TestSetSystemPrompt(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	engine.SetSystemPrompt("You are a test assistant")

	if engine.config.SystemPrompt != "You are a test assistant" {
		t.Errorf("Expected system prompt 'You are a test assistant', got '%s'", engine.config.SystemPrompt)
	}
}

func TestSetSessionID(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	engine.SetSessionID("test-session-123")

	if engine.sessionID != "test-session-123" {
		t.Errorf("Expected session ID 'test-session-123', got '%s'", engine.sessionID)
	}
}

func TestGetState(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	state := engine.GetState()
	if state == nil {
		t.Fatal("GetState returned nil")
	}
	if len(state.GetMessages()) != 0 {
		t.Error("New engine should have no messages in state")
	}
}

func TestAddTool(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	testTool := &mockTool{
		name:        "custom_tool",
		description: "A custom tool",
	}
	engine.AddTool(testTool)

	tool := engine.tools.Get("custom_tool")
	if tool == nil {
		t.Error("Expected custom_tool to be registered")
	}
}

func TestGetCacheClient(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	cc := engine.GetCacheClient()
	if cc == nil {
		t.Error("GetCacheClient returned nil")
	}
}

func TestSetAndGetRouter(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	if engine.GetRouter() != nil {
		t.Error("Expected nil router initially")
	}
}

func TestSetAndGetThinkingManager(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	tm := engine.GetThinkingManager()
	if tm == nil {
		t.Error("Expected non-nil thinking manager")
	}

	newTm := NewThinkingManager()
	newTm.Enable(20000)
	engine.SetThinkingManager(newTm)

	if engine.GetThinkingManager().GetBudget() != 20000 {
		t.Errorf("Expected budget 20000, got %d", engine.GetThinkingManager().GetBudget())
	}
}

func TestSetAndGetCostGuard(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	if engine.GetCostGuard() != nil {
		t.Error("Expected nil cost guard initially")
	}
}

func TestSetAndGetPrefetcher(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	if engine.GetPrefetcher() != nil {
		t.Error("Expected nil prefetcher initially")
	}
}

func TestConversationTreeDisabled(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	_, err := engine.Branch("nonexistent")
	if err == nil {
		t.Error("Expected error when tree not enabled")
	}

	err = engine.Checkout("nonexistent")
	if err == nil {
		t.Error("Expected error when tree not enabled")
	}

	if engine.GetBranches() != nil {
		t.Error("Expected nil branches when tree not enabled")
	}

	if engine.GetConversationHeadID() != "" {
		t.Error("Expected empty head ID when tree not enabled")
	}
}

func TestEnableConversationTree(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	engine.EnableConversationTree()

	if !engine.state.IsTreeEnabled() {
		t.Error("Expected tree to be enabled")
	}
}

func TestCalculateCost(t *testing.T) {
	usage := api.Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	cost := CalculateCost(usage)
	if cost <= 0 {
		t.Errorf("Expected positive cost, got %f", cost)
	}

	expectedInput := float64(1000) * 0.000015
	expectedOutput := float64(500) * 0.000075
	expected := expectedInput + expectedOutput

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestExtractResponseText(t *testing.T) {
	resp := &api.MessageResponse{
		Content: []api.ContentBlock{
			{Type: "thinking", Thinking: "internal thought"},
			{Type: "text", Text: "visible response"},
		},
	}

	text := extractResponseText(resp)
	if text != "visible response" {
		t.Errorf("Expected 'visible response', got '%s'", text)
	}
}

func TestExtractResponseTextNil(t *testing.T) {
	text := extractResponseText(nil)
	if text != "" {
		t.Errorf("Expected empty string for nil response, got '%s'", text)
	}
}

func TestExtractResponseTextEmpty(t *testing.T) {
	resp := &api.MessageResponse{
		Content: []api.ContentBlock{
			{Type: "thinking", Thinking: "only thinking"},
		},
	}

	text := extractResponseText(resp)
	if text != "" {
		t.Errorf("Expected empty string when no text block, got '%s'", text)
	}
}

func TestContainsCode(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"func main() {}", true},
		{"def hello():", true},
		{"package main", true},
		{"class MyClass:", true},
		{"import os", true},
		{"x => x + 1", true},
		{"a == b", true},
		{"hello world", false},
		{"this is plain text", false},
	}

	for _, tt := range tests {
		result := containsCode(tt.input)
		if result != tt.expected {
			t.Errorf("containsCode(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestNewQueryEngineWithHooks(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngineWithHooks(client, QueryConfig{}, nil)

	if engine == nil {
		t.Fatal("NewQueryEngineWithHooks returned nil")
	}
	if engine.hookExecutor == nil {
		t.Error("hookExecutor should not be nil")
	}
}

func TestSetHookManager(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	if engine.GetHookManager() != nil {
		t.Error("Expected nil hook manager initially")
	}
}

func TestQueryWithSystemPrompt(t *testing.T) {
	expectedResponse := &api.MessageResponse{
		ID:    "msg_sys_test",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-4-5",
		Content: []api.ContentBlock{
			{Type: "text", Text: "System response"},
		},
		StopReason: "end_turn",
		Usage:      api.Usage{InputTokens: 15, OutputTokens: 8},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	engine := NewQueryEngine(client, QueryConfig{SystemPrompt: "You are helpful"})

	result, err := engine.Query(context.Background(), "Hello with system")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result == nil {
		t.Fatal("Query returned nil result")
	}
}

func TestQueryMultipleTurns(t *testing.T) {
	expectedResponse := &api.MessageResponse{
		ID:    "msg_multi",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-4-5",
		Content: []api.ContentBlock{
			{Type: "text", Text: "Multi turn response"},
		},
		StopReason: "end_turn",
		Usage:      api.Usage{InputTokens: 20, OutputTokens: 10},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	engine := NewQueryEngine(client, QueryConfig{})

	_, err := engine.Query(context.Background(), "First question")
	if err != nil {
		t.Fatalf("First query failed: %v", err)
	}

	_, err = engine.Query(context.Background(), "Follow-up question")
	if err != nil {
		t.Fatalf("Second query failed: %v", err)
	}

	msgs := engine.GetState().GetMessages()
	if len(msgs) != 4 {
		t.Errorf("Expected 4 messages (2 user + 2 assistant), got %d", len(msgs))
	}

	usage := engine.GetState().GetUsage()
	if usage.InputTokens != 40 {
		t.Errorf("Expected cumulative input tokens 40, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 20 {
		t.Errorf("Expected cumulative output tokens 20, got %d", usage.OutputTokens)
	}
}

func TestQueryStopReasonToolUse(t *testing.T) {
	expectedResponse := &api.MessageResponse{
		ID:    "msg_tool_use",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-4-5",
		Content: []api.ContentBlock{
			{Type: "text", Text: "Let me check"},
		},
		StopReason: "tool_use",
		Usage:      api.Usage{InputTokens: 10, OutputTokens: 5},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	engine := NewQueryEngine(client, QueryConfig{})

	result, err := engine.Query(context.Background(), "Check something")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result.StopReason != StopReasonToolUse {
		t.Errorf("Expected stop reason 'tool_use', got '%s'", result.StopReason)
	}
}

func TestShutdownWithBackgroundGoroutine(t *testing.T) {
	client := api.NewClient("test-key")
	engine := NewQueryEngine(client, QueryConfig{})

	var wg sync.WaitGroup
	wg.Add(1)
	engine.bgWg.Add(1)
	go func() {
		defer engine.bgWg.Done()
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
	}()

	engine.Shutdown()
	wg.Wait()
}

type mockTool struct {
	name        string
	description string
	result      any
	err         error
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string         { return m.description }
func (m *mockTool) InputSchema() map[string]any { return nil }
func (m *mockTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

var _ tools.Tool = (*mockTool)(nil)
