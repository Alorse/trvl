[![Go Report Card](https://goreportcard.com/badge/github.com/MikkoParkkola/trvl)](https://goreportcard.com/report/github.com/MikkoParkkola/trvl)
[![CI](https://github.com/MikkoParkkola/trvl/actions/workflows/ci.yaml/badge.svg)](https://github.com/MikkoParkkola/trvl/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/github/v/release/MikkoParkkola/trvl)](https://github.com/MikkoParkkola/trvl/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/MikkoParkkola/trvl.svg)](https://pkg.go.dev/github.com/MikkoParkkola/trvl)
[![MCP](https://img.shields.io/badge/MCP-2025--11--25-blue)](https://modelcontextprotocol.io)

# trvl — The Travel MCP Server

![trvl demo](demo.gif)

> **18 travel tools for your AI assistant — flights, hotels, trains, buses, price alerts, destination intel. Free. One binary.**
>
> Also works as a standalone CLI with 22 commands.

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

trvl is an [MCP server](https://modelcontextprotocol.io/) + CLI that gives Claude, Cursor, Windsurf, and any MCP-compatible AI assistant direct access to Google Flights, Google Hotels, and European ground transport data. It searches, optimizes, and applies travel hacks automatically — no personal API keys required, no monthly fees, no scraping.

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
for s in trvl; do
  curl -fsSL "https://raw.githubusercontent.com/MikkoParkkola/trvl/main/.claude/skills/$s.md" -o "$HOME/.claude/skills/$s.md"
done
```

Now Claude knows about trvl in every project — just say "search flights" or "plan a trip".

### 4. Ask your AI to search

That's it. Your AI assistant now has 18 travel tools available. Just ask naturally:

- *"Search flights from JFK to Tokyo on July 1st, business class"*
- *"Find hotels in Paris for July 1-5, at least 4 stars"*
- *"What's the cheapest day to fly Helsinki to Barcelona in August?"*
- *"Where can I fly cheaply from Helsinki this weekend?"*
- *"How much would a week in Barcelona cost — flights and hotel?"*
- *"When should I fly to London? Check dates around July 15th"*
- *"Plan a trip: Helsinki -> Barcelona -> Rome -> Paris, cheapest routing"*
- *"Search buses from Prague to Krakow on May 3rd"*
- *"Compare train and bus prices Prague to Vienna"*
- *"Search flights from Amsterdam, Eindhoven, or Antwerp to Helsinki or Tallinn"*
- *"Show me travel deals from Helsinki under €400"*
- *"Alert me when flights to Tokyo drop below €500"*

## MCP Tools

| Tool | What it does | Example |
|------|-------------|---------|
| **search_flights** | Search flights on a specific date | HEL -> NRT, 2026-06-15, business class, nonstop |
| **search_dates** | Find cheapest day to fly across a date range | HEL -> BCN, June-August 2026 |
| **search_hotels** | Search hotels in any city | Tokyo, June 15-18, 4+ stars |
| **hotel_prices** | Compare hotel prices from Google (aggregated from multiple providers) |
| **hotel_reviews** | Get reviews for a specific hotel | Top reviews, sorted by rating or recency |
| **destination_info** | Travel intelligence for any city | Tokyo: weather, safety, holidays, currency |
| **calculate_trip_cost** | Estimate total trip cost (flights + hotel) | HEL -> BCN, Jul 1-8, 2 guests |
| **weekend_getaway** | Find cheap weekend destinations | From HEL in July, budget EUR 500 |
| **suggest_dates** | Smart date suggestions around a target date | HEL -> BCN around Jul 15, +/- 7 days |
| **optimize_multi_city** | Find cheapest routing for multi-city trips | HEL -> BCN, ROM, PAR -> HEL |
| **nearby_places** | Find points of interest near a location | Restaurants, attractions near hotel |
| **travel_guide** | Wikivoyage travel guide for a city | Neighbourhoods, getting around, safety |
| **local_events** | Find events during your trip dates | Concerts, festivals, exhibitions |
| **search_ground** | Search buses and trains (11 providers) | Prague -> Vienna, May 3rd, trains only |
| **search_airport_transfers** | Search airport-to-hotel or airport-to-city ground transport, plus taxi estimates | CDG -> Hotel Lutetia Paris, after 14:30 |
| **search_restaurants** | Find restaurants near a location (Google Maps) | Barcelona, italian cuisine |
| **search_deals** | Travel deals from 4 RSS feeds (error fares, flash sales) | Deals from HEL under EUR 400 |
| **plan_trip** | Plan a complete trip — flights + hotel in one parallel search | AMS→PRG, Jun 15–18, EUR |

### MCP Protocol Features (v2025-11-25)

| Feature | Details |
|---------|---------|
| **Structured content** | Typed JSON (`structuredContent`) alongside human-readable summaries |
| **Content annotations** | `audience: ["user"]` for summaries, `audience: ["assistant"]` for data |
| **Output schemas** | Full JSON Schema validation for all 18 tool responses |
| **Prompts** | `plan-trip`, `find-cheapest-dates`, `compare-hotels`, `where-should-i-go` |
| **Resources** | Airport codes (50 major hubs), flight/hotel usage guides |
| **Elicitation** | Interactive parameter collection when dates are missing |
| **Progressive disclosure** | Suggestions for follow-up searches in every response |
| **Booking links** | Direct Google Flights/Hotels links in results |

## Ground Transport Providers

trvl searches 11 ground transport providers in parallel, covering most of Europe:

| Provider | Protocol | Coverage | Starting price | Auth |
|----------|----------|----------|----------------|------|
| **Eurostar** | GraphQL | London ↔ Paris/Brussels/Amsterdam/Cologne | GBP 39+ | Browser cookies (Datadome) |
| **Deutsche Bahn** | REST (Vendo) | All European rail connections | EUR 37+ | None |
| **ÖBB** | Browser scraper | Austrian Railjet + cross-border (AT/DE/HU/IT) | EUR 38+ | Playwright session |
| **VR (via Digitransit)** | GraphQL | Finnish rail network | EUR 14+ | Public API key (embedded) |
| **NS** | REST | Dutch rail network | EUR 5+ | Public subscription key (embedded) |
| **Renfe** | Browser scraper | Spanish AVE high-speed + regional | EUR 36+ | Playwright session |
| **SNCF** | REST (BFF) | French TGV, TER, Intercity | Varies | Browser cookies (Datadome), curl fallback |
| **Trainline** | REST | Aggregated: SNCF, DB, Eurostar, Trenitalia, … | Varies | Browser cookies (Datadome) |
| **FlixBus** | REST | Pan-European buses (40+ countries) | EUR 5+ | None |
| **RegioJet** | REST | CZ/SK/AT/HU/DE/PL buses + trains | EUR 5+ | None |
| **Transitous** | MOTIS2 REST | Pan-European transit (schedule-based fallback) | — | None |

Two providers (NS, Digitransit/VR) use public API keys that are embedded in the binary — no signup or personal key is required from the user.

## How trvl Compares

| Feature | trvl | fli | Google Flights | Skyscanner | Kiwi |
|---------|------|-----|---------------|------------|------|
| Flight search | ✅ | ✅ | ✅ | ✅ | ✅ |
| Bus/train search | ✅ (11 providers: FlixBus, RegioJet, Eurostar, DB, ÖBB, NS, VR, SNCF, Trainline, Transitous, Renfe) | ❌ | ❌ | ❌ | ❌ |
| Price tracking | ✅ (watches with alerts) | ❌ | ❌ | ❌ | ❌ |
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

trvl also works as a standalone CLI tool with 22 commands:

All search commands accept `--currency <CODE>` (e.g. `--currency EUR`) to convert displayed prices. trvl detects the actual API currency and converts at the display layer — no hardcoded currencies.

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
trvl flights AMS,EIN,ANR HEL,TKU,TLL 2026-06-15     # Multi-airport search
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
trvl explore HEL                                        # Cheapest destinations from Helsinki
trvl explore JFK --from 2026-07-01 --to 2026-07-14      # With dates
trvl explore AMS --currency EUR                         # Display prices in EUR
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

### Plan a Trip

Search flights and hotels in one command. Runs both searches in parallel and shows a cost summary.

```bash
trvl trip AMS PRG --depart 2026-06-15 --return 2026-06-18 --currency EUR
trvl trip JFK LHR --depart 2026-08-01 --return 2026-08-10 --guests 2
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

### Buses & Trains

Searches 11 providers in parallel: FlixBus (buses, pan-European), RegioJet (buses+trains, CZ/SK/AT/HU/DE/PL), Eurostar/Snap (trains, London↔Paris/Brussels/Amsterdam/Cologne), Deutsche Bahn (trains, all European rail), ÖBB (trains, Austrian Railjet + cross-border, via browser automation), NS (Dutch railways), VR (Finnish railways, via Digitransit API), SNCF (trains, French TGV/TER), Trainline (aggregated rail across major European operators), Transitous.org (transit routing, pan-European), and Renfe (trains, Spanish AVE high-speed, EUR 36+). Airport transfers also add taxi fare estimates for door-to-door comparisons.

Trainline, ÖBB, and some other providers require browser cookie auth to bypass CAPTCHA. trvl loads cookies from your browser automatically via `internal/cookies`.

```bash
trvl ground Prague Vienna 2026-07-01                  # All 11 providers
trvl ground London Paris 2026-07-01                   # Eurostar + FlixBus + DB
trvl bus Prague Krakow 2026-07-01                     # Same command, bus alias
trvl train Prague Vienna 2026-07-01 --type train      # Trains only
trvl ground Prague Vienna 2026-07-01 --provider regiojet  # RegioJet only
trvl ground Vienna Salzburg 2026-07-01 --provider oebb    # ÖBB Railjet (EUR 38+)
trvl ground Helsinki Tampere 2026-07-01 --provider vr     # VR Finnish Railways (EUR 14+)
trvl ground Amsterdam Utrecht 2026-07-01 --provider ns    # NS Dutch Railways (EUR 5+)
trvl ground Paris Lyon 2026-07-01 --provider sncf         # SNCF TGV only
trvl ground Berlin Munich 2026-07-01 --provider db        # DB ICE (e.g. EUR 47.99)
trvl ground London Paris 2026-07-01 --provider trainline  # Trainline aggregated rail
trvl ground Madrid Barcelona 2026-07-01 --provider renfe   # Renfe AVE high-speed (EUR 36+)
trvl ground Prague Vienna 2026-07-01 --max-price 20       # Under EUR 20
trvl airport-transfer CDG "Hotel Lutetia Paris" 2026-07-01
trvl airport-transfer LHR "Paddington Station" 2026-07-01 --arrival-after 14:30
trvl airport-transfer CDG "Hotel Lutetia Paris" 2026-07-01 --provider taxi
```

### Price Watch

Track flight and hotel prices over time. Get alerts when prices drop below a threshold.

```bash
trvl watch add HEL BCN --depart 2026-07-01 --return 2026-07-08 --below 200
trvl watch list                                       # Show all active watches
trvl watch check                                      # Check current prices
trvl watch daemon --every 6h                          # Keep checking on a schedule
trvl watch history <id>                               # Price history for a watch
trvl watch remove <id>                                # Remove a watch
```

### Travel Deals

Aggregates error fares, flash sales, and deals from 4 RSS feeds (Secret Flying, Fly4Free, Holiday Pirates, The Points Guy). Deals also appear automatically in flight search results when a matching deal is found.

```bash
trvl deals                                            # All recent deals
trvl deals --from HEL,AMS --max-price 400             # From my airports, under €400
trvl deals --from Helsinki,Amsterdam,Prague            # City names also accepted
trvl deals --type error_fare                           # Error fares only
```

## How It Works

Google's travel frontend uses an internal gRPC-over-HTTP protocol called **batchexecute**. `trvl` speaks this protocol natively:

1. **Chrome TLS fingerprint** — [utls](https://github.com/refraction-networking/utls) impersonates Chrome's exact TLS ClientHello
2. **Flights** — `FlightsFrontendService/GetShoppingResults` with encoded filter arrays
3. **Hotels** — `TravelFrontendUi` embedded JSON parsing from `AF_initDataCallback` blocks
4. **Hotel prices** — `TravelFrontendUi/data/batchexecute` with rpcid `yY52ce`
5. **Explore** — `GetExploreDestinations` for destination discovery
6. **Destination info** — Parallel aggregation of 5 free APIs (Open-Meteo, REST Countries, Nager.Date, travel-advisory.info, ExchangeRate-API)
7. **Buses** — FlixBus public API (`global.api.flixbus.com`) with city autocomplete + search
8. **Trains (RegioJet)** — RegioJet public API (`brn-ybus-pubapi.sa.cz`) with route search + pricing
9. **Trains (Eurostar)** — `site-api.eurostar.com/gateway` GraphQL for London↔Paris/Brussels/Amsterdam/Cologne
10. **Trains (Deutsche Bahn)** — DB Vendo API (`int.bahn.de/web/api`) for all European rail connections
11. **Trains (ÖBB)** — Austrian Federal Railways via Playwright browser automation; live Railjet fares (EUR 38+)
12. **Trains (NS)** — NS Dutch Railways public API (`gateway.apiportal.ns.nl`) with embedded subscription key
13. **Trains (VR)** — Finnish Railways via Digitransit GraphQL API (`api.digitransit.fi`); fixed fares from Matka.fi
14. **Trains (SNCF)** — SNCF Connect API for French TGV, TER, and Intercity routes
15. **Trains (Trainline)** — Trainline aggregated rail API across major European operators; browser cookie auth for CAPTCHA bypass
16. **Transit (Transitous)** — `routing.spicebus.org` MOTIS2 API for pan-European transit routing
17. **Trains (Renfe)** — Spanish AVE high-speed and regional rail via Playwright browser scraper; fares EUR 36+
18. **Rate limiting** — per-provider token buckets (10 req/s FlixBus/RegioJet; 1 req/2s DB; 1 req/6s SNCF/Transitous; 1 req/20s Eurostar) with exponential backoff on 429/5xx

Most providers use pure HTTP — no Selenium, no Puppeteer. ÖBB, SNCF, and Renfe use an optional Playwright browser scraper for providers that require a live browser session (falls back gracefully if Playwright is not installed).

## Every Result is Bookable

Every flight and hotel result includes a `booking_url` — a direct link to Google Flights or Google Hotels where you can complete the booking:

```json
{
  "price": 113,
  "currency": "EUR",
  "airline": "Norwegian",
  "flight_number": "D8 2900",
  "booking_url": "https://www.google.com/travel/flights?q=Flights+to+BCN+from+HEL+on+2026-07-01"
}
```

The AI uses these to give you actionable recommendations: "Book here: [link]". No copying flight numbers into another search engine.

## At a Glance

| | |
|---|---|
| **Binary** | Single static ~15MB. Zero runtime dependencies. |
| **Data** | Real-time from Google Flights/Hotels/Explore/Maps + 11 ground providers (FlixBus, RegioJet, Eurostar, DB, ÖBB, NS, VR, SNCF, Trainline, Transitous, Renfe) + 5 free destination APIs |
| **Auth** | No personal API keys required. Two providers (NS, Digitransit/VR) use public keys embedded in the binary. Browser cookies loaded automatically for CAPTCHA-protected providers (Trainline, Eurostar, SNCF, ÖBB). |
| **MCP** | Full v2025-11-25 — 18 tools, 4 prompts, resources, structured content, sampling |
| **CLI** | 22 commands (+ 6 watch subcommands) with table/JSON output, color, shell completion |
| **Booking links** | Every flight and hotel result includes a direct Google booking link |
| **Travel hacks** | 30+ hacks auto-applied: nearby airports, throw-away returns, hotel splits |
| **Personal profile** | Remembers your FF status, luggage needs, favourite hotels, departure preferences |
| **Output** | Pretty tables with color (default) or JSON (`--format json`) |
| **Platforms** | Linux, macOS (amd64, arm64). Windows CI in progress. |
| **Code** | 145 Go files, ~44K LOC, 16 packages, 1100+ tests |
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
