package memory

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

type MemoryManager struct {
	promptMemory  *layers.PromptMemory
	sessionSearch *layers.SessionSearch
	skillMemory   *layers.SkillProceduralMemory
	userModel     *layers.UserModelingLayer
	dataStore     *store.Store
}

func NewMemoryManager() (*MemoryManager, error) {
	pm, err := layers.NewPromptMemory()
	if err != nil {
		return nil, fmt.Errorf("memory manager: %w", err)
	}

	s, err := store.NewStore()
	if err != nil {
		slog.Warn("memory manager: SQLite unavailable, session search disabled", "error", err)
	}

	return &MemoryManager{
		promptMemory: pm,
		dataStore:    s,
	}, nil
}

func NewMemoryManagerWithDir(dir string) (*MemoryManager, error) {
	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		return nil, fmt.Errorf("memory manager: %w", err)
	}

	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		slog.Warn("memory manager: SQLite unavailable, session search disabled", "error", err)
	}

	mm := &MemoryManager{
		promptMemory: pm,
		dataStore:    s,
	}

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
	}

	if s != nil {
		mm.sessionSearch = layers.NewSessionSearch(s)
	}

	return mm
}

func (mm *MemoryManager) BuildSystemContext(ctx context.Context, currentQuery string) string {
	// Hermes prompt assembly order: MEMORY.md → USER.md → Session Search → Skills index
	var parts []string

	parts = append(parts, mm.promptMemory.AutoLoad())

	if mm.sessionSearch != nil && currentQuery != "" {
		fragments, err := mm.sessionSearch.Search(ctx, currentQuery, 5)
		if err != nil {
			slog.Warn("memory manager: session search failed", "error", err)
		} else if len(fragments) > 0 {
			parts = append(parts, layers.FormatFragmentsStatic(fragments, 1000))
		}
	}

	// Layer 3: Skills index (only names + descriptions, full content loaded on demand)
	if mm.skillMemory != nil {
		skillPrompt := mm.skillMemory.BuildSkillPrompt()
		if skillPrompt != "" {
			parts = append(parts, skillPrompt)
		}
	}

	total := 0
	for _, p := range parts {
		total += len(p)
	}
	slog.Debug("memory manager: built system context", "chars", total, "layers", len(parts))

	return joinParts(parts)
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
