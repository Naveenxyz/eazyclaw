# Memory System

EazyClaw uses a file-native persistent memory model. Memory files are loaded from disk and injected into the agent's context at the start of each session. There is no vector database or embedding layer — everything is plain Markdown.

## Bootstrap Files

These files live in `/data/eazyclaw/memory/` and are loaded on every session startup.

### AGENTS.md
Workspace rules for the agent. Defines the order in which memory files are read, memory hygiene rules (what to write, what to skip), and safety guidelines. This file governs how the agent manages its own memory over time.

### SOUL.md
The agent's persona definition. Contains core truths about the agent's character, hard boundaries it will not cross, and channel-specific behavior rules. Examples:

- **WhatsApp**: short responses, plain text only, no Markdown formatting
- **Discord**: Markdown is supported, richer formatting allowed
- **Telegram**: moderate length, standard formatting

Also captures the general vibe and communication style the agent should maintain across interactions.

### BOOTSTRAP.md
A first-run onboarding script. Present only before onboarding is complete — it guides the agent to learn about its own identity and the user before engaging in regular tasks. Once onboarding is finished, this file is deleted from the memory directory so it is not loaded in subsequent sessions.

### IDENTITY.md
The agent's identity profile. Contains the agent's name, role description, vibe summary, preferred emoji, and avatar configuration. This is the agent's self-concept, distinct from SOUL.md which covers behavioral rules.

### USER.md
The user's profile. Contains name, pronouns, timezone, communication preferences, and current project context. The agent updates this file as it learns more about the user over time.

### MEMORY.md
Long-term curated memory. This is the primary persistent store for facts that matter across sessions:

- User preferences and habits
- Ongoing projects and their current status
- Stable technical facts (stack choices, conventions, credentials format)
- Open threads — tasks or questions that have not been resolved

The agent appends to and edits this file during sessions as new stable facts emerge.

### HEARTBEAT.md
A periodic task checklist executed on the heartbeat interval. Contains triggers for:

- Memory review and compaction checks
- System health monitoring
- Background task status monitoring
- Proactive nudges (e.g., reminders, scheduled summaries)

## Day-wise Memory Files

Each day's conversation context is written to a dated file:

```
/data/eazyclaw/memory/YYYY-MM-DD.md
```

These files are:
- Auto-created by the container entrypoint at startup if they do not exist
- Append-only during a session — new context is written at the end
- Included in session context based on `memory.daily_files_in_context` (default: 3 most recent days)

## Memory Compaction

When accumulated messages in a session exceed the `compaction.trigger_messages` threshold (default: 40), a compaction flush is triggered:

1. The agent writes a summary of the current session context to the day's memory file
2. The oldest messages are dropped, keeping only the most recent `keep_recent` messages (default: 10)
3. The compacted summary is prepended to the retained context so continuity is preserved

This keeps context window usage bounded during long sessions.

## Background Digest

A background goroutine runs on a configurable interval (default: every 10 minutes) to process accumulated context and update memory files without interrupting the active conversation. The digest:

- Reads the current session's accumulated messages
- Extracts stable facts and appends them to MEMORY.md or the daily file
- Runs a maximum of `max_runs` times per session (default: 20) to prevent runaway processing

## Customizing the Agent Persona

Edit `SOUL.md` directly via the **Memory** tab in the web dashboard. Changes take effect at the start of the next session — the current session's persona is not hot-reloaded.

## Data Volume Layout

```
/data/eazyclaw/
  memory/
    AGENTS.md
    SOUL.md
    IDENTITY.md
    USER.md
    MEMORY.md
    HEARTBEAT.md
    2025-01-15.md
    2025-01-16.md
    ...
  auth/
    gh/              # GitHub CLI config (GH_CONFIG_DIR)
  skills/            # Custom skill YAML definitions
```
