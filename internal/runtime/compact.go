package runtime

import (
	"fmt"
	"strings"
)

func CountTokens(text string) int {
	return len(strings.Fields(text))
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

func Compact(messages []Message, maxTokens int) []Message {
	if !ShouldCompact(messages, maxTokens) {
		return messages
	}

	if len(messages) < 6 {
		return messages
	}

	headEnd := findHeadBoundary(messages)
	head := messages[:headEnd]
	rest := messages[headEnd:]

	keepRecent := 4
	if len(rest) < keepRecent {
		return messages
	}

	tailStart := len(rest) - keepRecent
	tail := rest[tailStart:]
	middle := rest[:tailStart]

	prunedMiddle := pruneOldToolResults(middle)

	summary := summarizeOldMessages(prunedMiddle)

	result := make([]Message, 0, len(head)+1+len(tail))
	result = append(result, head...)
	result = append(result, Message{
		Role:    "system",
		Content: fmt.Sprintf("Previous context summary (turns %d-%d):\n%s", headEnd, headEnd+len(middle)-1, summary),
	})
	result = append(result, tail...)

	return sanitizeToolPairs(result)
}

func findHeadBoundary(messages []Message) int {
	for i, msg := range messages {
		if msg.Role == "user" {
			return i
		}
	}
	return 0
}

func pruneOldToolResults(messages []Message) []Message {
	maxToolResultChars := 500
	pruned := make([]Message, 0, len(messages))

	for _, msg := range messages {
		content, ok := msg.Content.(string)
		if !ok {
			pruned = append(pruned, msg)
			continue
		}

		if msg.Role == "tool" && len(content) > maxToolResultChars {
			pruned = append(pruned, Message{
				Role:      msg.Role,
				Content:   content[:maxToolResultChars] + "\n...[truncated]",
				Timestamp: msg.Timestamp,
				UUID:      msg.UUID,
			})
			continue
		}

		pruned = append(pruned, msg)
	}

	return pruned
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
