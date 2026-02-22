# EazyClaw

Self-hosted AI agent gateway that connects LLM providers to messaging platforms (Telegram, Discord, WhatsApp) with a built-in web dashboard, tool execution, persistent memory, and skill system.

## Why I Am Building EazyClaw

I started EazyClaw around one core belief: a personal AI assistant should be easy to own and run.

The goal is simple:
- clone this repo
- set a small set of env vars (for example just a `KIMI_API_KEY`)
- deploy to Railway (or any container host)
- connect a real chat channel (like WhatsApp)
- start chatting in minutes

I care less about giant feature surfaces and more about first-run success, fast iteration, and keeping the stack understandable.

> [!NOTE]
> EazyClaw is hugely inspired by projects like OpenClaw, nanobot, and other agent repos in this workspace.
> This project is my product-first take: fundamentally, I am building the assistant I personally want to use every day.

## Fundamental Architecture

EazyClaw is built as a small, pragmatic gateway with clean boundaries:
- Channels (`telegram`, `discord`, `whatsapp`, `web`) convert platform events into a unified inbound message shape.
- A typed message bus decouples channel adapters from core agent processing.
- The agent loop handles context build, provider call, tool execution, and response routing.
- Providers are pluggable (Anthropic/OpenAI/Gemini/Kimi/Moonshot/Zhipu).
- Tools are registry-based (`shell`, filesystem, web, memory, cron).
- Memory is file-native (`AGENTS.md`, `SOUL.md`, `IDENTITY.md`, `USER.md`, `MEMORY.md`, day files).
- UI is a built-in web dashboard for chat, memory browsing, status, and settings.

## What Is Built And What I’m Working Toward

Built now:
- Multi-provider model routing
- Telegram/Discord/Web channels
- WhatsApp bridge support (QR-based onboarding)
- Persistent sessions + memory model
- Cron and heartbeat runners
- Docker-first deployment path

Working toward:
- rock-solid one-command deploy templates (especially Railway)
- even smoother WhatsApp onboarding and health visibility
- stronger safety defaults and test coverage
- minimal-ops deployment with clear guardrails
- a great personal-agent experience without platform lock-in

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
- **WhatsApp** — Set `WHATSAPP_ENABLED=true`, start the bridge, and scan QR once

### Agent Tools (11 built-in)
| Tool | Description |
|---|---|
| `shell` | Execute bash commands (with deny patterns and timeout). Runtime includes `git`, `gh`, `rg`, `fd`, `tree`, `wget`, `zip`, `unzip`, `tmux`, `jq`, `node`, `npm`, `python3`, `uv` |
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
               Telegram / Discord / WhatsApp / Web Browser
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
WHATSAPP_ENABLED=true
WHATSAPP_BRIDGE_URL=ws://whatsapp-bridge:3001
WHATSAPP_BRIDGE_TOKEN=optional-shared-secret

# Web Dashboard
WEB_PASSWORD=your-secret    # Optional, no auth if unset
WEB_PORT=8080               # Default: 8080

# GitHub CLI auth (optional)
GH_TOKEN=ghp_xxx            # Used by `gh auth login --with-token`
```

### Getting a Discord Bot Token

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Click **New Application**, give it a name (e.g. "EazyClaw"), and click **Create**
3. Go to the **Bot** tab in the left sidebar
4. Click **Reset Token** (or **Add Bot** if this is a fresh app) and copy the token — this is your `DISCORD_BOT_TOKEN`
5. Under **Privileged Gateway Intents**, enable **Message Content Intent** (required for reading messages)
6. Go to the **OAuth2** tab → **URL Generator**
7. Select scopes: `bot`
8. Select bot permissions: `Send Messages`, `Read Message History`, `View Channels`
9. Copy the generated URL and open it in your browser to invite the bot to your server

```bash
# Add to your .env
DISCORD_BOT_TOKEN=your-token-here
```

### Getting a Telegram Bot Token

1. Open Telegram and search for [@BotFather](https://t.me/BotFather)
2. Send `/newbot`
3. Choose a display name (e.g. "EazyClaw")
4. Choose a username — must end in `bot` (e.g. `eazyclaw_bot`)
5. BotFather replies with your token — this is your `TELEGRAM_BOT_TOKEN`
6. Optionally send `/setdescription` to set a bio, and `/setuserpic` to set an avatar

```bash
# Add to your .env
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIjKlMnOpQrStUvWxYz
```

### GitHub CLI Auth

GitHub CLI (`gh`) is preinstalled in the runtime image, and its auth state is persisted in `/data/eazyclaw/auth/gh`.

```bash
# one-time auth inside the running container
docker compose exec eazyclaw sh -lc 'echo "$GH_TOKEN" | gh auth login --with-token'

# verify auth
docker compose exec eazyclaw gh auth status
```

Notes:
- Set `GH_TOKEN` in your `.env` before running the login command.
- `GH_CONFIG_DIR` is set to `/data/eazyclaw/auth/gh` so auth survives container restarts.

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
  whatsapp:
    enabled: false
    bridge_url: "ws://whatsapp-bridge:3001"
    bridge_token: ""
    allowed_users: []
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
| **Settings** | Configure Discord/Telegram settings and allowlists |

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
    ├── uv/                  # uv cache/tools/python installs
    ├── npm/                 # npm cache/global installs
    ├── skills/              # Skill packages
    └── auth/
        └── gh/              # GitHub CLI auth config
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
npm install
npm run build

# Docker
docker compose build
docker compose up -d
```

### WhatsApp Quick Start

```bash
# 1) Enable WhatsApp in env
export WHATSAPP_ENABLED=true
export WHATSAPP_BRIDGE_TOKEN=your-shared-token  # optional, recommended

# 2) Build + start app and bridge
docker compose up -d --build

# 3) Scan QR shown by bridge logs (first login only)
docker compose logs -f whatsapp-bridge
```

### Project Structure

```
cmd/eazyclaw/          # Main entry point, CLI commands
internal/
├── agent/             # Agent loop, context builder, session store, heartbeat, cron runner
├── bus/               # Message bus (inbound/outbound channels)
├── channel/           # Telegram, Discord, WhatsApp, Web (HTTP + WebSocket)
├── config/            # YAML + env config loader
├── provider/          # LLM provider implementations
├── router/            # Access control and session routing
├── skill/             # Skill loader and parser
└── tool/              # Tool registry + implementations
bridge/whatsapp/       # Bundled Node.js WhatsApp bridge (Baileys + WebSocket)
ui/                    # React + TypeScript + Tailwind frontend
```

## License

MIT. See [LICENSE](LICENSE).
