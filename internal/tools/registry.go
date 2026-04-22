package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/observability"
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
	mu             sync.RWMutex
	tools          map[string]Tool
	cache          *ResultCache
	chainOptimizer *ChainOptimizer
	batchExecutor  *BatchExecutor
	distribution   *ToolsetDistribution
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
	r.mu.Lock()
	r.tools[tool.Name()] = tool
	r.mu.Unlock()
}

func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	delete(r.tools, name)
	r.mu.Unlock()
}

func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

func (r *ToolRegistry) All() []Tool {
	r.mu.RLock()
	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	r.mu.RUnlock()
	return result
}

func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	r.mu.RUnlock()
	return names
}

func (r *ToolRegistry) Execute(ctx context.Context, name string, input map[string]any) (any, error) {
	r.mu.RLock()
	tool := r.tools[name]
	r.mu.RUnlock()
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

	start := time.Now()
	result, err := tool.Execute(ctx, input)
	duration := time.Since(start)

	observability.AuditToolExecution(name, input, auditResultToString(result), duration, err == nil, err)

	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(name, "sopa_") {
		go func() {
			if resultMap, ok := result.(map[string]any); ok {
				im := getIncidentMemory()
				if im != nil {
					_ = im.UpdateIncidentFromToolResult(sopaToolNameToIncidentName(name), resultMap)
				}
			}
		}()
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

func (r *ToolRegistry) SetDistribution(d *ToolsetDistribution) {
	r.distribution = d
}

func (r *ToolRegistry) GetDistribution() *ToolsetDistribution {
	return r.distribution
}

// SelectToolset returns tools filtered by complexity using the distribution.
// Falls back to All() if no distribution is configured or distribution returns empty.
// Always includes core tools (bash, read_file, write_file, edit_file, think, todowrite).
func (r *ToolRegistry) SelectToolset(ctx context.Context, complexity float64) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.distribution == nil {
		result := make([]Tool, 0, len(r.tools))
		for _, tool := range r.tools {
			result = append(result, tool)
		}
		return result
	}

	selected, err := r.distribution.SelectTools(ctx, complexity, r.tools)
	if err != nil || len(selected) == 0 {
		result := make([]Tool, 0, len(r.tools))
		for _, tool := range r.tools {
			result = append(result, tool)
		}
		return result
	}

	resultSet := make(map[string]bool, len(selected))
	result := make([]Tool, 0, len(selected)+6)
	for _, t := range selected {
		resultSet[t.Name()] = true
		result = append(result, t)
	}

	coreTools := []string{"bash", "read_file", "write_file", "edit_file", "think", "todowrite"}
	for _, name := range coreTools {
		if tool, exists := r.tools[name]; exists && !resultSet[name] {
			result = append(result, tool)
		}
	}

	return result
}

// AssessQueryComplexity computes a simple complexity score from a query string.
// Score ranges from 0.0 to 1.0.
func AssessQueryComplexity(input string) float64 {
	score := 0.0
	words := len(strings.Fields(input))
	if words > 50 {
		score += 0.2
	}
	if words > 100 {
		score += 0.2
	}
	if strings.Contains(input, "refactor") || strings.Contains(input, "architect") {
		score += 0.2
	}
	if strings.Contains(input, "debug") || strings.Contains(input, "fix") || strings.Contains(input, "error") {
		score += 0.1
	}
	if strings.Contains(input, "deploy") || strings.Contains(input, "production") {
		score += 0.15
	}
	if strings.Contains(input, "browser") || strings.Contains(input, "scrape") {
		score += 0.15
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func sopaToolNameToIncidentName(name string) string {
	switch name {
	case "sopa_list_faults":
		return "sopa_fault_tracking_list"
	default:
		return name
	}
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

func auditResultToString(result any) string {
	if result == nil {
		return ""
	}
	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
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
	defaultRegistry.Register(&DiffEditTool{})
	defaultRegistry.Register(&LineEditTool{})
	defaultRegistry.Register(&PreviewFileTool{})
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
	defaultRegistry.Register(&GitHubCreatePRTool{})
	defaultRegistry.Register(&GitHubListPRsTool{})
	defaultRegistry.Register(&GitHubMergePRTool{})
	defaultRegistry.Register(&GitHubCreateIssueTool{})
	defaultRegistry.Register(&GitHubListIssuesTool{})
	defaultRegistry.Register(&WorktreeCreateTool{})
	defaultRegistry.Register(&WorktreeRemoveTool{})
	defaultRegistry.Register(&WorktreeListTool{})
	defaultRegistry.Register(&WorktreeDiffTool{})
	defaultRegistry.Register(&WorktreeMergeTool{})
	defaultRegistry.Register(&SopaListNodesTool{})
	defaultRegistry.Register(&SopaGetNodeTool{})
	defaultRegistry.Register(&SopaNodeLogsTool{})
	defaultRegistry.Register(&SopaNodeTasksTool{})
	defaultRegistry.Register(&SopaClusterStatsTool{})
	defaultRegistry.Register(&SopaExecuteTaskTool{})
	defaultRegistry.Register(&SopaExecuteOrchestrationTool{})
	defaultRegistry.Register(&SopaTaskStatusTool{})
	defaultRegistry.Register(&SopaListFaultsTool{})
	defaultRegistry.Register(&SopaGetFaultTool{})
	defaultRegistry.Register(&SopaListFaultTypesTool{})
	defaultRegistry.Register(&SopaFaultWarrantyTool{})
	defaultRegistry.Register(&SopaListAuditsTool{})
	defaultRegistry.Register(&SopaApproveAuditTool{})
	defaultRegistry.Register(&SopaRejectAuditTool{})
	defaultRegistry.Register(&AuditQueryTool{})
	defaultRegistry.Register(&AuditStatsTool{})
	defaultRegistry.Register(&InvestigateIncidentTool{})
	defaultRegistry.Register(&IncidentTimelineTool{})
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

func SelectToolset(ctx context.Context, complexity float64) []Tool {
	return GetRegistry().SelectToolset(ctx, complexity)
}
