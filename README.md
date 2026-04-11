# SmartClaw

A self-improving AI agent that learns your workflow over time.

SmartClaw is an autonomous coding agent that grows smarter with every task. It features a learning loop that evaluates completed tasks, extracts reusable methods, and automatically creates skills. The more you use it, the better it understands how you work.

## Core Philosophy

**"越来越懂你的工作方式"** — Not "better at everything", but "better at *your* everything".

Unlike generic AI assistants that forget everything between sessions, SmartClaw:

- **Learns from completed tasks** — evaluates whether an approach is worth reusing, and if so, extracts it as a skill
- **Remembers across sessions** — 4-layer memory system with SQLite + FTS5 full-text search
- **Self-improves over time** — periodic nudges trigger the agent to consolidate memory and refine skills
- **Understands your preferences** — passively tracks communication style, knowledge background, and common workflows

## Features

### Agent Capabilities

- **Learning Loop**: Post-task evaluation → method extraction → skill creation → MEMORY.md auto-update
- **4-Layer Memory**: Prompt Memory (MEMORY.md/USER.md), Session Search (FTS5), Skill Procedural (lazy load), User Modeling
- **Periodic Nudge**: System-triggered self-review every 10 turns (configurable)
- **Smart Compaction**: Auto-compact with configurable thresholds, head protection, tool result pruning, and source-traceable summaries
- **Speculative Execution**: Dual-model routing — run fast + heavy models in parallel, accept fast result if similar, fall back to heavy if divergent
- **Adaptive Model Router**: Complexity-based model selection (fast/default/heavy) with cost-first, quality-first, or balanced strategies
- **Cost Guard**: Budget-aware spending with daily/session limits, warning thresholds, and automatic model downgrade when approaching limits

### Development Tools

- **73+ Built-in Tools**: File operations, code analysis, web tools, MCP integration, browser automation, Docker sandboxing, and more
- **101 Slash Commands**: Full productivity command suite with agent management, template system, and IDE integration
- **Modern TUI**: Terminal User Interface built with Bubble Tea
- **Interactive REPL**: Full conversation history with streaming responses
- **MCP Integration**: Connect to MCP servers, discover tools, read resources, and authenticate via OAuth
- **ACP Server**: Agent Communication Protocol for IDE integration (VS Code, Zed, JetBrains) via stdio JSON-RPC
- **VS Code Extension**: Official extension with chat sidebar, code explanation, fix, and test generation commands
- **Secure**: Permission system with 4 modes, sandboxed execution on Linux, Docker isolation
- **Token Tracking**: Real-time cost estimation with auto-compact at threshold

### Browser Automation

- **Headless Browser**: Navigate, click, type, screenshot, extract content, and fill forms using Chromium (via chromedp)
- **8 Browser Tools**: `browser_navigate`, `browser_click`, `browser_type`, `browser_screenshot`, `browser_extract`, `browser_wait`, `browser_select`, `browser_fill_form`

### Code Execution & Sandboxing

- **Execute Code Tool**: Run Python code in an RPC sandbox with direct access to SmartClaw tools (read_file, write_file, glob, grep, bash, web_search, web_fetch) — collapses multi-turn workflows into a single turn
- **Docker Sandbox**: Isolated container execution with project directory mounted at `/workspace`, supporting both one-shot and session-persistent containers
- **Linux Namespace Sandbox**: Native sandboxed execution using Linux namespaces for secure isolation

### Gateway & Cross-Platform

- **Unified Gateway**: Message → Route → Memory → Execute → Learn → Deliver
- **Platform Adapters**: Terminal, Web UI, Telegram, extensible to Discord
- **Cron Tasks**: Scheduled tasks as first-class agent tasks with full memory access
- **Session Routing**: userID-based routing, not platform-based — switch devices without losing context
- **Session Recording**: Record and replay full sessions for audit and review
- **Remote Trigger**: Execute commands on remote hosts via SSH

### Team Collaboration

- **Team Workspaces**: Create shared team spaces with AES-encrypted memory sync
- **Team Memory Sharing**: Share memories, sessions, and knowledge across team members
- **Team Tools**: `team_create`, `team_delete`, `team_share_memory`, `team_get_memories`, `team_search_memories`, `team_sync`, `team_share_session`

### Observability & Analytics

- **Metrics Dashboard**: Real-time query count, cache hit rate, token usage, cost estimation, tool execution stats, and per-model query counts
- **Distributed Tracing**: Request-level tracing for debugging latency and failures
- **Telemetry API**: REST endpoint (`/api/telemetry`) exposing full observability data

### Batch & RL Evaluation

- **Batch Runner**: Execute agent across hundreds of prompts in parallel, output ShareGPT-format training trajectories
- **RL Evaluation**: Run reward-based evaluation loops with configurable metrics (exact_match, code_quality, length_penalty)
- **Trajectory Export**: Export episode data with step-by-step rewards for reinforcement learning research

### OpenAI Compatibility

- **OpenAI API Format**: Full support for OpenAI-compatible API endpoints via `--openai` flag or config
- **Custom Base URL**: Point to any OpenAI-compatible provider with `--url` flag
- **Multi-Provider**: Switch between Anthropic and OpenAI-compatible backends seamlessly

## Architecture

```
Input → Reasoning → Tool Use → Memory → Output → Learning
                                                   ↓
                                          Evaluate: worth keeping?
                                                   ↓ Yes
                                          Extract: reusable method
                                                   ↓
                                          Write: skill to disk
                                                   ↓
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
    ↓
Evaluator: "Was this approach worth reusing?" (LLM judgment)
    ↓ Yes
Extractor: "What's the reusable method?" (LLM extraction)
    ↓
SkillWriter: Write SKILL.md to ~/.smartclaw/skills/
    ↓
Update MEMORY.md with learned pattern
    ↓
Next similar task → discovered and used automatically
```

### Speculative Execution

```
User Query
    ├── Fast Model (Haiku) → result in ~1s
    └── Heavy Model (Opus) → result in ~5s
            ↓
    Compare: similarity > 0.7?
        ↓ Yes              ↓ No
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
        ↓
  Complexity Score → Route to Tier
        ↓
  fast | default | heavy
```

## Quick Start

### Requirements

- Go 1.25+
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

`MEMORY.md` and `USER.md` can be edited directly — SmartClaw will reload them on next use.

## Available Tools (73+)

### File Operations

| Tool | Description |
|------|-------------|
| `bash` | Execute shell commands with timeout and background support |
| `read_file` | Read file contents |
| `write_file` | Write files |
| `edit_file` | String replacement editing |
| `glob` | File pattern matching |
| `grep` | Content search with regex support |
| `powershell` | Execute PowerShell commands (Windows) |

### Code Analysis

| Tool | Description |
|------|-------------|
| `lsp` | LSP operations (goto_definition, find_references, rename, diagnostics) |
| `ast_grep` | AST pattern search and replace |
| `code_search` | Semantic code search |
| `index` | Code indexing for search |

### Web & Browser

| Tool | Description |
|------|-------------|
| `web_fetch` | Fetch and convert URLs to markdown |
| `web_search` | Web search |
| `browser_navigate` | Navigate to URL in headless browser |
| `browser_click` | Click element by CSS selector |
| `browser_type` | Type text into element |
| `browser_screenshot` | Capture page screenshot |
| `browser_extract` | Extract page content/text |
| `browser_wait` | Wait for element or condition |
| `browser_select` | Select option in dropdown |
| `browser_fill_form` | Fill multiple form fields |

### MCP Integration

| Tool | Description |
|------|-------------|
| `mcp` | Execute tools on connected MCP servers (SSE/stdio transport) |
| `list_mcp_resources` | List resources available on an MCP server |
| `read_mcp_resource` | Read a resource from a connected MCP server |
| `mcp_auth` | Authenticate with MCP servers via OAuth flow |

### Agent & Learning

| Tool | Description |
|------|-------------|
| `agent` | Spawn sub-agents for parallel tasks |
| `skill` | Load and manage skills |
| `session` | Session management |
| `todowrite` | Todo list management with verification nudge |
| `config` | Configuration management |
| `memory` | 4-layer memory query and management (recall, search, store, layers, stats) |

### Code Execution & Sandboxing

| Tool | Description |
|------|-------------|
| `execute_code` | Run Python code in RPC sandbox with tool access — collapses multi-turn into single turn |
| `docker_exec` | Execute commands in isolated Docker containers (one-shot or session-persistent) |
| `repl` | Evaluate expressions in JavaScript (Node.js) or Python with sandboxed timeout |

### Git Operations

| Tool | Description |
|------|-------------|
| `git_ai` | AI-powered commit messages, code review, and PR descriptions |
| `git_status` | Git status for the working directory |
| `git_diff` | Git diff (staged or unstaged) |
| `git_log` | Recent git commit log |

### Batch & Parallel

| Tool | Description |
|------|-------------|
| `batch` | Execute multiple tool calls in batch |
| `parallel` | Execute multiple tool calls in parallel |
| `pipeline` | Chain tool calls with output piping |

### Team Collaboration

| Tool | Description |
|------|-------------|
| `team_create` | Create a team workspace for memory sharing |
| `team_delete` | Delete a team workspace |
| `team_share_memory` | Share a memory item with team |
| `team_get_memories` | Retrieve shared team memories |
| `team_search_memories` | Search across team memories |
| `team_sync` | Sync team state across members |
| `team_share_session` | Share a session with team |

### Remote & Messaging

| Tool | Description |
|------|-------------|
| `remote_trigger` | Execute commands on remote hosts via SSH |
| `send_message` | Send messages to channels/users across platforms (telegram, web, terminal) |

### Workflow & Planning

| Tool | Description |
|------|-------------|
| `enter_worktree` | Create a git worktree for parallel development |
| `exit_worktree` | Remove a git worktree and clean up |
| `enter_plan_mode` | Enter structured planning mode |
| `exit_plan_mode` | Exit planning mode and resume execution |
| `schedule_cron` | Schedule, list, and delete cron jobs |

### Media & Documents

| Tool | Description |
|------|-------------|
| `image` | Analyze and process images |
| `pdf` | Extract text from PDF documents |
| `audio` | Process and transcribe audio files |

### Cognitive Tools

| Tool | Description |
|------|-------------|
| `think` | Structured thinking step before action |
| `deep_think` | Extended reasoning for complex problems |
| `brief` | Concise topic summarization |
| `observe` | Observation mode for passive analysis |
| `lazy` | Lazy-load deferred tools on demand |
| `fork` | Fork current session for parallel exploration |

### Utility

| Tool | Description |
|------|-------------|
| `tool_search` | Search for available tools by keyword |
| `cache` | Manage tool result cache |
| `attach` | Attach to running process |
| `debug` | Toggle debug mode |
| `env` | Show environment variables |
| `sleep` | Sleep for specified duration |

## Slash Commands (101)

### Core

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/status` | Session status |
| `/exit` | Exit REPL |
| `/clear` | Clear session |
| `/version` | Show version |

### Model & Config

| Command | Description |
|---------|-------------|
| `/model [name]` | Show or set model |
| `/model-list` | List available models |
| `/config` | Show configuration |
| `/config-show` | Show full configuration |
| `/config-set` | Set config value |
| `/config-get` | Get config value |
| `/config-reset` | Reset configuration |
| `/config-export` | Export configuration |
| `/config-import` | Import configuration |
| `/set-api-key <key>` | Set API key |
| `/env` | Show environment |

### Session

| Command | Description |
|---------|-------------|
| `/session` | List sessions |
| `/resume` | Resume a session |
| `/save` | Save current session |
| `/export` | Export session (markdown/json) |
| `/import` | Import session |
| `/rename` | Rename session |
| `/fork` | Fork session for parallel exploration |
| `/rewind` | Rewind session state |
| `/share` | Share session |
| `/summary` | Session summary |
| `/attach` | Attach to process |

### Compaction

| Command | Description |
|---------|-------------|
| `/compact` | Show context usage |
| `/compact now` | Manually compact conversation history |
| `/compact auto` | Toggle auto-compact on/off |
| `/compact status` | Show compact statistics |
| `/compact config` | Show compact configuration |

### Agent System

| Command | Description |
|---------|-------------|
| `/agent` | Manage AI agents |
| `/agent-list` | List available agents |
| `/agent-switch` | Switch to an agent |
| `/agent-create` | Create custom agent |
| `/agent-delete` | Delete custom agent |
| `/agent-info` | Show agent info |
| `/agent-export` | Export agent configuration |
| `/agent-import` | Import agent configuration |
| `/subagent` | Spawn subagent |
| `/agents` | List available agents |

### Template System

| Command | Description |
|---------|-------------|
| `/template` | Manage prompt templates |
| `/template-list` | List templates |
| `/template-use` | Use a template |
| `/template-create` | Create template |
| `/template-delete` | Delete template |
| `/template-info` | Show template info |
| `/template-export` | Export template |
| `/template-import` | Import template |

### Memory & Learning

| Command | Description |
|---------|-------------|
| `/memory` | Show memory context |
| `/skills` | List available skills |
| `/observe` | Observe mode |

### MCP

| Command | Description |
|---------|-------------|
| `/mcp` | Manage MCP servers |
| `/mcp-add` | Add MCP server |
| `/mcp-remove` | Remove MCP server |
| `/mcp-list` | List MCP servers |
| `/mcp-start` | Start MCP server |
| `/mcp-stop` | Stop MCP server |

### Git

| Command | Description |
|---------|-------------|
| `/git-status` (`/gs`) | Show git status |
| `/git-diff` (`/gd`) | Show git diff |
| `/git-commit` (`/gc`) | Commit changes |
| `/git-branch` (`/gb`) | List branches |
| `/git-log` (`/gl`) | Show git log |
| `/diff` | Show diff |
| `/commit` | Git commit shortcut |

### Tools & Development

| Command | Description |
|---------|-------------|
| `/tools` | List available tools |
| `/tasks` | List or manage tasks |
| `/lsp` | LSP operations |
| `/read` | Read file |
| `/write` | Write file |
| `/exec` | Execute command |
| `/browse` | Open browser |
| `/web` | Web operations |
| `/ide` | IDE integration |
| `/install` | Install package |

### Diagnostics

| Command | Description |
|---------|-------------|
| `/doctor` | Run diagnostics |
| `/cost` | Show token usage and cost |
| `/stats` | Show session statistics |
| `/usage` | Usage stats |
| `/debug` | Toggle debug mode |
| `/inspect` | Inspect internal state |
| `/cache` | Manage cache |
| `/heapdump` | Heap dump |
| `/reset-limits` | Reset rate limits |

### Planning & Thinking

| Command | Description |
|---------|-------------|
| `/plan` | Plan mode |
| `/think` | Think mode |
| `/deepthink` | Deep thinking |
| `/ultraplan` | Ultra planning |
| `/thinkback` | Think back |

### Collaboration & Communication

| Command | Description |
|---------|-------------|
| `/invite` | Invite collaboration |
| `/feedback` | Send feedback |
| `/issue` | Issue tracker |

### UI & Personalization

| Command | Description |
|---------|-------------|
| `/theme` | Manage themes |
| `/color` | Color theme |
| `/vim` | Vim mode |
| `/keybindings` | Manage keybindings |
| `/statusline` | Status line |
| `/stickers` | Stickers |

### Mode Switching

| Command | Description |
|---------|-------------|
| `/fast` | Fast mode (use lighter model) |
| `/lazy` | Lazy loading mode |
| `/desktop` | Desktop mode |
| `/mobile` | Mobile mode |
| `/chrome` | Chrome integration |
| `/voice` | Voice mode control |

### Auth & Updates

| Command | Description |
|---------|-------------|
| `/login` | Authenticate with service |
| `/logout` | Clear authentication |
| `/upgrade` | Upgrade CLI version |
| `/api` | API operations |

### Misc

| Command | Description |
|---------|-------------|
| `/init` | Initialize new project |
| `/context` | Manage context |
| `/permissions` | Manage permissions |
| `/hooks` | Manage hooks |
| `/plugin` | Manage plugins |
| `/passes` | LSP passes |
| `/preview` | Preview changes |
| `/effort` | Effort tracking |
| `/tag` | Tag management |
| `/copy` | Copy to clipboard |
| `/files` | List files |
| `/advisor` | AI advisor |
| `/btw` | By the way |
| `/bughunter` | Bug hunting mode |
| `/insights` | Code insights |
| `/onboarding` | Onboarding |
| `/teleport` | Teleport mode |
| `/summary` | Session summary |

## Project Structure

```
cmd/
└── smartclaw/              # Application entrypoint

internal/
├── acp/                    # Agent Communication Protocol (IDE integration via JSON-RPC)
├── analytics/              # Usage analytics and reporting
├── api/                    # API client with prompt caching + OpenAI support
├── assistant/              # Assistant personality and behavior
├── auth/                   # OAuth authentication
├── batch/                  # Batch runner for parallel prompt execution
├── bootstrap/              # Bootstrap and first-run
├── bridge/                 # Bridge adapters
├── buddy/                  # Buddy system for guided assistance
├── cache/                  # Caching system with dependency tracking
├── cli/                    # CLI commands (repl, tui, web, acp, batch, rl-eval, gateway)
├── commands/               # 101 Slash commands
├── compact/                # Compaction service (auto, micro, time-based)
├── components/             # Reusable TUI components
├── config/                 # Configuration management
├── constants/              # Shared constants
├── coordinator/            # Task coordination
├── costguard/              # Budget-aware spending guard with model downgrade
├── entrypoints/            # Application entrypoint variants
├── gateway/                # Unified gateway (router, delivery, cron)
│   └── platform/           # Platform adapters (terminal, web, telegram)
├── git/                    # Git context and operations
├── history/                # Command history
├── hooks/                  # Hook system
├── keybindings/            # Keybinding configuration
├── learning/               # Learning loop (evaluator, extractor, skill writer, nudge)
├── logger/                 # Structured logging
├── mcp/                    # MCP protocol (client, transport, auth, registry, enhanced)
├── memdir/                 # Memory directory management
├── memory/                 # Memory manager (4-layer coordination)
│   └── layers/             # L1 Prompt, L2 Session Search, L3 Skill, L4 User Model
├── migrations/             # Database migrations
├── models/                 # Data models
├── moreright/              # Extended permission capabilities
├── native/                 # Native platform bindings
├── native_ts/              # TypeScript native bindings
├── observability/          # Metrics, tracing, and telemetry
├── outputstyles/           # Output formatting styles
├── permissions/            # Permission engine (4 modes)
├── plugins/                # Plugin system
├── process/                # Process management
├── provider/               # Multi-provider API abstraction
├── query/                  # Query engine
├── remote/                 # Remote execution
├── rl/                     # Reinforcement learning evaluation environment
├── routing/                # Adaptive model routing + speculative execution
├── runtime/                # Query engine, compaction, session
├── sandbox/                # Sandboxed execution (Linux namespaces, RPC)
├── schemas/                # JSON schemas for tool inputs
├── screens/                # Screen layout management
├── server/                 # Direct connect server
├── services/               # Shared services (recorder, playback, sync, LSP, OAuth, voice, compact, analytics, rate limit)
├── session/                # Session management
├── skills/                 # Skills system (bundled + learned)
├── state/                  # Application state
├── store/                  # SQLite persistence (WAL, FTS5, JSONL backup)
├── template/               # Prompt template engine
├── tools/                  # Tool implementations (73+ tools)
├── transports/             # Transport layer abstractions
├── tui/                    # Terminal UI (Bubble Tea)
├── types/                  # Shared type definitions
├── upstreamproxy/          # Upstream API proxy
├── utils/                  # Utility functions
├── vim/                    # Vim mode support
├── voice/                  # Voice input/output
├── watcher/                # File system watcher
└── web/                    # Web UI + WebSocket server

pkg/
├── output/                 # Shared output formatting
└── progress/               # Progress bar utilities

extensions/
└── vscode/                 # VS Code extension (chat sidebar, code actions)
```

## VS Code Extension

SmartClaw ships with a VS Code extension that connects via ACP (Agent Communication Protocol):

### Commands

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

# Run with coverage
go test -cover ./...
```

## License

MIT License
