# Contributing to trvl

## The 5-Minute Contribution

```bash
go install github.com/MikkoParkkola/trvl/cmd/trvl@latest
trvl flights HEL BCN 2026-07-01
```

Found wrong data? Prices off? Missing airline? **File an issue.** You don't need to write Go — the most valuable contributions are bug reports with real search examples.

## Building from Source

```bash
git clone https://github.com/MikkoParkkola/trvl.git
cd trvl
make build    # binary at ./bin/trvl
make test     # run all tests
make lint     # go vet
```

## Project Structure

```
cmd/trvl/           CLI commands (cobra)
internal/
  batchexec/        Google batchexecute protocol (shared)
  flights/          Flight search + calendar + grid
  hotels/           Hotel search + prices + reviews  
  explore/          Destination discovery
  destinations/     Weather, safety, POIs, guides, events
  trip/             Cost calculator, multi-city, weekend, smart dates
  models/           Data types + output formatting
  cache/            In-memory response cache
mcp/                MCP server (13 tools, stdio + HTTP)
capabilities/       mcp-gateway YAML files
.claude/skills/     Claude Code skill
```

## Open Problems (good first issues)

- [ ] **Google Maps restaurant ratings** — proof of concept exists (`internal/destinations/maps_proof_test.go`), needs parser implementation
- [ ] **Rome2Rio integration** — ground transport from airport to hotel
- [ ] **Price tracking** — `trvl watch HEL-NRT --below 500` with local SQLite
- [ ] **Windows support** — binary compiles, needs testing
- [ ] **CLI color output** — colored tables for terminal
- [ ] **Explore timeout** — increase to 60s for the explore endpoint
- [ ] **More airport names** — expand `internal/models/airports.go` beyond 200

## Pull Request Process

1. Fork, branch, make changes
2. `make test` passes
3. `go vet ./...` clean
4. Add tests for new features
5. PR description explains what and why

## Code Style

Standard Go. No special rules beyond `go vet` and the patterns you see in the codebase.

## License

MIT. Your contributions are also MIT-licensed.
