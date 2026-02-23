# Discord Channel Setup

## Getting a Bot Token

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Click **New Application** and give it a name
3. Navigate to the **Bot** tab in the left sidebar
4. Click **Reset Token** and copy the token — this is your `DISCORD_BOT_TOKEN`

> Keep this token secret. Anyone with it can control your bot.

## Enabling Required Intents

Under the **Bot** tab, scroll to **Privileged Gateway Intents** and enable:

- **Message Content Intent** — required for EazyClaw to read message content

## Inviting the Bot to Your Server

1. Go to **OAuth2** → **URL Generator**
2. Under **Scopes**, select: `bot`
3. Under **Bot Permissions**, select:
   - Send Messages
   - Read Message History
   - View Channels
4. Copy the generated URL and open it in a browser
5. Select the server you want to add the bot to and authorize

## Environment Variable

```env
DISCORD_BOT_TOKEN=your-token-here
```

## Configuration (config.yaml)

```yaml
channels:
  discord:
    allowed_users:
      - "123456789012345678"     # Discord user IDs
      - "987654321098765432"
    group_policy: allowlist      # allowlist | open
    dm:
      policy: allow              # allow | deny | pairing

    # Guild-level overrides (optional)
    guilds:
      "111222333444555666":        # Guild (server) ID
        require_mention: true      # Bot only responds when @mentioned
        channels:
          "777888999000111222":     # Channel ID
            allow: true
            require_mention: false  # Override guild-level setting
```

### Key Config Options

| Option | Values | Description |
|---|---|---|
| `allowed_users` | list of user IDs | Users permitted to interact with the bot |
| `group_policy` | `allowlist` / `open` | Who can use the bot in group channels |
| `dm.policy` | `allow` / `deny` / `pairing` | Controls direct message behavior |
| `guilds.<id>.require_mention` | `true` / `false` | Whether bot requires @mention in a guild |
| `guilds.<id>.channels.<id>.allow` | `true` / `false` | Whether bot responds in a specific channel |
| `guilds.<id>.channels.<id>.require_mention` | `true` / `false` | Override guild-level mention requirement |
