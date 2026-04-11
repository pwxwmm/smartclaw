package runtime

import (
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
)

type Message struct {
	Role      string      `json:"role"`
	Content   any `json:"content"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
	UUID      string      `json:"uuid,omitempty"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolUseContent struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]any `json:"input"`
}

type ToolResultContent struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   any `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}

type ThinkingContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonStopSeq   StopReason = "stop_sequence"
)

type Usage = api.Usage

type QueryConfig struct {
	Model               string
	MaxTokens           int
	SystemPrompt        string
	EnableLLMCompaction bool
	CompactionModel     string
}

type QueryResult struct {
	Message    Message       `json:"message"`
	Usage      Usage         `json:"usage"`
	StopReason StopReason    `json:"stop_reason"`
	Duration   time.Duration `json:"duration"`
	Cost       float64       `json:"cost"`
}

type StreamHandler func(event QueryEvent) error

type QueryEvent struct {
	Type      string      `json:"type"`
	Data      any `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type TokenBudget struct {
	Total     int  `json:"total"`
	Used      int  `json:"used"`
	Remaining int  `json:"remaining"`
	Warning   bool `json:"warning,omitempty"`
	Exceeded  bool `json:"exceeded,omitempty"`
}

type QueryState struct {
	mu               sync.RWMutex
	messages         []Message
	tree             *ConversationTree
	useTree          bool
	turnCount        int
	totalTokens      Usage
	budget           TokenBudget
	lastResponseTime time.Time
	lastSystemPrompt string
}

func NewQueryState() *QueryState {
	return &QueryState{
		messages: []Message{},
		tree:     NewConversationTree(),
	}
}

func (s *QueryState) EnableTree() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.useTree = true
}

func (s *QueryState) IsTreeEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.useTree
}

func (s *QueryState) AddMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	if s.useTree {
		s.tree.AddMessage(msg)
	}
}

func (s *QueryState) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.useTree && s.tree != nil {
		if treeMsgs := s.tree.GetLinearHistory(); len(treeMsgs) > 0 {
			return treeMsgs
		}
	}

	result := make([]Message, len(s.messages))
	copy(result, s.messages)
	return result
}

func (s *QueryState) UpdateUsage(usage Usage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalTokens.InputTokens += usage.InputTokens
	s.totalTokens.OutputTokens += usage.OutputTokens
}

func (s *QueryState) GetUsage() Usage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalTokens
}

func (s *QueryState) IncrementTurn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turnCount++
}

func (s *QueryState) GetTurnCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.turnCount
}

func (s *QueryState) GetLastSystemPrompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSystemPrompt
}

func (s *QueryState) SetLastSystemPrompt(prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSystemPrompt = prompt
}

func (s *QueryState) ReplaceMessages(messages []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = messages
}
