package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/tools"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type APICallMsg struct {
	text string
}

type APIResponseMsg struct {
	text          string
	thinkingBlock string
	tokens        int
}

type UserInputMsg struct {
	text string
}

type CommandMsg struct {
	cmd string
}

type OutputMsg struct {
	text string
}

type ErrorMsg struct {
	err string
}

type StreamChunkMsg struct {
	chunk string
}

type TickMsg struct{}

type streamingState struct {
	mu       sync.Mutex
	text     strings.Builder
	thinking strings.Builder
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m Model) processInput(input string) tea.Cmd {
	return func() tea.Msg {
		if strings.HasPrefix(input, "/") {
			return CommandMsg{cmd: input}
		}

		if m.apiClient == nil {
			return ErrorMsg{err: "No API key configured. Use /set-api-key or set ANTHROPIC_API_KEY"}
		}

		processedInput := input
		if DetectFileReferences(input) {
			_, processedInput = ParseFileReferences(input, m.workDir)
		}

		return APICallMsg{text: processedInput}
	}
}

func (m *Model) callAPI(input string) tea.Cmd {
	return func() tea.Msg {
		m.apiMu.Lock()
		defer m.apiMu.Unlock()

		userMsg := api.Message{
			Role:    "user",
			Content: []api.ContentBlock{{Type: "text", Text: input}},
		}
		m.messages = append(m.messages, userMsg)

		req := &api.MessageRequest{
			Model:     m.model,
			MaxTokens: 4096,
			Messages:  m.messages,
			Tools:     m.buildToolDefinitions(),
			System:    m.buildSystemPrompt(),
		}

		m.streamState.mu.Lock()
		m.streamState.text.Reset()
		m.streamState.thinking.Reset()
		m.streamState.mu.Unlock()

		parser := api.NewStreamMessageParser()

		var err error
		if m.apiClient.IsOpenAI {
			err = m.apiClient.StreamMessageOpenAI(context.Background(), req, func(event string, data []byte) error {
				result, err := parser.HandleEvent(event, data)
				if err != nil {
					return err
				}

				m.streamState.mu.Lock()
				if result.TextDelta != "" {
					m.streamState.text.WriteString(result.TextDelta)
				}
				if result.ThinkingDelta != "" {
					m.streamState.thinking.WriteString(result.ThinkingDelta)
				}
				m.streamState.mu.Unlock()

				return nil
			})
		} else {
			err = m.apiClient.StreamMessageSSE(context.Background(), req, func(event string, data []byte) error {
				result, err := parser.HandleEvent(event, data)
				if err != nil {
					return err
				}

				m.streamState.mu.Lock()
				if result.TextDelta != "" {
					m.streamState.text.WriteString(result.TextDelta)
				}
				if result.ThinkingDelta != "" {
					m.streamState.thinking.WriteString(result.ThinkingDelta)
				}
				m.streamState.mu.Unlock()

				return nil
			})
		}

		if err != nil {
			return ErrorMsg{err: err.Error()}
		}

		m.streamState.mu.Lock()
		finalResponse := m.streamState.text.String()
		thinkingText := m.streamState.thinking.String()
		m.streamState.text.Reset()
		m.streamState.thinking.Reset()
		m.streamState.mu.Unlock()

		var thinkingBlock string
		if thinkingText != "" {
			thinkingBlock = formatThinkingBlock(thinkingText, m.showThinking, m.width-2)
		}

		assistantMsg := api.Message{
			Role:    "assistant",
			Content: []api.ContentBlock{{Type: "text", Text: finalResponse}},
		}
		m.messages = append(m.messages, assistantMsg)

		if mcpResults := m.executeMCPCalls(finalResponse); len(mcpResults) > 0 {
			return APIResponseMsg{text: finalResponse + mcpResults, thinkingBlock: thinkingBlock, tokens: 0}
		}

		return APIResponseMsg{text: finalResponse, thinkingBlock: thinkingBlock, tokens: 0}
	}
}

func (m *Model) executeMCPCalls(response string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile("```mcp__([a-zA-Z0-9_-]+)__([a-zA-Z0-9_-]+)\\s*([\\s\\S]*?)```"),
		regexp.MustCompile("`mcp__([a-zA-Z0-9_-]+)__([a-zA-Z0-9_-]+)\\s*([\\s\\S]*?)`"),
		regexp.MustCompile("mcp__([a-zA-Z0-9_-]+)__([a-zA-Z0-9_-]+)\\s*(\\{[^}]*\\})?"),
	}

	var matches [][]string
	for _, p := range patterns {
		matches = p.FindAllStringSubmatch(response, -1)
		if len(matches) > 0 {
			break
		}
	}

	if len(matches) == 0 {
		return ""
	}

	mcpRegistry := tools.GetMCPRegistry()
	var resultBuilder strings.Builder
	executed := map[string]bool{}

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		serverName := match[1]
		toolName := match[2]

		callKey := serverName + "__" + toolName
		if executed[callKey] {
			continue
		}
		executed[callKey] = true

		var input map[string]any
		if len(match) > 3 && strings.TrimSpace(match[3]) != "" {
			paramStr := strings.TrimSpace(match[3])
			if strings.HasPrefix(paramStr, "{") {
				json.Unmarshal([]byte(paramStr), &input)
			}
		}
		if input == nil {
			input = make(map[string]any)
		}

		client, ok := mcpRegistry.Get(serverName)
		if !ok {
			resultBuilder.WriteString(fmt.Sprintf("\n\n❌ MCP server '%s' not connected", serverName))
			continue
		}

		result, err := client.InvokeTool(context.Background(), toolName, input)
		if err != nil {
			resultBuilder.WriteString(fmt.Sprintf("\n\n❌ mcp__%s__%s error: %v", serverName, toolName, err))
			continue
		}

		resultStr := formatMCPResult(result)
		if len(resultStr) > 5000 {
			resultStr = resultStr[:5000] + "\n  ... (truncated)"
		}
		resultBuilder.WriteString(fmt.Sprintf("\n\n📤 mcp__%s__%s result:\n%s", serverName, toolName, resultStr))

		toolResultMsg := api.Message{
			Role:    "user",
			Content: []api.ContentBlock{{Type: "text", Text: fmt.Sprintf("Tool mcp__%s__%s returned:\n%s\n\nPlease summarize this result for the user in a clear, readable format.", serverName, toolName, resultStr)}},
		}
		m.messages = append(m.messages, toolResultMsg)
	}

	return resultBuilder.String()
}

func formatMCPResult(result any) string {
	callResult, ok := result.(*sdk.CallToolResult)
	if !ok {
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b)
	}

	if callResult.IsError {
		var texts []string
		for _, c := range callResult.Content {
			if tc, ok := c.(*sdk.TextContent); ok && tc.Text != "" {
				texts = append(texts, tc.Text)
			}
		}
		return "Error: " + strings.Join(texts, "\n")
	}

	var parts []string
	for _, c := range callResult.Content {
		switch content := c.(type) {
		case *sdk.TextContent:
			text := strings.TrimSpace(content.Text)
			if text == "" {
				continue
			}

			var parsed any
			if err := json.Unmarshal([]byte(text), &parsed); err == nil {
				formatted, err := json.MarshalIndent(parsed, "", "  ")
				if err == nil {
					parts = append(parts, string(formatted))
					continue
				}
			}
			parts = append(parts, text)
		case *sdk.ImageContent:
			parts = append(parts, "[Image]")
		default:
			parts = append(parts, fmt.Sprintf("%v", content))
		}
	}

	return strings.Join(parts, "\n")
}

func (m *Model) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are SmartClaw, a helpful AI assistant. You have access to various tools to help the user.")

	mcpRegistry := tools.GetMCPRegistry()
	connectedServers := mcpRegistry.ListConnected()
	if len(connectedServers) > 0 {
		sb.WriteString("\n\nYou have access to MCP (Model Context Protocol) servers with the following tools:")
		for _, serverName := range connectedServers {
			client, ok := mcpRegistry.Get(serverName)
			if !ok || !client.IsReady() {
				continue
			}
			conn := client.GetConnection()
			if conn == nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("\n\n## %s Server\n", serverName))
			for _, mcpTool := range conn.Tools {
				sb.WriteString(fmt.Sprintf("- **mcp__%s__%s**: %s\n", serverName, mcpTool.Name, mcpTool.Description))
			}
		}
		sb.WriteString("\nTo use an MCP tool, include a tool_use block with the tool name (e.g. `mcp__sopa__list_nodes`) and the required input parameters.")
	}

	return sb.String()
}

func (m *Model) buildToolDefinitions() []api.ToolDefinition {
	complexity := tools.AssessQueryComplexity(m.lastInput)
	toolDefs := make([]api.ToolDefinition, 0)

	for _, t := range tools.SelectToolset(context.Background(), complexity) {
		toolDefs = append(toolDefs, api.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}

	mcpRegistry := tools.GetMCPRegistry()
	for _, serverName := range mcpRegistry.ListConnected() {
		client, ok := mcpRegistry.Get(serverName)
		if !ok || !client.IsReady() {
			continue
		}
		conn := client.GetConnection()
		if conn == nil {
			continue
		}
		for _, mcpTool := range conn.Tools {
			inputSchema, _ := mcpTool.InputSchema.(map[string]any)
			if inputSchema == nil {
				inputSchema = map[string]any{"type": "object"}
			}
			toolDefs = append(toolDefs, api.ToolDefinition{
				Name:        fmt.Sprintf("mcp__%s__%s", serverName, mcpTool.Name),
				Description: mcpTool.Description,
				InputSchema: inputSchema,
			})
		}
	}

	return toolDefs
}
