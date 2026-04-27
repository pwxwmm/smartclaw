package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

func TestShouldSpeculate(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		complexity float64
		want       bool
	}{
		{"short moderate query", "fix the bug", 0.3, true},
		{"too long", string(make([]byte, 600)), 0.3, false},
		{"too simple", "list files", 0.1, false},
		{"too complex", "architect a distributed system", 0.7, false},
		{"boundary low excluded", "hello", 0.199, false},
		{"boundary high excluded", "explain this", 0.501, false},
		{"just right", "refactor this function", 0.35, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldSpeculate(tt.query, tt.complexity); got != tt.want {
				t.Errorf("ShouldSpeculate(%q, %.2f) = %v, want %v", tt.query, tt.complexity, got, tt.want)
			}
		})
	}
}

func TestTextSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want float64
	}{
		{"identical", "hello world", "hello world", 1.0},
		{"empty both", "", "", 1.0},
		{"empty one", "hello", "", 0.0},
		{"partial overlap", "the quick brown fox", "the quick blue fox", 0.6},
		{"no overlap", "aaa bbb", "ccc ddd", 0.0},
		{"case insensitive", "Hello World", "hello world", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textSimilarity(tt.a, tt.b)
			if tt.want == 1.0 && got != 1.0 {
				t.Errorf("textSimilarity(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			} else if tt.want == 0.0 && got != 0.0 {
				t.Errorf("textSimilarity(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			} else if tt.want > 0 && tt.want < 1.0 {
				if got <= 0 || got >= 1.0 {
					t.Errorf("textSimilarity(%q, %q) = %v, want around %v", tt.a, tt.b, got, tt.want)
				}
			}
		})
	}
}

func makeSSEServer(model string, text string, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		resp := api.MessageResponse{
			ID:         "msg_" + model,
			Model:      model,
			Role:       "assistant",
			StopReason: "end_turn",
			Content: []api.ContentBlock{
				{Type: "text", Text: text},
			},
			Usage: api.Usage{InputTokens: 10, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSpeculativeExecutorSimilarResults(t *testing.T) {
	fastServer := makeSSEServer("haiku", "The function calculates the sum of two numbers.", 10*time.Millisecond)
	slowServer := makeSSEServer("sonnet", "The function calculates the sum of two numbers and returns it.", 50*time.Millisecond)
	defer fastServer.Close()
	defer slowServer.Close()

	fastClient := api.NewClientWithBaseURL("test-key", fastServer.URL)
	fastClient.Model = "haiku"

	slowClient := api.NewClientWithBaseURL("test-key", slowServer.URL)
	slowClient.Model = "sonnet"

	executor := NewSpeculativeExecutor(slowClient, fastClient)
	executor.SetEnabled(true)

	result, err := executor.Execute(context.Background(),
		[]api.Message{{Role: "user", Content: "explain this function"}},
		"You are helpful",
	)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.UsedModel != "fast" {
		t.Errorf("expected fast model for similar results, got %s (similarity=%.2f)", result.UsedModel, result.Similarity)
	}
	if result.FastResult == nil {
		t.Error("expected FastResult to be set")
	}
	if result.SlowResult == nil {
		t.Error("expected SlowResult to be set")
	}
	if result.Similarity <= 0 {
		t.Errorf("expected positive similarity, got %.2f", result.Similarity)
	}
}

func TestSpeculativeExecutorDifferentResults(t *testing.T) {
	fastServer := makeSSEServer("haiku", "aaa bbb ccc", 10*time.Millisecond)
	slowServer := makeSSEServer("sonnet", "xxx yyy zzz", 50*time.Millisecond)
	defer fastServer.Close()
	defer slowServer.Close()

	fastClient := api.NewClientWithBaseURL("test-key", fastServer.URL)

	slowClient := api.NewClientWithBaseURL("test-key", slowServer.URL)

	executor := NewSpeculativeExecutor(slowClient, fastClient)
	executor.SetEnabled(true)

	result, err := executor.Execute(context.Background(),
		[]api.Message{{Role: "user", Content: "test"}},
		"",
	)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.UsedModel != "slow" {
		t.Errorf("expected slow model for different results, got %s (similarity=%.2f)", result.UsedModel, result.Similarity)
	}
}

func TestSpeculativeExecutorFastError(t *testing.T) {
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	slowServer := makeSSEServer("sonnet", "Good response", 10*time.Millisecond)
	defer slowServer.Close()

	fastClient := api.NewClientWithBaseURL("test-key", errorServer.URL)

	slowClient := api.NewClientWithBaseURL("test-key", slowServer.URL)

	executor := NewSpeculativeExecutor(slowClient, fastClient)
	executor.SetEnabled(true)

	result, err := executor.Execute(context.Background(),
		[]api.Message{{Role: "user", Content: "test"}},
		"",
	)
	if err != nil {
		t.Fatalf("Execute should not fail when slow succeeds: %v", err)
	}
	if result.UsedModel != "slow" {
		t.Errorf("expected slow model fallback when fast fails, got %s", result.UsedModel)
	}
}

func TestSpeculativeExecutorNotEnabled(t *testing.T) {
	fastClient := api.NewClient("test-key")
	slowClient := api.NewClient("test-key")

	executor := NewSpeculativeExecutor(slowClient, fastClient)
	if executor.IsEnabled() {
		t.Error("expected executor to be disabled by default")
	}

	executor.SetEnabled(true)
	if !executor.IsEnabled() {
		t.Error("expected executor to be enabled after SetEnabled(true)")
	}
}

func TestExtractText(t *testing.T) {
	resp := &api.MessageResponse{
		Content: []api.ContentBlock{
			{Type: "thinking", Thinking: "internal"},
			{Type: "text", Text: "visible output"},
		},
	}
	text := extractText(resp)
	if text != "visible output" {
		t.Errorf("expected 'visible output', got %q", text)
	}

	if text := extractText(nil); text != "" {
		t.Errorf("expected empty string for nil, got %q", text)
	}
}
