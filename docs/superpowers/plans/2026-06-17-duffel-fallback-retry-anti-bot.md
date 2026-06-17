# Duffel Fallback + Anti-Bot Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Google's `unexpected flight data format` (a 200-with-anti-bot-body) retryable like a 429/5xx, and fall back to the Duffel API (one-way, round-trip, and multi-city via `slices[]`) when Google still fails after retries.

**Architecture:** Three layers. (1) `batchexec` gains a body validator hook so `doWithRetry` can treat a 200-OK anti-bot response as retryable, reusing the existing backoff + rate-limiter. (2) A new `internal/flights/duffel.go` provider maps Duffel offer-request responses (`slices[].segments[]`, ISO-8601 durations) into `models.FlightResult`. (3) `searchFlightsCore` (one-way/round-trip) and `SearchMultiCity` (multi-city) call Duffel as a fallback only when Google fails.

**Tech Stack:** Go 1.26 (`GOTOOLCHAIN=go1.26.2`), stdlib `net/http` + `encoding/json`, no new third-party deps. Duffel REST API v2 (`POST /air/offer_requests`).

## Global Constraints

- Module path: `github.com/MikkoParkkola/trvl`. Run Go with `GOTOOLCHAIN=go1.26.2` prefix on raw `go` commands.
- No new third-party dependencies. Standard library only.
- No API key required for existing functionality. Duffel is gated behind Duffel API keys read at runtime from env (never hardcoded in the binary): `DUFFEL_API_KEYS` (comma/whitespace-separated, multiple keys) takes precedence, falling back to a single `DUFFEL_API_KEY`. When none are set, the Duffel fallback is a no-op (zero behavioral change).
- Multiple Duffel keys are rotated **round-robin + failover**: each search starts from the next key (atomic counter spreads load), and on a key's failure the search walks the remaining keys in order before giving up.
- All tests must pass offline (no live network). Duffel/Google responses are exercised via `httptest` servers and JSON fixtures, never live endpoints.
- Preserve existing features: `--first` (FirstResult) and `--leg` multi-city. Changes to `SearchMultiCity` are strictly **additive** (Google stays primary; Duffel only fires on Google error).
- Duffel currency is NOT controllable per request — it is the organization's billing currency. Do not attempt to pass a currency param to Duffel. Currency normalization stays in the display layer (unchanged).
- Duffel results carry no booking deep-link → `BookingURL` is left empty (`""`) on Duffel-sourced `FlightResult`s.
- Lint clean: `staticcheck ./...` and `go vet ./...` must pass.

---

## File Structure

- **Create** `internal/flights/duration.go` — `parseISO8601Duration(string) int` (ISO-8601 duration → minutes). Pure helper, no deps.
- **Create** `internal/flights/duration_test.go` — table tests for the parser.
- **Create** `internal/batchexec/blocked.go` — `IsBlockedFlightResponse([]byte) bool` (detects 200-anti-bot responses).
- **Create** `internal/batchexec/blocked_test.go` — fixture-based tests.
- **Modify** `internal/batchexec/client.go` — add `doWithRetryValidated`, `PostFormValidated`; make `doWithRetry`/`PostForm` delegate with `nil`; wire `SearchFlightsGLCurr` to pass the validator and skip caching blocked bodies.
- **Create** `internal/batchexec/client_validated_test.go` — `httptest` server returning a blocked body N times then a good body, asserting retry happens.
- **Create** `internal/flights/duffel_keys.go` — runtime key loading (`duffelKeys`) + round-robin ordering (`duffelKeyOrder`).
- **Create** `internal/flights/duffel_keys_test.go` — env parsing + rotation tests.
- **Create** `internal/flights/duffel.go` — Duffel provider: `DuffelSlice` type, `SearchDuffel` (loops keys with round-robin+failover), `searchDuffelOnce`, response types, mapping, `DuffelEnabled`, test seam.
- **Create** `internal/flights/duffel_test.go` — fixture-based mapping tests via an `httptest` server.
- **Modify** `internal/flights/search.go` — wire Duffel fallback into `searchFlightsCore` when Google fails.
- **Modify** `internal/flights/multicity.go` — wire Duffel fallback into `SearchMultiCity` when Google fails (additive).
- **Modify** `docs/` — short provider note (folded into Task 6).

---

### Task 0: Create the working branch

- [ ] **Step 1: Create and switch to a new branch off the current branch**

```bash
cd /Users/alorse/Projects/local/mcp/trvl
git checkout -b feat/flights-duffel-fallback
git status
```

Expected: `On branch feat/flights-duffel-fallback`, working tree clean (the only untracked file is `docs/superpowers/plans/`).

- [ ] **Step 2: Commit the plan**

```bash
git add docs/superpowers/plans/2026-06-17-duffel-fallback-retry-anti-bot.md
git commit -m "docs: add Duffel fallback + anti-bot retry plan

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 1: ISO-8601 duration parser

Duffel reports durations as ISO-8601 (`"P1DT4H55M"`, `"PT12H35M"`). `FlightLeg.Duration` and `FlightResult.Duration` are integer minutes. This pure helper converts them.

**Files:**
- Create: `internal/flights/duration.go`
- Test: `internal/flights/duration_test.go`

**Interfaces:**
- Produces: `func parseISO8601Duration(s string) int` — returns total minutes; returns `0` for malformed/empty input (seconds are floored away).

- [ ] **Step 1: Write the failing test**

```go
package flights

import "testing"

func TestParseISO8601Duration(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"PT2H10M", 130},
		{"PT12H35M", 755},
		{"P1DT4H55M", 24*60 + 4*60 + 55}, // 1775
		{"PT45M", 45},
		{"PT3H", 180},
		{"P2D", 2 * 24 * 60},
		{"PT0S", 0},
		{"", 0},
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseISO8601Duration(c.in); got != c.want {
			t.Errorf("parseISO8601Duration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run TestParseISO8601Duration -v`
Expected: FAIL — `undefined: parseISO8601Duration`.

- [ ] **Step 3: Write minimal implementation**

```go
package flights

import (
	"strconv"
	"strings"
)

// parseISO8601Duration converts an ISO-8601 duration (e.g. "P1DT4H55M",
// "PT12H35M") into total minutes. Seconds are floored away. Malformed or empty
// input returns 0. Duffel reports flight/segment durations in this format,
// whereas models.FlightResult uses integer minutes.
func parseISO8601Duration(s string) int {
	if !strings.HasPrefix(s, "P") {
		return 0
	}
	s = s[1:] // drop leading 'P'

	datePart, timePart := s, ""
	if i := strings.Index(s, "T"); i >= 0 {
		datePart, timePart = s[:i], s[i+1:]
	}

	total := 0
	// Date part: only days are meaningful for flight durations.
	if strings.HasSuffix(datePart, "D") {
		if d, err := strconv.Atoi(strings.TrimSuffix(datePart, "D")); err == nil {
			total += d * 24 * 60
		}
	}

	// Time part: accumulate digits, flush on H/M/S designator.
	num := ""
	for _, r := range timePart {
		if r >= '0' && r <= '9' {
			num += string(r)
			continue
		}
		n, _ := strconv.Atoi(num)
		num = ""
		switch r {
		case 'H':
			total += n * 60
		case 'M':
			total += n
		case 'S':
			// floored away at minute granularity
		}
	}
	return total
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run TestParseISO8601Duration -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/flights/duration.go internal/flights/duration_test.go
git commit -m "feat(flights): add ISO-8601 duration parser for Duffel

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Detect 200-anti-bot responses (`IsBlockedFlightResponse`)

Google returns the anti-bot `ErrorResponse` with HTTP 200. After `DecodeFlightResponse`, the inner value is NOT a `[]any` flight array (it is the error object), which is what later produces `unexpected flight data format`. A *valid but empty* result still decodes to a `[]any` — so we must distinguish "blocked" from "zero results" (zero results is a legit answer, not retryable).

**Files:**
- Create: `internal/batchexec/blocked.go`
- Test: `internal/batchexec/blocked_test.go`

**Interfaces:**
- Consumes: `DecodeFlightResponse([]byte) (any, error)` (existing, decode.go:31).
- Produces: `func IsBlockedFlightResponse(body []byte) bool` — true when the body cannot be decoded into a flight array (anti-bot / garbage / empty envelope); false when it decodes to a `[]any` (including a valid empty-results array).

- [ ] **Step 1: Write the failing test**

```go
package batchexec

import "testing"

func TestIsBlockedFlightResponse(t *testing.T) {
	// Valid envelope: outer[0][2] is a JSON string that parses to a []any.
	// Inner array "[1,2,[],[]]" → decodes to []any → NOT blocked.
	valid := []byte(`)]}'` + "\n" + `[["wrb.fr","GetShoppingResults","[1,2,[],[]]"]]`)
	if IsBlockedFlightResponse(valid) {
		t.Errorf("valid flight envelope reported as blocked")
	}

	// Anti-bot: outer[0][2] is an object, not a flight-array JSON string.
	blocked := []byte(`)]}'` + "\n" + `[["wrb.fr","GetShoppingResults",null,null,null,null,"generic"],["di",42],["af.httprm",42,"-1",0]]`)
	if !IsBlockedFlightResponse(blocked) {
		t.Errorf("anti-bot response not reported as blocked")
	}

	// Empty / garbage body → blocked (retryable).
	if !IsBlockedFlightResponse([]byte("")) {
		t.Errorf("empty body should be reported as blocked")
	}
	if !IsBlockedFlightResponse([]byte(")]}'\nnot json")) {
		t.Errorf("garbage body should be reported as blocked")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/batchexec/ -run TestIsBlockedFlightResponse -v`
Expected: FAIL — `undefined: IsBlockedFlightResponse`.

- [ ] **Step 3: Write minimal implementation**

```go
package batchexec

// IsBlockedFlightResponse reports whether a flight response body is an anti-bot
// rejection rather than real flight data. Google returns its anti-abuse
// ErrorResponse with HTTP 200, so status alone cannot detect it.
//
// A real response (even one with zero results) decodes into a []any flight
// array via DecodeFlightResponse. A blocked response decodes into the error
// object (not a []any), or fails to decode entirely. Either case is treated as
// retryable. A valid empty-results array returns false (not blocked) so genuine
// "no flights" answers are not retried.
func IsBlockedFlightResponse(body []byte) bool {
	inner, err := DecodeFlightResponse(body)
	if err != nil {
		return true
	}
	_, ok := inner.([]any)
	return !ok
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/batchexec/ -run TestIsBlockedFlightResponse -v`
Expected: PASS. (If the `valid` fixture's inner string needs adjusting to satisfy `DecodeFlightResponse`, confirm `outer[0]` has ≥3 elements and `outer[0][2]` is a JSON string that parses to a `[]any`.)

- [ ] **Step 5: Commit**

```bash
git add internal/batchexec/blocked.go internal/batchexec/blocked_test.go
git commit -m "feat(batchexec): detect 200-anti-bot flight responses

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Make the anti-bot response retryable (injectable body validator)

Add an optional body validator to the retry loop so a 200-OK anti-bot response is retried with the same backoff/jitter/rate-limiter as a 429/5xx. The validator is caller-supplied, so `client.go` stays domain-agnostic (Hotels/Explore/Calendar keep `nil`).

**Files:**
- Modify: `internal/batchexec/client.go` (`doWithRetry` ~260, `PostForm` ~240, `SearchFlightsGLCurr` ~387)
- Test: `internal/batchexec/client_validated_test.go`

**Interfaces:**
- Consumes: `IsBlockedFlightResponse([]byte) bool` (Task 2); existing `isRetryable`, `backoffSleep`, `defaultMaxRetries`.
- Produces:
  - `func (c *Client) doWithRetryValidated(ctx, buildReq func() (*http.Request, error), retryableBody func([]byte) bool) (int, []byte, error)`
  - `func (c *Client) PostFormValidated(ctx context.Context, url, formBody string, retryableBody func([]byte) bool) (int, []byte, error)`
  - `doWithRetry` and `PostForm` keep their existing signatures, delegating with `retryableBody == nil`.

- [ ] **Step 1: Write the failing test**

```go
package batchexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPostFormValidated_RetriesBlockedBody(t *testing.T) {
	var calls atomic.Int32
	blocked := `)]}'` + "\n" + `[["wrb.fr","GetShoppingResults",null,null,null,null,"generic"]]`
	good := `)]}'` + "\n" + `[["wrb.fr","GetShoppingResults","[1,2,[],[]]"]]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if calls.Add(1) <= 2 {
			_, _ = w.Write([]byte(blocked))
			return
		}
		_, _ = w.Write([]byte(good))
	}))
	defer srv.Close()

	c := NewClient()
	c.SetBaseBackoffForTest(0) // see Step 3 note: avoid slow test

	status, body, err := c.PostFormValidated(context.Background(), srv.URL, "f.req=x", IsBlockedFlightResponse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if IsBlockedFlightResponse(body) {
		t.Fatalf("final body still blocked; retries did not reach the good response")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("server calls = %d, want 3 (2 blocked + 1 good)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/batchexec/ -run TestPostFormValidated_RetriesBlockedBody -v`
Expected: FAIL — `c.PostFormValidated undefined` and `c.SetBaseBackoffForTest undefined`.

- [ ] **Step 3: Add a test seam for backoff, then refactor the retry loop**

First add a per-client backoff override so the test does not sleep 1s+2s. In `client.go`, add a field to `Client` (find the struct definition near the top) and a setter:

```go
// In the Client struct, add:
//   baseBackoff time.Duration // 0 means use defaultBaseBackoff

// SetBaseBackoffForTest overrides the retry backoff base. Test-only.
func (c *Client) SetBaseBackoffForTest(d time.Duration) { c.baseBackoff = d }

// effectiveBackoff returns the configured base backoff, or the default.
func (c *Client) effectiveBackoff() time.Duration {
	if c.baseBackoff > 0 {
		return c.baseBackoff
	}
	return defaultBaseBackoff
}
```

Change `backoffSleep` to take the base and to no-op on zero:

```go
// backoffSleep sleeps for exponential backoff with jitter. base doubles each
// attempt (base, 2*base, 4*base) with +-25% jitter. A zero base returns
// immediately (test seam).
func backoffSleep(ctx context.Context, attempt int, base time.Duration) error {
	if base <= 0 {
		return nil
	}
	d := base << attempt
	jitter := time.Duration(float64(d) * (0.75 + rand.Float64()*0.5))
	timer := time.NewTimer(jitter)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

Then split `doWithRetry` into a validated variant. Replace the existing `doWithRetry` with:

```go
// doWithRetry executes an HTTP request with rate limiting and retry on 429/5xx.
func (c *Client) doWithRetry(ctx context.Context, buildReq func() (*http.Request, error)) (int, []byte, error) {
	return c.doWithRetryValidated(ctx, buildReq, nil)
}

// doWithRetryValidated is doWithRetry plus an optional body validator. When
// retryableBody != nil and a 200 response's body satisfies it, the response is
// treated as retryable (same backoff path as 429/5xx). Used to retry Google's
// 200-OK anti-bot responses. retryableBody == nil preserves status-only retry.
func (c *Client) doWithRetryValidated(ctx context.Context, buildReq func() (*http.Request, error), retryableBody func([]byte) bool) (int, []byte, error) {
	var lastStatus int
	var lastBody []byte
	var lastErr error

	for attempt := range defaultMaxRetries + 1 {
		if err := c.limiter.Wait(ctx); err != nil {
			return 0, nil, fmt.Errorf("rate limiter: %w", err)
		}

		req, err := buildReq()
		if err != nil {
			return 0, nil, err
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if attempt < defaultMaxRetries {
				slog.Warn("retry", "attempt", attempt, "error", err.Error())
				if sleepErr := backoffSleep(ctx, attempt, c.effectiveBackoff()); sleepErr != nil {
					return 0, nil, sleepErr
				}
				continue
			}
			return 0, nil, lastErr
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < defaultMaxRetries {
				slog.Warn("retry", "attempt", attempt, "error", readErr.Error())
				if sleepErr := backoffSleep(ctx, attempt, c.effectiveBackoff()); sleepErr != nil {
					return 0, nil, sleepErr
				}
				continue
			}
			return 0, nil, lastErr
		}

		lastStatus = resp.StatusCode
		lastBody = body
		lastErr = nil

		retryable := isRetryable(resp.StatusCode)
		if !retryable && resp.StatusCode == 200 && retryableBody != nil && retryableBody(body) {
			retryable = true
			slog.Warn("retry", "attempt", attempt, "reason", "blocked_body_200")
		}
		if !retryable {
			return lastStatus, lastBody, nil
		}

		if attempt < defaultMaxRetries {
			slog.Warn("retry", "attempt", attempt, "status", resp.StatusCode)
			if sleepErr := backoffSleep(ctx, attempt, c.effectiveBackoff()); sleepErr != nil {
				return 0, nil, sleepErr
			}
		}
	}

	if lastErr != nil {
		return 0, nil, lastErr
	}
	return lastStatus, lastBody, nil
}
```

Note: every existing `backoffSleep(ctx, attempt)` call site must become `backoffSleep(ctx, attempt, c.effectiveBackoff())`. The block above already does this; ensure no stale 2-arg calls remain.

- [ ] **Step 4: Add `PostFormValidated` and delegate `PostForm`**

```go
// PostForm sends a form-encoded POST, retrying on 429/5xx.
func (c *Client) PostForm(ctx context.Context, url, formBody string) (int, []byte, error) {
	return c.PostFormValidated(ctx, url, formBody, nil)
}

// PostFormValidated is PostForm with an optional 200-body retry validator.
func (c *Client) PostFormValidated(ctx context.Context, url, formBody string, retryableBody func([]byte) bool) (int, []byte, error) {
	return c.doWithRetryValidated(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(formBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
		req.Header.Set("User-Agent", chromeUA)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Origin", "https://www.google.com")
		req.Header.Set("Referer", "https://www.google.com/travel/flights")
		return req, nil
	}, retryableBody)
}
```

- [ ] **Step 5: Wire `SearchFlightsGLCurr` to use the validator and skip caching blocked bodies**

Replace the body of `SearchFlightsGLCurr` (the `PostForm`/cache section) with:

```go
	status, body, err := c.PostFormValidated(ctx, url, payload, IsBlockedFlightResponse)
	if err == nil && status == 200 && !IsBlockedFlightResponse(body) {
		c.setCached(url, payload, body, FlightCacheTTL)
	}
	return status, body, err
```

This guarantees a blocked body is never cached (otherwise a poisoned response would be served for the full TTL).

- [ ] **Step 6: Run the new test and the full batchexec suite**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/batchexec/ -v`
Expected: PASS, including `TestPostFormValidated_RetriesBlockedBody` (server hit 3 times) and all pre-existing tests (which use `nil` validator and unchanged behavior).

- [ ] **Step 7: Commit**

```bash
git add internal/batchexec/client.go internal/batchexec/client_validated_test.go
git commit -m "feat(batchexec): retry 200-anti-bot flight responses via body validator

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Duffel provider with multi-key rotation (`SearchDuffel`)

A self-contained provider that maps a list of slices into Duffel's offer-request API and returns `[]models.FlightResult`. One slice = one-way; two slices = round-trip; N slices = multi-city — all with combined pricing. Keys are read at runtime (never hardcoded) and rotated round-robin with failover.

**Files:**
- Create: `internal/flights/duffel_keys.go`
- Create: `internal/flights/duffel_keys_test.go`
- Create: `internal/flights/duffel.go`
- Test: `internal/flights/duffel_test.go`

**Interfaces:**
- Consumes: `parseISO8601Duration(string) int` (Task 1); `SearchOptions` (search.go:30); `models.FlightResult`, `models.FlightLeg`, `models.AirportInfo`.
- Produces:
  - `func duffelKeys() []string` — keys from `DUFFEL_API_KEYS` (comma/whitespace-separated) or single `DUFFEL_API_KEY`.
  - `func duffelKeyOrder(keys []string) []string` — round-robin reordering driven by an atomic counter.
  - `type DuffelSlice struct { Origin, Destination, DepartureDate string }`
  - `func SearchDuffel(ctx context.Context, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error)` — rotates keys (round-robin) and fails over to the next key on error.
  - `func searchDuffelOnce(ctx context.Context, key string, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error)` — single attempt with one key.
  - `func DuffelEnabled() bool` — true when at least one key is configured.
  - test seam `duffelSetEndpointForTest(url string) func()` — overrides the endpoint and restores on the returned func.

#### Step group A — key loading + rotation (`duffel_keys.go`)

- [ ] **Step 1: Write the failing key-rotation test**

```go
package flights

import "testing"

func TestDuffelKeys_EnvParsing(t *testing.T) {
	t.Setenv("DUFFEL_API_KEY", "")
	t.Setenv("DUFFEL_API_KEYS", "k1, k2 ,k3")
	got := duffelKeys()
	if len(got) != 3 || got[0] != "k1" || got[1] != "k2" || got[2] != "k3" {
		t.Fatalf("duffelKeys() = %v, want [k1 k2 k3]", got)
	}

	// Fallback to single key when DUFFEL_API_KEYS is unset.
	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "solo")
	if got := duffelKeys(); len(got) != 1 || got[0] != "solo" {
		t.Fatalf("single-key fallback = %v, want [solo]", got)
	}

	// None set → empty.
	t.Setenv("DUFFEL_API_KEY", "")
	if got := duffelKeys(); len(got) != 0 {
		t.Fatalf("no keys = %v, want empty", got)
	}
}

func TestDuffelKeyOrder_RoundRobin(t *testing.T) {
	keys := []string{"a", "b", "c"}
	// Each call starts from the next key; all keys present each time.
	first := duffelKeyOrder(keys)
	second := duffelKeyOrder(keys)
	if first[0] == second[0] {
		t.Errorf("round-robin did not advance: %v then %v", first, second)
	}
	if len(first) != 3 || len(second) != 3 {
		t.Errorf("ordering must contain all keys: %v %v", first, second)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run 'TestDuffelKeys|TestDuffelKeyOrder' -v`
Expected: FAIL — `undefined: duffelKeys`, `duffelKeyOrder`.

- [ ] **Step 3: Implement `duffel_keys.go`**

```go
package flights

import (
	"os"
	"strings"
	"sync/atomic"
)

// duffelKeyCounter drives round-robin key selection across searches.
var duffelKeyCounter atomic.Uint64

// duffelKeys returns the configured Duffel API keys, read at runtime so they are
// never compiled into the binary. DUFFEL_API_KEYS (comma/whitespace-separated)
// takes precedence; otherwise a single DUFFEL_API_KEY is used.
func duffelKeys() []string {
	if v := strings.TrimSpace(os.Getenv("DUFFEL_API_KEYS")); v != "" {
		fields := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == '\t'
		})
		out := make([]string, 0, len(fields))
		for _, f := range fields {
			if f = strings.TrimSpace(f); f != "" {
				out = append(out, f)
			}
		}
		return out
	}
	if v := strings.TrimSpace(os.Getenv("DUFFEL_API_KEY")); v != "" {
		return []string{v}
	}
	return nil
}

// duffelKeyOrder reorders keys round-robin so each search starts from the next
// key (load spread). Callers then fail over down the returned slice in order.
func duffelKeyOrder(keys []string) []string {
	n := len(keys)
	if n <= 1 {
		return keys
	}
	start := int((duffelKeyCounter.Add(1) - 1) % uint64(n))
	ordered := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ordered = append(ordered, keys[(start+i)%n])
	}
	return ordered
}
```

- [ ] **Step 4: Run the key tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run 'TestDuffelKeys|TestDuffelKeyOrder' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/flights/duffel_keys.go internal/flights/duffel_keys_test.go
git commit -m "feat(flights): runtime Duffel key loading + round-robin rotation

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

#### Step group B — provider + mapping (`duffel.go`)

- [ ] **Step 6: Write the failing provider test**

Create a fixture and a test that serves it from an `httptest` server.

```go
package flights

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

const duffelFixture = `{"data":{"offers":[
 {"total_amount":"881.00","total_currency":"USD","total_emissions_kg":"1200","owner":{"iata_code":"BR","name":"EVA Air"},
  "slices":[
   {"origin":{"iata_code":"HAM","name":"Hamburg"},"destination":{"iata_code":"FUK","name":"Fukuoka"},"duration":"P1DT4H55M",
    "segments":[
     {"origin":{"iata_code":"HAM","name":"Hamburg"},"destination":{"iata_code":"MUC","name":"Munich"},
      "departing_at":"2026-09-15T06:25:00","arriving_at":"2026-09-15T07:45:00","duration":"PT1H20M",
      "marketing_carrier":{"iata_code":"EW","name":"Eurowings"},"marketing_carrier_flight_number":"7170",
      "aircraft":{"name":"Airbus A320"},
      "passengers":[{"baggages":[{"type":"checked","quantity":2},{"type":"carry_on","quantity":1}]}]},
     {"origin":{"iata_code":"MUC","name":"Munich"},"destination":{"iata_code":"FUK","name":"Fukuoka"},
      "departing_at":"2026-09-15T12:00:00","arriving_at":"2026-09-16T11:20:00","duration":"PT12H35M",
      "marketing_carrier":{"iata_code":"BR","name":"EVA Air"},"marketing_carrier_flight_number":"72",
      "aircraft":{"name":"Boeing 777-300ER"},
      "passengers":[{"baggages":[{"type":"checked","quantity":2},{"type":"carry_on","quantity":1}]}]}
    ]}
  ]}
]}}`

func TestSearchDuffel_MapsOffer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Errorf("missing Authorization header")
		}
		if r.Header.Get("Duffel-Version") != "v2" {
			t.Errorf("Duffel-Version = %q, want v2", r.Header.Get("Duffel-Version"))
		}
		// Assert the request body carries the slices we passed in.
		var req struct {
			Data struct {
				Slices []map[string]string `json:"slices"`
			} `json:"data"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
		if len(req.Data.Slices) != 1 {
			t.Errorf("slices in request = %d, want 1", len(req.Data.Slices))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(duffelFixture))
	}))
	defer srv.Close()

	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "test_key")
	restore := duffelSetEndpointForTest(srv.URL)
	defer restore()

	got, err := SearchDuffel(context.Background(),
		[]DuffelSlice{{Origin: "HAM", Destination: "FUK", DepartureDate: "2026-09-15"}},
		SearchOptions{Adults: 1, CabinClass: models.Economy})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("results = %d, want 1", len(got))
	}
	f := got[0]
	if f.Price != 881.00 {
		t.Errorf("price = %v, want 881.00", f.Price)
	}
	if f.Currency != "USD" {
		t.Errorf("currency = %q, want USD", f.Currency)
	}
	if f.Provider != "duffel" {
		t.Errorf("provider = %q, want duffel", f.Provider)
	}
	if f.BookingURL != "" {
		t.Errorf("booking_url = %q, want empty", f.BookingURL)
	}
	if len(f.Legs) != 2 {
		t.Fatalf("legs = %d, want 2", len(f.Legs))
	}
	if f.Stops != 1 { // 2 segments in 1 slice → 1 stop
		t.Errorf("stops = %d, want 1", f.Stops)
	}
	if f.Duration != parseISO8601Duration("P1DT4H55M") {
		t.Errorf("duration = %d, want %d", f.Duration, parseISO8601Duration("P1DT4H55M"))
	}
	if f.Emissions != 1200000 { // 1200 kg * 1000
		t.Errorf("emissions = %d, want 1200000", f.Emissions)
	}
	if f.CarryOnIncluded == nil || !*f.CarryOnIncluded {
		t.Errorf("carry_on_included = %v, want true", f.CarryOnIncluded)
	}
	if f.CheckedBagsIncluded == nil || *f.CheckedBagsIncluded != 2 {
		t.Errorf("checked_bags_included = %v, want 2", f.CheckedBagsIncluded)
	}
	if f.Legs[0].AirlineCode != "EW" || f.Legs[0].FlightNumber != "7170" {
		t.Errorf("leg0 carrier = %q%q, want EW7170", f.Legs[0].AirlineCode, f.Legs[0].FlightNumber)
	}
	if f.Legs[1].Aircraft != "Boeing 777-300ER" {
		t.Errorf("leg1 aircraft = %q", f.Legs[1].Aircraft)
	}
}

func TestSearchDuffel_DisabledWithoutKey(t *testing.T) {
	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "")
	if DuffelEnabled() {
		t.Fatalf("DuffelEnabled() = true with no keys")
	}
	_, err := SearchDuffel(context.Background(),
		[]DuffelSlice{{Origin: "HAM", Destination: "FUK", DepartureDate: "2026-09-15"}},
		SearchOptions{Adults: 1})
	if err == nil {
		t.Fatalf("expected error when no Duffel keys are set")
	}
}

func TestSearchDuffel_FailoverToNextKey(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First key (whichever round-robin picks first) gets 401; retry with the
		// next key succeeds.
		if calls == 1 {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"errors":[{"title":"Unauthorized"}]}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(duffelFixture))
	}))
	defer srv.Close()

	t.Setenv("DUFFEL_API_KEYS", "bad,good")
	restore := duffelSetEndpointForTest(srv.URL)
	defer restore()

	got, err := SearchDuffel(context.Background(),
		[]DuffelSlice{{Origin: "HAM", Destination: "FUK", DepartureDate: "2026-09-15"}},
		SearchOptions{Adults: 1, CabinClass: models.Economy})
	if err != nil {
		t.Fatalf("expected failover to succeed, got error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("results after failover = %d, want 1", len(got))
	}
	if calls != 2 {
		t.Fatalf("server calls = %d, want 2 (1 failed key + 1 success)", calls)
	}
}
```

- [ ] **Step 7: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run TestSearchDuffel -v`
Expected: FAIL — `undefined: SearchDuffel`, `DuffelSlice`, `DuffelEnabled`, `searchDuffelOnce`, `duffelSetEndpointForTest`.

- [ ] **Step 8: Write the implementation**

```go
package flights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

const (
	duffelDefaultEndpoint = "https://api.duffel.com/air/offer_requests?return_offers=true&supplier_timeout=15000"
	duffelVersion         = "v2"
)

// duffelEndpoint is overridable in tests via duffelSetEndpointForTest.
var duffelEndpoint = duffelDefaultEndpoint

func duffelSetEndpointForTest(url string) func() {
	prev := duffelEndpoint
	duffelEndpoint = url
	return func() { duffelEndpoint = prev }
}

// DuffelEnabled reports whether at least one Duffel key is configured.
func DuffelEnabled() bool { return len(duffelKeys()) > 0 }

// DuffelSlice is one leg of a Duffel offer request. One slice = one-way; two =
// round-trip; N = multi-city, all priced as a single combined offer.
type DuffelSlice struct {
	Origin        string
	Destination   string
	DepartureDate string
}

// --- request types ---

type duffelReqSlice struct {
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
}

type duffelReqPassenger struct {
	Type string `json:"type"`
}

type duffelRequest struct {
	Data struct {
		Slices     []duffelReqSlice     `json:"slices"`
		Passengers []duffelReqPassenger `json:"passengers"`
		CabinClass string               `json:"cabin_class,omitempty"`
	} `json:"data"`
}

// --- response types ---

type duffelPlace struct {
	IATACode string `json:"iata_code"`
	Name     string `json:"name"`
}

type duffelBaggage struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
}

type duffelSegPassenger struct {
	Baggages []duffelBaggage `json:"baggages"`
}

type duffelSegment struct {
	Origin                       duffelPlace          `json:"origin"`
	Destination                  duffelPlace          `json:"destination"`
	DepartingAt                  string               `json:"departing_at"`
	ArrivingAt                   string               `json:"arriving_at"`
	Duration                     string               `json:"duration"`
	MarketingCarrier             duffelPlace          `json:"marketing_carrier"`
	MarketingCarrierFlightNumber string               `json:"marketing_carrier_flight_number"`
	Aircraft                     struct{ Name string } `json:"aircraft"`
	Passengers                   []duffelSegPassenger `json:"passengers"`
}

type duffelOfferSlice struct {
	Origin      duffelPlace     `json:"origin"`
	Destination duffelPlace     `json:"destination"`
	Duration    string          `json:"duration"`
	Segments    []duffelSegment `json:"segments"`
}

type duffelOffer struct {
	TotalAmount       string             `json:"total_amount"`
	TotalCurrency     string             `json:"total_currency"`
	TotalEmissionsKg  string             `json:"total_emissions_kg"`
	Owner             duffelPlace        `json:"owner"`
	Slices            []duffelOfferSlice `json:"slices"`
}

type duffelResponse struct {
	Data struct {
		Offers []duffelOffer `json:"offers"`
	} `json:"data"`
}

// SearchDuffel queries the Duffel offer-request API for the given slices and
// maps each offer into a models.FlightResult. Keys are rotated round-robin and
// the search fails over to the next key on error. Requires at least one key
// (DUFFEL_API_KEYS or DUFFEL_API_KEY).
func SearchDuffel(ctx context.Context, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error) {
	opts.defaults()
	keys := duffelKeys()
	if len(keys) == 0 {
		return nil, fmt.Errorf("duffel: no API keys configured (set DUFFEL_API_KEYS or DUFFEL_API_KEY)")
	}
	if len(slices) == 0 {
		return nil, fmt.Errorf("duffel: at least one slice required")
	}

	var lastErr error
	for _, key := range duffelKeyOrder(keys) {
		results, err := searchDuffelOnce(ctx, key, slices, opts)
		if err != nil {
			lastErr = err
			slog.Warn("duffel key failed, failing over", "error", err)
			continue
		}
		return results, nil
	}
	return nil, fmt.Errorf("duffel: all keys exhausted: %w", lastErr)
}

// searchDuffelOnce performs a single Duffel offer-request with one API key.
func searchDuffelOnce(ctx context.Context, key string, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error) {
	var req duffelRequest
	for _, s := range slices {
		req.Data.Slices = append(req.Data.Slices, duffelReqSlice{
			Origin: s.Origin, Destination: s.Destination, DepartureDate: s.DepartureDate,
		})
	}
	for i := 0; i < opts.Adults; i++ {
		req.Data.Passengers = append(req.Data.Passengers, duffelReqPassenger{Type: "adult"})
	}
	req.Data.CabinClass = opts.CabinClass.String() // "economy", "business", ...

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("duffel: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, duffelEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("duffel: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("Duffel-Version", duffelVersion)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("duffel: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("duffel: unexpected status %d", resp.StatusCode)
	}

	var decoded duffelResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("duffel: decode response: %w", err)
	}

	results := make([]models.FlightResult, 0, len(decoded.Data.Offers))
	for _, o := range decoded.Data.Offers {
		results = append(results, mapDuffelOffer(o))
	}
	return results, nil
}

// mapDuffelOffer converts a single Duffel offer into a FlightResult. All
// segments across all slices are flattened into Legs; Stops is the total number
// of in-leg connections (segments minus one per slice). BookingURL is empty:
// Duffel exposes no public booking deep-link.
func mapDuffelOffer(o duffelOffer) models.FlightResult {
	price, _ := strconv.ParseFloat(o.TotalAmount, 64)

	var legs []models.FlightLeg
	totalDuration, stops := 0, 0
	var carryOn *bool
	var checked *int

	for _, sl := range o.Slices {
		totalDuration += parseISO8601Duration(sl.Duration)
		if len(sl.Segments) > 0 {
			stops += len(sl.Segments) - 1
		}
		for si, seg := range sl.Segments {
			layover := 0
			if si > 0 {
				prev := sl.Segments[si-1]
				layover = minutesBetween(prev.ArrivingAt, seg.DepartingAt)
			}
			legs = append(legs, models.FlightLeg{
				DepartureAirport: models.AirportInfo{Code: seg.Origin.IATACode, Name: seg.Origin.Name},
				ArrivalAirport:   models.AirportInfo{Code: seg.Destination.IATACode, Name: seg.Destination.Name},
				DepartureTime:    seg.DepartingAt,
				ArrivalTime:      seg.ArrivingAt,
				Duration:         parseISO8601Duration(seg.Duration),
				Airline:          seg.MarketingCarrier.Name,
				AirlineCode:      seg.MarketingCarrier.IATACode,
				FlightNumber:     seg.MarketingCarrierFlightNumber,
				Aircraft:         seg.Aircraft.Name,
				LayoverMinutes:   layover,
			})
			// Baggage from the first segment's first passenger (offer-wide).
			if carryOn == nil && len(seg.Passengers) > 0 {
				co, ck := duffelBaggageCounts(seg.Passengers[0].Baggages)
				carryOn, checked = co, ck
			}
		}
	}

	emissionsKg, _ := strconv.Atoi(o.TotalEmissionsKg)

	return models.FlightResult{
		Price:               price,
		Currency:            o.TotalCurrency,
		Duration:            totalDuration,
		Stops:               stops,
		Provider:            "duffel",
		Legs:                legs,
		BookingURL:          "",
		CarryOnIncluded:     carryOn,
		CheckedBagsIncluded: checked,
		Emissions:           emissionsKg * 1000,
	}
}

func duffelBaggageCounts(bags []duffelBaggage) (*bool, *int) {
	co := false
	ck := 0
	for _, b := range bags {
		switch b.Type {
		case "carry_on":
			if b.Quantity > 0 {
				co = true
			}
		case "checked":
			ck += b.Quantity
		}
	}
	return &co, &ck
}

// minutesBetween parses two RFC3339-ish timestamps (Duffel omits zone:
// "2006-01-02T15:04:05") and returns the gap in minutes, or 0 on parse failure.
func minutesBetween(a, b string) int {
	const layout = "2006-01-02T15:04:05"
	ta, err1 := time.Parse(layout, a)
	tb, err2 := time.Parse(layout, b)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := int(tb.Sub(ta).Minutes())
	if d < 0 {
		return 0
	}
	return d
}
```

- [ ] **Step 9: Run tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run TestSearchDuffel -v`
Expected: PASS (`TestSearchDuffel_MapsOffer`, `TestSearchDuffel_DisabledWithoutKey`, `TestSearchDuffel_FailoverToNextKey`).

- [ ] **Step 10: Commit**

```bash
git add internal/flights/duffel.go internal/flights/duffel_test.go
git commit -m "feat(flights): add Duffel provider with round-robin+failover keys

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Wire Duffel fallback into one-way / round-trip search

When Google fails (after retries) in `searchFlightsCore`, fall back to Duffel before giving up. Per the agreed trigger, Duffel fires on Google failure independent of Kiwi.

**Files:**
- Modify: `internal/flights/search.go` (`searchFlightsCore`, search.go:142-192)
- Test: `internal/flights/search.go` is covered via a new test in `internal/flights/duffel_test.go` (fallback helper is unit-tested directly).

**Interfaces:**
- Consumes: `SearchDuffel`, `DuffelEnabled`, `DuffelSlice` (Task 4); existing `tripTypeForSearch`, `mergeFlightResults`.
- Produces: helper `func duffelSlicesForSearch(origin, destination, date string, opts SearchOptions) []DuffelSlice` — returns one slice for one-way, two for round-trip (when `opts.ReturnDate != ""`).

- [ ] **Step 1: Write the failing test**

Add to `internal/flights/duffel_test.go`:

```go
func TestDuffelSlicesForSearch(t *testing.T) {
	oneway := duffelSlicesForSearch("HAM", "FUK", "2026-09-15", SearchOptions{})
	if len(oneway) != 1 {
		t.Fatalf("one-way slices = %d, want 1", len(oneway))
	}
	if oneway[0].Origin != "HAM" || oneway[0].Destination != "FUK" || oneway[0].DepartureDate != "2026-09-15" {
		t.Errorf("one-way slice = %+v", oneway[0])
	}

	rt := duffelSlicesForSearch("HAM", "FUK", "2026-09-15", SearchOptions{ReturnDate: "2026-09-28"})
	if len(rt) != 2 {
		t.Fatalf("round-trip slices = %d, want 2", len(rt))
	}
	if rt[1].Origin != "FUK" || rt[1].Destination != "HAM" || rt[1].DepartureDate != "2026-09-28" {
		t.Errorf("return slice = %+v", rt[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run TestDuffelSlicesForSearch -v`
Expected: FAIL — `undefined: duffelSlicesForSearch`.

- [ ] **Step 3: Add the helper and wire the fallback**

Add the helper to `search.go`:

```go
// duffelSlicesForSearch builds Duffel slices for a point-to-point search: one
// slice for one-way, plus a reversed return slice when opts.ReturnDate is set.
func duffelSlicesForSearch(origin, destination, date string, opts SearchOptions) []DuffelSlice {
	slices := []DuffelSlice{{Origin: origin, Destination: destination, DepartureDate: date}}
	if opts.ReturnDate != "" {
		slices = append(slices, DuffelSlice{Origin: destination, Destination: origin, DepartureDate: opts.ReturnDate})
	}
	return slices
}
```

Then, in `searchFlightsCore`, after the existing Google+Kiwi block, replace the final failure section so Duffel is attempted when Google failed. Locate the block starting at `if googleErr != nil && kiwiErr != nil {` (search.go:174) and insert a Duffel attempt before it:

```go
	mergedFlights := mergeFlightResults(googleFlights, kiwiFlights, opts)
	if googleSucceeded || kiwiSucceeded {
		return &models.FlightSearchResult{
			Success:  true,
			Count:    len(mergedFlights),
			TripType: tripTypeForSearch(opts),
			Flights:  mergedFlights,
		}, nil
	}

	// Google failed → try Duffel (paid fallback, only when configured).
	if !googleSucceeded && DuffelEnabled() {
		duffelFlights, duffelErr := SearchDuffel(ctx, duffelSlicesForSearch(origin, destination, date, opts), opts)
		if duffelErr != nil {
			slog.Warn("duffel flight search failed", "origin", origin, "destination", destination, "date", date, "error", duffelErr)
		} else if len(duffelFlights) > 0 {
			return &models.FlightSearchResult{
				Success:  true,
				Count:    len(duffelFlights),
				TripType: tripTypeForSearch(opts),
				Flights:  duffelFlights,
			}, nil
		}
	}

	if googleErr != nil && kiwiErr != nil {
		err := errors.Join(googleErr, kiwiErr)
		return &models.FlightSearchResult{Error: err.Error()}, err
	}
```

The remaining `if googleErr != nil { ... }` / `if kiwiErr != nil { ... }` tail stays unchanged.

- [ ] **Step 4: Run the flights suite**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -v -run 'Duffel|SearchFlights'`
Expected: PASS, including `TestDuffelSlicesForSearch`. Pre-existing search tests stay green (Duffel is gated behind `DUFFEL_API_KEY`, unset in those tests → no-op).

- [ ] **Step 5: Commit**

```bash
git add internal/flights/search.go internal/flights/duffel_test.go
git commit -m "feat(flights): fall back to Duffel when Google one-way/round-trip fails

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Wire Duffel fallback into multi-city + document

Multi-city is the case that fails most. `SearchMultiCity` (multicity.go:65) calls `runGoogleFlightSearch` directly and returns its error. Add an **additive** Duffel fallback: Google stays primary; Duffel fires only on Google error. The `--leg` parsing and Google path are untouched.

**Files:**
- Modify: `internal/flights/multicity.go` (the error branch at multicity.go:87-90)
- Test: add to `internal/flights/duffel_test.go`
- Modify: `docs/` provider notes (a short section appended to the existing flights docs if present, else skip silently).

**Interfaces:**
- Consumes: `SearchDuffel`, `DuffelEnabled`, `DuffelSlice` (Task 4); existing `Leg` type (multicity.go:13).
- Produces: helper `func duffelSlicesForLegs(legs []Leg) []DuffelSlice` — maps each `Leg` to a `DuffelSlice` using the first origin/destination of each leg.

- [ ] **Step 1: Write the failing test**

Add to `internal/flights/duffel_test.go`:

```go
func TestDuffelSlicesForLegs(t *testing.T) {
	legs := []Leg{
		{Origins: []string{"HAM"}, Destinations: []string{"FUK"}, Date: "2026-09-15"},
		{Origins: []string{"NRT"}, Destinations: []string{"HAM"}, Date: "2026-09-28"},
	}
	slices := duffelSlicesForLegs(legs)
	if len(slices) != 2 {
		t.Fatalf("slices = %d, want 2", len(slices))
	}
	if slices[0].Origin != "HAM" || slices[0].Destination != "FUK" || slices[0].DepartureDate != "2026-09-15" {
		t.Errorf("slice0 = %+v", slices[0])
	}
	if slices[1].Origin != "NRT" || slices[1].Destination != "HAM" || slices[1].DepartureDate != "2026-09-28" {
		t.Errorf("slice1 = %+v", slices[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -run TestDuffelSlicesForLegs -v`
Expected: FAIL — `undefined: duffelSlicesForLegs`.

- [ ] **Step 3: Add the helper and wire the fallback**

Add to `multicity.go`:

```go
// duffelSlicesForLegs maps multi-city legs to Duffel slices, using the first
// origin/destination airport of each leg (Duffel takes a single IATA code per
// endpoint, unlike Google which accepts an airport set).
func duffelSlicesForLegs(legs []Leg) []DuffelSlice {
	slices := make([]DuffelSlice, 0, len(legs))
	for _, leg := range legs {
		if len(leg.Origins) == 0 || len(leg.Destinations) == 0 {
			continue
		}
		slices = append(slices, DuffelSlice{
			Origin:        leg.Origins[0],
			Destination:   leg.Destinations[0],
			DepartureDate: leg.Date,
		})
	}
	return slices
}
```

Replace the Google error branch in `SearchMultiCity` (multicity.go:87-90):

```go
	flights, err := runGoogleFlightSearch(ctx, DefaultClient(), filters, opts)
	if err != nil {
		// Google failed → try Duffel (paid fallback, native multi-city).
		if DuffelEnabled() {
			if duffelFlights, dErr := SearchDuffel(ctx, duffelSlicesForLegs(legs), opts); dErr == nil && len(duffelFlights) > 0 {
				return &models.FlightSearchResult{
					Success:  true,
					Count:    len(duffelFlights),
					TripType: "multi_city",
					Flights:  duffelFlights,
				}, nil
			}
		}
		return &models.FlightSearchResult{Error: err.Error()}, err
	}
```

The rest of `SearchMultiCity` (booking-link assignment, success return) is unchanged.

- [ ] **Step 4: Run the full flights suite + the multi-city tests**

Run: `GOTOOLCHAIN=go1.26.2 go test ./internal/flights/ -v`
Expected: PASS, including `TestDuffelSlicesForLegs` and the existing `multicity_test.go` tests (Duffel gated off when `DUFFEL_API_KEY` unset).

- [ ] **Step 5: Add a brief provider note (if a flights doc exists)**

Check for existing docs: `ls docs/` and any flights doc (e.g. a markdown file mentioning multi-city). If one exists, append a short note; if none, skip. Note content:

```markdown
## Duffel fallback

When Google Flights is unavailable (anti-bot rejection retried and exhausted),
trvl falls back to the Duffel API for one-way, round-trip, and multi-city
searches. Requires `DUFFEL_API_KEY`. Duffel offers carry no booking deep-link
(BookingURL is empty) and are priced in the Duffel organization's billing
currency (not request-controllable).
```

- [ ] **Step 6: Lint, vet, and full build**

Run:
```bash
GOTOOLCHAIN=go1.26.2 go vet ./...
GOTOOLCHAIN=go1.26.2 staticcheck ./... 2>/dev/null || echo "staticcheck not installed; CI will run it"
GOTOOLCHAIN=go1.26.2 go build ./...
GOTOOLCHAIN=go1.26.2 go test ./...
```
Expected: vet clean, build OK, all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/flights/multicity.go internal/flights/duffel_test.go docs/
git commit -m "feat(flights): fall back to Duffel for multi-city when Google fails

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Out of Scope (explicitly deferred)

- **Honest provider status / Completeness envelope** (the prior `2026-06-17-skiplagged-fallback-honest-status.md` plan) — independent; can layer on top later.
- **Kiwi / Skiplagged as Duffel-tier fallbacks** — not in this plan; current Kiwi merge behavior for one-way/round-trip is unchanged.
- **Duffel currency conversion** — handled by the existing display-layer conversion; no per-request currency control exists in Duffel.
- **Duffel order/booking flow** — search only; no deep-link, no order creation.
- **Duffel rate limiting / caching** — v1 uses a plain `http.Client`; if call volume grows, add caching/rate-limit in a follow-up.

## Self-Review Notes

- **Trigger semantics:** Per the agreed decision, Duffel fires when **Google** fails (not gated on Kiwi). For one-way/round-trip, if Google succeeds Duffel is never called (billing minimized); if Google fails, Duffel is tried regardless of Kiwi. For multi-city, Kiwi never applied, so it's Google→Duffel.
- **Retry count:** unchanged `defaultMaxRetries = 3` (4 attempts) before the body validator gives up and the caller falls back.
- **Caching guard:** `SearchFlightsGLCurr` must not cache a blocked body (Task 3, Step 5) — otherwise a poisoned response is served for the full TTL and the fallback never re-triggers.
- **Type consistency:** `DuffelSlice{Origin,Destination,DepartureDate}` used identically in Tasks 4/5/6; `SearchDuffel(ctx, []DuffelSlice, SearchOptions)` consistent across call sites; `parseISO8601Duration` consistent in Tasks 1/4.
- **Additive constraint:** `SearchMultiCity` and `cmd/trvl/flights.go` keep the Google path and `--leg`; only an error-branch fallback is added to `multicity.go`. `multi.go` is untouched.
- **Key handling:** keys are loaded at runtime via `duffelKeys()` (`DUFFEL_API_KEYS` preferred, single `DUFFEL_API_KEY` fallback) — never hardcoded in the binary. `SearchDuffel` rotates round-robin via `duffelKeyOrder` and fails over to the next key on error (`searchDuffelOnce` per attempt). `DuffelEnabled()` reflects key presence so the fallback is a no-op when no keys are set.
