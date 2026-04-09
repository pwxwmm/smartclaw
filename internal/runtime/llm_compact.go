package runtime

import (
	"fmt"
	"log/slog"
	"strings"
)

type LLMCompactor struct {
	createMessageFunc func(systemPrompt, userPrompt string) (string, error)
}

func NewLLMCompactor(createMessageFunc func(systemPrompt, userPrompt string) (string, error)) *LLMCompactor {
	return &LLMCompactor{createMessageFunc: createMessageFunc}
}

func (lc *LLMCompactor) CompactWithLLM(messages []Message, maxTokens int) []Message {
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

	summary := lc.summarizeWithLLM(prunedMiddle, headEnd)

	result := make([]Message, 0, len(head)+1+len(tail))
	result = append(result, head...)
	result = append(result, Message{
		Role:    "system",
		Content: fmt.Sprintf("Previous context summary (turns %d-%d):\n%s", headEnd, headEnd+len(middle)-1, summary),
	})
	result = append(result, tail...)

	return sanitizeToolPairs(result)
}

func (lc *LLMCompactor) summarizeWithLLM(messages []Message, startTurn int) string {
	if lc.createMessageFunc == nil {
		return summarizeOldMessages(messages)
	}

	conversation := formatMessagesForSummary(messages)

	systemPrompt := `Summarize the following conversation, preserving:
1. Key decisions and their reasons
2. Important context about the project
3. Tool calls and their results (brief)
4. Any unresolved issues

Keep the summary concise and factual. Do not add information not present in the conversation.`

	summary, err := lc.createMessageFunc(systemPrompt, conversation)
	if err != nil {
		slog.Warn("LLM compactor: summarization failed, falling back to simple summary", "error", err)
		return summarizeOldMessages(messages)
	}

	return summary
}

func formatMessagesForSummary(messages []Message) string {
	var parts []string
	totalChars := 0
	maxChars := 8000

	for i := len(messages) - 1; i >= 0 && totalChars < maxChars; i-- {
		msg := messages[i]
		content, ok := msg.Content.(string)
		if !ok {
			continue
		}
		if content == "" {
			continue
		}
		if msg.Role == "tool" && len(content) > 300 {
			content = content[:300] + "...[truncated]"
		}
		line := fmt.Sprintf("[%s]: %s", msg.Role, content)
		parts = append([]string{line}, parts...)
		totalChars += len(line)
	}

	return strings.Join(parts, "\n")
}
