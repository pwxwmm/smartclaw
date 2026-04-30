package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/instructkr/smartclaw/internal/services"
)

func TestTeamRegistryGetOrCreate(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}

	tms := registry.GetOrCreate("team1")
	if tms == nil {
		t.Fatal("GetOrCreate should return non-nil")
	}

	tms2 := registry.GetOrCreate("team1")
	if tms != tms2 {
		t.Error("GetOrCreate should return same instance for same ID")
	}
}

func TestTeamRegistryGet(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}

	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent team")
	}

	registry.GetOrCreate("team1")
	tms, ok := registry.Get("team1")
	if !ok || tms == nil {
		t.Error("Get should return true for existing team")
	}
}

func TestTeamRegistryRemove(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}

	registry.GetOrCreate("team1")
	registry.Remove("team1")

	_, ok := registry.Get("team1")
	if ok {
		t.Error("Team should be removed")
	}
}

func TestTeamRegistryList(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}

	registry.GetOrCreate("team1")
	registry.GetOrCreate("team2")

	ids := registry.List()
	if len(ids) != 2 {
		t.Errorf("Expected 2 teams, got %d", len(ids))
	}
}

func TestTeamCreateToolExecute(t *testing.T) {
	tool := &TeamCreateTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name":        "Test Team",
		"description": "A test team",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["name"] != "Test Team" {
		t.Errorf("Expected name 'Test Team', got %v", resultMap["name"])
	}
	if resultMap["created"] != true {
		t.Error("Expected created=true")
	}
	if resultMap["id"] == "" {
		t.Error("Expected non-empty team ID")
	}
}

func TestTeamCreateToolMissingName(t *testing.T) {
	tool := &TeamCreateTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute should not return error: %v", err)
	}
}

func TestTeamDeleteToolExecute(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}
	originalRegistry := defaultTeamRegistry
	defaultTeamRegistry = registry
	defer func() { defaultTeamRegistry = originalRegistry }()

	registry.GetOrCreate("team_del")

	tool := &TeamDeleteTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"id": "team_del",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["deleted"] != true {
		t.Error("Expected deleted=true")
	}
}

func TestTeamShareMemoryToolTeamNotFound(t *testing.T) {
	tool := &TeamShareMemoryTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"team_id": "nonexistent",
		"title":   "test",
		"content": "test content",
	})
	if err == nil {
		t.Error("Expected error for nonexistent team")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found', got: %v", err)
	}
}

func TestTeamShareMemoryToolExecute(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}
	originalRegistry := defaultTeamRegistry
	defaultTeamRegistry = registry
	defer func() { defaultTeamRegistry = originalRegistry }()

	registry.GetOrCreate("team_share")

	tool := &TeamShareMemoryTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"team_id":    "team_share",
		"title":      "Test Memory",
		"content":    "Some content",
		"type":       "code",
		"visibility": "team",
		"tags":       []any{"test", "example"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["shared"] != true {
		t.Error("Expected shared=true")
	}
	if resultMap["memory_id"] == "" {
		t.Error("Expected non-empty memory_id")
	}
}

func TestTeamGetMemoriesToolTeamNotFound(t *testing.T) {
	tool := &TeamGetMemoriesTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"team_id": "nonexistent",
	})
	if err == nil {
		t.Error("Expected error for nonexistent team")
	}
}

func TestTeamGetMemoriesToolExecute(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}
	originalRegistry := defaultTeamRegistry
	defaultTeamRegistry = registry
	defer func() { defaultTeamRegistry = originalRegistry }()

	tms := registry.GetOrCreate("team_get_test_unique")
	tms.SetTeam(&services.Team{})

	shareTool := &TeamShareMemoryTool{}
	shareTool.Execute(context.Background(), map[string]any{
		"team_id": "team_get_test_unique",
		"title":   "Memory 1",
		"content": "Content 1",
	})

	tool := &TeamGetMemoriesTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"team_id": "team_get_test_unique",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	count := resultMap["count"].(int)
	if count < 1 {
		t.Errorf("Expected at least 1 memory, got %d", count)
	}
}

func TestTeamGetMemoriesToolFilterByType(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}
	originalRegistry := defaultTeamRegistry
	defaultTeamRegistry = registry
	defer func() { defaultTeamRegistry = originalRegistry }()

	tms := registry.GetOrCreate("team_type")
	tms.SetTeam(&services.Team{})

	shareTool := &TeamShareMemoryTool{}
	shareTool.Execute(context.Background(), map[string]any{
		"team_id": "team_type",
		"title":   "Code Memory",
		"content": "code content",
		"type":    "code",
	})

	tool := &TeamGetMemoriesTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"team_id": "team_type",
		"type":    "code",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["count"].(int) < 1 {
		t.Errorf("Expected at least 1 code memory, got %d", resultMap["count"])
	}
}

func TestTeamSearchMemoriesToolTeamNotFound(t *testing.T) {
	tool := &TeamSearchMemoriesTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"team_id": "nonexistent",
		"query":   "test",
	})
	if err == nil {
		t.Error("Expected error for nonexistent team")
	}
}

func TestTeamSearchMemoriesToolExecute(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}
	originalRegistry := defaultTeamRegistry
	defaultTeamRegistry = registry
	defer func() { defaultTeamRegistry = originalRegistry }()

	tms := registry.GetOrCreate("team_search")
	tms.SetTeam(&services.Team{})

	shareTool := &TeamShareMemoryTool{}
	shareTool.Execute(context.Background(), map[string]any{
		"team_id": "team_search",
		"title":   "go best practices",
		"content": "use interfaces for abstraction",
	})

	tool := &TeamSearchMemoriesTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"team_id": "team_search",
		"query":   "go best",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["count"].(int) < 1 {
		t.Errorf("Expected at least 1 search result, got %d", resultMap["count"])
	}
}

func TestTeamShareSessionToolExecute(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}
	originalRegistry := defaultTeamRegistry
	defaultTeamRegistry = registry
	defer func() { defaultTeamRegistry = originalRegistry }()

	registry.GetOrCreate("team_session")

	tool := &TeamShareSessionTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"team_id":    "team_session",
		"session_id": "sess_123",
		"summary":    "A productive session",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["shared"] != true {
		t.Error("Expected shared=true")
	}
}

func TestTeamShareSessionToolTeamNotFound(t *testing.T) {
	tool := &TeamShareSessionTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"team_id":    "nonexistent",
		"session_id": "sess_123",
	})
	if err == nil {
		t.Error("Expected error for nonexistent team")
	}
}

func TestTeamSyncToolTeamNotFound(t *testing.T) {
	tool := &TeamSyncTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"team_id": "nonexistent",
	})
	if err == nil {
		t.Error("Expected error for nonexistent team")
	}
}

func TestTeamSyncToolInvalidEncryptKey(t *testing.T) {
	registry := &TeamRegistry{
		teams: make(map[string]*services.TeamMemorySync),
	}
	originalRegistry := defaultTeamRegistry
	defaultTeamRegistry = registry
	defer func() { defaultTeamRegistry = originalRegistry }()

	registry.GetOrCreate("team_sync")

	tool := &TeamSyncTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"team_id":     "team_sync",
		"api_endpoint": "http://localhost",
		"encrypt_key": "xyz",
	})
	if err == nil {
		t.Error("Expected error for invalid hex encrypt_key")
	}
}

func TestParseMemoryType(t *testing.T) {
	tests := []struct {
		input string
		want  services.MemoryType
	}{
		{"code", services.MemoryTypeCode},
		{"conversation", services.MemoryTypeConversation},
		{"decision", services.MemoryTypeDecision},
		{"pattern", services.MemoryTypePattern},
		{"preference", services.MemoryTypePreference},
		{"", services.MemoryTypeCode},
		{"unknown", services.MemoryTypeCode},
		{"CODE", services.MemoryTypeCode},
	}

	for _, tt := range tests {
		got := parseMemoryType(tt.input)
		if got != tt.want {
			t.Errorf("parseMemoryType(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestMemoryTypeStr(t *testing.T) {
	tests := []struct {
		input services.MemoryType
		want  string
	}{
		{services.MemoryTypeCode, "code"},
		{services.MemoryTypeConversation, "conversation"},
		{services.MemoryTypeDecision, "decision"},
		{services.MemoryTypePattern, "pattern"},
		{services.MemoryTypePreference, "preference"},
		{services.MemoryType(99), "unknown"},
	}

	for _, tt := range tests {
		got := memoryTypeStr(tt.input)
		if got != tt.want {
			t.Errorf("memoryTypeStr(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseVisibility(t *testing.T) {
	tests := []struct {
		input string
		want  services.MemoryVisibility
	}{
		{"private", services.VisibilityPrivate},
		{"team", services.VisibilityTeam},
		{"public", services.VisibilityPublic},
		{"", services.VisibilityTeam},
		{"unknown", services.VisibilityTeam},
	}

	for _, tt := range tests {
		got := parseVisibility(tt.input)
		if got != tt.want {
			t.Errorf("parseVisibility(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestVisibilityStr(t *testing.T) {
	tests := []struct {
		input services.MemoryVisibility
		want  string
	}{
		{services.VisibilityPrivate, "private"},
		{services.VisibilityTeam, "team"},
		{services.VisibilityPublic, "public"},
		{services.MemoryVisibility(99), "unknown"},
	}

	for _, tt := range tests {
		got := visibilityStr(tt.input)
		if got != tt.want {
			t.Errorf("visibilityStr(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCreateTeamHint(t *testing.T) {
	hint := createTeamHint("xyz")
	if !strings.Contains(hint, "team_create") {
		t.Errorf("Hint should mention team_create, got: %q", hint)
	}
}
