# Web Dashboard

## Overview

The EazyClaw web dashboard is enabled by default and provides a real-time interface for interacting with the agent, managing memory, monitoring status, and configuring channel settings.

Default access: `http://localhost:8080`

## Authentication

Set a password via environment variable:

```env
WEB_PASSWORD=your-secret
```

**`WEB_PASSWORD` is required.** The server will refuse to start without it.

## Tabs

### Chat

Real-time chat interface backed by WebSocket:

- Full session history preserved across page reloads
- Tool call rendering — shows when and what tools the agent invokes
- Markdown support for formatted responses (code blocks, lists, bold, etc.)
- Suitable for direct interaction without needing a third-party channel

### Memory

File tree explorer for the agent's persistent memory store:

- Browses `/data/eazyclaw/memory/` directory structure
- Click any file to view its contents inline
- Edit files in-place directly from the browser
- Useful for inspecting or correcting what the agent has remembered

### Skills

Grid view of all installed EazyClaw skills:

- Displays each skill's name and description
- Shows the tools each skill exposes to the agent
- Lists skill dependencies and requirements
- Use this to verify skills are loaded correctly after installation

### Status

Live operational status of all configured providers and channels:

- Connection indicators (connected / disconnected / error) per channel
- Provider health (LLM backend, memory, etc.)
- Updates in real time without requiring a page refresh
- First place to check when something isn't responding

### Settings

Configuration and access control management:

- View and update allowed user lists for Discord and Telegram
- Approve pending users when `dm_policy: pairing` is active
- Manage channel-specific allowlists
- Changes apply immediately without restarting the service

## Design

The dashboard uses a dark **"Neural Command Center"** aesthetic — high contrast, dark backgrounds, and status indicators styled for at-a-glance readability.
