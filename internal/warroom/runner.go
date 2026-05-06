package warroom

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/tools"
)

type LLMAgentRunner struct {
	client       *api.Client
	onFinding    func(sessionID string, agentType DomainAgentType, finding Finding)
	onStatus     func(sessionID string, agentType DomainAgentType, status AgentStatus)
	onLog        func(sessionID string, agentType DomainAgentType, text string)
	mu           sync.Mutex
	activeAgents map[string]context.CancelFunc
}

func NewLLMAgentRunner(client *api.Client) *LLMAgentRunner {
	return &LLMAgentRunner{
		client:       client,
		activeAgents: make(map[string]context.CancelFunc),
	}
}

func (r *LLMAgentRunner) RunAgent(ctx context.Context, agentType DomainAgentType, task string, agentTools []string) (string, error) {
	agent, ok := BuiltInAgents[agentType]
	if !ok {
		return "", fmt.Errorf("unknown agent type: %s", agentType)
	}

	if r.client == nil {
		return "", fmt.Errorf("LLM client not configured")
	}

	systemPrompt := buildAgentSystemPrompt(agent, task)
	agentAPITools := buildAgentAPITools(agentTools)

	messages := []api.MessageParam{
		{Role: "user", Content: task},
	}

	const maxIterations = 5
	var allText strings.Builder

	for iteration := 0; iteration < maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return allText.String(), ctx.Err()
		default:
		}

		req := &api.MessageRequest{
			Model:     r.client.Model,
			MaxTokens: 4096,
			Messages:  messages,
			System: []api.SystemBlock{
				{
					Type: "text",
					Text: systemPrompt,
				},
			},
			Stream: false,
			Tools:  agentAPITools,
		}

		resp, err := r.client.CreateMessageWithSystem(ctx, messages, req.System)
		if err != nil {
			slog.Error("warroom: agent LLM call failed", "agent", agentType, "iteration", iteration, "error", err)
			return allText.String(), err
		}

		var toolUseBlocks []api.ContentBlock
		for _, block := range resp.Content {
			if block.Type == "tool_use" {
				toolUseBlocks = append(toolUseBlocks, block)
			}
			if block.Type == "text" && block.Text != "" {
				allText.WriteString(block.Text)
				allText.WriteString("\n")
				if r.onLog != nil {
					r.onLog("", agentType, block.Text)
				}
			}
		}

		if len(toolUseBlocks) == 0 {
			break
		}

		toolResults := make([]api.ContentBlock, 0, len(toolUseBlocks))
		for _, block := range toolUseBlocks {
			result, toolErr := r.executeAgentTool(ctx, block.Name, block.Input)
			if toolErr != nil {
				toolResults = append(toolResults, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   toolErr.Error(),
					IsError:   true,
				})
			} else {
				resultStr := formatToolResult(result)
				toolResults = append(toolResults, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   resultStr,
				})

				if r.onFinding != nil {
					finding := Finding{
						ID:          uuid.New().String(),
						AgentType:   agentType,
						Category:    "symptom",
						Title:       fmt.Sprintf("%s: %s", agent.Name, block.Name),
						Description: truncateString(resultStr, 500),
						Confidence:  0.5,
						Evidence:    []string{truncateString(resultStr, 200)},
						CreatedAt:   time.Now(),
					}
					r.onFinding("", agentType, finding)
				}
			}
		}

		messages = append(messages, api.MessageParam{
			Role:    "assistant",
			Content: resp.Content,
		})
		messages = append(messages, api.MessageParam{
			Role:    "user",
			Content: toolResults,
		})
	}

	return allText.String(), nil
}

func (r *LLMAgentRunner) RunAgentAsync(ctx context.Context, sessionID string, agentType DomainAgentType, task string, agentTools []string) {
	agentCtx, cancel := context.WithCancel(ctx)
	key := sessionID + ":" + string(agentType)
	r.mu.Lock()
	r.activeAgents[key] = cancel
	r.mu.Unlock()

	if r.onStatus != nil {
		r.onStatus(sessionID, agentType, AgentStatusRunning)
	}

	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.activeAgents, key)
			r.mu.Unlock()
			cancel()
		}()

		result, err := r.RunAgent(agentCtx, agentType, task, agentTools)

		if err != nil && agentCtx.Err() == nil {
			slog.Error("warroom: agent failed", "agent", agentType, "error", err)
			if r.onStatus != nil {
				r.onStatus(sessionID, agentType, AgentStatusFailed)
			}
			return
		}

		if result != "" && r.onFinding != nil {
			finding := Finding{
				ID:          uuid.New().String(),
				AgentType:   agentType,
				Category:    "symptom",
				Title:       fmt.Sprintf("%s investigation result", BuiltInAgents[agentType].Name),
				Description: truncateString(result, 500),
				Confidence:  0.6,
				Evidence:    []string{truncateString(result, 200)},
				CreatedAt:   time.Now(),
			}
			r.onFinding(sessionID, agentType, finding)
		}

		if r.onStatus != nil {
			if agentCtx.Err() != nil {
				r.onStatus(sessionID, agentType, AgentStatusComplete)
			} else {
				r.onStatus(sessionID, agentType, AgentStatusComplete)
			}
		}
	}()
}

func (r *LLMAgentRunner) StopAgent(sessionID string, agentType DomainAgentType) {
	key := sessionID + ":" + string(agentType)
	r.mu.Lock()
	cancel, ok := r.activeAgents[key]
	r.mu.Unlock()
	if ok && cancel != nil {
		cancel()
	}
}

func (r *LLMAgentRunner) executeAgentTool(ctx context.Context, name string, input map[string]any) (any, error) {
	return tools.Execute(ctx, name, input)
}

func buildAgentSystemPrompt(agent DomainAgent, task string) string {
	var sb strings.Builder
	sb.WriteString(agent.SystemPrompt())
	sb.WriteString("\n\nYou are part of a War Room investigating an incident.\n")
	sb.WriteString("Investigation Steps:\n")
	for _, step := range agent.InvestigationSteps {
		sb.WriteString(fmt.Sprintf("- %s\n", step))
	}
	sb.WriteString(fmt.Sprintf("\nFocus areas: %s\n", strings.Join(agent.FocusAreas, ", ")))
	sb.WriteString("\nUse your available tools to investigate. Be concise and focused.\n")
	sb.WriteString("Report your findings clearly with evidence.\n")
	return sb.String()
}

func DomainAgentSystemPrompt(agentType DomainAgentType) string {
	agent, ok := BuiltInAgents[agentType]
	if !ok {
		return ""
	}
	return agent.SystemPrompt()
}

func (a DomainAgent) SystemPrompt() string {
	if a.Description != "" {
		return a.Name + ": " + a.Description
	}
	return a.Name
}

func buildAgentAPITools(toolNames []string) []api.ToolDefinition {
	if len(toolNames) == 0 {
		return api.BuiltinTools
	}

	toolMap := make(map[string]bool, len(toolNames))
	for _, name := range toolNames {
		toolMap[name] = true
	}

	var result []api.ToolDefinition
	for _, td := range api.BuiltinTools {
		if toolMap[td.Name] {
			result = append(result, td)
		}
	}

	if len(result) == 0 {
		return api.BuiltinTools
	}

	return result
}

func formatToolResult(result any) string {
	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", result)
		}
		return string(data)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
