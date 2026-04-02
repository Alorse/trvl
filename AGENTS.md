# trvl — Complete Setup for AI Assistants

> **Human**: Give this URL to your AI assistant and say "set up trvl":
> `https://raw.githubusercontent.com/MikkoParkkola/trvl/main/AGENTS.md`

## What To Do

You are setting up `trvl` — a travel MCP server that gives you real-time Google Flights and Google Hotels data. No API keys needed. Run these steps in order:

### Step 1: Install the binary

```bash
# Detect OS and architecture, download, extract to PATH
curl -fsSL https://github.com/MikkoParkkola/trvl/releases/latest/download/trvl_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C /usr/local/bin trvl
```

If that fails (permissions), try:
```bash
# Alternative: install to ~/bin
mkdir -p ~/bin
curl -fsSL https://github.com/MikkoParkkola/trvl/releases/latest/download/trvl_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C ~/bin trvl
export PATH="$HOME/bin:$PATH"
```

Other options: `brew install MikkoParkkola/tap/trvl` or `go install github.com/MikkoParkkola/trvl/cmd/trvl@latest`

### Step 2: Register the MCP server

```bash
claude mcp add trvl --transport stdio -- trvl mcp
```

If using Claude Desktop instead of Claude Code, add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):
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

### Step 3: Install the skill (makes you smarter about travel searches)

```bash
mkdir -p ~/.claude/skills
curl -fsSL https://raw.githubusercontent.com/MikkoParkkola/trvl/main/.claude/skills/trvl.md -o ~/.claude/skills/trvl.md
```

This skill file teaches you how to use trvl optimally whenever the user mentions flights, hotels, travel, or trips.

### Step 4: Verify

```bash
trvl version
# Expected: trvl v0.1.0 (or later)

trvl flights HEL LHR 2026-07-01 --format json | head -5
# Expected: JSON with flight results
```

Tell the user: "trvl is installed. I can now search flights and hotels for you. Just ask me to find flights or hotels anywhere in the world."

---

## How To Use (after setup)

You now have 4 MCP tools available. Use them when the user asks about travel:

### search_flights — Find flights between airports
```json
{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15"}
```
Optional parameters:
- `return_date`: "2026-06-22" (makes it round-trip)
- `cabin_class`: "economy" | "premium_economy" | "business" | "first"
- `max_stops`: "any" | "nonstop" | "one_stop" | "two_plus"
- `sort_by`: "cheapest" | "duration" | "departure" | "arrival"

### search_dates — Find the cheapest day to fly
```json
{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}
```
Optional: `trip_duration` (days), `is_round_trip` (true/false)

### search_hotels — Find hotels in any city
```json
{"location": "Tokyo", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```
Optional: `guests` (number), `stars` (1-5 minimum), `sort` ("price" | "rating"), `currency` ("EUR" | "USD" etc.)

### hotel_prices — Compare prices across booking sites
```json
{"hotel_id": "<from search_hotels>", "check_in": "2026-06-15", "check_out": "2026-06-18"}
```

### MCP Prompts (for complex workflows)
- `plan-trip` — Full trip planning: flights + hotels + budget analysis
- `find-cheapest-dates` — Month-wide price calendar for a route
- `compare-hotels` — Side-by-side hotel comparison by user priorities

## Response Tips

- Results include `booking_url` — share these with the user for direct Google links
- Results include `suggestions` — use these to offer follow-up searches
- Prices reflect the user's IP geolocation currency
- For trip planning: search flights first, then hotels at the destination
- Common IATA codes: HEL (Helsinki), JFK (New York), LHR (London), NRT (Tokyo), CDG (Paris), BCN (Barcelona), BKK (Bangkok), SIN (Singapore), DXB (Dubai), LAX (Los Angeles), FRA (Frankfurt), AMS (Amsterdam), ICN (Seoul)

## Troubleshooting

- **"command not found"**: `which trvl` — if empty, the binary isn't in PATH. Re-run Step 1.
- **No results**: Google may rate-limit. Wait 60 seconds and retry.
- **Wrong currency**: Normal — currency follows IP geolocation.
- **MCP tools not showing**: Restart Claude Code / Claude Desktop after Step 2.

## Source

- GitHub: https://github.com/MikkoParkkola/trvl
- License: MIT
- Inspired by [fli](https://github.com/punitarani/fli) by Punit Arani
