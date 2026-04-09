# Claude Code Go 重写计划

## 项目概述

**目标**: 将 Claude Code v2.1.88 从 TypeScript 完整重写为 Go

**源代码**: `/Users/jw/vscodeProjects/claude-code-source-code/` (512,664 行，1,884 文件)

**目标位置**: `/Users/jw/vscodeProjects/claw-code/go/`

**预估工作量**: ~30,000 行 Go 代码

---

## 架构设计

### TypeScript → Go 模块映射

```
TypeScript                          Go
─────────────────────────────────────────────────
src/main.tsx                    →  cmd/claw/main.go
src/query.ts                    →  internal/runtime/query.go
src/QueryEngine.ts              →  internal/runtime/engine.go
src/Tool.ts                     →  internal/tools/types.go
src/tools.ts                    →  internal/tools/registry.go
src/commands.ts                 →  internal/commands/registry.go
src/services/api/               →  internal/api/
src/services/mcp/               →  internal/mcp/
src/services/compact/           →  internal/runtime/compact.go
src/services/analytics/         →  internal/analytics/
src/services/oauth/             →  internal/auth/oauth.go
src/utils/permissions/          →  internal/permissions/
src/utils/sandbox/              →  internal/sandbox/
src/state/                      →  internal/runtime/state.go
src/types/                      →  internal/types/
```

### Go 项目结构

```
go/
├── cmd/
│   └── claw/
│       └── main.go                 # CLI 入口
│
├── internal/
│   ├── api/
│   │   ├── client.go              # Anthropic API 客户端
│   │   ├── streaming.go           # SSE 流式响应
│   │   ├── types.go               # API 类型定义
│   │   ├── errors.go              # API 错误处理
│   │   └── retry.go               # 重试逻辑
│   │
│   ├── tools/
│   │   ├── types.go               # 工具类型和接口
│   │   ├── registry.go            # 工具注册表
│   │   ├── executor.go            # 工具执行器
│   │   ├── bash.go                # BashTool
│   │   ├── file_read.go           # FileReadTool
│   │   ├── file_write.go          # FileWriteTool
│   │   ├── file_edit.go           # FileEditTool
│   │   ├── glob.go                # GlobTool
│   │   ├── grep.go                # GrepTool
│   │   ├── web_fetch.go           # WebFetchTool
│   │   ├── web_search.go          # WebSearchTool
│   │   ├── agent.go               # AgentTool
│   │   ├── task.go                # Task 相关工具
│   │   ├── mcp.go                 # MCP 工具
│   │   ├── skill.go               # SkillTool
│   │   ├── config.go              # ConfigTool
│   │   ├── todo.go                # TodoWriteTool
│   │   ├── lsp.go                 # LSPTool
│   │   ├── notebook.go            # NotebookEditTool
│   │   ├── ask.go                 # AskUserQuestionTool
│   │   ├── plan.go                # Plan 模式工具
│   │   └── all_others.go          # 其他所有工具
│   │
│   ├── commands/
│   │   ├── registry.go            # 命令注册表
│   │   ├── types.go               # 命令类型
│   │   ├── help.go                # /help
│   │   ├── status.go              # /status
│   │   ├── model.go               # /model
│   │   ├── config.go              # /config
│   │   ├── permissions.go         # /permissions
│   │   ├── session.go             # /session, /resume
│   │   ├── compact.go             # /compact
│   │   ├── cost.go                # /cost
│   │   ├── memory.go              # /memory
│   │   ├── mcp.go                 # /mcp
│   │   ├── agents.go              # /agents
│   │   ├── tasks.go               # /tasks
│   │   ├── skills.go              # /skills
│   │   ├── plan.go                # /plan
│   │   ├── review.go              # /review
│   │   ├── diff.go                # /diff
│   │   ├── init.go                # /init
│   │   ├── login.go               # /login
│   │   ├── logout.go              # /logout
│   │   └── all_others.go          # 其他所有命令
│   │
│   ├── runtime/
│   │   ├── engine.go              # QueryEngine
│   │   ├── query.go               # 主查询循环
│   │   ├── session.go             # 会话管理
│   │   ├── state.go               # 应用状态
│   │   ├── compact.go             # 上下文压缩
│   │   ├── persistence.go         # 持久化
│   │   └── conversation.go        # 对话管理
│   │
│   ├── mcp/
│   │   ├── types.go               # MCP 类型
│   │   ├── client.go              # MCP 客户端
│   │   ├── server.go              # MCP 服务器
│   │   ├── transport.go           # 传输层
│   │   ├── protocol.go            # JSON-RPC 协议
│   │   ├── oauth.go               # OAuth 认证
│   │   └── registry.go            # 服务器注册
│   │
│   ├── permissions/
│   │   ├── types.go               # 权限类型
│   │   ├── engine.go              # 权限引擎
│   │   ├── rules.go               # 规则系统
│   │   ├── sandbox.go             # 沙箱执行
│   │   ├── bash.go                # Bash 权限
│   │   └── prompt.go              # 交互提示
│   │
│   ├── auth/
│   │   ├── oauth.go               # OAuth 流程
│   │   ├── token.go               # Token 管理
│   │   └── storage.go             # 凭据存储
│   │
│   ├── analytics/
│   │   ├── telemetry.go           # 遥测
│   │   └── events.go              # 事件跟踪
│   │
│   ├── sandbox/
│   │   ├── sandbox_linux.go       # Linux 沙箱
│   │   ├── sandbox_other.go       # 其他平台
│   │   └── isolation.go           # 隔离逻辑
│   │
│   ├── git/
│   │   └── context.go             # Git 上下文
│   │
│   ├── types/
│   │   ├── message.go             # 消息类型
│   │   ├── tool.go                # 工具类型
│   │   ├── permission.go          # 权限类型
│   │   └── ids.go                 # ID 类型
│   │
│   └── cli/
│       ├── root.go                # 根命令
│       ├── repl.go                # REPL 模式
│       ├── prompt.go              # Prompt 模式
│       └── output.go              # 输出处理
│
├── pkg/
│   ├── output/
│   │   ├── render.go              # 终端渲染
│   │   ├── color.go               # 颜色处理
│   │   └── format.go              # 格式化
│   │
│   └── utils/
│       ├── file.go                # 文件工具
│       ├── path.go                # 路径工具
│       ├── string.go              # 字符串工具
│       └── json.go                # JSON 工具
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 实现阶段

### 阶段 1: 核心 API 和流式 (已完成 ✅)

**参考文件**:
- `src/services/api/claude.ts` (3,419 行)
- `src/services/api/streaming.ts`

**实现文件**:
- `internal/api/client.go`
- `internal/api/streaming.go`
- `internal/api/types.go`

**核心功能**:
- [x] API 客户端基础
- [x] 消息类型
- [ ] SSE 流式响应
- [ ] 错误处理和重试
- [ ] Token 计数

**关键接口**:
```go
type Client struct {
    APIKey     string
    BaseURL    string
    HTTPClient *http.Client
    Model      string
}

func (c *Client) CreateMessage(ctx context.Context, req *MessageRequest) (*MessageResponse, error)
func (c *Client) StreamMessage(ctx context.Context, req *MessageRequest, handler StreamHandler) error
```

---

### 阶段 2: 工具系统基础 (已完成 ✅)

**参考文件**:
- `src/Tool.ts` (792 行)
- `src/tools.ts`

**实现文件**:
- `internal/tools/types.go`
- `internal/tools/registry.go`
- `internal/tools/executor.go`

**核心功能**:
- [x] 工具接口定义
- [x] 工具注册表
- [x] 基础执行器
- [ ] 并行工具执行
- [ ] 工具结果映射

**关键接口**:
```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]interface{}
    Validate(input map[string]interface{}) error
    CheckPermissions(input map[string]interface{}) PermissionResult
    Execute(ctx context.Context, input map[string]interface{}) (interface{}, error)
}

type ToolRegistry struct {
    tools map[string]Tool
}

func (r *ToolRegistry) Register(tool Tool)
func (r *ToolRegistry) Get(name string) Tool
func (r *ToolRegistry) All() []Tool
```

---

### 阶段 3: 核心工具

#### 3.1 BashTool

**参考文件**: `src/tools/BashTool/` (1,143 行)

**实现文件**: `internal/tools/bash.go`

**核心功能**:
```go
type BashTool struct {
    workDir   string
    timeout   time.Duration
    sandbox   *SandboxManager
}

type BashInput struct {
    Command           string `json:"command"`
    Timeout           int    `json:"timeout,omitempty"`
    Description       string `json:"description,omitempty"`
    RunInBackground   bool   `json:"run_in_background,omitempty"`
    DangerouslyDisableSandbox bool `json:"dangerously_disable_sandbox,omitempty"`
}

type BashOutput struct {
    Stdout        string `json:"stdout"`
    Stderr        string `json:"stderr"`
    ExitCode      int    `json:"exit_code"`
    Interrupted   bool   `json:"interrupted,omitempty"`
    BackgroundID  string `json:"background_task_id,omitempty"`
}
```

**实现要点**:
- 命令执行和超时处理
- 沙箱隔离（Linux namespace）
- 后台任务支持
- 输出截断和格式化
- 安全分析（命令解析）

#### 3.2 FileReadTool

**参考文件**: `src/tools/FileReadTool/`

**实现文件**: `internal/tools/file_read.go`

**核心功能**:
```go
type FileReadInput struct {
    Path   string `json:"path"`
    Offset int    `json:"offset,omitempty"`
    Limit  int    `json:"limit,omitempty"`
}

type FileReadOutput struct {
    Content  string `json:"content"`
    Path     string `json:"path"`
    Lines    int    `json:"lines,omitempty"`
    Encoding string `json:"encoding,omitempty"`
}
```

**实现要点**:
- 文件读取（支持大文件分页）
- 编码检测（UTF-8, Latin-1 等）
- PDF 支持（基础）
- 图像支持（Base64）
- 行尾检测

#### 3.3 FileWriteTool

**参考文件**: `src/tools/FileWriteTool/`

**实现文件**: `internal/tools/file_write.go`

**核心功能**:
```go
type FileWriteInput struct {
    Path    string `json:"path"`
    Content string `json:"content"`
}

type FileWriteOutput struct {
    Path    string `json:"path"`
    Written int    `json:"written"`
}
```

**实现要点**:
- 原子写入（临时文件 + rename）
- 目录自动创建
- 权限设置
- 文件历史记录

#### 3.4 FileEditTool

**参考文件**: `src/tools/FileEditTool/`

**实现文件**: `internal/tools/file_edit.go`

**核心功能**:
```go
type FileEditInput struct {
    Path        string `json:"path"`
    OldString   string `json:"old_string"`
    NewString   string `json:"new_string"`
    ReplaceAll  bool   `json:"replace_all,omitempty"`
}

type FileEditOutput struct {
    Path     string `json:"path"`
    Replaced int    `json:"replaced"`
}
```

**实现要点**:
- 字符串替换
- 多处替换
- 文件历史记录
- 撤销支持

---

### 阶段 4: 搜索工具

#### 4.1 GlobTool

**参考文件**: `src/tools/GlobTool/`

**实现文件**: `internal/tools/glob.go`

**核心功能**:
```go
type GlobInput struct {
    Pattern string   `json:"pattern"`
    Exclude []string `json:"exclude,omitempty"`
}

type GlobOutput struct {
    Files []string `json:"files"`
    Count int      `json:"count"`
}
```

**实现要点**:
- Glob 模式匹配
- 排除模式
- 递归搜索
- 性能优化

#### 4.2 GrepTool

**参考文件**: `src/tools/GrepTool/`

**实现文件**: `internal/tools/grep.go`

**核心功能**:
```go
type GrepInput struct {
    Pattern     string   `json:"pattern"`
    Path        string   `json:"path,omitempty"`
    Include     []string `json:"include,omitempty"`
    Exclude     []string `json:"exclude,omitempty"`
    Context     int      `json:"context,omitempty"`
    IgnoreCase  bool     `json:"ignore_case,omitempty"`
}

type GrepOutput struct {
    Matches []GrepMatch `json:"matches"`
    Count   int         `json:"count"`
}

type GrepMatch struct {
    File    string `json:"file"`
    Line    int    `json:"line"`
    Content string `json:"content"`
}
```

**实现要点**:
- 正则表达式搜索
- 文件过滤
- 上下文行
- 性能优化（可考虑调用 ripgrep）

#### 4.3 WebFetchTool

**参考文件**: `src/tools/WebFetchTool/`

**实现文件**: `internal/tools/web_fetch.go`

**核心功能**:
```go
type WebFetchInput struct {
    URL     string `json:"url"`
    Timeout int    `json:"timeout,omitempty"`
    Format  string `json:"format,omitempty"` // text, markdown, html
}

type WebFetchOutput struct {
    Content string `json:"content"`
    URL     string `json:"url"`
    Status  int    `json:"status"`
}
```

**实现要点**:
- HTTP 客户端
- 超时处理
- Markdown 转换
- 错误处理

#### 4.4 WebSearchTool

**参考文件**: `src/tools/WebSearchTool/`

**实现文件**: `internal/tools/web_search.go`

**核心功能**:
```go
type WebSearchInput struct {
    Query string `json:"query"`
    Limit int    `json:"limit,omitempty"`
}

type WebSearchOutput struct {
    Results []SearchResult `json:"results"`
}

type SearchResult struct {
    Title   string `json:"title"`
    URL     string `json:"url"`
    Snippet string `json:"snippet"`
}
```

---

### 阶段 5: Agent 系统

#### 5.1 AgentTool

**参考文件**: `src/tools/AgentTool/`

**实现文件**: `internal/tools/agent.go`

**核心功能**:
```go
type AgentInput struct {
    Description   string `json:"description"`
    Prompt        string `json:"prompt"`
    SubagentType  string `json:"subagent_type,omitempty"`
    Name          string `json:"name,omitempty"`
    Model         string `json:"model,omitempty"`
    MaxDepth      int    `json:"max_depth,omitempty"`
}

type AgentOutput struct {
    Description string   `json:"description"`
    Status      string   `json:"status"`
    Result      string   `json:"result,omitempty"`
    Depth       int      `json:"depth"`
    MaxDepth    int      `json:"max_depth"`
}
```

**实现要点**:
- 子 Agent 生成
- Fork 模式（子进程）
- Worktree 隔离
- 递归深度限制
- 结果聚合

#### 5.2 Task 工具

**参考文件**: `src/tools/Task*.ts`

**实现文件**: `internal/tools/task.go`

**工具列表**:
- TaskCreateTool
- TaskGetTool
- TaskListTool
- TaskUpdateTool
- TaskStopTool
- TaskOutputTool

**核心功能**:
```go
type Task struct {
    ID          string    `json:"id"`
    Description string    `json:"description"`
    Status      string    `json:"status"` // pending, in_progress, completed, failed
    Priority    int       `json:"priority"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    Result      string    `json:"result,omitempty"`
}
```

---

### 阶段 6: MCP 协议

**参考文件**: `src/services/mcp/`

**实现文件**: `internal/mcp/`

#### 6.1 类型定义

**实现文件**: `internal/mcp/types.go`

```go
type McpServerConfig struct {
    Name        string                 `json:"name"`
    Transport   string                 `json:"transport"` // stdio, sse, ws, http
    Command     string                 `json:"command,omitempty"`
    Args        []string               `json:"args,omitempty"`
    URL         string                 `json:"url,omitempty"`
    Env         map[string]string      `json:"env,omitempty"`
    OAuth       *McpOAuthConfig        `json:"oauth,omitempty"`
}

type McpTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"input_schema"`
}

type McpResource struct {
    URI         string                 `json:"uri"`
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
}
```

#### 6.2 客户端实现

**实现文件**: `internal/mcp/client.go`

```go
type McpClient struct {
    config     McpServerConfig
    transport  Transport
    tools      []McpTool
    resources  []McpResource
}

func (c *McpClient) Connect(ctx context.Context) error
func (c *McpClient) ListTools(ctx context.Context) ([]McpTool, error)
func (c *McpClient) InvokeTool(ctx context.Context, name string, input interface{}) (interface{}, error)
func (c *McpClient) ListResources(ctx context.Context) ([]McpResource, error)
func (c *McpClient) ReadResource(ctx context.Context, uri string) (interface{}, error)
```

#### 6.3 传输层

**实现文件**: `internal/mcp/transport.go`

```go
type Transport interface {
    Connect(ctx context.Context) error
    Close() error
    Send(request *JsonRpcRequest) error
    Receive() (*JsonRpcResponse, error)
}

// 实现:
type StdioTransport struct { ... }
type SSETransport struct { ... }
type WebSocketTransport struct { ... }
type HTTPTransport struct { ... }
```

#### 6.4 OAuth 流程

**实现文件**: `internal/mcp/oauth.go`

```go
type McpOAuthConfig struct {
    ClientID     string `json:"client_id"`
    AuthorizeURL string `json:"authorize_url"`
    TokenURL     string `json:"token_url"`
    Scopes       []string `json:"scopes,omitempty"`
}

func (o *OAuthFlow) Start(ctx context.Context) (*TokenSet, error)
func (o *OAuthFlow) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error)
```

---

### 阶段 7: 权限系统

**参考文件**: `src/utils/permissions/`

**实现文件**: `internal/permissions/`

#### 7.1 权限类型

**实现文件**: `internal/permissions/types.go`

```go
type PermissionMode string

const (
    PermissionModeReadOnly         PermissionMode = "read-only"
    PermissionModeWorkspaceWrite   PermissionMode = "workspace-write"
    PermissionModeDangerFullAccess PermissionMode = "danger-full-access"
)

type PermissionResult string

const (
    PermissionAllow PermissionResult = "allow"
    PermissionDeny  PermissionResult = "deny"
    PermissionAsk   PermissionResult = "ask"
)

type PermissionRule struct {
    ToolPattern string          `json:"tool_pattern"`
    Result      PermissionResult `json:"result"`
    Reason      string          `json:"reason,omitempty"`
}
```

#### 7.2 权限引擎

**实现文件**: `internal/permissions/engine.go`

```go
type PermissionEngine struct {
    mode                 PermissionMode
    alwaysAllowRules     []PermissionRule
    alwaysDenyRules      []PermissionRule
    alwaysAskRules       []PermissionRule
    sessionDecisions     map[string]PermissionResult
}

func (e *PermissionEngine) Check(toolName string, input map[string]interface{}) PermissionResult
func (e *PermissionEngine) AddRule(rule PermissionRule)
func (e *PermissionEngine) RecordDecision(toolName string, result PermissionResult)
```

#### 7.3 规则匹配

**实现文件**: `internal/permissions/rules.go`

```go
func MatchPattern(pattern, toolName string) bool {
    // 支持通配符:
    // "*" 匹配所有
    // "bash" 精确匹配
    // "file_*" 前缀匹配
    // "*_tool" 后缀匹配
}
```

#### 7.4 沙箱执行

**实现文件**: `internal/permissions/sandbox.go`

```go
type SandboxConfig struct {
    Enabled            bool
    FilesystemMode     string   // "off", "workspace-only", "allow-list"
    AllowedPaths       []string
    NetworkIsolation   bool
    NamespaceIsolation bool
}

func (s *Sandbox) WrapCommand(cmd *exec.Cmd) error
func (s *Sandbox) ValidatePath(path string) error
```

---

### 阶段 8: 会话管理 (已完成 ✅)

**参考文件**: `src/state/`, `src/utils/sessionPersistence.ts`

**实现文件**: `internal/runtime/session.go`

**核心功能**:
```go
type Session struct {
    ID        string
    Messages  []Message
    CreatedAt time.Time
    UpdatedAt time.Time
    Model     string
}

func (s *Session) AddMessage(role string, content interface{})
func (s *Session) Save(path string) error
func LoadSession(path string) (*Session, error)
```

---

### 阶段 9: 上下文压缩

**参考文件**: `src/services/compact/`

**实现文件**: `internal/runtime/compact.go`

**核心功能**:
```go
type CompactionConfig struct {
    MaxTokens      int
    PreserveRecent int
    Strategy       string // "auto", "snip", "collapse"
}

func CompactMessages(messages []Message, config CompactionConfig) ([]Message, error)
func EstimateTokens(messages []Message) int
```

---

### 阶段 10: Slash 命令

**参考文件**: `src/commands/`

**实现文件**: `internal/commands/`

#### 10.1 命令注册表

**实现文件**: `internal/commands/registry.go`

```go
type Command struct {
    Name        string
    Summary     string
    Usage       string
    Handler     CommandHandler
    Aliases     []string
}

type CommandHandler func(ctx context.Context, args []string, session *Session) error

type CommandRegistry struct {
    commands map[string]Command
}

func (r *CommandRegistry) Register(cmd Command)
func (r *CommandRegistry) Execute(name string, args []string, session *Session) error
func (r *CommandRegistry) Help() string
```

#### 10.2 核心命令

| 命令 | 参考文件 | 功能 |
|------|---------|------|
| `/help` | `src/commands/help/` | 显示帮助 |
| `/status` | `src/commands/status/` | 会话状态 |
| `/model` | `src/commands/model/` | 模型切换 |
| `/config` | `src/commands/config/` | 配置管理 |
| `/permissions` | `src/commands/permissions/` | 权限管理 |
| `/memory` | `src/commands/memory/` | 内存系统 |
| `/compact` | `src/commands/compact/` | 上下文压缩 |
| `/cost` | `src/commands/cost/` | 成本统计 |
| `/resume` | `src/commands/resume/` | 会话恢复 |
| `/session` | `src/commands/session/` | 会话管理 |
| `/init` | `src/commands/init/` | 项目初始化 |
| `/diff` | `src/commands/diff/` | Git diff |
| `/mcp` | `src/commands/mcp/` | MCP 管理 |
| `/login` | `src/commands/login/` | 登录 |
| `/logout` | `src/commands/logout/` | 登出 |
| `/plan` | `src/commands/plan/` | 计划模式 |
| `/review` | `src/commands/review/` | 代码审查 |
| `/agents` | `src/commands/agents/` | Agent 管理 |
| `/tasks` | `src/commands/tasks/` | 任务管理 |
| `/skills` | `src/commands/skills/` | 技能管理 |

---

### 阶段 11: OAuth 认证

**参考文件**: `src/services/oauth/`

**实现文件**: `internal/auth/oauth.go`

**核心功能**:
```go
type OAuthConfig struct {
    ClientID     string
    AuthorizeURL string
    TokenURL     string
    Scopes       []string
    CallbackPort int
}

type TokenSet struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    ExpiresAt    time.Time `json:"expires_at"`
}

func (o *OAuthFlow) Start(ctx context.Context) (*TokenSet, error)
func (o *OAuthFlow) Refresh(ctx context.Context) (*TokenSet, error)
func SaveCredentials(creds *TokenSet) error
func LoadCredentials() (*TokenSet, error)
```

---

### 阶段 12: 插件系统

**参考文件**: `src/plugins/`, `src/services/plugins/`

**实现文件**: `internal/plugins/`

**核心功能**:
- 插件加载
- 生命周期管理
- Hook 注入
- 工具扩展

---

### 阶段 13: Hooks 系统

**参考文件**: `src/hooks/`, `src/services/tools/toolHooks.ts`

**实现文件**: `internal/hooks/`

**核心功能**:
```go
type Hook struct {
    Type       string   // "PreToolUse", "PostToolUse"
    Command    string   // Shell 命令
    Tools      []string // 匹配的工具
}

func ExecutePreToolUseHook(hook Hook, toolName string, input map[string]interface{}) (HookResult, error)
func ExecutePostToolUseHook(hook Hook, toolName string, input, output interface{}) (HookResult, error)
```

---

### 阶段 14: Skills 系统

**参考文件**: `src/skills/`

**实现文件**: `internal/skills/`

**核心功能**:
- SKILL.md 加载
- 技能发现
- 动态加载
- 打包技能

---

### 阶段 15: Voice Mode

**参考文件**: `src/services/voice.ts`

**实现文件**: `internal/voice/`

**核心功能**:
- Push-to-talk
- Speech-to-text
- Voice keyterms

---

## Agent 工作分配

### Agent 1: 工具系统 (bg_56a0106a)

**负责**: 所有 40+ 工具的实现

**工作范围**:
```
internal/tools/
├── types.go          # 工具类型定义
├── registry.go       # 工具注册表
├── executor.go       # 执行器
├── bash.go           # BashTool
├── file_read.go      # FileReadTool
├── file_write.go     # FileWriteTool
├── file_edit.go      # FileEditTool
├── glob.go           # GlobTool
├── grep.go           # GrepTool
├── web_fetch.go      # WebFetchTool
├── web_search.go     # WebSearchTool
├── agent.go          # AgentTool
├── task.go           # Task tools
├── mcp.go            # MCP tools
├── skill.go          # SkillTool
├── config.go         # ConfigTool
├── todo.go           # TodoWriteTool
├── lsp.go            # LSPTool
├── notebook.go       # NotebookEditTool
├── ask.go            # AskUserQuestionTool
├── plan.go           # Plan tools
└── all_others.go     # 其他所有工具
```

**参考目录**: `/Users/jw/vscodeProjects/claude-code-source-code/src/tools/`

**输出要求**:
- 每个工具一个文件
- 完整实现核心功能
- Go 风格错误处理
- 完整注释

---

### Agent 2: MCP 协议 (bg_933a58ef)

**负责**: MCP 协议完整实现

**工作范围**:
```
internal/mcp/
├── types.go          # MCP 类型定义
├── client.go         # MCP 客户端
├── server.go         # MCP 服务器
├── transport.go      # 传输层实现
├── protocol.go       # JSON-RPC 协议
├── oauth.go          # OAuth 认证
└── registry.go       # 服务器注册
```

**参考目录**: `/Users/jw/vscodeProjects/claude-code-source-code/src/services/mcp/`

**输出要求**:
- 支持所有传输类型
- 完整 OAuth 流程
- 错误处理和重连

---

### Agent 3: 权限系统 (bg_de361aed)

**负责**: 权限系统完整实现

**工作范围**:
```
internal/permissions/
├── types.go          # 权限类型
├── engine.go         # 权限引擎
├── rules.go          # 规则系统
├── sandbox.go        # 沙箱执行
├── bash.go           # Bash 权限
└── prompt.go         # 交互提示
```

**参考目录**: `/Users/jw/vscodeProjects/claude-code-source-code/src/utils/permissions/`

**输出要求**:
- 三种权限模式
- 规则匹配
- 沙箱隔离
- 交互式提示

---

### Agent 4: 命令系统 (bg_f74d1ec3)

**负责**: 所有 80+ 命令的实现

**工作范围**:
```
internal/commands/
├── registry.go       # 命令注册表
├── types.go          # 命令类型
├── help.go           # /help
├── status.go         # /status
├── model.go          # /model
├── config.go         # /config
├── permissions.go    # /permissions
├── session.go        # /session, /resume
├── compact.go        # /compact
├── cost.go           # /cost
├── memory.go         # /memory
├── mcp.go            # /mcp
├── agents.go         # /agents
├── tasks.go          # /tasks
├── skills.go         # /skills
├── plan.go           # /plan
├── review.go         # /review
├── diff.go           # /diff
├── init.go           # /init
├── login.go          # /login
├── logout.go         # /logout
└── all_others.go     # 其他所有命令
```

**参考目录**: `/Users/jw/vscodeProjects/claude-code-source-code/src/commands/`

**输出要求**:
- 每个命令实现完整
- 帮助文本
- 参数解析

---

## 代码规范

### Go 代码风格

```go
// 1. 包注释
// Package tools provides tool implementations for Claude Code.
package tools

// 2. 导出函数注释
// Execute runs the tool with the given input and returns the result.
func (t *Tool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
    // 实现
}

// 3. 错误处理
if err != nil {
    return nil, fmt.Errorf("failed to execute tool %s: %w", t.name, err)
}

// 4. Context 传递
func (t *Tool) Execute(ctx context.Context, ...) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
}

// 5. 接口定义
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]interface{}
    Execute(ctx context.Context, input map[string]interface{}) (interface{}, error)
}
```

### 文件命名

- 使用 `snake_case`: `file_read.go`, `web_fetch.go`
- 测试文件: `file_read_test.go`
- 平台特定: `sandbox_linux.go`, `sandbox_other.go`

### 目录结构

- 每个包一个目录
- 内部包放在 `internal/`
- 公共包放在 `pkg/`

---

## 测试策略

### 单元测试

```go
// file_read_test.go
func TestFileReadTool_Execute(t *testing.T) {
    tool := NewFileReadTool(".")
    
    // 测试正常读取
    output, err := tool.Execute(context.Background(), map[string]interface{}{
        "path": "test.txt",
    })
    assert.NoError(t, err)
    assert.Contains(t, output.Content, "expected content")
    
    // 测试错误处理
    _, err = tool.Execute(context.Background(), map[string]interface{}{
        "path": "nonexistent.txt",
    })
    assert.Error(t, err)
}
```

### 集成测试

```go
// integration_test.go
func TestFullWorkflow(t *testing.T) {
    // 1. 创建会话
    session := runtime.NewSession("claude-opus-4-6")
    
    // 2. 执行工具
    tool := tools.NewBashTool(".")
    output, err := tool.Execute(context.Background(), map[string]interface{}{
        "command": "echo hello",
    })
    assert.NoError(t, err)
    
    // 3. 验证结果
    assert.Equal(t, "hello\n", output.Stdout)
}
```

---

## 依赖管理

### go.mod

```go
module github.com/instructkr/smartclaw

go 1.22

require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.2
    github.com/go-resty/resty/v2 v2.11.0
    github.com/gorilla/websocket v1.5.1
    github.com/stretchr/testify v1.8.4
    go.uber.org/zap v1.26.0
)
```

### 外部依赖

| 功能 | 库 |
|------|-----|
| CLI | `spf13/cobra` |
| 配置 | `spf13/viper` |
| HTTP | `go-resty/resty` |
| WebSocket | `gorilla/websocket` |
| 日志 | `uber-go/zap` |
| 测试 | `stretchr/testify` |

---

## Makefile

```makefile
.PHONY: build test clean install

BINARY_NAME=claw
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/claw

test:
	go test -v -race ./...

clean:
	rm -rf bin/

install: build
	cp bin/$(BINARY_NAME) /usr/local/bin/

run:
	go run ./cmd/claw

fmt:
	go fmt ./...

lint:
	golangci-lint run
```

---

## 完成标准

### Phase 完成 checklist

每个阶段完成需要满足：

- [ ] 所有文件已创建
- [ ] 代码编译通过 (`go build ./...`)
- [ ] 单元测试通过 (`go test ./...`)
- [ ] 代码格式化 (`go fmt ./...`)
- [ ] 无 lint 错误 (`golangci-lint run`)
- [ ] 文档已更新

### 项目完成标准

- [ ] 所有 40+ 工具实现
- [ ] 所有 80+ 命令实现
- [ ] MCP 协议支持
- [ ] 权限系统完整
- [ ] 会话管理
- [ ] OAuth 认证
- [ ] 测试覆盖率 > 60%
- [ ] 文档完整

---

## 时间估算

| 阶段 | 预估时间 | 优先级 |
|------|---------|--------|
| Phase 1-3 | 已完成 | - |
| Phase 4 | 2-3 天 | 高 |
| Phase 5 | 3-4 天 | 高 |
| Phase 6 | 3-4 天 | 中 |
| Phase 7 | 2-3 天 | 中 |
| Phase 8 | 已完成 | - |
| Phase 9 | 2-3 天 | 中 |
| Phase 10 | 3-4 天 | 中 |
| Phase 11 | 2 天 | 低 |
| Phase 12 | 3-4 天 | 低 |
| Phase 13 | 2-3 天 | 低 |
| Phase 14 | 2-3 天 | 低 |
| Phase 15 | 2 天 | 低 |

**总计**: 约 30-40 工作日

---

## 联系和协调

### 当前运行的 Agents

| Agent ID | 任务 | 状态 |
|----------|------|------|
| bg_56a0106a | 工具系统 | 运行中 |
| bg_933a58ef | MCP 协议 | 运行中 |
| bg_de361aed | 权限系统 | 运行中 |
| bg_f74d1ec3 | 命令系统 | 运行中 |

### 协调规则

1. 每个 Agent 独立工作，不修改其他 Agent 负责的文件
2. 共享类型定义在 `internal/types/`
3. 完成后提交到各自的分支
4. 定期同步主分支

---

## 文档更新

### README.md 结构

```markdown
# Claw Code - Go Edition

## Overview
## Features
## Installation
## Quick Start
## Commands
## Tools
## Configuration
## Development
## Contributing
## License
```

### API 文档

使用 Go doc 标准：

```go
// Package tools provides tool implementations for Claude Code.
//
// The tools package implements the tool system used by Claude Code
// to interact with the user's environment. Each tool provides a
// specific capability such as file operations, shell execution,
// web fetching, etc.
//
// Example usage:
//
//    tool := tools.NewBashTool(".")
//    output, err := tool.Execute(ctx, map[string]interface{}{
//        "command": "echo hello",
//    })
package tools
```

---

## 总结

这个计划涵盖了从 TypeScript 到 Go 的完整重写过程。4 个 Agent 正在并行工作：

1. **工具系统** - 实现 40+ 工具
2. **MCP 协议** - 完整 MCP 支持
3. **权限系统** - 安全和权限管理
4. **命令系统** - 80+ slash 命令

预计 30-40 工作日完成完整重写。
