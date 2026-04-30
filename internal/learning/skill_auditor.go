package learning

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

type SkillAuditor struct {
	store          *store.Store
	skillsDir      string
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

type AuditConfig struct {
	StaleThreshold time.Duration
	MinUseCount    int
}

func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		StaleThreshold: 30 * 24 * time.Hour,
		MinUseCount:    3,
	}
}

func NewSkillAuditor(s *store.Store, skillsDir string) *SkillAuditor {
	if skillsDir == "" {
		home, _ := os.UserHomeDir()
		skillsDir = filepath.Join(home, ".smartclaw", "skills")
	}
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	return &SkillAuditor{
		store:          s,
		skillsDir:      skillsDir,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}
}

func (sa *SkillAuditor) Close() {
	if sa.shutdownCancel != nil {
		sa.shutdownCancel()
	}
}

type AuditResult struct {
	Evicted []string
	Kept    []string
}

func (sa *SkillAuditor) AuditStaleSkills(cfg AuditConfig) (*AuditResult, error) {
	if sa.store == nil {
		slog.Debug("skill auditor: no store, skipping audit")
		return &AuditResult{}, nil
	}

	result := &AuditResult{}

	stale, err := sa.store.GetStaleSkills(cfg.StaleThreshold)
	if err != nil {
		return nil, fmt.Errorf("skill auditor: get stale: %w", err)
	}

	if len(stale) == 0 {
		return result, nil
	}

	slog.Info("skill auditor: found stale skills", "count", len(stale))

	for _, skill := range stale {
		if skill.UseCount >= cfg.MinUseCount {
			result.Kept = append(result.Kept, skill.Name)
			continue
		}

		if err := sa.evictSkill(skill.Name); err != nil {
			slog.Warn("skill auditor: failed to evict skill", "name", skill.Name, "error", err)
			result.Kept = append(result.Kept, skill.Name)
			continue
		}

		result.Evicted = append(result.Evicted, skill.Name)
		slog.Info("skill auditor: evicted stale skill", "name", skill.Name, "use_count", skill.UseCount)
	}

	return result, nil
}

func (sa *SkillAuditor) evictSkill(name string) error {
	if err := sa.store.DeleteSkill(name); err != nil {
		return fmt.Errorf("delete from db: %w", err)
	}

	skillDir := filepath.Join(sa.skillsDir, name)
	if _, err := os.Stat(skillDir); err == nil {
		backupDir := filepath.Join(sa.skillsDir, ".evicted")
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("mkdir evicted: %w", err)
		}

		backupPath := filepath.Join(backupDir, name+".bak")
		if err := os.Rename(skillDir, backupPath); err != nil {
			slog.Warn("skill auditor: failed to move evicted skill to backup", "name", name, "error", err)
			if err := os.RemoveAll(skillDir); err != nil {
				return fmt.Errorf("remove skill dir: %w", err)
			}
		}
	}

	return nil
}

func (sa *SkillAuditor) RecordSkillUse(name string) {
	if sa.store == nil {
		return
	}
	if err := sa.store.IncrementSkillUseCount(sa.shutdownCtx, name); err != nil {
		slog.Warn("skill auditor: failed to record skill use", "name", name, "error", err)
	}
}

func (sa *SkillAuditor) SyncLearnedSkillsToStore() error {
	if sa.store == nil {
		return nil
	}

	entries, err := os.ReadDir(sa.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("skill auditor: read skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		skillPath := filepath.Join(sa.skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		record := &store.SkillRecord{
			Name:     entry.Name(),
			Content:  string(data),
			Source:   "learned",
			UseCount: 0,
		}

		if err := sa.store.UpsertSkill(sa.shutdownCtx, record); err != nil {
			slog.Warn("skill auditor: failed to sync skill to store", "name", entry.Name(), "error", err)
		}
	}

	return nil
}
