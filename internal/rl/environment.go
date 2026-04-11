package rl

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

type EnvironmentConfig struct {
	TaskType      string
	MaxSteps      int
	Timeout       time.Duration
	SuccessMetric string
}

func DefaultEnvironmentConfig() EnvironmentConfig {
	return EnvironmentConfig{
		MaxSteps:      10,
		Timeout:       300 * time.Second,
		SuccessMetric: "exact_match",
	}
}

type StepResult struct {
	Step        int            `json:"step"`
	Action      string         `json:"action"`
	Observation string         `json:"observation"`
	Reward      float64        `json:"reward"`
	Done        bool           `json:"done"`
	Info        map[string]any `json:"info,omitempty"`
}

type EpisodeResult struct {
	Steps       []StepResult  `json:"steps"`
	TotalReward float64       `json:"total_reward"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Model       string        `json:"model"`
}

type Environment struct {
	config    EnvironmentConfig
	client    *api.Client
	rewards   RewardFunction
	obsParser ObservationParser
}

func NewEnvironment(client *api.Client, cfg EnvironmentConfig) *Environment {
	return &Environment{
		config:    cfg,
		client:    client,
		rewards:   GetRewardFunction(cfg.SuccessMetric),
		obsParser: &DefaultObservationParser{},
	}
}

func (e *Environment) RunEpisode(ctx context.Context, taskPrompt string) (*EpisodeResult, error) {
	startTime := time.Now()

	epCtx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	episode := &EpisodeResult{
		Steps: make([]StepResult, 0, e.config.MaxSteps),
		Model: e.client.Model,
	}

	messages := []api.Message{
		{Role: "user", Content: taskPrompt},
	}

	totalReward := 0.0

	for step := 0; step < e.config.MaxSteps; step++ {
		select {
		case <-epCtx.Done():
			episode.Duration = time.Since(startTime)
			episode.TotalReward = totalReward
			return episode, epCtx.Err()
		default:
		}

		resp, err := e.client.CreateMessage(messages, buildTaskSystemPrompt(e.config.TaskType))
		if err != nil {
			return nil, fmt.Errorf("rl: step %d: %w", step, err)
		}

		var response string
		for _, block := range resp.Content {
			if block.Type == "text" {
				response += block.Text
			}
		}

		messages = append(messages, api.Message{Role: "assistant", Content: response})

		obs := e.obsParser.Parse(response)
		reward := e.rewards.Compute(taskPrompt, response, step, obs)
		totalReward += reward

		done := e.rewards.IsDone(taskPrompt, response, step, obs)

		stepResult := StepResult{
			Step:        step,
			Action:      response,
			Observation: obs.Text,
			Reward:      reward,
			Done:        done,
			Info: map[string]any{
				"tokens_in":  resp.Usage.InputTokens,
				"tokens_out": resp.Usage.OutputTokens,
			},
		}

		episode.Steps = append(episode.Steps, stepResult)

		if done {
			break
		}

		feedback := e.rewards.Feedback(reward, step)
		if feedback != "" {
			messages = append(messages, api.Message{Role: "user", Content: feedback})
		}
	}

	episode.Duration = time.Since(startTime)
	episode.TotalReward = totalReward
	episode.Success = totalReward > 0.5

	return episode, nil
}

func (e *Environment) SetRewardFunction(fn RewardFunction) {
	e.rewards = fn
}

type RewardFunction interface {
	Compute(task, response string, step int, obs *Observation) float64
	IsDone(task, response string, step int, obs *Observation) bool
	Feedback(reward float64, step int) string
	Name() string
}

type Observation struct {
	Text      string
	HasCode   bool
	HasError  bool
	ToolCalls int
}

type ObservationParser interface {
	Parse(response string) *Observation
}

type DefaultObservationParser struct{}

func (p *DefaultObservationParser) Parse(response string) *Observation {
	obs := &Observation{Text: response}
	codeIndicators := []string{"func ", "def ", "class ", "import ", "```"}
	for _, ind := range codeIndicators {
		if contains(response, ind) {
			obs.HasCode = true
			break
		}
	}
	errorIndicators := []string{"error:", "Error:", "failed:", "panic:", "exception"}
	for _, ind := range errorIndicators {
		if contains(response, ind) {
			obs.HasError = true
			break
		}
	}
	return obs
}

type ExactMatchReward struct{}

func (r *ExactMatchReward) Compute(task, response string, step int, obs *Observation) float64 {
	if obs.HasError {
		return -0.1
	}
	if step == 0 && obs.HasCode {
		return 0.5
	}
	return 0.1
}

func (r *ExactMatchReward) IsDone(task, response string, step int, obs *Observation) bool {
	return obs.HasCode && !obs.HasError || step >= 5
}

func (r *ExactMatchReward) Feedback(reward float64, step int) string {
	if reward < 0 {
		return "The last step produced an error. Please fix it."
	}
	return ""
}

func (r *ExactMatchReward) Name() string { return "exact_match" }

type CodeQualityReward struct{}

func (r *CodeQualityReward) Compute(task, response string, step int, obs *Observation) float64 {
	reward := 0.0
	if obs.HasCode {
		reward += 0.3
	}
	if obs.HasError {
		reward -= 0.2
	}
	reward -= float64(step) * 0.05
	return math.Max(reward, -1.0)
}

func (r *CodeQualityReward) IsDone(task, response string, step int, obs *Observation) bool {
	return step >= 8
}

func (r *CodeQualityReward) Feedback(reward float64, step int) string {
	if reward < 0 {
		return "Try a different approach. The current code has issues."
	}
	if reward > 0.2 {
		return "Good progress. Continue refining."
	}
	return ""
}

func (r *CodeQualityReward) Name() string { return "code_quality" }

type LengthPenaltyReward struct{}

func (r *LengthPenaltyReward) Compute(task, response string, step int, obs *Observation) float64 {
	reward := 1.0
	if len(response) > 5000 {
		reward -= 0.3
	}
	if obs.HasError {
		reward -= 0.5
	}
	reward -= float64(step) * 0.1
	return math.Max(reward, -1.0)
}

func (r *LengthPenaltyReward) IsDone(task, response string, step int, obs *Observation) bool {
	return step >= 5
}

func (r *LengthPenaltyReward) Feedback(reward float64, step int) string {
	return ""
}

func (r *LengthPenaltyReward) Name() string { return "length_penalty" }

var rewardRegistry = map[string]RewardFunction{
	"exact_match":    &ExactMatchReward{},
	"code_quality":   &CodeQualityReward{},
	"length_penalty": &LengthPenaltyReward{},
}

func GetRewardFunction(name string) RewardFunction {
	if fn, ok := rewardRegistry[name]; ok {
		return fn
	}
	return &ExactMatchReward{}
}

func RegisterRewardFunction(name string, fn RewardFunction) {
	rewardRegistry[name] = fn
}

type TrajectoryExporter struct {
	outputDir string
	mu        sync.Mutex
}

func NewTrajectoryExporter(outputDir string) *TrajectoryExporter {
	return &TrajectoryExporter{outputDir: outputDir}
}

func (te *TrajectoryExporter) Export(episode *EpisodeResult, taskID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	if err := os.MkdirAll(te.outputDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(te.outputDir, fmt.Sprintf("%s_%d.json", taskID, time.Now().UnixNano()))
	data, err := json.MarshalIndent(episode, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func buildTaskSystemPrompt(taskType string) string {
	return "You are an AI assistant performing a coding task. Complete the task step by step."
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
