package layers

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const DecayThreshold = 0.3

type SkillSummary struct {
	Name        string
	Description string
	Tags        []string
	Triggers    []string
	Source      string
}

type SkillProceduralMemory struct {
	index     map[string]*SkillSummary
	fullCache map[string]string
	skillsDir string
	bundled   map[string]*SkillSummary
	mu        sync.RWMutex
	scores    map[string]float64
}

func NewSkillProceduralMemory(skillsDir string, bundledSummaries map[string]*SkillSummary) *SkillProceduralMemory {
	if skillsDir == "" {
		home, _ := os.UserHomeDir()
		skillsDir = filepath.Join(home, ".smartclaw", "skills")
	}

	if bundledSummaries == nil {
		bundledSummaries = make(map[string]*SkillSummary)
	}

	return &SkillProceduralMemory{
		index:     make(map[string]*SkillSummary),
		fullCache: make(map[string]string),
		skillsDir: skillsDir,
		bundled:   bundledSummaries,
		scores:    make(map[string]float64),
	}
}

func (spm *SkillProceduralMemory) UpdateScores(scores map[string]float64) {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	spm.scores = scores
}

func (spm *SkillProceduralMemory) LoadIndex() error {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	for name, summary := range spm.bundled {
		spm.index[name] = summary
	}

	if err := spm.loadLocalSkillsIndex(); err != nil {
		slog.Warn("skill memory: failed to load local skills index", "error", err)
	}

	slog.Info("skill memory: loaded index", "skills", len(spm.index))
	return nil
}

func (spm *SkillProceduralMemory) loadLocalSkillsIndex() error {
	entries, err := os.ReadDir(spm.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(spm.skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		summary := parseSkillSummary(string(data), entry.Name())
		summary.Source = "learned"
		spm.index[entry.Name()] = summary
	}

	return nil
}

func (spm *SkillProceduralMemory) GetFullSkill(name string) (string, error) {
	spm.mu.RLock()
	if cached, ok := spm.fullCache[name]; ok {
		spm.mu.RUnlock()
		return cached, nil
	}
	spm.mu.RUnlock()

	if summary, ok := spm.index[name]; ok && summary.Source == "bundled" {
		return fmt.Sprintf("# %s\n%s", summary.Name, summary.Description), nil
	}

	skillPath := filepath.Join(spm.skillsDir, name, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return "", fmt.Errorf("skill memory: load %q: %w", name, err)
	}

	content := string(data)

	spm.mu.Lock()
	spm.fullCache[name] = content
	spm.mu.Unlock()

	return content, nil
}

func (spm *SkillProceduralMemory) BuildSkillPrompt() string {
	spm.mu.RLock()
	defer spm.mu.RUnlock()

	if len(spm.index) == 0 {
		return ""
	}

	type skillEntry struct {
		name    string
		summary *SkillSummary
		score   float64
	}

	entries := make([]skillEntry, 0, len(spm.index))
	for name, summary := range spm.index {
		score := 0.5
		if s, ok := spm.scores[name]; ok {
			score = s
		}
		if score < DecayThreshold {
			continue
		}
		entries = append(entries, skillEntry{name: name, summary: summary, score: score})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score
	})

	var sb strings.Builder
	sb.WriteString("## Available Skills\n\n")

	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", entry.name, entry.summary.Description))
	}

	return sb.String()
}

func (spm *SkillProceduralMemory) GetIndex() map[string]*SkillSummary {
	spm.mu.RLock()
	defer spm.mu.RUnlock()

	result := make(map[string]*SkillSummary, len(spm.index))
	for k, v := range spm.index {
		result[k] = v
	}
	return result
}

func (spm *SkillProceduralMemory) RefreshSkill(name string) error {
	spm.mu.Lock()
	defer spm.mu.Unlock()

	delete(spm.fullCache, name)

	skillPath := filepath.Join(spm.skillsDir, name, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return fmt.Errorf("skill memory: refresh %q: %w", name, err)
	}

	summary := parseSkillSummary(string(data), name)
	summary.Source = "learned"
	spm.index[name] = summary

	return nil
}

func parseSkillSummary(content, fallbackName string) *SkillSummary {
	summary := &SkillSummary{
		Name:        fallbackName,
		Description: "",
		Tags:        []string{},
		Triggers:    []string{},
	}

	lines := strings.Split(content, "\n")
	inSection := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && summary.Description == "" {
			name := strings.TrimPrefix(trimmed, "# ")
			name = strings.TrimSuffix(name, " Skill")
			if name != "" {
				summary.Name = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
			}
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			inSection = strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			continue
		}

		if summary.Description == "" && trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "<!") && !strings.HasPrefix(trimmed, "<!--") {
			summary.Description = trimmed
			continue
		}

		if inSection == "tags" && strings.HasPrefix(trimmed, "- ") {
			summary.Tags = append(summary.Tags, strings.TrimPrefix(trimmed, "- "))
		}
		if inSection == "triggers" && strings.HasPrefix(trimmed, "- ") {
			summary.Triggers = append(summary.Triggers, strings.TrimPrefix(trimmed, "- "))
		}
	}

	return summary
}
