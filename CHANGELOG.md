# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-04-04

### Added
- `trvl trip` command: one-search trip planning (flights + hotels + ground in parallel)
- Trainline provider: 7th ground transport provider (`f92d7bd`)
- Airport transfer search as ground sub-command (`f58bb49`)
- `trvl watch` daemon mode: background polling on a configurable schedule (`7d07e89`)
- `internal/cookies` package: browser cookie auth for CAPTCHA-protected providers (SNCF, Trainline) (`f529104`)
- `ResolveLocationName`: IATA code → human-readable city name in hotels and ground results
- `DetectSourceCurrency`: session-cached currency detection (single API call, reused across renders)
- IATA alias map with 34 airport codes mapped to city names for deal filtering

### Changed
- `--currency` flag now available on all 20 CLI commands (dates, explore, grid, ground, deals, weekend, suggest, multi-city — previously flights + hotels only)
- Ground transport deduplication: same provider + time + price collapsed into one row (`7e82ede`)
- Demo GIF rewritten as 4-act narrative: Discover / Plan / Book / Monitor (`85385b7`, `181eab3`)
- `DetectSourceCurrency` result cached per session — eliminates repeated API calls on calendar/grid renders

### Fixed
- Hardcoded EUR removed from entire codebase — API source currency detected and stamped at response layer (`c9b7ab0`, `c40cd02`, `acd3f8a`)
- Grid, explore, and calendar were mislabelling PLN (and other currencies) as EUR (`71c95e2`, `19f9423`, `d875abb`)
- DB trains: endpoint corrected, real prices extracted from `angebote.preise.gesamt.ab` (`b402c4c`)
- Ground date filtering: RegioJet multi-day results now filtered to requested departure date (`38aa83c`)
- Ground train-type recognition: RegioJet vehicleTypes mapping corrected (trains no longer classified as buses)
- Deal city-name filtering: substring + IATA alias match (e.g. "Paris" matches CDG/ORY deals) (`38aa83c`)
- UTF-8 deal title truncation: byte-slice cut replaced with rune-safe truncation

## [0.3.0] - 2026-04-03

### Added
- Ground transport: FlixBus, RegioJet, Eurostar/Snap, Deutsche Bahn, SNCF, Transitous
- Price tracking: `trvl watch` with threshold alerts and history
- Hotel amenity extraction from Google Hotels search data (18 codes + description)
- Hotel detail page amenity enrichment (opt-in, fetches full amenity lists per hotel)
- Hotel amenity filtering (pool, wifi, breakfast, etc.)
- Hotel filters: price range, rating, distance from center, sort by stars/distance
- Restaurant search via Google Maps (MCP tool)
- MCP 2025-11-25 full compliance: ping, completion/complete, logging/setLevel
- Rate limiting on all API clients
- Watch MCP resources: trvl://watches, trvl://watch/{id}
- Travel deals aggregation from 4 RSS feeds (Secret Flying, Fly4Free, Holiday Pirates, The Points Guy)
- Deal alerts shown inline in flight search results
- Multi-airport search: `trvl flights AMS,EIN HEL,TLL` searches all combos in parallel
- Route watches: monitor prices without specific dates (scans next 60 days)
- Smart price advice: error fare detection (30%+ drops), trend warnings
- CLI eye-candy: box-drawing banners, summaries, booking hints
- Display-width-aware table alignment (ANSI colors + emojis)
- CODE_OF_CONDUCT.md (Contributor Covenant 2.1)

### Changed
- Eurostar searches Snap deals first (up to 50% off), falls back to regular fares
- Improved test coverage across all packages (trip 47%→84%, watch 56%→84%, batchexec 66%→74%)
- README restructured: MCP-first, CLI secondary
- 16 MCP tools (was 13), 20 CLI commands (was 14)

### Fixed
- Zero-price routes filtered from ground transport results
- RegioJet currency parameter now passed correctly
- FlixBus city names populated in leg data
- HTTP server timeouts added (DoS prevention)
- Table alignment with ANSI color codes and emoji characters

## [0.2.0] - 2026-04-02

### Added
- **Explore destinations** — discover cheapest flights from any airport (`trvl explore HEL`)
- **CalendarGraph** — visual price grid across departure and return date ranges (`trvl grid`)
- **Destination intelligence** — weather, safety, holidays, currency, and country info from 6 free APIs (`destination_info` tool)
- **Trip cost calculator** — estimate total cost including flights and hotel (`calculate_trip_cost` tool)
- **Multi-city optimizer** — find cheapest routing order for up to 6 cities (`optimize_multi_city` tool)
- **Weekend getaway finder** — cheapest weekend destinations ranked by total cost (`weekend_getaway` tool)
- **Smart date suggestions** — analyze prices around a target date with savings insights (`suggest_dates` tool)
- **Hotel reviews** — guest review summaries and scores (`hotel_reviews` tool)
- **Nearby places** — points of interest from OpenStreetMap (`nearby_places` tool)
- **Travel guide** — local tips and practical info (`travel_guide` tool)
- **Local events** — upcoming events at destination (`local_events` tool)
- MCP structured content with content annotations (`audience`, `priority`)
- MCP elicitation for interactive parameter collection
- MCP output schemas with full JSON Schema validation for all tools
- MCP prompts: `plan-trip`, `find-cheapest-dates`, `compare-hotels`
- MCP resources: airport codes, flight/hotel usage guides, session summary
- Progressive disclosure with follow-up suggestions in every response
- Travel profile support for personalized recommendations
- 4 Claude Code skills: trvl, travel-hacks, travel-agent, travel-agent-compact
- Booking links to Google Flights and Google Hotels in results
- Docker support (`docker run ghcr.io/mikkoparkkola/trvl`)

### Changed
- Expanded from 4 to 13 MCP tools
- Upgraded MCP protocol to v2025-11-25

## [0.1.0] - 2026-03-15

### Added
- **Flight search** — real-time Google Flights data via batchexecute protocol (`search_flights` tool)
- **Date search** — cheapest flight prices across a date range (`search_dates` tool)
- **Hotel search** — Google Hotels with ratings, prices, and amenities (`search_hotels` tool)
- **Hotel prices** — compare prices across booking providers (`hotel_prices` tool)
- Chrome TLS fingerprint via utls for reliable access
- MCP server with stdio transport (4 tools)
- CLI with table and JSON output formats
- Rate limiting with token bucket and exponential backoff
- Single static binary, zero runtime dependencies
- MIT license

[0.4.0]: https://github.com/MikkoParkkola/trvl/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/MikkoParkkola/trvl/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/MikkoParkkola/trvl/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/MikkoParkkola/trvl/releases/tag/v0.1.0
