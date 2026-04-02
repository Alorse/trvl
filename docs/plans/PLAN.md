# trvl — Work Plan

**Status**: DRAFT
**Date**: 2026-04-02
**Design Doc**: [DESIGN.md](../DESIGN.md)
**Tracks**: AC-1 through AC-8

---

## DoR Checklist

| Gate | Status | Evidence |
|------|--------|----------|
| Testable AC defined | PASS | DESIGN.md §8: 30 testable ACs across 8 groups |
| Tech selection rationale | PASS | DESIGN.md §5: Go vs Python vs Rust comparison |
| Fail-fast criteria | PASS | DESIGN.md §10: 4 kill criteria, validation order defined |
| Risk analysis | PASS | DESIGN.md §9: 6 risks with L×I scoring and mitigations |
| Dependencies identified | PASS | DESIGN.md §11: utls, cobra, no CGO |
| Legal review | PASS | DESIGN.md §12: MIT, GDPR N/A, same approach as fli |
| Architecture documented | PASS | DESIGN.md §4: package structure, protocol, data models |
| Scope bounded | PASS | DESIGN.md §3: 9 goals, 6 non-goals |
| Attribution | PASS | DESIGN.md §0: fli by Punit Arani credited |

---

## Phase 0: Fail-Fast Validation (Priority: BLOCKING)

**Goal**: Validate that the core approach works before investing in full implementation. Maps to FF-1 through FF-4.

### Task 0.1: utls Chrome Impersonation Proof
- **AC**: FF-1
- **Action**: Write `internal/batchexec/client.go` with utls `HelloChrome_Auto`. Make a single GET to `google.com` and verify the TLS handshake succeeds without 403.
- **Verify**: `go test ./internal/batchexec/ -run TestChromeHandshake` passes
- **Kill if**: Google returns 403 or connection reset

### Task 0.2: Flight Search Proof
- **AC**: FF-1, AC-2.1
- **Action**: Port fli's flight search payload encoding to Go. Make a single search (HEL→NRT, 2026-05-15). Parse the response.
- **Verify**: Returns ≥1 flight result with non-zero price
- **Kill if**: Empty response or unparseable format
- **Depends on**: Task 0.1

### Task 0.3: Hotel Search Proof
- **AC**: FF-2, AC-4.1
- **Action**: Encode hotel search payload with rpcid `AtySUc` for "Helsinki, 2026-05-15 to 2026-05-18". POST to `TravelFrontendUi/data/batchexecute`. Parse response.
- **Verify**: Returns ≥1 hotel with name and price
- **Kill if**: rpcid returns error, empty, or format is completely undocumented
- **Depends on**: Task 0.1

### Task 0.4: Hotel Price Proof
- **AC**: FF-2, AC-5.1
- **Action**: Use hotel_id from Task 0.3 result. Encode price lookup with rpcid `yY52ce`. Parse provider prices.
- **Verify**: Returns ≥1 provider with price
- **Kill if**: rpcid fails or returns no providers
- **Depends on**: Task 0.3

**Phase 0 exit gate**: All 4 tasks pass. If any kill condition triggers, stop and reassess.

---

## Phase 1: Core Protocol Layer

**Goal**: Robust, tested batchexecute client. Maps to AC-1.

### Task 1.1: batchexecute Client
- **AC**: AC-1.1, AC-1.5
- **File**: `internal/batchexec/client.go`
- **Action**: Production HTTP client with utls, proper headers, timeout, context cancellation
- **Test**: Unit test with mock server, integration test against live endpoint

### Task 1.2: Request Encoder
- **AC**: AC-1.1
- **File**: `internal/batchexec/encode.go`
- **Action**: `f.req` payload encoder — takes rpcid + args, returns URL-encoded form body
- **Test**: Golden file tests comparing encoded output to known-good payloads from fli

### Task 1.3: Response Parser
- **AC**: AC-1.4, AC-1.5
- **File**: `internal/batchexec/decode.go`
- **Action**: Strip `)]}'`, parse length-prefixed lines, extract rpcid responses
- **Test**: Golden file tests with captured response bodies

### Task 1.4: Rate Limiter
- **AC**: AC-1.2, AC-1.3
- **File**: `internal/batchexec/ratelimit.go`
- **Action**: Token bucket (10/s), exponential backoff retry (3 attempts), 429/5xx detection
- **Test**: Unit test verifying rate enforcement and retry behavior

---

## Phase 2: Flight Search

**Goal**: Feature parity with fli for flight search. Maps to AC-2, AC-3.

### Task 2.1: Flight Search Payload Encoding
- **AC**: AC-2.1, AC-2.3, AC-2.4, AC-2.5
- **File**: `internal/flights/encode.go`
- **Action**: Port fli's `FlightSearchFilters.encode()` to Go. Support cabin class, stops, sort, airlines.
- **Test**: Golden file comparison with fli's encoded payloads
- **Depends on**: Phase 1

### Task 2.2: Flight Search Response Parsing
- **AC**: AC-2.1, AC-2.6
- **File**: `internal/flights/parse.go`
- **Action**: Parse flight results — extract price, currency, duration, stops, legs with airline/times
- **Test**: Golden file tests with captured flight responses

### Task 2.3: Flight Search Integration
- **AC**: AC-2.1, AC-2.2
- **File**: `internal/flights/search.go`
- **Action**: `SearchFlights(ctx, filters) ([]FlightResult, error)` — one-way and round-trip
- **Test**: Integration test against live API (tagged `//go:build integration`)

### Task 2.4: Date Search
- **AC**: AC-3.1, AC-3.2, AC-3.3
- **File**: `internal/flights/dates.go`
- **Action**: Date range search — cheapest dates for a route
- **Test**: Integration test returning ≥1 date-price pair
- **Depends on**: Task 2.1

---

## Phase 3: Hotel Search

**Goal**: Working hotel search and price lookup. Maps to AC-4, AC-5. This is the novel contribution.

### Task 3.1: Hotel Search Payload Encoding
- **AC**: AC-4.1, AC-4.3, AC-4.4
- **File**: `internal/hotels/encode.go`
- **Action**: Encode hotel search payload for rpcid `AtySUc`. Parameters: location, check-in, check-out, guests, stars.
- **Approach**: Capture payloads from Chrome DevTools on google.com/travel/hotels, reverse-engineer the array structure
- **Test**: Golden file tests

### Task 3.2: Hotel Search Response Parsing
- **AC**: AC-4.1, AC-4.2
- **File**: `internal/hotels/parse.go`
- **Action**: Parse hotel results — name, hotel_id, rating, stars, price, address, coordinates, amenities
- **Reference**: SerpAPI's Google Hotels JSON output as field mapping guide
- **Test**: Golden file tests with captured responses

### Task 3.3: Hotel Search Integration
- **AC**: AC-4.1, AC-4.5
- **File**: `internal/hotels/search.go`
- **Action**: `SearchHotels(ctx, filters) ([]HotelResult, error)`
- **Test**: Integration test for "Helsinki" returning ≥1 hotel

### Task 3.4: Hotel Price Lookup
- **AC**: AC-5.1, AC-5.2, AC-5.3
- **File**: `internal/hotels/prices.go`
- **Action**: Price lookup by hotel_id using rpcid `yY52ce`. Parse provider list.
- **Test**: Integration test returning ≥1 provider price
- **Depends on**: Task 3.3 (needs hotel_id)

---

## Phase 4: CLI

**Goal**: Polished CLI with table and JSON output. Maps to AC-6.

### Task 4.1: CLI Skeleton
- **AC**: AC-6.4
- **File**: `cmd/trvl/main.go` + subcommand files
- **Action**: cobra commands: `flights`, `dates`, `hotels`, `prices`, `mcp`
- **Test**: `trvl --help` shows all commands

### Task 4.2: Flight Commands
- **AC**: AC-6.1, AC-6.2, AC-6.3
- **Action**: Wire `trvl flights` and `trvl dates` to flight search, table + JSON output
- **Test**: `trvl flights HEL NRT 2026-05-15 --format json | jq .` validates JSON
- **Depends on**: Phase 2

### Task 4.3: Hotel Commands
- **AC**: AC-6.1, AC-6.2, AC-6.3
- **Action**: Wire `trvl hotels` and `trvl prices` to hotel search, table + JSON output
- **Test**: `trvl hotels Helsinki --checkin 2026-05-15 --checkout 2026-05-18 --format json | jq .`
- **Depends on**: Phase 3

### Task 4.4: Output Formatting
- **AC**: AC-6.1, AC-6.5
- **File**: `internal/models/output.go`
- **Action**: Table formatter for terminal, proper exit codes, stderr for errors
- **Test**: Verify exit codes and stderr/stdout separation

---

## Phase 5: MCP Server

**Goal**: AI agent integration. Maps to AC-7.

### Task 5.1: MCP stdio Server
- **AC**: AC-7.1, AC-7.3, AC-7.4, AC-7.5
- **File**: `mcp/server.go`, `mcp/tools.go`
- **Action**: JSON-RPC over stdin/stdout. Handle `initialize`, `tools/list`, `tools/call`.
- **Test**: Write a test harness that sends JSON-RPC messages to stdin and validates responses

### Task 5.2: MCP HTTP Server
- **AC**: AC-7.2
- **File**: `mcp/server.go`
- **Action**: HTTP transport on configurable port
- **Test**: `curl -X POST http://localhost:8000/mcp` with initialize request
- **Depends on**: Task 5.1

### Task 5.3: Gateway Capability YAMLs
- **AC**: G9
- **Files**: `capabilities/trvl_search_flights.yaml`, `trvl_search_dates.yaml`, `trvl_search_hotels.yaml`, `trvl_hotel_prices.yaml`
- **Action**: Create mcp-gateway capability definitions using `service: cli` provider
- **Depends on**: Phase 4

---

## Phase 6: Quality & Release

**Goal**: Production-ready release. Maps to AC-8.

### Task 6.1: Linting & Vet
- **AC**: AC-8.1, AC-8.3
- **Action**: `go vet ./...` and `golangci-lint run` pass clean

### Task 6.2: Test Coverage
- **AC**: AC-8.2
- **Action**: `go test -coverprofile=coverage.out ./...` — ≥80% on batchexec/ and models/

### Task 6.3: Cross-Compilation
- **AC**: AC-8.4, AC-8.5
- **File**: `.goreleaser.yaml`, `Makefile`
- **Action**: Build for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64. Verify binary < 15MB.

### Task 6.4: GitHub Release
- **Action**: Create GitHub repo, push, tag v0.1.0, goreleaser creates release assets
- **Depends on**: All phases

---

## Dependency Graph

```
Phase 0 (fail-fast)
  └─> Phase 1 (batchexec)
       ├─> Phase 2 (flights)
       │    └─> Task 4.2 (flight CLI)
       └─> Phase 3 (hotels)
            └─> Task 4.3 (hotel CLI)

Phase 4 (CLI) ──> Phase 5 (MCP) ──> Phase 6 (release)
     └─> Task 4.1 (skeleton, parallel with Phase 2/3)
```

## Parallelism Opportunities

- **Phase 2 + Phase 3**: Independent once Phase 1 is done — can run in parallel
- **Task 4.1** (CLI skeleton): Can start during Phase 1
- **Task 5.1** (MCP): Can start once any one search type works
- **Task 6.1-6.2** (quality): Continuous, not blocked

## Estimated Task Count

| Phase | Tasks | Blocking |
|-------|-------|----------|
| 0: Fail-fast | 4 | Yes — all must pass |
| 1: Protocol | 4 | Yes — foundation |
| 2: Flights | 4 | No — parallel with Phase 3 |
| 3: Hotels | 4 | No — parallel with Phase 2 |
| 4: CLI | 4 | Partial — skeleton early |
| 5: MCP | 3 | After Phase 4 |
| 6: Release | 4 | Final gate |
| **Total** | **27 tasks** | |

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Flight search accuracy | ≥90% match with fli results | Compare 10 searches side-by-side |
| Hotel search coverage | Top 20 cities return results | Automated test suite |
| Response time | <3s for search, <1s for price lookup | CLI timing |
| Binary size | <15MB | `ls -la` release binary |
| Test coverage | ≥80% on core packages | `go test -cover` |
