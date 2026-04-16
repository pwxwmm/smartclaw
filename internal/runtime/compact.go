package runtime

import (
	"fmt"
	"strings"

	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/utils"
)

const (
	defaultMaxToolResultChars    = 2000
	aggressiveMaxToolResultChars = 500
)

// Range represents a range of original messages that were compressed into a summary.
// Used for source tracing — enables going back to the original conversation.
type Range struct {
	StartMsgID string
	EndMsgID   string
	TurnStart  int
	TurnEnd    int
}

// CompactionResult captures the result of a compaction operation.
type CompactionResult struct {
	Summary      string
	SourceRanges []Range
	KeptMessages []Message
}

type CompactConfig struct {
	MaxTokens           int
	MaxToolResultChars  int
	AggressiveThreshold float64
}

func DefaultCompactConfig(maxTokens int) CompactConfig {
	return CompactConfig{
		MaxTokens:           maxTokens,
		MaxToolResultChars:  defaultMaxToolResultChars,
		AggressiveThreshold: 1.5,
	}
}

func CountTokens(text string) int {
	return utils.CountTokens(text)
}

func CountMessagesTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		if str, ok := msg.Content.(string); ok {
			total += CountTokens(str)
		}
		total += 4
	}
	return total
}

func ShouldCompact(messages []Message, maxTokens int) bool {
	return CountMessagesTokens(messages) > maxTokens
}

// Compact performs context compaction with token-budget-driven tail selection and source tracing.
// It preserves head system messages, compresses the middle into a summary with a source range,
// and keeps a tail section selected by token budget rather than a fixed count.
func Compact(messages []Message, maxTokens int) []Message {
	return CompactWithConfig(messages, DefaultCompactConfig(maxTokens))
}

func CompactWithConfig(messages []Message, cfg CompactConfig) []Message {
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8000
	}

	if !ShouldCompact(messages, maxTokens) {
		return messages
	}

	if len(messages) < 6 {
		return messages
	}

	headEnd := findHeadBoundary(messages)
	head := messages[:headEnd]
	rest := messages[headEnd:]

	tailBudget := maxTokens / 3
	tailStart := findTailBoundary(rest, tailBudget)
	if tailStart <= 0 {
		tailStart = len(rest) - 4
		if tailStart < 0 {
			tailStart = 0
		}
	}

	if tailStart >= len(rest) {
		return messages
	}

	tail := rest[tailStart:]
	middle := rest[:tailStart]

	if len(middle) == 0 {
		return messages
	}

	totalTokens := CountMessagesTokens(messages)
	overBudgetRatio := float64(totalTokens) / float64(maxTokens)
	maxToolChars := cfg.MaxToolResultChars
	if maxToolChars <= 0 {
		maxToolChars = defaultMaxToolResultChars
	}
	if overBudgetRatio > cfg.AggressiveThreshold {
		maxToolChars = aggressiveMaxToolResultChars
	}

	prunedMiddle := pruneOldToolResults(middle, maxToolChars)

	summary := summarizeOldMessages(prunedMiddle)

	startUUID := firstUUID(middle)
	endUUID := lastUUID(middle)
	turnStart := headEnd
	turnEnd := headEnd + len(middle) - 1

	result := make([]Message, 0, len(head)+1+len(tail))
	result = append(result, head...)
	result = append(result, Message{
		Role:    "system",
		Content: fmt.Sprintf("[Compaction: turns %d-%d, uuid_start:%s, uuid_end:%s]\nPrevious context summary:\n%s", turnStart, turnEnd, startUUID, endUUID, summary),
	})
	result = append(result, tail...)

	return sanitizeToolPairs(result)
}

// TraceBack retrieves original messages from the store for a given compaction range.
// This enables going back to the source conversation from a compaction summary.
func TraceBack(s *store.Store, sessionID string, r Range) ([]*store.Message, error) {
	if s == nil {
		return nil, fmt.Errorf("compaction traceback: store is nil")
	}

	messages, err := s.GetSessionMessages(sessionID)
	if err != nil {
		return nil, fmt.Errorf("compaction traceback: %w", err)
	}

	if r.TurnStart > 0 || r.TurnEnd > 0 {
		var filtered []*store.Message
		for _, msg := range messages {
			filtered = append(filtered, msg)
		}
		if len(filtered) > 0 {
			return filtered, nil
		}
	}

	return messages, nil
}

// ParseCompactionMeta extracts Range metadata from a compaction summary message content.
// Returns nil if the content doesn't contain compaction metadata.
func ParseCompactionMeta(content string) *Range {
	if !strings.Contains(content, "[Compaction:") {
		return nil
	}

	r := &Range{}

	if idx := strings.Index(content, "uuid_start:"); idx >= 0 {
		rest := content[idx+len("uuid_start:"):]
		end := strings.IndexAny(rest, ",]")
		if end > 0 {
			r.StartMsgID = rest[:end]
		}
	}

	if idx := strings.Index(content, "uuid_end:"); idx >= 0 {
		rest := content[idx+len("uuid_end:"):]
		end := strings.IndexAny(rest, "]\n")
		if end > 0 {
			r.EndMsgID = rest[:end]
		}
	}

	if idx := strings.Index(content, "turns "); idx >= 0 {
		rest := content[idx+len("turns "):]
		fmt.Sscanf(rest, "%d-%d", &r.TurnStart, &r.TurnEnd)
	}

	return r
}

// findTailBoundary walks backward from the end of messages, counting tokens,
// until the token budget is exhausted. Returns the index where the tail starts.
func findTailBoundary(messages []Message, tokenBudget int) int {
	if tokenBudget <= 0 {
		tokenBudget = 500
	}

	used := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := 4
		if str, ok := messages[i].Content.(string); ok {
			msgTokens += CountTokens(str)
		}
		if used+msgTokens > tokenBudget {
			return i + 1
		}
		used += msgTokens
	}

	return 0
}

func findHeadBoundary(messages []Message) int {
	for i, msg := range messages {
		if msg.Role == "user" {
			return i
		}
	}
	return 0
}

func pruneOldToolResults(messages []Message, maxToolResultChars int) []Message {
	if maxToolResultChars <= 0 {
		maxToolResultChars = defaultMaxToolResultChars
	}

	pruned := make([]Message, 0, len(messages))

	for _, msg := range messages {
		content, ok := msg.Content.(string)
		if !ok {
			pruned = append(pruned, msg)
			continue
		}

		if msg.Role == "tool" && len(content) > maxToolResultChars {
			truncated := smartTruncate(content, maxToolResultChars)
			pruned = append(pruned, Message{
				Role:      msg.Role,
				Content:   truncated,
				Timestamp: msg.Timestamp,
				UUID:      msg.UUID,
			})
			continue
		}

		pruned = append(pruned, msg)
	}

	return pruned
}

func smartTruncate(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	headLen := maxLen * 2 / 3
	tailLen := maxLen / 6
	if headLen+tailLen > maxLen {
		headLen = maxLen - tailLen
	}

	head := content[:headLen]
	tail := content[len(content)-tailLen:]

	lines := strings.Split(content[:maxLen], "\n")
	var lastStructural string
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, " ") {
			lastStructural = trimmed
			break
		}
	}

	result := head
	if lastStructural != "" {
		result += "\n...[last context: " + truncateCompact(lastStructural, 80) + "]...\n"
	}
	result += tail

	return result
}

func sanitizeToolPairs(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	seenToolUse := false

	for _, msg := range messages {
		if msg.Role == "assistant" {
			content, ok := msg.Content.(string)
			if ok && (strings.Contains(content, "tool_use") || strings.Contains(content, "\"type\":\"tool_use\"")) {
				seenToolUse = true
			}
		}

		if msg.Role == "tool" && !seenToolUse {
			continue
		}

		if msg.Role == "user" {
			seenToolUse = false
		}

		result = append(result, msg)
	}

	return result
}

func summarizeOldMessages(messages []Message) string {
	var parts []string
	for _, msg := range messages {
		var content string
		if str, ok := msg.Content.(string); ok {
			content = str
		}
		parts = append(parts, msg.Role+": "+truncateCompact(content, 200))
	}
	return strings.Join(parts, "\n")
}

func truncateCompact(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func firstUUID(messages []Message) string {
	for _, m := range messages {
		if m.UUID != "" {
			return m.UUID
		}
	}
	return ""
}

func lastUUID(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].UUID != "" {
			return messages[i].UUID
		}
	}
	return ""
}
