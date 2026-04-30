package skills

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

// SkillMeta represents a skill's metadata in the marketplace registry.
type SkillMeta struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Version     string   `json:"version"`
	Tags        []string `json:"tags"`
	Category    string   `json:"category"`
	Triggers    []string `json:"triggers"`
	Tools       []string `json:"tools"`
	Downloads   int      `json:"downloads"`
	Rating      float64  `json:"rating"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Source      string   `json:"source"`
	Content     string   `json:"content,omitempty"`
}

// SkillSearchResult holds paginated skill search results.
type SkillSearchResult struct {
	Skills    []SkillMeta `json:"skills"`
	Total     int         `json:"total"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
}

// Registry manages the skill marketplace index — local skills on disk
// plus a SQLite-backed index for marketplace metadata caching.
type Registry struct {
	skillsDir string
	store     *store.Store
	index     map[string]*SkillMeta
	indexMu   sync.RWMutex
}

// NewRegistry creates a new skill registry backed by the given skills directory and store.
func NewRegistry(skillsDir string, st *store.Store) *Registry {
	return &Registry{
		skillsDir: skillsDir,
		store:     st,
		index:     make(map[string]*SkillMeta),
	}
}

// BuildIndex scans the skillsDir for .md files (and SKILL.md inside directories),
// reads YAML frontmatter + content, and populates the in-memory index.
// It also indexes bundled skills so they appear in marketplace search.
func (r *Registry) BuildIndex() error {
	r.indexMu.Lock()
	defer r.indexMu.Unlock()

	for name, bundled := range BundledSkillDefinitions {
		r.index[name] = &SkillMeta{
			Name:        bundled.Name,
			Description: bundled.Description,
			Author:      "smartclaw",
			Version:     "1.0",
			Tags:        bundled.Tags,
			Category:    inferCategory(bundled.Tags),
			Triggers:    bundled.Triggers,
			Tools:       bundled.Tools,
			Downloads:   0,
			Rating:      4.5,
			CreatedAt:   time.Now().Format(time.RFC3339),
			UpdatedAt:   time.Now().Format(time.RFC3339),
			Source:      "bundled",
		}
	}

	os.MkdirAll(r.skillsDir, 0755)
	entries, err := os.ReadDir(r.skillsDir)
	if err != nil {
		slog.Warn("registry: failed to read skills dir", "path", r.skillsDir, "error", err)
		return nil
	}

	for _, entry := range entries {
		var skillPath string
		if entry.IsDir() {
			skillPath = filepath.Join(r.skillsDir, entry.Name(), "SKILL.md")
		} else if strings.HasSuffix(entry.Name(), ".md") {
			skillPath = filepath.Join(r.skillsDir, entry.Name())
		} else {
			continue
		}

		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		name := inferSkillName(skillPath, entry)
		meta := r.parseSkillMeta(name, string(data), "local")
		r.index[name] = meta
	}

	if r.store != nil {
		r.loadDBIndex()
	}

	slog.Info("registry: built skill index", "count", len(r.index))
	return nil
}

// Search searches the registry index by query, category, and tag.
// Supports pagination via page and pageSize.
func (r *Registry) Search(query string, category, tag string, page, pageSize int) (*SkillSearchResult, error) {
	r.indexMu.RLock()
	defer r.indexMu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	q := strings.ToLower(query)
	var matches []SkillMeta

	for _, meta := range r.index {
		if category != "" && meta.Category != category {
			continue
		}
		if tag != "" && !containsTag(meta.Tags, tag) {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(meta.Name), q) &&
				!strings.Contains(strings.ToLower(meta.Description), q) &&
				!containsTagMatch(meta.Tags, q) {
				continue
			}
		}
		matches = append(matches, *meta)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Downloads != matches[j].Downloads {
			return matches[i].Downloads > matches[j].Downloads
		}
		return matches[i].Rating > matches[j].Rating
	})

	total := len(matches)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &SkillSearchResult{
		Skills:   matches[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Get returns skill metadata by name.
func (r *Registry) Get(name string) (*SkillMeta, error) {
	r.indexMu.RLock()
	defer r.indexMu.RUnlock()

	meta, ok := r.index[name]
	if !ok {
		return nil, fmt.Errorf("skill not found in registry: %s", name)
	}
	return meta, nil
}

func (r *Registry) Install(name, content string, meta SkillMeta) error {
	skillDir := filepath.Join(r.skillsDir, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("registry: failed to create skill dir: %w", err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("registry: failed to write skill file: %w", err)
	}

	meta.Source = "local"
	meta.Downloads = meta.Downloads + 1
	meta.UpdatedAt = time.Now().Format(time.RFC3339)

	r.indexMu.Lock()
	r.index[name] = &meta
	r.indexMu.Unlock()

	if r.store != nil {
		r.upsertDBIndex(&meta)
	}

	return nil
}

func (r *Registry) Uninstall(name string) error {
	skillDir := filepath.Join(r.skillsDir, name)
	if err := os.RemoveAll(skillDir); err != nil {
		skillPath := filepath.Join(r.skillsDir, name+".md")
		if err2 := os.Remove(skillPath); err2 != nil {
			return fmt.Errorf("registry: failed to remove skill: %w", err)
		}
	}

	r.indexMu.Lock()
	if meta, ok := r.index[name]; ok && meta.Source == "local" {
		delete(r.index, name)
	}
	r.indexMu.Unlock()

	return nil
}

func (r *Registry) Publish(meta SkillMeta) error {
	meta.Source = "marketplace"
	meta.UpdatedAt = time.Now().Format(time.RFC3339)
	if meta.CreatedAt == "" {
		meta.CreatedAt = time.Now().Format(time.RFC3339)
	}

	r.indexMu.Lock()
	r.index[meta.Name] = &meta
	r.indexMu.Unlock()

	if r.store != nil {
		r.upsertDBIndex(&meta)
	}

	return nil
}

func (r *Registry) ListInstalled() ([]SkillMeta, error) {
	r.indexMu.RLock()
	defer r.indexMu.RUnlock()

	var result []SkillMeta
	for _, meta := range r.index {
		if meta.Source == "local" || meta.Source == "bundled" {
			result = append(result, *meta)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func (r *Registry) parseSkillMeta(name, content, source string) *SkillMeta {
	schema, _, err := ParseSKILLFrontmatter(content)
	if err == nil && schema != nil {
		meta := &SkillMeta{
			Name:        coalesce(schema.Name, name),
			Description: schema.Description,
			Author:      schema.Author,
			Version:     schema.Version,
			Tags:        schema.Tags,
			Category:    inferCategory(schema.Tags),
			Triggers:    schema.Triggers,
			Tools:       schema.Tools,
			Downloads:   0,
			Rating:      0,
			CreatedAt:   time.Now().Format(time.RFC3339),
			UpdatedAt:   time.Now().Format(time.RFC3339),
			Source:      source,
		}
		if len(schema.SlashCommands) > 0 {
			for _, sc := range schema.SlashCommands {
				meta.Triggers = append(meta.Triggers, "/"+sc.Name)
			}
		}
	return meta
	}

	description := extractSection(content, "Description", "Triggers")
	if description == "" {
		description = extractDescription(content)
	}
	tools := extractList(content, "## Tools")
	triggers := extractCommands(content)
	tags := extractList(content, "## Tags")

	return &SkillMeta{
		Name:        name,
		Description: description,
		Author:      "",
		Version:     "1.0",
		Tags:        tags,
		Category:    inferCategory(tags),
		Triggers:    triggers,
		Tools:       tools,
		Downloads:   0,
		Rating:      0,
		CreatedAt:   time.Now().Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
		Source:      source,
	}
}

func (r *Registry) loadDBIndex() {
	rows, err := r.store.DB().Query(`
		SELECT name, description, author, version, tags, category,
		       downloads, rating, source, content, installed_at, updated_at
		FROM skill_registry_index
	`)
	if err != nil {
		slog.Warn("registry: failed to load DB index", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, description, author, version, tagsJSON, category, source string
		var content, installedAt, updatedAt sql.NullString
		var downloads int
		var rating float64

		if err := rows.Scan(&name, &description, &author, &version, &tagsJSON, &category,
			&downloads, &rating, &source, &content, &installedAt, &updatedAt); err != nil {
			continue
		}

		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			tags = nil
		}

		if existing, ok := r.index[name]; ok && (existing.Source == "bundled" || existing.Source == "local") {
			continue
		}

		meta := &SkillMeta{
			Name:        name,
			Description: description,
			Author:      author,
			Version:     version,
			Tags:        tags,
			Category:    category,
			Downloads:   downloads,
			Rating:      rating,
			Source:      source,
			UpdatedAt:   updatedAt.String,
		}
		if installedAt.Valid {
			meta.CreatedAt = installedAt.String
		}
		if content.Valid {
			meta.Content = content.String
		}
		r.index[name] = meta
	}
}

func (r *Registry) upsertDBIndex(meta *SkillMeta) {
	tagsJSON, _ := json.Marshal(meta.Tags)

	_, err := r.store.DB().Exec(`
		INSERT INTO skill_registry_index (name, description, author, version, tags, category,
		                                  downloads, rating, source, content, installed_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			author = excluded.author,
			version = excluded.version,
			tags = excluded.tags,
			category = excluded.category,
			downloads = excluded.downloads,
			rating = excluded.rating,
			source = excluded.source,
			content = excluded.content,
			updated_at = excluded.updated_at
	`, meta.Name, meta.Description, meta.Author, meta.Version, string(tagsJSON), meta.Category,
		meta.Downloads, meta.Rating, meta.Source, meta.Content,
		meta.CreatedAt, meta.UpdatedAt)
	if err != nil {
		slog.Warn("registry: failed to upsert DB index", "name", meta.Name, "error", err)
	}
}

func inferSkillName(skillPath string, entry os.DirEntry) string {
	if entry.IsDir() {
		return entry.Name()
	}
	return strings.TrimSuffix(entry.Name(), ".md")
}

func inferCategory(tags []string) string {
	tagCategoryMap := map[string]string{
		"code": "code-review", "review": "code-review", "quality": "code-review",
		"deployment": "deployment", "devops": "deployment", "release": "deployment", "ci-cd": "deployment",
		"debugging": "debugging", "troubleshooting": "debugging", "fixes": "debugging", "errors": "debugging",
		"security": "security", "audit": "security", "hardening": "security",
		"sre": "sre", "monitoring": "sre", "incident": "sre",
		"testing": "testing", "tdd": "testing", "automation": "testing",
		"refactoring": "code-style", "clean-code": "code-style", "simplification": "code-style",
		"api": "automation", "batch": "automation", "loop": "automation",
	}

	for _, t := range tags {
		if cat, ok := tagCategoryMap[strings.ToLower(t)]; ok {
			return cat
		}
	}
	return "automation"
}

func containsTag(tags []string, target string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, target) {
			return true
		}
	}
	return false
}

func containsTagMatch(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}
