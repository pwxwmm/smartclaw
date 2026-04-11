package constants

const (
	Version = "1.0.0"
	Name    = "SmartClaw"
	Author  = "weimengmeng 天气晴"
	Email   = "1300042631@qq.com"
)

const (
	ModelClaudeOpus45   = "claude-opus-4-5"
	ModelClaudeSonnet45 = "claude-sonnet-4-5"
	ModelClaudeHaiku3   = "claude-haiku-3-5"
	ModelClaudeSonnet37 = "claude-sonnet-3-7"
	ModelClaudeOpus37   = "claude-opus-3-7"
)

const DefaultModel = ModelClaudeSonnet45

const (
	MaxTokensDefault  = 4096
	MaxTokensMax      = 200000
	MaxRetries        = 3
	RequestTimeout    = 120
	SSEConnectTimeout = 30
)

const (
	PermissionReadOnly       = "read-only"
	PermissionWorkspaceWrite = "workspace-write"
	PermissionDangerFull     = "danger-full-access"
)

const (
	ToolBash      = "bash"
	ToolRead      = "read"
	ToolWrite     = "write"
	ToolEdit      = "edit"
	ToolGlob      = "glob"
	ToolGrep      = "grep"
	ToolWebFetch  = "web_fetch"
	ToolWebSearch = "web_search"
	ToolLSP       = "lsp"
	ToolAgent     = "agent"
	ToolTask      = "task"
	ToolMCP       = "mcp"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)

const StopReasonEndTurn = "end_turn"
const StopReasonToolUse = "tool_use"
const StopReasonMaxTokens = "max_tokens"
const StopReasonStopSequence = "stop_sequence"

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusStopped   = "stopped"
)

const TokenPricingInput = 0.000015
const TokenPricingOutput = 0.000075

const (
	EventMessageStart      = "message_start"
	EventContentBlockStart = "content_block_start"
	EventContentBlockDelta = "content_block_delta"
	EventContentBlockStop  = "content_block_stop"
	EventMessageDelta      = "message_delta"
	EventMessageStop       = "message_stop"
	EventPing              = "ping"
)

const DefaultConfigPath = "~/.smartclaw/config.yaml"
const DefaultSessionsDir = "~/.smartclaw/sessions"
const DefaultSkillsDir = "~/.smartclaw/skills"
const DefaultPluginsDir = "~/.smartclaw/plugins"

const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
)

const EnvAPIKey = "ANTHROPIC_API_KEY"
const EnvBaseURL = "ANTHROPIC_BASE_URL"
const EnvDebug = "DEBUG"
