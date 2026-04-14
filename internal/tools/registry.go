package tools

import (
	"context"
	"fmt"
	"time"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Execute(ctx context.Context, input map[string]any) (any, error)
}

type BaseTool struct {
	name        string
	description string
	inputSchema map[string]any
}

func (t *BaseTool) Name() string                { return t.name }
func (t *BaseTool) Description() string         { return t.description }
func (t *BaseTool) InputSchema() map[string]any { return t.inputSchema }

func NewBaseTool(name, description string, inputSchema map[string]any) BaseTool {
	return BaseTool{name: name, description: description, inputSchema: inputSchema}
}

type ToolRegistry struct {
	tools          map[string]Tool
	cache          *ResultCache
	chainOptimizer *ChainOptimizer
	batchExecutor  *BatchExecutor
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:         make(map[string]Tool),
		cache:         NewResultCache(100, 5*time.Minute),
		batchExecutor: NewBatchExecutor(),
	}
}

func NewRegistryWithoutCache() *ToolRegistry {
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

func (r *ToolRegistry) Execute(ctx context.Context, name string, input map[string]any) (any, error) {
	tool := r.Get(name)
	if tool == nil {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	if r.batchExecutor != nil && r.batchExecutor.Enqueue(name, input) {
		return map[string]any{
			"status":     "deferred",
			"tool":       name,
			"queue_size": r.batchExecutor.QueueSize(),
		}, nil
	}

	if r.cache != nil {
		if result, ok := r.cache.Get(name, input); ok {
			return result, nil
		}
	}

	result, err := tool.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	if r.chainOptimizer != nil {
		r.chainOptimizer.RecordCall(name, input, result)
	}

	if r.cache != nil {
		depFiles := extractDepFiles(name, input)
		r.cache.Set(name, input, result, depFiles)
	}

	return result, nil
}

func (r *ToolRegistry) SetCache(cache *ResultCache) {
	r.cache = cache
}

func (r *ToolRegistry) GetCache() *ResultCache {
	return r.cache
}

func (r *ToolRegistry) InvalidateCache(paths []string) {
	if r.cache != nil {
		r.cache.Invalidate(paths)
	}
}

// SetChainOptimizer sets the chain optimizer for this registry.
func (r *ToolRegistry) SetChainOptimizer(o *ChainOptimizer) {
	r.chainOptimizer = o
}

// GetChainOptimizer returns the chain optimizer, if any.
func (r *ToolRegistry) GetChainOptimizer() *ChainOptimizer {
	return r.chainOptimizer
}

func (r *ToolRegistry) SetBatchExecutor(be *BatchExecutor) {
	r.batchExecutor = be
}

func (r *ToolRegistry) GetBatchExecutor() *BatchExecutor {
	return r.batchExecutor
}

func extractDepFiles(toolName string, input map[string]any) []string {
	var files []string

	pathKeys := []string{"path", "file_path", "filepath", "filename", "directory", "dir"}
	for _, key := range pathKeys {
		if v, ok := input[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				files = append(files, s)
			}
		}
	}

	if toolName == "glob" || toolName == "grep" || toolName == "ast_grep" {
		if v, ok := input["path"]; ok {
			if s, ok := v.(string); ok && s != "" {
				files = append(files, s)
			}
		}
	}

	return files
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
	defaultRegistry.Register(newPowerShellTool())
	defaultRegistry.Register(&McpExecuteTool{})
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
	defaultRegistry.Register(&TeamShareMemoryTool{})
	defaultRegistry.Register(&TeamGetMemoriesTool{})
	defaultRegistry.Register(&TeamSearchMemoriesTool{})
	defaultRegistry.Register(&TeamSyncTool{})
	defaultRegistry.Register(&TeamShareSessionTool{})
	defaultRegistry.Register(&GitAITool{})
	defaultRegistry.Register(&GitStatusTool{})
	defaultRegistry.Register(&GitDiffTool{})
	defaultRegistry.Register(&GitLogTool{})
	defaultRegistry.Register(&EnterPlanModeTool{})
	defaultRegistry.Register(&SyntheticOutputTool{})
	defaultRegistry.Register(&ExitPlanModeTool{})
	defaultRegistry.Register(&McpAuthTool{})
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
	defaultRegistry.Register(&ExecuteCodeTool{})
	defaultRegistry.Register(&DockerSandboxTool{})
	defaultRegistry.Register(&BrowserNavigateTool{})
	defaultRegistry.Register(&BrowserClickTool{})
	defaultRegistry.Register(&BrowserTypeTool{})
	defaultRegistry.Register(&BrowserScreenshotTool{})
	defaultRegistry.Register(&BrowserExtractTool{})
	defaultRegistry.Register(&BrowserWaitTool{})
	defaultRegistry.Register(&BrowserSelectTool{})
	defaultRegistry.Register(&BrowserFillFormTool{})
	defaultRegistry.Register(&MemoryRecallTool{})
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

func Execute(ctx context.Context, name string, input map[string]any) (any, error) {
	return defaultRegistry.Execute(ctx, name, input)
}
