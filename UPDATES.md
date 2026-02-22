# EazyClaw Updates Plan

## Issues Found

### 1. Memory Explorer shows "No files"
- **Not a bug** — the API works correctly (`GET /api/memory` returns valid response)
- **Cause**: `/data/memory/` directory in container is empty — no default files are seeded
- **Fix**: Update `entrypoint.sh` to create a default `MEMORY.md` in `/data/memory/` on first run

### 2. Skills tab shows "No skills loaded"
- **Not a bug** — the skill loader works correctly
- **Cause**: `/data/skills/` directory is empty — no default skills ship with EazyClaw
- **Fix**: Create 4 default skill packages and bundle them in the Docker image

### 3. UI looks generic/flat
- The "Neural Command Center" aesthetic is barely present — it's just dark backgrounds with faint violet borders
- No visual depth, atmosphere, or premium polish
- Empty states are plain text with no visual interest
- IconRail has no branding, too minimal
- Cards look like basic dark boxes

---

## Planned Changes

### A. Seed Default Memory File

**File**: `entrypoint.sh`
- Add: if `/data/memory/MEMORY.md` doesn't exist, create it with a starter template
- Template contains sections: "# Agent Memory", "## Notes", "## Important Context"

### B. Create 4 Default Skills

Each skill lives in `/data/skills/<name>/skill.md` using EazyClaw's format:

#### 1. `web-research` — Deep Web Research
- Uses: `web_search`, `web_fetch`, `memory_write`
- Instructions: Multi-step research workflow — search, fetch top results, extract key info, synthesize findings, save to memory
- Tools: `research` (combines search + fetch + summarize)

#### 2. `code-review` — Code Analysis & Review
- Uses: `shell`, `read_file`, `list_dir`, `write_file`
- Instructions: Analyze code quality, find bugs, suggest improvements, check for security issues
- Tools: `analyze_code` (run linters, read files, produce review)

#### 3. `daily-digest` — Automated Daily Summary
- Uses: `web_search`, `memory_read`, `memory_write`, `cron`
- Instructions: Create daily briefings — check news, review memory for pending tasks, generate summary
- Tools: `create_digest` (search + summarize + write to memory)

#### 4. `task-manager` — Task & Todo Tracking
- Uses: `memory_read`, `memory_write`, `memory_search`, `cron`
- Instructions: Manage tasks in memory files — add/complete/list todos, set reminders via cron
- Tools: `manage_tasks` (CRUD operations on task lists in memory)

**Docker changes**:
- Copy skill directories into `/defaults/skills/` in Dockerfile
- `entrypoint.sh` copies defaults to `/data/skills/` if empty

### C. UI Overhaul — Premium "Neural Command Center"

#### C1. `index.css` — Foundation
- Add subtle dot grid pattern background (`background-image: radial-gradient(...)`)
- Add noise texture overlay for depth (CSS-only, no image)
- Better `.glass-card` with more visible frosted blur + stronger glow on hover
- New `.glass-card-glow` variant with animated border (gradient rotation)
- Animated gradient border utility for focused inputs
- Better selection/focus ring styles using violet glow
- New `.section-header` utility for consistent page headers
- Ambient gradient background utility class

#### C2. `IconRail.tsx` — Branding & Polish
- Add "EC" monogram logo at top (styled text, not an image)
- Wider rail: 64px instead of 56px
- Active tab: gradient left border (violet→cyan) + background glow + icon shadow
- Hover: subtle scale transform + color shift
- Divider line between navigation icons and connection status
- Better tooltip with glass effect

#### C3. `DashboardPage.tsx` — Layout
- Update grid to `grid-cols-[64px_1fr]` for wider IconRail
- Add subtle ambient gradient orbs in background (fixed position, pointer-events-none)

#### C4. `MemoryTab.tsx` + `FileTree.tsx` — Empty States
- Replace "No files" with visual empty state: Brain icon (large, muted) + "No memory files yet" + "Create your first memory file to get started" + prominent "New File" button
- Better toolbar with section header styling
- File tree items with better hover/active transitions

#### C5. `SkillsTab.tsx` — Empty State
- Replace "No skills loaded" with visual empty state: Puzzle icon + "No skills installed" + description of how skills work + link to skills directory

#### C6. `ChatInput.tsx` — Animated Border
- Replace static border with animated gradient border on focus (violet→cyan rotation)
- Better send button with gradient background
- Textarea instead of input for multi-line support

#### C7. `MessageBubble.tsx` — Better Styling
- User messages: gradient background (violet-tinted) instead of flat violet/10
- Assistant messages: subtle left accent border (cyan)
- Tool results: better code block styling with syntax highlighting colors
- Role labels with icon indicators

#### C8. `SessionList.tsx` — Better Design
- Session items with preview of last message
- Better active state with glow effect
- "New Chat" button at top

#### C9. `StatusTab.tsx` + `StatusCard.tsx` — Visual Flair
- Metric cards with large numbers and labels
- Animated status ring around provider icons
- Better grid layout with section headers

#### C10. `LoginPage.tsx` — Minor Polish
- Add subtle grid pattern to background
- Better input focus states matching new system

---

## File Change Summary

| File | Action |
|---|---|
| `entrypoint.sh` | Add memory + skills seeding |
| `Dockerfile` | Copy default skills to /defaults/skills/ |
| `data/skills/web-research/skill.md` | NEW — web research skill |
| `data/skills/code-review/skill.md` | NEW — code review skill |
| `data/skills/daily-digest/skill.md` | NEW — daily digest skill |
| `data/skills/task-manager/skill.md` | NEW — task manager skill |
| `ui/src/index.css` | Enhanced atmospheric CSS |
| `ui/src/components/layout/IconRail.tsx` | Branding + polish |
| `ui/src/pages/DashboardPage.tsx` | Wider grid + ambient effects |
| `ui/src/pages/LoginPage.tsx` | Minor polish |
| `ui/src/components/memory/MemoryTab.tsx` | Better empty state + toolbar |
| `ui/src/components/memory/FileTree.tsx` | Better empty state |
| `ui/src/components/chat/ChatInput.tsx` | Animated gradient border |
| `ui/src/components/chat/MessageBubble.tsx` | Better bubble styling |
| `ui/src/components/chat/SessionList.tsx` | Better session items |
| `ui/src/components/skills/SkillsTab.tsx` | Better empty state |
| `ui/src/components/status/StatusTab.tsx` | Better cards + headers |
| `ui/src/components/status/StatusCard.tsx` | Animated status indicators |

## Build & Verify

1. `go vet ./...`
2. `cd ui && yarn build`
3. `docker compose build`
4. `docker compose up -d`
5. Verify: memory files visible, 4 skills loaded, UI looks premium
