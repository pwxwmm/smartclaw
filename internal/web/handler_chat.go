package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/pool"
	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
	"github.com/instructkr/smartclaw/internal/utils"
)

func (h *Handler) handleChat(client *Client, msg WSMessage) {
	if h.apiClient == nil {
		h.sendError(client, "API client not configured. Check ~/.smartclaw/config.json")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.cancelFuncs[client.ID] = cancel
	h.mu.Unlock()
	defer func() {
		cancel()
		h.mu.Lock()
		delete(h.cancelFuncs, client.ID)
		h.mu.Unlock()
	}()

	if msg.Model != "" {
		h.mu.Lock()
		h.clientModels[client.ID] = msg.Model
		h.mu.Unlock()
	}

	reqClient := h.clientForRequest(client.ID)

	h.clientSessMu.RLock()
	sess := h.clientSess[client.ID]
	h.clientSessMu.RUnlock()
	if sess == nil {
		if h.sessMgr != nil {
			sess = h.sessMgr.NewSession(reqClient.Model, client.UserID)
		} else {
			sess = &session.Session{ID: "temp", UserID: client.UserID, Model: reqClient.Model, Messages: []session.Message{}}
		}
		h.clientSessMu.Lock()
		h.clientSess[client.ID] = sess
		h.clientSessMu.Unlock()
	}

	sess.AddMessage("user", msg.Content)
	h.syncMessageToStore(sess, "user", msg.Content, 0)
	h.autoSaveSession(sess)

	h.sendToClient(client, WSResponse{Type: "session_active", ID: sess.ID, Title: sess.Title})

	messages := make([]api.Message, 0, len(sess.Messages))
	for _, m := range sess.Messages {
		messages = append(messages, api.Message{Role: m.Role, Content: m.Content})
	}

	if len(msg.Images) > 0 {
		var imageData []struct {
			Data string `json:"data"`
			Type string `json:"type"`
		}
		if json.Unmarshal(msg.Images, &imageData) == nil && len(imageData) > 0 {
			if len(messages) > 0 {
				lastMsg := messages[len(messages)-1]
				if reqClient.IsOpenAI {
					var contentBlocks []map[string]any
					for _, img := range imageData {
						contentBlocks = append(contentBlocks, map[string]any{
							"type": "image_url",
							"image_url": map[string]string{
								"url": "data:" + img.Type + ";base64," + img.Data,
							},
						})
					}
					contentBlocks = append(contentBlocks, map[string]any{
						"type": "text",
						"text": lastMsg.Content,
					})
					messages[len(messages)-1] = api.Message{Role: "user", Content: contentBlocks}
				} else {
					var contentBlocks []map[string]any
					for _, img := range imageData {
						contentBlocks = append(contentBlocks, map[string]any{
							"type": "image",
							"source": map[string]any{
								"type":       "base64",
								"media_type": img.Type,
								"data":       img.Data,
							},
						})
					}
					contentBlocks = append(contentBlocks, map[string]any{
						"type": "text",
						"text": lastMsg.Content,
					})
					messages[len(messages)-1] = api.Message{Role: "user", Content: contentBlocks}
				}
			}
		}
	}

	var systemPrompt string
	if h.memMgr != nil {
		userMem := h.memMgr.ForUser(client.UserID)
		systemPrompt = userMem.BuildPrompt()
	} else {
		systemPrompt = h.prompt.Build()
	}

	fullContentBuilder := pool.GetBuffer()
	defer pool.PutBuffer(fullContentBuilder)
	var openaiOutputChars int

	if reqClient.IsOpenAI {
		req := &api.MessageRequest{
			Model:     reqClient.Model,
			MaxTokens: 8192,
			Messages:  messages,
			System:    systemPrompt,
		}

		err := reqClient.StreamMessageOpenAI(ctx, req, func(event string, data []byte) error {
			switch event {
			case "content_block_delta":
				var payload struct {
					Delta struct {
						Type     string `json:"type"`
						Text     string `json:"text,omitempty"`
						Thinking string `json:"thinking,omitempty"`
					} `json:"delta"`
				}
				if json.Unmarshal(data, &payload) == nil {
					if payload.Delta.Type == "text_delta" && payload.Delta.Text != "" {
						fullContentBuilder.WriteString(payload.Delta.Text)
						openaiOutputChars += len(payload.Delta.Text)
						h.sendToClient(client, WSResponse{Type: "token", Content: payload.Delta.Text})
					}
				}
			case "message_stop":
				fullContent := fullContentBuilder.String()
				sess.AddMessage("assistant", fullContent)
				h.syncMessageToStore(sess, "assistant", fullContent, 0)
				h.autoSaveSession(sess)
				tokens := openaiOutputChars / 4
				model := reqClient.Model
				inputTokens := tokens / 3
				outputTokens := tokens * 2 / 3
				cost, breakdown := h.costGuard.CalculateCost(model, inputTokens, outputTokens)
				h.sendToClient(client, WSResponse{Type: "done", Tokens: tokens, Cost: cost, CostBreakdown: &breakdown, Model: model})
			}
			return nil
		})
		if err != nil {
			h.sendError(client, fmt.Sprintf("API error: %v", err))
		}
		return
	}

	// Anthropic SSE streaming with agentic tool loop
	const maxIterations = 10
	allTextBuilder := pool.GetBuffer()
	defer pool.PutBuffer(allTextBuilder)
	var totalInputTokens, totalOutputTokens int

	for iteration := 0; iteration < maxIterations; iteration++ {
		req := &api.MessageRequest{
			Model:     reqClient.Model,
			MaxTokens: 8192,
			Messages:  messages,
			System:    systemPrompt,
			Stream:    true,
			Tools:     api.BuiltinTools,
		}

		parser := api.NewStreamMessageParser()
		iterTextBuilder := pool.GetBuffer()

		err := reqClient.StreamMessageSSE(ctx, req, func(event string, data []byte) error {
			result, err := parser.HandleEvent(event, data)
			if err != nil {
				return err
			}

			if result.Error != nil {
				return result.Error
			}

			if result.TextDelta != "" {
				iterTextBuilder.WriteString(result.TextDelta)
				h.sendToClient(client, WSResponse{Type: "token", Content: result.TextDelta})
			}

			if result.ThinkingDelta != "" && h.showThinking {
				h.sendToClient(client, WSResponse{Type: "thinking", Content: result.ThinkingDelta})
			}

			return nil
		})

		if err != nil {
			pool.PutBuffer(iterTextBuilder)
			h.sendError(client, fmt.Sprintf("API error: %v", err))
			return
		}

		resp := parser.GetMessage()
		totalInputTokens += resp.Usage.InputTokens
		totalOutputTokens += resp.Usage.OutputTokens
		allTextBuilder.WriteString(iterTextBuilder.String())
		pool.PutBuffer(iterTextBuilder)

		blocks := parser.GetContentBlocks()
		var toolUseBlocks []api.ContentBlock
		for _, block := range blocks {
			if block.Type == "tool_use" || block.Type == "server_tool_use" {
				toolUseBlocks = append(toolUseBlocks, block)
			}
		}

		if len(toolUseBlocks) == 0 {
			break
		}

		toolResults := make([]api.ContentBlock, 0, len(toolUseBlocks))
		for _, block := range toolUseBlocks {
			annotations := generateEditAnnotations(block.Name, block.Input)
			h.sendToClient(client, WSResponse{
				Type:        "tool_start",
				ID:          block.ID,
				Tool:        block.Name,
				Input:       block.Input,
				Annotations: annotations,
			})

			isAgentTool := block.Name == "agent"
			if isAgentTool {
				h.sendToClient(client, WSResponse{
					Type:     "agent_status",
					ID:       block.ID,
					Status:   "running",
					Progress: 0.1,
				})
				h.broadcastAgentStatus(block.ID, "running")
			}

			if h.needsApproval(client.ID, block.Name, block.Input) {
				approved, approvalErr := h.requestApproval(client, block.ID, block.Name, block.Input)
				if approvalErr != nil || !approved {
					reason := "Tool execution denied by user"
					if approvalErr != nil {
						reason = approvalErr.Error()
					}
					utils.Go(func() { observability.AuditDenial(block.Name, block.ID, client.ID, reason) })
					toolResults = append(toolResults, api.ContentBlock{
						Type:      "tool_result",
						ToolUseID: block.ID,
						Content:   reason,
						IsError:   true,
					})
					h.sendToClient(client, WSResponse{
						Type:   "tool_output",
						ID:     block.ID,
						Output: reason,
					})
					h.sendToClient(client, WSResponse{
						Type:     "tool_end",
						ID:       block.ID,
						Duration: 0,
					})
					if isAgentTool {
						h.sendToClient(client, WSResponse{
							Type:     "agent_status",
							ID:       block.ID,
							Status:   "error",
							Progress: 0,
						})
						h.broadcastAgentStatus(block.ID, "error")
					}
					continue
				}
			}

			startTime := time.Now()
			output, toolErr := h.executeTool(ctx, block.Name, block.Input)
			duration := time.Since(startTime).Milliseconds()

			if isAgentTool {
				if toolErr != nil {
					h.sendToClient(client, WSResponse{
						Type:     "agent_status",
						ID:       block.ID,
						Status:   "failed",
						Progress: 0,
					})
					h.broadcastAgentStatus(block.ID, "error")
				} else {
					h.sendToClient(client, WSResponse{
						Type:     "agent_status",
						ID:       block.ID,
						Status:   "done",
						Progress: 1.0,
					})
					h.broadcastAgentStatus(block.ID, "done")
				}
			}

			if toolErr != nil {
				toolResults = append(toolResults, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   toolErr.Error(),
					IsError:   true,
				})
				h.sendToClient(client, WSResponse{
					Type:   "tool_output",
					ID:     block.ID,
					Output: toolErr.Error(),
				})
			} else {
				toolResults = append(toolResults, api.ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   output,
				})
				h.sendToClient(client, WSResponse{
					Type:        "tool_output",
					ID:          block.ID,
					Output:      output,
					Annotations: annotations,
				})
			}

			h.sendToClient(client, WSResponse{
				Type:     "tool_end",
				ID:       block.ID,
				Duration: duration,
			})
		}

		assistantContent := make([]api.ContentBlock, len(blocks))
		copy(assistantContent, blocks)
		messages = append(messages, api.MessageParam{
			Role:    "assistant",
			Content: assistantContent,
		})

		messages = append(messages, api.MessageParam{
			Role:    "user",
			Content: toolResults,
		})
	}

	allTextContent := allTextBuilder.String()
	sess.AddMessage("assistant", allTextContent)
	h.syncMessageToStore(sess, "assistant", allTextContent, 0)
	h.autoSaveSession(sess)

	totalTokens := totalInputTokens + totalOutputTokens
	model := reqClient.Model
	cost, breakdown := h.costGuard.CalculateCost(model, totalInputTokens, totalOutputTokens)
	h.sendToClient(client, WSResponse{
		Type:          "done",
		Tokens:        totalTokens,
		Cost:          cost,
		CostBreakdown: &breakdown,
		Model:         model,
	})
}

func (h *Handler) autoSaveSession(sess *session.Session) {
	if h.dataStore != nil {
		storeSess := &store.Session{
			ID:        sess.ID,
			UserID:    sess.UserID,
			Source:    "web",
			Model:     sess.Model,
			Title:     sess.Title,
			Tokens:    sess.Tokens,
			Cost:      sess.Cost,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
		}
		h.wg.Add(1)
		utils.Go(func() {
			defer h.wg.Done()
			if err := h.dataStore.UpsertSession(context.Background(), storeSess); err != nil {
				slog.Warn("failed to upsert session", "error", err, "session_id", storeSess.ID)
			}
		})
	}
	if h.sessMgr != nil {
		h.wg.Add(1)
		utils.Go(func() {
			defer h.wg.Done()
			h.sessMgr.Save(sess)
		})
	}
}

func (h *Handler) syncMessageToStore(sess *session.Session, role, content string, tokens int) {
	if h.dataStore == nil {
		return
	}
	h.wg.Add(1)
	utils.Go(func() {
		defer h.wg.Done()
		if err := h.dataStore.InsertSessionMessage(sess.ID, role, content, tokens); err != nil {
			slog.Warn("failed to insert session message", "error", err, "session_id", sess.ID)
		}
	})
}

func (h *Handler) Wait() {
	h.wg.Wait()
}

func (h *Handler) executeTool(ctx context.Context, name string, input map[string]any) (string, error) {
	prepared := make(map[string]any, len(input))
	for k, v := range input {
		prepared[k] = v
	}

	for _, key := range []string{"path", "file_path", "filepath", "filename", "directory", "dir"} {
		if v, ok := prepared[key].(string); ok && v != "" && !filepath.IsAbs(v) {
			prepared[key] = filepath.Join(h.workDir, v)
		}
	}

	if name == "bash" {
		if _, ok := prepared["workdir"]; !ok {
			prepared["workdir"] = h.workDir
		}
	}

	result, err := tools.Execute(ctx, name, prepared)
	if err != nil {
		return "", err
	}

	return resultToString(result), nil
}

func resultToString(result any) string {
	if result == nil {
		return ""
	}
	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		pe := pool.GetJSONEncoder(nil)
		if pe.Encode(result) != nil {
			pool.PutJSONEncoder(pe)
			return fmt.Sprint(v)
		}
		s := string(pe.Bytes())
		pool.PutJSONEncoder(pe)
		return s
	}
}

func generateEditAnnotations(toolName string, input map[string]any) []DiffAnnotation {
	if toolName != "edit_file" && toolName != "diff_edit" && toolName != "line_edit" {
		return nil
	}

	oldStr, _ := input["old_string"].(string)
	newStr, _ := input["new_string"].(string)
	if oldStr == "" && newStr == "" {
		return nil
	}

	reason := annotateDiff(oldStr, newStr)
	return []DiffAnnotation{{HunkIndex: 0, Reason: reason}}
}

func annotateDiff(oldStr, newStr string) string {
	oldLower := strings.ToLower(oldStr)
	newLower := strings.ToLower(newStr)

	hasOldErr := strings.Contains(oldLower, "error") || strings.Contains(oldLower, "err")
	hasNewErr := strings.Contains(newLower, "error") || strings.Contains(newLower, "err")
	hasNewReturn := strings.Contains(newLower, "return")

	if !hasOldErr && hasNewErr {
		return "Added error handling for failure path"
	}
	if !hasNewErr && hasOldErr {
		return "Simplified error handling"
	}
	if hasNewReturn && !strings.Contains(oldLower, "return") {
		return "Added early return to prevent fall-through"
	}
	if len(newStr) > len(oldStr) {
		return "Extended functionality with additional logic"
	}
	if len(oldStr) > len(newStr) {
		return "Simplified by removing redundant code"
	}
	if isRenameOnly(oldStr, newStr) {
		return "Renamed for clarity"
	}
	return "Modified logic"
}

func isRenameOnly(oldStr, newStr string) bool {
	oldTokens := tokenizeIdentifiers(oldStr)
	newTokens := tokenizeIdentifiers(newStr)
	if len(oldTokens) != len(newTokens) {
		return false
	}
	renames := 0
	for i := range oldTokens {
		if oldTokens[i] != newTokens[i] {
			renames++
		}
	}
	return renames > 0 && renames <= 2
}

func tokenizeIdentifiers(s string) []string {
	var tokens []string
	var cur strings.Builder
	for _, r := range s {
		if isIdentRune(r) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
