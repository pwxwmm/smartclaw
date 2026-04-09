# SmartClaw

A self-improving AI agent that learns your workflow over time.

SmartClaw is not just another AI coding tool — it's an agent that gets smarter with every task. Inspired by the Hermes architecture, it features a learning loop that evaluates completed tasks, extracts reusable methods, and automatically creates skills. The more you use it, the better it understands how you work.

## Core Philosophy

**"越来越懂你的工作方式"** — Not "better at everything", but "better at *your* everything".

Unlike generic AI assistants that forget everything between sessions, SmartClaw:
- **Learns from completed tasks** — evaluates whether an approach is worth reusing, and if so, extracts it as a skill
- **Remembers across sessions** — 4-layer memory system with SQLite + FTS5 full-text search
- **Self-improves over time** — periodic nudges trigger the agent to consolidate memory and refine skills
- **Understands your preferences** — passively tracks communication style, knowledge background, and common workflows

## Features

### Agent Capabilities
- **🧠 Learning Loop**: Post-task evaluation → method extraction → skill creation → MEMORY.md auto-update
- **💾 4-Layer Memory**: Prompt Memory (MEMORY.md/USER.md), Session Search (FTS5), Skill Procedural (lazy load), User Modeling
- **🔔 Periodic Nudge**: System-triggered self-review every 10 turns (configurable)
- **🔄 Smart Compaction**: Head protection + tool result pruning + source-traceable summaries

### Development Tools
- **🛠️ 57+ Built-in Tools**: File operations, code analysis, web tools, and more
- **🖥️ Modern TUI**: Terminal User Interface with bubbletea
- **💬 Interactive REPL**: Full conversation history with streaming responses
- **🔌 Extensible**: MCP protocol, plugins, hooks, and skills support
- **🔐 Secure**: Permission system with 4 modes
- **📊 Token Tracking**: Real-time cost estimation

### Gateway & Cross-Platform
- **🌐 Unified Gateway**: Message → Route → Memory → Execute → Learn → Deliver
- **📱 Platform Adapters**: Terminal, Web UI, extensible to Telegram/Discord
- **⏰ Cron Tasks**: Scheduled tasks as first-class agent tasks with full memory access
- **🔗 Session Routing**: userID-based routing, not platform-based — switch devices without losing context

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

### Installation

```bash
go build -o bin/smartclaw ./cmd/claw/
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
api_key: your_key_here
model: claude-sonnet-4-5
max_tokens: 4096
permission: ask
log_level: info
```

### Memory Configuration

SmartClaw automatically creates and manages these files:

- `~/.smartclaw/MEMORY.md` — System memory (auto-updated by learning loop)
- `~/.smartclaw/USER.md` — User profile (auto-evolved from observations)
- `~/.smartclaw/state.db` — SQLite database with FTS5 index
- `~/.smartclaw/skills/` — Learned and bundled skills
- `~/.smartclaw/cron/` — Scheduled task definitions

You can edit `MEMORY.md` and `USER.md` directly — SmartClaw will reload them.

## Available Tools

### File Operations
- `bash` - Execute shell commands
- `read_file` - Read file contents
- `write_file` - Write files
- `edit_file` - String replacement editing
- `glob` - File pattern matching
- `grep` - Content search

### Code Analysis
- `lsp` - LSP operations (goto_definition, find_references, etc.)
- `ast_grep` - AST pattern search
- `code_search` - Semantic code search

### Web Tools
- `web_fetch` - Fetch URLs
- `web_search` - Web search

### Agent & Learning
- `agent` - Spawn sub-agents for parallel tasks
- `skill` - Load and manage skills
- `session` - Session management
- `todowrite` - Todo list management
- `config` - Configuration management

## Slash Commands

### Core
- `/help` - Show available commands
- `/status` - Session status
- `/exit` - Exit REPL
- `/clear` - Clear session

### Model & Config
- `/model [name]` - Show or set model
- `/model-list` - List available models
- `/config` - Show configuration
- `/set-api-key <key>` - Set API key

### Session
- `/session` - List sessions
- `/resume <id>` - Resume session
- `/save` - Save current session
- `/export` - Export session

### Diagnostics
- `/doctor` - Run diagnostics
- `/cost` - Show token usage
- `/compact` - Compact session history

## Project Structure

```
internal/
├── api/            # API client with prompt caching
├── auth/           # OAuth authentication
├── cache/          # Caching system
├── cli/            # CLI commands
├── commands/       # Slash commands
├── config/         # Configuration management
├── gateway/        # Unified gateway (router, delivery, cron)
│   └── platform/   # Platform adapters (terminal, web)
├── learning/       # Learning loop (evaluator, extractor, skill writer, nudge)
├── memory/         # Memory manager (4-layer coordination)
│   └── layers/     # L1 Prompt, L2 Session Search, L3 Skill, L4 User Model
├── runtime/        # Query engine, compaction, session
├── skills/         # Skills system (bundled + learned)
├── store/          # SQLite persistence (WAL, FTS5, JSONL backup)
├── tools/          # Tool implementations (57+)
├── mcp/            # MCP protocol
├── tui/            # Terminal UI
└── web/            # Web UI + WebSocket server
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

- `ANTHROPIC_API_KEY` - API key for Anthropic
- `SMARTCLAW_MODEL` - Default model to use
- `SMARTCLAW_CONFIG` - Path to config file
- `SMARTCLAW_SESSION_DIR` - Directory for session storage
- `SMARTCLAW_LOG_LEVEL` - Log level (debug, info, warn, error)

## Testing

```bash
# Run all tests
go test ./...

# Run specific package
go test ./internal/learning/...
go test ./internal/store/...
go test ./internal/memory/layers/...

# Run with coverage
go test -cover ./...
```

## Roadmap

- [x] Learning loop (evaluate → extract → write skill)
- [x] 4-layer memory system
- [x] SQLite + FTS5 persistent storage
- [x] Smart compaction with source tracing
- [x] Periodic nudge engine
- [x] Unified gateway with session routing
- [x] Cron tasks as first-class agent tasks
- [x] Prompt caching optimization
- [ ] LLM-based memory compression (EnforceLimit)
- [ ] Cross-platform continuity (Telegram, Discord)
- [ ] Long-term planning capabilities
- [ ] Skill Hub (community skill sharing)

## License

MIT License
