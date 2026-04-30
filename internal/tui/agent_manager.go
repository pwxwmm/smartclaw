package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type AgentSource string

const (
	AgentSourceBuiltIn         AgentSource = "built-in"
	AgentSourceUserSettings    AgentSource = "userSettings"
	AgentSourceProjectSettings AgentSource = "projectSettings"
	AgentSourcePlugin          AgentSource = "plugin"
)

type PermissionMode string

const (
	PermissionModeAsk              PermissionMode = "ask"
	PermissionModeReadOnly         PermissionMode = "read-only"
	PermissionModeWorkspaceWrite   PermissionMode = "workspace-write"
	PermissionModeDangerFullAccess PermissionMode = "danger-full-access"
)

type AgentMemoryScope string

const (
	AgentMemoryUser    AgentMemoryScope = "user"
	AgentMemoryProject AgentMemoryScope = "project"
	AgentMemoryLocal   AgentMemoryScope = "local"
)

type AgentIsolationMode string

const (
	AgentIsolationWorktree AgentIsolationMode = "worktree"
	AgentIsolationRemote   AgentIsolationMode = "remote"
)

type McpServerSpec struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type HookConfig struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Tools   []string `json:"tools"`
}

type HooksSettings map[string][]HookConfig

type AgentDefinition struct {
	AgentType          string             `json:"agentType"`
	WhenToUse          string             `json:"whenToUse"`
	Tools              []string           `json:"tools,omitempty"`
	DisallowedTools    []string           `json:"disallowedTools,omitempty"`
	Skills             []string           `json:"skills,omitempty"`
	McpServers         []McpServerSpec    `json:"mcpServers,omitempty"`
	Hooks              HooksSettings      `json:"hooks,omitempty"`
	Color              string             `json:"color,omitempty"`
	Model              string             `json:"model,omitempty"`
	Effort             string             `json:"effort,omitempty"`
	PermissionMode     PermissionMode     `json:"permissionMode,omitempty"`
	MaxTurns           int                `json:"maxTurns,omitempty"`
	Filename           string             `json:"filename,omitempty"`
	BaseDir            string             `json:"baseDir,omitempty"`
	InitialPrompt      string             `json:"initialPrompt,omitempty"`
	Memory             AgentMemoryScope   `json:"memory,omitempty"`
	Isolation          AgentIsolationMode `json:"isolation,omitempty"`
	Background         bool               `json:"background,omitempty"`
	Source             AgentSource        `json:"source"`
	SystemPrompt       string             `json:"systemPrompt"`
	RequiredMcpServers []string           `json:"requiredMcpServers,omitempty"`
	OmitClaudeMd       bool               `json:"omitClaudeMd,omitempty"`
}

type AgentManager struct {
	mu           sync.RWMutex
	agents       map[string]*AgentDefinition
	currentAgent *AgentDefinition
	configPath   string
	projectDir   string
	onAgentSwitch func(agentType string) error
}

func NewAgentManager(projectDir string) *AgentManager {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".smartclaw", "agents")

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: configPath,
		projectDir: projectDir,
	}

	am.loadBuiltInAgents()
	am.loadCustomAgents()
	am.loadProjectAgents()

	if general, exists := am.agents["general-purpose"]; exists {
		am.currentAgent = general
	} else if len(am.agents) > 0 {
		for _, agent := range am.agents {
			am.currentAgent = agent
			break
		}
	}

	return am
}

func (am *AgentManager) loadBuiltInAgents() {
	builtInAgents := []*AgentDefinition{
		{
			AgentType:    "general-purpose",
			WhenToUse:    "General purpose agent for any task",
			Source:       AgentSourceBuiltIn,
			Color:        "blue",
			SystemPrompt: "You are a helpful AI assistant. You can help with a wide variety of tasks including coding, analysis, writing, and problem-solving. Be thorough but concise in your responses.",
		},
		{
			AgentType:    "explore",
			WhenToUse:    "Explore codebases, find patterns, understand architecture",
			Source:       AgentSourceBuiltIn,
			Color:        "green",
			Tools:        []string{"read", "glob", "grep", "lsp_symbols", "lsp_find_references", "lsp_goto_definition"},
			SystemPrompt: "You are an exploration agent specialized in understanding codebases. Your job is to search, navigate, and analyze code to answer questions about architecture, patterns, and implementations. Focus on finding relevant files and understanding relationships between components. Be efficient - use grep and glob to quickly locate relevant code. Summarize your findings clearly.",
			OmitClaudeMd: true,
		},
		{
			AgentType:    "plan",
			WhenToUse:    "Plan implementation strategies, break down complex tasks",
			Source:       AgentSourceBuiltIn,
			Color:        "yellow",
			Tools:        []string{"read", "glob", "grep", "lsp_symbols"},
			SystemPrompt: "You are a planning agent specialized in breaking down complex tasks into actionable steps. Analyze requirements, identify dependencies, and create clear implementation plans. Consider edge cases, error handling, and testing strategies. Output structured plans with numbered steps. Do not implement code - only plan and advise.",
			OmitClaudeMd: true,
		},
		{
			AgentType:    "code-review",
			WhenToUse:    "Review code for quality, security, and best practices",
			Source:       AgentSourceBuiltIn,
			Color:        "purple",
			Tools:        []string{"read", "grep", "lsp_find_references", "ast_grep_search"},
			SystemPrompt: "You are a code review specialist. Analyze code for:\n- Security vulnerabilities (SQL injection, XSS, auth issues)\n- Performance problems\n- Code smells and anti-patterns\n- Best practice violations\n- Test coverage gaps\nProvide specific, actionable feedback with line references. Prioritize issues by severity.",
		},
		{
			AgentType:    "test-engineer",
			WhenToUse:    "Design and implement test strategies",
			Source:       AgentSourceBuiltIn,
			Color:        "cyan",
			Tools:        []string{"read", "write", "edit", "glob", "grep"},
			SystemPrompt: "You are a test engineering specialist. Design comprehensive test strategies including:\n- Unit tests\n- Integration tests\n- Edge case identification\n- Test coverage analysis\n- Mock/stub strategies\nWrite clear, maintainable test code. Focus on meaningful test cases that catch real bugs.",
		},
		{
			AgentType:    "devops",
			WhenToUse:    "CI/CD, containerization, infrastructure automation",
			Source:       AgentSourceBuiltIn,
			Color:        "orange",
			Tools:        []string{"bash", "read", "write", "edit", "glob"},
			SystemPrompt: "You are a DevOps specialist. Help with:\n- CI/CD pipeline configuration\n- Docker/containerization\n- Kubernetes deployments\n- Infrastructure as Code (Terraform, Ansible)\n- Monitoring and logging setup\n- Security hardening\nProvide production-ready configurations with best practices.",
		},
		{
			AgentType:    "security",
			WhenToUse:    "Security audit, vulnerability assessment",
			Source:       AgentSourceBuiltIn,
			Color:        "red",
			Tools:        []string{"read", "grep", "ast_grep_search"},
			SystemPrompt: "You are a security specialist. Perform security audits focusing on:\n- OWASP Top 10 vulnerabilities\n- Authentication and authorization flaws\n- Sensitive data exposure\n- Injection vulnerabilities\n- Security misconfiguration\n- Cryptographic weaknesses\nProvide severity ratings and remediation steps for each finding.",
		},
		{
			AgentType:    "architect",
			WhenToUse:    "System architecture design and technical decisions",
			Source:       AgentSourceBuiltIn,
			Color:        "magenta",
			Tools:        []string{"read", "glob", "grep", "lsp_symbols"},
			SystemPrompt: "You are a software architect. Help with:\n- System design and architecture patterns\n- Technology selection and trade-offs\n- Scalability and performance considerations\n- API design\n- Data modeling\n- Integration patterns\nProvide clear architectural diagrams (in ASCII or mermaid) and justify design decisions.",
		},
		{
			AgentType:    "refactor",
			WhenToUse:    "Code refactoring and technical debt reduction",
			Source:       AgentSourceBuiltIn,
			Color:        "teal",
			Tools:        []string{"read", "edit", "glob", "grep", "ast_grep_search", "ast_grep_replace"},
			SystemPrompt: "You are a refactoring specialist. Help improve code quality by:\n- Identifying code smells and anti-patterns\n- Applying design patterns appropriately\n- Improving code readability and maintainability\n- Reducing complexity and duplication\n- Modernizing legacy code\nMake incremental, safe refactoring changes with clear justification.",
		},
		{
			AgentType:    "docs",
			WhenToUse:    "Documentation generation and improvement",
			Source:       AgentSourceBuiltIn,
			Color:        "gray",
			Tools:        []string{"read", "write", "edit", "glob", "grep"},
			SystemPrompt: "You are a documentation specialist. Help with:\n- API documentation\n- README files\n- Code comments\n- User guides\n- Architecture documentation\n- Changelog maintenance\nWrite clear, concise, well-structured documentation. Follow documentation best practices.",
		},
	}

	for _, agent := range builtInAgents {
		am.agents[agent.AgentType] = agent
	}
}

func (am *AgentManager) loadCustomAgents() {
	if am.configPath == "" {
		return
	}

	files, err := filepath.Glob(filepath.Join(am.configPath, "*.md"))
	if err != nil {
		return
	}

	for _, file := range files {
		agent := am.parseAgentFromMarkdown(file, AgentSourceUserSettings)
		if agent != nil {
			am.agents[agent.AgentType] = agent
		}
	}
}

func (am *AgentManager) loadProjectAgents() {
	if am.projectDir == "" {
		return
	}

	projectAgentsDir := filepath.Join(am.projectDir, ".smartclaw", "agents")
	files, err := filepath.Glob(filepath.Join(projectAgentsDir, "*.md"))
	if err != nil {
		return
	}

	for _, file := range files {
		agent := am.parseAgentFromMarkdown(file, AgentSourceProjectSettings)
		if agent != nil {
			am.agents[agent.AgentType] = agent
		}
	}
}

func (am *AgentManager) parseAgentFromMarkdown(filePath string, source AgentSource) *AgentDefinition {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	content := string(data)
	frontmatter, body := parseFrontmatter(content)

	name, ok := frontmatter["name"].(string)
	if !ok || name == "" {
		return nil
	}

	description, _ := frontmatter["description"].(string)
	if description == "" {
		return nil
	}

	agent := &AgentDefinition{
		AgentType:    name,
		WhenToUse:    description,
		Source:       source,
		SystemPrompt: strings.TrimSpace(body),
		Filename:     strings.TrimSuffix(filepath.Base(filePath), ".md"),
		BaseDir:      filepath.Dir(filePath),
	}

	if color, ok := frontmatter["color"].(string); ok {
		agent.Color = color
	}
	if model, ok := frontmatter["model"].(string); ok {
		agent.Model = model
	}
	if effort, ok := frontmatter["effort"].(string); ok {
		agent.Effort = effort
	}
	if permMode, ok := frontmatter["permissionMode"].(string); ok {
		agent.PermissionMode = PermissionMode(permMode)
	}
	if initialPrompt, ok := frontmatter["initialPrompt"].(string); ok {
		agent.InitialPrompt = initialPrompt
	}
	if memory, ok := frontmatter["memory"].(string); ok {
		agent.Memory = AgentMemoryScope(memory)
	}
	if background, ok := frontmatter["background"].(bool); ok {
		agent.Background = background
	}
	if maxTurns, ok := frontmatter["maxTurns"].(int); ok {
		agent.MaxTurns = maxTurns
	}

	if tools, ok := frontmatter["tools"].([]any); ok {
		for _, t := range tools {
			if s, ok := t.(string); ok {
				agent.Tools = append(agent.Tools, s)
			}
		}
	}
	if disallowedTools, ok := frontmatter["disallowedTools"].([]any); ok {
		for _, t := range disallowedTools {
			if s, ok := t.(string); ok {
				agent.DisallowedTools = append(agent.DisallowedTools, s)
			}
		}
	}
	if skills, ok := frontmatter["skills"].([]any); ok {
		for _, s := range skills {
			if str, ok := s.(string); ok {
				agent.Skills = append(agent.Skills, str)
			}
		}
	}

	return agent
}

func parseFrontmatter(content string) (map[string]any, string) {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || lines[0] != "---" {
		return make(map[string]any), content
	}

	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return make(map[string]any), content
	}

	frontmatterStr := strings.Join(lines[1:endIndex], "\n")
	body := strings.Join(lines[endIndex+1:], "\n")

	frontmatter := make(map[string]any)
	for _, line := range strings.Split(frontmatterStr, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				items := strings.Split(strings.Trim(value, "[]"), ",")
				var arr []any
				for _, item := range items {
					arr = append(arr, strings.TrimSpace(strings.Trim(item, "\"")))
				}
				frontmatter[key] = arr
			} else if value == "true" {
				frontmatter[key] = true
			} else if value == "false" {
				frontmatter[key] = false
			} else {
				frontmatter[key] = strings.Trim(value, "\"")
			}
		}
	}

	return frontmatter, body
}

func (am *AgentManager) GetCurrentAgent() *AgentDefinition {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.currentAgent
}

func (am *AgentManager) SetCurrentAgent(agentType string) error {
	am.mu.Lock()
	agent, exists := am.agents[agentType]
	if !exists {
		am.mu.Unlock()
		return fmt.Errorf("agent not found: %s", agentType)
	}
	am.currentAgent = agent
	callback := am.onAgentSwitch
	am.mu.Unlock()

	if callback != nil {
		if err := callback(agentType); err != nil {
			return fmt.Errorf("agent switch callback failed: %w", err)
		}
	}
	return nil
}

func (am *AgentManager) SetOnAgentSwitch(fn func(agentType string) error) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.onAgentSwitch = fn
}

func (am *AgentManager) GetAgent(agentType string) (*AgentDefinition, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	agent, exists := am.agents[agentType]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentType)
	}
	return agent, nil
}

func (am *AgentManager) ListAgents() []*AgentDefinition {
	am.mu.RLock()
	defer am.mu.RUnlock()
	var agents []*AgentDefinition
	for _, agent := range am.agents {
		agents = append(agents, agent)
	}
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Source != agents[j].Source {
			return agents[i].Source < agents[j].Source
		}
		return agents[i].AgentType < agents[j].AgentType
	})
	return agents
}

func (am *AgentManager) ListAgentsBySource() map[AgentSource][]*AgentDefinition {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make(map[AgentSource][]*AgentDefinition)
	for _, agent := range am.agents {
		result[agent.Source] = append(result[agent.Source], agent)
	}
	return result
}

func (am *AgentManager) CreateCustomAgent(agent *AgentDefinition) error {
	if agent.AgentType == "" {
		return fmt.Errorf("agent type cannot be empty")
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.agents[agent.AgentType]; exists {
		return fmt.Errorf("agent already exists: %s", agent.AgentType)
	}

	agent.Source = AgentSourceUserSettings
	am.agents[agent.AgentType] = agent

	return am.saveCustomAgent(agent)
}

func (am *AgentManager) UpdateCustomAgent(agentType string, updates *AgentDefinition) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	agent, exists := am.agents[agentType]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentType)
	}

	if agent.Source == AgentSourceBuiltIn {
		return fmt.Errorf("cannot modify built-in agent: %s", agentType)
	}

	if updates.WhenToUse != "" {
		agent.WhenToUse = updates.WhenToUse
	}
	if updates.SystemPrompt != "" {
		agent.SystemPrompt = updates.SystemPrompt
	}
	if updates.Tools != nil {
		agent.Tools = updates.Tools
	}
	if updates.DisallowedTools != nil {
		agent.DisallowedTools = updates.DisallowedTools
	}
	if updates.Model != "" {
		agent.Model = updates.Model
	}
	if updates.PermissionMode != "" {
		agent.PermissionMode = updates.PermissionMode
	}
	if updates.Color != "" {
		agent.Color = updates.Color
	}

	return am.saveCustomAgent(agent)
}

func (am *AgentManager) DeleteCustomAgent(agentType string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	agent, exists := am.agents[agentType]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentType)
	}

	if agent.Source == AgentSourceBuiltIn {
		return fmt.Errorf("cannot delete built-in agent: %s", agentType)
	}

	if am.currentAgent != nil && am.currentAgent.AgentType == agentType {
		if general, exists := am.agents["general-purpose"]; exists {
			am.currentAgent = general
		}
	}

	delete(am.agents, agentType)

	if agent.Filename != "" {
		filePath := filepath.Join(am.configPath, agent.Filename+".md")
		os.Remove(filePath)
	}

	return nil
}

func (am *AgentManager) saveCustomAgent(agent *AgentDefinition) error {
	if am.configPath == "" {
		return fmt.Errorf("config path not set")
	}

	if err := os.MkdirAll(am.configPath, 0755); err != nil {
		return err
	}

	filename := agent.Filename
	if filename == "" {
		filename = agent.AgentType
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", agent.AgentType))
	sb.WriteString(fmt.Sprintf("description: %s\n", agent.WhenToUse))
	if agent.Color != "" {
		sb.WriteString(fmt.Sprintf("color: %s\n", agent.Color))
	}
	if agent.Model != "" {
		sb.WriteString(fmt.Sprintf("model: %s\n", agent.Model))
	}
	if agent.PermissionMode != "" {
		sb.WriteString(fmt.Sprintf("permissionMode: %s\n", agent.PermissionMode))
	}
	if len(agent.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("tools: [%s]\n", strings.Join(agent.Tools, ", ")))
	}
	if len(agent.DisallowedTools) > 0 {
		sb.WriteString(fmt.Sprintf("disallowedTools: [%s]\n", strings.Join(agent.DisallowedTools, ", ")))
	}
	if len(agent.Skills) > 0 {
		sb.WriteString(fmt.Sprintf("skills: [%s]\n", strings.Join(agent.Skills, ", ")))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(agent.SystemPrompt)

	filePath := filepath.Join(am.configPath, filename+".md")
	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

func (am *AgentManager) GetSystemPrompt() string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	if am.currentAgent == nil {
		return ""
	}
	return am.currentAgent.SystemPrompt
}

func (am *AgentManager) FormatAgentInfo(agent *AgentDefinition) string {
	var sb strings.Builder

	sb.WriteString("╭─────────────────────────────────────────────────╮\n")
	sb.WriteString(fmt.Sprintf("│  %-45s │\n", agent.AgentType))
	sb.WriteString("╰─────────────────────────────────────────────────╯\n\n")

	sb.WriteString(fmt.Sprintf("  描述:         %s\n", agent.WhenToUse))
	sb.WriteString(fmt.Sprintf("  来源:         %s\n", agent.Source))
	if agent.Model != "" {
		sb.WriteString(fmt.Sprintf("  模型:         %s\n", agent.Model))
	}
	if agent.PermissionMode != "" {
		sb.WriteString(fmt.Sprintf("  权限模式:     %s\n", agent.PermissionMode))
	}
	if agent.Color != "" {
		sb.WriteString(fmt.Sprintf("  颜色:         %s\n", agent.Color))
	}
	if len(agent.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("  允许工具:     %s\n", strings.Join(agent.Tools, ", ")))
	}
	if len(agent.DisallowedTools) > 0 {
		sb.WriteString(fmt.Sprintf("  禁用工具:     %s\n", strings.Join(agent.DisallowedTools, ", ")))
	}
	if len(agent.Skills) > 0 {
		sb.WriteString(fmt.Sprintf("  技能:         %s\n", strings.Join(agent.Skills, ", ")))
	}
	if agent.MaxTurns > 0 {
		sb.WriteString(fmt.Sprintf("  最大轮次:     %d\n", agent.MaxTurns))
	}
	if agent.Memory != "" {
		sb.WriteString(fmt.Sprintf("  内存范围:     %s\n", agent.Memory))
	}

	sb.WriteString("\n  系统提示:\n")
	sb.WriteString("  ─────────────────────────────────────────────\n")
	lines := strings.Split(agent.SystemPrompt, "\n")
	for _, line := range lines {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}
	sb.WriteString("  ─────────────────────────────────────────────\n")

	return sb.String()
}

func (am *AgentManager) FormatAgentList() string {
	var sb strings.Builder

	sb.WriteString("╭──────────────────────────────────────────────────────────╮\n")
	sb.WriteString("│                    🤖 Agent 列表                          │\n")
	sb.WriteString("╰──────────────────────────────────────────────────────────╯\n\n")

	am.mu.RLock()
	agentsBySource := make(map[AgentSource][]*AgentDefinition)
	for _, agent := range am.agents {
		agentsBySource[agent.Source] = append(agentsBySource[agent.Source], agent)
	}
	currentAgent := am.currentAgent
	am.mu.RUnlock()

	sourceOrder := []AgentSource{AgentSourceBuiltIn, AgentSourceUserSettings, AgentSourceProjectSettings, AgentSourcePlugin}
	sourceNames := map[AgentSource]string{
		AgentSourceBuiltIn:         "内置 Agents",
		AgentSourceUserSettings:    "用户自定义",
		AgentSourceProjectSettings: "项目 Agents",
		AgentSourcePlugin:          "插件 Agents",
	}

	for _, source := range sourceOrder {
		agents := agentsBySource[source]
		if len(agents) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("◆ %s\n", sourceNames[source]))
		for _, agent := range agents {
			current := ""
			if currentAgent != nil && currentAgent.AgentType == agent.AgentType {
				current = " ✓"
			}
			sb.WriteString(fmt.Sprintf("  %-20s %s%s\n",
				agent.AgentType,
				agent.WhenToUse,
				current))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("使用方法:\n")
	sb.WriteString("  /agent switch <name>    - 切换到指定 Agent\n")
	sb.WriteString("  /agent info <name>      - 查看 Agent 详情\n")
	sb.WriteString("  /agent create           - 创建自定义 Agent\n")
	sb.WriteString("  /agent delete <name>    - 删除自定义 Agent\n")

	return sb.String()
}

func (am *AgentManager) ExportAgent(agentType string, format string) (string, error) {
	am.mu.RLock()
	agent, exists := am.agents[agentType]
	am.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("agent not found: %s", agentType)
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(agent, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "markdown", "md":
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("name: %s\n", agent.AgentType))
		sb.WriteString(fmt.Sprintf("description: %s\n", agent.WhenToUse))
		if agent.Color != "" {
			sb.WriteString(fmt.Sprintf("color: %s\n", agent.Color))
		}
		if agent.Model != "" {
			sb.WriteString(fmt.Sprintf("model: %s\n", agent.Model))
		}
		if agent.PermissionMode != "" {
			sb.WriteString(fmt.Sprintf("permissionMode: %s\n", agent.PermissionMode))
		}
		if len(agent.Tools) > 0 {
			sb.WriteString(fmt.Sprintf("tools: [%s]\n", strings.Join(agent.Tools, ", ")))
		}
		if len(agent.DisallowedTools) > 0 {
			sb.WriteString(fmt.Sprintf("disallowedTools: [%s]\n", strings.Join(agent.DisallowedTools, ", ")))
		}
		sb.WriteString("---\n\n")
		sb.WriteString(agent.SystemPrompt)
		return sb.String(), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func (am *AgentManager) ImportAgent(data string, format string) error {
	var agent AgentDefinition

	switch format {
	case "json":
		if err := json.Unmarshal([]byte(data), &agent); err != nil {
			return err
		}
	case "markdown", "md":
		frontmatter, body := parseFrontmatter(data)
		name, ok := frontmatter["name"].(string)
		if !ok || name == "" {
			return fmt.Errorf("missing agent name in frontmatter")
		}
		description, _ := frontmatter["description"].(string)
		agent.AgentType = name
		agent.WhenToUse = description
		agent.SystemPrompt = strings.TrimSpace(body)
		if color, ok := frontmatter["color"].(string); ok {
			agent.Color = color
		}
		if model, ok := frontmatter["model"].(string); ok {
			agent.Model = model
		}
		if permMode, ok := frontmatter["permissionMode"].(string); ok {
			agent.PermissionMode = PermissionMode(permMode)
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return am.CreateCustomAgent(&agent)
}
