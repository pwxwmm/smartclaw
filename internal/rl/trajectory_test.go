package rl

import (
	"bytes"
	"encoding/json"
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
