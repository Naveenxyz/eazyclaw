# Telegram Channel Setup

## Getting a Bot Token

1. Open Telegram and search for **@BotFather**
2. Send `/newbot` to start the creation wizard
3. Pick a **display name** for your bot (e.g. `My EazyClaw Bot`)
4. Pick a **username** — must be unique and end in `bot` (e.g. `my_eazyclaw_bot`)
5. BotFather will reply with your token — copy it immediately

> Treat this token like a password. Do not share it or commit it to version control.

## Environment Variable

```env
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIjklMNOpqrSTUvwxYZ
```

## Optional Bot Customization via BotFather

```
/setdescription   — Sets the bio shown on the bot's profile
/setuserpic       — Upload a profile picture for your bot
/setcommands      — Define slash commands shown in the command menu
```

## Configuration (config.yaml)

```yaml
channels:
  telegram:
    enabled: true
    dm_policy: pairing           # allow | deny | pairing
    group_policy: allowlist      # allowlist | open
    allowed_users:
      - 123456789                # Telegram user IDs (integers)
      - 987654321

    # Restrict bot to specific chats (groups or channels)
    allowed_chats:
      - -1001234567890           # Group/supergroup chat IDs (negative)
      - -1009876543210
```

### Key Config Options

| Option | Values | Description |
|---|---|---|
| `allowed_users` | list of user IDs | Users permitted to interact with the bot |
| `group_policy` | `allowlist` / `open` | Who can use the bot in group chats |
| `dm_policy` | `allow` / `deny` / `pairing` | Controls direct message behavior |
| `allowed_chats` | list of chat IDs | Restricts bot to specific groups or channels |

## User Approval Workflow

When `dm_policy` is set to `pairing`, new users must be approved before they can interact with the bot:

1. A new user sends the bot a message
2. Their request appears as **pending** in the EazyClaw web dashboard
3. Navigate to the **Settings** tab in the dashboard
4. Find the pending user ID and click **Approve**
5. The user can now interact with the bot

To find a user's Telegram ID, you can use `@userinfobot` on Telegram or check the pending requests in the dashboard Settings tab.
