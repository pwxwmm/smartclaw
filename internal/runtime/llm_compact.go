package runtime

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

type LLMCompactor struct {
	createMessageFunc func(systemPrompt, userPrompt string) (string, error)
	previousSummary   string
	mu                sync.Mutex
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

	prunedMiddle := pruneOldToolResults(middle, defaultMaxToolResultChars)

	summary := lc.updatePreviousSummary(prunedMiddle, headEnd)

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

func (lc *LLMCompactor) updatePreviousSummary(messages []Message, startTurn int) string {
	lc.mu.Lock()
	prev := lc.previousSummary
	lc.mu.Unlock()

	if lc.createMessageFunc == nil {
		summary := summarizeOldMessages(messages)
		lc.mu.Lock()
		lc.previousSummary = summary
		lc.mu.Unlock()
		return summary
	}

	var summary string
	if prev != "" {
		summary = lc.incrementalSummarize(prev, messages)
	} else {
		summary = lc.summarizeWithLLM(messages, startTurn)
	}

	lc.mu.Lock()
	lc.previousSummary = summary
	lc.mu.Unlock()

	return summary
}

func (lc *LLMCompactor) incrementalSummarize(previousSummary string, newMessages []Message) string {
	newConversation := formatMessagesForSummary(newMessages)

	systemPrompt := `You have a previous summary of an ongoing conversation. New messages have been added since.
Update the summary to incorporate the new information while preserving:
1. Key decisions and their reasons
2. Important context about the project
3. Tool calls and their results (brief)
4. Any unresolved issues

Keep the summary concise and factual. Do not add information not present in the conversation or previous summary.`

	userPrompt := fmt.Sprintf("Previous summary:\n%s\n\nNew messages:\n%s", previousSummary, newConversation)

	summary, err := lc.createMessageFunc(systemPrompt, userPrompt)
	if err != nil {
		slog.Warn("LLM compactor: incremental summarization failed, falling back", "error", err)
		return lc.summarizeWithLLM(newMessages, 0)
	}

	return summary
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
