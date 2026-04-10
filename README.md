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

### Development Tools

- **63 Built-in Tools**: File operations, code analysis, web tools, MCP integration, and more
- **Modern TUI**: Terminal User Interface built with Bubble Tea
- **Interactive REPL**: Full conversation history with streaming responses
- **MCP Integration**: Connect to MCP servers, discover tools, read resources, and authenticate via OAuth
- **Secure**: Permission system with 4 modes, sandboxed execution on Linux
- **Token Tracking**: Real-time cost estimation with auto-compact at threshold

### Gateway & Cross-Platform

- **Unified Gateway**: Message → Route → Memory → Execute → Learn → Deliver
- **Platform Adapters**: Terminal, Web UI, extensible to Telegram/Discord
- **Cron Tasks**: Scheduled tasks as first-class agent tasks with full memory access
- **Session Routing**: userID-based routing, not platform-based — switch devices without losing context
- **Session Recording**: Record and replay full sessions for audit and review

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

## Quick Start

### Requirements

- Go 1.25+
- Anthropic API key

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
./bin/smartclaw --model claude-sonnet-4-5 repl
```

### Configuration

Set your Anthropic API key:

```bash
export ANTHROPIC_API_KEY=your_key_here
```

Or create `~/.smartclaw/config.yaml`:

```yaml
api_key: your_api_key_here
model: claude-sonnet-4-5
max_tokens: 4096
permission: ask
log_level: info
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

`MEMORY.md` and `USER.md` can be edited directly — SmartClaw will reload them on next use.

## Available Tools

### File Operations

| Tool | Description |
|------|-------------|
| `bash` | Execute shell commands with timeout and background support |
| `read_file` | Read file contents |
| `write_file` | Write files |
| `edit_file` | String replacement editing |
| `glob` | File pattern matching |
| `grep` | Content search with regex support |

### Code Analysis

| Tool | Description |
|------|-------------|
| `lsp` | LSP operations (goto_definition, find_references, rename, diagnostics) |
| `ast_grep` | AST pattern search and replace |
| `code_search` | Semantic code search |

### Web Tools

| Tool | Description |
|------|-------------|
| `web_fetch` | Fetch and convert URLs to markdown |
| `web_search` | Web search |

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
| `todowrite` | Todo list management |
| `config` | Configuration management |

### Evaluation & Execution

| Tool | Description |
|------|-------------|
| `repl` | Evaluate expressions in JavaScript (Node.js) or Python with sandboxed timeout |
| `schedule_cron` | Schedule, list, and delete cron jobs |
| `enter_worktree` | Create a git worktree for parallel development |
| `exit_worktree` | Remove a git worktree and clean up |

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

## Slash Commands

### Core

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/status` | Session status |
| `/exit` | Exit REPL |
| `/clear` | Clear session |

### Model & Config

| Command | Description |
|---------|-------------|
| `/model [name]` | Show or set model |
| `/model-list` | List available models |
| `/config` | Show configuration |
| `/set-api-key <key>` | Set API key |

### Session

| Command | Description |
|---------|-------------|
| `/session new` | Create new session |
| `/session list` | List all sessions |
| `/session load <id>` | Load a saved session |
| `/session save` | Save current session |
| `/session delete <id>` | Delete a session |
| `/session export <id>` | Export session (markdown/json) |
| `/session record start` | Start recording session |
| `/session record stop` | Stop recording session |
| `/session record status` | Show recording status |
| `/session replay <file>` | Replay a session recording |

### Compaction

| Command | Description |
|---------|-------------|
| `/compact` | Show context usage |
| `/compact now` | Manually compact conversation history |
| `/compact auto` | Toggle auto-compact on/off |
| `/compact status` | Show compact statistics |
| `/compact config` | Show compact configuration |

### Diagnostics

| Command | Description |
|---------|-------------|
| `/doctor` | Run diagnostics |
| `/cost` | Show token usage |

## Project Structure

```
cmd/
└── smartclaw/          # Application entrypoint

internal/
├── api/                # API client with prompt caching
├── auth/               # OAuth authentication
├── bootstrap/          # Bootstrap and first-run
├── bridge/             # Bridge adapters
├── cache/              # Caching system
├── cli/                # CLI commands
├── commands/           # Slash commands
├── compact/            # Compaction service (auto, micro, time-based)
├── components/         # Reusable TUI components
├── config/             # Configuration management
├── gateway/            # Unified gateway (router, delivery, cron)
│   └── platform/       # Platform adapters (terminal, web)
├── git/                # Git context and operations
├── hooks/              # Hook system
├── learning/           # Learning loop (evaluator, extractor, skill writer, nudge)
├── mcp/                # MCP protocol (client, transport, auth, registry)
├── memory/             # Memory manager (4-layer coordination)
│   └── layers/         # L1 Prompt, L2 Session Search, L3 Skill, L4 User Model
├── models/             # Data models
├── permissions/        # Permission engine (4 modes)
├── runtime/            # Query engine, compaction, session
├── sandbox/            # Sandboxed execution (Linux namespaces)
├── services/           # Shared services (recorder, playback, sync)
├── skills/             # Skills system (bundled + learned)
├── store/              # SQLite persistence (WAL, FTS5, JSONL backup)
├── tools/              # Tool implementations (63 tools)
├── tui/                # Terminal UI (Bubble Tea)
├── voice/              # Voice input/output
└── web/                # Web UI + WebSocket server

pkg/
├── output/             # Shared output formatting
└── progress/           # Progress bar utilities
```

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

# Run with coverage
go test -cover ./...
```

## License

MIT License
