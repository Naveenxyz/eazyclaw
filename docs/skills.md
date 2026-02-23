# Skills System

## Overview

Skills extend the agent's capabilities by dropping skill packages into `/data/eazyclaw/skills/`. All skills in that directory are auto-loaded on startup — no configuration required.

## Built-in Skills

EazyClaw ships with four default skills:

| Skill | Description |
|---|---|
| `weather` | Weather information lookups |
| `github` | GitHub operations via the `gh` CLI |
| `web-researcher` | Web search and content fetching |
| `skill-creator` | Meta-skill for creating new skills |

## Skill Structure

Each skill is a directory containing a `skill.md` file. The `skill.md` file describes the skill's tools and instructions that are injected into the agent's context.

```
/data/eazyclaw/skills/
└── your-skill-name/
    └── skill.md
```

## Creating Custom Skills

1. Create a directory under `/data/eazyclaw/skills/`:

   ```
   mkdir -p /data/eazyclaw/skills/my-skill
   ```

2. Add a `skill.md` file with the following sections:
   - **Skill name** — identifier for the skill
   - **Description** — what the skill does
   - **Tool definitions** — tools the agent can invoke
   - **Usage instructions** — how and when to use the skill

3. Restart EazyClaw (or wait for the next startup). Skills are loaded automatically.

### Example Directory Structure

```
/data/eazyclaw/skills/
├── weather/
│   └── skill.md
├── github/
│   └── skill.md
├── web-researcher/
│   └── skill.md
├── skill-creator/
│   └── skill.md
└── my-custom-skill/
    └── skill.md
```

## Dashboard

All loaded skills are visible in the web dashboard under the **Skills** tab, where you can inspect their definitions and current status.
