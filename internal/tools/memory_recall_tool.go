package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/memory"
)

var (
	memoryManagerMu   sync.RWMutex
	memoryManagerInst *memory.MemoryManager
)

// SetMemoryManagerForTools registers the MemoryManager instance for use by
// the memory recall tool. Call this during application initialization.
func SetMemoryManagerForTools(mm *memory.MemoryManager) {
	memoryManagerMu.Lock()
	defer memoryManagerMu.Unlock()
	memoryManagerInst = mm
}

func getMemoryManager() *memory.MemoryManager {
	memoryManagerMu.RLock()
	defer memoryManagerMu.RUnlock()
	return memoryManagerInst
}

// MemoryRecallTool exposes the 4-layer memory framework as a queryable tool.
// Actions: recall, search, store, layers, stats
type MemoryRecallTool struct{ BaseTool }

func (t *MemoryRecallTool) Name() string { return "memory" }
func (t *MemoryRecallTool) Description() string {
	return "Query and manage the 4-layer memory system. Actions: recall (search across sessions), search (FTS5 query), store (save a fact), layers (show memory state), stats (usage stats)"
}

func (t *MemoryRecallTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"recall", "search", "store", "layers", "stats"},
				"description": "Memory operation",
				"default":     "recall",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query for recall/search actions",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "Key for store action",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "Value for store action",
			},
			"layer": map[string]any{
				"type":        "string",
				"enum":        []string{"prompt", "session", "skill", "user"},
				"description": "Specific memory layer to target",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max results for search",
				"default":     5,
			},
		},
		"required": []string{"action"},
	}
}

func (t *MemoryRecallTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	action, _ := input["action"].(string)
	if action == "" {
		action = "recall"
	}

	switch action {
	case "recall":
		return t.recall(ctx, input)
	case "search":
		return t.search(input)
	case "store":
		return t.store(input)
	case "layers":
		return t.layers()
	case "stats":
		return t.stats()
	default:
		return nil, fmt.Errorf("unknown action: %s (valid: recall, search, store, layers, stats)", action)
	}
}

func (t *MemoryRecallTool) recall(ctx context.Context, input map[string]any) (any, error) {
	query, _ := input["query"].(string)
	limit := 5
	if l, ok := input["limit"].(int); ok && l > 0 {
		limit = l
	}

	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".smartclaw")

	// L1: Search MEMORY.md
	var results []map[string]any
	memoryPath := filepath.Join(baseDir, "MEMORY.md")
	if data, err := os.ReadFile(memoryPath); err == nil {
		content := string(data)
		if query == "" || strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
			results = append(results, map[string]any{
				"layer":   "L1_prompt",
				"source":  "MEMORY.md",
				"content": truncate(content, 2000),
				"matched": query == "" || strings.Contains(strings.ToLower(content), strings.ToLower(query)),
			})
		}
	}

	// L1: Search USER.md
	userPath := filepath.Join(baseDir, "USER.md")
	if data, err := os.ReadFile(userPath); err == nil {
		content := string(data)
		if query == "" || strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
			results = append(results, map[string]any{
				"layer":   "L1_prompt",
				"source":  "USER.md",
				"content": truncate(content, 1000),
				"matched": query == "" || strings.Contains(strings.ToLower(content), strings.ToLower(query)),
			})
		}
	}

	// L2: Search session store (SQLite FTS5)
	if mm := getMemoryManager(); mm != nil {
		if ss := mm.GetSessionSearch(); ss != nil && query != "" {
			fragments, err := ss.Search(ctx, query, limit)
			if err != nil {
				slog.Warn("memory recall: L2 session search failed", "error", err)
				results = append(results, map[string]any{
					"layer":  "L2_session",
					"source": "state.db",
					"error":  err.Error(),
				})
			} else if len(fragments) > 0 {
				var fragmentResults []map[string]any
				for _, f := range fragments {
					fragmentResults = append(fragmentResults, map[string]any{
						"session_id": f.SessionID,
						"role":       f.Role,
						"content":    truncate(f.Content, 300),
						"relevance":  f.Relevance,
						"timestamp":  f.Timestamp.Format(time.RFC3339),
					})
				}
				results = append(results, map[string]any{
					"layer":     "L2_session",
					"source":    "state.db",
					"query":     query,
					"fragments": fragmentResults,
					"count":     len(fragmentResults),
				})
			} else {
				results = append(results, map[string]any{
					"layer":  "L2_session",
					"source": "state.db",
					"query":  query,
					"count":  0,
				})
			}
		}
	} else {
		dbPath := filepath.Join(baseDir, "state.db")
		if stat, err := os.Stat(dbPath); err == nil && stat.Size() > 0 {
			results = append(results, map[string]any{
				"layer":  "L2_session",
				"source": "state.db",
				"note":   "Session search requires active MemoryManager connection",
				"query":  query,
			})
		}
	}

	// L3: List available skills
	skillsDir := filepath.Join(baseDir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		var skillNames []string
		for _, e := range entries {
			if e.IsDir() {
				skillNames = append(skillNames, e.Name())
			}
		}
		if query != "" {
			var matched []string
			for _, s := range skillNames {
				if strings.Contains(strings.ToLower(s), strings.ToLower(query)) {
					matched = append(matched, s)
				}
			}
			if len(matched) > 0 {
				results = append(results, map[string]any{
					"layer":   "L3_skill",
					"source":  "skills/",
					"matched": matched,
					"count":   len(matched),
				})
			}
		} else {
			results = append(results, map[string]any{
				"layer":  "L3_skill",
				"source": "skills/",
				"count":  len(skillNames),
				"skills": skillNames,
			})
		}
	}

	// L4: User observations
	if mm := getMemoryManager(); mm != nil {
		if um := mm.GetUserModel(); um != nil {
			model := um.GetModel()
			obsData := map[string]any{
				"layer":    "L4_user_model",
				"source":   "user_observations",
				"has_data": len(model.Preferences) > 0 || len(model.KnowledgeBg) > 0,
			}
			if len(model.Preferences) > 0 {
				obsData["preferences"] = model.Preferences
			}
			if model.CommunicationStyle != "" {
				obsData["communication_style"] = model.CommunicationStyle
			}
			if len(model.KnowledgeBg) > 0 {
				obsData["knowledge_background"] = model.KnowledgeBg
			}
			if len(model.CommonPatterns) > 0 {
				var patterns []map[string]any
				for _, p := range model.CommonPatterns {
					patterns = append(patterns, map[string]any{
						"pattern":   p.Pattern,
						"frequency": p.Frequency,
					})
				}
				obsData["common_patterns"] = patterns
			}
			if !model.LastUpdated.IsZero() {
				obsData["last_updated"] = model.LastUpdated.Format(time.RFC3339)
			}
			results = append(results, obsData)
		}
	} else {
		results = append(results, map[string]any{
			"layer":  "L4_user_model",
			"source": "user_observations",
			"note":   "User modeling data available through active MemoryManager",
		})
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
	}, nil
}

func (t *MemoryRecallTool) search(input map[string]any) (any, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return nil, ErrRequiredField("query")
	}

	limit := 5
	if l, ok := input["limit"].(int); ok && l > 0 {
		limit = l
	}

	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".smartclaw")

	var results []map[string]any

	// Search all text files in .smartclaw
	searchPaths := []string{"MEMORY.md", "USER.md", "SOUL.md", "AGENTS.md"}
	for _, name := range searchPaths {
		fullPath := filepath.Join(baseDir, name)
		if data, err := os.ReadFile(fullPath); err == nil {
			content := string(data)
			lower := strings.ToLower(content)
			lowerQuery := strings.ToLower(query)
			if strings.Contains(lower, lowerQuery) {
				// Extract context around match
				idx := strings.Index(lower, lowerQuery)
				start := idx - 100
				if start < 0 {
					start = 0
				}
				end := idx + len(query) + 100
				if end > len(content) {
					end = len(content)
				}
				snippet := content[start:end]
				results = append(results, map[string]any{
					"source":  name,
					"snippet": snippet,
					"offset":  start,
				})
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
	}, nil
}

func (t *MemoryRecallTool) store(input map[string]any) (any, error) {
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)
	if key == "" {
		return nil, ErrRequiredField("key")
	}
	if value == "" {
		return nil, ErrRequiredField("value")
	}

	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".smartclaw")
	os.MkdirAll(baseDir, 0755)

	layer, _ := input["layer"].(string)
	if layer == "" {
		layer = "prompt"
	}

	switch layer {
	case "prompt":
		if mm := getMemoryManager(); mm != nil {
			if pm := mm.GetPromptMemory(); pm != nil {
				entry := fmt.Sprintf("- **%s**: %s (stored %s)", key, value, time.Now().Format("2006-01-02"))
				if err := pm.AppendToSection("## Stored Facts", entry); err != nil {
					return nil, fmt.Errorf("failed to store via MemoryManager: %w", err)
				}
				return map[string]any{
					"layer":   "prompt",
					"key":     key,
					"stored":  true,
					"handler": "MemoryManager",
				}, nil
			}
		}
		memoryPath := filepath.Join(baseDir, "MEMORY.md")
		var content string
		if data, err := os.ReadFile(memoryPath); err == nil {
			content = string(data)
		}
		entry := fmt.Sprintf("\n- **%s**: %s (stored %s)", key, value, time.Now().Format("2006-01-02"))
		content += entry
		if err := os.WriteFile(memoryPath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write MEMORY.md: %w", err)
		}
		return map[string]any{
			"layer":   "prompt",
			"key":     key,
			"stored":  true,
			"path":    memoryPath,
			"handler": "file_fallback",
		}, nil

	default:
		return map[string]any{
			"layer":   layer,
			"key":     key,
			"stored":  false,
			"message": "only prompt layer storage is supported via this tool; use MemoryManager for other layers",
		}, nil
	}
}

func (t *MemoryRecallTool) layers() (any, error) {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".smartclaw")

	layers := []map[string]any{}

	// L1: Prompt Memory
	for _, name := range []string{"MEMORY.md", "USER.md", "SOUL.md", "AGENTS.md"} {
		fullPath := filepath.Join(baseDir, name)
		if data, err := os.ReadFile(fullPath); err == nil {
			layers = append(layers, map[string]any{
				"layer": "L1_prompt",
				"file":  name,
				"size":  len(data),
				"chars": len(string(data)),
			})
		}
	}

	// L2: Session Search
	dbPath := filepath.Join(baseDir, "state.db")
	if stat, err := os.Stat(dbPath); err == nil {
		layers = append(layers, map[string]any{
			"layer": "L2_session",
			"file":  "state.db",
			"size":  stat.Size(),
		})
	}

	// L3: Skill Procedural
	skillsDir := filepath.Join(baseDir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		layers = append(layers, map[string]any{
			"layer": "L3_skill",
			"dir":   "skills/",
			"count": len(entries),
		})
	}

	// L4: User Modeling
	layers = append(layers, map[string]any{
		"layer": "L4_user_model",
		"table": "user_observations",
	})

	return map[string]any{
		"layers": layers,
		"count":  len(layers),
	}, nil
}

func (t *MemoryRecallTool) stats() (any, error) {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".smartclaw")

	stats := map[string]any{}

	// Count files and sizes
	totalSize := int64(0)
	fileCount := 0
	filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	stats["base_dir"] = baseDir
	stats["total_files"] = fileCount
	stats["total_size_bytes"] = totalSize
	stats["total_size_kb"] = totalSize / 1024

	return stats, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
