# Session Handoff — 2026-04-04

## What was done

### New features
- `trvl trip` — one-search trip planning (flights + hotels + cost summary)
- `--currency` on all 22 CLI commands, no hardcoded currencies anywhere
- Currency detection via `DetectSourceCurrency` (cached globally per session)
- `ResolveLocationName` for IATA→city name in hotel/ground searches
- MCP `plan_trip` tool (18th MCP tool)
- Deal city-name filtering with IATA alias map (34 airports)
- Browser cookie auth for CAPTCHA providers (`internal/cookies`)
- Chrome TLS fingerprint (`ChromeHTTPClient`) shared across all ground providers

### Ground transport providers (10 total)
| Provider | Status | Prices | Notes |
|----------|--------|--------|-------|
| DB | ✅ Working | ✅ EUR 47-74 (domestic), schedules (intl) | `services-bahn.de` endpoint, UUID correlation ID |
| Eurostar | ✅ Working | ✅ GBP 39-190 | GraphQL `cheapestFaresLists` format, `x-platform: web` header |
| NS | ✅ Working | EUR 5+ (public API) | Dutch railways, embedded subscription key |
| VR | ✅ Working | ✅ EUR 14+ (fixed fares table) | Digitransit GraphQL API; Matka.fi fares |
| ÖBB | ✅ Wired | ✅ EUR 38+ (via browser automation) | Playwright scraper; HAFAS direct returns 200 but parser partial |
| FlixBus | ✅ Working | ✅ | No changes needed |
| RegioJet | ✅ Working | ✅ | Date filtering fixed |
| Transitous | ✅ Working | Schedule only | No changes needed |
| Trainline | 🔨 Wired | 403 from Go utls | Correct API captured (`/api/journey-search/`), Datadome blocks Go TLS |
| SNCF | ❌ Broken | Endpoint moved | Calendar API returns 404, need new endpoint |

### Demo GIF
- 4-act narrative, clean screens between acts
- All EUR prices correct (currency detection working)
- Committed and pushed

### Documentation updated
- README.md: all provider counts updated to 10, VR and ÖBB added to provider list, comparison table, How It Works, At a Glance, CLI examples
- CHANGELOG.md: VR, ÖBB, NS additions documented

## P0 — Fix in next session

### 1. ÖBB HAFAS parser (30 min)
File: `internal/ground/oebb.go`
- API returns 200 with `outConL: 6` connections and `locL: 12` locations
- `parseOebbConnections()` only produces 1 route with empty times
- The HAFAS `dep.dDate` format is `YYYYMMDD`, `dep.dTime` is `HHMMSS`
- Need to check: does `oebbCon.Dep.DDate` match the actual JSON field name?
- Debug: response is 63KB for Vienna→Prague, saved to tool-results
- Browser scraper (`browser_scraper.go`) is the current working path for ÖBB prices

### 2. Trainline TLS (1h)
File: `internal/ground/trainline.go`
- The NEW API at `thetrainline.com/api/journey-search/` works from Playwright headless
- Our Go `utls` Chrome fingerprint gets 403 from Datadome
- Playwright's Chromium (v131) passes — different TLS than utls
- Options:
  a. Update utls Chrome preset to match newer version
  b. Use Playwright as a subprocess for Trainline only (same as ÖBB browser scraper)
  c. Proxy through `nab` which has `rquest` (BoringSSL, Chrome 146 fingerprint)
  d. Shell out to `nab fetch` with `--raw` flag for the POST
- The request format is correct (captured and verified):
  ```json
  POST /api/journey-search/
  Headers: x-version: 4.46.32109, Content-Type: application/json
  Body: {passengers, transitDefinitions[{origin: "urn:trainline:generic:loc:8267", ...}], transportModes: ["mixed"]}
  ```

### 3. DB cross-border prices
- `angebote.preise.gesamt.ab` only populated for domestic DE routes
- Try `tagesbestpreis` endpoint for cross-border pricing (different request body needed)

## P1 — Nice to have

- SNCF: Need to capture new API from sncf-connect.com (SPA, heavy JS)
- ÖBB Nightjet: The ÖBB HAFAS response includes Nightjet connections — just need parser fix
- NS prices: NS uses fixed fare system, may need `catalogue-api/v1/fixed/price` endpoint
- Demo GIF: Re-record when ground shows trains with prices for ÖBB + VR routes

## Key files
- Ground providers: `internal/ground/{deutschebahn,eurostar,ns,oebb,trainline,flixbus,regiojet,transitous,sncf,digitransit}.go`
- Browser scraper: `internal/ground/browser_scraper.go` + `internal/ground/scraper.py`
- Shared Chrome TLS: `internal/batchexec/client.go` → `ChromeHTTPClient()`
- Cookie auth: `internal/cookies/browser.go`
- Currency detection: `internal/flights/calendar.go` → `DetectSourceCurrency()` (cached)
- Trip planning: `internal/trip/plan.go`, `cmd/trvl/trip.go`
- Captured API specs: `.internal/eurostar-api-captured.md`

## Provider summary (for README/marketing)
- Eurostar: London↔Paris/Brussels/Amsterdam, GBP 39+
- DB: German ICE + cross-border, EUR 37+
- ÖBB: Austrian Railjet, EUR 38+ (via browser automation)
- NS: Dutch railways, EUR 5+
- VR: Finnish railways, EUR 14+ (via Digitransit API)
- FlixBus: Pan-European buses, EUR 9+
- RegioJet: Central European buses/trains, EUR 11+
- Transitous: Pan-European schedules (GTFS)
- Trainline: Wired (Datadome)
- SNCF: Wired (Datadome)

## How to continue
```bash
cd ~/github/trvl
cat .internal/SESSION-HANDOFF-2026-04-04.md
# Fix ÖBB HAFAS parser (oebb.go) — API already returns 6 connections, just parse wrong
# Then tackle Trainline TLS (try browser_scraper.go approach, same as ÖBB)
```
