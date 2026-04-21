// Package plans provides persistent storage for plans using Markdown files
// with YAML frontmatter stored in .smartclaw/plans/
package plans

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Plan statuses
const (
	StatusDraft     = "draft"
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusAbandoned = "abandoned"
)

// Plan represents a persisted plan.
type Plan struct {
	ID           string    `json:"id"            yaml:"id"`
	Title        string    `json:"title"         yaml:"title"`
	Content      string    `json:"content"       yaml:"-"`
	Status       string    `json:"status"        yaml:"status"`
	CreatedAt    time.Time `json:"created_at"    yaml:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"    yaml:"updated_at"`
	WorkspaceDir string    `json:"workspace_dir" yaml:"workspace_dir"`
	Tags         []string  `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// frontmatter represents the YAML frontmatter fields for serialization.
type frontmatter struct {
	ID           string    `yaml:"id"`
	Title        string    `yaml:"title"`
	Status       string    `yaml:"status"`
	CreatedAt    time.Time `yaml:"created_at"`
	UpdatedAt    time.Time `yaml:"updated_at"`
	WorkspaceDir string    `yaml:"workspace_dir"`
	Tags         []string  `yaml:"tags,omitempty"`
}

// PlanStore manages persistent plans on disk.
type PlanStore struct {
	plansDir string
	mu       sync.RWMutex
}

// NewPlanStore creates a new PlanStore, creating the plans directory if needed.
// baseDir is the project root (where .smartclaw/ lives).
func NewPlanStore(baseDir string) (*PlanStore, error) {
	plansDir := filepath.Join(baseDir, ".smartclaw", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		return nil, fmt.Errorf("create plans directory %s: %w", plansDir, err)
	}
	return &PlanStore{plansDir: plansDir}, nil
}

// generateID creates a timestamp-based plan ID.
func generateID(t time.Time) string {
	return fmt.Sprintf("%s-%06d", t.Format("20060102-150405"), t.Nanosecond()/1000)
}

// planPath returns the file path for a plan with the given ID.
func (ps *PlanStore) planPath(id string) string {
	return filepath.Join(ps.plansDir, id+".md")
}

// Create creates a new plan with an auto-generated ID and persists it.
func (ps *PlanStore) Create(title, content, workspaceDir string) (*Plan, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	now := time.Now().UTC()
	p := &Plan{
		ID:           generateID(now),
		Title:        title,
		Content:      content,
		Status:       StatusDraft,
		CreatedAt:    now,
		UpdatedAt:    now,
		WorkspaceDir: workspaceDir,
	}

	if err := ps.save(p); err != nil {
		return nil, fmt.Errorf("save plan %s: %w", p.ID, err)
	}
	return p, nil
}

// Get loads a plan by ID.
func (ps *PlanStore) Get(id string) (*Plan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return ps.load(id)
}

// List returns all plans sorted by creation time (newest first).
func (ps *PlanStore) List() ([]*Plan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return ps.listAll()
}

// ListByStatus returns plans filtered by status, sorted by creation time (newest first).
func (ps *PlanStore) ListByStatus(status string) ([]*Plan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	all, err := ps.listAll()
	if err != nil {
		return nil, err
	}

	var filtered []*Plan
	for _, p := range all {
		if p.Status == status {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// Update changes the content of a plan and updates its timestamp.
func (ps *PlanStore) Update(id string, content string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	p, err := ps.load(id)
	if err != nil {
		return fmt.Errorf("load plan %s: %w", id, err)
	}

	p.Content = content
	p.UpdatedAt = time.Now().UTC()

	if err := ps.save(p); err != nil {
		return fmt.Errorf("save plan %s: %w", id, err)
	}
	return nil
}

// SetStatus changes the status of a plan.
func (ps *PlanStore) SetStatus(id string, status string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	p, err := ps.load(id)
	if err != nil {
		return fmt.Errorf("load plan %s: %w", id, err)
	}

	p.Status = status
	p.UpdatedAt = time.Now().UTC()

	if err := ps.save(p); err != nil {
		return fmt.Errorf("save plan %s: %w", id, err)
	}
	return nil
}

// Delete removes a plan file.
func (ps *PlanStore) Delete(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	path := ps.planPath(id)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete plan %s: %w", id, err)
	}
	return nil
}

// Search performs a simple case-insensitive text search in titles and content.
func (ps *PlanStore) Search(query string) ([]*Plan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	all, err := ps.listAll()
	if err != nil {
		return nil, err
	}

	pattern := "(?i)" + regexp.QuoteMeta(query)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile search pattern: %w", err)
	}

	var results []*Plan
	for _, p := range all {
		if re.MatchString(p.Title) || re.MatchString(p.Content) {
			results = append(results, p)
		}
	}
	return results, nil
}

// save writes a plan to disk in Markdown+YAML frontmatter format.
func (ps *PlanStore) save(p *Plan) error {
	fm := frontmatter{
		ID:           p.ID,
		Title:        p.Title,
		Status:       p.Status,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
		WorkspaceDir: p.WorkspaceDir,
		Tags:         p.Tags,
	}

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return fmt.Errorf("marshal frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(string(fmBytes))
	sb.WriteString("---\n\n")
	sb.WriteString(p.Content)
	sb.WriteString("\n")

	path := ps.planPath(p.ID)
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

// load reads a plan from disk by ID.
func (ps *PlanStore) load(id string) (*Plan, error) {
	path := ps.planPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plan file %s: %w", path, err)
	}

	content := string(data)

	fmStart := strings.Index(content, "---\n")
	if fmStart != 0 {
		return nil, fmt.Errorf("plan %s: missing opening frontmatter delimiter", id)
	}

	fmEnd := strings.Index(content[4:], "---\n")
	if fmEnd < 0 {
		return nil, fmt.Errorf("plan %s: missing closing frontmatter delimiter", id)
	}

	fmText := content[4 : fmEnd+4]
	body := content[fmEnd+8:]

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return nil, fmt.Errorf("plan %s: parse frontmatter: %w", id, err)
	}

	return &Plan{
		ID:           fm.ID,
		Title:        fm.Title,
		Content:      strings.TrimSpace(body),
		Status:       fm.Status,
		CreatedAt:    fm.CreatedAt,
		UpdatedAt:    fm.UpdatedAt,
		WorkspaceDir: fm.WorkspaceDir,
		Tags:         fm.Tags,
	}, nil
}

// listAll reads all plan files from disk.
func (ps *PlanStore) listAll() ([]*Plan, error) {
	entries, err := os.ReadDir(ps.plansDir)
	if err != nil {
		return nil, fmt.Errorf("read plans directory: %w", err)
	}

	var plans []*Plan
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		p, err := ps.load(id)
		if err != nil {
			continue
		}
		plans = append(plans, p)
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})

	return plans, nil
}
