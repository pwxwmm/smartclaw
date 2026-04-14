package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/provider"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/session"
)

type Handler struct {
	hub          *Hub
	workDir      string
	apiClient    *api.Client
	router       *provider.Router
	sessMgr      *session.Manager
	showThinking bool
	clientSess   map[string]*session.Session
	prompt       *runtime.PromptBuilder
}

func NewHandler(hub *Hub, workDir string, apiClient *api.Client) *Handler {
	mgr, err := session.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: session manager init failed: %v\n", err)
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

	return &Handler{
		hub:          hub,
		workDir:      workDir,
		apiClient:    apiClient,
		router:       router,
		sessMgr:      mgr,
		showThinking: showThinking,
		clientSess:   make(map[string]*session.Session),
		prompt:       runtime.NewPromptBuilder(),
	}
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
	Model   string          `json:"model,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type WSResponse struct {
	Type     string        `json:"type"`
	Content  string        `json:"content,omitempty"`
	Tool     string        `json:"tool,omitempty"`
	Input    any           `json:"input,omitempty"`
	Output   string        `json:"output,omitempty"`
	Duration int64         `json:"duration,omitempty"`
	ID       string        `json:"id,omitempty"`
	Title    string        `json:"title,omitempty"`
	Status   string        `json:"status,omitempty"`
	Progress float64       `json:"progress,omitempty"`
	Tokens   int           `json:"tokens,omitempty"`
	Cost     float64       `json:"cost,omitempty"`
	Message  string        `json:"message,omitempty"`
	Tree     []FileNode    `json:"tree,omitempty"`
	Sessions []SessionInfo `json:"sessions,omitempty"`
	Config   any           `json:"config,omitempty"`
	Text     string        `json:"text,omitempty"`
	Messages []MsgItem     `json:"messages,omitempty"`
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
	case "session_delete":
		h.handleSessionDelete(client, msg)
	case "abort":
		h.sendToClient(client, WSResponse{Type: "aborted", Message: "Request aborted"})
	default:
		h.sendError(client, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

func (h *Handler) handleChat(client *Client, msg WSMessage) {
	if h.apiClient == nil {
		h.sendError(client, "API client not configured. Check ~/.smartclaw/config.json")
		return
	}

	if msg.Model != "" {
		h.apiClient.SetModel(msg.Model)
	}

	sess := h.clientSess[client.ID]
	if sess == nil {
		if h.sessMgr != nil {
			sess = h.sessMgr.NewSession(h.apiClient.Model)
		} else {
			sess = &session.Session{ID: "temp", Model: h.apiClient.Model, Messages: []session.Message{}}
		}
		h.clientSess[client.ID] = sess
	}

	sess.AddMessage("user", msg.Content)
	h.autoSaveSession(sess)

	h.sendToClient(client, WSResponse{Type: "session_active", ID: sess.ID, Title: sess.Title})

	messages := make([]api.Message, 0, len(sess.Messages))
	for _, m := range sess.Messages {
		messages = append(messages, api.Message{Role: m.Role, Content: m.Content})
	}

	systemPrompt := h.prompt.Build()

	var fullContent string

	if h.apiClient.IsOpenAI {
		req := &api.MessageRequest{
			Model:     h.apiClient.Model,
			MaxTokens: 8192,
			Messages:  messages,
			System:    systemPrompt,
		}

		err := h.apiClient.StreamMessageOpenAI(context.Background(), req, func(event string, data []byte) error {
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
						h.sendToClient(client, WSResponse{Type: "token", Content: payload.Delta.Text})
					} else if payload.Delta.Type == "thinking_delta" && payload.Delta.Thinking != "" {
					}
				}
			case "message_stop":
				sess.AddMessage("assistant", fullContent)
				h.autoSaveSession(sess)
				h.sendToClient(client, WSResponse{Type: "done", Tokens: 0, Cost: 0})
			}
			return nil
		})
		if err != nil {
			h.sendError(client, fmt.Sprintf("API error: %v", err))
		}
		return
	}

	resp, err := h.apiClient.CreateMessage(messages, systemPrompt)
	if err != nil {
		h.sendError(client, fmt.Sprintf("API error: %v", err))
		return
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	h.sendToClient(client, WSResponse{
		Type:    "token",
		Content: content,
	})

	tokens := resp.Usage.InputTokens + resp.Usage.OutputTokens

	sess.AddMessage("assistant", content)
	h.autoSaveSession(sess)

	h.sendToClient(client, WSResponse{
		Type:   "done",
		Tokens: tokens,
		Cost:   float64(tokens) * 0.000003,
	})
}

func (h *Handler) autoSaveSession(sess *session.Session) {
	if h.sessMgr == nil {
		return
	}
	go h.sessMgr.Save(sess)
}

func (h *Handler) handleCommand(client *Client, msg WSMessage) {
	result := fmt.Sprintf("Command: %s %v", msg.Name, msg.Args)
	h.sendToClient(client, WSResponse{Type: "done", Content: result})
}

func (h *Handler) handleModelSwitch(client *Client, msg WSMessage) {
	if msg.Model != "" {
		if h.router != nil {
			if pClient, err := h.router.ClientForModel(msg.Model); err == nil {
				h.apiClient = pClient
			} else {
				h.apiClient.SetModel(msg.Model)
			}
		} else {
			h.apiClient.SetModel(msg.Model)
		}
	}
	h.sendToClient(client, WSResponse{
		Type:    "model_changed",
		Message: fmt.Sprintf("Model switched to %s", h.apiClient.Model),
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
	if h.sessMgr == nil {
		h.sendToClient(client, WSResponse{Type: "session_list", Sessions: []SessionInfo{}})
		return
	}

	sessions, err := h.sessMgr.List()
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	var infos []SessionInfo
	for _, s := range sessions {
		infos = append(infos, SessionInfo{
			ID:           s.ID,
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
	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	model := msg.Model
	if model == "" {
		model = h.apiClient.Model
	}

	sess := h.sessMgr.NewSession(model)
	if err := h.sessMgr.Save(sess); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to save session: %v", err))
		return
	}

	h.clientSess[client.ID] = sess

	h.sendToClient(client, WSResponse{
		Type:    "session_created",
		ID:      sess.ID,
		Message: "New session created",
	})
}

func (h *Handler) handleSessionLoad(client *Client, msg WSMessage) {
	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	sess, err := h.sessMgr.Load(msg.ID)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
		return
	}

	h.clientSess[client.ID] = sess

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
	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
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
