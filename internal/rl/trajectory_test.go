package rl

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewTrajectoryCompressor(t *testing.T) {
	tc := NewTrajectoryCompressor()
	if tc.maxBytes != 64*1024 {
		t.Errorf("expected maxBytes=65536, got %d", tc.maxBytes)
	}
	if !tc.preserveSystem {
		t.Error("expected preserveSystem=true")
	}
	if !tc.preserveTools {
		t.Error("expected preserveTools=true")
	}
	if !tc.preserveRewards {
		t.Error("expected preserveRewards=true")
	}
}

func TestCompressWithinBudget(t *testing.T) {
	tc := NewTrajectoryCompressor()
	traj := &Trajectory{
		ID:        "test-1",
		SessionID: "sess-1",
		Task:      "small task",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "You are a helpful assistant.", Timestamp: time.Now()},
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "Hi there!", Reward: 0.5, Timestamp: time.Now()},
		},
		Outcome:     "success",
		TotalReward: 0.5,
		CreatedAt:   time.Now(),
	}

	compressed, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if compressed.Compressed {
		t.Error("should not be marked compressed when within budget")
	}
	if len(compressed.Steps) != len(traj.Steps) {
		t.Error("steps should not be removed when within budget")
	}
	if compressed.Steps[1].Content != "Hello" {
		t.Error("content should not be truncated when within budget")
	}
}

func TestCompressOverBudget(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(500)

	longContent := strings.Repeat("x", 10000)
	traj := &Trajectory{
		ID:        "test-big",
		SessionID: "sess-big",
		Task:      "big task",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "System prompt.", Timestamp: time.Now()},
			{Role: "user", Content: longContent, Reward: 0, Timestamp: time.Now()},
			{Role: "assistant", Content: longContent, Reward: 0.3, Timestamp: time.Now()},
		},
		Outcome:     "partial",
		TotalReward: 0.3,
		CreatedAt:   time.Now(),
	}

	compressed, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !compressed.Compressed {
		t.Error("should be marked compressed when over budget")
	}

	data, _ := json.Marshal(compressed)
	if len(data) > tc.maxBytes {
		t.Errorf("compressed size %d exceeds budget %d", len(data), tc.maxBytes)
	}

	if compressed.Steps[0].Role != "system" {
		t.Error("system message should be preserved")
	}
}

func TestCompressTruncatesLongContent(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(2000)

	longContent := strings.Repeat("a", 10000)
	traj := &Trajectory{
		ID:        "trunc-1",
		SessionID: "sess-trunc",
		Task:      "truncation test",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: longContent, Reward: 1.0, Timestamp: time.Now()},
		},
		Outcome:     "success",
		TotalReward: 1.0,
		CreatedAt:   time.Now(),
	}

	compressed, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if compressed.Steps[0].Content == longContent {
		t.Error("long content should have been truncated")
	}
	if !strings.Contains(compressed.Steps[0].Content, "[truncated]") {
		t.Error("truncated content should contain truncation marker")
	}
}

func TestCompressNilTrajectory(t *testing.T) {
	tc := NewTrajectoryCompressor()
	_, err := tc.Compress(nil)
	if err == nil {
		t.Error("expected error for nil trajectory")
	}
}

func TestCompressRemovesZeroRewardSteps(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(300)

	traj := &Trajectory{
		ID:        "zero-reward",
		SessionID: "sess-zr",
		Task:      "test",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "sys", Timestamp: time.Now()},
			{Role: "user", Content: "msg1", Reward: 0, Timestamp: time.Now()},
			{Role: "user", Content: "msg2", Reward: 0, Timestamp: time.Now()},
			{Role: "assistant", Content: "msg3", Reward: 0.5, Timestamp: time.Now()},
		},
		Outcome:     "partial",
		TotalReward: 0.5,
		CreatedAt:   time.Now(),
	}

	compressed, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasSystem := false
	for _, s := range compressed.Steps {
		if s.Role == "system" {
			hasSystem = true
		}
		if s.Reward == 0 && s.Role != "system" {
			t.Error("zero-reward non-system steps should be removed when over budget")
		}
	}
	if !hasSystem {
		t.Error("system message should be preserved")
	}
}

func TestExportShareGPT(t *testing.T) {
	tc := NewTrajectoryCompressor()
	traj := &Trajectory{
		ID:        "sharegpt-1",
		SessionID: "sess-sg",
		Task:      "test",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "You are helpful.", Timestamp: time.Now()},
			{Role: "user", Content: "Write a function", Timestamp: time.Now()},
			{Role: "assistant", Content: "func hello() {}", ToolCalls: []ToolCall{{Name: "bash", Input: map[string]any{"cmd": "echo hi"}, Output: "hi"}}, Timestamp: time.Now()},
			{Role: "tool", Content: "", ToolResult: &ToolResult{Name: "bash", Content: "hi", Success: true}, Timestamp: time.Now()},
		},
		Outcome:     "success",
		TotalReward: 0.8,
		CreatedAt:   time.Now(),
	}

	result, err := tc.ExportShareGPT(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	convs, ok := result[0]["conversations"].([]map[string]any)
	if !ok {
		t.Fatal("conversations should be []map[string]any")
	}

	if len(convs) != 4 {
		t.Fatalf("expected 4 conversations, got %d", len(convs))
	}

	roleMap := map[string]string{
		"system":    "system",
		"user":      "human",
		"assistant": "gpt",
		"tool":      "tool",
	}
	for i, conv := range convs {
		from, _ := conv["from"].(string)
		expectedRole := roleMap[traj.Steps[i].Role]
		if from != expectedRole {
			t.Errorf("step %d: expected from=%s, got %s", i, expectedRole, from)
		}
	}

	if result[0]["id"] != "sharegpt-1" {
		t.Error("id should be preserved")
	}
	if result[0]["outcome"] != "success" {
		t.Error("outcome should be preserved")
	}
}

func TestExportShareGPTNilTrajectory(t *testing.T) {
	tc := NewTrajectoryCompressor()
	_, err := tc.ExportShareGPT(nil)
	if err == nil {
		t.Error("expected error for nil trajectory")
	}
}

func TestExportJSONL(t *testing.T) {
	tc := NewTrajectoryCompressor()
	trajectories := []*Trajectory{
		{
			ID: "jsonl-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "hi", Timestamp: time.Now()}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: time.Now(),
		},
		{
			ID: "jsonl-2", SessionID: "s2", Task: "t2",
			Steps: []TrajectoryStep{{Role: "user", Content: "bye", Timestamp: time.Now()}},
			Outcome: "failure", TotalReward: 0.1, CreatedAt: time.Now(),
		},
	}

	var buf bytes.Buffer
	err := tc.ExportJSONL(trajectories, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var t1, t2 Trajectory
	if err := json.Unmarshal([]byte(lines[0]), &t1); err != nil {
		t.Fatalf("line 1 unmarshal: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &t2); err != nil {
		t.Fatalf("line 2 unmarshal: %v", err)
	}

	if t1.ID != "jsonl-1" {
		t.Errorf("expected ID jsonl-1, got %s", t1.ID)
	}
	if t2.ID != "jsonl-2" {
		t.Errorf("expected ID jsonl-2, got %s", t2.ID)
	}
}

func TestImportShareGPTRoundtrip(t *testing.T) {
	tc := NewTrajectoryCompressor()

	sharegptData := []byte(`{
		"conversations": [
			{"from": "system", "value": "You are helpful."},
			{"from": "human", "value": "Write code"},
			{"from": "gpt", "value": "func main() {}"},
			{"from": "tool", "value": "output here"}
		],
		"id": "rt-1",
		"outcome": "success",
		"total_reward": 0.9
	}`)

	traj, err := tc.ImportShareGPT(sharegptData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if traj.ID != "rt-1" {
		t.Errorf("expected ID rt-1, got %s", traj.ID)
	}
	if traj.Outcome != "success" {
		t.Errorf("expected outcome success, got %s", traj.Outcome)
	}
	if traj.TotalReward != 0.9 {
		t.Errorf("expected total_reward 0.9, got %f", traj.TotalReward)
	}
	if len(traj.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(traj.Steps))
	}

	expectedRoles := []string{"system", "user", "assistant", "tool"}
	expectedContents := []string{"You are helpful.", "Write code", "func main() {}", "output here"}
	for i, s := range traj.Steps {
		if s.Role != expectedRoles[i] {
			t.Errorf("step %d: expected role %s, got %s", i, expectedRoles[i], s.Role)
		}
		if s.Content != expectedContents[i] {
			t.Errorf("step %d: expected content %q, got %q", i, expectedContents[i], s.Content)
		}
	}
}

func TestImportShareGPTInvalidJSON(t *testing.T) {
	tc := NewTrajectoryCompressor()
	_, err := tc.ImportShareGPT([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCalculateReward(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:      "reward-1",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "task", Reward: 0.5, Timestamp: time.Now()},
			{Role: "assistant", Content: "response", Reward: 0.8, Metadata: map[string]any{"code_quality": 0.7}, Timestamp: time.Now()},
		},
		TotalReward: 1.3,
		CreatedAt:   time.Now(),
	}

	metrics := RewardMetrics{
		ExactMatch:   0.4,
		CodeQuality:  0.3,
		LengthPenalty: 0.2,
		SuccessBonus: 0.5,
	}

	reward := tc.CalculateReward(traj, metrics)
	if reward <= 0 {
		t.Errorf("expected positive reward, got %f", reward)
	}
}

func TestCalculateRewardWithLengthPenalty(t *testing.T) {
	tc := NewTrajectoryCompressor()

	longContent := strings.Repeat("x", 10000)
	traj := &Trajectory{
		ID:      "reward-long",
		Outcome: "failure",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: longContent, Reward: 0.1, Timestamp: time.Now()},
		},
		TotalReward: 0.1,
		CreatedAt:   time.Now(),
	}

	metrics := RewardMetrics{
		ExactMatch:   0.3,
		CodeQuality:  0.3,
		LengthPenalty: 0.5,
		SuccessBonus: 1.0,
	}

	reward := tc.CalculateReward(traj, metrics)
	if reward >= 0 {
		t.Errorf("expected negative reward due to length penalty, got %f", reward)
	}
}

func TestCalculateRewardNilTrajectory(t *testing.T) {
	tc := NewTrajectoryCompressor()
	metrics := RewardMetrics{ExactMatch: 1.0, SuccessBonus: 1.0}
	reward := tc.CalculateReward(nil, metrics)
	if reward != 0 {
		t.Errorf("expected 0 reward for nil trajectory, got %f", reward)
	}
}

func TestPreserveSystemFlag(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(200).WithPreserveSystem(true)
	traj := &Trajectory{
		ID:      "preserve-sys",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "System instructions here", Reward: 0, Timestamp: time.Now()},
			{Role: "user", Content: "User message", Reward: 0, Timestamp: time.Now()},
		},
		TotalReward: 0,
		CreatedAt:   time.Now(),
	}

	compressed, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasSystem := false
	for _, s := range compressed.Steps {
		if s.Role == "system" {
			hasSystem = true
		}
	}
	if !hasSystem {
		t.Error("system message should be preserved when preserveSystem=true")
	}
}

func TestPreserveToolsFlag(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(300).WithPreserveTools(true)
	traj := &Trajectory{
		ID:      "preserve-tools",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "Using tool", Reward: 0, ToolCalls: []ToolCall{{Name: "bash", Input: map[string]any{"cmd": "ls"}, Output: "file.txt"}}, Timestamp: time.Now()},
			{Role: "tool", Content: "", Reward: 0, ToolResult: &ToolResult{Name: "bash", Content: "file.txt", Success: true}, Timestamp: time.Now()},
		},
		TotalReward: 0,
		CreatedAt:   time.Now(),
	}

	compressed, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasToolCall := false
	hasToolResult := false
	for _, s := range compressed.Steps {
		if len(s.ToolCalls) > 0 {
			hasToolCall = true
		}
		if s.ToolResult != nil {
			hasToolResult = true
		}
	}
	if !hasToolCall {
		t.Error("tool call steps should be preserved when preserveTools=true")
	}
	if !hasToolResult {
		t.Error("tool result steps should be preserved when preserveTools=true")
	}
}

func TestPreserveRewardsFlag(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(300).WithPreserveRewards(true)
	traj := &Trajectory{
		ID:      "preserve-rewards",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "Good answer", Reward: 0.8, Timestamp: time.Now()},
			{Role: "user", Content: "Meh question", Reward: 0, Timestamp: time.Now()},
		},
		TotalReward: 0.8,
		CreatedAt:   time.Now(),
	}

	compressed, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range compressed.Steps {
		if s.Reward > 0 && s.Content != "Good answer" {
			t.Error("non-zero reward steps should be preserved when preserveRewards=true")
		}
	}
}

func TestWithMaxBytes(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(1024)
	if tc.maxBytes != 1024 {
		t.Errorf("expected maxBytes=1024, got %d", tc.maxBytes)
	}
}

func TestCompressDoesNotMutateOriginal(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(200)
	originalContent := strings.Repeat("a", 5000)

	traj := &Trajectory{
		ID:      "no-mutate",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: originalContent, Reward: 1.0, Timestamp: time.Now()},
		},
		TotalReward: 1.0,
		CreatedAt:   time.Now(),
	}

	_, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if traj.Steps[0].Content != originalContent {
		t.Error("original trajectory should not be mutated by Compress")
	}
	if traj.Compressed {
		t.Error("original trajectory Compressed flag should not be set")
	}
}

func TestExportOpenAIFineTuning(t *testing.T) {
	tc := NewTrajectoryCompressor()
	traj := &Trajectory{
		ID:        "openai-1",
		SessionID: "sess-oai",
		Task:      "test",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "You are helpful.", Timestamp: time.Now()},
			{Role: "user", Content: "Write a function", Timestamp: time.Now()},
			{Role: "assistant", Content: "func hello() {}", Timestamp: time.Now()},
		},
		Outcome:     "success",
		TotalReward: 0.8,
		CreatedAt:   time.Now(),
	}

	result, err := tc.ExportOpenAIFineTuning(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages, ok := result["messages"].([]map[string]any)
	if !ok {
		t.Fatal("messages should be []map[string]any")
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	expectedRoles := []string{"system", "user", "assistant"}
	for i, msg := range messages {
		role, _ := msg["role"].(string)
		if role != expectedRoles[i] {
			t.Errorf("message %d: expected role %s, got %s", i, expectedRoles[i], role)
		}
	}

	content, _ := messages[2]["content"].(string)
	if content != "func hello() {}" {
		t.Errorf("expected content 'func hello() {}', got %q", content)
	}

	metadata, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be map[string]any")
	}
	if metadata["trajectory_id"] != "openai-1" {
		t.Error("metadata should contain trajectory_id")
	}
	if metadata["outcome"] != "success" {
		t.Error("metadata should contain outcome")
	}
	if metadata["total_reward"] != 0.8 {
		t.Error("metadata should contain total_reward")
	}
}

func TestExportOpenAIFineTuningWithToolCalls(t *testing.T) {
	tc := NewTrajectoryCompressor()
	traj := &Trajectory{
		ID:        "openai-tools-1",
		SessionID: "sess-oai-tools",
		Task:      "test",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "You are helpful.", Timestamp: time.Now()},
			{Role: "user", Content: "List files", Timestamp: time.Now()},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{
				{Name: "bash", Input: map[string]any{"cmd": "ls -la"}, Output: "file1.txt\nfile2.txt"},
			}, Timestamp: time.Now()},
			{Role: "tool", Content: "", ToolResult: &ToolResult{Name: "bash", Content: "file1.txt\nfile2.txt", Success: true}, Timestamp: time.Now()},
			{Role: "assistant", Content: "Here are the files: file1.txt, file2.txt", Timestamp: time.Now()},
		},
		Outcome:     "success",
		TotalReward: 0.9,
		CreatedAt:   time.Now(),
	}

	result, err := tc.ExportOpenAIFineTuning(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages, ok := result["messages"].([]map[string]any)
	if !ok {
		t.Fatal("messages should be []map[string]any")
	}
	if len(messages) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(messages))
	}

	assistantMsg := messages[2]
	toolCalls, ok := assistantMsg["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatal("assistant message with tool calls should have tool_calls field")
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0]["type"] != "function" {
		t.Error("tool_call type should be 'function'")
	}
	fn, _ := toolCalls[0]["function"].(map[string]any)
	if fn["name"] != "bash" {
		t.Error("tool_call function name should be 'bash'")
	}
	argsStr, _ := fn["arguments"].(string)
	var args map[string]any
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		t.Fatalf("arguments should be valid JSON: %v", err)
	}
	if args["cmd"] != "ls -la" {
		t.Error("arguments should contain cmd=ls -la")
	}

	toolMsg := messages[3]
	if toolMsg["role"] != "tool" {
		t.Error("tool result message should have role 'tool'")
	}
	if toolMsg["tool_call_id"] != "bash" {
		t.Error("tool result message should have tool_call_id='bash'")
	}
	if toolMsg["content"] != "file1.txt\nfile2.txt" {
		t.Error("tool result message content should match ToolResult.Content")
	}
}

func TestExportOpenAIFineTuningNilTrajectory(t *testing.T) {
	tc := NewTrajectoryCompressor()
	_, err := tc.ExportOpenAIFineTuning(nil)
	if err == nil {
		t.Error("expected error for nil trajectory")
	}
}

func TestLastCompressionStatsRemoved_UseCompressWithStats(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:      "stats-1",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "hi", Reward: 0.5, Timestamp: time.Now()},
		},
		TotalReward: 0.5,
		CreatedAt:   time.Now(),
	}

	_, stats, err := tc.CompressWithStats(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats from CompressWithStats")
	}
	if stats.OriginalBytes <= 0 {
		t.Error("OriginalBytes should be positive")
	}
	if stats.CompressedBytes <= 0 {
		t.Error("CompressedBytes should be positive")
	}
	if stats.CompressionRatio <= 0 || stats.CompressionRatio > 1.0 {
		t.Errorf("CompressionRatio should be in (0,1] for within-budget, got %f", stats.CompressionRatio)
	}
	if stats.OriginalStepCount != 2 {
		t.Errorf("OriginalStepCount should be 2, got %d", stats.OriginalStepCount)
	}
	if stats.Method == "" {
		t.Error("Method should not be empty")
	}
}

func TestCompressWithStats(t *testing.T) {
	tc := NewTrajectoryCompressor()
	traj := &Trajectory{
		ID:      "cws-1",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "hi", Reward: 0.5, Timestamp: time.Now()},
		},
		TotalReward: 0.5,
		CreatedAt:   time.Now(),
	}

	compressed, stats, err := tc.CompressWithStats(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if compressed == nil {
		t.Fatal("compressed trajectory should not be nil")
	}
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.OriginalStepCount != 2 {
		t.Errorf("OriginalStepCount should be 2, got %d", stats.OriginalStepCount)
	}
	if stats.CompressedStepCount != 2 {
		t.Errorf("CompressedStepCount should be 2, got %d", stats.CompressedStepCount)
	}
}

func TestCompressWithStatsNilTrajectory(t *testing.T) {
	tc := NewTrajectoryCompressor()
	_, _, err := tc.CompressWithStats(nil)
	if err == nil {
		t.Error("expected error for nil trajectory")
	}
}

func TestExportShareGPTNoCompressionStatsByDefault(t *testing.T) {
	tc := NewTrajectoryCompressor()
	traj := &Trajectory{
		ID:      "sg-nostats-1",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
		},
		TotalReward: 0.5,
		CreatedAt:   time.Now(),
	}

	result, err := tc.ExportShareGPT(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result[0]["compression_stats"]; ok {
		t.Error("compression_stats should not be present in ExportShareGPT by default")
	}
}

func TestExportShareGPTWithCompressionStats(t *testing.T) {
	tc := NewTrajectoryCompressor()
	traj := &Trajectory{
		ID:      "sg-stats-1",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
		},
		TotalReward: 0.5,
		CreatedAt:   time.Now(),
	}

	_, _ = tc.Compress(traj)

	result, err := tc.ExportShareGPT(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cs, ok := result[0]["compression_stats"].(map[string]any)
	if !ok {
		t.Fatal("compression_stats should be present after compress")
	}
	if _, ok := cs["original_size"]; !ok {
		t.Error("compression_stats should contain original_size")
	}
	if _, ok := cs["compressed_size"]; !ok {
		t.Error("compression_stats should contain compressed_size")
	}
	if _, ok := cs["method"]; !ok {
		t.Error("compression_stats should contain method")
	}
}

func TestLastCompressionStats(t *testing.T) {
	tc := NewTrajectoryCompressor()
	if tc.LastCompressionStats() != nil {
		t.Error("expected nil stats before any compression")
	}

	traj := &Trajectory{
		ID:      "stats-direct-1",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "hi", Reward: 0.5, Timestamp: time.Now()},
		},
		TotalReward: 0.5,
		CreatedAt:   time.Now(),
	}

	_, err := tc.Compress(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := tc.LastCompressionStats()
	if stats == nil {
		t.Fatal("expected non-nil stats after Compress")
	}
	if stats.OriginalStepCount != 2 {
		t.Errorf("OriginalStepCount should be 2, got %d", stats.OriginalStepCount)
	}
	if stats.Method == "" {
		t.Error("Method should not be empty")
	}
}

func TestExportPipelineShareGPT(t *testing.T) {
	tc := NewTrajectoryCompressor()
	dir := t.TempDir()
	ep := NewExportPipeline(tc, dir, "sharegpt")

	trajectories := []*Trajectory{
		{
			ID: "pipe-sg-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "hi", Timestamp: time.Now()}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: time.Now(),
		},
	}

	result, err := ep.ProcessAndExport(context.Background(), trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalInput != 1 {
		t.Errorf("TotalInput should be 1, got %d", result.TotalInput)
	}
	if result.FilesWritten != 1 {
		t.Errorf("FilesWritten should be 1, got %d", result.FilesWritten)
	}
	if _, err := os.Stat(filepath.Join(dir, "pipe-sg-1.json")); os.IsNotExist(err) {
		t.Error("expected file pipe-sg-1.json to exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "_manifest.json")); os.IsNotExist(err) {
		t.Error("expected _manifest.json to exist")
	}
}

func TestExportPipelineOpenAI(t *testing.T) {
	tc := NewTrajectoryCompressor()
	dir := t.TempDir()
	ep := NewExportPipeline(tc, dir, "openai")

	trajectories := []*Trajectory{
		{
			ID: "pipe-oai-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "hello", Timestamp: time.Now()}},
			Outcome: "success", TotalReward: 0.7, CreatedAt: time.Now(),
		},
	}

	result, err := ep.ProcessAndExport(context.Background(), trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FilesWritten != 1 {
		t.Errorf("FilesWritten should be 1, got %d", result.FilesWritten)
	}
	if _, err := os.Stat(filepath.Join(dir, "pipe-oai-1.json")); os.IsNotExist(err) {
		t.Error("expected file pipe-oai-1.json to exist")
	}

	data, err := os.ReadFile(filepath.Join(dir, "pipe-oai-1.json"))
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	var exported map[string]any
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("unmarshal exported: %v", err)
	}
	if _, ok := exported["messages"]; !ok {
		t.Error("OpenAI export should have 'messages' key")
	}
	if _, ok := exported["metadata"]; !ok {
		t.Error("OpenAI export should have 'metadata' key")
	}
}

func TestExportPipelineJSONL(t *testing.T) {
	tc := NewTrajectoryCompressor()
	dir := t.TempDir()
	ep := NewExportPipeline(tc, dir, "jsonl")

	trajectories := []*Trajectory{
		{
			ID: "pipe-jsonl-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "hi", Timestamp: time.Now()}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: time.Now(),
		},
	}

	result, err := ep.ProcessAndExport(context.Background(), trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FilesWritten != 1 {
		t.Errorf("FilesWritten should be 1, got %d", result.FilesWritten)
	}
	if _, err := os.Stat(filepath.Join(dir, "pipe-jsonl-1.jsonl")); os.IsNotExist(err) {
		t.Error("expected file pipe-jsonl-1.jsonl to exist")
	}

	data, err := os.ReadFile(filepath.Join(dir, "pipe-jsonl-1.jsonl"))
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	var traj Trajectory
	if err := json.Unmarshal(data, &traj); err != nil {
		t.Fatalf("unmarshal jsonl: %v", err)
	}
	if traj.ID != "pipe-jsonl-1" {
		t.Errorf("expected ID pipe-jsonl-1, got %s", traj.ID)
	}
}

func TestExportPipelineDeduplication(t *testing.T) {
	tc := NewTrajectoryCompressor()
	dir := t.TempDir()
	ep := NewExportPipeline(tc, dir, "sharegpt")

	trajectories := []*Trajectory{
		{
			ID: "dup-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "first", Timestamp: time.Now()}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: time.Now(),
		},
		{
			ID: "dup-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "duplicate", Timestamp: time.Now()}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: time.Now(),
		},
		{
			ID: "dup-2", SessionID: "s2", Task: "t2",
			Steps: []TrajectoryStep{{Role: "user", Content: "unique", Timestamp: time.Now()}},
			Outcome: "failure", TotalReward: 0.1, CreatedAt: time.Now(),
		},
	}

	result, err := ep.ProcessAndExport(context.Background(), trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalInput != 3 {
		t.Errorf("TotalInput should be 3, got %d", result.TotalInput)
	}
	if result.DuplicatesRemoved != 1 {
		t.Errorf("DuplicatesRemoved should be 1, got %d", result.DuplicatesRemoved)
	}
	if result.FilesWritten != 2 {
		t.Errorf("FilesWritten should be 2, got %d", result.FilesWritten)
	}
}

func TestExportPipelineContextCancellation(t *testing.T) {
	tc := NewTrajectoryCompressor()
	dir := t.TempDir()
	ep := NewExportPipeline(tc, dir, "sharegpt")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	trajectories := []*Trajectory{
		{ID: "ctx-1", Outcome: "success",
			Steps: []TrajectoryStep{{Role: "user", Content: "hi", Timestamp: time.Now()}},
			CreatedAt: time.Now()},
	}

	_, err := ep.ProcessAndExport(ctx, trajectories)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestExportPipelineManifest(t *testing.T) {
	tc := NewTrajectoryCompressor()
	dir := t.TempDir()
	ep := NewExportPipeline(tc, dir, "sharegpt")

	trajectories := []*Trajectory{
		{ID: "manifest-1", Outcome: "success",
			Steps: []TrajectoryStep{{Role: "user", Content: "hi", Timestamp: time.Now()}},
			CreatedAt: time.Now()},
	}

	_, err := ep.ProcessAndExport(context.Background(), trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	manifestPath := filepath.Join(dir, "_manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest PipelineResult
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest.TotalInput != 1 {
		t.Errorf("manifest TotalInput should be 1, got %d", manifest.TotalInput)
	}
	if manifest.FilesWritten != 1 {
		t.Errorf("manifest FilesWritten should be 1, got %d", manifest.FilesWritten)
	}
}

func TestSemanticDedup(t *testing.T) {
	tc := NewTrajectoryCompressor()
	now := time.Now()

	traj := &Trajectory{
		ID:      "dedup-1",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "I will help you write a function to sort an array of numbers in Go", Reward: 0.1, Timestamp: now},
			{Role: "assistant", Content: "I will help you write a function to sort an array of strings in Go", Reward: 0.2, Timestamp: now.Add(time.Second)},
			{Role: "user", Content: "Thanks", Reward: 0, Timestamp: now.Add(2 * time.Second)},
		},
		TotalReward: 0.3,
		CreatedAt:   now,
	}

	deduped := tc.semanticDedup(traj)
	if len(deduped.Steps) != 2 {
		t.Fatalf("expected 2 steps after dedup, got %d", len(deduped.Steps))
	}

	merged := deduped.Steps[0]
	if merged.Role != "assistant" {
		t.Errorf("merged step role should be assistant, got %s", merged.Role)
	}
	if math.Abs(merged.Reward-0.3) > 0.01 {
		t.Errorf("merged reward should be 0.3 (0.1+0.2), got %f", merged.Reward)
	}
	if merged.Content != "I will help you write a function to sort an array of strings in Go" {
		t.Errorf("merged content should be from the last step, got %q", merged.Content)
	}
	if !merged.Timestamp.Equal(now.Add(time.Second)) {
		t.Error("merged timestamp should be from the last step")
	}
}

func TestSemanticDedupNoMergeDifferentRoles(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:      "dedup-roles",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "I will help you write a function", Timestamp: time.Now()},
			{Role: "assistant", Content: "I will help you write a function", Timestamp: time.Now()},
		},
		TotalReward: 0,
		CreatedAt:   time.Now(),
	}

	deduped := tc.semanticDedup(traj)
	if len(deduped.Steps) != 2 {
		t.Errorf("steps with different roles should not be merged, got %d steps", len(deduped.Steps))
	}
}

func TestSemanticDedupNoMergeLowSimilarity(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:      "dedup-low-sim",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "The quick brown fox jumps over the lazy dog", Timestamp: time.Now()},
			{Role: "assistant", Content: "Implement a binary search tree with insert and delete", Timestamp: time.Now()},
		},
		TotalReward: 0,
		CreatedAt:   time.Now(),
	}

	deduped := tc.semanticDedup(traj)
	if len(deduped.Steps) != 2 {
		t.Errorf("steps with low similarity should not be merged, got %d steps", len(deduped.Steps))
	}
}

func TestSemanticDedupWithToolCalls(t *testing.T) {
	tc := NewTrajectoryCompressor()
	now := time.Now()

	traj := &Trajectory{
		ID:      "dedup-tools",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "Let me check the files in the directory", Reward: 0.1, ToolCalls: []ToolCall{{Name: "bash", Input: map[string]any{"cmd": "ls"}, Output: "a.txt"}}, Timestamp: now},
			{Role: "assistant", Content: "Let me check the files in the directory now", Reward: 0.2, ToolCalls: []ToolCall{{Name: "bash", Input: map[string]any{"cmd": "ls -la"}, Output: "a.txt b.txt"}}, Timestamp: now.Add(time.Second)},
		},
		TotalReward: 0.3,
		CreatedAt:   now,
	}

	deduped := tc.semanticDedup(traj)
	if len(deduped.Steps) != 1 {
		t.Fatalf("expected 1 merged step, got %d", len(deduped.Steps))
	}

	merged := deduped.Steps[0]
	if len(merged.ToolCalls) != 2 {
		t.Errorf("merged step should have 2 tool calls, got %d", len(merged.ToolCalls))
	}
	if math.Abs(merged.Reward-0.3) > 0.01 {
		t.Errorf("merged reward should be 0.3, got %f", merged.Reward)
	}
}

func TestSemanticDedupSingleStep(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:      "dedup-single",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
		},
		TotalReward: 0,
		CreatedAt:   time.Now(),
	}

	deduped := tc.semanticDedup(traj)
	if len(deduped.Steps) != 1 {
		t.Errorf("single step should not be affected, got %d steps", len(deduped.Steps))
	}
}

func TestSemanticDedupEmptySteps(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:        "dedup-empty",
		Outcome:   "success",
		Steps:     []TrajectoryStep{},
		CreatedAt: time.Now(),
	}

	deduped := tc.semanticDedup(traj)
	if len(deduped.Steps) != 0 {
		t.Errorf("empty steps should remain empty, got %d steps", len(deduped.Steps))
	}
}

func TestRemoveLowQualitySteps(t *testing.T) {
	tc := NewTrajectoryCompressor()
	now := time.Now()

	traj := &Trajectory{
		ID:      "low-quality",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "You are helpful", Reward: 0, Timestamp: now},
			{Role: "assistant", Content: "Bad response", Reward: -0.5, Timestamp: now.Add(time.Second)},
			{Role: "assistant", Content: "Empty filler text", Reward: 0, Timestamp: now.Add(2 * time.Second)},
			{Role: "assistant", Content: "Good response", Reward: 0.8, Timestamp: now.Add(3 * time.Second)},
		},
		TotalReward: 0.3,
		CreatedAt:   now,
	}

	filtered := tc.removeLowQualitySteps(traj)

	if len(filtered.Steps) != 2 {
		t.Fatalf("expected 2 steps (removed negative reward and zero-reward no-code), got %d", len(filtered.Steps))
	}

	hasSystem := false
	for _, s := range filtered.Steps {
		if s.Role == "system" {
			hasSystem = true
		}
		if s.Reward < 0 {
			t.Error("negative reward steps should be removed")
		}
	}
	if !hasSystem {
		t.Error("system step should be preserved")
	}
}

func TestRemoveLowQualityStepsPreservesCodeSteps(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:      "low-quality-code",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "func main() {}", Reward: 0, Timestamp: time.Now()},
			{Role: "assistant", Content: "Plain text without code", Reward: 0, Timestamp: time.Now()},
		},
		TotalReward: 0,
		CreatedAt:   time.Now(),
	}

	filtered := tc.removeLowQualitySteps(traj)
	if len(filtered.Steps) != 1 {
		t.Fatalf("expected 1 step (zero-reward code kept, zero-reward no-code removed), got %d", len(filtered.Steps))
	}
	if filtered.Steps[0].Content != "func main() {}" {
		t.Errorf("expected code step to be kept, got %q", filtered.Steps[0].Content)
	}
}

func TestRemoveLowQualityStepsPreservesToolCallSteps(t *testing.T) {
	tc := NewTrajectoryCompressor()

	traj := &Trajectory{
		ID:      "low-quality-tools",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "Using tool", Reward: 0, ToolCalls: []ToolCall{{Name: "bash", Input: map[string]any{"cmd": "ls"}, Output: "file.txt"}}, Timestamp: time.Now()},
			{Role: "assistant", Content: "No tool, no code, zero reward", Reward: 0, Timestamp: time.Now()},
		},
		TotalReward: 0,
		CreatedAt:   time.Now(),
	}

	filtered := tc.removeLowQualitySteps(traj)
	if len(filtered.Steps) != 1 {
		t.Fatalf("expected 1 step (tool call kept, no-tool no-code removed), got %d", len(filtered.Steps))
	}
	if len(filtered.Steps[0].ToolCalls) != 1 {
		t.Error("tool call step should be preserved")
	}
}

func TestCompressBatch(t *testing.T) {
	tc := NewTrajectoryCompressor()

	trajectories := []*Trajectory{
		{
			ID: "batch-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "hello", Timestamp: time.Now()}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: time.Now(),
		},
		{
			ID: "batch-2", SessionID: "s2", Task: "t2",
			Steps: []TrajectoryStep{{Role: "user", Content: "world", Timestamp: time.Now()}},
			Outcome: "failure", TotalReward: 0.1, CreatedAt: time.Now(),
		},
	}

	compressed, stats, err := tc.CompressBatch(trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(compressed) != 2 {
		t.Errorf("expected 2 compressed trajectories, got %d", len(compressed))
	}
	if stats.TotalOriginal != 2 {
		t.Errorf("TotalOriginal should be 2, got %d", stats.TotalOriginal)
	}
	if stats.TotalCompressed != 2 {
		t.Errorf("TotalCompressed should be 2, got %d", stats.TotalCompressed)
	}
	if stats.DuplicatesRemoved != 0 {
		t.Errorf("DuplicatesRemoved should be 0, got %d", stats.DuplicatesRemoved)
	}
}

func TestCompressBatchEmpty(t *testing.T) {
	tc := NewTrajectoryCompressor()

	compressed, stats, err := tc.CompressBatch([]*Trajectory{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(compressed) != 0 {
		t.Errorf("expected 0 compressed trajectories, got %d", len(compressed))
	}
	if stats.TotalOriginal != 0 {
		t.Errorf("TotalOriginal should be 0, got %d", stats.TotalOriginal)
	}
}

func TestBatchDuplicateRemoval(t *testing.T) {
	tc := NewTrajectoryCompressor()
	now := time.Now()

	trajectories := []*Trajectory{
		{
			ID: "dup-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "first", Timestamp: now}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: now,
		},
		{
			ID: "dup-1", SessionID: "s1", Task: "t1",
			Steps: []TrajectoryStep{{Role: "user", Content: "duplicate ID", Timestamp: now}},
			Outcome: "success", TotalReward: 0.5, CreatedAt: now,
		},
		{
			ID: "unique-1", SessionID: "s2", Task: "t2",
			Steps: []TrajectoryStep{{Role: "user", Content: "unique", Timestamp: now}},
			Outcome: "failure", TotalReward: 0.1, CreatedAt: now,
		},
	}

	compressed, stats, err := tc.CompressBatch(trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.DuplicatesRemoved != 1 {
		t.Errorf("expected 1 duplicate removed (same ID), got %d", stats.DuplicatesRemoved)
	}
	if len(compressed) != 2 {
		t.Errorf("expected 2 compressed trajectories, got %d", len(compressed))
	}
	if stats.TotalOriginal != 3 {
		t.Errorf("TotalOriginal should be 3, got %d", stats.TotalOriginal)
	}
	if stats.TotalCompressed != 2 {
		t.Errorf("TotalCompressed should be 2, got %d", stats.TotalCompressed)
	}
}

func TestBatchNearDuplicateRemoval(t *testing.T) {
	tc := NewTrajectoryCompressor()
	now := time.Now()

	trajectories := []*Trajectory{
		{
			ID: "near-1", SessionID: "s1", Task: "sort array",
			Steps: []TrajectoryStep{
				{Role: "user", Content: "sort", Timestamp: now},
				{Role: "assistant", Content: "ok", Timestamp: now},
			},
			Outcome: "success", TotalReward: 0.50, CreatedAt: now,
		},
		{
			ID: "near-2", SessionID: "s2", Task: "sort array",
			Steps: []TrajectoryStep{
				{Role: "user", Content: "sort", Timestamp: now},
			},
			Outcome: "success", TotalReward: 0.50, CreatedAt: now,
		},
	}

	compressed, stats, err := tc.CompressBatch(trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.DuplicatesRemoved != 1 {
		t.Errorf("expected 1 near-duplicate removed (same Task+Outcome+Reward), got %d", stats.DuplicatesRemoved)
	}
	if len(compressed) != 1 {
		t.Errorf("expected 1 compressed trajectory, got %d", len(compressed))
	}
	// The one with more steps should be kept
	if len(compressed[0].Steps) < 2 {
		t.Error("should keep the trajectory with more steps")
	}
}

func TestBatchNearDuplicateDifferentReward(t *testing.T) {
	tc := NewTrajectoryCompressor()
	now := time.Now()

	trajectories := []*Trajectory{
		{
			ID: "diff-r1", SessionID: "s1", Task: "sort array",
			Steps: []TrajectoryStep{{Role: "user", Content: "sort", Timestamp: now}},
			Outcome: "success", TotalReward: 0.50, CreatedAt: now,
		},
		{
			ID: "diff-r2", SessionID: "s2", Task: "sort array",
			Steps: []TrajectoryStep{{Role: "user", Content: "sort", Timestamp: now}},
			Outcome: "success", TotalReward: 0.20, CreatedAt: now,
		},
	}

	compressed, stats, err := tc.CompressBatch(trajectories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.DuplicatesRemoved != 0 {
		t.Errorf("different rewards should not be near-duplicates, got %d removed", stats.DuplicatesRemoved)
	}
	if len(compressed) != 2 {
		t.Errorf("expected 2 compressed trajectories (different rewards), got %d", len(compressed))
	}
}

func TestCompressWithStatsOverBudget(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(500)
	longContent := strings.Repeat("x", 10000)

	traj := &Trajectory{
		ID:      "stats-overbudget",
		Outcome: "partial",
		Steps: []TrajectoryStep{
			{Role: "system", Content: "System prompt.", Timestamp: time.Now()},
			{Role: "user", Content: longContent, Reward: 0, Timestamp: time.Now()},
			{Role: "assistant", Content: longContent, Reward: 0.3, Timestamp: time.Now()},
		},
		TotalReward: 0.3,
		CreatedAt:   time.Now(),
	}

	compressed, stats, err := tc.CompressWithStats(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !compressed.Compressed {
		t.Error("trajectory should be marked compressed")
	}
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.OriginalStepCount != 3 {
		t.Errorf("OriginalStepCount should be 3, got %d", stats.OriginalStepCount)
	}
	if stats.CompressedStepCount > 3 {
		t.Errorf("CompressedStepCount should be <= 3, got %d", stats.CompressedStepCount)
	}
	if stats.OriginalBytes <= 0 {
		t.Error("OriginalBytes should be positive")
	}
	if stats.CompressedBytes <= 0 {
		t.Error("CompressedBytes should be positive")
	}
	if stats.CompressionRatio <= 0 {
		t.Error("CompressionRatio should be positive")
	}
	if stats.Method == "none" {
		t.Error("Method should not be 'none' when compression was applied")
	}
}

func TestCompressPipelineSemanticDedupFirst(t *testing.T) {
	tc := NewTrajectoryCompressor().WithMaxBytes(2000)
	now := time.Now()

	traj := &Trajectory{
		ID:      "pipeline-dedup",
		Outcome: "success",
		Steps: []TrajectoryStep{
			{Role: "assistant", Content: "I will help you write a function to sort an array of numbers", Reward: 0.1, Timestamp: now},
			{Role: "assistant", Content: "I will help you write a function to sort an array of strings", Reward: 0.2, Timestamp: now.Add(time.Second)},
			{Role: "assistant", Content: strings.Repeat("func sort(arr []int) { ... }\n", 100), Reward: 0.5, Timestamp: now.Add(2 * time.Second)},
		},
		TotalReward: 0.8,
		CreatedAt:   now,
	}

	compressed, stats, err := tc.CompressWithStats(traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.StepsMerged > 0 {
		if len(compressed.Steps) >= 3 {
			t.Errorf("expected steps to be merged by semanticDedup, got %d steps", len(compressed.Steps))
		}
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b     string
		expected float64
	}{
		{"hello world", "hello world", 1.0},
		{"hello world", "hello", 0.5},
		{"hello world", "goodbye world", 1.0 / 3.0},
		{"", "", 1.0},
		{"hello", "", 0.0},
		{"", "hello", 0.0},
		{"the quick brown fox", "the quick brown fox jumps", 4.0 / 5.0},
	}

	for i, tt := range tests {
		got := jaccardSimilarity(tt.a, tt.b)
		if math.Abs(got-tt.expected) > 0.01 {
			t.Errorf("test %d: jaccardSimilarity(%q, %q) = %f, want %f", i, tt.a, tt.b, got, tt.expected)
		}
	}
}
