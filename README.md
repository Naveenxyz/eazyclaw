# EazyClaw

Self-hosted AI agent gateway that connects LLM providers to messaging platforms (Telegram, Discord) with a built-in web dashboard, tool execution, persistent memory, and skill system.

## Quick Start

```bash
# 1. Configure environment
cp .env.example .env
# Edit .env with your API keys

# 2. Build and run
docker compose up -d

# 3. Open dashboard
open http://localhost:8080
```

## Features

### LLM Providers
- **Anthropic** (Claude) — `ANTHROPIC_API_KEY`
- **OpenAI** (GPT) — `OPENAI_API_KEY`
- **Google Gemini** — `GEMINI_API_KEY`
- **Kimi Coding** — `KIMI_API_KEY`
- **Moonshot** — `MOONSHOT_API_KEY`
- **Zhipu** — `ZHIPU_API_KEY`

Set any combination of API keys. The first available provider is used by default, or configure `default_model` in `config.yaml`.

### Messaging Channels
- **Web Dashboard** — Browser-based chat with WebSocket, enabled by default on port 8080
- **Telegram** — Set `TELEGRAM_BOT_TOKEN` to enable
- **Discord** — Set `DISCORD_BOT_TOKEN` to enable

### Agent Tools (10 built-in)
| Tool | Description |
|---|---|
| `shell` | Execute bash commands (with deny patterns and timeout) |
| `read_file` | Read files from workspace |
| `write_file` | Create or overwrite files |
| `edit_file` | String replacement edits |
| `list_dir` | List directory contents |
| `web_fetch` | Fetch URLs, convert HTML to Markdown |
| `web_search` | DuckDuckGo search |
| `memory_read` | Read from persistent memory |
| `memory_write` | Write/append to memory files |
| `memory_search` | Full-text search across memory |
| `cron` | Manage scheduled cron jobs |

### SOUL.md Persona System
Define agent personality, communication style, and values in `/data/eazyclaw/memory/SOUL.md`. The system prompt reads this file to shape the agent's identity. A default template is created on first run.

### Bootstrap Identity Files
OpenClaw-style bootstrap files are loaded from `/data/eazyclaw/memory/`:
- `AGENTS.md`
- `SOUL.md`
- `BOOTSTRAP.md` (first-run only; delete after onboarding)
- `IDENTITY.md`
- `USER.md`

### USER.md + Day-Wise Memory
- `/data/eazyclaw/memory/USER.md` stores user preferences and profile notes.
- `/data/eazyclaw/memory/MEMORY.md` stores long-term durable memory.
- `/data/eazyclaw/memory/YYYY-MM-DD.md` stores day-wise memory updates.
- Name/preferred-name/timezone/pronouns facts are routed to `USER.md`.
- Memory is updated during pre-compaction flushes, compaction events, and background digest passes (not full turn-by-turn transcript dumps).

### Heartbeat Runner
Periodic synthetic messages that prompt the agent to review `/data/eazyclaw/memory/HEARTBEAT.md` for active tasks. Configurable interval, disabled by default.

```yaml
heartbeat:
  enabled: true
  interval: 30m
```

### Cron Scheduler
The agent can create and manage its own cron jobs via the `cron` tool. Jobs are persisted to `/data/eazyclaw/cron/jobs.json` and checked every minute.

```
# Agent can run:
cron add --schedule "0 9 * * *" --task "Review memory and summarize activity"
cron list
cron remove --id <job-id>
```

### Memory Explorer
Browse, view, and edit agent memory files through the web dashboard's Memory tab. Supports Markdown rendering and in-place editing.

### Skills System
Drop skill packages into `/data/eazyclaw/skills/` to extend the agent with custom tool definitions and instructions. Skills are automatically loaded on startup.

## Architecture

```
                    Telegram / Discord / Web Browser
                              |
                    ┌─────────┴─────────┐
                    │     CHANNELS       │
                    └─────────┬─────────┘
                              │
                         MESSAGE BUS
                              │
                    ┌─────────┴─────────┐
                    │    AGENT LOOP      │
                    │  ┌─────────────┐   │
                    │  │  Provider   │   │
                    │  │  Tool Exec  │   │
                    │  │  Session    │   │
                    │  └─────────────┘   │
                    └───────────────────┘
                              │
          ┌───────┬───────┬───┴───┬────────┐
       Anthropic OpenAI Gemini  Kimi   Moonshot
```

## Configuration

Configuration is loaded from `/data/eazyclaw/config.yaml` with environment variable overrides.

### Environment Variables

```bash
# LLM Providers (set at least one)
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
GEMINI_API_KEY=...
KIMI_API_KEY=...
MOONSHOT_API_KEY=...
ZHIPU_API_KEY=...

# Channels
TELEGRAM_BOT_TOKEN=...
DISCORD_BOT_TOKEN=...

# Web Dashboard
WEB_PASSWORD=your-secret    # Optional, no auth if unset
WEB_PORT=8080               # Default: 8080
```

### config.yaml

See [`config.example.yaml`](config.example.yaml) for the full reference. Key sections:

```yaml
data_dir: /data/eazyclaw
workspace_dir: /data/eazyclaw/workspace

providers:
  default_model: "claude-sonnet-4-6"

channels:
  discord:
    allowed_users: []           # empty = allow all
    group_policy: "allowlist"   # "allowlist" or "open"
  telegram:
    allowed_users: []
    group_policy: "allowlist"
  web:
    enabled: true
    port: 8080

tools:
  shell:
    deny_patterns: ["rm -rf /", "sudo"]
    timeout: 60s
    workspace_only: true
  browser:
    headless: true

heartbeat:
  enabled: false
  interval: 30m

skills_dir: /data/eazyclaw/skills
```

## Web Dashboard

The dashboard provides 5 tabs:

| Tab | Description |
|---|---|
| **Chat** | Real-time WebSocket chat with session history, tool call rendering, and Markdown support |
| **Memory** | File tree explorer for `/data/eazyclaw/memory/` — browse, view Markdown, and edit files in-place |
| **Skills** | Grid view of installed skills with tool and dependency details |
| **Status** | Live provider and channel status with connection indicators |
| **Settings** | Configure Discord/Telegram allowlists, guild settings, and DM policies |

### Design
Dark "Neural Command Center" aesthetic with deep space backgrounds, frosted glass cards, violet/cyan accents, and JetBrains Mono typography.

## Data Volume

All persistent data lives in the `/data/eazyclaw` root within the `/data` Docker volume:

```
/data/
└── eazyclaw/
    ├── config.yaml          # Runtime configuration
    ├── workspace/           # Shell/file tool sandbox
    ├── sessions/            # Chat session history (JSON)
    ├── memory/              # Persistent agent memory + identity/bootstrap files
    │   ├── AGENTS.md        # Agent operating instructions
    │   ├── SOUL.md          # Agent persona
    │   ├── IDENTITY.md      # Agent identity profile
    │   ├── USER.md          # User profile + preferences
    │   ├── HEARTBEAT.md     # Periodic task checklist
    │   ├── MEMORY.md        # Long-term memory
    │   └── YYYY-MM-DD.md    # Day-wise memory log
    ├── cron/
    │   └── jobs.json        # Scheduled cron jobs
    ├── skills/              # Skill packages
    └── auth/                # OAuth tokens
```

## Development

### Prerequisites
- Go 1.25+
- Node.js 22+
- Docker (for container builds)

### Local Build

```bash
# Backend
go vet ./...
go build -o eazyclaw ./cmd/eazyclaw/

# Frontend
cd ui
yarn install
yarn build

# Docker
docker compose build
docker compose up -d
```

### Project Structure

```
cmd/eazyclaw/          # Main entry point, CLI commands
internal/
├── agent/             # Agent loop, context builder, session store, heartbeat, cron runner
├── bus/               # Message bus (inbound/outbound channels)
├── channel/           # Telegram, Discord, Web (HTTP + WebSocket)
├── config/            # YAML + env config loader
├── provider/          # LLM provider implementations
├── router/            # Access control and session routing
├── skill/             # Skill loader and parser
└── tool/              # Tool registry + implementations
ui/                    # React + TypeScript + Tailwind frontend
```

## License

Private.
