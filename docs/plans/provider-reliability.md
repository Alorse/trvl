# Provider Reliability Improvements

## 1. Booking.com: Elicitation-based WAF flow

**Problem**: Browser escape hatch opens booking.com silently with 15s timeout. User doesn't notice → timeout → error_count climbs.

**Fix**: In `runPreflight`, when Tier 4 triggers in interactive MCP context, use elicitation instead of silent browser open:
1. Pass elicit function through search context (new context key)
2. Before opening browser, elicit: "Booking.com needs a browser visit. Please open booking.com, search for any hotel, then click Done."
3. After user confirms, read cookies via kooky
4. Cookie persistence (`cookie_cache.go`) caches session for 24h

**Files**: `internal/providers/runtime.go`, `internal/hotels/search.go`, `mcp/tools_hotels.go`

## 2. Auto-healing: provider_status in search responses

**Problem**: External provider failures are silent. LLM can't fix what it can't see.

**Fix**: Return `provider_status` in search_hotels structured output:
```json
{"id":"booking","status":"error","error":"WAF 202","fix":"Call test_provider id=booking"}
```
LLM can then autonomously call test_provider → apply auto-suggest → configure_provider → retry.

**Files**: `internal/providers/runtime.go` (return per-provider status), `mcp/tools_hotels.go` (include in response)

## 3. Merge pipeline: external results not visible

**Problem**: Hostelworld "Iona Inn" at EUR 180 returns from runtime (last_success proves it) but doesn't appear in search output. Google's cheapest-6 fill the result set.

**Fix**: Ensure external-only results always surface. Either raise the merge budget or tag external results as priority-include.

**Files**: `internal/hotels/search.go`, `internal/models/merge.go`
