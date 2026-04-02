# trvl — Setup Instructions for AI Assistants

> Give your AI assistant this URL: `https://raw.githubusercontent.com/MikkoParkkola/trvl/main/AGENTS.md`

## What is trvl?

trvl is a CLI tool and MCP server for searching Google Flights and Google Hotels. Free, no API keys needed. Single binary.

## Quick Install (pick one)

```bash
# macOS/Linux — download binary
curl -fsSL https://github.com/MikkoParkkola/trvl/releases/latest/download/trvl_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C /usr/local/bin trvl

# Homebrew
brew install MikkoParkkola/tap/trvl

# Go
go install github.com/MikkoParkkola/trvl/cmd/trvl@latest
```

## Claude Code Skill (optional but recommended)

Install the trvl skill so Claude knows how to use it in any project:
```bash
curl -fsSL https://raw.githubusercontent.com/MikkoParkkola/trvl/main/.claude/skills/trvl.md -o ~/.claude/skills/trvl.md
```

This teaches Claude to use trvl's MCP tools when you mention flights, hotels, travel, or trips.

## MCP Server Setup

### Claude Code

Run this command:
```bash
claude mcp add trvl --transport stdio -- trvl mcp
```

Or add to `.claude.json`:
```json
{
  "mcpServers": {
    "trvl": {
      "command": "trvl",
      "args": ["mcp"]
    }
  }
}
```

### Claude Desktop

Add to `claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "trvl": {
      "command": "trvl",
      "args": ["mcp"]
    }
  }
}
```

### Cursor / Windsurf / Other MCP Clients

Add to your MCP config (usually `.cursor/mcp.json` or similar):
```json
{
  "mcpServers": {
    "trvl": {
      "command": "trvl",
      "args": ["mcp"]
    }
  }
}
```

### mcp-gateway

Add to `gateway.yaml`:
```yaml
backends:
  trvl:
    transport: stdio
    command: trvl mcp
```

## Available MCP Tools

### search_flights
Search flights between two airports.
```json
{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15"}
```
Optional: `return_date`, `cabin_class` (economy/business/first), `max_stops` (any/nonstop), `sort_by` (cheapest/duration)

### search_dates
Find the cheapest dates to fly.
```json
{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}
```
Optional: `trip_duration` (days), `is_round_trip` (bool)

### search_hotels
Search hotels in any city.
```json
{"location": "Tokyo", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```
Optional: `guests` (int), `stars` (1-5 minimum), `sort` (price/rating)

### hotel_prices
Compare booking provider prices for a specific hotel.
```json
{"hotel_id": "<id from search_hotels>", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```

## Available MCP Prompts

- `plan-trip` — Plan a complete trip with flights + hotels + budget
- `find-cheapest-dates` — Find the cheapest month to fly a route
- `compare-hotels` — Compare hotels by price, rating, or location

## CLI Usage (if not using MCP)

```bash
trvl flights HEL NRT 2026-06-15                    # Search flights
trvl flights HEL BCN 2026-07-01 --return 2026-07-08  # Round-trip
trvl dates HEL NRT --from 2026-06-01 --to 2026-06-30  # Cheapest dates
trvl hotels "Tokyo" --checkin 2026-06-15 --checkout 2026-06-18  # Hotels
trvl prices "<hotel_id>" --checkin 2026-06-15 --checkout 2026-06-18  # Prices

# Add --format json for structured output
trvl flights HEL NRT 2026-06-15 --format json
```

## Verify It Works

```bash
trvl version          # Should print: trvl v0.1.0
trvl flights HEL LHR 2026-07-01  # Should return flight results
```

## Troubleshooting

- **"command not found"**: Make sure the binary is in your PATH. Try `which trvl`.
- **No results**: Google may rate-limit. Wait a minute and retry.
- **Wrong currency**: Currency follows your IP geolocation. Use a VPN for different currency.

## Source

GitHub: https://github.com/MikkoParkkola/trvl
License: MIT
