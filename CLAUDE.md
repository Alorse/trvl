# trvl

Travel MCP server + CLI. 34 MCP tools, 34 CLI commands. Go 1.26, no frameworks.

## Architecture

```
cmd/trvl/          CLI commands (cobra-style, one file per command)
  main.go          Entrypoint
  mcp.go           MCP stdio server launcher
  flights.go       Flight search command
  hotels.go        Hotel search command
  ...
internal/          Domain packages (one per data source)
  flights/         Google Flights scraping + protobuf encoding
  hotels/          Google Hotels scraping
  ground/          Buses, trains, ferries (16 providers)
  destinations/    City intelligence (weather, safety, holidays)
  deals/           RSS deal feeds
  hacks/           Travel hack detectors (18 parallel)
  lounges/         Airport lounge data
  baggage/         Airline baggage rules
  weather/         Open-Meteo forecasts
  models/          Shared types (FlightResult, HotelResult, etc.)
  preferences/     User prefs (~/.trvl/preferences.json)
  cache/           HTTP response caching
  ...
mcp/               MCP server (tools, resources, prompts)
  server.go        Server setup + tool registration
  tools*.go        Tool handlers (one file per domain)
capabilities/      MCP capability YAML definitions
.claude/skills/    Bundled Claude skill
```

## Commands

```bash
make build                          # Build binary to bin/trvl
make test                           # go test ./...
make test-proof                     # go test -v -count=1 -race ./...
make lint                           # go vet + staticcheck
go test -short ./...                # Fast tests (skip network)
go test ./internal/flights/...      # Single package
staticcheck ./...                   # Lint (CI runs this)
go vet ./...                        # Vet (CI runs this)
```

## CI

GitHub Actions (`.github/workflows/ci.yaml`): build, vet, staticcheck, govulncheck, test with race detector, coverage threshold (50%). Runs on ubuntu + windows, Go 1.25.9.

## Key Details

- **No API keys required** for core functionality (Google Flights/Hotels scraped directly)
- **Optional API keys**: Ticketmaster, Foursquare, Geoapify, OpenTripMap (env vars)
- **User prefs**: `~/.trvl/preferences.json` (home airports, budgets, loyalty status)
- **License**: PolyForm Noncommercial 1.0.0
- **Module**: `github.com/MikkoParkkola/trvl`

## Dev Notes

- Protobuf-style encoding for Google Flights requests (no .proto files, hand-rolled)
- Flight filters use nested protobuf arrays with precise slot indexing
- Test files ending in `_probe_test.go` hit live Google endpoints (skip with `-short`)
- `internal/models/` is the shared type package -- all packages import from here
- MCP tool handlers in `mcp/tools*.go` delegate to `internal/` packages
