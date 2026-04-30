package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/permissions"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/session"
)

// newTestHandler creates a Handler suitable for testing without real DB/LLM deps.
func newTestHandler() *Handler {
	hub := NewHub()
	return &Handler{
		hub:                 hub,
		workDir:             ".",
		prompt:              runtime.NewPromptBuilder(),
		unifiedPerm:         permissions.NewUnifiedPermissionEngine(permissions.NewApprovalGate(), nil),
		costGuard:           costguard.NewCostGuard(costguard.DefaultBudgetConfig()),
		clientSess:          make(map[string]*session.Session),
		clientModels:        make(map[string]string),
		clientCache:         make(map[string]*api.Client),
		pendingApprovals:    make(map[string]chan bool),
		pendingApprovalMeta: make(map[string]*ApprovalMeta),
		approvalHistory:     make([]ApprovalRecord, 0),
		autoApproved:        make(map[string]map[string]bool),
		cancelFuncs:         make(map[string]context.CancelFunc),
		shutdownCtx:         context.Background(),
	}
}

// newTestHandlerWithClient creates a Handler with an apiClient set.
func newTestHandlerWithClient() *Handler {
	h := newTestHandler()
	h.apiClient = &api.Client{Model: "test-model-xyz"}
	return h
}

// newTestWebServer creates a minimal WebServer for HTTP handler tests.
func newTestWebServer() *WebServer {
	h := newTestHandler()
	am := newTestAuthManager("")
	return &WebServer{
		hub:         h.hub,
		handler:     h,
		authManager: am,
		noAuth:      true,
	}
}

// ---------- validateWSMessage tests ----------

func TestValidateWSMessage_EmptyType(t *testing.T) {
	h := newTestHandler()
	err := h.validateWSMessage(WSMessage{Type: ""})
	if err == nil {
		t.Fatal("expected error for empty type, got nil")
	}
	if err.Error() != "message type is required" {
		t.Errorf("error = %q, want %q", err.Error(), "message type is required")
	}
}

func TestValidateWSMessage_UnknownType(t *testing.T) {
	h := newTestHandler()
	err := h.validateWSMessage(WSMessage{Type: "nonexistent_type"})
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "unknown message type") {
		t.Errorf("error = %q, want it to contain 'unknown message type'", err.Error())
	}
}

func TestValidateWSMessage_ValidType(t *testing.T) {
	h := newTestHandler()
	for typ := range validWSTypes {
		// Supply required data for types that need it
		msg := WSMessage{Type: typ}
		if typesRequiringData[typ] {
			msg.Content = "test"
		}
		err := h.validateWSMessage(msg)
		if err != nil {
			t.Errorf("validateWSMessage(%q) returned error: %v", typ, err)
		}
	}
}

func TestValidateWSMessage_TypeRequiringDataButNoData(t *testing.T) {
	h := newTestHandler()
	for typ := range typesRequiringData {
		msg := WSMessage{Type: typ} // no Content, Data, Path, Name, ID, Model, Title, Args
		err := h.validateWSMessage(msg)
		if err == nil {
			t.Errorf("validateWSMessage(%q) should return error when data is required but missing", typ)
		}
		if !strings.Contains(err.Error(), "requires data payload") {
			t.Errorf("error = %q, want it to contain 'requires data payload'", err.Error())
		}
	}
}

func TestValidateWSMessage_TypeRequiringDataWithData(t *testing.T) {
	h := newTestHandler()
	// Test with Content field
	for typ := range typesRequiringData {
		msg := WSMessage{Type: typ, Content: "some content"}
		err := h.validateWSMessage(msg)
		if err != nil {
			t.Errorf("validateWSMessage(%q) with Content should return nil, got: %v", typ, err)
		}
	}
}

func TestValidateWSMessage_TypeRequiringDataWithVariousFields(t *testing.T) {
	h := newTestHandler()
	// Test that various data fields satisfy the data requirement
	tests := []struct {
		name string
		msg  WSMessage
	}{
		{"content", WSMessage{Type: "chat", Content: "hello"}},
		{"path", WSMessage{Type: "file_open", Path: "/tmp/test.go"}},
		{"name", WSMessage{Type: "model", Name: "claude"}},
		{"id", WSMessage{Type: "session_load", ID: "sess-123"}},
		{"model", WSMessage{Type: "model", Model: "gpt-4"}},
		{"title", WSMessage{Type: "session_new", Title: "My Session"}},
		{"args", WSMessage{Type: "cmd", Args: []string{"ls"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateWSMessage(tt.msg)
			if err != nil {
				t.Errorf("validateWSMessage with %s should return nil, got: %v", tt.name, err)
			}
		})
	}
}

func TestValidateWSMessage_TypeNotRequiringDataWithNoData(t *testing.T) {
	h := newTestHandler()
	for typ := range validWSTypes {
		if !typesRequiringData[typ] {
			msg := WSMessage{Type: typ}
			err := h.validateWSMessage(msg)
			if err != nil {
				t.Errorf("validateWSMessage(%q) for type not requiring data should return nil, got: %v", typ, err)
			}
		}
	}
}

// ---------- Handler construction tests ----------

func TestNewHandler_NonNilFields(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, ".", nil)

	if h.hub == nil {
		t.Error("hub should not be nil")
	}
	if h.prompt == nil {
		t.Error("prompt should not be nil")
	}
	if h.unifiedPerm == nil {
		t.Error("unifiedPerm should not be nil")
	}
	if h.costGuard == nil {
		t.Error("costGuard should not be nil")
	}
}

func TestNewHandler_PendingApprovals(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, ".", nil)

	if h.pendingApprovals == nil {
		t.Error("pendingApprovals should be initialized")
	}
	if len(h.pendingApprovals) != 0 {
		t.Errorf("pendingApprovals should be empty, got %d entries", len(h.pendingApprovals))
	}
}

func TestNewHandler_AutoApproved(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, ".", nil)

	if h.autoApproved == nil {
		t.Error("autoApproved should be initialized")
	}
	if len(h.autoApproved) != 0 {
		t.Errorf("autoApproved should be empty, got %d entries", len(h.autoApproved))
	}
}

func TestNewHandler_PendingApprovalMeta(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, ".", nil)

	if h.pendingApprovalMeta == nil {
		t.Error("pendingApprovalMeta should be initialized")
	}
}

func TestNewHandler_ApprovalHistory(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, ".", nil)

	if h.approvalHistory == nil {
		t.Error("approvalHistory should be initialized")
	}
	if len(h.approvalHistory) != 0 {
		t.Errorf("approvalHistory should be empty, got %d entries", len(h.approvalHistory))
	}
}

func TestNewHandler_CancelFuncs(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, ".", nil)

	if h.cancelFuncs == nil {
		t.Error("cancelFuncs should be initialized")
	}
}

func TestNewHandler_ClientMaps(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, ".", nil)

	if h.clientModels == nil {
		t.Error("clientModels should be initialized")
	}
	if h.clientCache == nil {
		t.Error("clientCache should be initialized")
	}
	if h.clientSess == nil {
		t.Error("clientSess should be initialized")
	}
}

// ---------- GetStats tests ----------

func TestGetStats_WithAPIClient(t *testing.T) {
	h := newTestHandlerWithClient()
	stats := h.GetStats()

	if stats.Model != "test-model-xyz" {
		t.Errorf("Model = %q, want %q", stats.Model, "test-model-xyz")
	}
	if stats.TokensLimit != 200000 {
		t.Errorf("TokensLimit = %d, want 200000", stats.TokensLimit)
	}
}

func TestGetStats_NilAPIClient(t *testing.T) {
	h := newTestHandler() // no apiClient
	stats := h.GetStats()

	if stats.Model != "sre-model" {
		t.Errorf("Model = %q, want %q", stats.Model, "sre-model")
	}
	if stats.TokensLimit != 200000 {
		t.Errorf("TokensLimit = %d, want 200000", stats.TokensLimit)
	}
}

// ---------- HandleMessage routing tests ----------

func TestHandleMessage_InvalidJSON(t *testing.T) {
	h := newTestHandler()
	hub := NewHub()
	client := NewClient(hub, "test-user")

	h.HandleMessage(client, []byte("not json at all"))

	select {
	case msg := <-client.send:
		var resp WSResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.Type != "error" {
			t.Errorf("response type = %q, want %q", resp.Type, "error")
		}
		if resp.Message != "Invalid message format" {
			t.Errorf("response message = %q, want %q", resp.Message, "Invalid message format")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error response")
	}
}

func TestHandleMessage_UnknownType(t *testing.T) {
	h := newTestHandler()
	hub := NewHub()
	client := NewClient(hub, "test-user")

	msg, _ := json.Marshal(WSMessage{Type: "bogus_type"})
	h.HandleMessage(client, msg)

	select {
	case raw := <-client.send:
		var resp WSResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if resp.Type != "error" {
			t.Errorf("type = %q, want error", resp.Type)
		}
		if !strings.Contains(resp.Content, "unknown message type") {
			t.Errorf("content = %q, want to contain 'unknown message type'", resp.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestHandleMessage_ValidTypeMissingRequiredData(t *testing.T) {
	h := newTestHandler()
	hub := NewHub()
	client := NewClient(hub, "test-user")

	msg, _ := json.Marshal(WSMessage{Type: "chat"}) // chat requires data
	h.HandleMessage(client, msg)

	select {
	case raw := <-client.send:
		var resp WSResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if resp.Type != "error" {
			t.Errorf("type = %q, want error", resp.Type)
		}
		if !strings.Contains(resp.Content, "requires data payload") {
			t.Errorf("content = %q, want to contain 'requires data payload'", resp.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestHandleMessage_ValidTypesRoute(t *testing.T) {
	// Test that valid message types with required data are routed without
	// validation errors. We check 3 different types: "abort", "approval_list",
	// and "session_list" (these don't need apiClient/DB to pass validation).
	h := newTestHandler()
	hub := NewHub()

	tests := []struct {
		name string
		msg  WSMessage
	}{
		{"abort_no_data", WSMessage{Type: "abort"}},
		{"approval_list_no_data", WSMessage{Type: "approval_list"}},
		{"session_list_no_data", WSMessage{Type: "session_list"}},
		{"memory_layers_no_data", WSMessage{Type: "memory_layers"}},
		{"mcp_catalog_no_data", WSMessage{Type: "mcp_catalog"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(hub, "test-user")
			raw, _ := json.Marshal(tt.msg)
			h.HandleMessage(client, raw)

			// Read any response from client.send - we just verify no validation error
			select {
			case resp := <-client.send:
				var wsResp WSResponse
				if err := json.Unmarshal(resp, &wsResp); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				// We should NOT get a validation error (type="error" with
				// content containing "unknown" or "required" or "data payload")
				if wsResp.Type == "error" &&
					(strings.Contains(wsResp.Content, "unknown message type") ||
						strings.Contains(wsResp.Content, "requires data payload") ||
						strings.Contains(wsResp.Content, "message type is required")) {
					t.Errorf("got validation error for valid type %q: %s", tt.msg.Type, wsResp.Content)
				}
			case <-time.After(time.Second):
				// Some handlers may not send a response, which is fine - no error means routing worked
			}
		})
	}
}

func TestHandleMessage_EmptyType(t *testing.T) {
	h := newTestHandler()
	hub := NewHub()
	client := NewClient(hub, "test-user")

	msg, _ := json.Marshal(WSMessage{Type: ""})
	h.HandleMessage(client, msg)

	select {
	case raw := <-client.send:
		var resp WSResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if resp.Type != "error" {
			t.Errorf("type = %q, want error", resp.Type)
		}
		if !strings.Contains(resp.Content, "message type is required") {
			t.Errorf("content = %q, want to contain 'message type is required'", resp.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

// ---------- WSResponse JSON serialization tests ----------

func TestWSResponse_JSONSerialization(t *testing.T) {
	now := time.Now()
	resp := WSResponse{
		Type:     "tool_result",
		Content:  "hello world",
		Tool:     "bash",
		Input:    map[string]any{"command": "ls"},
		Output:   "file1.go\nfile2.go",
		Duration: 150,
		ID:       "msg-123",
		Title:    "Test Title",
		Status:   "completed",
		Progress: 0.75,
		Tokens:   1024,
		Cost:     0.035,
		Message:  "done",
		Model:    "claude-3",
		Text:     "some text",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal WSResponse: %v", err)
	}

	var decoded WSResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal WSResponse: %v", err)
	}

	if decoded.Type != resp.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, resp.Type)
	}
	if decoded.Content != resp.Content {
		t.Errorf("Content = %q, want %q", decoded.Content, resp.Content)
	}
	if decoded.Tool != resp.Tool {
		t.Errorf("Tool = %q, want %q", decoded.Tool, resp.Tool)
	}
	if decoded.Duration != resp.Duration {
		t.Errorf("Duration = %d, want %d", decoded.Duration, resp.Duration)
	}
	if decoded.ID != resp.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, resp.ID)
	}
	if decoded.Title != resp.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, resp.Title)
	}
	if decoded.Status != resp.Status {
		t.Errorf("Status = %q, want %q", decoded.Status, resp.Status)
	}
	if decoded.Progress != resp.Progress {
		t.Errorf("Progress = %f, want %f", decoded.Progress, resp.Progress)
	}
	if decoded.Tokens != resp.Tokens {
		t.Errorf("Tokens = %d, want %d", decoded.Tokens, resp.Tokens)
	}
	if decoded.Cost != resp.Cost {
		t.Errorf("Cost = %f, want %f", decoded.Cost, resp.Cost)
	}
	if decoded.Message != resp.Message {
		t.Errorf("Message = %q, want %q", decoded.Message, resp.Message)
	}
	if decoded.Model != resp.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, resp.Model)
	}
	if decoded.Text != resp.Text {
		t.Errorf("Text = %q, want %q", decoded.Text, resp.Text)
	}
	_ = now // suppress unused warning
}

func TestWSResponse_EmptyFields(t *testing.T) {
	resp := WSResponse{Type: "error"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if m["type"] != "error" {
		t.Errorf("type = %v, want error", m["type"])
	}
	// omitempty fields should not appear
	if _, ok := m["content"]; ok {
		t.Error("content should be omitted when empty")
	}
	if _, ok := m["tool"]; ok {
		t.Error("tool should be omitted when empty")
	}
}

func TestWSResponse_WithTree(t *testing.T) {
	tree := []FileNode{
		{Name: "src", Type: "dir", Children: []FileNode{
			{Name: "main.go", Type: "file", Size: 1024},
		}},
	}
	resp := WSResponse{Type: "file_tree", Tree: tree}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WSResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.Tree) != 1 {
		t.Fatalf("Tree len = %d, want 1", len(decoded.Tree))
	}
	if decoded.Tree[0].Name != "src" {
		t.Errorf("Tree[0].Name = %q, want src", decoded.Tree[0].Name)
	}
	if len(decoded.Tree[0].Children) != 1 {
		t.Fatalf("Tree[0].Children len = %d, want 1", len(decoded.Tree[0].Children))
	}
	if decoded.Tree[0].Children[0].Name != "main.go" {
		t.Errorf("Children[0].Name = %q, want main.go", decoded.Tree[0].Children[0].Name)
	}
}

func TestWSResponse_WithSessions(t *testing.T) {
	sessions := []SessionInfo{
		{ID: "s1", Title: "Test", Model: "claude", MessageCount: 5, CreatedAt: "2024-01-01", UpdatedAt: "2024-01-02"},
	}
	resp := WSResponse{Type: "session_list", Sessions: sessions}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WSResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.Sessions) != 1 {
		t.Fatalf("Sessions len = %d, want 1", len(decoded.Sessions))
	}
	if decoded.Sessions[0].ID != "s1" {
		t.Errorf("Sessions[0].ID = %q, want s1", decoded.Sessions[0].ID)
	}
	if decoded.Sessions[0].MessageCount != 5 {
		t.Errorf("Sessions[0].MessageCount = %d, want 5", decoded.Sessions[0].MessageCount)
	}
}

func TestWSResponse_WithMessages(t *testing.T) {
	msgs := []MsgItem{
		{Role: "user", Content: "hello", Timestamp: "2024-01-01T00:00:00Z"},
		{Role: "assistant", Content: "hi", Timestamp: "2024-01-01T00:00:01Z"},
	}
	resp := WSResponse{Type: "chat", Messages: msgs}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WSResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(decoded.Messages))
	}
	if decoded.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want user", decoded.Messages[0].Role)
	}
}

// ---------- FileNode struct tests ----------

func TestFileNode_JSONSerialization(t *testing.T) {
	node := FileNode{
		Name: "project",
		Type: "dir",
		Size: 0,
		Children: []FileNode{
			{Name: "main.go", Type: "file", Size: 2048},
			{Name: "utils", Type: "dir", Children: []FileNode{
				{Name: "helpers.go", Type: "file", Size: 512},
			}},
		},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded FileNode
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Name != "project" {
		t.Errorf("Name = %q, want project", decoded.Name)
	}
	if decoded.Type != "dir" {
		t.Errorf("Type = %q, want dir", decoded.Type)
	}
	if len(decoded.Children) != 2 {
		t.Fatalf("Children len = %d, want 2", len(decoded.Children))
	}
	if decoded.Children[0].Size != 2048 {
		t.Errorf("Children[0].Size = %d, want 2048", decoded.Children[0].Size)
	}
	if len(decoded.Children[1].Children) != 1 {
		t.Fatalf("nested Children len = %d, want 1", len(decoded.Children[1].Children))
	}
}

func TestFileNode_Empty(t *testing.T) {
	node := FileNode{}
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if m["name"] != "" {
		t.Errorf("name should be empty string, got %v", m["name"])
	}
}

func TestFileNode_OmitEmptyFields(t *testing.T) {
	node := FileNode{Name: "test.go", Type: "file"}
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Size is omitempty and zero, should not appear
	if _, ok := m["size"]; ok {
		t.Error("size should be omitted when zero")
	}
	// Children is omitempty and nil, should not appear
	if _, ok := m["children"]; ok {
		t.Error("children should be omitted when nil")
	}
}

// ---------- SessionInfo struct tests ----------

func TestSessionInfo_JSONSerialization(t *testing.T) {
	si := SessionInfo{
		ID:           "sess-abc123",
		UserID:       "user1",
		Title:        "My Session",
		Model:        "claude-3-opus",
		MessageCount: 42,
		CreatedAt:    "2024-01-15T10:30:00Z",
		UpdatedAt:    "2024-01-15T11:00:00Z",
	}

	data, err := json.Marshal(si)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded SessionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != si.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, si.ID)
	}
	if decoded.UserID != si.UserID {
		t.Errorf("UserID = %q, want %q", decoded.UserID, si.UserID)
	}
	if decoded.Title != si.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, si.Title)
	}
	if decoded.Model != si.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, si.Model)
	}
	if decoded.MessageCount != si.MessageCount {
		t.Errorf("MessageCount = %d, want %d", decoded.MessageCount, si.MessageCount)
	}
	if decoded.CreatedAt != si.CreatedAt {
		t.Errorf("CreatedAt = %q, want %q", decoded.CreatedAt, si.CreatedAt)
	}
	if decoded.UpdatedAt != si.UpdatedAt {
		t.Errorf("UpdatedAt = %q, want %q", decoded.UpdatedAt, si.UpdatedAt)
	}
}

func TestSessionInfo_OmitEmptyUserID(t *testing.T) {
	si := SessionInfo{ID: "s1", Title: "T", Model: "m", MessageCount: 0, CreatedAt: "2024-01-01", UpdatedAt: "2024-01-01"}
	data, err := json.Marshal(si)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// UserID is omitempty, should not appear when empty
	if _, ok := m["userId"]; ok {
		t.Error("userId should be omitted when empty")
	}
}

// ---------- StatsResponse struct tests ----------

func TestStatsResponse_JSONSerialization(t *testing.T) {
	sr := StatsResponse{
		TokensUsed:   5000,
		TokensLimit:  200000,
		Cost:         0.15,
		Model:        "claude-3-opus",
		SessionCount: 3,
	}

	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded StatsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.TokensUsed != sr.TokensUsed {
		t.Errorf("TokensUsed = %d, want %d", decoded.TokensUsed, sr.TokensUsed)
	}
	if decoded.TokensLimit != sr.TokensLimit {
		t.Errorf("TokensLimit = %d, want %d", decoded.TokensLimit, sr.TokensLimit)
	}
	if decoded.Cost != sr.Cost {
		t.Errorf("Cost = %f, want %f", decoded.Cost, sr.Cost)
	}
	if decoded.Model != sr.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, sr.Model)
	}
	if decoded.SessionCount != sr.SessionCount {
		t.Errorf("SessionCount = %d, want %d", decoded.SessionCount, sr.SessionCount)
	}
}

func TestStatsResponse_DefaultValues(t *testing.T) {
	sr := StatsResponse{}
	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded StatsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.TokensUsed != 0 {
		t.Errorf("default TokensUsed = %d, want 0", decoded.TokensUsed)
	}
	if decoded.TokensLimit != 0 {
		t.Errorf("default TokensLimit = %d, want 0", decoded.TokensLimit)
	}
	if decoded.Cost != 0 {
		t.Errorf("default Cost = %f, want 0", decoded.Cost)
	}
	if decoded.Model != "" {
		t.Errorf("default Model = %q, want empty", decoded.Model)
	}
}

// ---------- ApprovalMeta and ApprovalRecord struct tests ----------

func TestApprovalMeta_JSONSerialization(t *testing.T) {
	ts := time.Now()
	meta := ApprovalMeta{
		ToolName:    "bash",
		Input:       map[string]any{"command": "rm -rf /"},
		RequestedAt: ts,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ApprovalMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ToolName != "bash" {
		t.Errorf("ToolName = %q, want bash", decoded.ToolName)
	}
}

func TestApprovalRecord_JSONSerialization(t *testing.T) {
	ts := time.Now()
	rec := ApprovalRecord{
		BlockID:   "block-1",
		ToolName:  "write_file",
		Decision:  "approved",
		Timestamp: ts,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ApprovalRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.BlockID != "block-1" {
		t.Errorf("BlockID = %q, want block-1", decoded.BlockID)
	}
	if decoded.ToolName != "write_file" {
		t.Errorf("ToolName = %q, want write_file", decoded.ToolName)
	}
	if decoded.Decision != "approved" {
		t.Errorf("Decision = %q, want approved", decoded.Decision)
	}
}

// ---------- WSMessage struct tests ----------

func TestWSMessage_JSONSerialization(t *testing.T) {
	msg := WSMessage{
		Type:    "chat",
		Content: "hello world",
		Name:    "test-name",
		Args:    []string{"arg1", "arg2"},
		Path:    "/tmp/test",
		ID:      "id-123",
		Title:   "My Title",
		Model:   "claude-3",
		Data:    json.RawMessage(`{"key":"value"}`),
		Images:  json.RawMessage(`[{"url":"http://example.com/img.png"}]`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WSMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Type != "chat" {
		t.Errorf("Type = %q, want chat", decoded.Type)
	}
	if decoded.Content != "hello world" {
		t.Errorf("Content = %q, want hello world", decoded.Content)
	}
	if decoded.Name != "test-name" {
		t.Errorf("Name = %q, want test-name", decoded.Name)
	}
	if len(decoded.Args) != 2 {
		t.Fatalf("Args len = %d, want 2", len(decoded.Args))
	}
	if decoded.Args[0] != "arg1" {
		t.Errorf("Args[0] = %q, want arg1", decoded.Args[0])
	}
	if decoded.Path != "/tmp/test" {
		t.Errorf("Path = %q, want /tmp/test", decoded.Path)
	}
	if decoded.ID != "id-123" {
		t.Errorf("ID = %q, want id-123", decoded.ID)
	}
	if decoded.Title != "My Title" {
		t.Errorf("Title = %q, want My Title", decoded.Title)
	}
	if decoded.Model != "claude-3" {
		t.Errorf("Model = %q, want claude-3", decoded.Model)
	}
}

func TestWSMessage_OmitEmptyFields(t *testing.T) {
	msg := WSMessage{Type: "abort"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if m["type"] != "abort" {
		t.Errorf("type = %v, want abort", m["type"])
	}
	// These omitempty fields should not appear
	omitFields := []string{"content", "name", "args", "path", "id", "title", "model", "data", "images"}
	for _, field := range omitFields {
		if _, ok := m[field]; ok {
			t.Errorf("%q should be omitted when empty", field)
		}
	}
}

// ---------- MsgItem struct tests ----------

func TestMsgItem_JSONSerialization(t *testing.T) {
	mi := MsgItem{
		Role:      "assistant",
		Content:   "response text",
		Timestamp: "2024-06-15T12:00:00Z",
	}

	data, err := json.Marshal(mi)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded MsgItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Role != "assistant" {
		t.Errorf("Role = %q, want assistant", decoded.Role)
	}
	if decoded.Content != "response text" {
		t.Errorf("Content = %q, want response text", decoded.Content)
	}
	if decoded.Timestamp != "2024-06-15T12:00:00Z" {
		t.Errorf("Timestamp = %q, want 2024-06-15T12:00:00Z", decoded.Timestamp)
	}
}

// ---------- handleFrontendTelemetry tests ----------

func TestHandleFrontendTelemetry_Post(t *testing.T) {
	s := newTestWebServer()
	req := httptest.NewRequest("POST", "/api/telemetry/frontend", strings.NewReader(`{"event":"click"}`))
	w := httptest.NewRecorder()

	s.handleFrontendTelemetry(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
}

func TestHandleFrontendTelemetry_NonPost(t *testing.T) {
	s := newTestWebServer()

	methods := []string{"GET", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/telemetry/frontend", nil)
			w := httptest.NewRecorder()
			s.handleFrontendTelemetry(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}

			var result map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if result["error"] != "method not allowed" {
				t.Errorf("error = %q, want 'method not allowed'", result["error"])
			}
		})
	}
}

// ---------- handleAgentsAPI tests ----------

func TestHandleAgentsAPI_NonGet(t *testing.T) {
	s := newTestWebServer()

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/agents", nil)
			w := httptest.NewRecorder()
			s.handleAgentsAPI(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// ---------- handleMemoryAPI tests ----------

func TestHandleMemoryAPI_NonGet(t *testing.T) {
	s := newTestWebServer()

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/memory", nil)
			w := httptest.NewRecorder()
			s.handleMemoryAPI(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleMemoryAPI_Get_NilMemMgr(t *testing.T) {
	s := newTestWebServer()
	// handler.memMgr is nil by default in newTestHandler

	req := httptest.NewRequest("GET", "/api/memory", nil)
	w := httptest.NewRecorder()
	s.handleMemoryAPI(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if errMsg, ok := result["error"]; ok {
		if errMsg != "Memory manager not available" {
			t.Errorf("error = %v, want 'Memory manager not available'", errMsg)
		}
	}
}

// ---------- handleMemorySearch tests ----------

func TestHandleMemorySearch_NonPost(t *testing.T) {
	s := newTestWebServer()

	methods := []string{"GET", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/memory/search", nil)
			w := httptest.NewRecorder()
			s.handleMemorySearch(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleMemorySearch_Post_NilMemMgr(t *testing.T) {
	s := newTestWebServer()

	body := `{"query":"test","limit":5}`
	req := httptest.NewRequest("POST", "/api/memory/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMemorySearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result []any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	// Should return empty array when memMgr is nil
	if len(result) != 0 {
		t.Errorf("result length = %d, want 0", len(result))
	}
}

func TestHandleMemorySearch_Post_InvalidJSON(t *testing.T) {
	s := newTestWebServer()

	req := httptest.NewRequest("POST", "/api/memory/search", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMemorySearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------- handleStats tests ----------

func TestHandleStats(t *testing.T) {
	s := newTestWebServer()
	s.handler.apiClient = &api.Client{Model: "test-model"}

	req := httptest.NewRequest("GET", "/api/stats", nil)
	w := httptest.NewRecorder()
	s.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result StatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Model != "test-model" {
		t.Errorf("model = %q, want test-model", result.Model)
	}
	if result.TokensLimit != 200000 {
		t.Errorf("tokensLimit = %d, want 200000", result.TokensLimit)
	}
}

func TestHandleStats_DefaultModel(t *testing.T) {
	s := newTestWebServer()
	// apiClient is nil

	req := httptest.NewRequest("GET", "/api/stats", nil)
	w := httptest.NewRecorder()
	s.handleStats(w, req)

	var result StatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Model != "sre-model" {
		t.Errorf("model = %q, want sre-model", result.Model)
	}
}

// ---------- handleConfig tests ----------

func TestHandleConfig_MethodNotAllowed(t *testing.T) {
	s := newTestWebServer()

	req := httptest.NewRequest("PATCH", "/api/config", nil)
	w := httptest.NewRecorder()
	s.handleConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleConfig_Get_NoConfigFile(t *testing.T) {
	s := newTestWebServer()

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	s.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleConfig_Post_InvalidJSON(t *testing.T) {
	s := newTestWebServer()

	req := httptest.NewRequest("POST", "/api/config", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.handleConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------- handleFileUpload tests ----------

func TestHandleFileUpload_NonPost(t *testing.T) {
	s := newTestWebServer()

	methods := []string{"GET", "PUT", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/upload", nil)
			w := httptest.NewRecorder()
			s.handleFileUpload(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}

			var result map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if result["error"] != "method not allowed" {
				t.Errorf("error = %q, want 'method not allowed'", result["error"])
			}
		})
	}
}

// ---------- handleFileContent tests ----------

func TestHandleFileContent_MissingPath(t *testing.T) {
	s := newTestWebServer()

	req := httptest.NewRequest("GET", "/api/file", nil)
	w := httptest.NewRecorder()
	s.handleFileContent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["error"] != "path required" {
		t.Errorf("error = %q, want 'path required'", result["error"])
	}
}

// ---------- validWSTypes map coverage test ----------

func TestValidWSTypes_AllTypesRequiringDataAreValid(t *testing.T) {
	for typ := range typesRequiringData {
		if !validWSTypes[typ] {
			t.Errorf("typesRequiringData[%q] = true but validWSTypes[%q] = false", typ, typ)
		}
	}
}

func TestValidWSTypes_SomeTypesDontRequireData(t *testing.T) {
	typesWithoutData := []string{"abort", "approval_list", "approval_history", "skill_list", "memory_layers", "memory_stats", "memory_observations", "skill_health", "wiki_pages", "mcp_list", "mcp_catalog", "agent_list", "template_list", "cron_list"}
	for _, typ := range typesWithoutData {
		if !validWSTypes[typ] {
			t.Errorf("expected %q in validWSTypes", typ)
		}
		if typesRequiringData[typ] {
			t.Errorf("expected %q NOT in typesRequiringData", typ)
		}
	}
}

// ---------- sendToClient / sendError tests ----------

func TestSendError(t *testing.T) {
	h := newTestHandler()
	hub := NewHub()
	client := NewClient(hub, "test-user")

	h.sendError(client, "test error message")

	select {
	case raw := <-client.send:
		var resp WSResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if resp.Type != "error" {
			t.Errorf("type = %q, want error", resp.Type)
		}
		if resp.Message != "test error message" {
			t.Errorf("message = %q, want 'test error message'", resp.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error response")
	}
}

func TestSendToClient(t *testing.T) {
	h := newTestHandler()
	hub := NewHub()
	client := NewClient(hub, "test-user")

	resp := WSResponse{Type: "test", Content: "hello"}
	h.sendToClient(client, resp)

	select {
	case raw := <-client.send:
		var decoded WSResponse
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if decoded.Type != "test" {
			t.Errorf("type = %q, want test", decoded.Type)
		}
		if decoded.Content != "hello" {
			t.Errorf("content = %q, want hello", decoded.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

// ---------- mustMarshalWSResponse test ----------

func TestMustMarshalWSResponse(t *testing.T) {
	resp := WSResponse{Type: "test", Content: "data"}
	data := mustMarshalWSResponse(resp)
	if data == nil {
		t.Fatal("mustMarshalWSResponse returned nil")
	}

	var decoded WSResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if decoded.Type != "test" {
		t.Errorf("type = %q, want test", decoded.Type)
	}
}

// ---------- clientForRequest tests ----------

func TestClientForRequest_DefaultModel(t *testing.T) {
	h := newTestHandlerWithClient()
	client := h.clientForRequest("client-1")
	if client == nil {
		t.Fatal("client should not be nil")
	}
	if client.Model != "test-model-xyz" {
		t.Errorf("model = %q, want test-model-xyz", client.Model)
	}
}

func TestClientForRequest_CustomModel(t *testing.T) {
	h := newTestHandlerWithClient()
	h.clientModels["client-2"] = "custom-model"

	client := h.clientForRequest("client-2")
	if client == nil {
		t.Fatal("client should not be nil")
	}
	if client.Model != "custom-model" {
		t.Errorf("model = %q, want custom-model", client.Model)
	}
}

func TestClientForRequest_CachedCustomModel(t *testing.T) {
	h := newTestHandlerWithClient()
	h.clientModels["client-3"] = "cached-model"

	// First call should create and cache
	client1 := h.clientForRequest("client-3")
	// Second call should return from cache
	client2 := h.clientForRequest("client-3")

	if client1 != client2 {
		t.Error("expected same client instance from cache")
	}
}

// ---------- handleAuthLogin method test ----------

func TestHandleAuthLogin_NonPost(t *testing.T) {
	am := newTestAuthManager("testkey")
	s := &WebServer{authManager: am}

	methods := []string{"GET", "PUT", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/auth/login", nil)
			w := httptest.NewRecorder()
			s.handleAuthLogin(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// ---------- handleSessions tests ----------

func TestHandleSessions_NilStore_NilMgr(t *testing.T) {
	s := newTestWebServer()
	s.handler.dataStore = nil
	s.handler.sessMgr = nil

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	s.handleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result []any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// ---------- handleMemoryUpdateAPI tests ----------

func TestHandleMemoryUpdateAPI_NonPost(t *testing.T) {
	s := newTestWebServer()

	methods := []string{"GET", "PUT", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/memory/update", nil)
			w := httptest.NewRecorder()
			s.handleMemoryUpdateAPI(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleMemoryUpdateAPI_Post_InvalidJSON(t *testing.T) {
	s := newTestWebServer()

	req := httptest.NewRequest("POST", "/api/memory/update", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.handleMemoryUpdateAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleMemoryUpdateAPI_Post_InvalidFile(t *testing.T) {
	s := newTestWebServer()

	body := `{"file":"invalid","content":"test"}`
	req := httptest.NewRequest("POST", "/api/memory/update", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleMemoryUpdateAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleMemoryUpdateAPI_Post_EmptyContent(t *testing.T) {
	s := newTestWebServer()

	body := `{"file":"memory","content":""}`
	req := httptest.NewRequest("POST", "/api/memory/update", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleMemoryUpdateAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
