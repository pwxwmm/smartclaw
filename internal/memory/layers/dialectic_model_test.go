package layers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

func newDialecticTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mockLLMFunc(responses ...string) DialecticLLMFunc {
	idx := 0
	return func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		if idx >= len(responses) {
			return "", fmt.Errorf("mock LLM: no more responses")
		}
		resp := responses[idx]
		idx++
		return resp, nil
	}
}

func insertTestObservations(t *testing.T, s *store.Store, userID string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		_, err := s.DB().Exec(
			`INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"preference", "lang", fmt.Sprintf("go-%d", i), 0.8, time.Now(), "test-session", userID,
		)
		if err != nil {
			t.Fatalf("insert observation %d: %v", i, err)
		}
	}
}

func TestNewDialecticUserModel(t *testing.T) {
	s := newDialecticTestStore(t)
	config := DefaultDialecticConfig()
	llm := mockLLMFunc("analysis result")

	dm := NewDialecticUserModel(s, nil, config, llm)
	if dm == nil {
		t.Fatal("NewDialecticUserModel returned nil")
	}
	if dm.config.Cadence != 10 {
		t.Errorf("expected Cadence=10, got %d", dm.config.Cadence)
	}
	if dm.config.ReasoningLevel != ReasoningStandard {
		t.Errorf("expected ReasoningLevel=standard, got %s", dm.config.ReasoningLevel)
	}
}

func TestNewDialecticUserModel_Defaults(t *testing.T) {
	s := newDialecticTestStore(t)
	llm := mockLLMFunc("result")

	config := DialecticConfig{}
	dm := NewDialecticUserModel(s, nil, config, llm)

	if dm.config.Cadence != 10 {
		t.Errorf("empty config should default Cadence to 10, got %d", dm.config.Cadence)
	}
	if dm.config.ReasoningLevel != ReasoningStandard {
		t.Errorf("empty config should default ReasoningLevel to standard, got %s", dm.config.ReasoningLevel)
	}
	if dm.config.MaxObservations != 50 {
		t.Errorf("empty config should default MaxObservations to 50, got %d", dm.config.MaxObservations)
	}
}

func TestDialecticUserModel_CadenceGating(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 5)

	config := DialecticConfig{
		Cadence:         3,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	callCount := 0
	llm := func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		callCount++
		return "- insight from LLM\n", nil
	}

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	if dm.ShouldTrigger() {
		t.Error("should not trigger with 0 messages")
	}

	dm.ProcessMessage(ctx, "user", "msg1")
	if dm.GetMessageCount() != 1 {
		t.Errorf("expected messageCount=1, got %d", dm.GetMessageCount())
	}
	if dm.ShouldTrigger() {
		t.Error("should not trigger after 1 message (cadence=3)")
	}

	dm.ProcessMessage(ctx, "user", "msg2")
	if dm.ShouldTrigger() {
		t.Error("should not trigger after 2 messages (cadence=3)")
	}

	dm.ProcessMessage(ctx, "user", "msg3")
	if callCount == 0 {
		t.Error("LLM should have been called after cadence reached")
	}
}

func TestDialecticUserModel_ProcessMessage_IncrementsCounter(t *testing.T) {
	s := newDialecticTestStore(t)
	config := DialecticConfig{Cadence: 100, ReasoningLevel: ReasoningQuick, AutoUpdate: false}
	llm := mockLLMFunc("result")

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	dm.ProcessMessage(ctx, "user", "hello")
	dm.ProcessMessage(ctx, "user", "world")
	dm.ProcessMessage(ctx, "user", "test")

	if dm.GetMessageCount() != 3 {
		t.Errorf("expected 3 messages, got %d", dm.GetMessageCount())
	}
}

func TestDialecticUserModel_ProcessMessage_IgnoresNonUser(t *testing.T) {
	s := newDialecticTestStore(t)
	config := DialecticConfig{Cadence: 100, ReasoningLevel: ReasoningQuick, AutoUpdate: false}
	llm := mockLLMFunc("result")

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	dm.ProcessMessage(ctx, "assistant", "response1")
	dm.ProcessMessage(ctx, "system", "system msg")
	dm.ProcessMessage(ctx, "tool", "tool result")

	if dm.GetMessageCount() != 0 {
		t.Errorf("non-user messages should not increment counter, got %d", dm.GetMessageCount())
	}
}

func TestDialecticUserModel_ResetCadence(t *testing.T) {
	s := newDialecticTestStore(t)
	config := DialecticConfig{Cadence: 100, ReasoningLevel: ReasoningQuick, AutoUpdate: false}
	llm := mockLLMFunc("result")

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	dm.ProcessMessage(ctx, "user", "msg1")
	dm.ProcessMessage(ctx, "user", "msg2")
	if dm.GetMessageCount() != 2 {
		t.Errorf("expected 2 messages, got %d", dm.GetMessageCount())
	}

	dm.ResetCadence()
	if dm.GetMessageCount() != 0 {
		t.Errorf("expected 0 after reset, got %d", dm.GetMessageCount())
	}
}

func TestDialecticUserModel_QuickReasoning(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 5)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := mockLLMFunc("- user prefers Go\n- user is detail-oriented\n")

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	err := dm.ProcessMessage(ctx, "user", "trigger")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	rounds := dm.GetRounds()
	if len(rounds) != 1 {
		t.Fatalf("quick: expected 1 round, got %d", len(rounds))
	}
	if rounds[0].Phase != "analyze" {
		t.Errorf("quick: expected phase=analyze, got %s", rounds[0].Phase)
	}
}

func TestDialecticUserModel_StandardReasoning(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 5)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningStandard,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := mockLLMFunc(
		"- user prefers Go\n- user is detail-oriented\n",
		"- user prefers concise code\n- user follows TDD\n",
	)

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	err := dm.ProcessMessage(ctx, "user", "trigger")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	rounds := dm.GetRounds()
	if len(rounds) != 2 {
		t.Fatalf("standard: expected 2 rounds, got %d", len(rounds))
	}

	if rounds[0].Phase != "analyze" {
		t.Errorf("standard round 1: expected phase=analyze, got %s", rounds[0].Phase)
	}
	if rounds[1].Phase != "synthesize" {
		t.Errorf("standard round 2: expected phase=synthesize, got %s", rounds[1].Phase)
	}
}

func TestDialecticUserModel_DeepReasoning(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 5)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningDeep,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := mockLLMFunc(
		"- user prefers Go\n- user is detail-oriented\n",
		"- might be overfitting to recent data\n- missing communication style analysis\n",
		"- user prefers Go with careful attention to detail\n- needs more data on communication style\n",
	)

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	err := dm.ProcessMessage(ctx, "user", "trigger")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	rounds := dm.GetRounds()
	if len(rounds) != 3 {
		t.Fatalf("deep: expected 3 rounds, got %d", len(rounds))
	}

	if rounds[0].Phase != "analyze" {
		t.Errorf("deep round 1: expected phase=analyze, got %s", rounds[0].Phase)
	}
	if rounds[1].Phase != "critique" {
		t.Errorf("deep round 2: expected phase=critique, got %s", rounds[1].Phase)
	}
	if rounds[2].Phase != "synthesize" {
		t.Errorf("deep round 3: expected phase=synthesize, got %s", rounds[2].Phase)
	}
}

func TestDialecticUserModel_RoundRecording(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 3)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := mockLLMFunc("- user likes vim\n- user prefers dark theme\n")

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	err := dm.ProcessMessage(ctx, "user", "trigger")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	rounds := dm.GetRounds()
	if len(rounds) != 1 {
		t.Fatalf("expected 1 round, got %d", len(rounds))
	}

	r := rounds[0]
	if r.RoundNumber != 1 {
		t.Errorf("expected RoundNumber=1, got %d", r.RoundNumber)
	}
	if r.Phase != "analyze" {
		t.Errorf("expected Phase=analyze, got %s", r.Phase)
	}
	if r.Reasoning == "" {
		t.Error("Reasoning should not be empty")
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
	if len(r.Insights) != 2 {
		t.Errorf("expected 2 insights, got %d", len(r.Insights))
	}
	if r.Input == "" {
		t.Error("Input should not be empty")
	}
}

func TestDialecticUserModel_GetInsights(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 3)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningDeep,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := mockLLMFunc(
		"- analysis insight 1\n- analysis insight 2\n",
		"- critique insight 1\n",
		"- synthesize insight 1\n- synthesize insight 2\n- synthesize insight 3\n",
	)

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	dm.ProcessMessage(ctx, "user", "trigger")

	insights := dm.GetInsights()
	totalExpected := 2 + 1 + 3
	if len(insights) != totalExpected {
		t.Errorf("expected %d total insights, got %d", totalExpected, len(insights))
	}
}

func TestDialecticUserModel_NilLLMFunc(t *testing.T) {
	s := newDialecticTestStore(t)
	config := DialecticConfig{Cadence: 1, ReasoningLevel: ReasoningQuick, AutoUpdate: false}

	dm := NewDialecticUserModel(s, nil, config, nil)
	ctx := context.Background()

	err := dm.RunDialecticCycle(ctx)
	if err == nil {
		t.Error("expected error with nil LLM function")
	}
}

func TestDialecticUserModel_NilStore(t *testing.T) {
	config := DialecticConfig{Cadence: 1, ReasoningLevel: ReasoningQuick, AutoUpdate: false}
	llm := mockLLMFunc("result")

	dm := NewDialecticUserModel(nil, nil, config, llm)
	ctx := context.Background()

	err := dm.RunDialecticCycle(ctx)
	if err != nil {
		t.Errorf("nil store should not error, just skip: %v", err)
	}
}

func TestDialecticUserModel_NoObservations(t *testing.T) {
	s := newDialecticTestStore(t)
	config := DialecticConfig{Cadence: 1, ReasoningLevel: ReasoningQuick, AutoUpdate: false}
	llm := mockLLMFunc("result")

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	err := dm.RunDialecticCycle(ctx)
	if err != nil {
		t.Fatalf("empty observations should not error: %v", err)
	}

	rounds := dm.GetRounds()
	if len(rounds) != 0 {
		t.Errorf("expected 0 rounds with no observations, got %d", len(rounds))
	}
}

func TestDialecticUserModel_CadenceResetsAfterCycle(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 5)

	config := DialecticConfig{
		Cadence:         2,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	callCount := 0
	llm := func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		callCount++
		return "- insight\n", nil
	}

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	dm.ProcessMessage(ctx, "user", "msg1")
	dm.ProcessMessage(ctx, "user", "msg2")

	if callCount != 1 {
		t.Errorf("expected 1 LLM call after first cycle, got %d", callCount)
	}
	if dm.GetMessageCount() != 0 {
		t.Errorf("counter should reset after cycle, got %d", dm.GetMessageCount())
	}

	dm.ProcessMessage(ctx, "user", "msg3")
	if dm.GetMessageCount() != 1 {
		t.Errorf("expected 1 after reset, got %d", dm.GetMessageCount())
	}
}

func TestDialecticUserModel_ConcurrentProcessMessage(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 10)

	config := DialecticConfig{
		Cadence:         100,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := mockLLMFunc("- insight\n")

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dm.ProcessMessage(ctx, "user", "concurrent msg")
		}()
	}
	wg.Wait()

	if dm.GetMessageCount() != 50 {
		t.Errorf("expected 50 after concurrent increments, got %d", dm.GetMessageCount())
	}
}

func TestDialecticUserModel_SetUserID(t *testing.T) {
	s := newDialecticTestStore(t)

	userID := "test-user-42"
	_, err := s.DB().Exec(
		`INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"preference", "editor", "vim", 0.9, time.Now(), "sess-1", userID,
	)
	if err != nil {
		t.Fatalf("insert test observation: %v", err)
	}

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	var capturedPrompt string
	llm := func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		capturedPrompt = prompt
		return "- user likes vim\n", nil
	}

	dm := NewDialecticUserModel(s, nil, config, llm)
	dm.SetUserID(userID)
	ctx := context.Background()

	err = dm.ProcessMessage(ctx, "user", "trigger")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if capturedPrompt == "" {
		t.Error("LLM should have been called")
	}
}

func TestDialecticUserModel_ExtractInsights(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"bullet points", "- insight 1\n- insight 2\n- insight 3\n", 3},
		{"no bullets", "just plain text", 1},
		{"empty string", "", 0},
		{"mixed", "Some text\n- bullet 1\nMore text\n- bullet 2\n", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insights := extractInsights(tt.input)
			if len(insights) != tt.expected {
				t.Errorf("expected %d insights, got %d", tt.expected, len(insights))
			}
		})
	}
}

func TestDialecticUserModel_TruncateForStorage(t *testing.T) {
	short := "hello"
	long := ""
	for i := 0; i < 3000; i++ {
		long += "x"
	}

	if truncateForStorage(short, 10) != short {
		t.Error("short string should not be truncated")
	}

	truncated := truncateForStorage(long, 100)
	if len(truncated) != 100 {
		t.Errorf("expected 100 chars, got %d", len(truncated))
	}
}

func TestDialecticUserModel_UserModelingLayerIntegration(t *testing.T) {
	s := newDialecticTestStore(t)
	dir := t.TempDir()

	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	uml := NewUserModelingLayer(s, pm)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := mockLLMFunc("- user prefers Go\n")
	dm := NewDialecticUserModel(s, pm, config, llm)

	uml.SetDialecticModel(dm)

	if uml.GetDialecticModel() == nil {
		t.Error("GetDialecticModel should return the set model")
	}

	insertTestObservations(t, s, "default", 3)

	ctx := context.Background()
	err = uml.RunDialecticCycle(ctx)
	if err != nil {
		t.Fatalf("RunDialecticCycle failed: %v", err)
	}

	rounds := uml.GetDialecticModel().GetRounds()
	if len(rounds) != 1 {
		t.Errorf("expected 1 round, got %d", len(rounds))
	}
}

func TestDialecticUserModel_UserModelingLayerRunDialecticCycle_Nil(t *testing.T) {
	s := newDialecticTestStore(t)
	uml := NewUserModelingLayer(s, nil)

	err := uml.RunDialecticCycle(context.Background())
	if err == nil {
		t.Error("expected error when dialectic model is nil")
	}
}

func TestDialecticUserModel_AutoUpdate(t *testing.T) {
	s := newDialecticTestStore(t)
	dir := t.TempDir()

	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	insertTestObservations(t, s, "default", 3)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      true,
	}

	llm := mockLLMFunc("- user prefers Go\n- user likes testing\n")

	dm := NewDialecticUserModel(s, pm, config, llm)
	ctx := context.Background()

	err = dm.ProcessMessage(ctx, "user", "trigger")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	content := pm.GetUserContent()
	if content == "" {
		t.Error("USER.md should have been updated")
	}
}

func TestDialecticUserModel_LLMError(t *testing.T) {
	s := newDialecticTestStore(t)
	insertTestObservations(t, s, "default", 3)

	config := DialecticConfig{
		Cadence:         1,
		ReasoningLevel:  ReasoningQuick,
		MaxObservations: 50,
		AutoUpdate:      false,
	}

	llm := func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		return "", fmt.Errorf("LLM unavailable")
	}

	dm := NewDialecticUserModel(s, nil, config, llm)
	ctx := context.Background()

	err := dm.ProcessMessage(ctx, "user", "trigger")
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

func TestDefaultDialecticConfig(t *testing.T) {
	cfg := DefaultDialecticConfig()
	if cfg.Cadence != 10 {
		t.Errorf("expected default Cadence=10, got %d", cfg.Cadence)
	}
	if cfg.ReasoningLevel != ReasoningStandard {
		t.Errorf("expected default ReasoningLevel=standard, got %s", cfg.ReasoningLevel)
	}
	if cfg.MaxObservations != 50 {
		t.Errorf("expected default MaxObservations=50, got %d", cfg.MaxObservations)
	}
	if cfg.AutoUpdate != true {
		t.Error("expected default AutoUpdate=true")
	}
}
