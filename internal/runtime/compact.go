package runtime

import (
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

	keepCount := len(messages)
	if keepCount < 4 {
		return messages
	}

	keepCount = 4

	summarized := summarizeOldMessages(messages[:len(messages)-keepCount])

	result := make([]Message, 0, keepCount+1)
	result = append(result, Message{
		Role:    "system",
		Content: "Previous context summary:\n" + summarized,
	})
	result = append(result, messages[len(messages)-keepCount:]...)

	return result
}

func summarizeOldMessages(messages []Message) string {
	var parts []string
	for _, msg := range messages {
		var content string
		if str, ok := msg.Content.(string); ok {
			content = str
		}
		parts = append(parts, msg.Role+": "+truncate(content, 200))
	}
	return strings.Join(parts, "\n")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
