package batch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

func newMockBatchClient(t *testing.T, handler http.Handler) *api.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := api.NewClientWithBaseURL("test-key", server.URL)
	client.Model = "test-model"
	return client
}

func mockBatchHandler(responses []string) http.Handler {
	i := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := textBatchResponse("default response")
		if i < len(responses) {
			resp = textBatchResponse(responses[i])
		}
		i++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
}

func textBatchResponse(text string) api.MessageResponse {
	return api.MessageResponse{
		ID:    "msg_batch",
		Type:  "message",
		Role:  "assistant",
		Model: "test-model",
		Content: []api.ContentBlock{
			{Type: "text", Text: text},
		},
		StopReason: "end_turn",
		Usage:      api.Usage{InputTokens: 50, OutputTokens: 100},
	}
}

func writePromptsFile(t *testing.T, dir string, items []PromptItem) string {
	t.Helper()
	path := filepath.Join(dir, "prompts.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create prompts file: %v", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, item := range items {
		data, _ := json.Marshal(item)
		w.Write(data)
		w.WriteByte('\n')
	}
	w.Flush()
	return path
}

func TestNewRunner(t *testing.T) {
	handler := mockBatchHandler([]string{"hello"})
	client := newMockBatchClient(t, handler)

	cfg := DefaultBatchConfig()
	runner := NewRunner(client, cfg)

	if runner == nil {
		t.Fatal("NewRunner returned nil")
	}
	if runner.config.Parallelism != 4 {
		t.Errorf("expected Parallelism=4, got %d", runner.config.Parallelism)
	}
	if runner.config.Model != "claude-sonnet-4-5" {
		t.Errorf("expected default model, got %s", runner.config.Model)
	}
}

func TestRunner_SinglePrompt(t *testing.T) {
	handler := mockBatchHandler([]string{"Hello, world!"})
	client := newMockBatchClient(t, handler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	promptsFile := writePromptsFile(t, tmpDir, []PromptItem{
		{ID: "p1", Content: "Say hello"},
	})

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 1,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     30 * time.Second,
	}

	runner := NewRunner(client, cfg)
	stats, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.Total != 1 {
		t.Errorf("expected Total=1, got %d", stats.Total)
	}
	if stats.Completed != 1 {
		t.Errorf("expected Completed=1, got %d", stats.Completed)
	}
	if stats.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", stats.Failed)
	}

	resultsFile := filepath.Join(outputDir, "results.jsonl")
	if _, err := os.Stat(resultsFile); os.IsNotExist(err) {
		t.Error("results.jsonl should exist")
	}

	data, err := os.ReadFile(resultsFile)
	if err != nil {
		t.Fatalf("read results: %v", err)
	}

	var result BatchResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.ID != "p1" {
		t.Errorf("expected ID=p1, got %s", result.ID)
	}
	if result.Response == "" {
		t.Error("response should not be empty")
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestRunner_MultiplePrompts(t *testing.T) {
	responses := []string{"Response 1", "Response 2", "Response 3"}
	handler := mockBatchHandler(responses)
	client := newMockBatchClient(t, handler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	promptsFile := writePromptsFile(t, tmpDir, []PromptItem{
		{ID: "p1", Content: "Prompt 1"},
		{ID: "p2", Content: "Prompt 2"},
		{ID: "p3", Content: "Prompt 3"},
	})

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 2,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     30 * time.Second,
	}

	runner := NewRunner(client, cfg)
	stats, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("expected Total=3, got %d", stats.Total)
	}
	if stats.Completed != 3 {
		t.Errorf("expected Completed=3, got %d", stats.Completed)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("read results: %v", err)
	}

	lines := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		if scanner.Text() != "" {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("expected 3 result lines, got %d", lines)
	}

	sharegptData, err := os.ReadFile(filepath.Join(outputDir, "sharegpt.jsonl"))
	if err != nil {
		t.Fatalf("read sharegpt: %v", err)
	}

	sharegptLines := 0
	scanner = bufio.NewScanner(strings.NewReader(string(sharegptData)))
	for scanner.Scan() {
		if scanner.Text() != "" {
			sharegptLines++
		}
	}
	if sharegptLines != 3 {
		t.Errorf("expected 3 sharegpt lines, got %d", sharegptLines)
	}
}

func TestRunner_ContextCancellation(t *testing.T) {
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(textBatchResponse("slow"))
	})
	client := newMockBatchClient(t, slowHandler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	items := make([]PromptItem, 10)
	for i := range items {
		items[i] = PromptItem{ID: fmt.Sprintf("p%d", i), Content: fmt.Sprintf("Prompt %d", i)}
	}
	promptsFile := writePromptsFile(t, tmpDir, items)

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 2,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     30 * time.Second,
	}

	runner := NewRunner(client, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	stats, err := runner.Run(ctx)
	if err != nil {
		t.Logf("Run returned error (expected on cancel): %v", err)
	}
	if stats == nil {
		t.Fatal("stats should not be nil even on cancel")
	}
	t.Logf("Completed %d of %d before cancel", stats.Completed, stats.Total)
}

func TestRunner_OutputDirAutoCreated(t *testing.T) {
	handler := mockBatchHandler([]string{"hello"})
	client := newMockBatchClient(t, handler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "nested", "deep", "output")

	promptsFile := writePromptsFile(t, tmpDir, []PromptItem{
		{ID: "p1", Content: "Say hello"},
	})

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 1,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     10 * time.Second,
	}

	runner := NewRunner(client, cfg)
	_, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("output directory should be auto-created")
	}
}

func TestRunner_InvalidPromptsFile(t *testing.T) {
	handler := mockBatchHandler([]string{"hello"})
	client := newMockBatchClient(t, handler)

	tmpDir := t.TempDir()

	cfg := BatchConfig{
		PromptsFile: filepath.Join(tmpDir, "nonexistent.jsonl"),
		OutputDir:   filepath.Join(tmpDir, "output"),
		Parallelism: 1,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     10 * time.Second,
	}

	runner := NewRunner(client, cfg)
	_, err := runner.Run(context.Background())
	if err == nil {
		t.Error("expected error for missing prompts file")
	}
}

func TestRunner_LLMError(t *testing.T) {
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, `{"error": "server error"}`)
	})
	client := newMockBatchClient(t, errorHandler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	promptsFile := writePromptsFile(t, tmpDir, []PromptItem{
		{ID: "p1", Content: "Will fail"},
	})

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 1,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     10 * time.Second,
	}

	runner := NewRunner(client, cfg)
	stats, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run should not return error for LLM failures: %v", err)
	}

	if stats.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", stats.Failed)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("read results: %v", err)
	}

	var result BatchResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.Error == "" {
		t.Error("result should have an error message")
	}
}

func TestRunner_SystemPromptOverride(t *testing.T) {
	handler := mockBatchHandler([]string{"response"})
	client := newMockBatchClient(t, handler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	promptsFile := writePromptsFile(t, tmpDir, []PromptItem{
		{ID: "p1", Content: "Say hi", System: "Custom system prompt"},
	})

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 1,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     10 * time.Second,
	}

	runner := NewRunner(client, cfg)
	stats, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if stats.Completed != 1 {
		t.Errorf("expected Completed=1, got %d", stats.Completed)
	}
}

func TestDefaultBatchConfig(t *testing.T) {
	cfg := DefaultBatchConfig()
	if cfg.Parallelism != 4 {
		t.Errorf("expected Parallelism=4, got %d", cfg.Parallelism)
	}
	if cfg.Model != "claude-sonnet-4-5" {
		t.Errorf("expected Model=claude-sonnet-4-5, got %s", cfg.Model)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected MaxTokens=4096, got %d", cfg.MaxTokens)
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("expected Timeout=120s, got %v", cfg.Timeout)
	}
}

func TestRunner_EmptyPromptsFile(t *testing.T) {
	handler := mockBatchHandler([]string{"hello"})
	client := newMockBatchClient(t, handler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	promptsFile := filepath.Join(tmpDir, "empty.jsonl")
	if err := os.WriteFile(promptsFile, []byte(""), 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 1,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     10 * time.Second,
	}

	runner := NewRunner(client, cfg)
	stats, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if stats.Total != 0 {
		t.Errorf("expected Total=0 for empty file, got %d", stats.Total)
	}
}

func TestRunner_PlainTextPrompts(t *testing.T) {
	handler := mockBatchHandler([]string{"response"})
	client := newMockBatchClient(t, handler)

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	promptsFile := filepath.Join(tmpDir, "prompts.jsonl")

	content := "Just a plain text prompt\nAnother plain text line\n"
	if err := os.WriteFile(promptsFile, []byte(content), 0644); err != nil {
		t.Fatalf("write prompts: %v", err)
	}

	cfg := BatchConfig{
		PromptsFile: promptsFile,
		OutputDir:   outputDir,
		Parallelism: 1,
		Model:       "test-model",
		MaxTokens:   1024,
		Timeout:     10 * time.Second,
	}

	runner := NewRunner(client, cfg)
	stats, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("expected Total=2, got %d", stats.Total)
	}
	if stats.Completed != 2 {
		t.Errorf("expected Completed=2, got %d", stats.Completed)
	}
}
