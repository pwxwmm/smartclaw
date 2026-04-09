package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SessionMemory struct {
	SessionID string                 `json:"session_id"`
	Messages  []MemoryMessage        `json:"messages"`
	Summary   string                 `json:"summary"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type MemoryMessage struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Tokens  int       `json:"tokens,omitempty"`
	Time    time.Time `json:"time"`
}

type SessionMemoryService struct {
	basePath string
	memories map[string]*SessionMemory
	mu       sync.RWMutex
}

func NewSessionMemoryService() (*SessionMemoryService, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	basePath := filepath.Join(home, ".smartclaw", "session-memory")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	svc := &SessionMemoryService{
		basePath: basePath,
		memories: make(map[string]*SessionMemory),
	}

	svc.loadAll()
	return svc, nil
}

func (s *SessionMemoryService) Create(sessionID string) *SessionMemory {
	s.mu.Lock()
	defer s.mu.Unlock()

	mem := &SessionMemory{
		SessionID: sessionID,
		Messages:  make([]MemoryMessage, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	s.memories[sessionID] = mem
	return mem
}

func (s *SessionMemoryService) Get(sessionID string) *SessionMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.memories[sessionID]
}

func (s *SessionMemoryService) AddMessage(sessionID string, role, content string, tokens int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mem, ok := s.memories[sessionID]
	if !ok {
		mem = s.Create(sessionID)
	}

	msg := MemoryMessage{
		Role:    role,
		Content: content,
		Tokens:  tokens,
		Time:    time.Now(),
	}

	mem.Messages = append(mem.Messages, msg)
	mem.UpdatedAt = time.Now()

	return s.save(mem)
}

func (s *SessionMemoryService) Summarize(sessionID string, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mem, ok := s.memories[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	mem.Summary = summary
	mem.UpdatedAt = time.Now()

	return s.save(mem)
}

func (s *SessionMemoryService) GetSummary(sessionID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mem, ok := s.memories[sessionID]
	if !ok {
		return ""
	}

	return mem.Summary
}

func (s *SessionMemoryService) GetMessages(sessionID string) []MemoryMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mem, ok := s.memories[sessionID]
	if !ok {
		return nil
	}

	return mem.Messages
}

func (s *SessionMemoryService) GetTotalTokens(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mem, ok := s.memories[sessionID]
	if !ok {
		return 0
	}

	total := 0
	for _, msg := range mem.Messages {
		total += msg.Tokens
	}

	return total
}

func (s *SessionMemoryService) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.memories, sessionID)

	path := filepath.Join(s.basePath, sessionID+".json")
	return os.Remove(path)
}

func (s *SessionMemoryService) List() []*SessionMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*SessionMemory, 0, len(s.memories))
	for _, mem := range s.memories {
		result = append(result, mem)
	}

	return result
}

func (s *SessionMemoryService) save(mem *SessionMemory) error {
	data, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.basePath, mem.SessionID+".json")
	return ioutil.WriteFile(path, data, 0644)
}

func (s *SessionMemoryService) loadAll() error {
	entries, err := ioutil.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.basePath, entry.Name())
		data, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}

		var mem SessionMemory
		if err := json.Unmarshal(data, &mem); err != nil {
			continue
		}

		s.memories[mem.SessionID] = &mem
	}

	return nil
}

func (s *SessionMemoryService) GetContextForPrompt(sessionID string, maxTokens int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mem, ok := s.memories[sessionID]
	if !ok {
		return ""
	}

	if mem.Summary != "" {
		return "Previous session summary:\n" + mem.Summary
	}

	var parts []string
	currentTokens := 0

	for i := len(mem.Messages) - 1; i >= 0; i-- {
		msg := mem.Messages[i]
		if currentTokens+msg.Tokens > maxTokens {
			break
		}

		parts = append([]string{msg.Role + ": " + truncate(msg.Content, 200)}, parts...)
		currentTokens += msg.Tokens
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

type PromptSuggestion struct {
	Pattern    string    `json:"pattern"`
	Suggestion string    `json:"suggestion"`
	Count      int       `json:"count"`
	CreatedAt  time.Time `json:"created_at"`
}

type PromptSuggestionService struct {
	suggestions []PromptSuggestion
	mu          sync.RWMutex
}

func NewPromptSuggestionService() *PromptSuggestionService {
	return &PromptSuggestionService{
		suggestions: make([]PromptSuggestion, 0),
	}
}

func (s *PromptSuggestionService) AddSuggestion(pattern, suggestion string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.suggestions {
		if s.suggestions[i].Pattern == pattern {
			s.suggestions[i].Suggestion = suggestion
			s.suggestions[i].Count++
			s.suggestions[i].CreatedAt = time.Now()
			return
		}

		s.suggestions = append(s.suggestions, PromptSuggestion{
			Pattern:    pattern,
			Suggestion: suggestion,
			Count:      1,
			CreatedAt:  time.Now(),
		})
	}
}

func (s *PromptSuggestionService) GetSuggestion(pattern string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sug := range s.suggestions {
		if sug.Pattern == pattern {
			return sug.Suggestion
		}
	}

	return ""
}

func (s *PromptSuggestionService) GetTopSuggestions(limit int) []PromptSuggestion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := len(s.suggestions)
	if n > limit {
		n = limit
	}

	result := make([]PromptSuggestion, n)
	copy(result, s.suggestions[:n])

	return result
}
