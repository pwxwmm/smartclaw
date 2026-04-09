package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TodoItem represents a single todo item
type TodoItem struct {
	Content  string `json:"content"`
	Status   string `json:"status"`   // pending, in_progress, completed, cancelled
	Priority string `json:"priority"` // high, medium, low
}

// TodoList represents a list of todos for a session
type TodoList struct {
	SessionID string     `json:"session_id"`
	Todos     []TodoItem `json:"todos"`
	UpdatedAt time.Time  `json:"updated_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// TodoManager manages todo lists with persistence
type TodoManager struct {
	mu       sync.RWMutex
	basePath string
	todos    map[string]*TodoList // session_id -> TodoList
}

// NewTodoManager creates a new todo manager
func NewTodoManager() (*TodoManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	basePath := filepath.Join(home, ".smartclaw", "todos")
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	tm := &TodoManager{
		basePath: basePath,
		todos:    make(map[string]*TodoList),
	}

	// Load existing todos
	tm.loadAll()

	return tm, nil
}

// Get returns the todo list for a session
func (tm *TodoManager) Get(sessionID string) []TodoItem {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if list, exists := tm.todos[sessionID]; exists {
		return list.Todos
	}
	return []TodoItem{}
}

// Set updates the todo list for a session
func (tm *TodoManager) Set(sessionID string, todos []TodoItem) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()

	list, exists := tm.todos[sessionID]
	if !exists {
		list = &TodoList{
			SessionID: sessionID,
			CreatedAt: now,
		}
		tm.todos[sessionID] = list
	}

	// Check if all todos are completed
	allDone := len(todos) > 0
	for _, t := range todos {
		if t.Status != "completed" {
			allDone = false
			break
		}
	}

	// If all done, clear the list
	if allDone {
		list.Todos = []TodoItem{}
	} else {
		list.Todos = todos
	}
	list.UpdatedAt = now

	// Persist to disk
	return tm.save(sessionID)
}

// Update updates a single todo item
func (tm *TodoManager) Update(sessionID string, index int, todo TodoItem) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	list, exists := tm.todos[sessionID]
	if !exists || index < 0 || index >= len(list.Todos) {
		return &Error{Code: "INVALID_INDEX", Message: "invalid todo index"}
	}

	list.Todos[index] = todo
	list.UpdatedAt = time.Now()

	return tm.save(sessionID)
}

// Add adds a new todo item
func (tm *TodoManager) Add(sessionID string, todo TodoItem) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	list, exists := tm.todos[sessionID]
	if !exists {
		list = &TodoList{
			SessionID: sessionID,
			CreatedAt: time.Now(),
		}
		tm.todos[sessionID] = list
	}

	list.Todos = append(list.Todos, todo)
	list.UpdatedAt = time.Now()

	return tm.save(sessionID)
}

// Clear clears all todos for a session
func (tm *TodoManager) Clear(sessionID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.todos, sessionID)

	// Remove from disk
	path := tm.getPath(sessionID)
	os.Remove(path)

	return nil
}

// ListAll returns all session IDs with todos
func (tm *TodoManager) ListAll() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	sessions := make([]string, 0, len(tm.todos))
	for id := range tm.todos {
		sessions = append(sessions, id)
	}
	return sessions
}

// GetOldTodos returns the previous state of todos for comparison
func (tm *TodoManager) GetOldTodos(sessionID string) []TodoItem {
	return tm.Get(sessionID)
}

// CheckVerificationNudge checks if verification nudge is needed
func (tm *TodoManager) CheckVerificationNudge(sessionID string, newTodos []TodoItem) bool {
	// Check if:
	// 1. All todos are completed
	// 2. At least 3 todos were completed
	// 3. None of them was a verification step
	if len(newTodos) < 3 {
		return false
	}

	allDone := true
	hasVerification := false

	for _, t := range newTodos {
		if t.Status != "completed" {
			allDone = false
		}
		if containsVerification(t.Content) {
			hasVerification = true
		}
	}

	return allDone && !hasVerification
}

// Private methods

func (tm *TodoManager) save(sessionID string) error {
	list, exists := tm.todos[sessionID]
	if !exists {
		return nil
	}

	path := tm.getPath(sessionID)
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (tm *TodoManager) load(sessionID string) error {
	path := tm.getPath(sessionID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var list TodoList
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	tm.todos[sessionID] = &list
	return nil
}

func (tm *TodoManager) loadAll() error {
	entries, err := os.ReadDir(tm.basePath)
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

		sessionID := entry.Name()[:len(entry.Name())-5] // remove .json
		tm.load(sessionID)
	}

	return nil
}

func (tm *TodoManager) getPath(sessionID string) string {
	return filepath.Join(tm.basePath, sessionID+".json")
}

func containsVerification(content string) bool {
	// Check if content contains verification-related keywords
	keywords := []string{"verif", "test", "check", "validate", "ensure"}
	for _, kw := range keywords {
		if containsIgnoreCase(content, kw) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive substring check
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sLower[i] = c
	}

	for i := 0; i < len(substr); i++ {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		substrLower[i] = c
	}

	return contains(string(sLower), string(substrLower))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
