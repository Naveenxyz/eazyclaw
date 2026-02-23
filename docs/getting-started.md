# Getting Started with EazyClaw

EazyClaw is an AI agent gateway — a self-hosted bot that connects AI models to Telegram, Discord, and a web dashboard, with persistent memory, a cron scheduler, and a skills system.

**Cheapest viable setup:** Kimi Coding (~$9/mo) + Railway ($5/mo) = **~$14/mo total**

---

## What You Need

1. **Kimi API key** — for the AI model
2. **Telegram bot token** — your main chat interface
3. **GitHub token** *(optional)* — `GH_TOKEN` for private repo access or code skills
4. **Railway account** — for hosting

---

## Step 1: Get a Kimi API Key

1. Go to [kimi.com](https://kimi.com) and sign up
2. Subscribe to the **Coding** plan (~$9/mo)
3. Navigate to API settings and generate a key
4. It will start with `sk-kimi-`

Keep this somewhere safe — you'll need it in Step 4.

---

## Step 2: Create a Telegram Bot

1. Open Telegram and search for **@BotFather**
2. Send `/newbot`
3. Follow the prompts — pick a display name, then a username
4. Username **must end in `bot`** (e.g. `myagent_bot`)
5. BotFather hands you a token — looks like `123456789:ABCdef...`

That token is your `TELEGRAM_BOT_TOKEN`.

---

## Step 3: GitHub Token *(Optional)*

Skip this if you don't need GitHub access in your agent.

1. Go to GitHub → Settings → Developer settings → Personal access tokens → **Fine-grained tokens**
2. Generate a new token with the repo scopes you need
3. Token starts with `github_pat_`

---

## Step 4: Deploy to Railway

Two options — pick one.

### Option A: Railway CLI *(Recommended)*

```bash
# Install Railway CLI
npm install -g @railway/cli

# Log in
railway login

# Clone and enter the repo
git clone https://github.com/your-org/eazyclaw.git
cd eazyclaw

# Initialize a new Railway project
railway init

# Set your environment variables
railway variables set KIMI_API_KEY=sk-kimi-...
railway variables set TELEGRAM_BOT_TOKEN=123456789:ABCdef...
railway variables set GH_TOKEN=github_pat_...      # optional

# Add persistent storage
railway volume add --mount /data

# Deploy
railway up
```

### Option B: Railway Dashboard

1. Fork this repo to your GitHub account
2. Go to [railway.app](https://railway.app) and create a new project
3. Choose **Deploy from GitHub repo** and select your fork
4. Under **Variables**, add:
   - `KIMI_API_KEY`
   - `TELEGRAM_BOT_TOKEN`
   - `GH_TOKEN` *(optional)*
5. Under **Volumes**, add a volume mounted at `/data`
6. Go to **Settings → Networking** and click **Generate Domain**
7. Hit **Deploy**

---

## Step 5: Start Chatting

1. Open Telegram and find your bot by its username
2. Send `/start` or just say hello
3. **First message requires approval** — go to your Railway dashboard, open the **Settings** tab, and approve the pending user request
4. After approval, the bot responds normally

---

## What You Get

| Feature | Description |
|---|---|
| Telegram bot | Primary chat interface |
| Web dashboard | Browser UI at your Railway domain |
| Persistent memory | Survives restarts — stored in `/data` |
| Shell access | Run commands via chat |
| Cron scheduler | Schedule recurring tasks |
| Skills system | Extend with custom skills |

---

## Quick Local Setup

Prefer to run locally first? Use Docker Compose.

```bash
git clone https://github.com/your-org/eazyclaw.git
cd eazyclaw

# Copy and fill in your env vars
cp .env.example .env
# Edit .env with your KIMI_API_KEY, TELEGRAM_BOT_TOKEN, etc.

docker compose up
```

The web dashboard runs at `http://localhost:8080` by default.

---

## Next Steps

- **Customize your agent** — edit `SOUL.md` to set personality, name, and behavior
- **Add Discord** — see [channels/discord.md](channels/discord.md)
- **Switch AI providers** — see [providers.md](providers.md) for OpenAI, Anthropic, Gemini, and more
- **Configure the agent** — see [configuration.md](configuration.md) for all env vars
- **Enable heartbeat** — set up a ping so Railway doesn't spin down your instance
- **Create cron jobs** — schedule daily summaries, reminders, or data fetches via the dashboard

---

*Something broken? Check the logs in Railway dashboard under the Deployments tab.*
