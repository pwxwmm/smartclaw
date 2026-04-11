package skills

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/mcp"
)

type McpSkillBuilderConfig struct {
	ServerName string            `json:"server_name"`
	ToolFilter []string          `json:"tool_filter,omitempty"`
	Prefix     string            `json:"prefix,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Template   SkillTemplateType `json:"template"`
}

type SkillTemplateType string

const (
	TemplateDefault  SkillTemplateType = "default"
	TemplateCode     SkillTemplateType = "code"
	TemplateData     SkillTemplateType = "data"
	TemplateWorkflow SkillTemplateType = "workflow"
)

type McpSkillBuilderPipeline struct {
	mcpRegistry *mcp.McpRegistry
	builders    map[string]*McpSkillBuilderConfig
	skills      map[string]*Skill
	mu          sync.RWMutex
	manager     *SkillManager
}

func NewMcpSkillBuilderPipeline(mcpRegistry *mcp.McpRegistry, manager *SkillManager) *McpSkillBuilderPipeline {
	return &McpSkillBuilderPipeline{
		mcpRegistry: mcpRegistry,
		builders:    make(map[string]*McpSkillBuilderConfig),
		skills:      make(map[string]*Skill),
		manager:     manager,
	}
}

func (p *McpSkillBuilderPipeline) RegisterBuilder(config McpSkillBuilderConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.builders[config.ServerName] = &config
}

func (p *McpSkillBuilderPipeline) UnregisterBuilder(serverName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.builders, serverName)
}

func (p *McpSkillBuilderPipeline) BuildSkills(ctx context.Context) error {
	p.mu.RLock()
	builders := make([]*McpSkillBuilderConfig, 0, len(p.builders))
	for _, b := range p.builders {
		builders = append(builders, b)
	}
	p.mu.RUnlock()

	for _, builder := range builders {
		if err := p.buildSkillsFromServer(ctx, builder); err != nil {
			continue
		}
	}

	return nil
}

func (p *McpSkillBuilderPipeline) buildSkillsFromServer(ctx context.Context, config *McpSkillBuilderConfig) error {
	conn := p.mcpRegistry.Get(config.ServerName)
	if conn == nil {
		return fmt.Errorf("MCP server not found: %s", config.ServerName)
	}

	for _, tool := range conn.Tools {
		if len(config.ToolFilter) > 0 && !contains(config.ToolFilter, tool.Name) {
			continue
		}

		skillName := fmt.Sprintf("%s-%s", config.Prefix, tool.Name)
		if config.Prefix == "" {
			skillName = tool.Name
		}

		skill := p.generateSkillFromTool(skillName, tool, config)

		p.mu.Lock()
		p.skills[skillName] = skill
		p.mu.Unlock()

		if p.manager != nil {
			p.manager.mu.Lock()
			p.manager.skills[skillName] = skill
			p.manager.mu.Unlock()
		}
	}

	return nil
}

func (p *McpSkillBuilderPipeline) generateSkillFromTool(name string, tool mcp.McpTool, config *McpSkillBuilderConfig) *Skill {
	content := p.generateSkillContent(name, tool, config)

	return &Skill{
		Name:        name,
		Description: tool.Description,
		Content:     content,
		Tools:       []string{fmt.Sprintf("mcp:%s:%s", config.ServerName, tool.Name)},
		Tags:        append([]string{"mcp", config.ServerName}, config.Tags...),
		Source:      "mcp",
		Enabled:     true,
		LoadedAt:    time.Now(),
		Metadata: map[string]any{
			"server_name": config.ServerName,
			"tool_name":   tool.Name,
			"template":    string(config.Template),
		},
	}
}

func (p *McpSkillBuilderPipeline) generateSkillContent(name string, tool mcp.McpTool, config *McpSkillBuilderConfig) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Skill\n\n", name))
	sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description))
	sb.WriteString("## Triggers\n")
	sb.WriteString(fmt.Sprintf("- /%s\n\n", name))
	sb.WriteString("## Tools\n")
	sb.WriteString(fmt.Sprintf("- mcp:%s:%s\n\n", config.ServerName, tool.Name))

	if config.Template == TemplateCode {
		sb.WriteString("## Instructions\n\n")
		sb.WriteString("1. **Input Preparation**\n")
		sb.WriteString("   - Prepare the required parameters\n")
		sb.WriteString("   - Validate input format\n")
		sb.WriteString("   - Handle optional parameters\n\n")
		sb.WriteString("2. **Execution**\n")
		sb.WriteString("   - Call the MCP tool\n")
		sb.WriteString("   - Handle the response\n")
		sb.WriteString("   - Process results\n\n")
		sb.WriteString("3. **Output Processing**\n")
		sb.WriteString("   - Format the output\n")
		sb.WriteString("   - Handle errors\n")
		sb.WriteString("   - Provide feedback\n\n")
	}

	if config.Template == TemplateWorkflow {
		sb.WriteString("## Workflow\n\n")
		sb.WriteString("```mermaid\n")
		sb.WriteString("graph TD\n")
		sb.WriteString("    A[Start] --> B[Prepare Input]\n")
		sb.WriteString("    B --> C[Execute Tool]\n")
		sb.WriteString("    C --> D{Success?}\n")
		sb.WriteString("    D -->|Yes| E[Process Output]\n")
		sb.WriteString("    D -->|No| F[Handle Error]\n")
		sb.WriteString("    E --> G[End]\n")
		sb.WriteString("    F --> G\n")
		sb.WriteString("```\n\n")
	}

	sb.WriteString("## Tags\n")
	for _, tag := range config.Tags {
		sb.WriteString(fmt.Sprintf("- %s\n", tag))
	}
	sb.WriteString("- mcp\n")

	return sb.String()
}

func (p *McpSkillBuilderPipeline) GetBuiltSkills() []*Skill {
	p.mu.RLock()
	defer p.mu.RUnlock()

	skills := make([]*Skill, 0, len(p.skills))
	for _, s := range p.skills {
		skills = append(skills, s)
	}
	return skills
}

func (p *McpSkillBuilderPipeline) GetBuiltSkill(name string) *Skill {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.skills[name]
}

func (p *McpSkillBuilderPipeline) RefreshSkills(ctx context.Context) error {
	p.mu.Lock()
	p.skills = make(map[string]*Skill)
	p.mu.Unlock()

	return p.BuildSkills(ctx)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

var defaultPipeline *McpSkillBuilderPipeline

func InitMcpSkillBuilderPipeline(mcpRegistry *mcp.McpRegistry, manager *SkillManager) {
	defaultPipeline = NewMcpSkillBuilderPipeline(mcpRegistry, manager)
}

func GetMcpSkillBuilderPipeline() *McpSkillBuilderPipeline {
	return defaultPipeline
}

func RegisterMcpSkillBuilder(config McpSkillBuilderConfig) error {
	if defaultPipeline == nil {
		return fmt.Errorf("MCP skill builder pipeline not initialized")
	}
	defaultPipeline.RegisterBuilder(config)
	return nil
}

func BuildMcpSkills(ctx context.Context) error {
	if defaultPipeline == nil {
		return fmt.Errorf("MCP skill builder pipeline not initialized")
	}
	return defaultPipeline.BuildSkills(ctx)
}
