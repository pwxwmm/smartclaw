package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/instructkr/smartclaw/internal/template"
)

const codeBlock = "```"

type PromptTemplate = template.Template
type TemplateVariable = template.Variable

type TemplateManager struct {
	core *template.TemplateManager
}

func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		core: template.NewTemplateManager(),
	}
}

func (tm *TemplateManager) GetTemplate(id string) (*PromptTemplate, error) {
	return tm.core.Get(id)
}

func (tm *TemplateManager) ListTemplates() []*PromptTemplate {
	return tm.core.List()
}

func (tm *TemplateManager) ListTemplatesByCategory() map[string][]*PromptTemplate {
	return tm.core.ListByCategory()
}

func (tm *TemplateManager) CreateTemplate(t *PromptTemplate) error {
	return tm.core.Create(t)
}

func (tm *TemplateManager) UpdateTemplate(id string, t *PromptTemplate) error {
	return tm.core.Update(id, t)
}

func (tm *TemplateManager) DeleteTemplate(id string) error {
	return tm.core.Delete(id)
}

func (tm *TemplateManager) RenderTemplate(id string, variables map[string]string) (string, error) {
	return tm.core.Render(id, variables)
}

func (tm *TemplateManager) GetTemplateVariables(id string) ([]TemplateVariable, error) {
	return tm.core.GetVariables(id)
}

func (tm *TemplateManager) SearchTemplates(query string) []*PromptTemplate {
	return tm.core.Search(query)
}

func (tm *TemplateManager) ExportTemplate(id string, format string) (string, error) {
	return tm.core.ExportTemplate(id, format)
}

func (tm *TemplateManager) ImportTemplate(data string, format string) error {
	return tm.core.ImportTemplate(data, format)
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
		for _, t := range templates {
			builtIn := ""
			if t.IsBuiltIn {
				builtIn = " [内置]"
			}
			sb.WriteString(fmt.Sprintf("  %-20s %s%s\n",
				t.ID,
				t.Description,
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

func (tm *TemplateManager) FormatTemplateInfo(t *PromptTemplate) string {
	var sb strings.Builder

	sb.WriteString("╭─────────────────────────────────────────────────╮\n")
	sb.WriteString(fmt.Sprintf("│  %-45s │\n", t.Name))
	sb.WriteString("╰─────────────────────────────────────────────────╯\n\n")

	sb.WriteString(fmt.Sprintf("  ID:          %s\n", t.ID))
	sb.WriteString(fmt.Sprintf("  描述:        %s\n", t.Description))
	if t.Category != "" {
		sb.WriteString(fmt.Sprintf("  分类:        %s\n", t.Category))
	}
	if len(t.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("  标签:        %s\n", strings.Join(t.Tags, ", ")))
	}
	if t.Author != "" {
		sb.WriteString(fmt.Sprintf("  作者:        %s\n", t.Author))
	}
	sb.WriteString(fmt.Sprintf("  类型:        %s\n", map[bool]string{true: "内置", false: "自定义"}[t.IsBuiltIn]))

	if len(t.Variables) > 0 {
		sb.WriteString("\n  变量:\n")
		for _, v := range t.Variables {
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
	lines := strings.Split(t.Content, "\n")
	for _, line := range lines {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}
	sb.WriteString("  ─────────────────────────────────────────────\n")

	return sb.String()
}
