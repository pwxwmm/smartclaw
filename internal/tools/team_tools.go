package tools

import (
	"context"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/services"
	"github.com/instructkr/smartclaw/internal/store"
)

type TeamRegistry struct {
	teams map[string]*services.TeamMemorySync
	store *store.Store
	mu    sync.RWMutex
}

var defaultTeamRegistry = &TeamRegistry{
	teams: make(map[string]*services.TeamMemorySync),
}

func GetTeamRegistry() *TeamRegistry {
	return defaultTeamRegistry
}

func (tr *TeamRegistry) SetStore(s *store.Store) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.store = s
}

func (tr *TeamRegistry) GetOrCreate(teamID string) *services.TeamMemorySync {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tms, ok := tr.teams[teamID]; ok {
		return tms
	}

	tms := services.NewTeamMemorySync(teamID)
	tr.teams[teamID] = tms
	return tms
}

func (tr *TeamRegistry) Get(teamID string) (*services.TeamMemorySync, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	tms, ok := tr.teams[teamID]
	return tms, ok
}

func (tr *TeamRegistry) Remove(teamID string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	delete(tr.teams, teamID)

	if tr.store != nil {
		if err := tr.store.DeleteTeam(context.Background(), teamID); err != nil {
			slog.Warn("team: failed to delete team from store", "team_id", teamID, "error", err)
		}
	}
}

func (tr *TeamRegistry) List() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	ids := make([]string, 0, len(tr.teams))
	for id := range tr.teams {
		ids = append(ids, id)
	}
	return ids
}

func (tr *TeamRegistry) PersistTeam(ctx context.Context, team *services.Team) {
	if tr.store == nil {
		return
	}

	settingsJSON, _ := json.Marshal(team.Settings)
	record := &store.TeamRecord{
		ID:          team.ID,
		Name:        team.Name,
		Description: team.Description,
		Settings:    string(settingsJSON),
		CreatedAt:   team.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   team.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if err := tr.store.SaveTeam(ctx, record); err != nil {
		slog.Warn("team: failed to persist team", "team_id", team.ID, "error", err)
	}
}

func (tr *TeamRegistry) PersistMemory(ctx context.Context, teamID string, memory *services.Memory) {
	if tr.store == nil {
		return
	}

	tagsJSON, _ := json.Marshal(memory.Tags)
	record := &store.TeamMemoryRecord{
		TeamID:     teamID,
		MemoryID:   memory.ID,
		Title:      memory.Title,
		Content:    memory.Content,
		Type:       memoryTypeStr(memory.Type),
		Visibility: visibilityStr(memory.Visibility),
		Tags:       string(tagsJSON),
		AuthorID:   memory.UserID,
		CreatedAt:  memory.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if err := tr.store.SaveTeamMemory(ctx, record); err != nil {
		slog.Warn("team: failed to persist memory", "team_id", teamID, "error", err)
	}
}

func (tr *TeamRegistry) LoadFromStore(ctx context.Context) {
	if tr.store == nil {
		return
	}

	teams, err := tr.store.ListTeams(ctx)
	if err != nil {
		slog.Warn("team: failed to load teams from store", "error", err)
		return
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()

	for _, record := range teams {
		if _, exists := tr.teams[record.ID]; exists {
			continue
		}

		tms := services.NewTeamMemorySync(record.ID)

		var settings services.TeamSettings
		if record.Settings != "" {
			json.Unmarshal([]byte(record.Settings), &settings)
		}

		var createdAt, updatedAt time.Time
		if t, err := time.Parse("2006-01-02 15:04:05", record.CreatedAt); err == nil {
			createdAt = t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", record.UpdatedAt); err == nil {
			updatedAt = t
		}

		team := &services.Team{
			ID:          record.ID,
			Name:        record.Name,
			Description: record.Description,
			Members:     []services.TeamMember{},
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
			Settings:    settings,
		}
		tms.SetTeam(team)
		tr.teams[record.ID] = tms
	}

	slog.Info("team: loaded teams from store", "count", len(teams))
}

type TeamCreateTool struct{ BaseTool }

func (t *TeamCreateTool) Name() string { return "team_create" }
func (t *TeamCreateTool) Description() string {
	return "Create a new team workspace for memory sharing"
}

func (t *TeamCreateTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string", "description": "Team name"},
			"description": map[string]any{"type": "string", "description": "Team description"},
		},
		"required": []string{"name"},
	}
}

func (t *TeamCreateTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	name, _ := input["name"].(string)
	desc, _ := input["description"].(string)

	teamID := fmt.Sprintf("team_%d", time.Now().UnixNano())
	registry := GetTeamRegistry()
	tms := registry.GetOrCreate(teamID)

	team := &services.Team{
		ID:          teamID,
		Name:        name,
		Description: desc,
		Members:     []services.TeamMember{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Settings: services.TeamSettings{
			AutoSync:         false,
			SyncInterval:     300,
			MaxMemories:      1000,
			EnableEncryption: false,
			AllowPublicShare: true,
		},
	}
	tms.SetTeam(team)

	registry.PersistTeam(ctx, team)

	return map[string]any{
		"id":          teamID,
		"name":        name,
		"description": desc,
		"created":     true,
	}, nil
}

type TeamDeleteTool struct{ BaseTool }

func (t *TeamDeleteTool) Name() string        { return "team_delete" }
func (t *TeamDeleteTool) Description() string { return "Delete a team workspace" }

func (t *TeamDeleteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "Team ID to delete"},
		},
		"required": []string{"id"},
	}
}

func (t *TeamDeleteTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id, _ := input["id"].(string)
	registry := GetTeamRegistry()
	registry.Remove(id)
	return map[string]any{"id": id, "deleted": true}, nil
}

type TeamShareMemoryTool struct{ BaseTool }

func (t *TeamShareMemoryTool) Name() string        { return "team_share_memory" }
func (t *TeamShareMemoryTool) Description() string { return "Share a memory with the team" }

func (t *TeamShareMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"team_id":    map[string]any{"type": "string"},
			"title":      map[string]any{"type": "string"},
			"content":    map[string]any{"type": "string"},
			"type":       map[string]any{"type": "string", "description": "Memory type: code, conversation, decision, pattern, preference"},
			"visibility": map[string]any{"type": "string", "description": "Visibility: private, team, public"},
			"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required": []string{"team_id", "title", "content"},
	}
}

func (t *TeamShareMemoryTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	teamID, _ := input["team_id"].(string)
	title, _ := input["title"].(string)
	content, _ := input["content"].(string)
	memTypeStr, _ := input["type"].(string)
	visStr, _ := input["visibility"].(string)

	registry := GetTeamRegistry()
	tms, ok := registry.Get(teamID)
	if !ok {
		return nil, fmt.Errorf("team not found: %s", createTeamHint(teamID))
	}

	memType := parseMemoryType(memTypeStr)
	visibility := parseVisibility(visStr)

	var tags []string
	if raw, ok := input["tags"].([]any); ok {
		for _, t := range raw {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	memory := &services.Memory{
		Title:      title,
		Content:    content,
		Type:       memType,
		Visibility: visibility,
		Tags:       tags,
		UserID:     "current_user",
		Metadata:   make(map[string]any),
	}

	if err := tms.ShareMemory(ctx, memory); err != nil {
		return nil, err
	}

	registry.PersistMemory(ctx, teamID, memory)

	return map[string]any{
		"memory_id": memory.ID,
		"team_id":   teamID,
		"title":     title,
		"shared":    true,
	}, nil
}

type TeamGetMemoriesTool struct{ BaseTool }

func (t *TeamGetMemoriesTool) Name() string        { return "team_get_memories" }
func (t *TeamGetMemoriesTool) Description() string { return "Get memories from a team workspace" }

func (t *TeamGetMemoriesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"team_id": map[string]any{"type": "string"},
			"type":    map[string]any{"type": "string", "description": "Filter by memory type"},
			"tag":     map[string]any{"type": "string", "description": "Filter by tag"},
		},
		"required": []string{"team_id"},
	}
}

func (t *TeamGetMemoriesTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	teamID, _ := input["team_id"].(string)
	memTypeStr, _ := input["type"].(string)
	tag, _ := input["tag"].(string)

	registry := GetTeamRegistry()
	tms, ok := registry.Get(teamID)
	if !ok {
		return nil, fmt.Errorf("team not found: %s", createTeamHint(teamID))
	}

	var memories []*services.Memory
	var err error

	if tag != "" {
		memories, err = tms.GetMemoriesByTag(ctx, tag)
	} else if memTypeStr != "" {
		memories, err = tms.GetMemoriesByType(ctx, parseMemoryType(memTypeStr))
	} else {
		memories, err = tms.GetTeamMemories(ctx)
	}

	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(memories))
	for _, m := range memories {
		result = append(result, map[string]any{
			"id":         m.ID,
			"title":      m.Title,
			"type":       memoryTypeStr(m.Type),
			"visibility": visibilityStr(m.Visibility),
			"tags":       m.Tags,
			"created_at": m.CreatedAt,
			"version":    m.Version,
		})
	}

	return map[string]any{
		"memories": result,
		"count":    len(result),
		"team_id":  teamID,
	}, nil
}

type TeamSearchMemoriesTool struct{ BaseTool }

func (t *TeamSearchMemoriesTool) Name() string        { return "team_search_memories" }
func (t *TeamSearchMemoriesTool) Description() string { return "Search team memories by query" }

func (t *TeamSearchMemoriesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"team_id": map[string]any{"type": "string"},
			"query":   map[string]any{"type": "string"},
		},
		"required": []string{"team_id", "query"},
	}
}

func (t *TeamSearchMemoriesTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	teamID, _ := input["team_id"].(string)
	query, _ := input["query"].(string)

	registry := GetTeamRegistry()
	tms, ok := registry.Get(teamID)
	if !ok {
		return nil, fmt.Errorf("team not found: %s", createTeamHint(teamID))
	}

	memories, err := tms.SearchMemories(ctx, query)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(memories))
	for _, m := range memories {
		result = append(result, map[string]any{
			"id":    m.ID,
			"title": m.Title,
			"type":  memoryTypeStr(m.Type),
		})
	}

	return map[string]any{
		"results": result,
		"count":   len(result),
		"query":   query,
	}, nil
}

type TeamSyncTool struct{ BaseTool }

func (t *TeamSyncTool) Name() string        { return "team_sync" }
func (t *TeamSyncTool) Description() string { return "Sync team memories with remote server" }

func (t *TeamSyncTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"team_id":      map[string]any{"type": "string"},
			"api_endpoint": map[string]any{"type": "string"},
			"api_key":      map[string]any{"type": "string"},
			"encrypt_key":  map[string]any{"type": "string", "description": "Hex-encoded encryption key (32 bytes)"},
		},
		"required": []string{"team_id"},
	}
}

func (t *TeamSyncTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	teamID, _ := input["team_id"].(string)
	apiEndpoint, _ := input["api_endpoint"].(string)
	apiKey, _ := input["api_key"].(string)
	encryptKeyStr, _ := input["encrypt_key"].(string)

	registry := GetTeamRegistry()
	tms, ok := registry.Get(teamID)
	if !ok {
		return nil, fmt.Errorf("team not found: %s", createTeamHint(teamID))
	}

	if apiEndpoint != "" {
		var encryptKey []byte
		if encryptKeyStr != "" {
			var err error
			encryptKey, err = hex.DecodeString(encryptKeyStr)
			if err != nil {
				return nil, fmt.Errorf("invalid encrypt_key: must be hex-encoded, %w", err)
			}
			if len(encryptKey) != aes.BlockSize {
				return nil, fmt.Errorf("invalid encrypt_key: must be %d bytes (hex-encoded to %d chars)", aes.BlockSize, aes.BlockSize*2)
			}
		}
		tms.Configure(apiEndpoint, apiKey, encryptKey)
	}

	if err := tms.Sync(ctx); err != nil {
		return nil, fmt.Errorf("sync failed: %w", err)
	}

	return map[string]any{
		"team_id":   teamID,
		"synced":    true,
		"last_sync": tms.GetLastSyncTime(),
	}, nil
}

type TeamShareSessionTool struct{ BaseTool }

func (t *TeamShareSessionTool) Name() string { return "team_share_session" }
func (t *TeamShareSessionTool) Description() string {
	return "Share a conversation session with the team"
}

func (t *TeamShareSessionTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"team_id":    map[string]any{"type": "string"},
			"session_id": map[string]any{"type": "string"},
			"summary":    map[string]any{"type": "string", "description": "Brief summary of the session"},
		},
		"required": []string{"team_id", "session_id"},
	}
}

func (t *TeamShareSessionTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	teamID, _ := input["team_id"].(string)
	sessionID, _ := input["session_id"].(string)
	summary, _ := input["summary"].(string)

	registry := GetTeamRegistry()
	tms, ok := registry.Get(teamID)
	if !ok {
		return nil, fmt.Errorf("team not found: %s", createTeamHint(teamID))
	}

	if summary == "" {
		summary = fmt.Sprintf("Session %s shared on %s", sessionID, time.Now().Format("2006-01-02"))
	}

	memory := &services.Memory{
		Title:      fmt.Sprintf("Session: %s", sessionID),
		Content:    summary,
		Type:       services.MemoryTypeConversation,
		Visibility: services.VisibilityTeam,
		Tags:       []string{"session", "shared"},
		UserID:     "current_user",
		Metadata: map[string]any{
			"session_id": sessionID,
		},
	}

	if err := tms.ShareMemory(ctx, memory); err != nil {
		return nil, err
	}

	registry.PersistMemory(ctx, teamID, memory)

	return map[string]any{
		"memory_id":  memory.ID,
		"team_id":    teamID,
		"session_id": sessionID,
		"shared":     true,
	}, nil
}

func parseMemoryType(s string) services.MemoryType {
	switch strings.ToLower(s) {
	case "code":
		return services.MemoryTypeCode
	case "conversation":
		return services.MemoryTypeConversation
	case "decision":
		return services.MemoryTypeDecision
	case "pattern":
		return services.MemoryTypePattern
	case "preference":
		return services.MemoryTypePreference
	default:
		return services.MemoryTypeCode
	}
}

func memoryTypeStr(t services.MemoryType) string {
	switch t {
	case services.MemoryTypeCode:
		return "code"
	case services.MemoryTypeConversation:
		return "conversation"
	case services.MemoryTypeDecision:
		return "decision"
	case services.MemoryTypePattern:
		return "pattern"
	case services.MemoryTypePreference:
		return "preference"
	default:
		return "unknown"
	}
}

func parseVisibility(s string) services.MemoryVisibility {
	switch strings.ToLower(s) {
	case "private":
		return services.VisibilityPrivate
	case "team":
		return services.VisibilityTeam
	case "public":
		return services.VisibilityPublic
	default:
		return services.VisibilityTeam
	}
}

func visibilityStr(v services.MemoryVisibility) string {
	switch v {
	case services.VisibilityPrivate:
		return "private"
	case services.VisibilityTeam:
		return "team"
	case services.VisibilityPublic:
		return "public"
	default:
		return "unknown"
	}
}

func createTeamHint(teamID string) string {
	return fmt.Sprintf("%s (use team_create to create a team first)", teamID)
}
