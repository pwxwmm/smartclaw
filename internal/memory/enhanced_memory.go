package memory

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryType string

const (
	MemoryTypeUser      MemoryType = "user"
	MemoryTypeFeedback  MemoryType = "feedback"
	MemoryTypeProject   MemoryType = "project"
	MemoryTypeReference MemoryType = "reference"
)

type MemoryScope string

const (
	ScopeUser    MemoryScope = "user"
	ScopeProject MemoryScope = "project"
	ScopeLocal   MemoryScope = "local"
)

type MemoryHeader struct {
	ID          string        `json:"id"`
	Filename    string        `json:"filename"`
	Filepath    string        `json:"filepath"`
	Type        MemoryType    `json:"type"`
	Description string        `json:"description"`
	Tags        []string      `json:"tags"`
	MtimeMs     int64         `json:"mtime_ms"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Age         time.Duration `json:"age"`
}

type MemoryFile struct {
	Header         MemoryHeader `json:"header"`
	Content        string       `json:"content"`
	RawFrontmatter string       `json:"raw_frontmatter"`
}

type EnhancedMemoryStore struct {
	basePath string
	scope    MemoryScope
	memories map[string]*MemoryFile
	headers  []*MemoryHeader
	mu       sync.RWMutex
}

func NewEnhancedMemoryStore(basePath string, scope MemoryScope) (*EnhancedMemoryStore, error) {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		basePath = filepath.Join(home, ".smartclaw", "memories")
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	store := &EnhancedMemoryStore{
		basePath: basePath,
		scope:    scope,
		memories: make(map[string]*MemoryFile),
		headers:  make([]*MemoryHeader, 0),
	}

	store.scanMemories()

	return store, nil
}

func (s *EnhancedMemoryStore) scanMemories() error {
	entries, err := ioutil.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || name == "MEMORY.md" {
			continue
		}

		path := filepath.Join(s.basePath, name)
		mem, err := s.loadMemoryFile(path)
		if err != nil {
			continue
		}

		s.memories[mem.Header.ID] = mem
		s.headers = append(s.headers, &mem.Header)
	}

	sort.Slice(s.headers, func(i, j int) bool {
		return s.headers[i].MtimeMs > s.headers[j].MtimeMs
	})

	return nil
}

func (s *EnhancedMemoryStore) loadMemoryFile(path string) (*MemoryFile, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	frontmatter, body := parseFrontmatter(content)

	header := MemoryHeader{
		ID:        generateMemoryID(),
		Filename:  filepath.Base(path),
		Filepath:  path,
		MtimeMs:   getFileMtime(path),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if frontmatter != "" {
		var fm struct {
			Type        MemoryType `json:"type"`
			Description string     `json:"description"`
			Tags        []string   `json:"tags"`
		}
		if err := json.Unmarshal([]byte(frontmatter), &fm); err == nil {
			header.Type = fm.Type
			header.Description = fm.Description
			header.Tags = fm.Tags
		}
	} else {
		header.Type = MemoryTypeUser
		header.Description = extractDescription(body, 100)
	}

	info, _ := os.Stat(path)
	if info != nil {
		header.Age = time.Since(info.ModTime())
	}

	return &MemoryFile{
		Header:         header,
		Content:        body,
		RawFrontmatter: frontmatter,
	}, nil
}

func parseFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}

	end := strings.Index(content[4:], "\n---\n")
	if end == -1 {
		return "", content
	}

	frontmatter := content[4 : end+4]
	body := content[end+8:]

	return frontmatter, body
}

func extractDescription(content string, maxLen int) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sb.WriteString(line)
		sb.WriteString(" ")
		if sb.Len() >= maxLen {
			break
		}
	}

	desc := strings.TrimSpace(sb.String())
	if len(desc) > maxLen {
		desc = desc[:maxLen-3] + "..."
	}

	return desc
}

func getFileMtime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UnixMilli()
	}
	return info.ModTime().UnixMilli()
}

func generateMemoryID() string {
	return fmt.Sprintf("mem_%d", time.Now().UnixNano())
}

func (s *EnhancedMemoryStore) CreateMemory(memType MemoryType, description, content string, tags []string) (*MemoryFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateMemoryID()
	filename := fmt.Sprintf("%s_%s.md", memType, id[:8])
	path := filepath.Join(s.basePath, filename)

	now := time.Now()
	header := MemoryHeader{
		ID:          id,
		Filename:    filename,
		Filepath:    path,
		Type:        memType,
		Description: description,
		Tags:        tags,
		MtimeMs:     now.UnixMilli(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	frontmatter := map[string]interface{}{
		"type":        memType,
		"description": description,
		"tags":        tags,
		"created":     now.Format(time.RFC3339),
	}

	fmBytes, err := json.Marshal(frontmatter)
	if err != nil {
		return nil, err
	}

	fullContent := fmt.Sprintf("---\n%s\n---\n\n%s", string(fmBytes), content)

	if err := ioutil.WriteFile(path, []byte(fullContent), 0644); err != nil {
		return nil, err
	}

	mem := &MemoryFile{
		Header:         header,
		Content:        content,
		RawFrontmatter: string(fmBytes),
	}

	s.memories[id] = mem
	s.headers = append([]*MemoryHeader{&header}, s.headers...)

	return mem, nil
}

func (s *EnhancedMemoryStore) GetMemory(id string) (*MemoryFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mem, ok := s.memories[id]
	if !ok {
		return nil, fmt.Errorf("memory not found: %s", id)
	}

	return mem, nil
}

func (s *EnhancedMemoryStore) UpdateMemory(id, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mem, ok := s.memories[id]
	if !ok {
		return fmt.Errorf("memory not found: %s", id)
	}

	fullContent := fmt.Sprintf("---\n%s\n---\n\n%s", mem.RawFrontmatter, content)

	if err := ioutil.WriteFile(mem.Header.Filepath, []byte(fullContent), 0644); err != nil {
		return err
	}

	mem.Content = content
	mem.Header.UpdatedAt = time.Now()
	mem.Header.MtimeMs = time.Now().UnixMilli()

	return nil
}

func (s *EnhancedMemoryStore) DeleteMemory(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mem, ok := s.memories[id]
	if !ok {
		return fmt.Errorf("memory not found: %s", id)
	}

	if err := os.Remove(mem.Header.Filepath); err != nil {
		return err
	}

	delete(s.memories, id)

	for i, h := range s.headers {
		if h.ID == id {
			s.headers = append(s.headers[:i], s.headers[i+1:]...)
			break
		}
	}

	return nil
}

func (s *EnhancedMemoryStore) ListMemories() []*MemoryHeader {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MemoryHeader, len(s.headers))
	copy(result, s.headers)
	return result
}

func (s *EnhancedMemoryStore) GetMemoriesByType(memType MemoryType) []*MemoryHeader {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MemoryHeader
	for _, h := range s.headers {
		if h.Type == memType {
			result = append(result, h)
		}
	}
	return result
}

func (s *EnhancedMemoryStore) GetMemoriesByTag(tag string) []*MemoryHeader {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MemoryHeader
	for _, h := range s.headers {
		for _, t := range h.Tags {
			if t == tag {
				result = append(result, h)
				break
			}
		}
	}
	return result
}

func (s *EnhancedMemoryStore) SearchMemories(query string) []*MemoryHeader {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(query)
	var result []*MemoryHeader

	for _, h := range s.headers {
		if strings.Contains(strings.ToLower(h.Description), query) ||
			strings.Contains(strings.ToLower(h.Filename), query) {
			result = append(result, h)
			continue
		}

		for _, tag := range h.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				result = append(result, h)
				break
			}
		}
	}

	return result
}

func (s *EnhancedMemoryStore) GetStaleMemories(maxAge time.Duration) []*MemoryHeader {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MemoryHeader
	now := time.Now()

	for _, h := range s.headers {
		age := now.Sub(h.UpdatedAt)
		if age > maxAge {
			result = append(result, h)
		}
	}

	return result
}

func (s *EnhancedMemoryStore) GetMemoryAge(id string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mem, ok := s.memories[id]
	if !ok {
		return 0, fmt.Errorf("memory not found: %s", id)
	}

	return time.Since(mem.Header.UpdatedAt), nil
}

func (s *EnhancedMemoryStore) FormatMemoryAge(age time.Duration) string {
	days := int(age.Hours() / 24)
	hours := int(age.Hours()) % 24

	if days > 0 {
		return fmt.Sprintf("%d days old", days)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours old", hours)
	}
	return "recent"
}

func (s *EnhancedMemoryStore) BuildMemoryPrompt(maxTokens int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("# Memory Context\n\n")

	typeGroups := make(map[MemoryType][]*MemoryHeader)
	for _, h := range s.headers {
		typeGroups[h.Type] = append(typeGroups[h.Type], h)
	}

	for _, memType := range []MemoryType{MemoryTypeUser, MemoryTypeProject, MemoryTypeFeedback, MemoryTypeReference} {
		group := typeGroups[memType]
		if len(group) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s Memories\n\n", strings.Title(string(memType))))

		for _, h := range group {
			ageStr := s.FormatMemoryAge(h.Age)
			sb.WriteString(fmt.Sprintf("### %s (%s)\n", h.Description, ageStr))
			if len(h.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(h.Tags, ", ")))
			}

			if mem, ok := s.memories[h.ID]; ok {
				content := mem.Content
				if len(content) > 500 {
					content = content[:500] + "..."
				}
				sb.WriteString("\n")
				sb.WriteString(content)
				sb.WriteString("\n\n")
			}
		}
	}

	result := sb.String()
	estimatedTokens := len(result) / 4

	if estimatedTokens > maxTokens {
		truncateLen := maxTokens * 4
		if truncateLen < len(result) {
			result = result[:truncateLen] + "\n\n... [memory truncated]"
		}
	}

	return result
}

func (s *EnhancedMemoryStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	typeCounts := make(map[MemoryType]int)
	for _, h := range s.headers {
		typeCounts[h.Type]++
	}

	return map[string]interface{}{
		"total_memories": len(s.headers),
		"by_type":        typeCounts,
		"scope":          s.scope,
		"base_path":      s.basePath,
	}
}

func (s *EnhancedMemoryStore) ExportMemory(id string) (string, error) {
	mem, err := s.GetMemory(id)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("---\n%s\n---\n\n%s", mem.RawFrontmatter, mem.Content), nil
}

func (s *EnhancedMemoryStore) ImportMemory(content string) (*MemoryFile, error) {
	frontmatter, body := parseFrontmatter(content)

	var fm struct {
		Type        MemoryType `json:"type"`
		Description string     `json:"description"`
		Tags        []string   `json:"tags"`
	}

	if frontmatter != "" {
		if err := json.Unmarshal([]byte(frontmatter), &fm); err != nil {
			fm.Type = MemoryTypeUser
			fm.Description = extractDescription(body, 100)
		}
	} else {
		fm.Type = MemoryTypeUser
		fm.Description = extractDescription(body, 100)
	}

	return s.CreateMemory(fm.Type, fm.Description, body, fm.Tags)
}
