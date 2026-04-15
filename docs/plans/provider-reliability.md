# Provider Reliability Improvements

All items completed 2026-04-15.

## 1. Booking.com: Elicitation-based WAF flow ✅

**Root cause**: Browser escape hatch opened booking.com silently with 15s timeout. User never noticed → timeout → error_count climbed.

**Fix**: Added `ElicitConfirmFunc` context key (`providers/cookies.go`). MCP server threads its `ElicitFunc` into the context (`mcp/server.go`). `tryBrowserEscapeHatch` now prompts the user via elicitation before opening the browser, and extends the cookie-change deadline to 30s after confirmation.

**Files changed**: `internal/providers/cookies.go`, `internal/providers/runtime.go`, `mcp/server.go`

## 2. Auto-healing: provider_status in search responses ✅

**Root cause**: External provider failures were invisible to the LLM. No way to autonomously diagnose or fix.

**Fix**: Added `ProviderStatus` type to models (`ID`, `Name`, `Status`, `Results`, `Error`, `FixHint`). `Runtime.SearchHotels` now returns `[]ProviderStatus` alongside results. `providerFixHint()` pattern-matches errors to emit actionable hints (WAF block, results_path mismatch, rate limit). `HotelSearchResult.ProviderStatuses` propagates to MCP response.

**Files changed**: `internal/models/hotel.go`, `internal/providers/runtime.go`, `internal/hotels/search.go`

## 3. Merge pipeline: external results not visible ✅

**Root cause**: `filterHotels()` dropped external results with `Rating == 0` (MinRating=4.0 filter). `preferences.FilterHotels()` dropped external results with `ReviewCount == 0 && Rating == 0` and those with `ReviewCount < 20`. External providers don't return rating/review data in the same format as Google.

**Fix**: Added `models.HasExternalProviderSource()` helper. All three filter paths now skip quality-based filters for external provider results that lack rating/review data. Added pre/post-filter logging (`countBySource`, `countExternalSources`) gated on external results presence.

**Files changed**: `internal/models/merge.go`, `internal/models/merge_test.go`, `internal/hotels/search.go`, `internal/preferences/preferences.go`
