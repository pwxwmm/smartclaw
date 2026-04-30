# SmartClaw Zed Extension

Connect [SmartClaw](https://github.com/instructkr/smartclaw) to [Zed](https://zed.dev) via ACP (Agent Communication Protocol).

## Prerequisites

- SmartClaw installed and available on `PATH`:
  ```bash
  go build -o /usr/local/bin/smartclaw ./cmd/smartclaw/
  ```
- `ANTHROPIC_API_KEY` environment variable set (or configured in `~/.smartclaw/config.yaml`)

## Installation

1. Clone or copy this directory
2. Build the extension WASM:
   ```bash
   cd extensions/zed
   cargo build --target wasm32-wasip1 --release
   ```
3. Install into Zed's extension directory, or use `zed --install-extension .`

## Usage

### Context Server (Primary)

The extension registers `smartclaw acp` as a Zed context server. Zed spawns the subprocess and communicates via MCP/ACP (stdio JSON-RPC with Content-Length framing). All SmartClaw tools become available in Zed's assistant panel.

In Zed's assistant panel, select "SmartClaw ACP" as the context server.

### Slash Command

Type `/smartclaw` in the assistant panel:

| Command | Description |
|---------|-------------|
| `/smartclaw ask <question>` | Ask SmartClaw a question |
| `/smartclaw explain` | Explain selected code |
| `/smartclaw fix` | Fix issues in selected code |
| `/smartclaw test` | Generate tests for selected code |
| `/smartclaw <text>` | Shorthand for "ask" |

## Architecture

```
Zed Assistant Panel
    │
    ├─ Context Server ──► smartclaw acp (subprocess, stdio JSON-RPC)
    │                      ├─ initialize
    │                      ├─ tools/list → all SmartClaw tools
    │                      └─ tools/call → execute tools
    │
    └─ /smartclaw slash command ──► formats prompt for assistant
```

Zed extensions run as WASM and cannot spawn subprocesses directly. The context server mechanism bridges this: Zed spawns `smartclaw acp` and handles the MCP/ACP stdio communication natively.

## Protocol

ACP uses JSON-RPC 2.0 with Content-Length framing (identical to MCP/LSP):

```
Content-Length: 87\r\n\r\n
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}
```

Supported methods: `initialize`, `initialized`, `tools/list`, `tools/call`, `shutdown`.

## Configuration

Set the SmartClaw binary path in `~/.smartclaw/config.yaml`:

```yaml
api_key: your_key_here
model: claude-opus-4-6
```

Or set environment variables in Zed's settings under `context_servers.smartclaw.env`.
