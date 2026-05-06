package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/gateway"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/permissions"
	"github.com/instructkr/smartclaw/internal/pool"
	"github.com/instructkr/smartclaw/internal/provider"
	"github.com/instructkr/smartclaw/internal/routing"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/wiki"
)

type ApprovalMeta struct {
	ToolName    string         `json:"toolName"`
	Input       map[string]any `json:"input"`
	RequestedAt time.Time      `json:"requestedAt"`
}

type ApprovalRecord struct {
	BlockID   string    `json:"blockId"`
	ToolName  string    `json:"toolName"`
	Decision  string    `json:"decision"`
	Timestamp time.Time `json:"timestamp"`
}

	type Handler struct {
	hub           *Hub
	workDir       string
	apiClient     *api.Client
	router        *provider.Router
	modelRouter   *routing.ModelRouter
	sessMgr       *session.Manager
	dataStore     *store.Store
	memMgr        *memory.MemoryManager
	wikiClient    *wiki.WikiClient
	mcpRegistry   *mcp.MCPServerRegistry
	cronTrigger   *gateway.CronTrigger
	gw            *gateway.Gateway
	skillTracker  *learning.SkillTracker
	workflowSvc   *WorkflowServiceHelper
	showThinking  bool
	clientSess    map[string]*session.Session
	clientSessMu  sync.RWMutex
	prompt        *runtime.PromptBuilder
	unifiedPerm   *permissions.UnifiedPermissionEngine
	costGuard     *costguard.CostGuard
	clientModels  map[string]string
	clientCache   map[string]*api.Client
	clientCacheMu sync.RWMutex

	mu                 sync.Mutex
	pendingApprovals   map[string]chan bool
	pendingApprovalMeta map[string]*ApprovalMeta
	approvalHistory    []ApprovalRecord
	autoApproved       map[string]map[string]bool
	cancelFuncs        map[string]context.CancelFunc
	wg                 sync.WaitGroup
	shutdownCtx        context.Context
	shutdownCancel     context.CancelFunc
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

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	h := &Handler{
		hub:                hub,
		workDir:            workDir,
		apiClient:          apiClient,
		router:             router,
		modelRouter:        routing.NewModelRouter(routing.DefaultRoutingConfig()),
		sessMgr:            mgr,
		dataStore:          dataStore,
		showThinking:       showThinking,
		clientSess:         make(map[string]*session.Session),
		prompt:             runtime.NewPromptBuilder(),
		unifiedPerm:         permissions.NewUnifiedPermissionEngine(permissions.NewApprovalGate(), nil),
		costGuard:          costguard.NewCostGuard(costguard.DefaultBudgetConfig()),
		clientModels:       make(map[string]string),
		clientCache:        make(map[string]*api.Client),
		pendingApprovals:   make(map[string]chan bool),
		pendingApprovalMeta: make(map[string]*ApprovalMeta),
		approvalHistory:    make([]ApprovalRecord, 0),
		autoApproved:       make(map[string]map[string]bool),
		cancelFuncs:        make(map[string]context.CancelFunc),
		shutdownCtx:        shutdownCtx,
		shutdownCancel:     shutdownCancel,
	}

	_ = h.unifiedPerm.LoadApprovalConfigFromDefaultPath()

	h.wikiClient = newWikiClientFromConfig()

	h.cronTrigger = gateway.NewCronTrigger(dataStore, nil)

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

	baseURL = sanitizeBaseURL(baseURL)

	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if model == "" {
		model = "sre-model"
	}
	return
}

func sanitizeBaseURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u := strings.TrimRight(rawURL, "#?/&")
	u = strings.TrimSuffix(u, "/chat/completions")
	u = strings.TrimSuffix(u, "/v1/messages")
	return u
}

func isOpenAI() bool {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	var cfg struct {
		OpenAI  bool   `json:"openai"`
		BaseURL string `json:"base_url"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		if cfg.OpenAI {
			return true
		}
		cleaned := sanitizeBaseURL(cfg.BaseURL)
		if strings.Contains(cfg.BaseURL, "/chat/completions") || strings.Contains(cleaned, "/v1") && !strings.Contains(cleaned, "anthropic.com") {
			return true
		}
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

func newWikiClientFromConfig() *wiki.WikiClient {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return wiki.NewWikiClient(wiki.WikiConfig{})
	}
	var cfg struct {
		Wiki struct {
			BaseURL   string `json:"base_url"`
			APIToken  string `json:"api_token"`
			SpaceName string `json:"space_name"`
			Enabled   bool   `json:"enabled"`
		} `json:"wiki"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		return wiki.NewWikiClient(wiki.WikiConfig{
			BaseURL:   cfg.Wiki.BaseURL,
			APIToken:  cfg.Wiki.APIToken,
			SpaceName: cfg.Wiki.SpaceName,
			Enabled:   cfg.Wiki.Enabled,
		})
	}
	return wiki.NewWikiClient(wiki.WikiConfig{})
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
	Images  json.RawMessage `json:"images,omitempty"`
}

type DiffAnnotation struct {
	HunkIndex int    `json:"hunk_index"`
	Reason    string `json:"reason"`
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
	Data          any                      `json:"data,omitempty"`
	Annotations   []DiffAnnotation         `json:"annotations,omitempty"`
	Path          string                   `json:"path,omitempty"`
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

	if err := h.validateWSMessage(msg); err != nil {
		h.sendToClient(client, WSResponse{Type: "error", Content: err.Error()})
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
	case "git_status":
		h.handleGitStatus(client, msg)
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
	case "approval_list":
		h.handleApprovalListWS(client)
	case "approval_history":
		h.handleApprovalHistoryWS(client)
	case "approval_bulk":
		h.handleApprovalBulkWS(client, msg)
	case "skill_list":
		h.handleSkillListWS(client)
	case "skill_detail":
		h.handleSkillDetailWS(client, msg)
	case "skill_toggle":
		h.handleSkillToggleWS(client, msg)
	case "skill_search":
		h.handleSkillSearchWS(client, msg)
	case "skill_create":
		h.handleSkillCreateWS(client, msg)
	case "memory_layers":
		h.handleMemoryLayersWS(client)
	case "memory_search":
		h.handleMemorySearchWS(client, msg)
	case "memory_recall":
		h.handleMemoryRecallWS(client, msg)
	case "memory_store":
		h.handleMemoryStoreWS(client, msg)
	case "memory_update":
		h.handleMemoryUpdateWS(client, msg)
	case "memory_stats":
		h.handleMemoryStatsWS(client)
	case "memory_observations":
		h.handleMemoryObservationsWS(client)
	case "memory_observation_delete":
		h.handleMemoryObservationDeleteWS(client, msg)
	case "skill_edit":
		h.handleSkillEditWS(client, msg)
	case "skill_health":
		h.handleSkillHealthWS(client)
	case "skill_improve":
		h.handleSkillImproveWS(client, msg)
	case "session_fragments":
		h.handleSessionFragmentsWS(client, msg)
	case "wiki_search":
		h.handleWikiSearchWS(client, msg)
	case "wiki_pages":
		h.handleWikiPagesWS(client)
	case "wiki_page_content":
		h.handleWikiPageContentWS(client, msg)
	case "mcp_list":
		h.handleMCPListWS(client)
	case "mcp_add":
		h.handleMCPAddWS(client, msg)
	case "mcp_remove":
		h.handleMCPRemoveWS(client, msg)
	case "mcp_start":
		h.handleMCPStartWS(client, msg)
	case "mcp_stop":
		h.handleMCPStopWS(client, msg)
	case "mcp_catalog":
		h.handleMCPCatalogWS(client)
	case "mcp_tools":
		h.handleMCPToolsWS(client, msg)
	case "mcp_resources":
		h.handleMCPResourcesWS(client, msg)
	case "agent_list":
		h.handleAgentListWS(client)
	case "agent_stop":
		h.handleAgentStopWS(client, msg)
	case "agent_output":
		h.handleAgentOutputWS(client, msg)
	case "agent_switch":
		h.handleAgentSwitchWS(client, msg)
	case "template_list":
		h.handleTemplateListWS(client)
	case "template_create":
		h.handleTemplateCreateWS(client, msg)
	case "template_update":
		h.handleTemplateUpdateWS(client, msg)
	case "template_delete":
		h.handleTemplateDeleteWS(client, msg)
	case "cron_list":
		h.handleCronListWS(client)
	case "cron_create":
		h.handleCronCreateWS(client, msg)
	case "cron_delete":
		h.handleCronDeleteWS(client, msg)
	case "cron_toggle":
		h.handleCronToggleWS(client, msg)
	case "cron_run":
		h.handleCronRunWS(client, msg)
	case "chat_search":
		h.handleChatSearchWS(client, msg)
	case "arena_chat":
		h.handleArenaChat(client, msg)
	case "arena_vote":
		h.handleArenaVote(client, msg)
	case "watchdog_status":
		h.handleWatchdogStatusWS(client)
	case "change_project":
		h.handleChangeProject(client, msg)
	case "get_recent_projects":
		h.handleGetRecentProjects(client)
	case "browse_dirs":
		h.handleBrowseDirs(client, msg)
	case "warroom_start":
		h.handleWarRoomStartWS(client, msg)
	case "warroom_status":
		h.handleWarRoomStatusWS(client, msg)
	case "warroom_stop":
		h.handleWarRoomStopWS(client, msg)
	case "warroom_list":
		h.handleWarRoomListWS(client)
	case "warroom_assign_task":
		h.handleWarRoomAssignTaskWS(client, msg)
	case "warroom_broadcast":
		h.handleWarRoomBroadcastWS(client, msg)
	default:
		h.sendError(client, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

var validWSTypes = map[string]bool{
	"chat": true, "cmd": true, "model": true,
	"file_open": true, "file_save": true, "file_tree": true, "git_status": true,
	"session_list": true, "session_new": true, "session_load": true,
	"session_rename": true, "session_delete": true,
	"abort": true, "tool_approval": true,
	"approval_list": true, "approval_history": true, "approval_bulk": true,
	"skill_list": true, "skill_detail": true, "skill_toggle": true,
	"skill_search": true, "skill_create": true, "skill_edit": true,
	"skill_health": true, "skill_improve": true,
	"memory_layers": true, "memory_search": true, "memory_recall": true,
	"memory_store": true, "memory_update": true, "memory_stats": true,
	"memory_observations": true, "memory_observation_delete": true,
	"session_fragments": true,
	"wiki_search": true, "wiki_pages": true, "wiki_page_content": true,
	"mcp_list": true, "mcp_add": true, "mcp_remove": true,
	"mcp_start": true, "mcp_stop": true, "mcp_catalog": true,
	"mcp_tools": true, "mcp_resources": true,
	"agent_list": true, "agent_stop": true, "agent_output": true, "agent_switch": true,
	"template_list": true, "template_create": true, "template_update": true, "template_delete": true,
	"cron_list": true, "cron_create": true, "cron_delete": true, "cron_toggle": true, "cron_run": true,
	"chat_search": true,
	"arena_chat": true, "arena_vote": true,
	"watchdog_status": true,
	"change_project": true, "get_recent_projects": true, "browse_dirs": true,
	"warroom_start": true, "warroom_status": true, "warroom_stop": true,
	"warroom_list": true, "warroom_assign_task": true, "warroom_broadcast": true,
}

var typesRequiringData = map[string]bool{
	"chat": true, "cmd": true, "model": true,
	"file_open": true, "file_save": true, "file_tree": true,
	"session_new": true, "session_load": true,
	"session_rename": true, "session_delete": true,
	"tool_approval": true, "approval_bulk": true,
	"skill_detail": true, "skill_toggle": true,
	"skill_search": true, "skill_create": true, "skill_edit": true,
	"skill_improve": true,
	"memory_search": true, "memory_recall": true,
	"memory_store": true, "memory_update": true,
	"memory_observation_delete": true, "session_fragments": true,
	"wiki_search": true, "wiki_page_content": true,
	"mcp_add": true, "mcp_remove": true,
	"mcp_start": true, "mcp_stop": true,
	"mcp_tools": true, "mcp_resources": true,
	"agent_stop": true, "agent_output": true, "agent_switch": true,
	"template_create": true, "template_update": true, "template_delete": true,
	"cron_create": true, "cron_delete": true, "cron_toggle": true, "cron_run": true,
	"arena_chat": true, "arena_vote": true,
	"change_project": true,
	"warroom_start": true, "warroom_status": true, "warroom_stop": true,
	"warroom_assign_task": true, "warroom_broadcast": true,
}

func (h *Handler) validateWSMessage(msg WSMessage) error {
	if msg.Type == "" {
		return fmt.Errorf("message type is required")
	}

	if !validWSTypes[msg.Type] {
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}

	if typesRequiringData[msg.Type] {
		if len(msg.Data) == 0 && msg.Content == "" && msg.Path == "" && msg.Name == "" && msg.ID == "" && msg.Model == "" && msg.Title == "" && len(msg.Args) == 0 {
			return fmt.Errorf("message type %s requires data payload", msg.Type)
		}
	}

	return nil
}

func (h *Handler) clientForRequest(clientID string) *api.Client {
	model := h.apiClient.Model
	if m, ok := h.clientModels[clientID]; ok {
		model = m
	}
	if model == h.apiClient.Model {
		return h.apiClient
	}

	h.clientCacheMu.RLock()
	if c, ok := h.clientCache[model]; ok {
		h.clientCacheMu.RUnlock()
		return c
	}
	h.clientCacheMu.RUnlock()

	h.clientCacheMu.Lock()
	defer h.clientCacheMu.Unlock()

	if c, ok := h.clientCache[model]; ok {
		return c
	}

	c := api.NewClientWithModel(h.apiClient.APIKey, h.apiClient.BaseURL, model)
	c.IsOpenAI = h.apiClient.IsOpenAI
	h.clientCache[model] = c
	return c
}

func (h *Handler) sendToClient(client *Client, resp WSResponse) {
	pe := pool.GetJSONEncoder(nil)
	if err := pe.Encode(resp); err != nil {
		pool.PutJSONEncoder(pe)
		return
	}
	data := make([]byte, len(pe.Bytes()))
	copy(data, pe.Bytes())
	pool.PutJSONEncoder(pe)

	if resp.Type == "token" || resp.Type == "thinking" {
		select {
		case client.sendImmediate <- data:
		case <-time.After(5 * time.Second):
			slog.Warn("websocket immediate send timeout", "client_id", client.ID)
		}
		return
	}

	select {
	case client.send <- data:
	case <-time.After(5 * time.Second):
		slog.Warn("websocket send timeout, client may be slow", "client_id", client.ID)
	}
}

func (h *Handler) sendError(client *Client, message string) {
	h.sendToClient(client, WSResponse{Type: "error", Message: message})
}

func mustMarshalWSResponse(resp WSResponse) []byte {
	pe := pool.GetJSONEncoder(nil)
	if err := pe.Encode(resp); err != nil {
		pool.PutJSONEncoder(pe)
		return nil
	}
	data := make([]byte, len(pe.Bytes()))
	copy(data, pe.Bytes())
	pool.PutJSONEncoder(pe)
	return data
}

func (h *Handler) sendJSON(client *Client, v any) {
	pe := pool.GetJSONEncoder(nil)
	if err := pe.Encode(v); err != nil {
		pool.PutJSONEncoder(pe)
		return
	}
	data := make([]byte, len(pe.Bytes()))
	copy(data, pe.Bytes())
	pool.PutJSONEncoder(pe)

	select {
	case client.send <- data:
	case <-time.After(5 * time.Second):
		slog.Warn("websocket send timeout, client may be slow", "client_id", client.ID)
	}
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

func (h *Handler) Close() {
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
	if h.gw != nil {
		h.gw.Close()
	}
	h.wg.Wait()
}

func (h *Handler) getWorkDir() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.workDir
}
