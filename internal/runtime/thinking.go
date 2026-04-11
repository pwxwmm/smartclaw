package runtime

import (
	"os"
	"strconv"
	"sync"
)

// ThinkingManager tracks extended thinking state across a session.
type ThinkingManager struct {
	mu      sync.RWMutex
	enabled bool
	budget  int
}

// NewThinkingManager creates a ThinkingManager that reads initial state from env vars.
func NewThinkingManager() *ThinkingManager {
	tm := &ThinkingManager{
		enabled: false,
		budget:  10000,
	}

	if v := os.Getenv("SMARTCLAW_THINKING_ENABLED"); v == "true" {
		tm.enabled = true
	}
	if v := os.Getenv("SMARTCLAW_THINKING_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			tm.budget = n
		}
	}

	return tm
}

func (tm *ThinkingManager) Enable(budget int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.enabled = true
	if budget > 0 {
		tm.budget = budget
	}
}

func (tm *ThinkingManager) Disable() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.enabled = false
}

func (tm *ThinkingManager) IsEnabled() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.enabled
}

func (tm *ThinkingManager) GetBudget() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.budget
}

func (tm *ThinkingManager) SetBudget(budget int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if budget > 0 {
		tm.budget = budget
	}
}
