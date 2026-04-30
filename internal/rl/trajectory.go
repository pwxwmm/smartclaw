package rl

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

type ToolCall struct {
	Name   string         `json:"name"`
	Input  map[string]any `json:"input"`
	Output string         `json:"output,omitempty"`
}

type ToolResult struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Success bool   `json:"success"`
}

type TrajectoryStep struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolResult *ToolResult    `json:"tool_result,omitempty"`
	Reward     float64        `json:"reward,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

type Trajectory struct {
	ID          string           `json:"id"`
	SessionID   string           `json:"session_id"`
	Task        string           `json:"task"`
	Steps       []TrajectoryStep `json:"steps"`
	Outcome     string           `json:"outcome"`
	TotalReward float64          `json:"total_reward"`
	CreatedAt   time.Time        `json:"created_at"`
	Compressed  bool             `json:"compressed"`
}

type RewardMetrics struct {
	ExactMatch   float64
	CodeQuality  float64
	LengthPenalty float64
	SuccessBonus float64
}

type TrajectoryCompressor struct {
	maxBytes        int
	preserveSystem  bool
	preserveTools   bool
	preserveRewards bool
	lastStats       *CompressionStats
}

const defaultMaxBytes = 64 * 1024

func NewTrajectoryCompressor() *TrajectoryCompressor {
	return &TrajectoryCompressor{
		maxBytes:        defaultMaxBytes,
		preserveSystem:  true,
		preserveTools:   true,
		preserveRewards: true,
	}
}

func (tc *TrajectoryCompressor) WithMaxBytes(n int) *TrajectoryCompressor {
	tc.maxBytes = n
	return tc
}

func (tc *TrajectoryCompressor) WithPreserveSystem(v bool) *TrajectoryCompressor {
	tc.preserveSystem = v
	return tc
}

func (tc *TrajectoryCompressor) WithPreserveTools(v bool) *TrajectoryCompressor {
	tc.preserveTools = v
	return tc
}

func (tc *TrajectoryCompressor) WithPreserveRewards(v bool) *TrajectoryCompressor {
	tc.preserveRewards = v
	return tc
}

func (tc *TrajectoryCompressor) LastCompressionStats() *CompressionStats {
	return tc.lastStats
}

// Compress compresses a trajectory to fit within maxBytes.
// Pipeline: size check → semanticDedup → truncateContent → removeLowQualitySteps → removeZeroRewardSteps → aggressiveTruncate → iterativeTruncate
func (tc *TrajectoryCompressor) Compress(traj *Trajectory) (*Trajectory, error) {
	compressed, _, err := tc.compressInternal(traj)
	return compressed, err
}

// CompressWithStats compresses a trajectory and returns compression statistics.
func (tc *TrajectoryCompressor) CompressWithStats(traj *Trajectory) (*Trajectory, *CompressionStats, error) {
	return tc.compressInternal(traj)
}

func (tc *TrajectoryCompressor) compressInternal(traj *Trajectory) (*Trajectory, *CompressionStats, error) {
	if traj == nil {
		return nil, nil, fmt.Errorf("rl: compress: trajectory is nil")
	}

	compressed := tc.cloneTrajectory(traj)
	originalStepCount := len(compressed.Steps)

	origData, _ := json.Marshal(compressed)
	originalBytes := len(origData)

	stats := &CompressionStats{
		OriginalBytes:     originalBytes,
		OriginalStepCount: originalStepCount,
		Timestamp:         time.Now(),
		Method:            "none",
	}

	data, err := json.Marshal(compressed)
	if err != nil {
		return nil, nil, fmt.Errorf("rl: compress: marshal: %w", err)
	}
	if len(data) <= tc.maxBytes {
		stats.CompressedBytes = len(data)
		stats.CompressedStepCount = len(compressed.Steps)
		stats.CompressionRatio = safeRatio(len(data), originalBytes)
		tc.lastStats = stats
		return compressed, stats, nil
	}

	compressed = tc.semanticDedup(compressed)
	stepsAfter := len(compressed.Steps)
	if stepsAfter < originalStepCount {
		stats.StepsMerged = originalStepCount - stepsAfter
		stats.Method = "semantic_dedup"
	}

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, nil, fmt.Errorf("rl: compress: marshal after semantic dedup: %w", err)
	}
	if len(data) <= tc.maxBytes {
		compressed.Compressed = true
		stats.CompressedBytes = len(data)
		stats.CompressedStepCount = len(compressed.Steps)
		stats.CompressionRatio = safeRatio(len(data), originalBytes)
		if stats.Method == "none" {
			stats.Method = "semantic_dedup"
		}
		tc.lastStats = stats
		return compressed, stats, nil
	}

	compressed = tc.truncateContent(compressed)
	if stats.Method == "none" || stats.Method == "semantic_dedup" {
		stats.Method = "truncate"
	}

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, nil, fmt.Errorf("rl: compress: marshal after truncate: %w", err)
	}
	if len(data) <= tc.maxBytes {
		compressed.Compressed = true
		stats.CompressedBytes = len(data)
		stats.CompressedStepCount = len(compressed.Steps)
		stats.CompressionRatio = safeRatio(len(data), originalBytes)
		tc.lastStats = stats
		return compressed, stats, nil
	}

	stepsBeforeQL := len(compressed.Steps)
	compressed = tc.removeLowQualitySteps(compressed)
	stepsRemovedQL := stepsBeforeQL - len(compressed.Steps)
	stats.StepsRemoved += stepsRemovedQL
	if stepsRemovedQL > 0 {
		stats.Method = "remove_zero_reward"
	}

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, nil, fmt.Errorf("rl: compress: marshal after remove low quality: %w", err)
	}
	if len(data) <= tc.maxBytes {
		compressed.Compressed = true
		stats.CompressedBytes = len(data)
		stats.CompressedStepCount = len(compressed.Steps)
		stats.CompressionRatio = safeRatio(len(data), originalBytes)
		tc.lastStats = stats
		return compressed, stats, nil
	}

	stepsBeforeZR := len(compressed.Steps)
	compressed = tc.removeZeroRewardSteps(compressed)
	stats.StepsRemoved += stepsBeforeZR - len(compressed.Steps)
	if stats.Method == "truncate" {
		stats.Method = "remove_zero_reward"
	}

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, nil, fmt.Errorf("rl: compress: marshal after remove zero reward: %w", err)
	}
	if len(data) <= tc.maxBytes {
		compressed.Compressed = true
		stats.CompressedBytes = len(data)
		stats.CompressedStepCount = len(compressed.Steps)
		stats.CompressionRatio = safeRatio(len(data), originalBytes)
		tc.lastStats = stats
		return compressed, stats, nil
	}

	compressed = tc.aggressiveTruncate(compressed)
	stats.Method = "aggressive"

	data, err = json.Marshal(compressed)
	if err != nil {
		return nil, nil, fmt.Errorf("rl: compress: marshal after aggressive truncate: %w", err)
	}

	if len(data) > tc.maxBytes {
		compressed = tc.iterativeTruncate(compressed)
		stats.Method = "iterative"
	}

	compressed.Compressed = true
	data, _ = json.Marshal(compressed)
	stats.CompressedBytes = len(data)
	stats.CompressedStepCount = len(compressed.Steps)
	stats.CompressionRatio = safeRatio(len(data), originalBytes)
	tc.lastStats = stats
	return compressed, stats, nil
}

func safeRatio(compressed, original int) float64 {
	if original == 0 {
		return 0
	}
	return float64(compressed) / float64(original)
}

// CompressBatch compresses multiple trajectories with diversity preservation.
// Removes exact-duplicate (same ID) and near-duplicate (same Task+Outcome+TotalReward within 0.01) trajectories,
// keeping the one with more steps. Then compresses each individually.
func (tc *TrajectoryCompressor) CompressBatch(trajectories []*Trajectory) ([]*Trajectory, *BatchCompressionStats, error) {
	if len(trajectories) == 0 {
		return nil, &BatchCompressionStats{}, nil
	}

	totalOriginal := len(trajectories)

	seenIDs := make(map[string]bool, len(trajectories))
	deduped := make([]*Trajectory, 0, len(trajectories))
	for _, t := range trajectories {
		if seenIDs[t.ID] {
			continue
		}
		seenIDs[t.ID] = true
		deduped = append(deduped, t)
	}

	// near-duplicate: same Task, same Outcome, TotalReward within 0.01
	type nearDedupKey struct {
		Task    string
		Outcome string
		Reward  int64
	}
	bestNear := make(map[nearDedupKey]*Trajectory)
	for _, t := range deduped {
		key := nearDedupKey{
			Task:    t.Task,
			Outcome: t.Outcome,
			Reward:  int64(math.Round(t.TotalReward * 100)),
		}
		if existing, ok := bestNear[key]; ok {
			if len(t.Steps) > len(existing.Steps) {
				bestNear[key] = t
			}
		} else {
			bestNear[key] = t
		}
	}

	final := make([]*Trajectory, 0, len(bestNear))
	for _, t := range bestNear {
		final = append(final, t)
	}

	duplicatesRemoved := totalOriginal - len(final)

	compressed := make([]*Trajectory, 0, len(final))
	totalRatio := 0.0
	for _, t := range final {
		c, err := tc.Compress(t)
		if err != nil {
			return nil, nil, fmt.Errorf("rl: compress batch: trajectory %s: %w", t.ID, err)
		}
		compressed = append(compressed, c)

		origData, _ := json.Marshal(t)
		compData, _ := json.Marshal(c)
		if len(origData) > 0 {
			totalRatio += float64(len(compData)) / float64(len(origData))
		}
	}

	avgRatio := 0.0
	if len(compressed) > 0 {
		avgRatio = totalRatio / float64(len(compressed))
	}

	batchStats := &BatchCompressionStats{
		TotalOriginal:       totalOriginal,
		TotalCompressed:     len(compressed),
		DuplicatesRemoved:   duplicatesRemoved,
		AvgCompressionRatio: avgRatio,
	}

	return compressed, batchStats, nil
}

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

const jaccardThreshold = 0.8

// semanticDedup merges consecutive steps with same role and Jaccard similarity > 0.8.
func (tc *TrajectoryCompressor) semanticDedup(traj *Trajectory) *Trajectory {
	if len(traj.Steps) <= 1 {
		return traj
	}

	merged := make([]TrajectoryStep, 0, len(traj.Steps))
	current := traj.Steps[0]

	for i := 1; i < len(traj.Steps); i++ {
		next := traj.Steps[i]
		if current.Role == next.Role && jaccardSimilarity(current.Content, next.Content) > jaccardThreshold {
			current = mergeSteps(current, next)
		} else {
			merged = append(merged, current)
			current = next
		}
	}
	merged = append(merged, current)

	traj.Steps = merged
	return traj
}

func mergeSteps(a, b TrajectoryStep) TrajectoryStep {
	result := TrajectoryStep{
		Role:      b.Role,
		Content:   b.Content,
		Reward:    a.Reward + b.Reward,
		Timestamp: b.Timestamp,
	}

	result.ToolCalls = make([]ToolCall, len(a.ToolCalls))
	copy(result.ToolCalls, a.ToolCalls)
	result.ToolCalls = append(result.ToolCalls, b.ToolCalls...)

	if b.ToolResult != nil {
		result.ToolResult = &ToolResult{
			Name:    b.ToolResult.Name,
			Content: b.ToolResult.Content,
			Success: b.ToolResult.Success,
		}
	} else if a.ToolResult != nil {
		result.ToolResult = &ToolResult{
			Name:    a.ToolResult.Name,
			Content: a.ToolResult.Content,
			Success: a.ToolResult.Success,
		}
	}

	if len(a.Metadata) > 0 || len(b.Metadata) > 0 {
		result.Metadata = make(map[string]any, len(a.Metadata)+len(b.Metadata))
		for k, v := range a.Metadata {
			result.Metadata[k] = v
		}
		for k, v := range b.Metadata {
			result.Metadata[k] = v
		}
	}

	return result
}

func jaccardSimilarity(a, b string) float64 {
	setA := wordSet(a)
	setB := wordSet(b)
	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}
	if len(setA) == 0 || len(setB) == 0 {
		return 0.0
	}

	intersection := 0
	for word := range setA {
		if setB[word] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	return float64(intersection) / float64(union)
}

func wordSet(s string) map[string]bool {
	words := strings.Fields(s)
	set := make(map[string]bool, len(words))
	for _, w := range words {
		if w != "" {
			set[w] = true
		}
	}
	return set
}

var codeIndicators = []string{"func ", "def ", "class ", "import ", "```", "var ", "const ", "let ", "fn ", "pub "}

func hasCodeIndicators(content string) bool {
	for _, ind := range codeIndicators {
		if strings.Contains(content, ind) {
			return true
		}
	}
	return false
}

// removeLowQualitySteps removes negative-reward steps, then zero-reward steps with no tool calls and no code.
func (tc *TrajectoryCompressor) removeLowQualitySteps(traj *Trajectory) *Trajectory {
	filtered := make([]TrajectoryStep, 0, len(traj.Steps))
	for _, s := range traj.Steps {
		if tc.preserveSystem && s.Role == "system" {
			filtered = append(filtered, s)
			continue
		}
		if s.Reward < 0 {
			continue
		}
		if s.Reward == 0 && len(s.ToolCalls) == 0 && !hasCodeIndicators(s.Content) {
			if tc.preserveTools && s.ToolResult != nil {
				filtered = append(filtered, s)
				continue
			}
			continue
		}
		filtered = append(filtered, s)
	}
	traj.Steps = filtered
	return traj
}

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

var roleToShareGPT = map[string]string{
	"system":    "system",
	"user":      "human",
	"assistant": "gpt",
	"tool":      "tool",
	"function":  "tool",
	"human":     "human",
	"gpt":       "gpt",
}

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
			"from":  from,
			"value": value,
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

	if tc.lastStats != nil {
		result[0]["compression_stats"] = map[string]any{
			"original_size":   tc.lastStats.OriginalBytes,
			"compressed_size": tc.lastStats.CompressedBytes,
			"method":          tc.lastStats.Method,
		}
	}

	return result, nil
}

// ExportOpenAIFineTuning exports a trajectory in OpenAI fine-tuning format.
func (tc *TrajectoryCompressor) ExportOpenAIFineTuning(traj *Trajectory) (map[string]any, error) {
	if traj == nil {
		return nil, fmt.Errorf("rl: export openai: trajectory is nil")
	}

	messages := make([]map[string]any, 0, len(traj.Steps))

	for _, s := range traj.Steps {
		msg := map[string]any{
			"role":    s.Role,
			"content": s.Content,
		}

		if len(s.ToolCalls) > 0 {
			openAIToolCalls := make([]map[string]any, 0, len(s.ToolCalls))
			for _, tc2 := range s.ToolCalls {
				argsBytes, err := json.Marshal(tc2.Input)
				if err != nil {
					return nil, fmt.Errorf("rl: export openai: marshal tool_call args: %w", err)
				}
				openAIToolCalls = append(openAIToolCalls, map[string]any{
					"type": "function",
					"function": map[string]any{
						"name":      tc2.Name,
						"arguments": string(argsBytes),
					},
				})
			}
			msg["tool_calls"] = openAIToolCalls
		}

		if s.ToolResult != nil {
			msg["role"] = "tool"
			msg["content"] = s.ToolResult.Content
			msg["tool_call_id"] = s.ToolResult.Name
		}

		messages = append(messages, msg)
	}

	result := map[string]any{
		"messages": messages,
		"metadata": map[string]any{
			"trajectory_id": traj.ID,
			"outcome":       traj.Outcome,
			"total_reward":  traj.TotalReward,
			"compressed":    traj.Compressed,
		},
	}

	if tc.lastStats != nil {
		result["metadata"].(map[string]any)["compression_stats"] = map[string]any{
			"original_size":   tc.lastStats.OriginalBytes,
			"compressed_size": tc.lastStats.CompressedBytes,
			"method":          tc.lastStats.Method,
		}
	}

	return result, nil
}

func (tc *TrajectoryCompressor) ExportJSONL(trajectories []*Trajectory, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	for _, traj := range trajectories {
		if err := encoder.Encode(traj); err != nil {
			return fmt.Errorf("rl: export jsonl: %w", err)
		}
	}
	return nil
}

func (tc *TrajectoryCompressor) ImportShareGPT(data []byte) (*Trajectory, error) {
	var raw struct {
		Conversations []struct {
			From  string `json:"from"`
			Value string `json:"value"`
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
