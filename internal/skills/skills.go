package skills

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/memory/layers"
)

type Skill struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Path        string                 `json:"path"`
	Content     string                 `json:"content"`
	Commands    []string               `json:"commands"`
	Tools       []string               `json:"tools,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Source      string                 `json:"source"` // "bundled", "local", "mcp"
	Metadata    map[string]any `json:"metadata,omitempty"`
	LoadedAt    time.Time              `json:"loaded_at"`
	Schema      *SkillSchema           `json:"schema,omitempty"`
}

func (s *Skill) PlatformAllowed(platform string) bool {
	if s.Schema == nil {
		return true
	}
	return IsPlatformAllowed(s.Schema, platform)
}

func (s *Skill) ResolveConfig(provided map[string]any) (map[string]any, []error) {
	if s.Schema == nil {
		return provided, nil
	}
	return ResolveConfigVars(s.Schema, provided)
}

type McpSkillBuilder struct {
	ServerName string `json:"server_name"`
	ToolName   string `json:"tool_name"`
	Template   string `json:"template"`
}

type SkillManager struct {
	skills            map[string]*Skill
	bundledSkills     map[string]*Skill
	mcpBuilders       []McpSkillBuilder
	mu                sync.RWMutex
	skillsDir         string
	watcher           *SkillWatcher
	memoryIntegration *MemoryIntegration
	skillMemory       *layers.SkillProceduralMemory
	lazyLoad          bool
}

type SkillWatcher struct {
	manager  *SkillManager
	interval time.Duration
	stopChan chan struct{}
	running  bool
	mu       sync.Mutex
}

func NewSkillManager() *SkillManager {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".smartclaw", "skills")

	sm := &SkillManager{
		skills:        make(map[string]*Skill),
		bundledSkills: make(map[string]*Skill),
		mcpBuilders:   make([]McpSkillBuilder, 0),
		skillsDir:     skillsDir,
	}

	sm.loadBundledSkills()
	sm.loadLocalSkills()

	return sm
}

func NewSkillManagerLazy() *SkillManager {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".smartclaw", "skills")

	sm := &SkillManager{
		skills:        make(map[string]*Skill),
		bundledSkills: make(map[string]*Skill),
		mcpBuilders:   make([]McpSkillBuilder, 0),
		skillsDir:     skillsDir,
		lazyLoad:      true,
	}

	bundledSummaries := sm.buildBundledSummaries()
	sm.skillMemory = layers.NewSkillProceduralMemory(skillsDir, bundledSummaries)
	if err := sm.skillMemory.LoadIndex(); err != nil {
		slog.Warn("skill manager: failed to load skill memory index", "error", err)
	}

	sm.loadBundledSkillSummaries()
	sm.loadLocalSkillSummaries()

	return sm
}

func (sm *SkillManager) buildBundledSummaries() map[string]*layers.SkillSummary {
	summaries := make(map[string]*layers.SkillSummary, len(BundledSkillDefinitions))
	for name, bundled := range BundledSkillDefinitions {
		summaries[name] = &layers.SkillSummary{
			Name:        bundled.Name,
			Description: bundled.Description,
			Tags:        bundled.Tags,
			Triggers:    bundled.Triggers,
			Source:      "bundled",
		}
	}
	return summaries
}

func (sm *SkillManager) loadBundledSkillSummaries() {
	for name, bundledSkill := range BundledSkillDefinitions {
		skill := &Skill{
			Name:        bundledSkill.Name,
			Description: bundledSkill.Description,
			Commands:    bundledSkill.Triggers,
			Tools:       bundledSkill.Tools,
			Tags:        bundledSkill.Tags,
			Source:      "bundled",
			Enabled:     true,
			LoadedAt:    time.Now(),
			Metadata:    make(map[string]any),
		}
		sm.bundledSkills[name] = skill
		sm.skills[name] = skill
	}
}

func (sm *SkillManager) loadLocalSkillSummaries() {
	os.MkdirAll(sm.skillsDir, 0755)

	entries, err := os.ReadDir(sm.skillsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(sm.skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		name := entry.Name()
		description := extractDescription(string(data))
		commands := extractCommands(string(data))
		tags := extractList(string(data), "## Tags")

		skill := &Skill{
			Name:        name,
			Description: description,
			Commands:    commands,
			Tags:        tags,
			Source:      "local",
			Enabled:     true,
			LoadedAt:    time.Now(),
			Metadata:    make(map[string]any),
		}
		sm.skills[name] = skill
	}
}

func (sm *SkillManager) loadBundledSkills() {
	for name, bundledSkill := range BundledSkillDefinitions {
		skill := &Skill{
			Name:        bundledSkill.Name,
			Description: bundledSkill.Description,
			Content:     bundledSkill.Content,
			Tools:       bundledSkill.Tools,
			Commands:    bundledSkill.Triggers,
			Tags:        bundledSkill.Tags,
			Source:      "bundled",
			Enabled:     true,
			LoadedAt:    time.Now(),
			Metadata:    make(map[string]any),
		}
		sm.bundledSkills[name] = skill
		sm.skills[name] = skill
	}
}

func (sm *SkillManager) loadLocalSkills() {
	os.MkdirAll(sm.skillsDir, 0755)

	entries, err := os.ReadDir(sm.skillsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		var skillPath string
		if entry.IsDir() {
			skillPath = filepath.Join(sm.skillsDir, entry.Name())
		} else if strings.HasSuffix(entry.Name(), ".md") {
			skillPath = filepath.Join(sm.skillsDir, entry.Name())
		} else {
			continue
		}

		if _, err := sm.Load(skillPath); err != nil {
			continue
		}
	}
}

func (sm *SkillManager) Load(path string) (*Skill, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var skillPath string
	if stat.IsDir() {
		skillPath = filepath.Join(path, "SKILL.md")
	} else {
		skillPath = path
	}

	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	name := filepath.Base(filepath.Dir(skillPath))
	if name == "." || name == "" {
		name = strings.TrimSuffix(filepath.Base(skillPath), ".md")
	}

	skill := sm.parseSkillContent(name, string(content), "local")
	skill.Path = skillPath

	sm.mu.Lock()
	sm.skills[name] = skill
	sm.mu.Unlock()

	return skill, nil
}

func (sm *SkillManager) parseSkillContent(name, content, source string) *Skill {
	schema, _, err := ParseSKILLFrontmatter(content)
	if err == nil && schema != nil {
		commands := schema.Triggers
		if len(schema.SlashCommands) > 0 {
			for _, sc := range schema.SlashCommands {
				commands = append(commands, sc.Name)
			}
		}

		return &Skill{
			Name:        coalesce(schema.Name, name),
			Description: schema.Description,
			Content:     content,
			Tools:       schema.Tools,
			Commands:    commands,
			Tags:        schema.Tags,
			Author:      schema.Author,
			Version:     schema.Version,
			Source:      source,
			Enabled:     true,
			LoadedAt:    time.Now(),
			Metadata:    make(map[string]any),
			Schema:      schema,
		}
	}

	// Fallback: legacy plain-markdown parsing
	description := extractSection(content, "Description", "Triggers")
	if description == "" {
		description = extractDescription(content)
	}

	tools := extractList(content, "## Tools")
	commands := extractCommands(content)
	tags := extractList(content, "## Tags")

	return &Skill{
		Name:        name,
		Description: description,
		Content:     content,
		Tools:       tools,
		Commands:    commands,
		Tags:        tags,
		Source:      source,
		Enabled:     true,
		LoadedAt:    time.Now(),
		Metadata:    make(map[string]any),
	}
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func (sm *SkillManager) Get(name string) *Skill {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.skills[name]
}

func (sm *SkillManager) List() []*Skill {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Skill, 0, len(sm.skills))
	for _, s := range sm.skills {
		result = append(result, s)
	}
	return result
}

func (sm *SkillManager) ListBundled() []*Skill {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Skill, 0, len(sm.bundledSkills))
	for _, s := range sm.bundledSkills {
		result = append(result, s)
	}
	return result
}

func (sm *SkillManager) ListByTag(tag string) []*Skill {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Skill, 0)
	for _, skill := range sm.skills {
		for _, t := range skill.Tags {
			if t == tag {
				result = append(result, skill)
				break
			}
		}
	}
	return result
}

func (sm *SkillManager) Enable(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	skill, ok := sm.skills[name]
	if !ok {
		return fmt.Errorf("skill not found: %s", name)
	}

	skill.Enabled = true
	return nil
}

func (sm *SkillManager) Disable(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	skill, ok := sm.skills[name]
	if !ok {
		return fmt.Errorf("skill not found: %s", name)
	}

	skill.Enabled = false
	return nil
}

func (sm *SkillManager) Reload(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	skill, ok := sm.skills[name]
	if !ok {
		return fmt.Errorf("skill not found: %s", name)
	}

	if skill.Source == "bundled" {
		return fmt.Errorf("cannot reload bundled skill: %s", name)
	}

	if skill.Path == "" {
		return fmt.Errorf("skill has no file path: %s", name)
	}

	content, err := os.ReadFile(skill.Path)
	if err != nil {
		return fmt.Errorf("failed to read skill file: %w", err)
	}

	updated := sm.parseSkillContent(name, string(content), skill.Source)
	updated.Path = skill.Path
	sm.skills[name] = updated

	return nil
}

func (sm *SkillManager) GetContent(name string) (string, error) {
	if sm.lazyLoad && sm.skillMemory != nil {
		return sm.skillMemory.GetFullSkill(name)
	}

	skill := sm.Get(name)
	if skill == nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return skill.Content, nil
}

func (sm *SkillManager) AddMcpSkillBuilder(builder McpSkillBuilder) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.mcpBuilders = append(sm.mcpBuilders, builder)
}

func (sm *SkillManager) GetMcpSkillBuilders() []McpSkillBuilder {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return append([]McpSkillBuilder{}, sm.mcpBuilders...)
}

func (sm *SkillManager) StartWatcher(interval time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.watcher != nil && sm.watcher.running {
		return
	}

	sm.watcher = &SkillWatcher{
		manager:  sm,
		interval: interval,
		stopChan: make(chan struct{}),
		running:  true,
	}

	go sm.watcher.watch()
}

func (sm *SkillManager) StopWatcher() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.watcher != nil && sm.watcher.running {
		close(sm.watcher.stopChan)
		sm.watcher.running = false
	}
}

func (w *SkillWatcher) watch() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.manager.loadLocalSkills()
		}
	}
}

func (sm *SkillManager) Search(query string) []*Skill {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]*Skill, 0)

	for _, skill := range sm.skills {
		if strings.Contains(strings.ToLower(skill.Name), query) ||
			strings.Contains(strings.ToLower(skill.Description), query) {
			results = append(results, skill)
			continue
		}

		for _, tag := range skill.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, skill)
				break
			}
		}
	}

	return results
}

func extractSection(content, startMarker, endMarker string) string {
	lines := strings.Split(content, "\n")
	var collecting bool
	var section []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, startMarker) {
			collecting = true
			continue
		}

		if collecting && endMarker != "" && strings.Contains(trimmed, endMarker) {
			break
		}

		if collecting && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			section = append(section, trimmed)
		}
	}

	return strings.Join(section, " ")
}

func extractList(content, marker string) []string {
	lines := strings.Split(content, "\n")
	var items []string
	var collecting bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, marker) {
			collecting = true
			continue
		}

		if collecting {
			if strings.HasPrefix(trimmed, "##") {
				break
			}
			if strings.HasPrefix(trimmed, "- ") {
				item := strings.TrimPrefix(trimmed, "- ")
				items = append(items, strings.TrimSpace(item))
			}
		}
	}

	return items
}

func extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "##") {
			continue
		}
		if len(line) > 100 {
			return line[:100] + "..."
		}
		return line
	}
	return ""
}

func extractCommands(content string) []string {
	var commands []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- `/") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				cmd := strings.Trim(parts[1], "`")
				commands = append(commands, cmd)
			}
		}
	}
	return commands
}

func (sm *SkillManager) SetMemoryIntegration(mi *MemoryIntegration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.memoryIntegration = mi
}

func (sm *SkillManager) GetMemoryIntegration() *MemoryIntegration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.memoryIntegration
}

func (sm *SkillManager) SetSkillMemory(spm *layers.SkillProceduralMemory) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.skillMemory = spm
}

func (sm *SkillManager) GetSkillMemory() *layers.SkillProceduralMemory {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.skillMemory
}

func (sm *SkillManager) GetContextAwareSkills(ctx context.Context, sessionID string) []*Skill {
	sm.mu.RLock()
	skills := make([]*Skill, 0, len(sm.skills))
	for _, s := range sm.skills {
		skills = append(skills, s)
	}
	skillMem := sm.skillMemory
	sm.mu.RUnlock()

	if sm.memoryIntegration == nil && skillMem == nil {
		return skills
	}

	if sm.memoryIntegration != nil {
		return sm.memoryIntegration.GetRelevantSkills(ctx, sessionID, skills)
	}

	return skills
}

func (sm *SkillManager) GetSkillSummaries() []SkillSummary {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	summaries := make([]SkillSummary, 0, len(sm.skills))
	for _, s := range sm.skills {
		if !s.Enabled {
			continue
		}
		summaries = append(summaries, SkillSummary{
			Name:        s.Name,
			Description: s.Description,
			Source:      s.Source,
			Triggers:    s.Commands,
			Tags:        s.Tags,
		})
	}
	return summaries
}

func (sm *SkillManager) LoadSkillOnDemand(name string) (string, error) {
	if sm.lazyLoad && sm.skillMemory != nil {
		return sm.skillMemory.GetFullSkill(name)
	}

	sm.mu.RLock()
	skill, ok := sm.skills[name]
	sm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	if skill.Content != "" {
		return skill.Content, nil
	}

	if skill.Path != "" {
		data, err := os.ReadFile(skill.Path)
		if err != nil {
			return "", fmt.Errorf("failed to read skill file: %w", err)
		}
		return string(data), nil
	}

	return "", fmt.Errorf("skill has no content or path: %s", name)
}

type SkillSummary struct {
	Name        string
	Description string
	Source      string
	Triggers    []string
	Tags        []string
}
