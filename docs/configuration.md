# Configuration Reference

EazyClaw loads its configuration from `/data/eazyclaw/config.yaml`. Any value in the config file can be overridden using environment variables.

## Environment Variables

| Variable | Description |
|---|---|
| `ANTHROPIC_API_KEY` | Anthropic (Claude) API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `KIMI_API_KEY` | Kimi Coding API key |
| `MOONSHOT_API_KEY` | Moonshot API key |
| `ZHIPU_API_KEY` | Zhipu API key |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token |
| `DISCORD_BOT_TOKEN` | Discord bot token |
| `WHATSAPP_ENABLED` | Enable WhatsApp bridge (`true`/`false`) |
| `WHATSAPP_BRIDGE_URL` | Bridge WebSocket URL (default: `ws://whatsapp-bridge:3001`) |
| `WHATSAPP_BRIDGE_TOKEN` | Shared secret for bridge authentication |
| `WEB_PASSWORD` | Web dashboard password (**required**) |
| `WEB_PORT` | Web dashboard port (default: `8080`) |
| `GH_TOKEN` | GitHub CLI auth token |

## config.yaml Reference

```yaml
# Directory where persistent data (memory, auth, workspace) is stored
data_dir: /data/eazyclaw

# Directory used as the agent's working directory for shell tools
workspace_dir: /data/eazyclaw/workspace

providers:
  # Default model used when no channel-specific model is set
  default_model: k2p5

  kimi_coding:
    model: k2p5
    base_url: "https://api.kimi.com/coding/"

  # anthropic:
  #   model: claude-sonnet-4-6

  # openai:
  #   model: gpt-4.1

  # gemini:
  #   model: gemini-2.5-flash

  # moonshot:
  #   model: kimi-k2.5
  #   base_url: "https://api.moonshot.ai/v1"

  # zhipu:
  #   model: glm-4-plus
  #   base_url: "https://open.bigmodel.cn/api/paas/v4"

channels:
  discord:
    enabled: true
    token: ""            # Overridden by DISCORD_BOT_TOKEN
    allowed_users: []    # List of Discord user IDs allowed to interact
    group_policy: allow  # Policy for server/guild messages: allow | deny
    dm_policy: allow     # Policy for direct messages: allow | deny

  telegram:
    enabled: true
    token: ""            # Overridden by TELEGRAM_BOT_TOKEN
    allowed_users: []    # List of Telegram user IDs or usernames
    group_policy: deny   # Policy for group chat messages: allow | deny
    dm_policy: allow     # Policy for direct messages: allow | deny

  whatsapp:
    enabled: false       # Overridden by WHATSAPP_ENABLED
    bridge_url: ws://whatsapp-bridge:3001   # Overridden by WHATSAPP_BRIDGE_URL
    bridge_token: ""     # Overridden by WHATSAPP_BRIDGE_TOKEN
    allowed_users: []    # List of phone numbers in E.164 format
    group_policy: deny   # Policy for group messages: allow | deny
    dm_policy: allow     # Policy for direct messages: allow | deny

  web:
    enabled: true
    port: 8080           # Overridden by WEB_PORT
    password: ""         # Overridden by WEB_PASSWORD (required)
    allowed_users: []    # Reserved for future per-user web auth

tools:
  shell:
    # Glob/regex patterns for commands that are always blocked
    deny_patterns:
      - "rm -rf /"
      - ":(){ :|:& };:"
    # Shell command execution timeout in seconds
    timeout: 120
    # Restrict shell to workspace_dir only (blocks absolute paths outside it)
    workspace_only: false

  browser:
    # Run browser in headless mode (no visible UI)
    headless: true

# Directory where custom skill definitions (.yaml) are stored
skills_dir: /data/eazyclaw/skills

heartbeat:
  # Enable periodic heartbeat task execution
  enabled: true
  # Interval between heartbeat runs (Go duration string)
  interval: 10m

memory:
  enabled: true
  # Maximum characters of memory context injected into each session
  context_max_chars: 8000
  # Number of recent daily memory files included in context
  daily_files_in_context: 3

  compaction:
    # Number of accumulated messages that triggers a compaction flush
    trigger_messages: 40
    # Number of recent messages to retain after compaction
    keep_recent: 10

  background:
    # Enable background digest processing
    enabled: true
    # Interval between background digest runs
    interval: 10m
    # Maximum number of background digest runs per session
    max_runs: 20
```

## GitHub CLI Authentication

The `GH_TOKEN` environment variable is used to authenticate the GitHub CLI inside the container. To persist auth across restarts:

```sh
docker compose exec eazyclaw sh -lc 'echo "$GH_TOKEN" | gh auth login --with-token'
```

The GitHub CLI config directory is set to `GH_CONFIG_DIR=/data/eazyclaw/auth/gh`, which lives on the persistent data volume. Authentication survives container rebuilds as long as the volume is retained.
