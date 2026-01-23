# CLI Telegram Chat Summary

CLI tool for authorizing on Telegram, parsing chats with unread messages, and summarizing them using LLMs.

## Prerequisites

- Go 1.25+
- Telegram API credentials from [my.telegram.org](https://my.telegram.org)

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

## Usage

```bash
make build
./bin/tg-summary
```

Or run directly:
```bash
make run
```

## Project Structure

```
cmd/tg-summary/     - Main entry point
internal/
  config/           - Configuration loading
  telegram/         - Telegram client wrapper (gotd)
```

## Development

- `make build` - Build binary
- `make run` - Run without building
- `make lint` - Run linter
- `make clean` - Clean build artifacts
