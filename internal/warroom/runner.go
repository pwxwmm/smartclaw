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
	coordinator  *WarRoomCoordinator
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

func (r *LLMAgentRunner) SetCoordinator(c *WarRoomCoordinator) {
	r.coordinator = c
}

func (r *LLMAgentRunner) RunAgent(ctx context.Context, agentType DomainAgentType, task string, agentTools []string, opts ...RunAgentOptions) (string, error) {
	agent, ok := BuiltInAgents[agentType]
	if !ok {
		return "", fmt.Errorf("unknown agent type: %s", agentType)
	}

	if r.client == nil {
		return "", fmt.Errorf("LLM client not configured")
	}

	var opt RunAgentOptions
	if len(opts) > 0 {
		opt = opts[0]
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

		currentSystemPrompt := systemPrompt

		if opt.BlackboardSnapshotFn != nil {
			if snapshot := opt.BlackboardSnapshotFn(); snapshot != "" {
				currentSystemPrompt += "\n\n" + snapshot
			}
		}

		if opt.SessionID != "" && r.coordinator != nil {
			if req, ok := r.coordinator.TryRecvHandoff(opt.SessionID); ok {
				handoffCtx := fmt.Sprintf(
					"\n\n[HANDOFF REQUEST from %s (priority: %s)]: %s\nContext: %s\nPlease address this request in your response.",
					req.FromAgent, req.Priority, req.Question, req.Context,
				)
				currentSystemPrompt += handoffCtx

				go func(sessionID string, request HandoffRequest, at DomainAgentType) {
					time.Sleep(5 * time.Second)
					resp := HandoffResponse{
						RequestID:  request.ID,
						FromAgent:  at,
						ToAgent:    request.FromAgent,
						Answer:     "Handoff received but could not process in time",
						Confidence: 0.3,
					}
					if r.coordinator != nil {
						r.coordinator.SendHandoffResponse(sessionID, resp)
					}
				}(opt.SessionID, req, agentType)
			}
		}

		req := &api.MessageRequest{
			Model:     r.client.Model,
			MaxTokens: 4096,
			Messages:  messages,
			System: []api.SystemBlock{
				{
					Type: "text",
					Text: currentSystemPrompt,
				},
			},
			Stream: false,
			Tools:  agentAPITools,
		}

		resp, err := r.client.CreateMessageWithTools(ctx, messages, req.System, agentAPITools)
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
					r.onLog(opt.SessionID, agentType, block.Text)
				}
			}
		}

		if len(toolUseBlocks) == 0 {
			break
		}

		toolResults := make([]api.ContentBlock, 0, len(toolUseBlocks))
		for _, block := range toolUseBlocks {
			var result any
			var toolErr error

			if block.Name == "warroom_handoff" && opt.SessionID != "" && r.coordinator != nil {
				result, toolErr = r.executeHandoffTool(ctx, opt.SessionID, agentType, block.Input)
			} else if block.Name == "warroom_evaluate" && opt.SessionID != "" && r.coordinator != nil {
				result, toolErr = r.executeEvaluateTool(ctx, opt.SessionID, agentType, block.Input)
			} else if block.Name == "warroom_blackboard_write" && opt.SessionID != "" && r.coordinator != nil {
				result, toolErr = r.executeBlackboardWriteTool(opt.SessionID, agentType, block.Input)
			} else {
				result, toolErr = r.executeAgentTool(ctx, block.Name, block.Input)
			}

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
					r.onFinding(opt.SessionID, agentType, finding)
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

func (r *LLMAgentRunner) executeHandoffTool(ctx context.Context, sessionID string, fromAgent DomainAgentType, input map[string]any) (any, error) {
	targetAgent, _ := input["target_agent"].(string)
	question, _ := input["question"].(string)
	contextStr, _ := input["context"].(string)
	priority, _ := input["priority"].(string)

	if priority == "" {
		priority = "medium"
	}
	if targetAgent == "" {
		return nil, fmt.Errorf("target_agent is required")
	}
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	req := HandoffRequest{
		FromAgent: fromAgent,
		ToAgent:   DomainAgentType(targetAgent),
		Question:  question,
		Context:   contextStr,
		Priority:  priority,
	}

	resp, err := r.coordinator.RequestHandoff(ctx, sessionID, req)
	if err != nil {
		return fmt.Sprintf("Handoff failed: %v", err), nil
	}

	return fmt.Sprintf("Response from %s (confidence: %.2f): %s", resp.FromAgent, resp.Confidence, resp.Answer), nil
}

func (r *LLMAgentRunner) executeEvaluateTool(ctx context.Context, sessionID string, agentType DomainAgentType, input map[string]any) (any, error) {
	findingID, _ := input["finding_id"].(string)
	agrees, _ := input["agrees"].(bool)
	adjustment, _ := input["confidence_adjustment"].(float64)
	notes, _ := input["notes"].(string)

	if findingID == "" {
		return nil, fmt.Errorf("finding_id is required")
	}

	if adjustment == 0 {
		if agrees {
			adjustment = 0.05
		} else {
			adjustment = -0.05
		}
	}

	r.coordinator.EvolveConfidence(sessionID, findingID, adjustment, fmt.Sprintf("%s: %s", agentType, notes))

	session := r.coordinator.GetSession(sessionID)
	if session != nil {
		for i := range session.Findings {
			if session.Findings[i].ID == findingID {
				xref := CrossReference{
					FindingID:    findingID,
					ReferencedBy: agentType,
					Agrees:       agrees,
					Notes:        notes,
				}
				session.Findings[i].CrossReferences = append(session.Findings[i].CrossReferences, xref)
				break
			}
		}
	}

	return fmt.Sprintf("Finding %s evaluated: agrees=%v, adjustment=%.2f", findingID, agrees, adjustment), nil
}

func (r *LLMAgentRunner) executeBlackboardWriteTool(sessionID string, agentType DomainAgentType, input map[string]any) (any, error) {
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)
	category, _ := input["category"].(string)

	if key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if value == "" {
		return nil, fmt.Errorf("value is required")
	}
	if category == "" {
		category = "observation"
	}

	bb, ok := r.coordinator.GetBlackboard(sessionID)
	if !ok || bb == nil {
		return nil, fmt.Errorf("blackboard not found for session %s", sessionID)
	}

	bb.WriteEntry(BlackboardEntry{
		Key:       key,
		Value:     value,
		Author:    agentType,
		Category:  category,
		Timestamp: time.Now(),
	})

	return fmt.Sprintf("Written to blackboard: [%s] %s (category: %s)", key, value, category), nil
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

		opts := RunAgentOptions{
			SessionID:            sessionID,
			BlackboardSnapshotFn: func() string { return r.coordinator.getBlackboardSnapshot(sessionID) },
		}

		result, err := r.RunAgent(agentCtx, agentType, task, agentTools, opts)

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
	sb.WriteString("\nCollaboration tools available:\n")
	sb.WriteString("- warroom_handoff: Ask another domain expert a specific question\n")
	sb.WriteString("- warroom_evaluate: Agree/disagree with findings from other agents\n")
	sb.WriteString("- warroom_blackboard_write: Share observations with other agents on the shared blackboard\n")
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
	baseTools := api.BuiltinTools

	if len(toolNames) > 0 {
		toolMap := make(map[string]bool, len(toolNames))
		for _, name := range toolNames {
			toolMap[name] = true
		}

		var result []api.ToolDefinition
		for _, td := range baseTools {
			if toolMap[td.Name] {
				result = append(result, td)
			}
		}

		if len(result) == 0 {
			result = baseTools
		}
		baseTools = result
	}

	warroomTools := []api.ToolDefinition{
		{
			Name:        "warroom_handoff",
			Description: "Request another agent in the War Room to investigate a specific question. Use this when you need information from a different domain expert. Session ID is injected automatically.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target_agent": map[string]any{
						"type":        "string",
						"description": "The agent to ask (network, database, infra, app, security, training, inference)",
					},
					"question": map[string]any{
						"type":        "string",
						"description": "The specific question to ask the other agent",
					},
					"context": map[string]any{
						"type":        "string",
						"description": "Additional context for the question",
					},
					"priority": map[string]any{
						"type":        "string",
						"description": "Priority level: low, medium, or high",
					},
				},
				"required": []string{"target_agent", "question"},
			},
		},
		{
			Name:        "warroom_evaluate",
			Description: "Evaluate a finding from another agent. Use this to agree or disagree with findings and adjust their confidence. Session ID is injected automatically.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"finding_id": map[string]any{
						"type":        "string",
						"description": "The ID of the finding to evaluate",
					},
					"agrees": map[string]any{
						"type":        "boolean",
						"description": "Whether you agree with this finding",
					},
					"confidence_adjustment": map[string]any{
						"type":        "number",
						"description": "Confidence adjustment (-0.3 to +0.3)",
					},
					"notes": map[string]any{
						"type":        "string",
						"description": "Explanation for your evaluation",
					},
				},
				"required": []string{"finding_id", "agrees"},
			},
		},
		{
			Name:        "warroom_blackboard_write",
			Description: "Write an observation or finding to the shared blackboard so other agents can see it. Use this to share important observations with the team.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
						"type":        "string",
						"description": "A short key for this observation (e.g. 'gpu_memory_status', 'error_pattern')",
					},
					"value": map[string]any{
						"type":        "string",
						"description": "The observation content",
					},
					"category": map[string]any{
						"type":        "string",
						"description": "Category: observation, metric, log_excerpt, hypothesis",
					},
				},
				"required": []string{"key", "value"},
			},
		},
	}

	return append(baseTools, warroomTools...)
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
