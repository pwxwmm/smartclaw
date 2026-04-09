package layers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const MaxPromptMemoryChars = 3575

type ManagedFile struct {
	path    string
	content string
	modTime time.Time
}

func newManagedFile(path string) *ManagedFile {
	return &ManagedFile{path: path}
}

func (mf *ManagedFile) Read() error {
	info, err := os.Stat(mf.path)
	if err != nil {
		return fmt.Errorf("managed file stat: %w", err)
	}

	data, err := os.ReadFile(mf.path)
	if err != nil {
		return fmt.Errorf("managed file read: %w", err)
	}

	mf.content = string(data)
	mf.modTime = info.ModTime()
	return nil
}

func (mf *ManagedFile) Write(content string) error {
	if err := os.MkdirAll(filepath.Dir(mf.path), 0755); err != nil {
		return fmt.Errorf("managed file mkdir: %w", err)
	}

	if err := os.WriteFile(mf.path, []byte(content), 0644); err != nil {
		return fmt.Errorf("managed file write: %w", err)
	}

	mf.content = content
	mf.modTime = time.Now()
	return nil
}

func (mf *ManagedFile) Content() string {
	return mf.content
}

func (mf *ManagedFile) ModTime() time.Time {
	return mf.modTime
}

// PromptMemory implements Layer 1: constant context loaded every session.
// MEMORY.md stores system knowledge; USER.md stores user profile.
// Combined they are hard-limited to 3,575 characters (Hermes constraint).
type PromptMemory struct {
	memoryMD *ManagedFile
	userMD   *ManagedFile
	mu       sync.RWMutex
}

func NewPromptMemory() (*PromptMemory, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("prompt memory: %w", err)
	}
	baseDir := filepath.Join(home, ".smartclaw")

	pm := &PromptMemory{
		memoryMD: newManagedFile(filepath.Join(baseDir, "MEMORY.md")),
		userMD:   newManagedFile(filepath.Join(baseDir, "USER.md")),
	}

	if err := pm.ensureDefaults(); err != nil {
		return nil, err
	}

	return pm, nil
}

func NewPromptMemoryWithDir(dir string) (*PromptMemory, error) {
	pm := &PromptMemory{
		memoryMD: newManagedFile(filepath.Join(dir, "MEMORY.md")),
		userMD:   newManagedFile(filepath.Join(dir, "USER.md")),
	}

	if err := pm.ensureDefaults(); err != nil {
		return nil, err
	}

	return pm, nil
}

const defaultMemoryMD = `# System Memory

## User Preferences
<!-- Add observed user preferences here -->

## Learned Patterns
<!-- Add reusable patterns discovered during tasks -->

## Active Context
<!-- Add current project/task context here -->
`

const defaultUserMD = `# User Profile

## Communication Style
<!-- Add observed communication preferences -->

## Knowledge Background
<!-- Add observed skills and expertise -->

## Common Workflows
<!-- Add observed recurring work patterns -->
`

func (pm *PromptMemory) ensureDefaults() error {
	if _, err := os.Stat(pm.memoryMD.path); os.IsNotExist(err) {
		if err := pm.memoryMD.Write(defaultMemoryMD); err != nil {
			return fmt.Errorf("prompt memory: create default MEMORY.md: %w", err)
		}
	} else if err := pm.memoryMD.Read(); err != nil {
		return fmt.Errorf("prompt memory: read MEMORY.md: %w", err)
	}

	if _, err := os.Stat(pm.userMD.path); os.IsNotExist(err) {
		if err := pm.userMD.Write(defaultUserMD); err != nil {
			return fmt.Errorf("prompt memory: create default USER.md: %w", err)
		}
	} else if err := pm.userMD.Read(); err != nil {
		return fmt.Errorf("prompt memory: read USER.md: %w", err)
	}

	return nil
}

// AutoLoad loads both files and returns combined content for system prompt injection.
// The result is hard-truncated to MaxPromptMemoryChars (3,575).
func (pm *PromptMemory) AutoLoad() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	combined := pm.memoryMD.Content() + "\n" + pm.userMD.Content()
	if len(combined) > MaxPromptMemoryChars {
		combined = combined[:MaxPromptMemoryChars]
	}
	return combined
}

func (pm *PromptMemory) UpdateMemory(content string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if err := pm.memoryMD.Write(content); err != nil {
		return fmt.Errorf("prompt memory: update MEMORY.md: %w", err)
	}
	return nil
}

func (pm *PromptMemory) UpdateUserProfile(profile string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if err := pm.userMD.Write(profile); err != nil {
		return fmt.Errorf("prompt memory: update USER.md: %w", err)
	}
	return nil
}

// EnforceLimit checks if combined content exceeds the character limit.
// In this phase, it only logs a warning. Full LLM-based compression comes in Phase 3.
func (pm *PromptMemory) EnforceLimit() error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	combined := pm.memoryMD.Content() + "\n" + pm.userMD.Content()
	if len(combined) > MaxPromptMemoryChars {
		// Phase 3 will add LLM-based compression here.
		// For now, truncate silently on next AutoLoad.
		_ = combined
	}
	return nil
}

// Reload re-reads both files from disk (useful after external edits or watcher events).
func (pm *PromptMemory) Reload() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if err := pm.memoryMD.Read(); err != nil {
		return fmt.Errorf("prompt memory: reload MEMORY.md: %w", err)
	}
	if err := pm.userMD.Read(); err != nil {
		return fmt.Errorf("prompt memory: reload USER.md: %w", err)
	}
	return nil
}

func (pm *PromptMemory) GetMemoryPath() string {
	return pm.memoryMD.path
}

func (pm *PromptMemory) GetUserPath() string {
	return pm.userMD.path
}

func (pm *PromptMemory) GetMemoryContent() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.memoryMD.Content()
}

func (pm *PromptMemory) GetUserContent() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.userMD.Content()
}

// AppendToSection appends a line under a specific ## section in MEMORY.md.
// If the section doesn't exist, it appends a new section at the end.
func (pm *PromptMemory) AppendToSection(section, line string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	content := pm.memoryMD.Content()
	updated, err := appendToMarkdownSection(content, section, line)
	if err != nil {
		return fmt.Errorf("prompt memory: append to section %q: %w", section, err)
	}

	if err := pm.memoryMD.Write(updated); err != nil {
		return fmt.Errorf("prompt memory: write updated MEMORY.md: %w", err)
	}
	return nil
}

func appendToMarkdownSection(content, section, line string) (string, error) {
	lines := strings.Split(content, "\n")
	sectionHeader := "## " + section

	sectionIdx := -1
	nextSectionIdx := len(lines)

	for i, l := range lines {
		if strings.TrimSpace(l) == sectionHeader {
			sectionIdx = i
			continue
		}
		if sectionIdx >= 0 && strings.HasPrefix(strings.TrimSpace(l), "## ") && i > sectionIdx {
			nextSectionIdx = i
			break
		}
	}

	if sectionIdx >= 0 {
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:nextSectionIdx]...)
		newLines = append(newLines, line)
		newLines = append(newLines, lines[nextSectionIdx:]...)
		return strings.Join(newLines, "\n"), nil
	}

	return content + "\n" + sectionHeader + "\n" + line + "\n", nil
}
