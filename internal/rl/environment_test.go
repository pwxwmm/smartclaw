package rl

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

type mockLLMHandler struct {
	mu        sync.Mutex
	responses []api.MessageResponse
	callCount int
}

func (h *mockLLMHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	idx := h.callCount
	if idx >= len(h.responses) {
		idx = len(h.responses) - 1
	}
	resp := h.responses[idx]
	h.callCount++

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func newMockClient(t *testing.T, handler http.Handler) *api.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := api.NewClientWithBaseURL("test-key", server.URL)
	client.Model = "test-model"
	return client
}

func textResponse(text string) api.MessageResponse {
	return api.MessageResponse{
		ID:    "msg_test",
		Type:  "message",
		Role:  "assistant",
		Model: "test-model",
		Content: []api.ContentBlock{
			{Type: "text", Text: text},
		},
		StopReason: "end_turn",
		Usage:      api.Usage{InputTokens: 10, OutputTokens: 20},
	}
}

func TestNewEnvironment(t *testing.T) {
	handler := &mockLLMHandler{responses: []api.MessageResponse{textResponse("hello")}}
	client := newMockClient(t, handler)

	cfg := DefaultEnvironmentConfig()
	env := NewEnvironment(client, cfg)

	if env == nil {
		t.Fatal("NewEnvironment returned nil")
	}
	if env.config.MaxSteps != 10 {
		t.Errorf("expected MaxSteps=10, got %d", env.config.MaxSteps)
	}
	if env.config.SuccessMetric != "exact_match" {
		t.Errorf("expected SuccessMetric=exact_match, got %s", env.config.SuccessMetric)
	}
	if env.rewards == nil {
		t.Error("reward function should not be nil")
	}
	if env.rewards.Name() != "exact_match" {
		t.Errorf("expected default reward=exact_match, got %s", env.rewards.Name())
	}
}

func TestNewEnvironment_CustomConfig(t *testing.T) {
	handler := &mockLLMHandler{responses: []api.MessageResponse{textResponse("hello")}}
	client := newMockClient(t, handler)

	cfg := EnvironmentConfig{
		TaskType:      "coding",
		MaxSteps:      3,
		Timeout:       10 * time.Second,
		SuccessMetric: "code_quality",
	}

	env := NewEnvironment(client, cfg)
	if env.config.MaxSteps != 3 {
		t.Errorf("expected MaxSteps=3, got %d", env.config.MaxSteps)
	}
	if env.rewards.Name() != "code_quality" {
		t.Errorf("expected code_quality, got %s", env.rewards.Name())
	}
}

func TestRunEpisode_Completes(t *testing.T) {
	handler := &mockLLMHandler{
		responses: []api.MessageResponse{
			textResponse("func main() { fmt.Println(\"hello\") }"),
		},
	}
	client := newMockClient(t, handler)

	cfg := EnvironmentConfig{
		MaxSteps:      5,
		Timeout:       30 * time.Second,
		SuccessMetric: "exact_match",
	}
	env := NewEnvironment(client, cfg)

	result, err := env.RunEpisode(context.Background(), "Write a hello world program")
	if err != nil {
		t.Fatalf("RunEpisode failed: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Steps) == 0 {
		t.Error("expected at least one step")
	}
	if result.Model != "test-model" {
		t.Errorf("expected model=test-model, got %s", result.Model)
	}
	if result.Duration == 0 {
		t.Error("duration should be > 0")
	}
	if result.Steps[0].Step != 0 {
		t.Errorf("first step should be 0, got %d", result.Steps[0].Step)
	}
}

func TestRunEpisode_MultipleSteps(t *testing.T) {
	handler := &mockLLMHandler{
		responses: []api.MessageResponse{
			textResponse("Let me think about this..."),
			textResponse("```python\ndef hello():\n    print('hello')\n```"),
		},
	}
	client := newMockClient(t, handler)

	cfg := EnvironmentConfig{
		MaxSteps:      5,
		Timeout:       30 * time.Second,
		SuccessMetric: "exact_match",
	}
	env := NewEnvironment(client, cfg)

	result, err := env.RunEpisode(context.Background(), "Write a hello function")
	if err != nil {
		t.Fatalf("RunEpisode failed: %v", err)
	}

	if len(result.Steps) < 2 {
		t.Errorf("expected at least 2 steps, got %d", len(result.Steps))
	}
}

func TestRunEpisode_ContextCancellation(t *testing.T) {
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		json.NewEncoder(w).Encode(textResponse("slow response"))
	})
	client := newMockClient(t, slowHandler)

	cfg := EnvironmentConfig{
		MaxSteps:      100,
		Timeout:       30 * time.Second,
		SuccessMetric: "length_penalty",
	}
	env := NewEnvironment(client, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := env.RunEpisode(ctx, "Write something")
	if err == nil {
		t.Error("expected error from cancelled context")
	}
	if result != nil && result.TotalReward != 0 {
		t.Logf("Partial result: %d steps, reward=%.2f", len(result.Steps), result.TotalReward)
	}
}

func TestRunEpisode_ErrorFromLLM(t *testing.T) {
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, `{"error": "internal server error"}`)
	})
	client := newMockClient(t, errorHandler)

	cfg := EnvironmentConfig{
		MaxSteps:      3,
		Timeout:       10 * time.Second,
		SuccessMetric: "exact_match",
	}
	env := NewEnvironment(client, cfg)

	result, err := env.RunEpisode(context.Background(), "Test prompt")
	if err == nil {
		t.Error("expected error from LLM failure")
	}
	if result != nil {
		t.Error("result should be nil on LLM error")
	}
}

func TestGetRewardFunction_Known(t *testing.T) {
	tests := []struct {
		name         string
		expectedType string
	}{
		{"exact_match", "exact_match"},
		{"code_quality", "code_quality"},
		{"length_penalty", "length_penalty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := GetRewardFunction(tt.name)
			if fn == nil {
				t.Error("GetRewardFunction returned nil")
			}
			if fn.Name() != tt.expectedType {
				t.Errorf("expected name=%s, got %s", tt.expectedType, fn.Name())
			}
		})
	}
}

func TestGetRewardFunction_Unknown(t *testing.T) {
	fn := GetRewardFunction("nonexistent")
	if fn == nil {
		t.Error("GetRewardFunction should return default for unknown names")
	}
	if fn.Name() != "exact_match" {
		t.Errorf("expected fallback to exact_match, got %s", fn.Name())
	}
}

type mockRewardFn struct {
	name string
}

func (m *mockRewardFn) Compute(task, response string, step int, obs *Observation) float64 {
	return 1.0
}
func (m *mockRewardFn) IsDone(task, response string, step int, obs *Observation) bool { return true }
func (m *mockRewardFn) Feedback(reward float64, step int) string                      { return "" }
func (m *mockRewardFn) Name() string                                                  { return m.name }

func TestRegisterRewardFunction(t *testing.T) {
	customFn := &mockRewardFn{name: "custom_test"}
	RegisterRewardFunction("custom_test", customFn)

	fn := GetRewardFunction("custom_test")
	if fn == nil {
		t.Error("registered function should be retrievable")
	}
	if fn.Name() != "custom_test" {
		t.Errorf("expected custom_test, got %s", fn.Name())
	}

	RegisterRewardFunction("custom_test", nil)
}

func TestSetRewardFunction(t *testing.T) {
	handler := &mockLLMHandler{responses: []api.MessageResponse{textResponse("ok")}}
	client := newMockClient(t, handler)

	env := NewEnvironment(client, DefaultEnvironmentConfig())
	customFn := &mockRewardFn{name: "override"}
	env.SetRewardFunction(customFn)

	if env.rewards.Name() != "override" {
		t.Errorf("expected override, got %s", env.rewards.Name())
	}
}

func TestExactMatchReward(t *testing.T) {
	r := &ExactMatchReward{}

	if r.Name() != "exact_match" {
		t.Errorf("expected exact_match, got %s", r.Name())
	}

	obs := &Observation{Text: "func main()", HasCode: true, HasError: false}
	if reward := r.Compute("task", "func main()", 0, obs); !approxEqual(reward, 0.5) {
		t.Errorf("step0 with code: expected 0.5, got %.6f", reward)
	}

	if reward := r.Compute("task", "func main()", 1, obs); !approxEqual(reward, 0.1) {
		t.Errorf("step1 with code: expected 0.1, got %.6f", reward)
	}

	obsErr := &Observation{Text: "error: something failed", HasCode: false, HasError: true}
	if reward := r.Compute("task", "error: something failed", 0, obsErr); !approxEqual(reward, -0.1) {
		t.Errorf("with error: expected -0.1, got %.6f", reward)
	}

	obsPlain := &Observation{Text: "thinking about it", HasCode: false, HasError: false}
	if reward := r.Compute("task", "thinking about it", 0, obsPlain); !approxEqual(reward, 0.1) {
		t.Errorf("plain text: expected 0.1, got %.6f", reward)
	}

	obsCode := &Observation{HasCode: true, HasError: false}
	if !r.IsDone("task", "response", 0, obsCode) {
		t.Error("should be done when code present and no error")
	}

	obsCodeErr := &Observation{HasCode: true, HasError: true}
	if r.IsDone("task", "response", 0, obsCodeErr) {
		t.Error("should not be done when error present")
	}

	obsNoCode := &Observation{HasCode: false, HasError: false}
	if !r.IsDone("task", "response", 5, obsNoCode) {
		t.Error("should be done at step 5")
	}

	if fb := r.Feedback(-0.1, 0); fb == "" {
		t.Error("should provide feedback for negative reward")
	}

	if fb := r.Feedback(0.5, 0); fb != "" {
		t.Error("should not provide feedback for positive reward")
	}
}

func TestCodeQualityReward(t *testing.T) {
	r := &CodeQualityReward{}

	if r.Name() != "code_quality" {
		t.Errorf("expected code_quality, got %s", r.Name())
	}

	obs := &Observation{HasCode: true, HasError: false}
	if reward := r.Compute("task", "func main()", 0, obs); !approxEqual(reward, 0.3) {
		t.Errorf("code no error: expected 0.3, got %.6f", reward)
	}

	obsCodeErr := &Observation{HasCode: true, HasError: true}
	if reward := r.Compute("task", "func main() error:", 0, obsCodeErr); !approxEqual(reward, 0.1) {
		t.Errorf("code with error: expected 0.1, got %.6f", reward)
	}

	obsStep := &Observation{HasCode: true, HasError: false}
	if reward := r.Compute("task", "func main()", 4, obsStep); !approxEqual(reward, 0.1) {
		t.Errorf("step4 code: expected 0.1, got %.6f", reward)
	}

	obsNoCodeErr := &Observation{HasCode: false, HasError: true}
	if reward := r.Compute("task", "error:", 0, obsNoCodeErr); !approxEqual(reward, -0.2) {
		t.Errorf("no code error: expected -0.2, got %.6f", reward)
	}

	if reward := r.Compute("task", "error:", 20, obsNoCodeErr); !approxEqual(reward, -1.0) {
		t.Errorf("clamped: expected -1.0, got %.6f", reward)
	}

	obsNeutral := &Observation{}
	if r.IsDone("task", "response", 7, obsNeutral) {
		t.Error("should not be done at step 7")
	}
	if !r.IsDone("task", "response", 8, obsNeutral) {
		t.Error("should be done at step 8")
	}

	if fb := r.Feedback(-0.1, 0); fb == "" {
		t.Error("should provide feedback for negative reward")
	}
	if fb := r.Feedback(0.3, 0); fb == "" {
		t.Error("should provide feedback for high reward")
	}
	if fb := r.Feedback(0.1, 0); fb != "" {
		t.Error("should not provide feedback for medium reward")
	}
}

func TestLengthPenaltyReward(t *testing.T) {
	r := &LengthPenaltyReward{}

	if r.Name() != "length_penalty" {
		t.Errorf("expected length_penalty, got %s", r.Name())
	}

	obs := &Observation{HasError: false}
	if reward := r.Compute("task", "short", 0, obs); !approxEqual(reward, 1.0) {
		t.Errorf("short response: expected 1.0, got %.6f", reward)
	}

	longResp := string(make([]byte, 6000))
	if reward := r.Compute("task", longResp, 0, obs); !approxEqual(reward, 0.7) {
		t.Errorf("long response: expected 0.7, got %.6f", reward)
	}

	obsErr := &Observation{HasError: true}
	if reward := r.Compute("task", "short error:", 0, obsErr); !approxEqual(reward, 0.5) {
		t.Errorf("with error: expected 0.5, got %.6f", reward)
	}

	if reward := r.Compute("task", "ok", 3, obs); !approxEqual(reward, 0.7) {
		t.Errorf("step3: expected 0.7, got %.6f", reward)
	}

	if reward := r.Compute("task", longResp, 12, obsErr); !approxEqual(reward, -1.0) {
		t.Errorf("clamped: expected -1.0, got %.6f", reward)
	}

	obsNeutral := &Observation{}
	if r.IsDone("task", "response", 4, obsNeutral) {
		t.Error("should not be done at step 4")
	}
	if !r.IsDone("task", "response", 5, obsNeutral) {
		t.Error("should be done at step 5")
	}

	if fb := r.Feedback(0.5, 0); fb != "" {
		t.Error("length_penalty should never provide feedback")
	}
}

func TestDefaultObservationParser(t *testing.T) {
	p := &DefaultObservationParser{}

	obs := p.Parse("func main() {}")
	if !obs.HasCode {
		t.Error("should detect 'func ' as code")
	}

	obs = p.Parse("here is some ```python code```")
	if !obs.HasCode {
		t.Error("should detect backticks as code")
	}

	obs = p.Parse("def hello():")
	if !obs.HasCode {
		t.Error("should detect 'def ' as code")
	}

	obs = p.Parse("just some regular text")
	if obs.HasCode {
		t.Error("should not detect code in plain text")
	}

	obs = p.Parse("error: something went wrong")
	if !obs.HasError {
		t.Error("should detect 'error:' as error")
	}

	obs = p.Parse("panic: runtime error")
	if !obs.HasError {
		t.Error("should detect 'panic:' as error")
	}

	obs = p.Parse("everything is fine")
	if obs.HasError {
		t.Error("should not detect error in plain text")
	}

	input := "some text here"
	obs = p.Parse(input)
	if obs.Text != input {
		t.Errorf("expected Text=%q, got %q", input, obs.Text)
	}
}

func TestTrajectoryExporter_Export(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "trajectories")
	exporter := NewTrajectoryExporter(outputDir)

	episode := &EpisodeResult{
		Steps: []StepResult{
			{Step: 0, Action: "think", Observation: "obs", Reward: 0.5, Done: false},
			{Step: 1, Action: "code", Observation: "result", Reward: 0.3, Done: true},
		},
		TotalReward: 0.8,
		Success:     true,
		Duration:    2 * time.Second,
		Model:       "test-model",
	}

	err := exporter.Export(episode, "task_001")
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("output directory should be created")
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(outputDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded EpisodeResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if loaded.TotalReward != 0.8 {
		t.Errorf("expected TotalReward=0.8, got %.2f", loaded.TotalReward)
	}
	if len(loaded.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(loaded.Steps))
	}
	if !loaded.Success {
		t.Error("expected Success=true")
	}
	if loaded.Model != "test-model" {
		t.Errorf("expected Model=test-model, got %s", loaded.Model)
	}
}

func TestTrajectoryExporter_ExportMultiple(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "trajectories")
	exporter := NewTrajectoryExporter(outputDir)

	for i := 0; i < 3; i++ {
		episode := &EpisodeResult{
			Steps:       []StepResult{{Step: 0, Reward: float64(i) * 0.1}},
			TotalReward: float64(i) * 0.1,
			Model:       "test-model",
		}
		if err := exporter.Export(episode, fmt.Sprintf("task_%03d", i)); err != nil {
			t.Fatalf("Export %d failed: %v", i, err)
		}
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 files, got %d", len(entries))
	}
}

func TestTrajectoryExporter_InvalidDir(t *testing.T) {
	exporter := NewTrajectoryExporter("/dev/null/impossible/path")
	episode := &EpisodeResult{Steps: []StepResult{}, TotalReward: 0}

	err := exporter.Export(episode, "task")
	if err == nil {
		t.Error("expected error for invalid output directory")
	}
}

func TestDefaultEnvironmentConfig(t *testing.T) {
	cfg := DefaultEnvironmentConfig()
	if cfg.MaxSteps != 10 {
		t.Errorf("expected MaxSteps=10, got %d", cfg.MaxSteps)
	}
	if cfg.Timeout != 300*time.Second {
		t.Errorf("expected Timeout=300s, got %v", cfg.Timeout)
	}
	if cfg.SuccessMetric != "exact_match" {
		t.Errorf("expected SuccessMetric=exact_match, got %s", cfg.SuccessMetric)
	}
}
