# Getting Started with EazyClaw

Get your own AI agent running on Telegram in under 10 minutes.

## What You Need

1. A **Kimi Coding** subscription (~$9/month) — cheapest way to get Claude-quality AI
2. A **Telegram bot token** — free, takes 60 seconds
3. A **GitHub personal access token** — so your agent can use `gh` (optional but recommended)
4. A **Railway account** — for deployment ($5/month hobby plan)

That's it. Four things. Let's go.

---

## Step 1: Get a Kimi Coding API Key

Kimi Coding gives you access to Claude Sonnet via their API at a fraction of the cost.

1. Go to [kimi.com](https://kimi.com) and sign up
2. Subscribe to the **Coding plan**
3. Go to API settings and copy your API key — it starts with `sk-kimi-`

> Why Kimi? It's the cheapest way to get Claude-level intelligence. You can swap in your own `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` later if you prefer.

## Step 2: Create a Telegram Bot

1. Open Telegram and search for **@BotFather**
2. Send `/newbot`
3. Pick a display name (e.g. "My EazyClaw")
4. Pick a username — must end in `bot` (e.g. `myeazyclaw_bot`)
5. BotFather replies with a token like `8362090283:AAGGFUgyu67Ibp...` — copy it

That's your `TELEGRAM_BOT_TOKEN`.

## Step 3: Create a GitHub Token (Optional)

This lets your agent run `gh` commands — create repos, open PRs, manage issues.

1. Go to [github.com/settings/tokens](https://github.com/settings/tokens?type=beta)
2. Click **Generate new token** (fine-grained)
3. Give it a name, set expiration, select the repos you want access to
4. Grant permissions: **Contents** (read/write), **Issues** (read/write), **Pull requests** (read/write)
5. Copy the token — it starts with `github_pat_`

## Step 4: Deploy to Railway

Two options — **CLI** (fastest) or **Dashboard** (no install needed).

### Option A: Railway CLI (Recommended)

```bash
# Install Railway CLI (if you haven't)
npm install -g @railway/cli

# Login
railway login

# Clone and enter the repo
git clone https://github.com/Naveenxyz/eazyclaw.git
cd eazyclaw

# Create project + service
railway init --name eazyclaw
railway add --service eazyclaw
railway service link eazyclaw

# Set environment variables
railway variable set KIMI_API_KEY=sk-kimi-your-key-here
railway variable set TELEGRAM_BOT_TOKEN=your-token-here
railway variable set WEB_PASSWORD=pick-a-password
railway variable set GH_TOKEN=github_pat_your-token-here

# Add persistent volume
railway volume add --mount-path /data

# Deploy
railway up

# Generate a public URL
railway domain
```

That's it. Your dashboard URL will be printed at the end.

### Option B: Railway Dashboard

1. Fork this repo on GitHub
2. Go to [railway.app](https://railway.app) and create a new project
3. Choose **Deploy from GitHub repo** and select your fork
4. Go to **Variables** and add:
   - `KIMI_API_KEY` — your Kimi key
   - `TELEGRAM_BOT_TOKEN` — from BotFather
   - `WEB_PASSWORD` — pick anything
   - `GH_TOKEN` — your GitHub token
5. Go to **Settings** → **Volumes** → add a volume at `/data`
6. Go to **Settings** → **Networking** → **Generate Domain**
7. Wait for the build to finish (~3 minutes)

## Step 5: Start Chatting

Open Telegram, find your bot, and send a message. That's it.

The first message from your Telegram account will show up as a **pending approval** in the web dashboard. Open the dashboard (your Railway URL), log in with the `WEB_PASSWORD` you set, go to **Settings**, and approve your user ID.

After that, your agent is live and ready.

---

## What You Get

- **Telegram bot** that responds with Claude-level intelligence
- **Web dashboard** for chat, memory browsing, cron jobs, status monitoring, and settings
- **Persistent memory** — your agent remembers things across conversations
- **Shell access** — your agent can run commands, write code, use git
- **Cron scheduler** — your agent can set its own recurring tasks
- **Skills system** — extend your agent with custom tool packages

## Quick Local Setup (Alternative)

If you'd rather run locally:

```bash
git clone https://github.com/your-username/eazyclaw.git
cd eazyclaw

# Create .env
cat > .env << 'EOF'
KIMI_API_KEY=sk-kimi-your-key
TELEGRAM_BOT_TOKEN=your-token
WEB_PASSWORD=pick-a-password
GH_TOKEN=github_pat_your-token
EOF

# Build and run
docker compose up -d

# Open dashboard
open http://localhost:8080
```

## Next Steps

- Edit `SOUL.md` in the Memory tab to customize your agent's personality
- Set up Discord by adding `DISCORD_BOT_TOKEN` (see [README](README.md#getting-a-discord-bot-token))
- Enable the heartbeat runner in Settings for proactive check-ins
- Create cron jobs from the Cron tab for scheduled tasks
