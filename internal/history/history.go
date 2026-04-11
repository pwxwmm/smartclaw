package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type HistoryEntry struct {
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
}

type History struct {
	entries  []HistoryEntry
	filePath string
	maxSize  int
	mu       sync.RWMutex
}

func NewHistory() (*History, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(home, ".smartclaw", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}

	history := &History{
		entries:  make([]HistoryEntry, 0),
		filePath: filePath,
		maxSize:  10000,
	}

	history.load()

	return history, nil
}

func (h *History) Add(command string, sessionID string, exitCode int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := HistoryEntry{
		Command:   command,
		Timestamp: time.Now(),
		SessionID: sessionID,
		ExitCode:  exitCode,
	}

	h.entries = append(h.entries, entry)

	if len(h.entries) > h.maxSize {
		h.entries = h.entries[len(h.entries)-h.maxSize:]
	}

	return h.appendToFile(entry)
}

func (h *History) Get(limit int) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if limit <= 0 || limit > len(h.entries) {
		limit = len(h.entries)
	}

	start := len(h.entries) - limit
	if start < 0 {
		start = 0
	}

	result := make([]HistoryEntry, limit)
	copy(result, h.entries[start:])

	return result
}

func (h *History) Search(query string, limit int) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make([]HistoryEntry, 0)
	for i := len(h.entries) - 1; i >= 0 && len(results) < limit; i-- {
		if contains(h.entries[i].Command, query) {
			results = append(results, h.entries[i])
		}
	}

	return results
}

func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.entries = make([]HistoryEntry, 0)

	return os.Remove(h.filePath)
}

func (h *History) GetUnique(limit int) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	seen := make(map[string]bool)
	unique := make([]string, 0)

	for i := len(h.entries) - 1; i >= 0 && len(unique) < limit; i-- {
		cmd := h.entries[i].Command
		if !seen[cmd] {
			seen[cmd] = true
			unique = append(unique, cmd)
		}
	}

	return unique
}

func (h *History) Stats() map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()

	unique := make(map[string]bool)
	var successCount int

	for _, entry := range h.entries {
		unique[entry.Command] = true
		if entry.ExitCode == 0 {
			successCount++
		}
	}

	return map[string]any{
		"total_commands":  len(h.entries),
		"unique_commands": len(unique),
		"success_rate":    float64(successCount) / float64(len(h.entries)) * 100,
	}
}

func (h *History) load() error {
	file, err := os.Open(h.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
			continue
		}
		h.entries = append(h.entries, entry)
	}

	return scanner.Err()
}

func (h *History) appendToFile(entry HistoryEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(h.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintln(file, string(data))
	return err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func (h *History) Autocomplete(prefix string) []string {
	return h.GetUnique(100)
}

func (h *History) LastCommand() *HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.entries) == 0 {
		return nil
	}

	entry := h.entries[len(h.entries)-1]
	return &entry
}

func (h *History) GetBySession(sessionID string) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make([]HistoryEntry, 0)
	for _, entry := range h.entries {
		if entry.SessionID == sessionID {
			results = append(results, entry)
		}
	}
	return results
}
