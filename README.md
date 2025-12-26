# WhatsApp CLI

![WhatsApp CLI](docs/heading.png)

WhatsApp from your terminal. Pipe it, script it, automate it.

> Exploring CLI tools as skills for AI agents. [Background below](#background).

## Features

- All your WhatsApp data — chats, messages, contacts, groups, media
- Script and automate — composable with jq, pipes, xargs, and standard Unix tools
- [AI agent ready](#ai-agent-integration) — install the skill for Claude, Cursor, and other assistants
- Flexible output — JSON for scripts, CSV for spreadsheets, tables for humans

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/eddmann/whatsapp-cli/main/install.sh | sh
```

Downloads the pre-built binary for your platform (macOS/Linux) to `~/.local/bin`.

### Homebrew

```bash
brew install eddmann/tap/whatsapp-cli
```

### From Source

Requires Go 1.24+ with CGO enabled (for SQLite). FFmpeg optional for audio conversion.

```bash
git clone https://github.com/eddmann/whatsapp-cli
cd whatsapp-cli
make build

# Binary at ./dist/whatsapp
```

## Quick Start

```bash
# 1. Authenticate with WhatsApp (scan QR code)
whatsapp auth login

# 2. List your chats to find JIDs
whatsapp chats

# 3. Read messages from a chat
whatsapp messages 1234567890@s.whatsapp.net

# 4. Send a message
whatsapp send 1234567890@s.whatsapp.net "Hello!"

# 5. Search across all messages
whatsapp search "meeting tomorrow"
```

## Command Reference

### Global Options

| Flag            | Description                                           |
| --------------- | ----------------------------------------------------- |
| `-f, --format`  | Output format: json (default), jsonl, csv, tsv, human |
| `--fields`      | Comma-separated fields to include in output           |
| `--no-header`   | Skip header row in CSV/TSV output                     |
| `--store DIR`   | Override store directory                              |
| `--timeout DUR` | Command timeout (default: 30s)                        |
| `-v, --verbose` | Verbose logging to stderr                             |
| `-V, --version` | Show version                                          |

### Authentication

```bash
whatsapp auth login      # QR code auth + initial sync
whatsapp auth logout     # Disconnect and clear session
whatsapp auth status     # Show connection status and DB stats
```

### Sync

```bash
whatsapp sync            # One-time message sync
whatsapp sync --follow   # Continuous sync (daemon mode)
```

### Chats & Messages

```bash
whatsapp chats                    # List all chats
whatsapp chats --groups           # Groups only
whatsapp chats --query "John"     # Filter by name

whatsapp messages <jid>           # View messages
whatsapp messages <jid> --limit 100
whatsapp messages <jid> --timeframe today
whatsapp messages <jid> --type image
```

### Search

```bash
whatsapp search "keyword"
whatsapp search "keyword" --chat <jid>
whatsapp search "keyword" --timeframe this_week
```

### Send, Forward, React

```bash
whatsapp send <jid> "message"
whatsapp send <jid> --file photo.jpg --caption "Check this"
whatsapp send <jid> "Reply" --reply-to <msg-id>

whatsapp forward <to-jid> <msg-id> --from <source-jid>

whatsapp react <msg-id> "thumbsup" --chat <jid>
whatsapp react <msg-id> --remove --chat <jid>
```

### Groups

```bash
whatsapp groups                   # List groups
whatsapp groups <jid>             # Group info + members
whatsapp groups join <code>       # Join via invite
whatsapp groups leave <jid>
whatsapp groups rename <jid> "Name"
```

### Other Commands

```bash
whatsapp contacts [--query]
whatsapp alias [<jid> <name>] [--remove]
whatsapp download <msg-id> --chat <jid>
whatsapp export <jid> [--output file.json]
whatsapp context [--chats N] [--messages N]
whatsapp doctor [--connect]
```

## Timeframe Presets

Use with `--timeframe` on messages and search:

| Preset        | Description        |
| ------------- | ------------------ |
| `last_hour`   | Past 60 minutes    |
| `today`       | Since midnight     |
| `yesterday`   | Yesterday only     |
| `last_3_days` | Past 3 days        |
| `this_week`   | Since Monday       |
| `last_week`   | Previous week      |
| `this_month`  | Since 1st of month |

## Composability

```bash
# Filter groups by name
whatsapp chats --groups | jq '.[] | select(.name | contains("work"))'

# Search today's messages and format
whatsapp search "meeting" --timeframe today | jq -r '.[] | "\(.sender_name): \(.content)"'

# Export messages to CSV
whatsapp messages <jid> --format csv > messages.csv
```

## Configuration

### Storage Location

All data is stored in `~/.config/whatsapp-cli/`:

```
~/.config/whatsapp-cli/
├── store/
│   ├── session.db      # WhatsApp session (whatsmeow)
│   ├── messages.db     # Messages & chats (SQLite + FTS5)
│   └── <jid>/          # Downloaded media files
└── aliases.json        # Local JID aliases
```

### Environment Variables

| Variable          | Description                                          |
| ----------------- | ---------------------------------------------------- |
| `WHATSAPP_FORMAT` | Default output format (json, jsonl, csv, tsv, human) |
| `XDG_CONFIG_HOME` | Override config directory base                       |

## AI Agent Integration

This CLI is available as an [Agent Skill](https://agentskills.io/) — it works with Claude Code, Cursor, and other compatible AI agents. See [`SKILL.md`](SKILL.md) for the skill definition.

### Install Agent Skill

```bash
curl -fsSL https://raw.githubusercontent.com/eddmann/whatsapp-cli/main/install-skill.sh | sh
```

Installs the skill to `~/.claude/skills/whatsapp/` and `~/.cursor/skills/whatsapp/`. Agents will auto-detect when you ask about WhatsApp messages.

## Development

```bash
git clone https://github.com/eddmann/whatsapp-cli
cd whatsapp-cli
make build                    # Build binary
make test                     # Run tests
make dev CMD="chats --limit 5"  # Build and run
```

## Background

I recently built [whatsapp-mcp](https://github.com/eddmann/whatsapp-mcp), an MCP server for WhatsApp. This got me thinking about alternative approaches to giving AI agents capabilities.

There's been a lot of discussion around the heavyweight nature of MCP. An alternative approach is to give agents [discoverable skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills) via well-documented CLI tooling. Give an LLM a terminal and let it use composable CLI tools to build up functionality and solve problems — the Unix philosophy applied to AI agents.

This project is an exploration of [Claude Code Skills](https://simonwillison.net/2025/Oct/16/claude-skills/) and the emerging [Agent Skills](https://agentskills.io/) standard for AI-tool interoperability. The goal was to build a CLI that works seamlessly as both:

1. **A traditional Unix tool** — composable, pipe-friendly, machine-readable
2. **An AI agent skill** — structured output, comprehensive documentation, predictable behavior

Going forward, another approach worth exploring is going one step further than CLI and providing a [code library that agents can import and use directly](https://www.anthropic.com/engineering/code-execution-with-mcp).

## License

MIT

## Credits

Built on [whatsmeow](https://github.com/tulir/whatsmeow).
