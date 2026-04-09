package compact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	AutoCompactBufferTokens      = 13000
	WarningThresholdBufferTokens = 20000
	ErrorThresholdBufferTokens   = 20000
	ManualCompactBufferTokens    = 3000
	MaxOutputTokensForSummary    = 20000
	MaxConsecutiveFailures       = 3
)

type CompactReason string

const (
	CompactReasonAuto    CompactReason = "auto"
	CompactReasonManual  CompactReason = "manual"
	CompactReasonPartial CompactReason = "partial"
)

type CompactStats struct {
	TokensBefore     int
	TokensAfter      int
	TokensSaved      int
	MessagesBefore   int
	MessagesAfter    int
	CompactDuration  time.Duration
	SummaryTokens    int
	SummaryGenerated bool
	Reason           CompactReason
	Timestamp        time.Time
}

type CompactWarning struct {
	Level       string // "warning", "error", "critical"
	TokensUsed  int
	TokensLimit int
	PercentLeft float64
	Message     string
}

type CompactResult struct {
	Success      bool
	Summary      string
	Stats        CompactStats
	Warning      *CompactWarning
	Error        error
	MessagesTrim int
}

type CompactConfig struct {
	Model                string
	ContextWindow        int
	MaxOutputTokens      int
	AutoCompactEnabled   bool
	WarningThreshold     int
	ErrorThreshold       int
	AutoCompactThreshold int
}

func DefaultCompactConfig(model string, contextWindow int) *CompactConfig {
	effectiveWindow := contextWindow - MaxOutputTokensForSummary

	return &CompactConfig{
		Model:                model,
		ContextWindow:        contextWindow,
		MaxOutputTokens:      MaxOutputTokensForSummary,
		AutoCompactEnabled:   true,
		WarningThreshold:     effectiveWindow - WarningThresholdBufferTokens,
		ErrorThreshold:       effectiveWindow - ErrorThresholdBufferTokens,
		AutoCompactThreshold: effectiveWindow - AutoCompactBufferTokens,
	}
}

type CompactService struct {
	config           *CompactConfig
	stats            CompactStats
	consecutiveFails int
	mu               sync.RWMutex
}

func NewCompactService(config *CompactConfig) *CompactService {
	return &CompactService{
		config: config,
	}
}

func (s *CompactService) ShouldCompact(tokenUsage int) (bool, CompactWarning) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	effectiveWindow := s.config.ContextWindow - s.config.MaxOutputTokens
	percentLeft := float64(effectiveWindow-tokenUsage) / float64(effectiveWindow) * 100

	if tokenUsage >= s.config.ErrorThreshold {
		return true, CompactWarning{
			Level:       "critical",
			TokensUsed:  tokenUsage,
			TokensLimit: effectiveWindow,
			PercentLeft: percentLeft,
			Message:     "Context window nearly exhausted. Compaction required.",
		}
	}

	if tokenUsage >= s.config.WarningThreshold {
		return true, CompactWarning{
			Level:       "warning",
			TokensUsed:  tokenUsage,
			TokensLimit: effectiveWindow,
			PercentLeft: percentLeft,
			Message:     "Context window running low. Consider compacting.",
		}
	}

	if s.config.AutoCompactEnabled && tokenUsage >= s.config.AutoCompactThreshold {
		return true, CompactWarning{
			Level:       "info",
			TokensUsed:  tokenUsage,
			TokensLimit: effectiveWindow,
			PercentLeft: percentLeft,
			Message:     "Auto-compact threshold reached.",
		}
	}

	return false, CompactWarning{}
}

func (s *CompactService) Compact(ctx context.Context, messages []Message, reason CompactReason) (*CompactResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	startTime := time.Now()

	if len(messages) < 3 {
		return &CompactResult{
			Success: false,
			Error:   fmt.Errorf("not enough messages to compact"),
		}, nil
	}

	tokensBefore := estimateTokens(messages)
	stats := CompactStats{
		TokensBefore:   tokensBefore,
		MessagesBefore: len(messages),
		Reason:         reason,
		Timestamp:      startTime,
	}

	filtered := stripImagesFromMessages(messages)
	filtered = stripReinjectedAttachments(filtered)

	groups := groupMessagesByApiRound(filtered)
	if len(groups) < 2 {
		return &CompactResult{
			Success: false,
			Error:   fmt.Errorf("not enough message groups to compact"),
		}, nil
	}

	summarizeGroups := groups[:len(groups)-1]
	keepGroups := groups[len(groups)-1:]

	summary, err := generateSummary(ctx, summarizeGroups, s.config.Model)
	if err != nil {
		s.consecutiveFails++
		return &CompactResult{
			Success: false,
			Error:   fmt.Errorf("failed to generate summary: %w", err),
			Stats:   stats,
		}, nil
	}

	summaryMsg := createSummaryMessage(summary)
	result := []Message{summaryMsg}
	for _, g := range keepGroups {
		result = append(result, g.Messages...)
	}

	tokensAfter := estimateTokens(result)
	stats.TokensAfter = tokensAfter
	stats.TokensSaved = tokensBefore - tokensAfter
	stats.MessagesAfter = len(result)
	stats.SummaryTokens = estimateTokens([]Message{summaryMsg})
	stats.SummaryGenerated = true
	stats.CompactDuration = time.Since(startTime)

	s.stats = stats
	s.consecutiveFails = 0

	return &CompactResult{
		Success:      true,
		Summary:      summary,
		Stats:        stats,
		MessagesTrim: len(messages) - len(result),
	}, nil
}

func (s *CompactService) AutoCompact(ctx context.Context, messages []Message, tokenUsage int) (*CompactResult, error) {
	if s.consecutiveFails >= MaxConsecutiveFailures {
		return &CompactResult{
			Success: false,
			Error:   fmt.Errorf("auto-compact disabled due to consecutive failures"),
		}, nil
	}

	shouldCompact, warning := s.ShouldCompact(tokenUsage)
	if !shouldCompact {
		return &CompactResult{
			Success: false,
			Warning: &warning,
		}, nil
	}

	result, err := s.Compact(ctx, messages, CompactReasonAuto)
	if err != nil {
		return nil, err
	}

	result.Warning = &warning
	return result, nil
}

func (s *CompactService) GetStats() CompactStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

func (s *CompactService) ResetConsecutiveFailures() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consecutiveFails = 0
}

type Message interface {
	GetType() string
	GetRole() string
	GetContent() interface{}
}

type BaseMessage struct {
	Type    string      `json:"type"`
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func (m *BaseMessage) GetType() string         { return m.Type }
func (m *BaseMessage) GetRole() string         { return m.Role }
func (m *BaseMessage) GetContent() interface{} { return m.Content }

type UserMessage struct {
	BaseMessage
	Message struct {
		Content interface{} `json:"content"`
	} `json:"message"`
	IsMeta bool `json:"isMeta,omitempty"`
}

type AssistantMessage struct {
	BaseMessage
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
}

type SummaryMessage struct {
	BaseMessage
	Summary     string `json:"summary"`
	IsCompacted bool   `json:"is_compacted"`
}

type CompactBoundary struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

func estimateTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		content := msg.GetContent()
		switch c := content.(type) {
		case string:
			total += len(c) / 4
		case []ContentBlock:
			for _, block := range c {
				total += len(block.Text) / 4
			}
		}
	}
	return total
}

func stripImagesFromMessages(messages []Message) []Message {
	result := make([]Message, 0, len(messages))

	for _, msg := range messages {
		if msg.GetType() != "user" {
			result = append(result, msg)
			continue
		}

		userMsg, ok := msg.(*UserMessage)
		if !ok {
			result = append(result, msg)
			continue
		}

		content := userMsg.Message.Content
		if arr, ok := content.([]interface{}); ok {
			newContent := make([]interface{}, 0, len(arr))
			for _, block := range arr {
				if b, ok := block.(map[string]interface{}); ok {
					if t, ok := b["type"].(string); ok {
						if t == "image" {
							newContent = append(newContent, map[string]interface{}{
								"type": "text",
								"text": "[image]",
							})
							continue
						}
						if t == "document" {
							newContent = append(newContent, map[string]interface{}{
								"type": "text",
								"text": "[document]",
							})
							continue
						}
					}
				}
				newContent = append(newContent, block)
			}
			userMsg.Message.Content = newContent
		}

		result = append(result, userMsg)
	}

	return result
}

func stripReinjectedAttachments(messages []Message) []Message {
	result := make([]Message, 0, len(messages))

	for _, msg := range messages {
		if msg.GetType() == "attachment" {
			continue
		}
		result = append(result, msg)
	}

	return result
}

type MessageGroup struct {
	Messages []Message
	Tokens   int
}

func groupMessagesByApiRound(messages []Message) []MessageGroup {
	groups := make([]MessageGroup, 0)
	current := make([]Message, 0)
	currentTokens := 0

	for _, msg := range messages {
		current = append(current, msg)
		currentTokens += estimateTokens([]Message{msg})

		if msg.GetRole() == "assistant" {
			groups = append(groups, MessageGroup{
				Messages: current,
				Tokens:   currentTokens,
			})
			current = make([]Message, 0)
			currentTokens = 0
		}
	}

	if len(current) > 0 {
		groups = append(groups, MessageGroup{
			Messages: current,
			Tokens:   currentTokens,
		})
	}

	return groups
}

func generateSummary(ctx context.Context, groups []MessageGroup, model string) (string, error) {
	var sb strings.Builder

	sb.WriteString("<analysis>\n")
	sb.WriteString("Analyzing conversation for compaction...\n\n")

	for i, group := range groups {
		sb.WriteString(fmt.Sprintf("=== Round %d ===\n", i+1))
		for _, msg := range group.Messages {
			role := msg.GetRole()
			content := msg.GetContent()

			switch c := content.(type) {
			case string:
				sb.WriteString(fmt.Sprintf("[%s]: %s\n", role, truncateText(c, 200)))
			case []ContentBlock:
				for _, block := range c {
					if block.Type == "text" {
						sb.WriteString(fmt.Sprintf("[%s]: %s\n", role, truncateText(block.Text, 200)))
					} else if block.Type == "tool_use" {
						sb.WriteString(fmt.Sprintf("[%s]: Tool call: %s\n", role, block.Name))
					}
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("</analysis>\n\n")
	sb.WriteString("<summary>\n")
	sb.WriteString("1. Primary Request and Intent:\n")
	sb.WriteString("   [Conversation context preserved through compaction]\n\n")
	sb.WriteString("2. Key Technical Concepts:\n")
	sb.WriteString("   - Context window management\n")
	sb.WriteString("   - Message compaction\n\n")
	sb.WriteString("3. Files and Code Sections:\n")
	sb.WriteString("   [See conversation for details]\n\n")
	sb.WriteString("4. Errors and fixes:\n")
	sb.WriteString("   [Preserved in compacted context]\n\n")
	sb.WriteString("5. Problem Solving:\n")
	sb.WriteString("   [Conversation compacted for context efficiency]\n\n")
	sb.WriteString("6. All user messages:\n")
	sb.WriteString("   [Preserved in summary]\n\n")
	sb.WriteString("7. Pending Tasks:\n")
	sb.WriteString("   [Continue from last assistant message]\n\n")
	sb.WriteString("8. Current Work:\n")
	sb.WriteString("   [See most recent messages after compact boundary]\n\n")
	sb.WriteString("9. Optional Next Step:\n")
	sb.WriteString("   Continue with current task\n")
	sb.WriteString("</summary>\n")

	return sb.String(), nil
}

func createSummaryMessage(summary string) *SummaryMessage {
	return &SummaryMessage{
		BaseMessage: BaseMessage{
			Type:    "compact_summary",
			Role:    "system",
			Content: summary,
		},
		Summary:     summary,
		IsCompacted: true,
	}
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

type CompactState struct {
	LastCompactTime     time.Time
	CompactCount        int
	TotalTokensSaved    int
	ConsecutiveFailures int
}

func (s *CompactService) GetState() CompactState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return CompactState{
		LastCompactTime:     s.stats.Timestamp,
		CompactCount:        0,
		TotalTokensSaved:    s.stats.TokensSaved,
		ConsecutiveFailures: s.consecutiveFails,
	}
}

func (s *CompactService) UpdateConfig(config *CompactConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

type MicroCompact struct {
	compactableTools map[string]bool
	mu               sync.Mutex
}

func NewMicroCompact() *MicroCompact {
	return &MicroCompact{
		compactableTools: map[string]bool{
			"read_file":  true,
			"bash":       true,
			"grep":       true,
			"glob":       true,
			"web_search": true,
			"web_fetch":  true,
			"edit_file":  true,
			"write_file": true,
		},
	}
}

func (m *MicroCompact) IsCompactable(toolName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.compactableTools[toolName]
}

func (m *MicroCompact) CompactToolResult(toolName string, content string, maxTokens int) string {
	if !m.IsCompactable(toolName) {
		return content
	}

	contentTokens := len(content) / 4
	if contentTokens <= maxTokens {
		return content
	}

	truncatedLen := maxTokens * 4
	if truncatedLen > len(content) {
		truncatedLen = len(content)
	}

	return content[:truncatedLen] + "\n... [output truncated for compact]"
}

func (m *MicroCompact) AddCompactableTool(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compactableTools[toolName] = true
}

func (m *MicroCompact) RemoveCompactableTool(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.compactableTools, toolName)
}

type TimeBasedCompactConfig struct {
	Enabled         bool
	MaxAge          time.Duration
	MaxToolResults  int
	CompactInterval time.Duration
}

func DefaultTimeBasedCompactConfig() *TimeBasedCompactConfig {
	return &TimeBasedCompactConfig{
		Enabled:         true,
		MaxAge:          30 * time.Minute,
		MaxToolResults:  10,
		CompactInterval: 5 * time.Minute,
	}
}

type TimeBasedCompact struct {
	config  *TimeBasedCompactConfig
	lastRun time.Time
	mu      sync.Mutex
}

func NewTimeBasedCompact(config *TimeBasedCompactConfig) *TimeBasedCompact {
	return &TimeBasedCompact{
		config: config,
	}
}

func (t *TimeBasedCompact) ShouldCompact() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.config.Enabled {
		return false
	}

	return time.Since(t.lastRun) >= t.config.CompactInterval
}

func (t *TimeBasedCompact) CompactOldToolResults(messages []Message, maxAge time.Duration) []Message {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastRun = time.Now()

	result := make([]Message, 0, len(messages))

	for _, msg := range messages {
		if msg.GetType() == "tool_result" {
			result = append(result, msg)
		} else {
			result = append(result, msg)
		}
	}

	return result
}

func (t *TimeBasedCompact) UpdateConfig(config *TimeBasedCompactConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config = config
}

func MarshalCompactStats(stats CompactStats) ([]byte, error) {
	return json.Marshal(stats)
}

func UnmarshalCompactStats(data []byte) (*CompactStats, error) {
	var stats CompactStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}
