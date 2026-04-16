package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Variable represents a template variable with metadata
type Variable struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultValue string `json:"defaultValue"`
	Required     bool   `json:"required"`
}

// Template represents a prompt template
type Template struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Content     string     `json:"content"`
	Variables   []Variable `json:"variables,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Category    string     `json:"category,omitempty"`
	Author      string     `json:"author,omitempty"`
	Version     string     `json:"version,omitempty"`
	IsBuiltIn   bool       `json:"isBuiltIn"`
	CreatedAt   string     `json:"createdAt,omitempty"`
	UpdatedAt   string     `json:"updatedAt,omitempty"`
}

// TemplateManager manages prompt templates with built-ins and persistence
type TemplateManager struct {
	mu         sync.RWMutex
	templates  map[string]*Template
	configPath string
}

// NewTemplateManager creates a new template manager with built-in templates
func NewTemplateManager() *TemplateManager {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".smartclaw", "templates")

	tm := &TemplateManager{
		templates:  make(map[string]*Template),
		configPath: configPath,
	}

	for _, t := range builtInTemplates {
		tm.templates[t.ID] = t
	}

	tm.loadCustomTemplates()

	return tm
}

// NewTemplateManagerWithPath creates a template manager with a custom config path
func NewTemplateManagerWithPath(configPath string) *TemplateManager {
	tm := &TemplateManager{
		templates:  make(map[string]*Template),
		configPath: configPath,
	}

	for _, t := range builtInTemplates {
		tm.templates[t.ID] = t
	}

	tm.loadCustomTemplates()

	return tm
}

// loadCustomTemplates loads custom templates from disk
func (tm *TemplateManager) loadCustomTemplates() {
	if tm.configPath == "" {
		return
	}

	files, err := filepath.Glob(filepath.Join(tm.configPath, "*.json"))
	if err != nil {
		return
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var t Template
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}

		t.IsBuiltIn = false
		tm.templates[t.ID] = &t
	}
}

// Get retrieves a template by ID
func (tm *TemplateManager) Get(id string) (*Template, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return t, nil
}

// List returns all templates sorted by built-in first, then name
func (tm *TemplateManager) List() []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	templates := make([]*Template, 0, len(tm.templates))
	for _, t := range tm.templates {
		templates = append(templates, t)
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].IsBuiltIn != templates[j].IsBuiltIn {
			return templates[i].IsBuiltIn
		}
		return templates[i].Name < templates[j].Name
	})
	return templates
}

// ListByCategory returns templates grouped by category
func (tm *TemplateManager) ListByCategory() map[string][]*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	result := make(map[string][]*Template)
	for _, t := range tm.templates {
		category := t.Category
		if category == "" {
			category = "Other"
		}
		result[category] = append(result[category], t)
	}
	return result
}

// Create adds a new custom template
func (tm *TemplateManager) Create(t *Template) error {
	if t.ID == "" {
		return fmt.Errorf("template ID cannot be empty")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.templates[t.ID]; exists {
		return fmt.Errorf("template already exists: %s", t.ID)
	}

	t.IsBuiltIn = false
	tm.templates[t.ID] = t

	return tm.saveTemplate(t)
}

// Update modifies an existing custom template
func (tm *TemplateManager) Update(id string, t *Template) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	existing, exists := tm.templates[id]
	if !exists {
		return fmt.Errorf("template not found: %s", id)
	}

	if existing.IsBuiltIn {
		return fmt.Errorf("cannot modify built-in template: %s", id)
	}

	t.ID = id
	t.IsBuiltIn = false
	tm.templates[id] = t

	return tm.saveTemplate(t)
}

// Delete removes a custom template
func (tm *TemplateManager) Delete(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	t, exists := tm.templates[id]
	if !exists {
		return fmt.Errorf("template not found: %s", id)
	}

	if t.IsBuiltIn {
		return fmt.Errorf("cannot delete built-in template: %s", id)
	}

	delete(tm.templates, id)

	filePath := filepath.Join(tm.configPath, id+".json")
	return os.Remove(filePath)
}

// Render substitutes {{var}} placeholders with provided values
func (tm *TemplateManager) Render(id string, variables map[string]string) (string, error) {
	t, err := tm.Get(id)
	if err != nil {
		return "", err
	}

	content := t.Content

	for _, v := range t.Variables {
		value, ok := variables[v.Name]
		if !ok {
			value = v.DefaultValue
		}
		if v.Required && value == "" {
			return "", fmt.Errorf("required variable '%s' is missing", v.Name)
		}
		placeholder := "{{" + v.Name + "}}"
		content = strings.ReplaceAll(content, placeholder, value)
	}

	for key, value := range variables {
		placeholder := "{{" + key + "}}"
		content = strings.ReplaceAll(content, placeholder, value)
	}

	return content, nil
}

// GetVariables returns the variables for a template
func (tm *TemplateManager) GetVariables(id string) ([]Variable, error) {
	t, err := tm.Get(id)
	if err != nil {
		return nil, err
	}
	return t.Variables, nil
}

// Search finds templates matching a query
func (tm *TemplateManager) Search(query string) []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	query = strings.ToLower(query)
	var results []*Template

	for _, t := range tm.templates {
		if strings.Contains(strings.ToLower(t.Name), query) ||
			strings.Contains(strings.ToLower(t.Description), query) ||
			strings.Contains(strings.ToLower(t.Content), query) {
			results = append(results, t)
			continue
		}

		for _, tag := range t.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, t)
				break
			}
		}
	}

	return results
}

// ExportTemplate exports a template to JSON or Markdown format
func (tm *TemplateManager) ExportTemplate(id string, format string) (string, error) {
	t, err := tm.Get(id)
	if err != nil {
		return "", err
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "markdown", "md":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", t.Name))
		sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", t.Description))
		if t.Category != "" {
			sb.WriteString(fmt.Sprintf("**Category**: %s\n\n", t.Category))
		}
		if len(t.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("**Tags**: %s\n\n", strings.Join(t.Tags, ", ")))
		}
		if len(t.Variables) > 0 {
			sb.WriteString("## Variables\n\n")
			for _, v := range t.Variables {
				required := ""
				if v.Required {
					required = " (required)"
				}
				sb.WriteString(fmt.Sprintf("- `{{%s}}`: %s%s\n", v.Name, v.Description, required))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("## Content\n\n```\n")
		sb.WriteString(t.Content)
		sb.WriteString("\n```\n")
		return sb.String(), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// ImportTemplate imports a template from JSON data
func (tm *TemplateManager) ImportTemplate(data string, format string) error {
	var t Template

	switch format {
	case "json":
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return tm.Create(&t)
}

// ExtractVariables scans content for {{var}} patterns and returns Variable definitions
func ExtractVariables(content string) []Variable {
	var vars []Variable
	seen := make(map[string]bool)

	start := 0
	for {
		idx := strings.Index(content[start:], "{{")
		if idx == -1 {
			break
		}
		idx += start
		endIdx := strings.Index(content[idx:], "}}")
		if endIdx == -1 {
			break
		}
		endIdx += idx

		name := strings.TrimSpace(content[idx+2 : endIdx])
		if name != "" && !seen[name] {
			seen[name] = true
			vars = append(vars, Variable{
				Name:     name,
				Required: true,
			})
		}
		start = endIdx + 2
	}

	return vars
}

// saveTemplate persists a template to disk
func (tm *TemplateManager) saveTemplate(t *Template) error {
	if tm.configPath == "" {
		return fmt.Errorf("config path not set")
	}

	if err := os.MkdirAll(tm.configPath, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(tm.configPath, t.ID+".json")
	return os.WriteFile(filePath, data, 0644)
}

// BuiltInTemplates returns the list of built-in template definitions
var builtInTemplates = getBuiltInTemplates()

func getBuiltInTemplates() []*Template {
	codeBlock := "```"
	return []*Template{
		{
			ID:          "code-review",
			Name:        "Code Review",
			Description: "Comprehensive code review covering quality, security, and performance",
			Content: strings.Join([]string{
				"Please review the following code comprehensively:",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"Review focus areas:",
				"1. Code quality and readability",
				"2. Potential security vulnerabilities",
				"3. Performance optimization suggestions",
				"4. Best practices compliance",
				"5. Test coverage",
				"",
				"Please provide a detailed review report with improvement suggestions.",
			}, "\n"),
			Variables: []Variable{
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "code", Description: "Code to review", DefaultValue: "", Required: true},
			},
			Tags:      []string{"review", "quality", "security"},
			Category:  "Code Quality",
			IsBuiltIn: true,
		},
		{
			ID:          "explain-code",
			Name:        "Explain Code",
			Description: "Detailed explanation of code functionality and implementation",
			Content: strings.Join([]string{
				"Please explain the following code in detail:",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"Please include:",
				"1. Main functionality",
				"2. Key algorithms and data structures",
				"3. Execution flow",
				"4. Edge cases",
				"5. Usage examples",
			}, "\n"),
			Variables: []Variable{
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "code", Description: "Code to explain", DefaultValue: "", Required: true},
			},
			Tags:      []string{"explain", "learning"},
			Category:  "Learning",
			IsBuiltIn: true,
		},
		{
			ID:          "refactor",
			Name:        "Refactor Suggestions",
			Description: "Analyze code and provide refactoring suggestions",
			Content: strings.Join([]string{
				"Please analyze the following code and provide refactoring suggestions:",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"Focus on:",
				"1. Code smell identification",
				"2. Design pattern application",
				"3. Duplicate code elimination",
				"4. Complexity reduction",
				"5. Testability improvement",
				"",
				"Please provide refactored code examples.",
			}, "\n"),
			Variables: []Variable{
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "code", Description: "Code to refactor", DefaultValue: "", Required: true},
			},
			Tags:      []string{"refactor", "quality"},
			Category:  "Code Quality",
			IsBuiltIn: true,
		},
		{
			ID:          "write-tests",
			Name:        "Write Tests",
			Description: "Generate test cases for code",
			Content: strings.Join([]string{
				"Please write test cases for the following code:",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"Requirements:",
				"1. Cover normal cases",
				"2. Cover edge cases",
				"3. Cover error cases",
				"4. Use table-driven tests (if applicable)",
				"5. Add clear test descriptions",
				"",
				"Test framework: {{testFramework}}",
			}, "\n"),
			Variables: []Variable{
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "code", Description: "Code to test", DefaultValue: "", Required: true},
				{Name: "testFramework", Description: "Test framework", DefaultValue: "testing", Required: false},
			},
			Tags:      []string{"test", "testing"},
			Category:  "Testing",
			IsBuiltIn: true,
		},
		{
			ID:          "debug",
			Name:        "Debug Help",
			Description: "Analyze code issues and provide solutions",
			Content: strings.Join([]string{
				"I'm encountering an issue with the following code:",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"Error message:",
				"{{error}}",
				"",
				"Please help me:",
				"1. Analyze the error cause",
				"2. Provide a solution",
				"3. Explain why this issue occurs",
				"4. How to avoid similar issues",
			}, "\n"),
			Variables: []Variable{
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "code", Description: "Problematic code", DefaultValue: "", Required: true},
				{Name: "error", Description: "Error message", DefaultValue: "", Required: false},
			},
			Tags:      []string{"debug", "error"},
			Category:  "Debugging",
			IsBuiltIn: true,
		},
		{
			ID:          "implement-feature",
			Name:        "Implement Feature",
			Description: "Implement a feature based on requirements",
			Content: strings.Join([]string{
				"Please help me implement the following feature:",
				"",
				"Requirement description:",
				"{{requirement}}",
				"",
				"Technical requirements:",
				"- Language: {{language}}",
				"- Framework: {{framework}}",
				"",
				"Please provide:",
				"1. Implementation approach",
				"2. Complete code",
				"3. Usage example",
				"4. Notes",
			}, "\n"),
			Variables: []Variable{
				{Name: "requirement", Description: "Feature description", DefaultValue: "", Required: true},
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "framework", Description: "Framework to use", DefaultValue: "", Required: false},
			},
			Tags:      []string{"implement", "feature"},
			Category:  "Development",
			IsBuiltIn: true,
		},
		{
			ID:          "api-docs",
			Name:        "API Documentation",
			Description: "Generate documentation for API",
			Content: strings.Join([]string{
				"Please generate documentation for the following API:",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"Documentation format: {{format}}",
				"",
				"Please include:",
				"1. API endpoint description",
				"2. Request parameters",
				"3. Response format",
				"4. Example request and response",
				"5. Error codes",
			}, "\n"),
			Variables: []Variable{
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "code", Description: "API code", DefaultValue: "", Required: true},
				{Name: "format", Description: "Documentation format", DefaultValue: "markdown", Required: false},
			},
			Tags:      []string{"docs", "api"},
			Category:  "Documentation",
			IsBuiltIn: true,
		},
		{
			ID:          "commit-message",
			Name:        "Commit Message",
			Description: "Generate commit message from code changes",
			Content: strings.Join([]string{
				"Generate a commit message based on the following code changes:",
				"",
				"{{diff}}",
				"",
				"Please generate:",
				"1. Concise title (50 chars max)",
				"2. Detailed description (if needed)",
				"3. Follow Conventional Commits format",
				"",
				"Format:",
				"<type>(<scope>): <subject>",
				"",
				"<body>",
				"",
				"<footer>",
			}, "\n"),
			Variables: []Variable{
				{Name: "diff", Description: "Code changes", DefaultValue: "", Required: true},
			},
			Tags:      []string{"git", "commit"},
			Category:  "Git",
			IsBuiltIn: true,
		},
		{
			ID:          "sql-optimization",
			Name:        "SQL Optimization",
			Description: "Analyze and optimize SQL queries",
			Content: strings.Join([]string{
				"Please analyze and optimize the following SQL query:",
				"",
				codeBlock + "sql",
				"{{sql}}",
				codeBlock,
				"",
				"Database type: {{dbType}}",
				"Table schema:",
				"{{schema}}",
				"",
				"Please provide:",
				"1. Performance analysis",
				"2. Index suggestions",
				"3. Optimized SQL",
				"4. Estimated improvement",
			}, "\n"),
			Variables: []Variable{
				{Name: "sql", Description: "SQL query", DefaultValue: "", Required: true},
				{Name: "dbType", Description: "Database type", DefaultValue: "PostgreSQL", Required: false},
				{Name: "schema", Description: "Table schema", DefaultValue: "(not provided)", Required: false},
			},
			Tags:      []string{"sql", "optimization"},
			Category:  "Database",
			IsBuiltIn: true,
		},
		{
			ID:          "security-audit",
			Name:        "Security Audit",
			Description: "Security audit for code",
			Content: strings.Join([]string{
				"Please perform a security audit on the following code:",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"Audit focus:",
				"1. OWASP Top 10 vulnerabilities",
				"2. Input validation issues",
				"3. Authentication/authorization issues",
				"4. Sensitive data handling",
				"5. Injection vulnerabilities",
				"",
				"Please provide:",
				"- Vulnerability list (sorted by severity)",
				"- Detailed explanation for each",
				"- Fix suggestions with code examples",
			}, "\n"),
			Variables: []Variable{
				{Name: "language", Description: "Programming language", DefaultValue: "go", Required: true},
				{Name: "code", Description: "Code to audit", DefaultValue: "", Required: true},
			},
			Tags:      []string{"security", "audit"},
			Category:  "Security",
			IsBuiltIn: true,
		},
	}
}
