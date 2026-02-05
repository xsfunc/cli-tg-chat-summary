# CLI Telegram Chat Summary

CLI tool that authenticates with Telegram, lets you pick a chat (or forum topic) via TUI, exports unread messages or a date range to a text file, and optionally marks them as read.
This repository currently does not call an LLM; it only exports messages so they can be summarized elsewhere.

## Prerequisites

- Go 1.25+
- Telegram API credentials from [my.telegram.org](https://my.telegram.org)

## Quick Start

```bash
cp .env.example .env
sed -i 's/TG_APP_ID=.*/TG_APP_ID=123456/' .env
sed -i 's/TG_APP_HASH=.*/TG_APP_HASH=your_api_hash/' .env
sed -i 's/TG_PHONE=.*/TG_PHONE=+1234567890/' .env
make build
./bin/tg-summary
```

## Setup

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Set environment variables:
   ```bash
   export TG_APP_ID=your_app_id
   export TG_APP_HASH=your_app_hash
   export TG_PHONE=+1234567890  # optional
   ```

   Tip: You can copy `.env.example` to `.env` and fill in your details:
   ```bash
   cp .env.example .env
   ```

## Usage

```bash
make build
./bin/tg-summary
```

Or run directly:
```bash
make run
```


## Date Range Export

You can export messages from a specific date range instead of just unread messages.
When using date range mode, messages will NOT be marked as read.

```bash
# Export from a specific date until now
./bin/tg-summary --since 2024-01-01

# Export messages within a specific range
./bin/tg-summary --since 2024-01-01 --until 2024-01-31
```

## How It Works

1. Authenticate with Telegram using `gotgproto`.
2. Fetch dialogs and show them in a TUI list (Bubble Tea).
3. If the selected chat is a forum, show a second TUI to select a topic.
4. Export messages to `exports/<Chat_or_Topic>_<YYYY-MM-DD>.txt` (or `exports/<Chat_or_Topic>_<YYYY-MM-DD>_to_<YYYY-MM-DD>.txt` for date ranges).
5. In unread mode, mark messages as read up to the max exported ID.

## Configuration

Required:
- `TG_APP_ID` integer app ID from Telegram.
- `TG_APP_HASH` app hash from Telegram.

Optional:
- `TG_PHONE` phone number for login.
- `LOG_LEVEL` `debug|info|warn|error` (default `info`).
- `RATE_LIMIT_MS` request interval in milliseconds (default `350`).

The session file is stored at `session/session.db`.

## CLI Flags

- `--since YYYY-MM-DD` start date for export (enables date range mode).
- `--until YYYY-MM-DD` end date for export (defaults to now when omitted).

## Output Format

Exports are written to `exports/` with a header and collapsed blocks per sender ID:
- `Chat Summary: <title>`
- `Export Date: <RFC1123>`
- `Total Messages: <count>`
- `[HH:MM] id=<sender_id>:` followed by indented message lines

Example file:
```text
Chat Summary: Project Team
Export Date: Mon, 27 Jan 2025 10:35:12 UTC
Total Messages: 3

[09:12] id=123:
  Morning! Status update?
[09:18-09:22] id=456:
  API is green, frontend build is running.
  Build is green, pushing summary in 30 min.
```

## Project Structure

```
cmd/tg-summary/     - CLI entry point and flag parsing
internal/app/       - Orchestrates login, TUI flow, export, mark-as-read
internal/config/    - Env config loader (.env supported)
internal/telegram/  - Telegram client wrapper (gotd + gotgproto)
internal/tui/       - Bubble Tea TUI models for chat/topic selection
```

## Development

| Command | Description |
|---------|-------------|
| `make` | Show help with all available commands |
| `make all` | Full cycle: tidy, lint, test, build |
| `make ci` | CI pipeline: clean build from scratch |
| `make build` | Compile binary to `bin/` directory |
| `make install` | Install to `$GOPATH/bin` |
| `make run` | Run via `go run` (example: `make run ARGS="-since 24h"`) |
| `make exec` | Build and run binary |
| `make clean` | Clean build artifacts |
| `make tidy` | Update dependencies (`go mod tidy`) |
| `make lint` | Run golangci-lint |
| `make test` | Run unit tests |
| `make test-nocache` | Run tests without cache |
| `make test-cover` | Run tests with coverage report |
| `make setup-hooks` | Install git hooks |
