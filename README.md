[English](#english) | [中文](#中文)

<a id="english"></a>

# SmartClaw

An AI-powered SRE/Ops platform with a self-improving agent that learns your workflow over time.

SmartClaw goes beyond coding assistance. It is a full-spectrum SRE platform featuring collaborative multi-agent fault diagnosis (War Room), alert correlation, automated remediation, topology-aware blast radius analysis, and a self-improving learning loop. The more you use it, the better it understands how you work and how your systems behave.

## Core Philosophy

**"越来越懂你的工作方式"** -- Not "better at everything", but "better at *your* everything".

Unlike generic AI assistants that forget everything between sessions, SmartClaw:

- **Learns from completed tasks** -- evaluates whether an approach is worth reusing, and if so, extracts it as a skill
- **Remembers across sessions** -- 4-layer memory system with SQLite + FTS5 full-text search
- **Self-improves over time** -- periodic nudges trigger the agent to consolidate memory and refine skills
- **Understands your preferences** -- passively tracks communication style, knowledge background, and common workflows
- **Diagnoses incidents collaboratively** -- War Room dispatches domain expert agents in parallel for SRE fault diagnosis

## War Room: Collaborative Multi-Agent Fault Diagnosis

The War Room is SmartClaw's flagship SRE capability. When an incident occurs, it orchestrates multiple domain expert agents to investigate in parallel, synthesize findings, and converge on root cause analysis.

### 8 Domain Expert Agents

| Agent | Domain | Specialization |
|-------|--------|----------------|
| Network | Network | TCP/UDP, DNS, load balancers, firewalls, packet analysis |
| Database | Database | MySQL, PostgreSQL, Redis, MongoDB, query performance, replication |
| Infrastructure | Infra | Kubernetes, Docker, CPU/Memory/Disk, kernel, systemd |
| Application | App | Code-level errors, memory leaks, thread deadlocks, performance |
| Security | Security | Intrusion detection, privilege escalation, misconfigurations, CVEs |
| Reasoning | Reasoning | Synthesis, cross-validation, confidence evolution, final analysis |
| Training | Training | Multi-node multi-GPU training, NCCL, Megatron, DeepSpeed, OOM |
| Inference | Inference | vLLM, SGLang, TensorRT-LLM, serving latency, GPU memory, batching |

### 4-Phase Staged Execution

```
Incident --> Smart Dispatcher --> Phase 1 (Parallel Agents)
                                       |
                               Phase 2 (Reasoning Synthesis)
                                       |
                         Phase 3 (Dynamic Expansion, if needed)
                                       |
                         Phase 4 (Final Analysis)
                                       |
                                 Shared Blackboard
                               (Entries, Hypotheses, Facts)
```

- **Phase 1: Parallel Investigation** -- The Smart Dispatcher analyzes incident keywords and dispatches relevant domain agents. Each agent investigates independently and writes findings to the Shared Blackboard.
- **Phase 2: Reasoning Synthesis** -- The Reasoning agent reads all blackboard entries, integrates findings, identifies patterns, and forms initial hypotheses.
- **Phase 3: Dynamic Expansion** -- If the Reasoning agent detects gaps or conflicting evidence, it recommends additional domain agents. New agents join and contribute to the blackboard.
- **Phase 4: Final Analysis** -- The Reasoning agent produces a comprehensive root cause report with confidence scores, supporting evidence, and remediation recommendations.

### Shared Blackboard

All agents share a mutable context. The blackboard contains:

- **Entries** -- Investigation findings from each agent, timestamped and attributed
- **Hypotheses** -- Proposed root causes with supporting and contradicting evidence
- **Shared Facts** -- Confirmed facts that all agents agree on

### Agent Handoff Protocol

Agents can request help from other agents mid-execution using the `warroom_handoff` tool. The requesting agent specifies which domain expertise it needs. The handoff times out after 30 seconds if the target agent is unavailable.

### Cross-Validation and Confidence Evolution

Findings are cross-validated across agents. When multiple agents produce matching evidence, confidence increases. When evidence contradicts, confidence decreases. Confidence is bounded in [0.1, 0.95] to prevent premature certainty or complete dismissal.

### 3 Agent Collaboration Tools

| Tool | Description |
|------|-------------|
| `warroom_handoff` | Request help from another domain agent mid-investigation |
| `warroom_evaluate` | Evaluate and cross-validate findings from other agents |
| `warroom_blackboard_write` | Write entries, hypotheses, or facts to the shared blackboard |

### Smart Dispatcher

The dispatcher uses keyword-based agent selection. Incident descriptions are scored against each agent's keyword profile. Agents above the relevance threshold are dispatched in Phase 1. The Reasoning agent can request additional agents in Phase 3.

## SRE-Specific Features

### Alert Engine (alertengine)

Alert correlation and fingerprinting. Groups related alerts into incidents, deduplicates noise, and generates stable fingerprints for tracking alert recurrence across sessions.

### Auto-Remediation (autoremediation)

Automated remediation with safety checks. Executes predefined or AI-generated remediation actions with pre-flight validation, rollback capability, and approval gates for high-risk operations.

### Topology

Blast radius analysis and dependency graph. Maps service dependencies, computes impact scope when a component fails, and visualizes propagation paths for cascading failures.

### Watchdog

Continuous monitoring and alerting. Runs periodic health checks against infrastructure and application endpoints, triggers alerts on threshold violations, and feeds data into the alert engine for correlation.

### Playbook

Runbook automation. Transforms manual SRE runbooks into executable, version-controlled playbooks with parameterized steps, conditional branching, and audit logging.

## Agent Capabilities

- **Learning Loop**: Post-task evaluation, method extraction, skill creation, MEMORY.md auto-update
- **4-Layer Memory**: Prompt Memory (MEMORY.md/USER.md), Session Search (FTS5), Skill Procedural (lazy load), User Modeling
- **Periodic Nudge**: System-triggered self-review every 10 turns (configurable)
- **Smart Compaction**: Auto-compact with configurable thresholds, head protection, tool result pruning, and source-traceable summaries
- **Speculative Execution**: Dual-model routing -- run fast + heavy models in parallel, accept fast result if similar, fall back to heavy if divergent
- **Adaptive Model Router**: Complexity-based model selection (fast/default/heavy) with cost-first, quality-first, or balanced strategies
- **Cost Guard**: Budget-aware spending with daily/session limits, warning thresholds, and automatic model downgrade when approaching limits

## Development Tools

- **73+ Built-in Tools**: File operations, code analysis, web tools, MCP integration, browser automation, Docker sandboxing, and more
- **101 Slash Commands**: Full productivity command suite with agent management, template system, and IDE integration
- **Modern TUI**: Terminal User Interface built with Bubble Tea
- **Interactive REPL**: Full conversation history with streaming responses
- **MCP Integration**: Connect to MCP servers, discover tools, read resources, and authenticate via OAuth
- **ACP Server**: Agent Communication Protocol for IDE integration (VS Code, Zed, JetBrains) via stdio JSON-RPC
- **VS Code Extension**: Official extension with chat sidebar, code explanation, fix, and test generation commands
- **Secure**: Permission system with 4 modes, sandboxed execution on Linux, Docker isolation
- **Token Tracking**: Real-time cost estimation with auto-compact at threshold

## Browser Automation

- **Headless Browser**: Navigate, click, type, screenshot, extract content, and fill forms using Chromium (via chromedp)
- **8 Browser Tools**: `browser_navigate`, `browser_click`, `browser_type`, `browser_screenshot`, `browser_extract`, `browser_wait`, `browser_select`, `browser_fill_form`

## Code Execution and Sandboxing

- **Execute Code Tool**: Run Python code in an RPC sandbox with direct access to SmartClaw tools -- collapses multi-turn workflows into a single turn
- **Docker Sandbox**: Isolated container execution with project directory mounted at `/workspace`, supporting both one-shot and session-persistent containers
- **Linux Namespace Sandbox**: Native sandboxed execution using Linux namespaces for secure isolation

## Gateway and Cross-Platform

- **Unified Gateway**: Message, Route, Memory, Execute, Learn, Deliver
- **Platform Adapters**: Terminal, Web UI, Telegram, extensible to Discord
- **Cron Tasks**: Scheduled tasks as first-class agent tasks with full memory access
- **Session Routing**: userID-based routing, not platform-based -- switch devices without losing context
- **Session Recording**: Record and replay full sessions for audit and review
- **Remote Trigger**: Execute commands on remote hosts via SSH

## Team Collaboration

- **Team Workspaces**: Create shared team spaces with AES-encrypted memory sync
- **Team Memory Sharing**: Share memories, sessions, and knowledge across team members
- **Team Tools**: `team_create`, `team_delete`, `team_share_memory`, `team_get_memories`, `team_search_memories`, `team_sync`, `team_share_session`

## Observability and Analytics

- **Metrics Dashboard**: Real-time query count, cache hit rate, token usage, cost estimation, tool execution stats, and per-model query counts
- **Distributed Tracing**: Request-level tracing for debugging latency and failures
- **Telemetry API**: REST endpoint (`/api/telemetry`) exposing full observability data

## Batch and RL Evaluation

- **Batch Runner**: Execute agent across hundreds of prompts in parallel, output ShareGPT-format training trajectories
- **RL Evaluation**: Run reward-based evaluation loops with configurable metrics (exact_match, code_quality, length_penalty)
- **Trajectory Export**: Export episode data with step-by-step rewards for reinforcement learning research

## OpenAI Compatibility

- **OpenAI API Format**: Full support for OpenAI-compatible API endpoints via `--openai` flag or config
- **Custom Base URL**: Point to any OpenAI-compatible provider with `--url` flag
- **Multi-Provider**: Switch between Anthropic and OpenAI-compatible backends seamlessly

## Desktop App

SmartClaw ships a native desktop application built with Wails v2:

- **Cross-Platform**: macOS, Windows, Linux
- **Native OS Integration**: System tray icon, native notifications, native window controls
- **Liquid Glass UI**: Modern translucent design language with blur effects and adaptive theming
- **Full Agent Access**: All CLI capabilities available through the desktop interface

## Architecture

### Agent Loop

```
Input --> Reasoning --> Tool Use --> Memory --> Output --> Learning
                                                          |
                                                 Evaluate: worth keeping?
                                                          | Yes
                                                 Extract: reusable method
                                                          |
                                                 Write: skill to disk
                                                          |
                                            Next time: use saved skill
```

### 4-Layer Memory System

| Layer | Name | Storage | Behavior |
|-------|------|---------|----------|
| L1 | Prompt Memory | `MEMORY.md` + `USER.md` | Auto-loaded every session, 3,575 char hard limit |
| L2 | Session Search | SQLite + FTS5 | Agent searches relevant history, LLM-summarized before injection |
| L3 | Skill Procedural | `~/.smartclaw/skills/` | Only loads skill name+description, full content on demand |
| L4 | User Modeling | `user_observations` table | Passively tracks preferences, auto-updates USER.md |

### Learning Loop

```
Task Complete
    |
Evaluator: "Was this approach worth reusing?" (LLM judgment)
    | Yes
Extractor: "What's the reusable method?" (LLM extraction)
    |
SkillWriter: Write SKILL.md to ~/.smartclaw/skills/
    |
Update MEMORY.md with learned pattern
    |
Next similar task -> discovered and used automatically
```

### Speculative Execution

```
User Query
    +-- Fast Model (Haiku) -> result in ~1s
    +-- Heavy Model (Opus) -> result in ~5s
            |
    Compare: similarity > 0.7?
        | Yes              | No
    Use fast result    Use heavy result
```

### Adaptive Model Routing

```
Query Complexity Signals:
  - Message length
  - Tool call count
  - History turn count
  - Code content detection
  - Retry count
  - Skill match
        |
  Complexity Score -> Route to Tier
        |
  fast | default | heavy
```

### War Room Architecture

```
Incident --> Smart Dispatcher --> Phase 1 (Parallel Agents)
                                       |
                               Phase 2 (Reasoning Synthesis)
                                       |
                         Phase 3 (Dynamic Expansion, if needed)
                                       |
                         Phase 4 (Final Analysis)
                                       |
                                 Shared Blackboard
                               (Entries, Hypotheses, Facts)
```

## Quick Start

### Requirements

- Go 1.26+
- Anthropic API key (or OpenAI-compatible API key)

### Installation

```bash
go build -o bin/smartclaw ./cmd/smartclaw/
```

### Basic Usage

```bash
# Start TUI mode (recommended)
./bin/smartclaw tui

# Start simple REPL
./bin/smartclaw repl

# Send a single prompt
./bin/smartclaw prompt "Explain this code"

# Use a specific model
./bin/smartclaw --model claude-opus-4-6 repl

# Start WebUI server
./bin/smartclaw web --port 8080

# Start ACP server for IDE integration
./bin/smartclaw acp

# Start multi-platform gateway
./bin/smartclaw gateway --adapters telegram,web --telegram-token <BOT_TOKEN>

# Run batch evaluation
./bin/smartclaw batch --prompts prompts.jsonl --output trajectories/

# Run RL evaluation loop
./bin/smartclaw rl-eval --tasks tasks.jsonl --metric code_quality --output rl-output/

# Use OpenAI-compatible API
./bin/smartclaw --openai --url https://api.your-provider.com/v1 repl
```

### War Room Usage

```bash
# Start War Room from CLI
./bin/smartclaw warroom --title "GPU训练OOM" --description "多机多卡训练任务频繁OOM"

# Start War Room for a network incident
./bin/smartclaw warroom --title "API延迟飙升" --description "前端服务API延迟从50ms飙升到5s"

# Start War Room for a database issue
./bin/smartclaw warroom --title "MySQL主从延迟" --description "从库复制延迟超过300秒"
```

### Configuration

Set your Anthropic API key:

```bash
export ANTHROPIC_API_KEY=your_key_here
```

Or create `~/.smartclaw/config.yaml`:

```yaml
api_key: your_api_key_here
model: claude-opus-4-6
max_tokens: 4096
permission: ask
log_level: info
openai: false
base_url: ""
show_thinking: true
```

### Data Directory

SmartClaw automatically creates and manages the following under `~/.smartclaw/`:

| Path | Description |
|------|-------------|
| `MEMORY.md` | System memory, auto-updated by the learning loop |
| `USER.md` | User profile, auto-evolved from observations |
| `state.db` | SQLite database with FTS5 index |
| `skills/` | Learned and bundled skills |
| `cron/` | Scheduled task definitions (JSON) |
| `recordings/` | Session recordings (JSONL) |
| `mcp/servers.json` | MCP server configurations |
| `exports/` | Exported sessions |
| `outbox/` | Queued cross-platform messages |

`MEMORY.md` and `USER.md` can be edited directly -- SmartClaw will reload them on next use.

## Available Tools (73+)

### File Operations (7 tools)

`bash`, `read_file`, `write_file`, `edit_file`, `glob`, `grep`, `powershell`

### Code Analysis (4 tools)

`lsp` (goto_definition, find_references, rename, diagnostics), `ast_grep`, `code_search`, `index`

### Web and Browser (10 tools)

`web_fetch`, `web_search`, `browser_navigate`, `browser_click`, `browser_type`, `browser_screenshot`, `browser_extract`, `browser_wait`, `browser_select`, `browser_fill_form`

### MCP Integration (4 tools)

`mcp`, `list_mcp_resources`, `read_mcp_resource`, `mcp_auth`

### Agent and Learning (6 tools)

`agent`, `skill`, `session`, `todowrite`, `config`, `memory`

### Code Execution and Sandboxing (3 tools)

`execute_code`, `docker_exec`, `repl`

### Git Operations (4 tools)

`git_ai`, `git_status`, `git_diff`, `git_log`

### Batch and Parallel (3 tools)

`batch`, `parallel`, `pipeline`

### Team Collaboration (7 tools)

`team_create`, `team_delete`, `team_share_memory`, `team_get_memories`, `team_search_memories`, `team_sync`, `team_share_session`

### Remote and Messaging (2 tools)

`remote_trigger`, `send_message`

### Workflow and Planning (5 tools)

`enter_worktree`, `exit_worktree`, `enter_plan_mode`, `exit_plan_mode`, `schedule_cron`

### Media and Documents (3 tools)

`image`, `pdf`, `audio`

### Cognitive Tools (6 tools)

`think`, `deep_think`, `brief`, `observe`, `lazy`, `fork`

### War Room Collaboration (3 tools)

`warroom_handoff`, `warroom_evaluate`, `warroom_blackboard_write`

### Utility (6 tools)

`tool_search`, `cache`, `attach`, `debug`, `env`, `sleep`

## Slash Commands (101)

### Core (5)

`/help`, `/status`, `/exit`, `/clear`, `/version`

### Model and Config (11)

`/model`, `/model-list`, `/config`, `/config-show`, `/config-set`, `/config-get`, `/config-reset`, `/config-export`, `/config-import`, `/set-api-key`, `/env`

### Session (11)

`/session`, `/resume`, `/save`, `/export`, `/import`, `/rename`, `/fork`, `/rewind`, `/share`, `/summary`, `/attach`

### Compaction (5)

`/compact`, `/compact now`, `/compact auto`, `/compact status`, `/compact config`

### Agent System (10)

`/agent`, `/agent-list`, `/agent-switch`, `/agent-create`, `/agent-delete`, `/agent-info`, `/agent-export`, `/agent-import`, `/subagent`, `/agents`

### Template System (8)

`/template`, `/template-list`, `/template-use`, `/template-create`, `/template-delete`, `/template-info`, `/template-export`, `/template-import`

### Memory and Learning (3)

`/memory`, `/skills`, `/observe`

### MCP (6)

`/mcp`, `/mcp-add`, `/mcp-remove`, `/mcp-list`, `/mcp-start`, `/mcp-stop`

### Git (7)

`/git-status` (`/gs`), `/git-diff` (`/gd`), `/git-commit` (`/gc`), `/git-branch` (`/gb`), `/git-log` (`/gl`), `/diff`, `/commit`

### Tools and Development (10)

`/tools`, `/tasks`, `/lsp`, `/read`, `/write`, `/exec`, `/browse`, `/web`, `/ide`, `/install`

### Diagnostics (9)

`/doctor`, `/cost`, `/stats`, `/usage`, `/debug`, `/inspect`, `/cache`, `/heapdump`, `/reset-limits`

### Planning and Thinking (5)

`/plan`, `/think`, `/deepthink`, `/ultraplan`, `/thinkback`

### Collaboration and Communication (3)

`/invite`, `/feedback`, `/issue`

### UI and Personalization (6)

`/theme`, `/color`, `/vim`, `/keybindings`, `/statusline`, `/stickers`

### Mode Switching (6)

`/fast`, `/lazy`, `/desktop`, `/mobile`, `/chrome`, `/voice`

### Auth and Updates (4)

`/login`, `/logout`, `/upgrade`, `/api`

### Misc (18)

`/init`, `/context`, `/permissions`, `/hooks`, `/plugin`, `/passes`, `/preview`, `/effort`, `/tag`, `/copy`, `/files`, `/advisor`, `/btw`, `/bughunter`, `/insights`, `/onboarding`, `/teleport`, `/summary`

## Project Structure

```
cmd/
  smartclaw/              # Application entrypoint

internal/
  acp/                    # Agent Communication Protocol (IDE integration via JSON-RPC)
  adapters/               # Agent runner adapters
  alertengine/            # Alert correlation and fingerprinting
  analytics/              # Usage analytics and reporting
  api/                    # API client with prompt caching + OpenAI support
  assistant/              # Assistant personality and behavior
  auth/                   # OAuth authentication
  autonomous/             # Autonomous agent loop
  autoremediation/        # Automated remediation with safety checks
  batch/                  # Batch runner for parallel prompt execution
  bootstrap/              # Bootstrap and first-run
  bridge/                 # Bridge adapters
  buddy/                  # Buddy system for guided assistance
  cache/                  # Caching system with dependency tracking
  chain/                  # Chain-of-thought management
  changerisk/             # Change risk analysis
  cli/                    # CLI commands (repl, tui, web, acp, batch, rl-eval, gateway, warroom)
  commands/               # 101 Slash commands
  compact/                # Compaction service (auto, micro, time-based)
  components/             # Reusable TUI components
  config/                 # Configuration management
  constants/              # Shared constants
  coordinator/            # Task coordination
  costguard/              # Budget-aware spending guard with model downgrade
  dap/                    # Debug Adapter Protocol
  diffengine/             # Diff computation engine
  entrypoints/            # Application entrypoint variants
  fingerprint/            # Service fingerprinting
  gateway/                # Unified gateway (router, delivery, cron)
    platform/             # Platform adapters (terminal, web, telegram)
  git/                    # Git context and operations
  github/                 # GitHub integration
  history/                # Command history
  hooks/                  # Hook system
  keybindings/            # Keybinding configuration
  learning/               # Learning loop (evaluator, extractor, skill writer, nudge)
  lifecycle/              # Lifecycle management
  logger/                 # Structured logging
  mcp/                    # MCP protocol (client, transport, auth, registry, enhanced)
  memdir/                 # Memory directory management
  memory/                 # Memory manager (4-layer coordination)
    layers/               # L1 Prompt, L2 Session Search, L3 Skill, L4 User Model
  migrations/             # Database migrations
  models/                 # Data models
  native/                 # Native platform bindings
  native_ts/              # TypeScript native bindings
  observability/          # Metrics, tracing, and telemetry
  operator/               # Kubernetes operator
  outputstyles/           # Output formatting styles
  patch/                  # Patch management
  permissions/            # Permission engine (4 modes)
  plugins/                # Plugin system
  pool/                   # Connection pool management
  process/                # Process management
  provider/               # Multi-provider API abstraction
  query/                  # Query engine
  remote/                 # Remote execution
  repomap/                # Repository map generation
  resilience/             # Resilience patterns
  rl/                     # Reinforcement learning evaluation environment
  routing/                # Adaptive model routing + speculative execution
  runtime/                # Query engine, compaction, session
  sandbox/                # Sandboxed execution (Linux namespaces, RPC)
  schemas/                # JSON schemas for tool inputs
  screens/                # Screen layout management
  security/               # Security scanning
  server/                 # Direct connect server
  serverauth/             # Server authentication
  services/               # Shared services (recorder, playback, sync, LSP, OAuth, voice, compact, analytics, rate limit)
  session/                # Session management
  shadow/                 # Shadow mode execution
  skills/                 # Skills system (bundled + learned)
  srecoder/               # Session recorder
  state/                  # Application state
  store/                  # SQLite persistence (WAL, FTS5, JSONL backup)
  template/               # Prompt template engine
  timetravel/             # Git time-travel debugging
  tools/                  # Tool implementations (73+ tools)
  topology/               # Dependency graph and blast radius analysis
  transports/             # Transport layer abstractions
  tui/                    # Terminal UI (Bubble Tea)
  types/                  # Shared type definitions
  upstreamproxy/          # Upstream API proxy
  utils/                  # Utility functions
  verifyfix/              # Fix verification
  vim/                    # Vim mode support
  voice/                  # Voice input/output
  watcher/                # File system watcher
  watchdog/               # Continuous monitoring and alerting
  warroom/                # War Room multi-agent collaborative fault diagnosis
  web/                    # Web UI + WebSocket server
  wiki/                   # Wiki/knowledge base
  worktree/               # Git worktree management

pkg/
  output/                 # Shared output formatting
  progress/               # Progress bar utilities

desktop/                  # Wails v2 desktop application (macOS/Windows/Linux)

extensions/
  vscode/                 # VS Code extension (chat sidebar, code actions)
```

## VS Code Extension

SmartClaw ships with a VS Code extension that connects via ACP (Agent Communication Protocol):

| Command | Description |
|---------|-------------|
| `SmartClaw: Ask` | Ask a question to SmartClaw |
| `SmartClaw: Open Chat` | Open the chat sidebar |
| `SmartClaw: Explain Code` | Explain selected code |
| `SmartClaw: Fix Code` | Fix issues in selected code |
| `SmartClaw: Generate Tests` | Generate tests for selected code |

### Installation

1. Build SmartClaw: `go build -o bin/smartclaw ./cmd/smartclaw/`
2. Add `smartclaw` to your PATH
3. Install the extension from `extensions/vscode/`
4. Open the SmartClaw sidebar in Explorer

## API Usage

```go
package main

import (
    "context"
    "fmt"

    "github.com/instructkr/smartclaw/internal/api"
    "github.com/instructkr/smartclaw/internal/gateway"
    "github.com/instructkr/smartclaw/internal/learning"
    "github.com/instructkr/smartclaw/internal/memory"
    "github.com/instructkr/smartclaw/internal/runtime"
)

func main() {
    client := api.NewClient("your-api-key")
    memManager, _ := memory.NewMemoryManager()
    learningLoop := learning.NewLearningLoop(nil, memManager.GetPromptMemory(), "")

    engineFactory := func() *runtime.QueryEngine {
        return runtime.NewQueryEngine(client, runtime.QueryConfig{})
    }

    gw := gateway.NewGateway(engineFactory, memManager, learningLoop)
    defer gw.Close()

    resp, err := gw.HandleMessage(context.Background(), "user-1", "terminal", "Hello!")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Content)
}
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | API key for Anthropic |
| `SMARTCLAW_MODEL` | Default model to use |
| `SMARTCLAW_CONFIG` | Path to config file |
| `SMARTCLAW_SESSION_DIR` | Directory for session storage |
| `SMARTCLAW_LOG_LEVEL` | Log level (debug, info, warn, error) |

## Testing

```bash
# Run all tests
go test ./...

# Run specific packages
go test ./internal/learning/...
go test ./internal/store/...
go test ./internal/memory/layers/...
go test ./internal/tools/...
go test ./internal/services/...
go test ./internal/sandbox/...
go test ./internal/compact/...
go test ./internal/routing/...
go test ./internal/costguard/...
go test ./internal/acp/...
go test ./internal/observability/...
go test ./internal/warroom/...

# Run with coverage
go test -cover ./...
```

## License

MIT License

---

<a id="中文"></a>

# SmartClaw

AI驱动的SRE/Ops平台，内置自我进化的智能体，持续学习你的工作方式。

SmartClaw远不止编码助手。它是一个全栈SRE平台，具备协作式多智能体故障诊断（War Room）、告警关联、自动修复、拓扑感知的爆炸半径分析，以及自我进化的学习循环。用得越多，它越懂你的工作方式和系统行为。

## 核心理念

**"越来越懂你的工作方式"** -- 不是"在所有事情上更好"，而是"在你的所有事情上更好"。

不同于会话间遗忘一切的通用AI助手，SmartClaw：

- **从已完成的任务中学习** -- 评估方法是否值得复用，如果是，提取为技能
- **跨会话记忆** -- 基于SQLite + FTS5全文搜索的4层记忆系统
- **持续自我进化** -- 周期性提醒触发智能体整合记忆、优化技能
- **理解你的偏好** -- 被动追踪沟通风格、知识背景和常见工作流
- **协作式故障诊断** -- War Room调度领域专家智能体并行调查SRE故障

## War Room：协作式多智能体故障诊断

War Room是SmartClaw的旗舰SRE能力。当故障发生时，它编排多个领域专家智能体并行调查、综合发现、收敛于根因分析。

### 8个领域专家智能体

| 智能体 | 领域 | 专长 |
|--------|------|------|
| Network | 网络 | TCP/UDP、DNS、负载均衡、防火墙、抓包分析 |
| Database | 数据库 | MySQL、PostgreSQL、Redis、MongoDB、查询性能、复制延迟 |
| Infrastructure | 基础设施 | Kubernetes、Docker、CPU/内存/磁盘、内核、systemd |
| Application | 应用 | 代码级错误、内存泄漏、线程死锁、性能瓶颈 |
| Security | 安全 | 入侵检测、权限提升、配置失误、CVE漏洞 |
| Reasoning | 推理 | 综合分析、交叉验证、置信度演化、最终结论 |
| Training | 训练 | 多机多卡训练、NCCL、Megatron、DeepSpeed、OOM |
| Inference | 推理 | vLLM、SGLang、TensorRT-LLM、服务延迟、GPU显存、批处理 |

### 4阶段分步执行

```
故障事件 --> 智能调度器 --> 阶段1（并行智能体调查）
                              |
                      阶段2（推理综合）
                              |
                阶段3（动态扩展，如需要）
                              |
                      阶段4（最终分析）
                              |
                        共享黑板
                   （条目、假设、事实）
```

- **阶段1：并行调查** -- 智能调度器分析故障关键词，调度相关领域智能体。每个智能体独立调查并将发现写入共享黑板。
- **阶段2：推理综合** -- Reasoning智能体读取所有黑板条目，整合发现，识别模式，形成初步假设。
- **阶段3：动态扩展** -- 如果Reasoning智能体检测到信息缺口或矛盾证据，它推荐额外的领域智能体加入调查。
- **阶段4：最终分析** -- Reasoning智能体产出包含置信度分数、支持证据和修复建议的综合根因报告。

### 共享黑板

所有智能体共享可变上下文。黑板包含：

- **条目（Entries）** -- 各智能体的调查发现，带时间戳和归属
- **假设（Hypotheses）** -- 提出的根因，附支持和矛盾证据
- **共享事实（Shared Facts）** -- 所有智能体一致确认的事实

### 智能体交接协议

智能体可以在执行过程中使用`warroom_handoff`工具请求其他智能体协助。请求方指定所需领域专长，交接超时时间为30秒。

### 交叉验证与置信度演化

发现会在智能体之间交叉验证。当多个智能体产生匹配证据时，置信度增加。当证据矛盾时，置信度降低。置信度限制在[0.1, 0.95]区间内，防止过早确定或完全否定。

### 3个智能体协作工具

| 工具 | 描述 |
|------|------|
| `warroom_handoff` | 请求其他领域智能体协助调查 |
| `warroom_evaluate` | 评估和交叉验证其他智能体的发现 |
| `warroom_blackboard_write` | 向共享黑板写入条目、假设或事实 |

### 智能调度器

调度器使用基于关键词的智能体选择。故障描述根据每个智能体的关键词画像进行评分，超过相关性阈值的智能体在阶段1被调度。Reasoning智能体可以在阶段3请求额外智能体。

## SRE专项功能

### 告警引擎（alertengine）

告警关联与指纹识别。将相关告警聚合为事件，去除重复噪音，生成稳定指纹用于追踪告警在会话间的复现。

### 自动修复（autoremediation）

带安全检查的自动修复。执行预定义或AI生成的修复动作，具备预检验证、回滚能力和高风险操作的审批门控。

### 拓扑（topology）

爆炸半径分析与依赖图谱。映射服务依赖关系，计算组件故障时的影响范围，可视化级联故障的传播路径。

### 看门狗（watchdog）

持续监控与告警。对基础设施和应用端点执行周期性健康检查，阈值违规时触发告警，并将数据送入告警引擎进行关联。

### 运维手册（playbook）

运维手册自动化。将手动SRE运维手册转化为可执行、版本控制的playbook，支持参数化步骤、条件分支和审计日志。

## 智能体能力

- **学习循环**：任务后评估、方法提取、技能创建、MEMORY.md自动更新
- **4层记忆**：提示记忆（MEMORY.md/USER.md）、会话搜索（FTS5）、技能过程（懒加载）、用户建模
- **周期性提醒**：每10轮系统触发的自我审查（可配置）
- **智能压缩**：可配置阈值的自动压缩、头部保护、工具结果裁剪、可溯源摘要
- **推测执行**：双模型路由，快速模型+重型模型并行运行，相似则采用快速结果，分歧则采用重型结果
- **自适应模型路由**：基于复杂度的模型选择（fast/default/heavy），支持成本优先、质量优先或均衡策略
- **成本守护**：预算感知的支出管理，日/会话限额、预警阈值、接近限额时自动降级模型

## 开发工具

- **73+内置工具**：文件操作、代码分析、Web工具、MCP集成、浏览器自动化、Docker沙箱等
- **101斜杠命令**：完整的效率命令套件，包含智能体管理、模板系统和IDE集成
- **现代TUI**：基于Bubble Tea构建的终端用户界面
- **交互式REPL**：完整的对话历史与流式响应
- **MCP集成**：连接MCP服务器、发现工具、读取资源、通过OAuth认证
- **ACP服务器**：通过stdio JSON-RPC实现IDE集成的智能体通信协议（VS Code、Zed、JetBrains）
- **VS Code扩展**：官方扩展，提供聊天侧边栏、代码解释、修复和测试生成命令
- **安全**：4模式权限系统、Linux沙箱执行、Docker隔离
- **令牌追踪**：实时成本估算，阈值自动压缩

## 浏览器自动化

- **无头浏览器**：使用Chromium（通过chromedp）导航、点击、输入、截图、提取内容、填写表单
- **8个浏览器工具**：`browser_navigate`、`browser_click`、`browser_type`、`browser_screenshot`、`browser_extract`、`browser_wait`、`browser_select`、`browser_fill_form`

## 代码执行与沙箱

- **执行代码工具**：在RPC沙箱中运行Python代码，可直接访问SmartClaw工具，将多轮工作流压缩为单轮
- **Docker沙箱**：隔离容器执行，项目目录挂载到`/workspace`，支持一次性与会话持久容器
- **Linux命名空间沙箱**：使用Linux命名空间的原生沙箱执行，提供安全隔离

## 网关与跨平台

- **统一网关**：消息、路由、记忆、执行、学习、交付
- **平台适配器**：终端、Web UI、Telegram，可扩展至Discord
- **定时任务**：计划任务作为一等智能体任务，拥有完整记忆访问
- **会话路由**：基于userID的路由，非基于平台，切换设备不丢失上下文
- **会话录制**：录制和回放完整会话，用于审计和复盘
- **远程触发**：通过SSH在远程主机执行命令

## 团队协作

- **团队工作区**：创建共享团队空间，使用AES加密记忆同步
- **团队记忆共享**：跨团队成员共享记忆、会话和知识
- **团队工具**：`team_create`、`team_delete`、`team_share_memory`、`team_get_memories`、`team_search_memories`、`team_sync`、`team_share_session`

## 可观测性与分析

- **指标仪表板**：实时查询数、缓存命中率、令牌使用、成本估算、工具执行统计、每模型查询数
- **分布式追踪**：请求级追踪，用于调试延迟和故障
- **遥测API**：REST端点（`/api/telemetry`）暴露完整可观测性数据

## 批量与RL评估

- **批量运行器**：跨数百个提示并行执行智能体，输出ShareGPT格式训练轨迹
- **RL评估**：运行基于奖励的评估循环，支持可配置指标（exact_match、code_quality、length_penalty）
- **轨迹导出**：导出含逐步奖励的事件数据，用于强化学习研究

## OpenAI兼容性

- **OpenAI API格式**：通过`--openai`标志或配置全面支持OpenAI兼容API端点
- **自定义Base URL**：使用`--url`标志指向任何OpenAI兼容提供商
- **多提供商**：在Anthropic和OpenAI兼容后端之间无缝切换

## 桌面应用

SmartClaw提供基于Wails v2构建的原生桌面应用：

- **跨平台**：macOS、Windows、Linux
- **原生OS集成**：系统托盘图标、原生通知、原生窗口控制
- **液态玻璃UI**：现代半透明设计语言，毛玻璃效果与自适应主题
- **完整智能体访问**：所有CLI功能均可在桌面界面使用

## 架构

### 智能体循环

```
输入 --> 推理 --> 工具使用 --> 记忆 --> 输出 --> 学习
                                                   |
                                          评估：值得保留？
                                                   | 是
                                          提取：可复用方法
                                                   |
                                          写入：技能到磁盘
                                                   |
                                     下次：使用已保存技能
```

### 4层记忆系统

| 层级 | 名称 | 存储 | 行为 |
|------|------|------|------|
| L1 | 提示记忆 | `MEMORY.md` + `USER.md` | 每次会话自动加载，3,575字符硬限制 |
| L2 | 会话搜索 | SQLite + FTS5 | 智能体搜索相关历史，LLM摘要后注入 |
| L3 | 技能过程 | `~/.smartclaw/skills/` | 仅加载技能名称+描述，按需加载完整内容 |
| L4 | 用户建模 | `user_observations`表 | 被动追踪偏好，自动更新USER.md |

### 学习循环

```
任务完成
    |
评估器："这个方法值得复用吗？"（LLM判断）
    | 是
提取器："可复用的方法是什么？"（LLM提取）
    |
技能写入器：将SKILL.md写入~/.smartclaw/skills/
    |
用学到的模式更新MEMORY.md
    |
下一个相似任务 -> 自动发现并使用
```

### 推测执行

```
用户查询
    +-- 快速模型（Haiku）-> 约1秒返回结果
    +-- 重型模型（Opus）-> 约5秒返回结果
            |
    比较：相似度 > 0.7？
        | 是              | 否
    使用快速结果    使用重型结果
```

### 自适应模型路由

```
查询复杂度信号：
  - 消息长度
  - 工具调用次数
  - 历史轮次
  - 代码内容检测
  - 重试次数
  - 技能匹配
        |
  复杂度分数 -> 路由到层级
        |
  fast | default | heavy
```

### War Room架构

```
故障事件 --> 智能调度器 --> 阶段1（并行智能体调查）
                              |
                      阶段2（推理综合）
                              |
                阶段3（动态扩展，如需要）
                              |
                      阶段4（最终分析）
                              |
                        共享黑板
                   （条目、假设、事实）
```

## 快速开始

### 环境要求

- Go 1.26+
- Anthropic API密钥（或OpenAI兼容API密钥）

### 安装

```bash
go build -o bin/smartclaw ./cmd/smartclaw/
```

### 基本用法

```bash
# 启动TUI模式（推荐）
./bin/smartclaw tui

# 启动简单REPL
./bin/smartclaw repl

# 发送单次提示
./bin/smartclaw prompt "解释这段代码"

# 使用指定模型
./bin/smartclaw --model claude-opus-4-6 repl

# 启动WebUI服务器
./bin/smartclaw web --port 8080

# 启动ACP服务器用于IDE集成
./bin/smartclaw acp

# 启动多平台网关
./bin/smartclaw gateway --adapters telegram,web --telegram-token <BOT_TOKEN>

# 运行批量评估
./bin/smartclaw batch --prompts prompts.jsonl --output trajectories/

# 运行RL评估循环
./bin/smartclaw rl-eval --tasks tasks.jsonl --metric code_quality --output rl-output/

# 使用OpenAI兼容API
./bin/smartclaw --openai --url https://api.your-provider.com/v1 repl
```

### War Room用法

```bash
# 从CLI启动War Room
./bin/smartclaw warroom --title "GPU训练OOM" --description "多机多卡训练任务频繁OOM"

# 启动War Room处理网络故障
./bin/smartclaw warroom --title "API延迟飙升" --description "前端服务API延迟从50ms飙升到5s"

# 启动War Room处理数据库问题
./bin/smartclaw warroom --title "MySQL主从延迟" --description "从库复制延迟超过300秒"
```

### 配置

设置Anthropic API密钥：

```bash
export ANTHROPIC_API_KEY=your_key_here
```

或创建`~/.smartclaw/config.yaml`：

```yaml
api_key: your_api_key_here
model: claude-opus-4-6
max_tokens: 4096
permission: ask
log_level: info
openai: false
base_url: ""
show_thinking: true
```

### 数据目录

SmartClaw在`~/.smartclaw/`下自动创建和管理以下内容：

| 路径 | 描述 |
|------|------|
| `MEMORY.md` | 系统记忆，由学习循环自动更新 |
| `USER.md` | 用户画像，从观察中自动演化 |
| `state.db` | 带FTS5索引的SQLite数据库 |
| `skills/` | 已学习和捆绑的技能 |
| `cron/` | 计划任务定义（JSON） |
| `recordings/` | 会话录制（JSONL） |
| `mcp/servers.json` | MCP服务器配置 |
| `exports/` | 导出的会话 |
| `outbox/` | 待发送的跨平台消息 |

`MEMORY.md`和`USER.md`可以直接编辑，SmartClaw会在下次使用时重新加载。

## 可用工具（73+）

### 文件操作（7个工具）

`bash`、`read_file`、`write_file`、`edit_file`、`glob`、`grep`、`powershell`

### 代码分析（4个工具）

`lsp`（goto_definition、find_references、rename、diagnostics）、`ast_grep`、`code_search`、`index`

### Web与浏览器（10个工具）

`web_fetch`、`web_search`、`browser_navigate`、`browser_click`、`browser_type`、`browser_screenshot`、`browser_extract`、`browser_wait`、`browser_select`、`browser_fill_form`

### MCP集成（4个工具）

`mcp`、`list_mcp_resources`、`read_mcp_resource`、`mcp_auth`

### 智能体与学习（6个工具）

`agent`、`skill`、`session`、`todowrite`、`config`、`memory`

### 代码执行与沙箱（3个工具）

`execute_code`、`docker_exec`、`repl`

### Git操作（4个工具）

`git_ai`、`git_status`、`git_diff`、`git_log`

### 批量与并行（3个工具）

`batch`、`parallel`、`pipeline`

### 团队协作（7个工具）

`team_create`、`team_delete`、`team_share_memory`、`team_get_memories`、`team_search_memories`、`team_sync`、`team_share_session`

### 远程与消息（2个工具）

`remote_trigger`、`send_message`

### 工作流与规划（5个工具）

`enter_worktree`、`exit_worktree`、`enter_plan_mode`、`exit_plan_mode`、`schedule_cron`

### 媒体与文档（3个工具）

`image`、`pdf`、`audio`

### 认知工具（6个工具）

`think`、`deep_think`、`brief`、`observe`、`lazy`、`fork`

### War Room协作（3个工具）

`warroom_handoff`、`warroom_evaluate`、`warroom_blackboard_write`

### 实用工具（6个工具）

`tool_search`、`cache`、`attach`、`debug`、`env`、`sleep`

## 斜杠命令（101）

### 核心（5个）

`/help`、`/status`、`/exit`、`/clear`、`/version`

### 模型与配置（11个）

`/model`、`/model-list`、`/config`、`/config-show`、`/config-set`、`/config-get`、`/config-reset`、`/config-export`、`/config-import`、`/set-api-key`、`/env`

### 会话（11个）

`/session`、`/resume`、`/save`、`/export`、`/import`、`/rename`、`/fork`、`/rewind`、`/share`、`/summary`、`/attach`

### 压缩（5个）

`/compact`、`/compact now`、`/compact auto`、`/compact status`、`/compact config`

### 智能体系统（10个）

`/agent`、`/agent-list`、`/agent-switch`、`/agent-create`、`/agent-delete`、`/agent-info`、`/agent-export`、`/agent-import`、`/subagent`、`/agents`

### 模板系统（8个）

`/template`、`/template-list`、`/template-use`、`/template-create`、`/template-delete`、`/template-info`、`/template-export`、`/template-import`

### 记忆与学习（3个）

`/memory`、`/skills`、`/observe`

### MCP（6个）

`/mcp`、`/mcp-add`、`/mcp-remove`、`/mcp-list`、`/mcp-start`、`/mcp-stop`

### Git（7个）

`/git-status`（`/gs`）、`/git-diff`（`/gd`）、`/git-commit`（`/gc`）、`/git-branch`（`/gb`）、`/git-log`（`/gl`）、`/diff`、`/commit`

### 工具与开发（10个）

`/tools`、`/tasks`、`/lsp`、`/read`、`/write`、`/exec`、`/browse`、`/web`、`/ide`、`/install`

### 诊断（9个）

`/doctor`、`/cost`、`/stats`、`/usage`、`/debug`、`/inspect`、`/cache`、`/heapdump`、`/reset-limits`

### 规划与思考（5个）

`/plan`、`/think`、`/deepthink`、`/ultraplan`、`/thinkback`

### 协作与通信（3个）

`/invite`、`/feedback`、`/issue`

### UI与个性化（6个）

`/theme`、`/color`、`/vim`、`/keybindings`、`/statusline`、`/stickers`

### 模式切换（6个）

`/fast`、`/lazy`、`/desktop`、`/mobile`、`/chrome`、`/voice`

### 认证与更新（4个）

`/login`、`/logout`、`/upgrade`、`/api`

### 其他（18个）

`/init`、`/context`、`/permissions`、`/hooks`、`/plugin`、`/passes`、`/preview`、`/effort`、`/tag`、`/copy`、`/files`、`/advisor`、`/btw`、`/bughunter`、`/insights`、`/onboarding`、`/teleport`、`/summary`

## 项目结构

```
cmd/
  smartclaw/              # 应用入口

internal/
  acp/                    # 智能体通信协议（通过JSON-RPC的IDE集成）
  adapters/               # 智能体运行器适配器
  alertengine/            # 告警关联与指纹识别
  analytics/              # 使用分析与报告
  api/                    # 带提示缓存+OpenAI支持的API客户端
  assistant/              # 助手个性与行为
  auth/                   # OAuth认证
  autonomous/             # 自主智能体循环
  autoremediation/        # 带安全检查的自动修复
  batch/                  # 并行提示执行的批量运行器
  bootstrap/              # 引导与首次运行
  bridge/                 # 桥接适配器
  buddy/                  # 引导式助手系统
  cache/                  # 带依赖追踪的缓存系统
  chain/                  # 思维链管理
  changerisk/             # 变更风险分析
  cli/                    # CLI命令（repl、tui、web、acp、batch、rl-eval、gateway、warroom）
  commands/               # 101个斜杠命令
  compact/                # 压缩服务（自动、微、基于时间）
  components/             # 可复用TUI组件
  config/                 # 配置管理
  constants/              # 共享常量
  coordinator/            # 任务协调
  costguard/              # 带模型降级的预算感知支出守护
  dap/                    # 调试适配器协议
  diffengine/             # 差异计算引擎
  entrypoints/            # 应用入口变体
  fingerprint/            # 服务指纹识别
  gateway/                # 统一网关（路由、交付、定时任务）
    platform/             # 平台适配器（终端、Web、Telegram）
  git/                    # Git上下文与操作
  github/                 # GitHub集成
  history/                # 命令历史
  hooks/                  # 钩子系统
  keybindings/            # 键绑定配置
  learning/               # 学习循环（评估器、提取器、技能写入器、提醒）
  lifecycle/              # 生命周期管理
  logger/                 # 结构化日志
  mcp/                    # MCP协议（客户端、传输、认证、注册、增强）
  memdir/                 # 记忆目录管理
  memory/                 # 记忆管理器（4层协调）
    layers/               # L1提示、L2会话搜索、L3技能、L4用户模型
  migrations/             # 数据库迁移
  models/                 # 数据模型
  native/                 # 原生平台绑定
  native_ts/              # TypeScript原生绑定
  observability/          # 指标、追踪和遥测
  operator/               # Kubernetes Operator
  outputstyles/           # 输出格式化样式
  patch/                  # 补丁管理
  permissions/            # 权限引擎（4种模式）
  plugins/                # 插件系统
  pool/                   # 连接池管理
  process/                # 进程管理
  provider/               # 多提供商API抽象
  query/                  # 查询引擎
  remote/                 # 远程执行
  repomap/                # 仓库地图生成
  resilience/             # 弹性模式
  rl/                     # 强化学习评估环境
  routing/                # 自适应模型路由+推测执行
  runtime/                # 查询引擎、压缩、会话
  sandbox/                # 沙箱执行（Linux命名空间、RPC）
  schemas/                # 工具输入的JSON模式
  screens/                # 屏幕布局管理
  security/               # 安全扫描
  server/                 # 直连服务器
  serverauth/             # 服务器认证
  services/               # 共享服务（录制器、回放、同步、LSP、OAuth、语音、压缩、分析、速率限制）
  session/                # 会话管理
  shadow/                 # 影子模式执行
  skills/                 # 技能系统（捆绑+学习）
  srecoder/               # 会话录制器
  state/                  # 应用状态
  store/                  # SQLite持久化（WAL、FTS5、JSONL备份）
  template/               # 提示模板引擎
  timetravel/             # Git时间旅行调试
  tools/                  # 工具实现（73+工具）
  topology/               # 依赖图谱与爆炸半径分析
  transports/             # 传输层抽象
  tui/                    # 终端UI（Bubble Tea）
  types/                  # 共享类型定义
  upstreamproxy/          # 上游API代理
  utils/                  # 工具函数
  verifyfix/              # 修复验证
  vim/                    # Vim模式支持
  voice/                  # 语音输入/输出
  watcher/                # 文件系统监视器
  watchdog/               # 持续监控与告警
  warroom/                # War Room多智能体协作故障诊断
  web/                    # Web UI + WebSocket服务器
  wiki/                   # Wiki/知识库
  worktree/               # Git工作树管理

pkg/
  output/                 # 共享输出格式化
  progress/               # 进度条工具

desktop/                  # Wails v2桌面应用（macOS/Windows/Linux）

extensions/
  vscode/                 # VS Code扩展（聊天侧边栏、代码操作）
```

## VS Code扩展

SmartClaw附带通过ACP（智能体通信协议）连接的VS Code扩展：

| 命令 | 描述 |
|------|------|
| `SmartClaw: Ask` | 向SmartClaw提问 |
| `SmartClaw: Open Chat` | 打开聊天侧边栏 |
| `SmartClaw: Explain Code` | 解释选中的代码 |
| `SmartClaw: Fix Code` | 修复选中代码中的问题 |
| `SmartClaw: Generate Tests` | 为选中代码生成测试 |

### 安装

1. 构建SmartClaw：`go build -o bin/smartclaw ./cmd/smartclaw/`
2. 将`smartclaw`添加到PATH
3. 从`extensions/vscode/`安装扩展
4. 在资源管理器中打开SmartClaw侧边栏

## API用法

```go
package main

import (
    "context"
    "fmt"

    "github.com/instructkr/smartclaw/internal/api"
    "github.com/instructkr/smartclaw/internal/gateway"
    "github.com/instructkr/smartclaw/internal/learning"
    "github.com/instructkr/smartclaw/internal/memory"
    "github.com/instructkr/smartclaw/internal/runtime"
)

func main() {
    client := api.NewClient("your-api-key")
    memManager, _ := memory.NewMemoryManager()
    learningLoop := learning.NewLearningLoop(nil, memManager.GetPromptMemory(), "")

    engineFactory := func() *runtime.QueryEngine {
        return runtime.NewQueryEngine(client, runtime.QueryConfig{})
    }

    gw := gateway.NewGateway(engineFactory, memManager, learningLoop)
    defer gw.Close()

    resp, err := gw.HandleMessage(context.Background(), "user-1", "terminal", "Hello!")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Content)
}
```

## 环境变量

| 变量 | 描述 |
|------|------|
| `ANTHROPIC_API_KEY` | Anthropic API密钥 |
| `SMARTCLAW_MODEL` | 默认使用的模型 |
| `SMARTCLAW_CONFIG` | 配置文件路径 |
| `SMARTCLAW_SESSION_DIR` | 会话存储目录 |
| `SMARTCLAW_LOG_LEVEL` | 日志级别（debug、info、warn、error） |

## 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包
go test ./internal/learning/...
go test ./internal/store/...
go test ./internal/memory/layers/...
go test ./internal/tools/...
go test ./internal/services/...
go test ./internal/sandbox/...
go test ./internal/compact/...
go test ./internal/routing/...
go test ./internal/costguard/...
go test ./internal/acp/...
go test ./internal/observability/...
go test ./internal/warroom/...

# 运行带覆盖率的测试
go test -cover ./...
```

## 许可证

MIT License
