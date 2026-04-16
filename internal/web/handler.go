package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/commands"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/permissions"
	"github.com/instructkr/smartclaw/internal/provider"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
)

type Handler struct {
	hub          *Hub
	workDir      string
	apiClient    *api.Client
	router       *provider.Router
	sessMgr      *session.Manager
	dataStore    *store.Store
	memMgr       *memory.MemoryManager
	showThinking bool
	clientSess   map[string]*session.Session
	clientSessMu sync.RWMutex
	prompt       *runtime.PromptBuilder
	approvalGate *permissions.ApprovalGate
	costGuard    *costguard.CostGuard
	clientModels map[string]string

	mu               sync.Mutex
	pendingApprovals map[string]chan bool
	autoApproved     map[string]map[string]bool
	cancelFuncs      map[string]context.CancelFunc
}

func NewHandler(hub *Hub, workDir string, apiClient *api.Client) *Handler {
	mgr, err := session.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: session manager init failed: %v\n", err)
	}

	var dataStore *store.Store
	if ds, err := store.NewStore(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: SQLite store init failed, using JSON sessions: %v\n", err)
	} else {
		dataStore = ds
	}

	showThinking := loadShowThinking()

	resolver, err := provider.LoadFromConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: provider config load failed: %v\n", err)
		resolver = provider.NewResolver()
	}
	router := provider.NewRouter(resolver)

	if apiClient != nil {
		p := &provider.Provider{
			Name:    "default",
			APIKey:  apiClient.APIKey,
			BaseURL: apiClient.BaseURL,
			Model:   apiClient.Model,
		}
		if apiClient.IsOpenAI {
			p.Mode = provider.ModeChatCompletions
		} else {
			p.Mode = provider.ModeAnthropicMessages
		}
		resolver.Register("default", p)
	} else if client, err := router.PrimaryClient(); err == nil {
		apiClient = client
	}

	if apiClient == nil {
		apiKey, baseURL, model := loadAPIConfig()
		if apiKey != "" {
			apiClient = api.NewClientWithModel(apiKey, baseURL, model)
			if isOpenAI() {
				apiClient.SetOpenAI(true)
			}
		}
	}

	h := &Handler{
		hub:              hub,
		workDir:          workDir,
		apiClient:        apiClient,
		router:           router,
		sessMgr:          mgr,
		dataStore:        dataStore,
		showThinking:     showThinking,
		clientSess:       make(map[string]*session.Session),
		prompt:           runtime.NewPromptBuilder(),
		approvalGate:     permissions.NewApprovalGate(),
		costGuard:        costguard.NewCostGuard(costguard.DefaultBudgetConfig()),
		clientModels:     make(map[string]string),
		pendingApprovals: make(map[string]chan bool),
		autoApproved:     make(map[string]map[string]bool),
		cancelFuncs:      make(map[string]context.CancelFunc),
	}

	_ = h.approvalGate.LoadConfigFromDefaultPath()

	return h
}

func loadAPIConfig() (apiKey, baseURL, model string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		return
	}

	var cfg struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		apiKey = cfg.APIKey
		baseURL = cfg.BaseURL
		model = cfg.Model
	}

	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if model == "" {
		model = "sre-model"
	}
	return
}

func isOpenAI() bool {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	var cfg struct {
		OpenAI bool `json:"openai"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		return cfg.OpenAI
	}
	return false
}

func loadShowThinking() bool {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	var cfg struct {
		ShowThinking bool `json:"show_thinking"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		return cfg.ShowThinking
	}
	return false
}

type WSMessage struct {
	Type    string          `json:"type"`
	Content string          `json:"content,omitempty"`
	Name    string          `json:"name,omitempty"`
	Args    []string        `json:"args,omitempty"`
	Path    string          `json:"path,omitempty"`
	ID      string          `json:"id,omitempty"`
	Title   string          `json:"title,omitempty"`
	Model   string          `json:"model,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type WSResponse struct {
	Type          string                   `json:"type"`
	Content       string                   `json:"content,omitempty"`
	Tool          string                   `json:"tool,omitempty"`
	Input         any                      `json:"input,omitempty"`
	Output        string                   `json:"output,omitempty"`
	Duration      int64                    `json:"duration,omitempty"`
	ID            string                   `json:"id,omitempty"`
	Title         string                   `json:"title,omitempty"`
	Status        string                   `json:"status,omitempty"`
	Progress      float64                  `json:"progress,omitempty"`
	Tokens        int                      `json:"tokens,omitempty"`
	Cost          float64                  `json:"cost,omitempty"`
	CostBreakdown *costguard.CostBreakdown `json:"costBreakdown,omitempty"`
	Message       string                   `json:"message,omitempty"`
	Tree          []FileNode               `json:"tree,omitempty"`
	Sessions      []SessionInfo            `json:"sessions,omitempty"`
	Config        any                      `json:"config,omitempty"`
	Text          string                   `json:"text,omitempty"`
	Messages      []MsgItem                `json:"messages,omitempty"`
	Model         string                   `json:"model,omitempty"`
}

type MsgItem struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type FileNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Size     int64      `json:"size,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

type SessionInfo struct {
	ID           string `json:"id"`
	UserID       string `json:"userId,omitempty"`
	Title        string `json:"title"`
	Model        string `json:"model"`
	MessageCount int    `json:"messageCount"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

func (h *Handler) HandleMessage(client *Client, raw []byte) {
	var msg WSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		h.sendError(client, "Invalid message format")
		return
	}

	switch msg.Type {
	case "chat":
		h.handleChat(client, msg)
	case "cmd":
		h.handleCommand(client, msg)
	case "model":
		h.handleModelSwitch(client, msg)
	case "file_open":
		h.handleFileOpen(client, msg)
	case "file_save":
		h.handleFileSave(client, msg)
	case "file_tree":
		h.handleFileTree(client, msg)
	case "session_list":
		h.handleSessionList(client)
	case "session_new":
		h.handleSessionNew(client, msg)
	case "session_load":
		h.handleSessionLoad(client, msg)
	case "session_rename":
		h.handleSessionRename(client, msg)
	case "session_delete":
		h.handleSessionDelete(client, msg)
	case "abort":
		h.handleAbort(client)
	case "tool_approval":
		h.handleToolApproval(client, msg)
	default:
		h.sendError(client, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

func (h *Handler) clientForRequest(clientID string) *api.Client {
	model := h.apiClient.Model
	if m, ok := h.clientModels[clientID]; ok {
		model = m
	}
	if model == h.apiClient.Model {
		return h.apiClient
	}
	c := api.NewClientWithModel(h.apiClient.APIKey, h.apiClient.BaseURL, model)
	c.IsOpenAI = h.apiClient.IsOpenAI
	return c
}

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

	var systemPrompt string
	if h.memMgr != nil {
		userMem := h.memMgr.ForUser(client.UserID)
		systemPrompt = userMem.BuildPrompt()
	} else {
		systemPrompt = h.prompt.Build()
	}

	var fullContent string
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
						fullContent += payload.Delta.Text
						openaiOutputChars += len(payload.Delta.Text)
						h.sendToClient(client, WSResponse{Type: "token", Content: payload.Delta.Text})
					} else if payload.Delta.Type == "thinking_delta" && payload.Delta.Thinking != "" {
						h.sendToClient(client, WSResponse{Type: "thinking", Content: payload.Delta.Thinking})
					}
				}
			case "message_stop":
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
	var allTextContent string
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
		var iterText string

		err := reqClient.StreamMessageSSE(ctx, req, func(event string, data []byte) error {
			result, err := parser.HandleEvent(event, data)
			if err != nil {
				return err
			}

			if result.Error != nil {
				return result.Error
			}

			if result.TextDelta != "" {
				iterText += result.TextDelta
				h.sendToClient(client, WSResponse{Type: "token", Content: result.TextDelta})
			}

			if result.ThinkingDelta != "" && h.showThinking {
				h.sendToClient(client, WSResponse{Type: "thinking", Content: result.ThinkingDelta})
			}

			return nil
		})

		if err != nil {
			h.sendError(client, fmt.Sprintf("API error: %v", err))
			return
		}

		resp := parser.GetMessage()
		totalInputTokens += resp.Usage.InputTokens
		totalOutputTokens += resp.Usage.OutputTokens
		allTextContent += iterText

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
			h.sendToClient(client, WSResponse{
				Type:  "tool_start",
				ID:    block.ID,
				Tool:  block.Name,
				Input: block.Input,
			})

			if h.needsApproval(client.ID, block.Name, block.Input) {
				approved, approvalErr := h.requestApproval(client, block.ID, block.Name, block.Input)
				if approvalErr != nil || !approved {
					reason := "Tool execution denied by user"
					if approvalErr != nil {
						reason = approvalErr.Error()
					}
					go observability.AuditDenial(block.Name, block.ID, client.ID, reason)
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
					continue
				}
			}

			startTime := time.Now()
			output, toolErr := h.executeTool(ctx, block.Name, block.Input)
			duration := time.Since(startTime).Milliseconds()

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
					Type:   "tool_output",
					ID:     block.ID,
					Output: output,
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
		go func() {
			if err := h.dataStore.UpsertSession(storeSess); err != nil {
				slog.Warn("failed to upsert session", "error", err, "session_id", storeSess.ID)
			}
		}()
	}
	if h.sessMgr != nil {
		go h.sessMgr.Save(sess)
	}
}

func (h *Handler) syncMessageToStore(sess *session.Session, role, content string, tokens int) {
	if h.dataStore == nil {
		return
	}
	go func() {
		if err := h.dataStore.InsertSessionMessage(sess.ID, role, content, tokens); err != nil {
			slog.Warn("failed to insert session message", "error", err, "session_id", sess.ID)
		}
	}()
}

func (h *Handler) executeTool(ctx context.Context, name string, input map[string]any) (string, error) {
	prepared := make(map[string]any, len(input))
	for k, v := range input {
		prepared[k] = v
	}

	// Resolve relative path inputs against workDir so registry tools (which use filepath.Abs) resolve correctly.
	for _, key := range []string{"path", "file_path", "filepath", "filename", "directory", "dir"} {
		if v, ok := prepared[key].(string); ok && v != "" && !filepath.IsAbs(v) {
			prepared[key] = filepath.Join(h.workDir, v)
		}
	}

	// BashTool reads "workdir" from input; inject if not already set.
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
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}

func (h *Handler) needsApproval(clientID, toolName string, input map[string]any) bool {
	if h.isAutoApproved(clientID, toolName) {
		return false
	}
	needs, _ := h.approvalGate.NeedsApproval(toolName, input)
	return needs
}

func (h *Handler) isAutoApproved(clientID, toolName string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if tools, ok := h.autoApproved[clientID]; ok {
		return tools[toolName]
	}
	return false
}

func (h *Handler) requestApproval(client *Client, blockID, toolName string, input map[string]any) (bool, error) {
	h.sendToClient(client, WSResponse{
		Type:  "tool_approval_request",
		ID:    blockID,
		Tool:  toolName,
		Input: input,
	})

	ch := make(chan bool, 1)
	h.mu.Lock()
	h.pendingApprovals[blockID] = ch
	h.mu.Unlock()

	select {
	case approved := <-ch:
		return approved, nil
	case <-time.After(5 * time.Minute):
		h.mu.Lock()
		delete(h.pendingApprovals, blockID)
		h.mu.Unlock()
		return false, fmt.Errorf("tool approval timed out after 5 minutes")
	}
}

func (h *Handler) handleCommand(client *Client, msg WSMessage) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdErr := commands.Execute(msg.Name, msg.Args)

	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	r.Close()
	outputStr := strings.TrimSpace(string(output))

	if outputStr != "" {
		h.sendToClient(client, WSResponse{Type: "cmd_result", Content: outputStr})
	}

	if cmdErr != nil && outputStr == "" {
		h.sendToClient(client, WSResponse{Type: "error", Message: cmdErr.Error()})
	}

	h.sendToClient(client, WSResponse{Type: "done"})
}

func (h *Handler) handleToolApproval(client *Client, msg WSMessage) {
	h.mu.Lock()
	ch, ok := h.pendingApprovals[msg.ID]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.pendingApprovals, msg.ID)

	switch msg.Content {
	case "approve":
		go observability.AuditApproval(msg.Name, msg.ID, true, client.ID)
		ch <- true
	case "always_approve":
		if h.autoApproved[client.ID] == nil {
			h.autoApproved[client.ID] = make(map[string]bool)
		}
		h.autoApproved[client.ID][msg.Name] = true
		go observability.AuditApproval(msg.Name, msg.ID, true, client.ID)
		ch <- true
	default:
		go observability.AuditDenial(msg.Name, msg.ID, client.ID, "user denied")
		ch <- false
	}
	h.mu.Unlock()
}

func (h *Handler) handleAbort(client *Client) {
	h.mu.Lock()
	cancel, ok := h.cancelFuncs[client.ID]
	if ok {
		cancel()
		delete(h.cancelFuncs, client.ID)
	}
	h.mu.Unlock()
	h.sendToClient(client, WSResponse{Type: "aborted", Message: "Request aborted"})
}

func (h *Handler) handleModelSwitch(client *Client, msg WSMessage) {
	if msg.Model != "" {
		h.mu.Lock()
		h.clientModels[client.ID] = msg.Model
		h.mu.Unlock()
	}

	model := h.apiClient.Model
	if m, ok := h.clientModels[client.ID]; ok {
		model = m
	}

	h.sendToClient(client, WSResponse{
		Type:    "model_changed",
		Message: fmt.Sprintf("Model switched to %s", model),
	})
}

func (h *Handler) handleFileOpen(client *Client, msg WSMessage) {
	path := filepath.Join(h.workDir, msg.Path)
	path = filepath.Clean(path)

	if !strings.HasPrefix(path, filepath.Clean(h.workDir)) {
		h.sendError(client, "Access denied: path outside work directory")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type:    "file_content",
		Content: string(data),
	})
}

func (h *Handler) handleFileSave(client *Client, msg WSMessage) {
	path := filepath.Join(h.workDir, msg.Path)
	path = filepath.Clean(path)

	if !strings.HasPrefix(path, filepath.Clean(h.workDir)) {
		h.sendError(client, "Access denied: path outside work directory")
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to create directory: %v", err))
		return
	}

	if err := os.WriteFile(path, []byte(msg.Content), 0644); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to write file: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type:    "done",
		Message: fmt.Sprintf("File saved: %s", msg.Path),
	})
}

func (h *Handler) handleFileTree(client *Client, msg WSMessage) {
	root := h.workDir
	if msg.Path != "" {
		root = filepath.Join(h.workDir, msg.Path)
	}

	tree, err := h.buildFileTree(root, 3)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to scan directory: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type: "file_tree",
		Tree: tree,
	})
}

func (h *Handler) buildFileTree(root string, maxDepth int) ([]FileNode, error) {
	if maxDepth <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var nodes []FileNode
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != ".smartclaw" {
			continue
		}

		node := FileNode{Name: name}

		if entry.IsDir() {
			skipDirs := map[string]bool{
				"node_modules": true, "vendor": true, ".git": true,
				"dist": true, "build": true, "bin": true, "__pycache__": true,
			}
			if skipDirs[name] {
				node.Type = "dir"
				continue
			}

			node.Type = "dir"
			children, err := h.buildFileTree(filepath.Join(root, name), maxDepth-1)
			if err == nil {
				node.Children = children
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				node.Size = info.Size()
			}
			node.Type = "file"
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (h *Handler) handleSessionList(client *Client) {
	if h.dataStore != nil {
		var sessions []*store.Session
		var err error
		if client.UserID != "" && client.UserID != "default" {
			sessions, err = h.dataStore.ListSessions(client.UserID, 50)
		} else {
			sessions, err = h.dataStore.ListAllSessions(50)
		}
		if err != nil {
			h.sendError(client, fmt.Sprintf("Failed to list sessions: %v", err))
			return
		}

		counts, _ := h.dataStore.GetMessageCountsBatch()
		var infos []SessionInfo
		for _, s := range sessions {
			infos = append(infos, SessionInfo{
				ID:           s.ID,
				UserID:       s.UserID,
				Title:        s.Title,
				Model:        s.Model,
				MessageCount: int(counts[s.ID]),
				CreatedAt:    s.CreatedAt.Format(time.RFC3339),
				UpdatedAt:    s.UpdatedAt.Format(time.RFC3339),
			})
		}
		h.sendToClient(client, WSResponse{Type: "session_list", Sessions: infos})
		return
	}

	if h.sessMgr == nil {
		h.sendToClient(client, WSResponse{Type: "session_list", Sessions: []SessionInfo{}})
		return
	}

	var sessions []session.SessionInfo
	var err error
	if client.UserID != "" && client.UserID != "default" {
		sessions, err = h.sessMgr.ListByUser(client.UserID)
	} else {
		sessions, err = h.sessMgr.List()
	}
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	var infos []SessionInfo
	for _, s := range sessions {
		infos = append(infos, SessionInfo{
			ID:           s.ID,
			UserID:       s.UserID,
			Title:        s.Title,
			Model:        s.Model,
			MessageCount: s.MessageCount,
			CreatedAt:    s.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    s.UpdatedAt.Format(time.RFC3339),
		})
	}

	h.sendToClient(client, WSResponse{Type: "session_list", Sessions: infos})
}

func (h *Handler) handleSessionNew(client *Client, msg WSMessage) {
	model := msg.Model
	if model == "" {
		model = h.apiClient.Model
		if m, ok := h.clientModels[client.ID]; ok {
			model = m
		}
	}

	if h.dataStore != nil {
		sess := h.sessMgr.NewSession(model, client.UserID)
		storeSess := &store.Session{
			ID:        sess.ID,
			UserID:    client.UserID,
			Source:    "web",
			Model:     model,
			Title:     sess.Title,
			Tokens:    0,
			Cost:      0,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
		}
		if err := h.dataStore.UpsertSession(storeSess); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to save session: %v", err))
			return
		}
		h.clientSessMu.Lock()
		h.clientSess[client.ID] = sess
		h.clientSessMu.Unlock()
		h.sendToClient(client, WSResponse{
			Type:    "session_created",
			ID:      sess.ID,
			Message: "New session created",
		})
		return
	}

	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	sess := h.sessMgr.NewSession(model, client.UserID)
	if err := h.sessMgr.Save(sess); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to save session: %v", err))
		return
	}

	h.clientSessMu.Lock()
	h.clientSess[client.ID] = sess
	h.clientSessMu.Unlock()

	h.sendToClient(client, WSResponse{
		Type:    "session_created",
		ID:      sess.ID,
		Message: "New session created",
	})
}

func (h *Handler) handleSessionLoad(client *Client, msg WSMessage) {
	if h.dataStore != nil {
		storeSess, err := h.dataStore.GetSession(msg.ID)
		if err != nil {
			h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
			return
		}
		if storeSess == nil {
			h.sendError(client, "Session not found")
			return
		}
		if storeSess.UserID != "" && storeSess.UserID != client.UserID && client.UserID != "default" {
			h.sendError(client, "Access denied: session belongs to another user")
			return
		}

		storeMsgs, _ := h.dataStore.GetSessionMessages(msg.ID)

		sess := &session.Session{
			ID:        storeSess.ID,
			UserID:    storeSess.UserID,
			CreatedAt: storeSess.CreatedAt,
			UpdatedAt: storeSess.UpdatedAt,
			Model:     storeSess.Model,
			Tokens:    storeSess.Tokens,
			Cost:      storeSess.Cost,
			Title:     storeSess.Title,
			Messages:  make([]session.Message, 0, len(storeMsgs)),
		}
		for _, m := range storeMsgs {
			sess.Messages = append(sess.Messages, session.Message{
				Role:      m.Role,
				Content:   m.Content,
				Timestamp: m.Timestamp,
				Tokens:    m.Tokens,
			})
		}

		h.clientSessMu.Lock()
		h.clientSess[client.ID] = sess
		h.clientSessMu.Unlock()

		var msgs []MsgItem
		for _, m := range sess.Messages {
			msgs = append(msgs, MsgItem{
				Role:      m.Role,
				Content:   m.Content,
				Timestamp: m.Timestamp.Format(time.RFC3339),
			})
		}

		h.sendToClient(client, WSResponse{
			Type:     "session_loaded",
			ID:       sess.ID,
			Title:    sess.Title,
			Messages: msgs,
		})
		return
	}

	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	sess, err := h.sessMgr.Load(msg.ID)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
		return
	}

	if sess.UserID != "" && sess.UserID != client.UserID && client.UserID != "default" {
		h.sendError(client, "Access denied: session belongs to another user")
		return
	}

	h.clientSessMu.Lock()
	h.clientSess[client.ID] = sess
	h.clientSessMu.Unlock()

	var msgs []MsgItem
	for _, m := range sess.Messages {
		msgs = append(msgs, MsgItem{
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(time.RFC3339),
		})
	}

	h.sendToClient(client, WSResponse{
		Type:     "session_loaded",
		ID:       sess.ID,
		Title:    sess.Title,
		Messages: msgs,
	})
}

func (h *Handler) handleSessionDelete(client *Client, msg WSMessage) {
	if h.dataStore != nil {
		storeSess, err := h.dataStore.GetSession(msg.ID)
		if err != nil {
			h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
			return
		}
		if storeSess == nil {
			h.sendError(client, "Session not found")
			return
		}
		if storeSess.UserID != "" && storeSess.UserID != client.UserID && client.UserID != "default" {
			h.sendError(client, "Access denied: session belongs to another user")
			return
		}
		if err := h.dataStore.DeleteSession(msg.ID); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to delete session: %v", err))
			return
		}
		h.sendToClient(client, WSResponse{
			Type:    "session_deleted",
			ID:      msg.ID,
			Message: "Session deleted",
		})
		return
	}

	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	sess, err := h.sessMgr.Load(msg.ID)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
		return
	}
	if sess.UserID != "" && sess.UserID != client.UserID && client.UserID != "default" {
		h.sendError(client, "Access denied: session belongs to another user")
		return
	}

	if err := h.sessMgr.Delete(msg.ID); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to delete session: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type:    "session_deleted",
		ID:      msg.ID,
		Message: "Session deleted",
	})
}

func (h *Handler) handleSessionRename(client *Client, msg WSMessage) {
	if msg.ID == "" {
		h.sendError(client, "Session ID is required")
		return
	}

	if h.dataStore != nil {
		if err := h.dataStore.UpdateSessionTitle(msg.ID, msg.Title); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to rename session: %v", err))
			return
		}
	} else if h.sessMgr != nil {
		if err := h.sessMgr.Rename(msg.ID, msg.Title); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to rename session: %v", err))
			return
		}
	}

	h.clientSessMu.RLock()
	sess, ok := h.clientSess[client.ID]
	h.clientSessMu.RUnlock()
	if ok && sess.ID == msg.ID {
		sess.Title = msg.Title
	}

	h.handleSessionList(client)
}

func (h *Handler) sendToClient(client *Client, resp WSResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	select {
	case client.send <- data:
	default:
	}
}

func (h *Handler) sendError(client *Client, message string) {
	h.sendToClient(client, WSResponse{Type: "error", Message: message})
}

type StatsResponse struct {
	TokensUsed   int     `json:"tokensUsed"`
	TokensLimit  int     `json:"tokensLimit"`
	Cost         float64 `json:"cost"`
	Model        string  `json:"model"`
	SessionCount int     `json:"sessionCount"`
}

func (h *Handler) GetStats() StatsResponse {
	model := "sre-model"
	if h.apiClient != nil {
		model = h.apiClient.Model
	}
	return StatsResponse{
		TokensLimit: 200000,
		Model:       model,
	}
}

func (h *Handler) StartSessionCleanup(ttl time.Duration) {
	if h.sessMgr != nil {
		h.sessMgr.StartCleanup(ttl)
	}
}
