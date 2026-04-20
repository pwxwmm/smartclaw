package convention

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Convention represents a learned codebase convention or pattern.
type Convention struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`    // "naming", "pattern", "gotcha", "style", "architecture", "testing"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Examples    []string  `json:"examples,omitempty"`
	Citation    *Citation `json:"citation,omitempty"`
	Confidence  float64   `json:"confidence"` // 0.0-1.0
	Source      string    `json:"source"`     // "user", "agent", "observation"
	AgentID     string    `json:"agent_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	UseCount    int       `json:"use_count"`
}

// Citation references a specific code location that supports the convention.
type Citation struct {
	File        string `json:"file"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	ContentHash string `json:"content_hash"` // SHA256 of the cited lines
}

// Filter for listing conventions.
type Filter struct {
	Category      string
	Source        string
	AgentID       string
	MinConfidence float64
}

// sharing tracks which agents a convention is shared with.
type sharing struct {
	ConventionID string   `json:"convention_id"`
	TargetAgents []string `json:"target_agents"`
}

// Store manages conventions with citation verification.
type Store struct {
	dir     string
	convs   map[string]*Convention
	shares  map[string]*sharing // conventionID -> sharing info
	mu      sync.RWMutex
}

// NewStore creates a new convention store backed by the given directory.
func NewStore(dir string) *Store {
	s := &Store{
		dir:    dir,
		convs:  make(map[string]*Convention),
		shares: make(map[string]*sharing),
	}
	_ = s.loadAll()
	return s
}

// loadAll reads all stored conventions from disk.
func (s *Store) loadAll() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var conv Convention
		if err := json.Unmarshal(data, &conv); err != nil {
			continue
		}
		s.convs[conv.ID] = &conv
	}
	// Load sharing info
	sharePath := filepath.Join(s.dir, "_sharing.json")
	data, err := os.ReadFile(sharePath)
	if err == nil {
		var list []sharing
		if json.Unmarshal(data, &list) == nil {
			for i := range list {
				s.shares[list[i].ConventionID] = &list[i]
			}
		}
	}
	return nil
}

// save persists a single convention to disk.
func (s *Store) save(conv *Convention) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, conv.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

// saveShares persists sharing info to disk.
func (s *Store) saveShares() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	var list []sharing
	for _, sh := range s.shares {
		list = append(list, *sh)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "_sharing.json"), data, 0o644)
}

// generateID creates a unique ID for a convention.
func generateID(conv *Convention) string {
	h := sha256.New()
	h.Write([]byte(conv.Category + ":" + conv.Title))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// Add stores a new convention. It generates an ID, sets timestamps, and saves to disk.
func (s *Store) Add(conv *Convention) error {
	if conv.ID == "" {
		conv.ID = generateID(conv)
	}
	now := time.Now()
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = now
	}
	if conv.LastUsed.IsZero() {
		conv.LastUsed = now
	}
	if conv.UseCount == 0 {
		conv.UseCount = 0
	}

	s.mu.Lock()
	s.convs[conv.ID] = conv
	s.mu.Unlock()

	return s.save(conv)
}

// Get retrieves a convention by ID.
func (s *Store) Get(id string) (*Convention, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.convs[id]
	if !ok {
		return nil, fmt.Errorf("convention %q not found", id)
	}
	return conv, nil
}

// List returns conventions matching the filter.
func (s *Store) List(filter Filter) []*Convention {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Convention
	for _, conv := range s.convs {
		if filter.Category != "" && conv.Category != filter.Category {
			continue
		}
		if filter.Source != "" && conv.Source != filter.Source {
			continue
		}
		if filter.AgentID != "" && conv.AgentID != filter.AgentID {
			continue
		}
		if filter.MinConfidence > 0 && conv.Confidence < filter.MinConfidence {
			continue
		}
		result = append(result, conv)
	}
	return result
}

// Delete removes a convention by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.convs[id]; !ok {
		return fmt.Errorf("convention %q not found", id)
	}
	delete(s.convs, id)
	delete(s.shares, id)

	path := filepath.Join(s.dir, id+".json")
	_ = os.Remove(path)
	_ = s.saveShares()
	return nil
}

// Search performs a keyword search on title and description fields.
func (s *Store) Search(query string, limit int) []*Convention {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var results []*Convention
	for _, conv := range s.convs {
		if strings.Contains(strings.ToLower(conv.Title), q) ||
			strings.Contains(strings.ToLower(conv.Description), q) {
			results = append(results, conv)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}

// ComputeContentHash computes SHA256 of the given lines.
func ComputeContentHash(lines []string) string {
	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(l))
		h.Write([]byte("\n"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
