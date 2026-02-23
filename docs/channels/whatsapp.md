# WhatsApp Channel Setup

## Enabling WhatsApp

Set the following in your `.env` file:

```env
WHATSAPP_ENABLED=true

# Optional: shared secret for authenticating the bridge connection
WHATSAPP_BRIDGE_TOKEN=your-shared-secret
```

## Starting the Services

WhatsApp support requires both the EazyClaw app and the WhatsApp bridge to run:

```bash
docker compose up -d --build
```

This starts:
- The main EazyClaw application
- The `whatsapp-bridge` container (Node.js + Baileys)

## First-Time Login (QR Code)

On first run, you must scan a QR code to link your WhatsApp account:

```bash
docker compose logs -f whatsapp-bridge
```

A QR code will appear in the logs. Open WhatsApp on your phone:

- Android: **Linked Devices** → **Link a Device**
- iPhone: **Settings** → **Linked Devices** → **Link a Device**

Scan the QR code. The session is saved to the `/data` volume and persists across restarts.

## Bridge Architecture

The WhatsApp integration uses a dedicated Node.js bridge built on the [Baileys](https://github.com/WhiskeySockets/Baileys) library:

- Bridge runs as a separate container (`whatsapp-bridge`)
- Communicates with EazyClaw via WebSocket at `ws://whatsapp-bridge:3001`
- Baileys connects to WhatsApp Web using a persistent session
- No official WhatsApp Business API required

## Configuration (config.yaml)

```yaml
channels:
  whatsapp:
    enabled: true
    bridge_url: ws://whatsapp-bridge:3001
    bridge_token: your-shared-secret    # Must match WHATSAPP_BRIDGE_TOKEN
    dm_policy: allow                    # allow | deny | pairing
    group_policy: allowlist             # allowlist | open
    allowed_users:
      - "15551234567@s.whatsapp.net"    # WhatsApp JID format
      - "15559876543@s.whatsapp.net"
```

## Channel Behavior (SOUL.md)

WhatsApp responses follow specific formatting rules defined in `SOUL.md`:

- Responses are **SHORT** — 1 to 3 sentences maximum
- **Plain text only** — no markdown, no bullet points, no bold/italic
- **Casual tone** — conversational, not formal or structured

This is intentional: WhatsApp is a messaging app, not a document viewer.

## Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| QR code expired | Took too long to scan | `docker compose restart whatsapp-bridge` to regenerate |
| Connection drops | Bridge lost connection | `docker compose logs -f whatsapp-bridge` to diagnose |
| Session lost | Volume not mounted | Ensure `/data` volume is configured in `docker-compose.yml` |
| Messages not received | Bridge not reachable | Verify `bridge_url` in config matches container name |

Session data is stored in the `/data` volume. Deleting this volume will require re-scanning the QR code.
