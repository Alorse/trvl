[![Go Report Card](https://goreportcard.com/badge/github.com/MikkoParkkola/trvl)](https://goreportcard.com/report/github.com/MikkoParkkola/trvl)
[![CI](https://github.com/MikkoParkkola/trvl/actions/workflows/ci.yaml/badge.svg)](https://github.com/MikkoParkkola/trvl/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/github/v/release/MikkoParkkola/trvl)](https://github.com/MikkoParkkola/trvl/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/MikkoParkkola/trvl.svg)](https://pkg.go.dev/github.com/MikkoParkkola/trvl)
[![MCP](https://img.shields.io/badge/MCP-2025--11--25-blue)](https://modelcontextprotocol.io)

# trvl — Travel MCP Server for AI Assistants

![trvl demo](demo.gif)

> **Real-time Google Flights + Hotels data for your AI assistant. Free. No API keys. One binary.**

### What it looks like

> **You:** I have €300 and a free weekend. Surprise me.
>
> **Claude (with trvl):** 🎲 Checking cheapest destinations from Helsinki...
>
> **Dubrovnik, Croatia** 🇭🇷
> ```
> ✈️ Ryanair HEL→DBV Fri 14:25→17:10 (nonstop) — €167 RT
> 🏨 Old Town Studios, 4.6★ — €42/night × 2 = €84
> 🌡️ 26°C, sunny, Adriatic swimming
> 💰 Total: €251 (€49 under budget!)
>
> Optimizations applied:
>   Fly Friday not Saturday: -€48
>   Split airlines (Ryanair out, easyJet back): -€31
>
> 📊 Naive booking: €350 → Optimized: €251 → Saved: €99 (28%)
> ```
> Want me to check nearby restaurants or events that weekend?

trvl is an [MCP server](https://modelcontextprotocol.io/) + CLI that gives Claude, Cursor, Windsurf, and any MCP-compatible AI assistant direct access to Google Flights and Google Hotels data. It searches, optimizes, and applies travel hacks automatically — no API keys, no monthly fees, no scraping.

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

### 3. (Optional) Install Claude Code skills

The repo includes 4 skill files that teach Claude how to use trvl optimally. To install globally:

```bash
mkdir -p ~/.claude/skills
for s in trvl travel-hacks travel-agent travel-agent-compact; do
  curl -fsSL "https://raw.githubusercontent.com/MikkoParkkola/trvl/main/.claude/skills/$s.md" -o "$HOME/.claude/skills/$s.md"
done
```

Now Claude knows about trvl in every project — just say "search flights" or "plan a trip".

### 4. Ask your AI to search

That's it. Your AI assistant now has 9 travel tools available. Just ask naturally:

- *"Search flights from JFK to Tokyo on July 1st, business class"*
- *"Find hotels in Paris for July 1-5, at least 4 stars"*
- *"What's the cheapest day to fly Helsinki to Barcelona in August?"*
- *"Where can I fly cheaply from Helsinki this weekend?"*
- *"How much would a week in Barcelona cost — flights and hotel?"*
- *"When should I fly to London? Check dates around July 15th"*
- *"Plan a trip: Helsinki -> Barcelona -> Rome -> Paris, cheapest routing"*

## MCP Tools

| Tool | What it does | Example |
|------|-------------|---------|
| **search_flights** | Search flights on a specific date | HEL -> NRT, 2026-06-15, business class, nonstop |
| **search_dates** | Find cheapest day to fly across a date range | HEL -> BCN, June-August 2026 |
| **search_hotels** | Search hotels in any city | Tokyo, June 15-18, 4+ stars |
| **hotel_prices** | Compare prices across booking providers | Booking.com vs Expedia vs Hotels.com |
| **destination_info** | Travel intelligence for any city | Tokyo: weather, safety, holidays, currency |
| **calculate_trip_cost** | Estimate total trip cost (flights + hotel) | HEL -> BCN, Jul 1-8, 2 guests |
| **weekend_getaway** | Find cheap weekend destinations | From HEL in July, budget EUR 500 |
| **suggest_dates** | Smart date suggestions around a target date | HEL -> BCN around Jul 15, +/- 7 days |
| **optimize_multi_city** | Find cheapest routing for multi-city trips | HEL -> BCN, ROM, PAR -> HEL |

### MCP Protocol Features (v2025-11-25)

| Feature | Details |
|---------|---------|
| **Structured content** | Typed JSON (`structuredContent`) alongside human-readable summaries |
| **Content annotations** | `audience: ["user"]` for summaries, `audience: ["assistant"]` for data |
| **Output schemas** | Full JSON Schema validation for all 9 tool responses |
| **Prompts** | `plan-trip`, `find-cheapest-dates`, `compare-hotels` |
| **Resources** | Airport codes (50 major hubs), flight/hotel usage guides |
| **Elicitation** | Interactive parameter collection when dates are missing |
| **Progressive disclosure** | Suggestions for follow-up searches in every response |
| **Booking links** | Direct Google Flights/Hotels links in results |

## How trvl Compares

| Feature | trvl | fli | Google Flights | Skyscanner | Kiwi |
|---------|------|-----|---------------|------------|------|
| Flight search | ✅ | ✅ | ✅ | ✅ | ✅ |
| Hotel search | ✅ | ❌ | ❌ | ❌ | ❌ |
| Hotel reviews | ✅ | ❌ | ❌ | ❌ | ❌ |
| Trip cost calculator | ✅ | ❌ | ❌ | ❌ | ❌ |
| Explore destinations | ✅ | ❌ | ✅ (web only) | ✅ (web) | ✅ |
| Multi-city optimizer | ✅ | ❌ | ❌ | ❌ | ⚠️ (1.7★) |
| Destination intelligence | ✅ | ❌ | ❌ | ❌ | ❌ |
| Travel hacks (auto-applied) | ✅ | ❌ | ❌ | ❌ | ❌ |
| MCP server | ✅ | ✅ | ❌ | ❌ | ❌ |
| Personal profile | ✅ | ❌ | ❌ | ❌ | ❌ |
| CLI | ✅ | ✅ | ❌ | ❌ | ❌ |
| No API key needed | ✅ | ✅ | N/A | N/A | N/A |
| Single binary | ✅ (Go) | ❌ (Python) | N/A | N/A | N/A |

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

trvl also works as a standalone CLI tool with 14 commands:

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

### Explore Destinations

```bash
trvl explore HEL                                   # Cheapest destinations from Helsinki
trvl explore JFK --from 2026-07-01 --to 2026-07-14 # With dates
```

### Price Grid

```bash
trvl grid HEL NRT --depart-from 2026-07-01 --depart-to 2026-07-07 \
                   --return-from 2026-07-08 --return-to 2026-07-14
```

### Destination Info

```bash
trvl destination "Tokyo"                           # Weather, safety, holidays, currency
trvl destination "Barcelona" --dates 2026-07-01,2026-07-08
```

### Trip Cost

```bash
trvl trip-cost HEL BCN --depart 2026-07-01 --return 2026-07-08 --guests 2
```

### Weekend Getaway

```bash
trvl weekend HEL --month july-2026                 # Top 10 cheapest weekends
trvl weekend HEL --month july-2026 --budget 500    # Under EUR 500 total
```

### Smart Date Suggestions

```bash
trvl suggest HEL BCN --around 2026-07-15 --flex 7  # Best dates +/- 7 days
```

### Multi-City Optimizer

```bash
trvl multi-city HEL --visit BCN,ROM,PAR --dates 2026-07-01,2026-07-21
```

## How It Works

Google's travel frontend uses an internal gRPC-over-HTTP protocol called **batchexecute**. `trvl` speaks this protocol natively:

1. **Chrome TLS fingerprint** — [utls](https://github.com/refraction-networking/utls) impersonates Chrome's exact TLS ClientHello
2. **Flights** — `FlightsFrontendService/GetShoppingResults` with encoded filter arrays
3. **Hotels** — `TravelFrontendUi` embedded JSON parsing from `AF_initDataCallback` blocks
4. **Hotel prices** — `TravelFrontendUi/data/batchexecute` with rpcid `yY52ce`
5. **Explore** — `GetExploreDestinations` for destination discovery
6. **Destination info** — Parallel aggregation of 5 free APIs (Open-Meteo, REST Countries, Nager.Date, travel-advisory.info, ExchangeRate-API)
7. **Rate limiting** — 10 req/s token bucket with exponential backoff on 429/5xx

No Selenium. No Puppeteer. No browser. Just HTTP.

## At a Glance

| | |
|---|---|
| **Binary** | Single static ~15MB. Zero runtime dependencies. |
| **Data** | Real-time from Google Flights + Google Hotels + 5 free APIs |
| **Auth** | None. No API keys, no accounts, no tokens. |
| **MCP** | Full v2025-11-25 — 13 tools, 3 prompts, resources, structured content, elicitation |
| **CLI** | 14 commands with table and JSON output |
| **Skills** | 4 Claude Code skills (trvl, travel-hacks, travel-agent, travel-agent-compact) |
| **Output** | Pretty tables (default) or JSON (`--format json`) |
| **Platforms** | Linux, macOS (amd64, arm64) |
| **Code** | 81 Go files, ~21K LOC, 11 packages |
| **Tests** | 600+ test functions, race-detector clean |
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
