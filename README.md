# EazyClaw

Self-hosted AI agent gateway that connects LLM providers to messaging platforms (Telegram, Discord, WhatsApp) with a built-in web dashboard, tool execution, persistent memory, and skill system.

## Quick Start

```bash
# Clone and configure
git clone https://github.com/Naveenxyz/eazyclaw.git
cd eazyclaw
cp .env.example .env
# Edit .env — set at least one provider API key (e.g. KIMI_API_KEY)

# Build and run
docker compose up -d

# Open dashboard
open http://localhost:8080
```

## Deploy to Railway

Get a personal AI agent running for ~$1/month (Kimi Coding $1 + Railway free tier).

See the **[Getting Started Guide](docs/getting-started.md)** — deploy to Telegram + Railway in under 15 minutes.

## Channels

- **[Web Dashboard](docs/channels/web-dashboard.md)** — Browser-based chat, enabled by default on port 8080
- **[Telegram](docs/channels/telegram.md)** — Set `TELEGRAM_BOT_TOKEN` to enable
- **[Discord](docs/channels/discord.md)** — Set `DISCORD_BOT_TOKEN` to enable
- **[WhatsApp](docs/channels/whatsapp.md)** — Set `WHATSAPP_ENABLED=true`, scan QR once

## Providers

Six LLM providers supported. Set any combination of API keys:

| Provider | Env Var |
|---|---|
| **Kimi Coding** (recommended) | `KIMI_API_KEY` |
| Anthropic (Claude) | `ANTHROPIC_API_KEY` |
| OpenAI (GPT) | `OPENAI_API_KEY` |
| Google Gemini | `GEMINI_API_KEY` |
| Moonshot | `MOONSHOT_API_KEY` |
| Zhipu | `ZHIPU_API_KEY` |

See [Providers](docs/providers.md) for details and model configuration.

## Agent Tools (11 built-in)

`shell` · `read_file` · `write_file` · `edit_file` · `list_dir` · `web_fetch` · `web_search` · `memory_read` · `memory_write` · `memory_search` · `cron`

Runtime includes `git`, `gh`, `rg`, `fd`, `tree`, `wget`, `jq`, `tmux`, `node`, `npm`, `python3`, `uv`.

## Documentation

| Guide | Description |
|---|---|
| [Getting Started](docs/getting-started.md) | Onboarding: Kimi + Railway in 15 minutes |
| [Providers](docs/providers.md) | All 6 providers, model config, multi-provider routing |
| [Configuration](docs/configuration.md) | config.yaml reference + environment variables |
| [Memory System](docs/memory-system.md) | Persona, bootstrap files, compaction, day-wise memory |
| [Skills](docs/skills.md) | Using and creating skill packages |
| [Architecture](docs/architecture.md) | System diagram, project structure, message flow |
| [Development](docs/development.md) | Local build, tests, Docker |
| **Channel Guides** | |
| [Discord](docs/channels/discord.md) | Bot token, permissions, guild config |
| [Telegram](docs/channels/telegram.md) | BotFather setup, user approval |
| [WhatsApp](docs/channels/whatsapp.md) | Bridge setup, QR login, troubleshooting |
| [Web Dashboard](docs/channels/web-dashboard.md) | Password, tabs, features |

## License

MIT. See [LICENSE](LICENSE).
