package layers

import (
	"path/filepath"
	"strings"
)

// ContextHint describes the current working context (file, language, task).
type ContextHint struct {
	FilePath string
	Language string
	TaskType string
	Keywords []string
}

// SkillInjector selects relevant skills based on context, reducing system prompt size.
type SkillInjector struct {
	skillMemory *SkillProceduralMemory
	langMap     map[string][]string
	taskMap     map[string][]string
	keywordMap  map[string][]string
}

// NewSkillInjector creates a SkillInjector backed by the given SkillProceduralMemory.
func NewSkillInjector(spm *SkillProceduralMemory) *SkillInjector {
	si := &SkillInjector{
		skillMemory: spm,
	}
	si.buildMaps()
	return si
}

// InjectSkills returns the names of skills relevant to the given context hint.
// It ranks by relevance and caps at maxSkills to control prompt size.
func (si *SkillInjector) InjectSkills(hint ContextHint, maxSkills int) []string {
	if maxSkills <= 0 {
		maxSkills = 5
	}

	scored := make(map[string]float64)

	// Score by language match
	detectedLang := hint.Language
	if detectedLang == "" && hint.FilePath != "" {
		detectedLang = langFromExt(hint.FilePath)
	}
	if detectedLang != "" {
		if skills, ok := si.langMap[detectedLang]; ok {
			for _, s := range skills {
				scored[s] += 2.0
			}
		}
	}

	// Score by task type
	if hint.TaskType != "" {
		if skills, ok := si.taskMap[hint.TaskType]; ok {
			for _, s := range skills {
				scored[s] += 1.5
			}
		}
	}

	// Score by keyword match
	for _, kw := range hint.Keywords {
		kwLower := strings.ToLower(kw)
		if skills, ok := si.keywordMap[kwLower]; ok {
			for _, s := range skills {
				scored[s] += 1.0
			}
		}
	}

	// Also score by skill tags matching detected language
	if si.skillMemory != nil {
		index := si.skillMemory.GetIndex()
		for name, summary := range index {
			for _, tag := range summary.Tags {
				if strings.EqualFold(tag, detectedLang) {
					scored[name] += 1.0
				}
				for _, kw := range hint.Keywords {
					if strings.EqualFold(tag, kw) {
						scored[name] += 0.5
					}
				}
			}
		}
	}

	// Sort by score descending
	type scoredEntry struct {
		name  string
		score float64
	}
	entries := make([]scoredEntry, 0, len(scored))
	for name, score := range scored {
		if score > 0 {
			entries = append(entries, scoredEntry{name, score})
		}
	}

	// Simple insertion sort (small N)
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].score > entries[j-1].score; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	result := make([]string, 0, maxSkills)
	for i := 0; i < len(entries) && i < maxSkills; i++ {
		result = append(result, entries[i].name)
	}

	return result
}

// BuildContextAwarePrompt builds a skill prompt containing only context-relevant skills.
// This replaces BuildSkillPrompt() when context-aware injection is enabled.
func (si *SkillInjector) BuildContextAwarePrompt(hint ContextHint, maxSkills int) string {
	relevant := si.InjectSkills(hint, maxSkills)
	if len(relevant) == 0 {
		return ""
	}

	if si.skillMemory == nil {
		return ""
	}

	index := si.skillMemory.GetIndex()

	var sb strings.Builder
	sb.WriteString("## Relevant Skills\n\n")

	for _, name := range relevant {
		if summary, ok := index[name]; ok {
			sb.WriteString("- **")
			sb.WriteString(name)
			sb.WriteString("**: ")
			sb.WriteString(summary.Description)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (si *SkillInjector) buildMaps() {
	si.langMap = map[string][]string{
		"go":         {"debugger", "refactoring", "testing", "performance"},
		"python":     {"debugger", "refactoring", "api-designer", "deployment"},
		"javascript": {"debugger", "refactoring", "api-designer", "deployment"},
		"typescript": {"debugger", "refactoring", "api-designer", "deployment"},
		"rust":       {"debugger", "performance", "security"},
		"java":       {"debugger", "refactoring", "deployment", "api-designer"},
		"ruby":       {"debugger", "refactoring", "deployment"},
	}

	si.taskMap = map[string][]string{
		"debug":       {"debugger", "stuck"},
		"fix":         {"debugger", "stuck"},
		"refactor":    {"refactoring", "simplify"},
		"test":        {"test-generator", "verify"},
		"deploy":      {"deployment", "verify"},
		"review":      {"code-review", "verify"},
		"document":    {"documentation"},
		"security":    {"security", "verify"},
		"performance": {"performance"},
		"api":         {"api-designer"},
		"commit":      {"git-expert"},
		"git":         {"git-expert"},
		"help":        {"stuck"},
	}

	si.keywordMap = map[string][]string{
		"auth":      {"security", "api-designer"},
		"jwt":       {"security", "api-designer"},
		"oauth":     {"security", "api-designer"},
		"database":  {"api-designer", "performance"},
		"sql":       {"api-designer", "performance"},
		"docker":    {"deployment", "batch"},
		"container": {"deployment"},
		"ci":        {"deployment", "batch"},
		"cd":        {"deployment", "batch"},
		"test":      {"test-generator", "verify"},
		"lint":      {"code-review", "simplify"},
		"format":    {"simplify"},
		"monitor":   {"performance", "loop"},
		"log":       {"debugger", "performance"},
		"error":     {"debugger", "stuck"},
		"bug":       {"debugger", "stuck"},
		"memory":    {"remember", "performance"},
		"cache":     {"performance"},
		"parallel":  {"batch", "loop"},
	}
}

func langFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py", ".pyw":
		return "python"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".css", ".scss", ".sass":
		return "css"
	case ".html", ".htm":
		return "html"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".sh", ".bash":
		return "bash"
	case ".sql":
		return "sql"
	default:
		return ""
	}
}
