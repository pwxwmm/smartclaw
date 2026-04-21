package rl

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"
)

// ToolCall represents a single tool invocation within a trajectory step.
type ToolCall struct {
	Name   string         `json:"name"`
	Input  map[string]any `json:"input"`
	Output string         `json:"output,omitempty"`
}

// ToolResult represents the result returned from a tool execution.
type ToolResult struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Success bool   `json:"success"`
}

// TrajectoryStep represents a single step in an RL trajectory.
type TrajectoryStep struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolResult *ToolResult    `json:"tool_result,omitempty"`
	Reward     float64        `json:"reward,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Trajectory represents a complete RL episode with all steps and metadata.
type Trajectory struct {
	ID          string           `json:"id"`
	SessionID   string           `json:"session_id"`
	Task        string           `json:"task"`
	Steps       []TrajectoryStep `json:"steps"`
	Outcome     string           `json:"outcome"` // "success", "failure", "partial"
	TotalReward float64          `json:"total_reward"`
	CreatedAt   time.Time        `json:"created_at"`
	Compressed  bool             `json:"compressed"`
}

// RewardMetrics configures weights for computing trajectory reward.
type RewardMetrics struct {
	ExactMatch   float64 // weight for exact match (0-1)
	CodeQuality  float64 // weight for code quality (0-1)
	LengthPenalty float64 // weight for length penalty (0-1)
	SuccessBonus float64 // bonus for successful outcome
}

// TrajectoryCompressor compresses trajectories to fit within a byte budget
// and exports them in ShareGPT-compatible format for RL training.
type TrajectoryCompressor struct {
	maxBytes        int  // default: 64KB
	preserveSystem  bool // default: true
	preserveTools   bool // default: true
	preserveRewards bool // default: true
}

const defaultMaxBytes = 64 * 1024 // 64KB

// NewTrajectoryCompressor creates a compressor with 64KB default budget.
func NewTrajectoryCompressor() *TrajectoryCompressor {
	return &TrajectoryCompressor{
		maxBytes:        defaultMaxBytes,
		preserveSystem:  true,
		preserveTools:   true,
		preserveRewards: true,
	}
}

// WithMaxBytes sets a custom byte budget.
func (tc *TrajectoryCompressor) WithMaxBytes(n int) *TrajectoryCompressor {
	tc.maxBytes = n
	return tc
}

// WithPreserveSystem sets whether system messages are preserved during compression.
func (tc *TrajectoryCompressor) WithPreserveSystem(v bool) *TrajectoryCompressor {
	tc.preserveSystem = v
	return tc
}

// WithPreserveTools sets whether tool calls are preserved during compression.
func (tc *TrajectoryCompressor) WithPreserveTools(v bool) *TrajectoryCompressor {
	tc.preserveTools = v
	return tc
}

// WithPreserveRewards sets whether reward-bearing steps are preserved during compression.
func (tc *TrajectoryCompressor) WithPreserveRewards(v bool) *TrajectoryCompressor {
	tc.preserveRewards = v
	return tc
}

// Compress compresses a trajectory to fit within maxBytes.
// Strategy: truncate long content, remove low-reward steps, preserve key data.
func (tc *TrajectoryCompressor) Compress(traj *Trajectory) (*Trajectory, error) {
	if traj == nil {
		return nil, fmt.Errorf("rl: compress: trajectory is nil")
	}

	compressed := tc.cloneTrajectory(traj)

	data, err := json.Marshal(compressed)
	if err != nil {
		return nil, fmt.Errorf("rl: compress: marshal: %w", err)
	}
	if len(data) <= tc.maxBytes {
		return compressed, nil
	}

	compressed = tc.truncateContent(compressed)

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, fmt.Errorf("rl: compress: marshal after truncate: %w", err)
	}
	if len(data) <= tc.maxBytes {
		compressed.Compressed = true
		return compressed, nil
	}

	compressed = tc.removeZeroRewardSteps(compressed)

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, fmt.Errorf("rl: compress: marshal after remove zero reward: %w", err)
	}
	if len(data) <= tc.maxBytes {
		compressed.Compressed = true
		return compressed, nil
	}

	compressed = tc.aggressiveTruncate(compressed)

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, fmt.Errorf("rl: compress: marshal after aggressive truncate: %w", err)
	}

	if len(data) > tc.maxBytes {
		compressed = tc.iterativeTruncate(compressed)
	}

	compressed.Compressed = true
	return compressed, nil
}

// cloneTrajectory creates a deep copy of a trajectory.
func (tc *TrajectoryCompressor) cloneTrajectory(traj *Trajectory) *Trajectory {
	clone := &Trajectory{
		ID:          traj.ID,
		SessionID:   traj.SessionID,
		Task:        traj.Task,
		Outcome:     traj.Outcome,
		TotalReward: traj.TotalReward,
		CreatedAt:   traj.CreatedAt,
		Compressed:  traj.Compressed,
	}
	clone.Steps = make([]TrajectoryStep, len(traj.Steps))
	for i, s := range traj.Steps {
		clone.Steps[i] = tc.cloneStep(s)
	}
	return clone
}

func (tc *TrajectoryCompressor) cloneStep(s TrajectoryStep) TrajectoryStep {
	clone := TrajectoryStep{
		Role:      s.Role,
		Content:   s.Content,
		Reward:    s.Reward,
		Timestamp: s.Timestamp,
	}
	if len(s.ToolCalls) > 0 {
		clone.ToolCalls = make([]ToolCall, len(s.ToolCalls))
		for j, tc2 := range s.ToolCalls {
			clone.ToolCalls[j] = ToolCall{
				Name:   tc2.Name,
				Output: tc2.Output,
			}
			if len(tc2.Input) > 0 {
				clone.ToolCalls[j].Input = make(map[string]any, len(tc2.Input))
				for k, v := range tc2.Input {
					clone.ToolCalls[j].Input[k] = v
				}
			}
		}
	}
	if s.ToolResult != nil {
		clone.ToolResult = &ToolResult{
			Name:    s.ToolResult.Name,
			Content: s.ToolResult.Content,
			Success: s.ToolResult.Success,
		}
	}
	if len(s.Metadata) > 0 {
		clone.Metadata = make(map[string]any, len(s.Metadata))
		for k, v := range s.Metadata {
			clone.Metadata[k] = v
		}
	}
	return clone
}

const truncateHeadSize = 2000
const truncateTailSize = 2000
const truncateMarker = "\n...[truncated]...\n"

// truncateContent truncates long content strings to head+tail.
func (tc *TrajectoryCompressor) truncateContent(traj *Trajectory) *Trajectory {
	for i := range traj.Steps {
		s := &traj.Steps[i]
		maxLen := truncateHeadSize + truncateTailSize + len(truncateMarker)
		if len(s.Content) > maxLen {
			s.Content = s.Content[:truncateHeadSize] + truncateMarker + s.Content[len(s.Content)-truncateTailSize:]
		}
		if s.ToolResult != nil && len(s.ToolResult.Content) > maxLen {
			s.ToolResult.Content = s.ToolResult.Content[:truncateHeadSize] + truncateMarker + s.ToolResult.Content[len(s.ToolResult.Content)-truncateTailSize:]
		}
		for j := range s.ToolCalls {
			if len(s.ToolCalls[j].Output) > maxLen {
				s.ToolCalls[j].Output = s.ToolCalls[j].Output[:truncateHeadSize] + truncateMarker + s.ToolCalls[j].Output[len(s.ToolCalls[j].Output)-truncateTailSize:]
			}
		}
	}
	return traj
}

// removeZeroRewardSteps removes steps with reward=0, respecting preserve flags.
func (tc *TrajectoryCompressor) removeZeroRewardSteps(traj *Trajectory) *Trajectory {
	filtered := make([]TrajectoryStep, 0, len(traj.Steps))
	for _, s := range traj.Steps {
		if tc.preserveSystem && s.Role == "system" {
			filtered = append(filtered, s)
			continue
		}
		if tc.preserveTools && (len(s.ToolCalls) > 0 || s.ToolResult != nil) {
			filtered = append(filtered, s)
			continue
		}
		if tc.preserveRewards && s.Reward != 0 {
			filtered = append(filtered, s)
			continue
		}
		if s.Reward == 0 {
			continue
		}
		filtered = append(filtered, s)
	}
	traj.Steps = filtered
	return traj
}

// aggressiveTruncate cuts content down further to fit the budget.
func (tc *TrajectoryCompressor) aggressiveTruncate(traj *Trajectory) *Trajectory {
	for i := range traj.Steps {
		s := &traj.Steps[i]
		if len(s.Content) > 500 {
			s.Content = s.Content[:500] + "\n...[truncated]..."
		}
		if s.ToolResult != nil && len(s.ToolResult.Content) > 500 {
			s.ToolResult.Content = s.ToolResult.Content[:500] + "\n...[truncated]..."
		}
		for j := range s.ToolCalls {
			if len(s.ToolCalls[j].Output) > 500 {
				s.ToolCalls[j].Output = s.ToolCalls[j].Output[:500] + "\n...[truncated]..."
			}
		}
	}
	return traj
}

// iterativeTruncate progressively reduces content until the trajectory fits within maxBytes.
func (tc *TrajectoryCompressor) iterativeTruncate(traj *Trajectory) *Trajectory {
	maxContentLen := 250
	for {
		for i := range traj.Steps {
			s := &traj.Steps[i]
			if len(s.Content) > maxContentLen {
				s.Content = s.Content[:maxContentLen] + "..."
			}
			if s.ToolResult != nil && len(s.ToolResult.Content) > maxContentLen {
				s.ToolResult.Content = s.ToolResult.Content[:maxContentLen] + "..."
			}
			for j := range s.ToolCalls {
				if len(s.ToolCalls[j].Output) > maxContentLen {
					s.ToolCalls[j].Output = s.ToolCalls[j].Output[:maxContentLen] + "..."
				}
			}
		}

		data, err := json.Marshal(traj)
		if err != nil || len(data) <= tc.maxBytes {
			break
		}

		maxContentLen /= 2
		if maxContentLen < 10 {
			break
		}
	}
	return traj
}

// roleToShareGPT maps internal roles to ShareGPT format roles.
var roleToShareGPT = map[string]string{
	"system":      "system",
	"user":        "human",
	"assistant":   "gpt",
	"tool":        "tool",
	"function":    "tool",
	"human":       "human",
	"gpt":         "gpt",
}

// ExportShareGPT exports a trajectory in ShareGPT-compatible format.
func (tc *TrajectoryCompressor) ExportShareGPT(traj *Trajectory) ([]map[string]any, error) {
	if traj == nil {
		return nil, fmt.Errorf("rl: export sharegpt: trajectory is nil")
	}

	conversations := make([]map[string]any, 0, len(traj.Steps))

	for _, s := range traj.Steps {
		from, ok := roleToShareGPT[s.Role]
		if !ok {
			from = s.Role
		}

		value := s.Content
		if len(s.ToolCalls) > 0 {
			tcJSON, err := json.Marshal(s.ToolCalls)
			if err != nil {
				return nil, fmt.Errorf("rl: export sharegpt: marshal tool_calls: %w", err)
			}
			if value != "" {
				value += "\n"
			}
			value += string(tcJSON)
		}
		if s.ToolResult != nil {
			if value != "" {
				value += "\n"
			}
			value += s.ToolResult.Content
		}

		entry := map[string]any{
			"from":   from,
			"value":  value,
		}
		conversations = append(conversations, entry)
	}

	result := []map[string]any{
		{
			"conversations": conversations,
			"id":            traj.ID,
			"outcome":       traj.Outcome,
			"total_reward":  traj.TotalReward,
		},
	}

	return result, nil
}

// ExportJSONL writes trajectories as one JSON object per line to writer.
func (tc *TrajectoryCompressor) ExportJSONL(trajectories []*Trajectory, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	for _, traj := range trajectories {
		if err := encoder.Encode(traj); err != nil {
			return fmt.Errorf("rl: export jsonl: %w", err)
		}
	}
	return nil
}

// ImportShareGPT parses ShareGPT format data into a Trajectory.
func (tc *TrajectoryCompressor) ImportShareGPT(data []byte) (*Trajectory, error) {
	var raw struct {
		Conversations []struct {
			From   string `json:"from"`
			Value  string `json:"value"`
		} `json:"conversations"`
		ID          string  `json:"id"`
		Outcome     string  `json:"outcome"`
		TotalReward float64 `json:"total_reward"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("rl: import sharegpt: unmarshal: %w", err)
	}

	shareGPTToRole := map[string]string{
		"system": "system",
		"human":  "user",
		"gpt":    "assistant",
		"tool":   "tool",
	}

	traj := &Trajectory{
		ID:          raw.ID,
		Outcome:     raw.Outcome,
		TotalReward: raw.TotalReward,
		CreatedAt:   time.Now(),
		Steps:       make([]TrajectoryStep, 0, len(raw.Conversations)),
	}

	for i, conv := range raw.Conversations {
		role, ok := shareGPTToRole[conv.From]
		if !ok {
			role = conv.From
		}
		traj.Steps = append(traj.Steps, TrajectoryStep{
			Role:      role,
			Content:   conv.Value,
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	return traj, nil
}

// CalculateReward computes total reward for a trajectory using the given metrics.
func (tc *TrajectoryCompressor) CalculateReward(traj *Trajectory, metrics RewardMetrics) float64 {
	if traj == nil || len(traj.Steps) == 0 {
		return 0
	}

	totalReward := 0.0
	stepCount := 0

	for _, s := range traj.Steps {
		stepReward := 0.0

	if s.Reward > 0 {
		stepReward += metrics.ExactMatch * s.Reward
	}

	if s.Metadata != nil {
		if quality, ok := s.Metadata["code_quality"]; ok {
			if q, ok := quality.(float64); ok {
				stepReward += metrics.CodeQuality * q
			}
		}
	}

	contentLen := len(s.Content)
	if contentLen > 5000 {
		penalty := float64(contentLen) / 50000.0
		stepReward -= metrics.LengthPenalty * math.Min(penalty, 1.0)
	}

		totalReward += stepReward
		stepCount++
	}

	if traj.Outcome == "success" {
		totalReward += metrics.SuccessBonus
	}

	if stepCount > 0 {
		totalReward /= float64(stepCount)
	}

	return totalReward
}
