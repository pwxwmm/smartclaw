package batch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/utils"
)

type BatchConfig struct {
	PromptsFile  string
	OutputDir    string
	Parallelism  int
	Model        string
	MaxTokens    int
	Timeout      time.Duration
	SystemPrompt string
}

func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		Parallelism: 4,
		Model:       "claude-sonnet-4-5",
		MaxTokens:   4096,
		Timeout:     120 * time.Second,
	}
}

type PromptItem struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	System  string `json:"system,omitempty"`
}

type BatchResult struct {
	ID        string        `json:"id"`
	Prompt    string        `json:"prompt"`
	Response  string        `json:"response"`
	Model     string        `json:"model"`
	Duration  time.Duration `json:"duration"`
	TokensIn  int           `json:"tokens_in"`
	TokensOut int           `json:"tokens_out"`
	Cost      float64       `json:"cost"`
	Error     string        `json:"error,omitempty"`
}

type BatchStats struct {
	Total     int
	Completed int
	Failed    int
	Skipped   int
	Duration  time.Duration
}

type Runner struct {
	config BatchConfig
	client *api.Client
	stats  atomic.Int64
	failed atomic.Int64
	total  int
}

func NewRunner(client *api.Client, cfg BatchConfig) *Runner {
	return &Runner{
		config: cfg,
		client: client,
	}
}

func (r *Runner) Run(ctx context.Context) (*BatchStats, error) {
	startTime := time.Now()

	prompts, err := r.loadPrompts()
	if err != nil {
		return nil, fmt.Errorf("batch: load prompts: %w", err)
	}
	r.total = len(prompts)

	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("batch: create output dir: %w", err)
	}

	slog.Info("batch: starting", "prompts", r.total, "parallelism", r.config.Parallelism, "model", r.config.Model)

	sem := make(chan struct{}, r.config.Parallelism)
	var wg sync.WaitGroup
	resultsCh := make(chan *BatchResult, r.config.Parallelism)

	writerDone := make(chan struct{})
	utils.Go(func() {
		defer close(writerDone)
		r.writeResults(resultsCh)
	})

	for i, prompt := range prompts {
		select {
		case <-ctx.Done():
			slog.Warn("batch: context cancelled", "completed", i, "remaining", r.total-i)
			break
		default:
		}

		sem <- struct{}{}
		wg.Add(1)

		p := prompt
		utils.Go(func() {
			defer wg.Done()
			defer func() { <-sem }()

			result := r.executePrompt(ctx, p)
			resultsCh <- result

			completed := r.stats.Add(1)
			if completed%10 == 0 || completed == int64(r.total) {
				slog.Info("batch: progress", "completed", completed, "total", r.total, "failed", r.failed.Load())
			}
		})
	}

	wg.Wait()
	close(resultsCh)
	<-writerDone

	stats := &BatchStats{
		Total:     r.total,
		Completed: int(r.stats.Load()),
		Failed:    int(r.failed.Load()),
		Duration:  time.Since(startTime),
	}

	slog.Info("batch: complete", "total", stats.Total, "completed", stats.Completed, "failed", stats.Failed, "duration", stats.Duration)

	return stats, nil
}

func (r *Runner) executePrompt(ctx context.Context, prompt PromptItem) *BatchResult {
	result := &BatchResult{
		ID:     prompt.ID,
		Prompt: prompt.Content,
		Model:  r.config.Model,
	}

	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	startTime := time.Now()

	system := prompt.System
	if system == "" {
		system = r.config.SystemPrompt
	}

	messages := []api.Message{
		{Role: "user", Content: prompt.Content},
	}

	var systemParam any
	if system != "" {
		systemParam = []api.SystemBlock{
			{
				Type:         "text",
				Text:         system,
				CacheControl: &api.CacheControl{Type: "ephemeral"},
			},
		}
	}

	resp, err := r.client.CreateMessageWithSystem(ctx, messages, systemParam)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Error = err.Error()
		r.failed.Add(1)
		return result
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	result.Response = content
	result.TokensIn = resp.Usage.InputTokens
	result.TokensOut = resp.Usage.OutputTokens
	result.Cost = runtime.CalculateCost(resp.Usage)

	return result
}

func (r *Runner) loadPrompts() ([]PromptItem, error) {
	file, err := os.Open(r.config.PromptsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var prompts []PromptItem
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		var item PromptItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			item = PromptItem{
				ID:      fmt.Sprintf("prompt_%d", lineNum),
				Content: line,
			}
		}

		if item.ID == "" {
			item.ID = fmt.Sprintf("prompt_%d", lineNum)
		}

		prompts = append(prompts, item)
	}

	return prompts, scanner.Err()
}

func (r *Runner) writeResults(ch chan *BatchResult) {
	resultsFile, err := os.Create(filepath.Join(r.config.OutputDir, "results.jsonl"))
	if err != nil {
		slog.Error("batch: failed to create results file", "error", err)
		return
	}
	defer resultsFile.Close()

	sharegptFile, err := os.Create(filepath.Join(r.config.OutputDir, "sharegpt.jsonl"))
	if err != nil {
		slog.Error("batch: failed to create sharegpt file", "error", err)
		return
	}
	defer sharegptFile.Close()

	resultsWriter := bufio.NewWriter(resultsFile)
	sharegptWriter := bufio.NewWriter(sharegptFile)

	for result := range ch {
		data, err := json.Marshal(result)
		if err != nil {
			slog.Warn("batch: failed to marshal result", "id", result.ID, "error", err)
			continue
		}
		resultsWriter.Write(data)
		resultsWriter.WriteByte('\n')

		if result.Error == "" {
			sharegpt := ShareGPTFormat{
				Conversations: []ShareGPTMessage{
					{From: "human", Value: result.Prompt},
					{From: "gpt", Value: result.Response},
				},
			}
			sgData, _ := json.Marshal(sharegpt)
			sharegptWriter.Write(sgData)
			sharegptWriter.WriteByte('\n')
		}
	}

	resultsWriter.Flush()
	sharegptWriter.Flush()
}

type ShareGPTFormat struct {
	Conversations []ShareGPTMessage `json:"conversations"`
}

type ShareGPTMessage struct {
	From  string `json:"from"`
	Value string `json:"value"`
}
