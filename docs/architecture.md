# Architecture

## System Diagram

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

## Message Flow

1. A channel receives a platform event
2. The event is converted into a unified inbound message shape
3. The message is published onto the message bus
4. The agent loop picks it up, builds context (memory + session), and calls the configured provider
5. If the provider requests tool use, tools are executed and results fed back
6. The response is routed back through the bus
7. The originating channel delivers the reply to the user

## Key Components

**Channels** — Convert platform-specific events (Telegram updates, Discord gateway events, WhatsApp messages, HTTP requests) into a unified message shape consumed by the rest of the system.

**Message Bus** — A typed, in-process bus that decouples channels from core processing. Inbound and outbound channels are separate, keeping components independent.

**Agent Loop** — Orchestrates the full request lifecycle: context build, provider call, tool execution loop, and response routing.

**Session Store** — SQLite-backed session persistence (`/data/eazyclaw/sessions/sessions.db`) with:
- message history storage
- compaction metadata
- token usage counters
- paginated reads for dashboard/API

**Providers** — Pluggable LLM backends. Supported: Anthropic, OpenAI, Gemini, Kimi, Moonshot, Zhipu.

**Tools** — Registry-based tool system. Built-in tools cover shell execution, filesystem access, web fetching, memory operations, and cron scheduling.

**Memory** — File-native persistent memory. Memory files live under `/data/eazyclaw/memory/` and are loaded into context on each request.

**UI** — Built-in web dashboard (React + TypeScript + Tailwind) for managing sessions, skills, memory, and configuration.

## Project Structure

```
cmd/eazyclaw/          # Main entry point, CLI commands
internal/
├── agent/             # Agent loop, context builder, session store, heartbeat, cron
├── bus/               # Message bus (inbound/outbound channels)
├── channel/           # Telegram, Discord, WhatsApp, Web (HTTP + WebSocket)
├── config/            # YAML + env config loader
├── provider/          # LLM provider implementations
├── router/            # Access control and session routing
├── skill/             # Skill loader and parser
└── tool/              # Tool registry + implementations
bridge/whatsapp/       # Bundled Node.js WhatsApp bridge (Baileys + WebSocket)
ui/                    # React + TypeScript + Tailwind frontend
defaults/              # Template files copied into Docker image
├── memory/            # Bootstrap memory files (SOUL.md, AGENTS.md, etc.)
└── skills/            # Default skill packages
docs/                  # Documentation
```
