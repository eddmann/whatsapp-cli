---
name: whatsapp
description: Query WhatsApp chats, messages, contacts, groups. Send, forward, react. Use when the user asks about their WhatsApp messages, wants to search history, send messages, or manage chats.
---

# whatsapp-cli Skill

Query and manage WhatsApp data via the `whatsapp` CLI.

## Prerequisites

- Install CLI: `curl -fsSL https://raw.githubusercontent.com/eddmann/whatsapp-cli/main/install.sh | sh`
- Authenticate: `whatsapp auth login` (scan QR code)

## Quick Context

Get aggregated WhatsApp data in one call:

```bash
whatsapp context                    # Connection status + recent chats with messages
whatsapp context --chats 10         # More recent chats
whatsapp context --messages 20      # More messages per chat
```

Messages come from a local database. Run `whatsapp sync` to fetch latest messages if history seems stale.

## Commands

Run `whatsapp --help` or `whatsapp <command> --help` to discover all options.

### Chats & Messages

```bash
whatsapp chats [--query NAME] [--groups] [--limit N]
whatsapp messages <JID> [--timeframe today] [--type image] [--limit N]
whatsapp search "keyword" [--chat JID] [--timeframe this_week]
```

### Send, Forward, React

**Always verify the recipient JID before sending.** Look up the contact first, confirm the name matches, then send:

```bash
# Step 1: Verify recipient
whatsapp chats --query "John" | jq -r '.[0] | "\(.jid) — \(.name)"'
# Step 2: Send only after confirming the JID matches the intended recipient
whatsapp send <JID> "message" [--file photo.jpg] [--reply-to MSG_ID]
whatsapp forward <TO_JID> <MSG_ID> --from <SOURCE_JID>
whatsapp react <MSG_ID> "thumbsup" --chat <JID>
```

### Groups

```bash
whatsapp groups [JID]               # List or get info
whatsapp groups join <CODE>         # Join via invite
whatsapp groups leave <JID>
```

### Other

```bash
whatsapp contacts [--query NAME]
whatsapp alias [JID NAME] [--remove]
whatsapp download <MSG_ID> --chat <JID>
whatsapp export <JID> [--output file.json]
whatsapp sync [--follow]
whatsapp doctor [--connect]
```

## JID Types

| Type       | Format                   | Example                     |
| ---------- | ------------------------ | --------------------------- |
| Individual | `<phone>@s.whatsapp.net` | `1234567890@s.whatsapp.net` |
| Group      | `<id>@g.us`              | `123456789-987654321@g.us`  |

Use `whatsapp chats` to look up JIDs.

## Timeframe Presets

`last_hour`, `today`, `yesterday`, `last_3_days`, `this_week`, `last_week`, `this_month`

## Message Types

`text`, `image`, `video`, `audio`, `document`, `sticker`

## Data Units

| Field      | Format                                     |
| ---------- | ------------------------------------------ |
| timestamp  | ISO8601 UTC (e.g., `2025-12-15T10:30:00Z`) |
| jid        | WhatsApp JID (see JID Types above)         |
| is_from_me | boolean                                    |

## Common Patterns

```bash
# Find a chat and read messages
whatsapp chats --query "John" | jq -r '.[0].jid'
whatsapp messages 1234567890@s.whatsapp.net --timeframe today

# Search and reply
whatsapp search "meeting" --timeframe this_week
whatsapp send <JID> 'See you there!' --reply-to <MSG_ID>

# Filter with jq
whatsapp messages <JID> | jq '[.[] | select(.is_from_me==false)]'

# Export for analysis
whatsapp export <JID> --output chat.json
```

## Auth Status

```bash
whatsapp auth status    # Check if authenticated
whatsapp auth login     # QR code auth
whatsapp auth logout    # Clear session
```

## Troubleshooting

```bash
# Auth expired or connection dropped
whatsapp auth status          # Check current state
whatsapp auth login           # Re-authenticate if needed

# Messages seem stale or missing
whatsapp sync                 # Pull latest messages
whatsapp doctor --connect     # Diagnose connection issues
```

If a send/forward fails, check `whatsapp auth status` first — an expired session is the most common cause. Re-run `whatsapp auth login` and retry.

## Exit Codes

- 0 = Success
- 1 = General error (check stderr)
