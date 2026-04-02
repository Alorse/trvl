# trvl — Travel MCP Server for AI Assistants

> **Real-time Google Flights + Hotels data for your AI assistant. Free. No API keys. One binary.**

Give your AI the power to search flights and hotels:

```
"Find me the cheapest nonstop flight from Helsinki to Barcelona in July"
"Search 4-star hotels in Tokyo for next weekend under $200/night"
"When is the cheapest time to fly JFK to London this summer?"
```

trvl is an [MCP server](https://modelcontextprotocol.io/) that gives Claude, Cursor, Windsurf, and any MCP-compatible AI assistant direct access to Google Flights and Google Hotels data — no API keys, no monthly fees, no scraping.

## Quick Setup (30 seconds)

### 1. Install

```bash
# macOS / Linux
curl -fsSL https://github.com/MikkoParkkola/trvl/releases/latest/download/trvl_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz -C /usr/local/bin trvl
```

<details>
<summary>More install options</summary>

```bash
# Homebrew
brew install MikkoParkkola/tap/trvl

# Go
go install github.com/MikkoParkkola/trvl/cmd/trvl@latest

# Docker
docker run --rm ghcr.io/mikkoparkkola/trvl flights HEL NRT 2026-06-15

# Build from source
git clone https://github.com/MikkoParkkola/trvl.git && cd trvl && make build
```

</details>

### 2. Connect to your AI assistant

**Claude Code:**
```bash
claude mcp add trvl --transport stdio -- trvl mcp
```

**Claude Desktop** — add to `claude_desktop_config.json`:
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

**Cursor / Windsurf / Other MCP clients** — add to your MCP config:
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

### 3. (Optional) Install the Claude Code skill

The repo includes a skill file at `.claude/skills/trvl.md` that teaches Claude how to use trvl optimally. If you cloned the repo, it loads automatically when working in the trvl directory. To install globally:

```bash
cp .claude/skills/trvl.md ~/.claude/skills/trvl.md
```

Now Claude knows about trvl in every project — just say "search flights" or "find hotels".

### 4. Ask your AI to search

That's it. Your AI assistant now has 4 travel tools available. Just ask naturally:

- *"Search flights from JFK to Tokyo on July 1st, business class"*
- *"Find hotels in Paris for July 1-5, at least 4 stars"*
- *"What's the cheapest day to fly Helsinki to Barcelona in August?"*
- *"Compare hotel prices for the Hilton in London"*

## MCP Tools

| Tool | What it does | Example |
|------|-------------|---------|
| **search_flights** | Search flights on a specific date | HEL → NRT, 2026-06-15, business class, nonstop |
| **search_dates** | Find cheapest day to fly across a date range | HEL → BCN, June-August 2026 |
| **search_hotels** | Search hotels in any city | Tokyo, June 15-18, 4+ stars |
| **hotel_prices** | Compare prices across booking providers | Booking.com vs Expedia vs Hotels.com |

### MCP Protocol Features (v2025-11-25)

| Feature | Details |
|---------|---------|
| **Structured content** | Typed JSON (`structuredContent`) alongside human-readable summaries |
| **Content annotations** | `audience: ["user"]` for summaries, `audience: ["assistant"]` for data |
| **Output schemas** | Full JSON Schema validation for all tool responses |
| **Prompts** | `plan-trip`, `find-cheapest-dates`, `compare-hotels` |
| **Resources** | Airport codes (50 major hubs), flight/hotel usage guides |
| **Progressive disclosure** | Suggestions for follow-up searches in every response |
| **Booking links** | Direct Google Flights/Hotels links in results |

### mcp-gateway Integration

```yaml
backends:
  trvl:
    transport: stdio
    command: trvl mcp
```

## For AI Assistants

Point your AI to this URL for automatic setup:

```
https://raw.githubusercontent.com/MikkoParkkola/trvl/main/AGENTS.md
```

Or the quick reference:

```
https://raw.githubusercontent.com/MikkoParkkola/trvl/main/llms.txt
```

---

## CLI Usage

trvl also works as a standalone CLI tool:

### Flights

```bash
$ trvl flights HEL NRT 2026-06-15

Found 86 flights (one_way)

| Price    | Duration | Stops   | Route                    | Airline               | Departs          |
+----------+----------+---------+--------------------------+-----------------------+------------------+
| EUR 603  | 24h 20m  | 2 stops | HEL -> CPH -> AUH -> NRT | Scandinavian Airlines | 2026-06-15T06:10 |
| EUR 656  | 24h 10m  | 2 stops | HEL -> CPH -> AUH -> NRT | Finnair               | 2026-06-15T06:20 |
| EUR 875  | 31h 20m  | 1 stop  | HEL -> IST -> NRT        | Turkish Airlines      | 2026-06-15T19:35 |
```

```bash
trvl flights JFK LHR 2026-07-01 --cabin business --stops nonstop
trvl flights HEL BCN 2026-07-01 --return 2026-07-08
trvl flights HEL NRT 2026-06-15 --format json       # JSON output
```

### Cheapest Dates

```bash
trvl dates HEL NRT --from 2026-06-01 --to 2026-06-30
trvl dates HEL BCN --from 2026-07-01 --to 2026-08-31 --duration 7 --round-trip
```

### Hotels

```bash
$ trvl hotels "Tokyo" --checkin 2026-06-15 --checkout 2026-06-18

Found 20 hotels:

| Name                              | Stars | Rating | Reviews | Price   |
+-----------------------------------+-------+--------+---------+---------+
| HOTEL MYSTAYS PREMIER Omori       | 4     | 4.1    | 2059    | 150 EUR |
| Hotel JAL City Tokyo Toyosu       | 4     | 4.2    | 1080    | 89 EUR  |
```

```bash
trvl hotels "Paris" --checkin 2026-07-01 --checkout 2026-07-05 --stars 4 --sort rating
trvl prices "<hotel_id>" --checkin 2026-06-15 --checkout 2026-06-18
```

## How It Works

Google's travel frontend uses an internal gRPC-over-HTTP protocol called **batchexecute**. `trvl` speaks this protocol natively:

1. **Chrome TLS fingerprint** — [utls](https://github.com/refraction-networking/utls) impersonates Chrome's exact TLS ClientHello
2. **Flights** — `FlightsFrontendService/GetShoppingResults` with encoded filter arrays
3. **Hotels** — `TravelFrontendUi` embedded JSON parsing from `AF_initDataCallback` blocks
4. **Hotel prices** — `TravelFrontendUi/data/batchexecute` with rpcid `yY52ce`
5. **Rate limiting** — 10 req/s token bucket with exponential backoff on 429/5xx

No Selenium. No Puppeteer. No browser. Just HTTP.

## At a Glance

| | |
|---|---|
| **Binary** | Single static 15MB. Zero runtime dependencies. |
| **Data** | Real-time from Google Flights + Google Hotels |
| **Auth** | None. No API keys, no accounts, no tokens. |
| **MCP** | Full v2025-11-25 — tools, prompts, resources, structured content |
| **Output** | Pretty tables (default) or JSON (`--format json`) |
| **Platforms** | Linux, macOS (amd64, arm64) |
| **Tests** | 325 functions, race-detector clean, 80%+ coverage |
| **License** | MIT |

## Attribution

Built on the shoulders of:

- **[fli](https://github.com/punitarani/fli)** by Punit Arani — the original Google Flights reverse-engineering library
- **[utls](https://github.com/refraction-networking/utls)** — Chrome TLS fingerprint impersonation
- **[icecreamsoft](https://icecreamsoft.hashnode.dev/building-a-web-app-for-travel-search)** — Google Hotels batchexecute documentation
- **[SerpAPI](https://serpapi.com/google-hotels-api)** — Hotel parameter reference

## Legal

`trvl` accesses Google's public-facing internal APIs. It does not bypass authentication, access protected content, or circumvent rate limits. Same approach as [fli](https://github.com/punitarani/fli) (1K+ stars, MIT licensed).

## License

[MIT](LICENSE)
