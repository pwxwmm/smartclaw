package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TemplateVariable struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultValue string `json:"defaultValue"`
	Required     bool   `json:"required"`
}

type PromptTemplate struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Content     string             `json:"content"`
	Variables   []TemplateVariable `json:"variables,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Category    string             `json:"category,omitempty"`
	Author      string             `json:"author,omitempty"`
	Version     string             `json:"version,omitempty"`
	IsBuiltIn   bool               `json:"isBuiltIn"`
	CreatedAt   string             `json:"createdAt,omitempty"`
	UpdatedAt   string             `json:"updatedAt,omitempty"`
}

type TemplateManager struct {
	templates  map[string]*PromptTemplate
	configPath string
}

const codeBlock = "```"

func getBuiltInTemplates() []*PromptTemplate {
	return []*PromptTemplate{
		{
			ID:          "code-review",
			Name:        "代码审查",
			Description: "对代码进行全面审查，包括质量、安全、性能等方面",
			Content: strings.Join([]string{
				"请对以下代码进行全面审查：",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"审查要点：",
				"1. 代码质量和可读性",
				"2. 潜在的安全漏洞",
				"3. 性能优化建议",
				"4. 最佳实践遵循情况",
				"5. 测试覆盖率",
				"",
				"请提供详细的审查报告和改进建议。",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "code", Description: "要审查的代码", DefaultValue: "", Required: true},
			},
			Tags:      []string{"review", "quality", "security"},
			Category:  "代码质量",
			IsBuiltIn: true,
		},
		{
			ID:          "explain-code",
			Name:        "代码解释",
			Description: "详细解释代码的功能和实现原理",
			Content: strings.Join([]string{
				"请详细解释以下代码的功能和实现原理：",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"请包括：",
				"1. 代码的主要功能",
				"2. 关键算法和数据结构",
				"3. 执行流程",
				"4. 潜在的边界情况",
				"5. 使用示例",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "code", Description: "要解释的代码", DefaultValue: "", Required: true},
			},
			Tags:      []string{"explain", "learning"},
			Category:  "学习",
			IsBuiltIn: true,
		},
		{
			ID:          "refactor",
			Name:        "重构建议",
			Description: "分析代码并提供重构建议",
			Content: strings.Join([]string{
				"请分析以下代码并提供重构建议：",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"重点关注：",
				"1. 代码异味识别",
				"2. 设计模式应用",
				"3. 重复代码消除",
				"4. 复杂度降低",
				"5. 可测试性改进",
				"",
				"请提供重构后的代码示例。",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "code", Description: "要重构的代码", DefaultValue: "", Required: true},
			},
			Tags:      []string{"refactor", "quality"},
			Category:  "代码质量",
			IsBuiltIn: true,
		},
		{
			ID:          "write-tests",
			Name:        "编写测试",
			Description: "为代码生成测试用例",
			Content: strings.Join([]string{
				"请为以下代码编写测试用例：",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"要求：",
				"1. 覆盖正常情况",
				"2. 覆盖边界情况",
				"3. 覆盖错误情况",
				"4. 使用表驱动测试（如适用）",
				"5. 添加清晰的测试描述",
				"",
				"测试框架：{{testFramework}}",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "code", Description: "要测试的代码", DefaultValue: "", Required: true},
				{Name: "testFramework", Description: "测试框架", DefaultValue: "testing", Required: false},
			},
			Tags:      []string{"test", "testing"},
			Category:  "测试",
			IsBuiltIn: true,
		},
		{
			ID:          "debug",
			Name:        "调试帮助",
			Description: "分析代码中的问题并提供解决方案",
			Content: strings.Join([]string{
				"我在以下代码中遇到了问题：",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"错误信息：",
				"{{error}}",
				"",
				"请帮我：",
				"1. 分析错误原因",
				"2. 提供解决方案",
				"3. 解释为什么会出现这个问题",
				"4. 如何避免类似问题",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "code", Description: "有问题的代码", DefaultValue: "", Required: true},
				{Name: "error", Description: "错误信息", DefaultValue: "", Required: false},
			},
			Tags:      []string{"debug", "error"},
			Category:  "调试",
			IsBuiltIn: true,
		},
		{
			ID:          "implement-feature",
			Name:        "功能实现",
			Description: "根据需求描述实现功能",
			Content: strings.Join([]string{
				"请帮我实现以下功能：",
				"",
				"需求描述：",
				"{{requirement}}",
				"",
				"技术要求：",
				"- 语言：{{language}}",
				"- 框架：{{framework}}",
				"",
				"请提供：",
				"1. 实现思路",
				"2. 完整代码",
				"3. 使用示例",
				"4. 注意事项",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "requirement", Description: "需求描述", DefaultValue: "", Required: true},
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "framework", Description: "使用的框架", DefaultValue: "", Required: false},
			},
			Tags:      []string{"implement", "feature"},
			Category:  "开发",
			IsBuiltIn: true,
		},
		{
			ID:          "api-docs",
			Name:        "API 文档生成",
			Description: "为 API 生成文档",
			Content: strings.Join([]string{
				"请为以下 API 生成文档：",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"文档格式：{{format}}",
				"",
				"请包括：",
				"1. API 端点描述",
				"2. 请求参数",
				"3. 响应格式",
				"4. 示例请求和响应",
				"5. 错误码说明",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "code", Description: "API 代码", DefaultValue: "", Required: true},
				{Name: "format", Description: "文档格式", DefaultValue: "markdown", Required: false},
			},
			Tags:      []string{"docs", "api"},
			Category:  "文档",
			IsBuiltIn: true,
		},
		{
			ID:          "commit-message",
			Name:        "提交信息生成",
			Description: "根据代码变更生成提交信息",
			Content: strings.Join([]string{
				"根据以下代码变更生成提交信息：",
				"",
				"{{diff}}",
				"",
				"请生成：",
				"1. 简洁的标题（50字符以内）",
				"2. 详细的描述（如有必要）",
				"3. 遵循 Conventional Commits 规范",
				"",
				"格式：",
				"<type>(<scope>): <subject>",
				"",
				"<body>",
				"",
				"<footer>",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "diff", Description: "代码变更", DefaultValue: "", Required: true},
			},
			Tags:      []string{"git", "commit"},
			Category:  "Git",
			IsBuiltIn: true,
		},
		{
			ID:          "sql-optimization",
			Name:        "SQL 优化",
			Description: "分析并优化 SQL 查询",
			Content: strings.Join([]string{
				"请分析并优化以下 SQL 查询：",
				"",
				codeBlock + "sql",
				"{{sql}}",
				codeBlock,
				"",
				"数据库类型：{{dbType}}",
				"表结构：",
				"{{schema}}",
				"",
				"请提供：",
				"1. 性能问题分析",
				"2. 索引建议",
				"3. 优化后的 SQL",
				"4. 优化效果预估",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "sql", Description: "SQL 查询", DefaultValue: "", Required: true},
				{Name: "dbType", Description: "数据库类型", DefaultValue: "PostgreSQL", Required: false},
				{Name: "schema", Description: "表结构", DefaultValue: "（未提供）", Required: false},
			},
			Tags:      []string{"sql", "optimization"},
			Category:  "数据库",
			IsBuiltIn: true,
		},
		{
			ID:          "security-audit",
			Name:        "安全审计",
			Description: "对代码进行安全审计",
			Content: strings.Join([]string{
				"请对以下代码进行安全审计：",
				"",
				codeBlock + "{{language}}",
				"{{code}}",
				codeBlock,
				"",
				"审计重点：",
				"1. OWASP Top 10 漏洞",
				"2. 输入验证问题",
				"3. 认证授权问题",
				"4. 敏感数据处理",
				"5. 注入漏洞",
				"",
				"请提供：",
				"- 漏洞列表（按严重程度排序）",
				"- 每个漏洞的详细说明",
				"- 修复建议和示例代码",
			}, "\n"),
			Variables: []TemplateVariable{
				{Name: "language", Description: "编程语言", DefaultValue: "go", Required: true},
				{Name: "code", Description: "要审计的代码", DefaultValue: "", Required: true},
			},
			Tags:      []string{"security", "audit"},
			Category:  "安全",
			IsBuiltIn: true,
		},
	}
}

var builtInTemplates = getBuiltInTemplates()

func NewTemplateManager() *TemplateManager {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".smartclaw", "templates")

	tm := &TemplateManager{
		templates:  make(map[string]*PromptTemplate),
		configPath: configPath,
	}

	for _, template := range builtInTemplates {
		tm.templates[template.ID] = template
	}

	tm.loadCustomTemplates()

	return tm
}

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

		var template PromptTemplate
		if err := json.Unmarshal(data, &template); err != nil {
			continue
		}

		template.IsBuiltIn = false
		tm.templates[template.ID] = &template
	}
}

func (tm *TemplateManager) GetTemplate(id string) (*PromptTemplate, error) {
	template, exists := tm.templates[id]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return template, nil
}

func (tm *TemplateManager) ListTemplates() []*PromptTemplate {
	var templates []*PromptTemplate
	for _, template := range tm.templates {
		templates = append(templates, template)
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].IsBuiltIn != templates[j].IsBuiltIn {
			return templates[i].IsBuiltIn
		}
		return templates[i].Name < templates[j].Name
	})
	return templates
}

func (tm *TemplateManager) ListTemplatesByCategory() map[string][]*PromptTemplate {
	result := make(map[string][]*PromptTemplate)
	for _, template := range tm.templates {
		category := template.Category
		if category == "" {
			category = "其他"
		}
		result[category] = append(result[category], template)
	}
	return result
}

func (tm *TemplateManager) CreateTemplate(template *PromptTemplate) error {
	if template.ID == "" {
		return fmt.Errorf("template ID cannot be empty")
	}

	if _, exists := tm.templates[template.ID]; exists {
		return fmt.Errorf("template already exists: %s", template.ID)
	}

	template.IsBuiltIn = false
	tm.templates[template.ID] = template

	return tm.saveTemplate(template)
}

func (tm *TemplateManager) UpdateTemplate(id string, template *PromptTemplate) error {
	existing, exists := tm.templates[id]
	if !exists {
		return fmt.Errorf("template not found: %s", id)
	}

	if existing.IsBuiltIn {
		return fmt.Errorf("cannot modify built-in template: %s", id)
	}

	template.ID = id
	template.IsBuiltIn = false
	tm.templates[id] = template

	return tm.saveTemplate(template)
}

func (tm *TemplateManager) DeleteTemplate(id string) error {
	template, exists := tm.templates[id]
	if !exists {
		return fmt.Errorf("template not found: %s", id)
	}

	if template.IsBuiltIn {
		return fmt.Errorf("cannot delete built-in template: %s", id)
	}

	delete(tm.templates, id)

	filePath := filepath.Join(tm.configPath, id+".json")
	return os.Remove(filePath)
}

func (tm *TemplateManager) saveTemplate(template *PromptTemplate) error {
	if tm.configPath == "" {
		return fmt.Errorf("config path not set")
	}

	if err := os.MkdirAll(tm.configPath, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(tm.configPath, template.ID+".json")
	return os.WriteFile(filePath, data, 0644)
}

func (tm *TemplateManager) RenderTemplate(id string, variables map[string]string) (string, error) {
	template, err := tm.GetTemplate(id)
	if err != nil {
		return "", err
	}

	content := template.Content

	for _, v := range template.Variables {
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

func (tm *TemplateManager) GetTemplateVariables(id string) ([]TemplateVariable, error) {
	template, err := tm.GetTemplate(id)
	if err != nil {
		return nil, err
	}
	return template.Variables, nil
}

func (tm *TemplateManager) SearchTemplates(query string) []*PromptTemplate {
	query = strings.ToLower(query)
	var results []*PromptTemplate

	for _, template := range tm.templates {
		if strings.Contains(strings.ToLower(template.Name), query) ||
			strings.Contains(strings.ToLower(template.Description), query) ||
			strings.Contains(strings.ToLower(template.Content), query) {
			results = append(results, template)
			continue
		}

		for _, tag := range template.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, template)
				break
			}
		}
	}

	return results
}

func (tm *TemplateManager) FormatTemplateList() string {
	var sb strings.Builder

	sb.WriteString("╭──────────────────────────────────────────────────────────╮\n")
	sb.WriteString("│                   📝 提示词模板列表                        │\n")
	sb.WriteString("╰──────────────────────────────────────────────────────────╯\n\n")

	templatesByCategory := tm.ListTemplatesByCategory()
	var categories []string
	for cat := range templatesByCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, category := range categories {
		templates := templatesByCategory[category]
		sb.WriteString(fmt.Sprintf("◆ %s\n", category))
		for _, template := range templates {
			builtIn := ""
			if template.IsBuiltIn {
				builtIn = " [内置]"
			}
			sb.WriteString(fmt.Sprintf("  %-20s %s%s\n",
				template.ID,
				template.Description,
				builtIn))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("使用方法:\n")
	sb.WriteString("  /template use <id>          - 使用模板\n")
	sb.WriteString("  /template info <id>         - 查看模板详情\n")
	sb.WriteString("  /template create <id>       - 创建自定义模板\n")
	sb.WriteString("  /template delete <id>       - 删除自定义模板\n")

	return sb.String()
}

func (tm *TemplateManager) FormatTemplateInfo(template *PromptTemplate) string {
	var sb strings.Builder

	sb.WriteString("╭─────────────────────────────────────────────────╮\n")
	sb.WriteString(fmt.Sprintf("│  %-45s │\n", template.Name))
	sb.WriteString("╰─────────────────────────────────────────────────╯\n\n")

	sb.WriteString(fmt.Sprintf("  ID:          %s\n", template.ID))
	sb.WriteString(fmt.Sprintf("  描述:        %s\n", template.Description))
	if template.Category != "" {
		sb.WriteString(fmt.Sprintf("  分类:        %s\n", template.Category))
	}
	if len(template.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("  标签:        %s\n", strings.Join(template.Tags, ", ")))
	}
	if template.Author != "" {
		sb.WriteString(fmt.Sprintf("  作者:        %s\n", template.Author))
	}
	sb.WriteString(fmt.Sprintf("  类型:        %s\n", map[bool]string{true: "内置", false: "自定义"}[template.IsBuiltIn]))

	if len(template.Variables) > 0 {
		sb.WriteString("\n  变量:\n")
		for _, v := range template.Variables {
			required := ""
			if v.Required {
				required = " (必填)"
			}
			sb.WriteString(fmt.Sprintf("    - %s: %s%s\n", v.Name, v.Description, required))
			if v.DefaultValue != "" {
				sb.WriteString(fmt.Sprintf("      默认值: %s\n", v.DefaultValue))
			}
		}
	}

	sb.WriteString("\n  模板内容:\n")
	sb.WriteString("  ─────────────────────────────────────────────\n")
	lines := strings.Split(template.Content, "\n")
	for _, line := range lines {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}
	sb.WriteString("  ─────────────────────────────────────────────\n")

	return sb.String()
}

func (tm *TemplateManager) ExportTemplate(id string, format string) (string, error) {
	template, err := tm.GetTemplate(id)
	if err != nil {
		return "", err
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(template, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "markdown", "md":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", template.Name))
		sb.WriteString(fmt.Sprintf("**描述**: %s\n\n", template.Description))
		if template.Category != "" {
			sb.WriteString(fmt.Sprintf("**分类**: %s\n\n", template.Category))
		}
		if len(template.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("**标签**: %s\n\n", strings.Join(template.Tags, ", ")))
		}
		if len(template.Variables) > 0 {
			sb.WriteString("## 变量\n\n")
			for _, v := range template.Variables {
				required := ""
				if v.Required {
					required = " (必填)"
				}
				sb.WriteString(fmt.Sprintf("- `{{%s}}`: %s%s\n", v.Name, v.Description, required))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("## 模板内容\n\n```\n")
		sb.WriteString(template.Content)
		sb.WriteString("\n```\n")
		return sb.String(), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func (tm *TemplateManager) ImportTemplate(data string, format string) error {
	var template PromptTemplate

	switch format {
	case "json":
		if err := json.Unmarshal([]byte(data), &template); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return tm.CreateTemplate(&template)
}
