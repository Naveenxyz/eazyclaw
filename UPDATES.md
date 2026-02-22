# EazyClaw UI Overhaul Plan

## Current State
- Container running healthy with WhatsApp bridge
- Memory fully working (9 files seeded via entrypoint.sh)
- Skills empty (out of scope for now)
- UI is functional but visually flat — needs the premium "Neural Command Center" treatment

## Problem
The UI looks like generic dark-mode Tailwind. Missing:
- Visual depth and atmosphere (no textures, patterns, ambient effects)
- Branding presence in the navigation
- Proper empty states (just plain "No files" / "No skills loaded" text)
- Premium card effects (current cards are barely visible dark boxes)
- Animated accents on interactive elements
- Distinctive character — nothing memorable or premium-feeling

---

## Changes (UI Only — 12 files)

### 1. `ui/src/index.css` — Design Foundation
- Subtle dot grid pattern on body background for depth
- CSS noise texture overlay (pure CSS, no image assets)
- Enhanced `.glass-card` with stronger backdrop-blur + visible border glow on hover
- New `.animated-border` utility — rotating gradient border for focused inputs/active elements
- `.section-header` utility for consistent page/section titles with gradient underline
- `.empty-state` utility for centered placeholder layouts
- Better scrollbar with thicker track on hover
- Enhanced focus-visible ring using violet glow
- Ambient gradient keyframe animation for background orbs

### 2. `ui/src/components/layout/IconRail.tsx` — Branding & Navigation
- "EC" monogram at top with gradient text (violet→cyan)
- Wider: 64px (was 56px)
- Active tab: gradient left border + radial glow behind icon + brighter icon color
- Hover: subtle translateX(2px) + color lift
- Separator line between nav icons and status dot
- Tooltip: glass-card styled with arrow indicator

### 3. `ui/src/pages/DashboardPage.tsx` — Layout
- Grid: `grid-cols-[64px_1fr]` (match wider IconRail)
- Fixed ambient gradient orbs behind content (pointer-events-none, z-0)
- Subtle top-right and bottom-left corner glows

### 4. `ui/src/pages/LoginPage.tsx` — Minor Polish
- Add dot grid pattern to background layer
- Animated gradient ring around the login card border
- Better focus states on input matching new system

### 5. `ui/src/components/memory/MemoryTab.tsx` — Toolbar & Layout
- Section header with gradient accent bar
- Better toolbar styling (elevated, with subtle bottom glow)
- "New File" button with gradient + hover glow

### 6. `ui/src/components/memory/FileTree.tsx` — Empty State
- Replace "No files" text with visual empty state:
  - Large muted Brain icon (48px)
  - "No memory files yet" heading
  - "Files will appear here as the agent builds memory" subtext
- Better tree item hover states with smooth transitions
- Active file: left gradient bar + subtle bg glow

### 7. `ui/src/components/chat/ChatInput.tsx` — Premium Input
- Animated gradient border on focus (violet→cyan, rotating via `@property`)
- Textarea (not input) for multi-line messages
- Send button: gradient background (violet→indigo), icon + text, hover glow
- Container: stronger glass effect with visible blur

### 8. `ui/src/components/chat/MessageBubble.tsx` — Better Bubbles
- User: gradient bg (subtle violet tint), right-aligned with stronger border
- Assistant: left cyan accent bar (3px), slightly elevated surface
- Tool results: monospace with better bg contrast, cyan left border
- Remove "You" / "Assistant" labels — replace with small avatar circles (colored dots)

### 9. `ui/src/components/chat/SessionList.tsx` — Better Sessions
- Header: "Sessions" with section-header styling
- Active session: gradient left border + background glow + stronger text
- Hover: smooth bg transition
- Monospace IDs with better truncation

### 10. `ui/src/components/skills/SkillsTab.tsx` — Empty State
- Visual empty state:
  - Large muted Puzzle icon (48px)
  - "No skills installed" heading
  - "Drop skill packages into /data/skills/ to extend the agent" subtext
- Section header when skills exist

### 11. `ui/src/components/status/StatusTab.tsx` + `StatusCard.tsx` — Visual Flair
- Section headers for "Providers", "Channels", "Sessions"
- StatusCard: animated pulsing green ring around status dot
- Session count: large monospace number with label
- Cards: better glass effect with hover elevation

### 12. `ui/src/components/chat/MessageList.tsx` — Empty Chat State
- Replace "Select a session or start chatting" with visual empty state:
  - Large muted MessageSquare icon
  - "Start a conversation" heading
  - "Type a message below to begin" subtext

---

## Files Changed

| File | Change |
|---|---|
| `ui/src/index.css` | Design system foundation — grid bg, noise, glass cards, utilities |
| `ui/src/components/layout/IconRail.tsx` | EC branding, wider, better active/hover states |
| `ui/src/pages/DashboardPage.tsx` | Wider grid, ambient background glows |
| `ui/src/pages/LoginPage.tsx` | Grid pattern bg, animated card border |
| `ui/src/components/memory/MemoryTab.tsx` | Better toolbar styling |
| `ui/src/components/memory/FileTree.tsx` | Visual empty state, better tree items |
| `ui/src/components/chat/ChatInput.tsx` | Animated gradient border, textarea, better send btn |
| `ui/src/components/chat/MessageBubble.tsx` | Gradient user bubbles, cyan assistant accent, avatar dots |
| `ui/src/components/chat/SessionList.tsx` | Better active state, section header |
| `ui/src/components/chat/MessageList.tsx` | Visual empty chat state |
| `ui/src/components/skills/SkillsTab.tsx` | Visual empty state |
| `ui/src/components/status/StatusTab.tsx` | Section headers, better layout |
| `ui/src/components/status/StatusCard.tsx` | Animated status ring, hover elevation |

## No Backend Changes
- No Go code touched
- No Dockerfile/entrypoint changes
- No config changes

## Verify
1. `cd ui && yarn build` — must compile
2. `docker compose build && docker compose up -d`
3. Visual check: every tab should feel polished and atmospheric
