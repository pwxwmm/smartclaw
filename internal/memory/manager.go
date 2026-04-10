package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

type MemoryManager struct {
	promptMemory  *layers.PromptMemory
	sessionSearch *layers.SessionSearch
	skillMemory   *layers.SkillProceduralMemory
	userModel     *layers.UserModelingLayer
	dataStore     *store.Store
	soulMD        *layers.ManagedFile
	agentsMD      *layers.ManagedFile
	baseDir       string
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
		slog.Warn("memory manager: SQLite unavailable, session search disabled", "error", err)
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
	var parts []string

	if mm.soulMD != nil && mm.soulMD.Content() != "" {
		parts = append(parts, mm.soulMD.Content())
	}

	if mm.agentsMD != nil && mm.agentsMD.Content() != "" {
		parts = append(parts, mm.agentsMD.Content())
	}

	promptCtx := mm.promptMemory.AutoLoad()
	if promptCtx != "" {
		parts = append(parts, promptCtx)
	}

	if mm.sessionSearch != nil && currentQuery != "" {
		fragments, err := mm.sessionSearch.Search(ctx, currentQuery, 5)
		if err != nil {
			slog.Warn("memory manager: session search failed", "error", err)
		} else if len(fragments) > 0 {
			parts = append(parts, layers.FormatFragmentsStatic(fragments, 1000))
		}
	}

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
