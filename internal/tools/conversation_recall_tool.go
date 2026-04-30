package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/store"
)

var (
	conversationRecallMu    sync.RWMutex
	conversationRecallLLM   learning.LLMClient
	conversationRecallStore *store.Store
)

// Global SkillTracker for recording skill invocations/outcomes from the skill tool.
var (
	globalSkillTrackerMu sync.RWMutex
	globalSkillTracker   *learning.SkillTracker
)

// SetGlobalSkillTracker registers the global SkillTracker used by the skill
// tool to record invocations and outcomes. Call this during application
// initialization after the store is available.
func SetGlobalSkillTracker(tracker *learning.SkillTracker) {
	globalSkillTrackerMu.Lock()
	defer globalSkillTrackerMu.Unlock()
	globalSkillTracker = tracker
}

// GetGlobalSkillTracker returns the global SkillTracker, or nil if not set.
func GetGlobalSkillTracker() *learning.SkillTracker {
	globalSkillTrackerMu.RLock()
	defer globalSkillTrackerMu.RUnlock()
	return globalSkillTracker
}

// SetLLMClientForConversationRecall registers the LLM client used by the
// recall_conversations tool for auto-summarization. Call this during
// application initialization after the API client is available.
func SetLLMClientForConversationRecall(client learning.LLMClient) {
	conversationRecallMu.Lock()
	defer conversationRecallMu.Unlock()
	conversationRecallLLM = client
}

// SetStoreForConversationRecall registers the store used by the
// recall_conversations tool for FTS5 search.
func SetStoreForConversationRecall(s *store.Store) {
	conversationRecallMu.Lock()
	defer conversationRecallMu.Unlock()
	conversationRecallStore = s
}

func getConversationRecallLLM() learning.LLMClient {
	conversationRecallMu.RLock()
	defer conversationRecallMu.RUnlock()
	return conversationRecallLLM
}

func getConversationRecallStore() *store.Store {
	conversationRecallMu.RLock()
	defer conversationRecallMu.RUnlock()
	return conversationRecallStore
}

type conversationHit struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Snippet   string `json:"snippet"`
}

type ConversationRecallTool struct{ BaseTool }

func (t *ConversationRecallTool) Name() string { return "recall_conversations" }
func (t *ConversationRecallTool) Description() string {
	return "Search across all historical conversations. Returns relevant messages from past sessions with optional LLM-summarized context."
}

func (t *ConversationRecallTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query for cross-session conversation search",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max results to return",
				"default":     10,
			},
			"since": map[string]any{
				"type":        "string",
				"description": "Only results at or after this ISO 8601 date",
			},
			"until": map[string]any{
				"type":        "string",
				"description": "Only results before or at this ISO 8601 date",
			},
			"role": map[string]any{
				"type":        "string",
				"enum":        []string{"user", "assistant"},
				"description": "Filter by message role",
			},
			"summarize": map[string]any{
				"type":        "boolean",
				"description": "Auto-summarize results when total content exceeds 3000 chars",
				"default":     true,
			},
		},
		"required": []string{"query"},
	}
}

func (t *ConversationRecallTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return nil, ErrRequiredField("query")
	}

	limit := 10
	if l, ok := input["limit"].(int); ok && l > 0 {
		limit = l
	}

	summarize := true
	if s, ok := input["summarize"].(bool); ok {
		summarize = s
	}

	role, _ := input["role"].(string)

	var since, until time.Time
	if s, ok := input["since"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	if u, ok := input["until"].(string); ok && u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			until = t
		}
	}

	s := getConversationRecallStore()
	if s == nil {
		if mm := getMemoryManager(); mm != nil {
			s = mm.GetStore()
		}
	}
	if s == nil {
		return nil, fmt.Errorf("recall_conversations: store not available")
	}

	opts := store.SearchOptions{
		Role:   role,
		Since:  since,
		Until:  until,
		Limit:  limit,
	}

	results, err := s.SearchMessagesAdvanced(query, opts)
	if err != nil {
		return nil, fmt.Errorf("recall_conversations: %w", err)
	}

	hits := make([]conversationHit, 0, len(results))
	totalChars := 0
	for _, r := range results {
		hits = append(hits, conversationHit{
			SessionID: r.SessionID,
			Role:      r.Role,
			Content:   r.Content,
			Timestamp: r.Timestamp.Format(time.RFC3339),
			Snippet:   r.Snippet,
		})
		totalChars += len(r.Content)
	}

	response := map[string]any{
		"query":       query,
		"results":     hits,
		"total_chars": totalChars,
	}

	if summarize && totalChars > 3000 {
		llmClient := getConversationRecallLLM()
		if llmClient != nil {
			summary, err := generateConversationSummary(ctx, llmClient, hits, query)
			if err != nil {
				response["summary_error"] = err.Error()
			} else {
				response["summary"] = summary
			}
		}
	}

	return response, nil
}

func generateConversationSummary(ctx context.Context, llmClient learning.LLMClient, hits []conversationHit, query string) (string, error) {
	var fragments []string
	for _, h := range hits {
		content := h.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fragments = append(fragments, fmt.Sprintf("[%s %s]: %s", h.Role, h.Timestamp, content))
	}

	systemPrompt := "You are a search result summarizer. Provide a concise summary of the relevant information found in conversation fragments. Focus on key facts, decisions, and outcomes."
	userPrompt := fmt.Sprintf("Summarize these conversation fragments about %q. Preserve key facts, decisions, and outcomes. Be concise.\n\n%s", query, strings.Join(fragments, "\n---\n"))

	return llmClient.CreateMessage(ctx, systemPrompt, userPrompt)
}
