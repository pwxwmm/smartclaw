package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/store"
)

type MemorySnapshot struct {
	MemoryContent string
	UserContent   string
}

type MemoryManager struct {
	promptMemory  *layers.PromptMemory
	sessionSearch *layers.SessionSearch
	skillMemory   *layers.SkillProceduralMemory
	userModel     *layers.UserModelingLayer
	dataStore     *store.Store
	soulMD        *layers.ManagedFile
	agentsMD      *layers.ManagedFile
	baseDir       string

	snapshots map[string]*MemorySnapshot
	snapMu    sync.RWMutex

	budget ContextBudget
}

func NewMemoryManager() (*MemoryManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("memory manager: %w", err)
	}
	dir := filepath.Join(home, ".smartclaw")
	return NewMemoryManagerWithDir(dir)
}

func NewMemoryManagerWithDir(dir string) (*MemoryManager, error) {
	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		return nil, fmt.Errorf("memory manager: %w", err)
	}

	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		slog.Warn("memory manager: SQLite unavailable, falling back to JSONL persistence", "error", err)
	}

	soulMD := layers.NewManagedFile(filepath.Join(dir, "SOUL.md"))
	if _, err := os.Stat(soulMD.Path()); err == nil {
		if err := soulMD.Read(); err != nil {
			slog.Warn("memory manager: failed to read SOUL.md", "error", err)
		}
	}

	agentsMD := layers.NewManagedFile(filepath.Join(dir, "AGENTS.md"))
	if _, err := os.Stat(agentsMD.Path()); err == nil {
		if err := agentsMD.Read(); err != nil {
			slog.Warn("memory manager: failed to read AGENTS.md", "error", err)
		}
	}

	mm := &MemoryManager{
		promptMemory: pm,
		dataStore:    s,
		soulMD:       soulMD,
		agentsMD:     agentsMD,
		baseDir:      dir,
		snapshots:    make(map[string]*MemorySnapshot),
		budget:       DefaultContextBudget(),
	}

	bundledSummaries := buildBundledSkillSummaries()
	skillMem := layers.NewSkillProceduralMemory(filepath.Join(dir, "skills"), bundledSummaries)
	if err := skillMem.LoadIndex(); err != nil {
		slog.Warn("memory manager: failed to load skill memory index", "error", err)
	}
	mm.skillMemory = skillMem

	if s != nil {
		mm.sessionSearch = layers.NewSessionSearch(s)
		mm.userModel = layers.NewUserModelingLayer(s, pm)
	}

	return mm, nil
}

func NewMemoryManagerWithComponents(pm *layers.PromptMemory, s *store.Store, sm *layers.SkillProceduralMemory) *MemoryManager {
	mm := &MemoryManager{
		promptMemory: pm,
		dataStore:    s,
		skillMemory:  sm,
		snapshots:    make(map[string]*MemorySnapshot),
		budget:       DefaultContextBudget(),
	}

	if s != nil {
		mm.sessionSearch = layers.NewSessionSearch(s)
	}

	return mm
}

func (mm *MemoryManager) BuildSystemContext(ctx context.Context, currentQuery string) string {
	ctx, buildSpan := observability.StartSpan(ctx, "memory.build_context")
	defer observability.EndSpan(buildSpan)

	var layerContents []LayerContent

	if mm.soulMD != nil && mm.soulMD.Content() != "" {
		_, span := observability.StartSpan(ctx, "memory.layer.soul")
		layerContents = append(layerContents, LayerContent{Name: LayerSOUL, Content: mm.soulMD.Content()})
		observability.RecordMemoryLayerSize("soul", len(mm.soulMD.Content()))
		observability.EndSpan(span)
	}

	if mm.agentsMD != nil && mm.agentsMD.Content() != "" {
		_, span := observability.StartSpan(ctx, "memory.layer.agents")
		layerContents = append(layerContents, LayerContent{Name: LayerAgents, Content: mm.agentsMD.Content()})
		observability.RecordMemoryLayerSize("agents", len(mm.agentsMD.Content()))
		observability.EndSpan(span)
	}

	memoryContent := mm.promptMemory.GetMemoryContent()
	if memoryContent != "" {
		_, span := observability.StartSpan(ctx, "memory.layer.memory")
		layerContents = append(layerContents, LayerContent{Name: LayerMemory, Content: memoryContent})
		observability.RecordMemoryLayerSize("memory", len(memoryContent))
		observability.EndSpan(span)
	}

	userContent := mm.promptMemory.GetUserContent()
	if userContent != "" {
		_, span := observability.StartSpan(ctx, "memory.layer.user")
		layerContents = append(layerContents, LayerContent{Name: LayerUser, Content: userContent})
		observability.RecordMemoryLayerSize("user", len(userContent))
		observability.EndSpan(span)
	}

	if mm.userModel != nil {
		honchoBlock := mm.userModel.BuildStaticBlock()
		if honchoBlock != "" {
			_, span := observability.StartSpan(ctx, "memory.layer.user_model")
			layerContents = append(layerContents, LayerContent{Name: LayerUserModel, Content: honchoBlock})
			observability.RecordMemoryLayerSize("user_model", len(honchoBlock))
			observability.EndSpan(span)
		}
	}

	if mm.skillMemory != nil {
		_, span := observability.StartSpan(ctx, "memory.layer.skills")
		skillPrompt := mm.skillMemory.BuildSkillPrompt()
		if skillPrompt != "" {
			layerContents = append(layerContents, LayerContent{Name: LayerSkills, Content: skillPrompt})
			observability.RecordMemoryLayerSize("skills", len(skillPrompt))
		}
		observability.EndSpan(span)
	}

	if mm.sessionSearch != nil && currentQuery != "" {
		_, span := observability.StartSpan(ctx, "memory.layer.session_search")
		fragments, err := mm.sessionSearch.Search(ctx, currentQuery, 5)
		if err != nil {
			slog.Warn("memory manager: session search failed", "error", err)
		} else if len(fragments) > 0 {
			content := layers.FormatFragmentsStatic(fragments, 1000)
			layerContents = append(layerContents, LayerContent{Name: LayerSessionSearch, Content: content})
			observability.RecordMemoryLayerSize("session_search", len(content))
		}
		observability.EndSpan(span)
	}

	allocated := mm.budget.Allocate(layerContents)

	var parts []string
	truncatedCount := 0
	for _, a := range allocated {
		parts = append(parts, a.Content)
		if a.Truncated {
			truncatedCount++
		}
	}

	total := 0
	for _, p := range parts {
		total += len(p)
	}
	slog.Debug("memory manager: built system context", "chars", total, "layers", len(parts), "truncated", truncatedCount)

	if buildSpan != nil {
		buildSpan.SetAttribute("total_chars", total)
		buildSpan.SetAttribute("layers", len(parts))
		buildSpan.SetAttribute("truncated", truncatedCount)
	}

	return joinParts(parts)
}

func (mm *MemoryManager) FreezeSnapshot(sessionID string) {
	mm.snapMu.Lock()
	defer mm.snapMu.Unlock()

	if _, exists := mm.snapshots[sessionID]; exists {
		return
	}

	mm.snapshots[sessionID] = &MemorySnapshot{
		MemoryContent: mm.promptMemory.GetMemoryContent(),
		UserContent:   mm.promptMemory.GetUserContent(),
	}
	slog.Debug("memory manager: frozen snapshot for session", "session", sessionID)
}

func (mm *MemoryManager) ClearSnapshot(sessionID string) {
	mm.snapMu.Lock()
	defer mm.snapMu.Unlock()
	delete(mm.snapshots, sessionID)
}

func (mm *MemoryManager) GetSnapshot(sessionID string) *MemorySnapshot {
	mm.snapMu.RLock()
	defer mm.snapMu.RUnlock()
	return mm.snapshots[sessionID]
}

func (mm *MemoryManager) BuildSystemContextWithSnapshot(ctx context.Context, currentQuery, sessionID string) string {
	ctx, buildSpan := observability.StartSpan(ctx, "memory.build_context_snapshot")
	defer observability.EndSpan(buildSpan)

	var layerContents []LayerContent

	if mm.soulMD != nil && mm.soulMD.Content() != "" {
		layerContents = append(layerContents, LayerContent{Name: LayerSOUL, Content: mm.soulMD.Content()})
		observability.RecordMemoryLayerSize("soul", len(mm.soulMD.Content()))
	}

	if mm.agentsMD != nil && mm.agentsMD.Content() != "" {
		layerContents = append(layerContents, LayerContent{Name: LayerAgents, Content: mm.agentsMD.Content()})
		observability.RecordMemoryLayerSize("agents", len(mm.agentsMD.Content()))
	}

	mm.snapMu.RLock()
	snapshot, hasSnapshot := mm.snapshots[sessionID]
	mm.snapMu.RUnlock()

	if hasSnapshot {
		if snapshot.MemoryContent != "" {
			layerContents = append(layerContents, LayerContent{Name: LayerMemory, Content: snapshot.MemoryContent})
			observability.RecordMemoryLayerSize("memory", len(snapshot.MemoryContent))
		}
		if snapshot.UserContent != "" {
			layerContents = append(layerContents, LayerContent{Name: LayerUser, Content: snapshot.UserContent})
			observability.RecordMemoryLayerSize("user", len(snapshot.UserContent))
		}
	} else {
		memoryContent := mm.promptMemory.GetMemoryContent()
		if memoryContent != "" {
			layerContents = append(layerContents, LayerContent{Name: LayerMemory, Content: memoryContent})
			observability.RecordMemoryLayerSize("memory", len(memoryContent))
		}
		userContent := mm.promptMemory.GetUserContent()
		if userContent != "" {
			layerContents = append(layerContents, LayerContent{Name: LayerUser, Content: userContent})
			observability.RecordMemoryLayerSize("user", len(userContent))
		}
	}

	if mm.userModel != nil {
		honchoBlock := mm.userModel.BuildStaticBlock()
		if honchoBlock != "" {
			layerContents = append(layerContents, LayerContent{Name: LayerUserModel, Content: honchoBlock})
			observability.RecordMemoryLayerSize("user_model", len(honchoBlock))
		}
	}

	if mm.skillMemory != nil {
		skillPrompt := mm.skillMemory.BuildSkillPrompt()
		if skillPrompt != "" {
			layerContents = append(layerContents, LayerContent{Name: LayerSkills, Content: skillPrompt})
			observability.RecordMemoryLayerSize("skills", len(skillPrompt))
		}
	}

	if mm.sessionSearch != nil && currentQuery != "" {
		fragments, err := mm.sessionSearch.Search(ctx, currentQuery, 5)
		if err != nil {
			slog.Warn("memory manager: session search failed", "error", err)
		} else if len(fragments) > 0 {
			content := layers.FormatFragmentsStatic(fragments, 1000)
			layerContents = append(layerContents, LayerContent{Name: LayerSessionSearch, Content: content})
			observability.RecordMemoryLayerSize("session_search", len(content))
		}
	}

	allocated := mm.budget.Allocate(layerContents)

	var parts []string
	truncatedCount := 0
	for _, a := range allocated {
		parts = append(parts, a.Content)
		if a.Truncated {
			truncatedCount++
		}
	}

	total := 0
	for _, p := range parts {
		total += len(p)
	}
	slog.Debug("memory manager: built system context", "chars", total, "layers", len(parts), "frozen", hasSnapshot, "truncated", truncatedCount)

	if buildSpan != nil {
		buildSpan.SetAttribute("total_chars", total)
		buildSpan.SetAttribute("frozen", hasSnapshot)
	}

	return joinParts(parts)
}

func (mm *MemoryManager) Reload() error {
	if err := mm.promptMemory.Reload(); err != nil {
		return err
	}
	if mm.soulMD != nil {
		if err := mm.soulMD.Read(); err != nil && !os.IsNotExist(err) {
			slog.Warn("memory manager: failed to reload SOUL.md", "error", err)
		}
	}
	if mm.agentsMD != nil {
		if err := mm.agentsMD.Read(); err != nil && !os.IsNotExist(err) {
			slog.Warn("memory manager: failed to reload AGENTS.md", "error", err)
		}
	}
	return nil
}

func (mm *MemoryManager) GetPromptMemory() *layers.PromptMemory {
	return mm.promptMemory
}

func (mm *MemoryManager) GetSessionSearch() *layers.SessionSearch {
	return mm.sessionSearch
}

func (mm *MemoryManager) GetSkillMemory() *layers.SkillProceduralMemory {
	return mm.skillMemory
}

func (mm *MemoryManager) GetStore() *store.Store {
	return mm.dataStore
}

func (mm *MemoryManager) GetUserModel() *layers.UserModelingLayer {
	return mm.userModel
}

func (mm *MemoryManager) GetSoulMD() *layers.ManagedFile {
	return mm.soulMD
}

func (mm *MemoryManager) GetAgentsMD() *layers.ManagedFile {
	return mm.agentsMD
}

func (mm *MemoryManager) SetBudget(budget ContextBudget) {
	mm.budget = budget
}

func (mm *MemoryManager) GetBudget() ContextBudget {
	return mm.budget
}

func (mm *MemoryManager) Close() error {
	if mm.dataStore != nil {
		return mm.dataStore.Close()
	}
	return nil
}

func joinParts(parts []string) string {
	result := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if result != "" {
			result += "\n\n"
		}
		result += p
	}
	return result
}

func buildBundledSkillSummaries() map[string]*layers.SkillSummary {
	defs := map[string]struct {
		Name        string
		Description string
		Tags        []string
		Triggers    []string
	}{
		"code-review":    {Name: "code-review", Description: "Review code changes with best practices and suggestions", Tags: []string{"code", "review", "quality"}, Triggers: []string{"/review", "/code-review"}},
		"git-expert":     {Name: "git-expert", Description: "Advanced git operations and workflow guidance", Tags: []string{"git", "version-control", "workflow"}, Triggers: []string{"/git", "/git-*"}},
		"test-generator": {Name: "test-generator", Description: "Generate comprehensive test suites", Tags: []string{"testing", "quality", "automation"}, Triggers: []string{"/test", "/generate-tests"}},
		"documentation":  {Name: "documentation", Description: "Generate and maintain documentation", Tags: []string{"docs", "communication", "clarity"}, Triggers: []string{"/doc", "/document", "/readme"}},
		"refactoring":    {Name: "refactoring", Description: "Safe code refactoring with tests", Tags: []string{"refactoring", "quality", "clean-code"}, Triggers: []string{"/refactor", "/restructure"}},
		"debugger":       {Name: "debugger", Description: "Debug and fix code issues", Tags: []string{"debugging", "troubleshooting", "fixes"}, Triggers: []string{"/debug", "/fix", "/troubleshoot"}},
		"api-designer":   {Name: "api-designer", Description: "Design and implement APIs", Tags: []string{"api", "design", "rest", "graphql"}, Triggers: []string{"/api", "/design-api", "/endpoint"}},
		"performance":    {Name: "performance", Description: "Analyze and optimize performance", Tags: []string{"performance", "optimization", "profiling"}, Triggers: []string{"/perf", "/optimize", "/profile"}},
		"security":       {Name: "security", Description: "Security analysis and hardening", Tags: []string{"security", "audit", "hardening"}, Triggers: []string{"/security", "/audit", "/hardening"}},
		"deployment":     {Name: "deployment", Description: "Deploy and configure applications", Tags: []string{"deployment", "devops", "release"}, Triggers: []string{"/deploy", "/release", "/ship"}},
		"batch":          {Name: "batch", Description: "Execute operations in batch mode", Tags: []string{"batch", "automation", "bulk"}, Triggers: []string{"/batch", "/bulk"}},
		"loop":           {Name: "loop", Description: "Create persistent execution loops", Tags: []string{"loop", "automation", "watcher"}, Triggers: []string{"/loop", "/repeat", "/watch"}},
		"remember":       {Name: "remember", Description: "Store and recall information across sessions", Tags: []string{"memory", "persistence", "context"}, Triggers: []string{"/remember", "/recall", "/memory"}},
		"verify":         {Name: "verify", Description: "Verify changes meet requirements", Tags: []string{"verification", "validation", "quality"}, Triggers: []string{"/verify", "/validate", "/check"}},
		"skillify":       {Name: "skillify", Description: "Convert code patterns into reusable skills", Tags: []string{"skills", "patterns", "reusability"}, Triggers: []string{"/skillify", "/make-skill"}},
		"simplify":       {Name: "simplify", Description: "Simplify complex code and logic", Tags: []string{"simplification", "refactoring", "clean-code"}, Triggers: []string{"/simplify", "/clean", "/reduce"}},
		"stuck":          {Name: "stuck", Description: "Help when stuck on a problem", Tags: []string{"help", "troubleshooting", "problem-solving"}, Triggers: []string{"/stuck", "/help", "/unblock"}},
		"claude-api":     {Name: "claude-api", Description: "Work with Claude API directly", Tags: []string{"api", "claude", "integration"}, Triggers: []string{"/claude-api", "/api-call"}},
		"keybindings":    {Name: "keybindings", Description: "Manage and configure keybindings", Tags: []string{"keybindings", "configuration", "shortcuts"}, Triggers: []string{"/keybindings", "/keys", "/shortcuts"}},
		"update-config":  {Name: "update-config", Description: "Update and manage configuration", Tags: []string{"configuration", "settings", "management"}, Triggers: []string{"/update-config", "/config", "/settings"}},
	}

	summaries := make(map[string]*layers.SkillSummary, len(defs))
	for name, def := range defs {
		summaries[name] = &layers.SkillSummary{
			Name:        def.Name,
			Description: def.Description,
			Tags:        def.Tags,
			Triggers:    def.Triggers,
			Source:      "bundled",
		}
	}
	return summaries
}
