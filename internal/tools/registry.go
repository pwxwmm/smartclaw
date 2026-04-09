package tools

import (
	"context"
	"fmt"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input map[string]interface{}) (interface{}, error)
}

type BaseTool struct {
	name        string
	description string
	inputSchema map[string]interface{}
}

func (t *BaseTool) Name() string                        { return t.name }
func (t *BaseTool) Description() string                 { return t.description }
func (t *BaseTool) InputSchema() map[string]interface{} { return t.inputSchema }

type ToolRegistry struct {
	tools map[string]Tool
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Unregister(name string) {
	delete(r.tools, name)
}

func (r *ToolRegistry) Get(name string) Tool {
	return r.tools[name]
}

func (r *ToolRegistry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

func (r *ToolRegistry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func (r *ToolRegistry) Execute(ctx context.Context, name string, input map[string]interface{}) (interface{}, error) {
	tool := r.Get(name)
	if tool == nil {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, input)
}

var defaultRegistry *ToolRegistry

func init() {
	defaultRegistry = NewRegistry()
	RegisterDefaultTools()
}

func RegisterDefaultTools() {
	defaultRegistry.Register(&BashTool{})
	defaultRegistry.Register(&ReadFileTool{})
	defaultRegistry.Register(&WriteFileTool{})
	defaultRegistry.Register(&EditFileTool{})
	defaultRegistry.Register(&GlobTool{})
	defaultRegistry.Register(&GrepTool{})
	defaultRegistry.Register(&WebFetchTool{})
	defaultRegistry.Register(&WebSearchTool{})
	defaultRegistry.Register(&LSPTool{})
	defaultRegistry.Register(&SessionTool{})
	defaultRegistry.Register(&AgentTool{})
	defaultRegistry.Register(&TodoWriteTool{})
	defaultRegistry.Register(&AskUserQuestionTool{})
	defaultRegistry.Register(&ConfigTool{})
	defaultRegistry.Register(&NotebookEditTool{})
	defaultRegistry.Register(&SkillTool{})
	defaultRegistry.Register(&ASTGrepTool{})
	defaultRegistry.Register(&CodeSearchTool{})
	defaultRegistry.Register(&ImageTool{})
	defaultRegistry.Register(&PDFTool{})
	defaultRegistry.Register(&AudioTool{})
	defaultRegistry.Register(&BatchTool{})
	defaultRegistry.Register(&ParallelTool{})
	defaultRegistry.Register(&PipelineTool{})
	defaultRegistry.Register(&PowerShellTool{})
	defaultRegistry.Register(&McpTool{})
	defaultRegistry.Register(&ListMcpResourcesTool{})
	defaultRegistry.Register(&ReadMcpResourceTool{})
	defaultRegistry.Register(&ScheduleCronTool{})
	defaultRegistry.Register(&RemoteTriggerTool{})
	defaultRegistry.Register(&SendMessageTool{})
	defaultRegistry.Register(&EnterWorktreeTool{})
	defaultRegistry.Register(&ExitWorktreeTool{})
	defaultRegistry.Register(&BriefTool{})
	defaultRegistry.Register(&SleepTool{})
	defaultRegistry.Register(&ToolSearchTool{})
	defaultRegistry.Register(&REPLTool{})
	defaultRegistry.Register(&TeamCreateTool{})
	defaultRegistry.Register(&TeamDeleteTool{})
	defaultRegistry.Register(&EnterPlanModeTool{})
	defaultRegistry.Register(&SyntheticOutputTool{})
	defaultRegistry.Register(&ExitPlanModeTool{})
	defaultRegistry.Register(&McpAuthTool{})
	defaultRegistry.Register(&BrowseTool{})
	defaultRegistry.Register(&AttachTool{})
	defaultRegistry.Register(&DebugTool{})
	defaultRegistry.Register(&IndexTool{})
	defaultRegistry.Register(&CacheTool{})
	defaultRegistry.Register(&ObserveTool{})
	defaultRegistry.Register(&LazyTool{})
	defaultRegistry.Register(&ThinkTool{})
	defaultRegistry.Register(&DeepThinkTool{})
	defaultRegistry.Register(&ForkTool{})
	defaultRegistry.Register(&EnvTool{})
}

func GetRegistry() *ToolRegistry {
	return defaultRegistry
}

func Register(tool Tool) {
	defaultRegistry.Register(tool)
}

func Get(name string) Tool {
	return defaultRegistry.Get(name)
}

func All() []Tool {
	return defaultRegistry.All()
}

func Execute(ctx context.Context, name string, input map[string]interface{}) (interface{}, error) {
	return defaultRegistry.Execute(ctx, name, input)
}
