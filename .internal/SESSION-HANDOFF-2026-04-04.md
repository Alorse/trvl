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

### Ground transport providers (9 total)
| Provider | Status | Prices | Issue |
|----------|--------|--------|-------|
| DB | ✅ Working | ✅ EUR 47-74 (domestic), schedules (intl) | `services-bahn.de` endpoint, UUID correlation ID |
| Eurostar | ✅ Working | ✅ GBP 39-190 | GraphQL `cheapestFaresLists` format, `x-platform: web` header |
| NS | ✅ Working | Schedule only (no price API) | Public API with embedded key, `plannedDurationInMinutes` |
| FlixBus | ✅ Working | ✅ | No changes needed |
| RegioJet | ✅ Working | ✅ | Date filtering fixed |
| Transitous | ✅ Working | Schedule only | No changes needed |
| ÖBB | 🔨 Partial | API returns 200, parser drops routes | HAFAS mgate, 6 connections returned but time parsing wrong |
| Trainline | 🔨 Wired | 403 from Go utls | Correct API captured (`/api/journey-search/`), Datadome blocks Go TLS |
| SNCF | ❌ Broken | Endpoint moved | Calendar API returns 404, need new endpoint |

### Demo GIF
- 4-act narrative, clean screens between acts
- All EUR prices correct (currency detection working)
- Committed and pushed

## P0 — Fix in next session

### 1. ÖBB HAFAS parser (30 min)
File: `internal/ground/oebb.go`
- API returns 200 with `outConL: 6` connections and `locL: 12` locations
- `parseOebbConnections()` only produces 1 route with empty times
- The HAFAS `dep.dDate` format is `YYYYMMDD`, `dep.dTime` is `HHMMSS`
- Need to check: does `oebbCon.Dep.DDate` match the actual JSON field name?
- Debug: response is 63KB for Vienna→Prague, saved to tool-results

### 2. Trainline TLS (1h)
File: `internal/ground/trainline.go`
- The NEW API at `thetrainline.com/api/journey-search/` works from Playwright headless
- Our Go `utls` Chrome fingerprint gets 403 from Datadome
- Playwright's Chromium (v131) passes — different TLS than utls
- Options:
  a. Update utls Chrome preset to match newer version
  b. Use Playwright as a subprocess for Trainline only
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
- Demo GIF: Re-record when ground shows trains with prices

## Key files
- Ground providers: `internal/ground/{deutschebahn,eurostar,ns,oebb,trainline,flixbus,regiojet,transitous,sncf}.go`
- Shared Chrome TLS: `internal/batchexec/client.go` → `ChromeHTTPClient()`
- Cookie auth: `internal/cookies/browser.go`
- Currency detection: `internal/flights/calendar.go` → `DetectSourceCurrency()` (cached)
- Trip planning: `internal/trip/plan.go`, `cmd/trvl/trip.go`
- Captured API specs: `.internal/eurostar-api-captured.md`

## How to continue
```bash
cd ~/github/trvl
cat .internal/SESSION-HANDOFF-2026-04-04.md
# Fix ÖBB parser first (quickest win — API already returns data)
# Then tackle Trainline TLS (try nab proxy approach)
```
