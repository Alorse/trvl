package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/waf"
	"golang.org/x/time/rate"
)

const (
	defaultRPS         = 0.5
	defaultBurst       = 1
	authCacheDuration  = 10 * time.Minute
	boundingBoxOffset  = 0.15
	maxResponseBytes   = 10 * 1024 * 1024 // 10 MB
)

// Runtime is the generic HTTP execution engine for configured providers.
type Runtime struct {
	registry *Registry
	clients  map[string]*providerClient
	mu       sync.RWMutex
}

// providerClient holds per-provider HTTP state.
type providerClient struct {
	config     *ProviderConfig
	client     *http.Client
	limiter    *rate.Limiter
	authMu     sync.RWMutex
	authValues map[string]string
	authExpiry time.Time
}

// NewRuntime creates a Runtime backed by the given registry.
func NewRuntime(registry *Registry) *Runtime {
	return &Runtime{
		registry: registry,
		clients:  make(map[string]*providerClient),
	}
}

// getOrCreateClient returns the providerClient for the given config,
// creating it on first access.
func (rt *Runtime) getOrCreateClient(cfg *ProviderConfig) *providerClient {
	rt.mu.RLock()
	pc, ok := rt.clients[cfg.ID]
	rt.mu.RUnlock()
	if ok {
		return pc
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Double-check after acquiring write lock.
	if pc, ok := rt.clients[cfg.ID]; ok {
		return pc
	}

	var httpClient *http.Client
	if cfg.TLS.Fingerprint == "chrome" {
		httpClient = batchexec.ChromeHTTPClient()
	} else {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	if httpClient.Jar == nil {
		jar, _ := cookiejar.New(nil)
		httpClient.Jar = jar
	}

	rps := cfg.RateLimit.RequestsPerSecond
	if rps <= 0 {
		rps = defaultRPS
	}
	burst := cfg.RateLimit.Burst
	if burst <= 0 {
		burst = defaultBurst
	}

	pc = &providerClient{
		config:     cfg,
		client:     httpClient,
		limiter:    rate.NewLimiter(rate.Limit(rps), burst),
		authValues: make(map[string]string),
	}
	rt.clients[cfg.ID] = pc
	return pc
}

// SearchHotels queries all hotel-category providers and returns combined results
// along with per-provider status entries so the caller can surface failures to
// the LLM for autonomous diagnosis.
func (rt *Runtime) SearchHotels(ctx context.Context, location string, lat, lon float64, checkin, checkout, currency string, guests int) ([]models.HotelResult, []models.ProviderStatus, error) {
	providers := rt.registry.ListByCategory("hotels")
	if len(providers) == 0 {
		return nil, nil, nil
	}

	type result struct {
		hotels []models.HotelResult
		err    error
		id     string
		name   string
	}

	results := make(chan result, len(providers))
	var wg sync.WaitGroup

	for _, cfg := range providers {
		wg.Add(1)
		go func(cfg *ProviderConfig) {
			defer wg.Done()
			hotels, err := rt.searchProvider(ctx, cfg, location, lat, lon, checkin, checkout, currency, guests)
			results <- result{hotels: hotels, err: err, id: cfg.ID, name: cfg.Name}
		}(cfg)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var statuses []models.ProviderStatus
	var combined []models.HotelResult
	var firstErr error
	for r := range results {
		if r.err != nil {
			slog.Warn("provider error", "provider", r.id, "error", r.err.Error())
			rt.registry.MarkError(r.id, r.err.Error())
			statuses = append(statuses, models.ProviderStatus{
				ID:      r.id,
				Name:    r.name,
				Status:  "error",
				Error:   r.err.Error(),
				FixHint: providerFixHint(r.err),
			})
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		rt.registry.MarkSuccess(r.id)
		statuses = append(statuses, models.ProviderStatus{
			ID:      r.id,
			Name:    r.name,
			Status:  "ok",
			Results: len(r.hotels),
		})
		combined = append(combined, r.hotels...)
	}

	if len(combined) == 0 && firstErr != nil {
		return nil, statuses, firstErr
	}
	return combined, statuses, nil
}

// providerFixHint generates an actionable LLM-readable hint for common failures.
func providerFixHint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "preflight"):
		return "Call test_provider with this provider's id to diagnose. WAF/auth may need refresh."
	case strings.Contains(msg, "results_path"):
		return "API response structure changed. Call test_provider to see current response shape, then configure_provider to update results_path."
	case strings.Contains(msg, "http 403"), strings.Contains(msg, "http 202"):
		return "WAF block detected. Try test_provider — if it fails, the provider may need browser cookie refresh."
	case strings.Contains(msg, "rate limit"):
		return "Rate limited. Wait and retry, or reduce request frequency in provider config."
	default:
		return "Call test_provider with this provider's id to diagnose the issue."
	}
}

func (rt *Runtime) searchProvider(ctx context.Context, cfg *ProviderConfig, location string, lat, lon float64, checkin, checkout, currency string, guests int) ([]models.HotelResult, error) {
	pc := rt.getOrCreateClient(cfg)

	// Rate limit.
	if err := pc.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	// Preflight auth if needed.
	if cfg.Auth != nil && cfg.Auth.Type == "preflight" {
		if err := rt.runPreflight(ctx, pc); err != nil {
			return nil, fmt.Errorf("preflight: %w", err)
		}
	}

	// Build variable map.
	neLat := lat + boundingBoxOffset
	neLon := lon + boundingBoxOffset
	swLat := lat - boundingBoxOffset
	swLon := lon - boundingBoxOffset

	vars := map[string]string{
		"${checkin}":  checkin,
		"${checkout}": checkout,
		"${currency}": currency,
		"${guests}":   strconv.Itoa(guests),
		"${lat}":      strconv.FormatFloat(lat, 'f', 6, 64),
		"${lon}":      strconv.FormatFloat(lon, 'f', 6, 64),
		"${ne_lat}":   strconv.FormatFloat(neLat, 'f', 6, 64),
		"${ne_lon}":   strconv.FormatFloat(neLon, 'f', 6, 64),
		"${sw_lat}":   strconv.FormatFloat(swLat, 'f', 6, 64),
		"${sw_lon}":   strconv.FormatFloat(swLon, 'f', 6, 64),
		"${location}": location,
	}

	// Add auth-extracted variables.
	pc.authMu.RLock()
	for k, v := range pc.authValues {
		vars["${"+k+"}"] = v
	}
	pc.authMu.RUnlock()

	// Build endpoint URL.
	endpoint := substituteVars(cfg.Endpoint, vars)

	// Build query params.
	if len(cfg.QueryParams) > 0 {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("parse endpoint: %w", err)
		}
		q := u.Query()
		for k, v := range cfg.QueryParams {
			q.Set(k, substituteVars(v, vars))
		}
		u.RawQuery = q.Encode()
		endpoint = u.String()
	}

	// Build body.
	var bodyReader io.Reader
	if cfg.Method == "POST" && cfg.BodyTemplate != "" {
		bodyReader = strings.NewReader(substituteVars(cfg.BodyTemplate, vars))
	}

	req, err := http.NewRequestWithContext(ctx, cfg.Method, endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add headers (with both template vars and env vars).
	for k, v := range cfg.Headers {
		req.Header.Set(k, substituteEnvVars(substituteVars(v, vars)))
	}

	// Send request.
	resp, err := pc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	// Parse JSON.
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	// Extract results array.
	resultsRaw := jsonPath(raw, cfg.ResponseMapping.ResultsPath)
	arr, ok := resultsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("results_path %q did not resolve to an array", cfg.ResponseMapping.ResultsPath)
	}

	// Map each element to HotelResult and tag with provider source.
	hotels := make([]models.HotelResult, 0, len(arr))
	for _, item := range arr {
		h := mapHotelResult(item, cfg.ResponseMapping.Fields)
		h.Sources = []models.PriceSource{{
			Provider: cfg.ID,
			Price:    h.Price,
			Currency: h.Currency,
		}}
		hotels = append(hotels, h)
	}

	return hotels, nil
}

// runPreflight performs a GET to the preflight URL and extracts auth values.
func (rt *Runtime) runPreflight(ctx context.Context, pc *providerClient) error {
	pc.authMu.RLock()
	if time.Now().Before(pc.authExpiry) {
		pc.authMu.RUnlock()
		return nil
	}
	pc.authMu.RUnlock()

	pc.authMu.Lock()
	defer pc.authMu.Unlock()

	// Double-check after lock.
	if time.Now().Before(pc.authExpiry) {
		return nil
	}

	if pc.config.Auth == nil || pc.config.Auth.PreflightURL == "" {
		return nil
	}

	// Tier 0: try loading persisted cookies from a previous successful session.
	// This makes browser escape hatch a one-time setup rather than per-search.
	loadCachedCookies(pc.client, pc.config.Auth.PreflightURL)

	resp, body, err := doPreflightRequest(ctx, pc.client, pc.config.Auth)
	if err != nil {
		return err
	}

	extracted := applyExtractions(pc.config.Auth.Extractions, resp, body, pc.authValues)

	// Fallback tier cascade:
	//   Tier 1: preflight request already ran above (extracted ok? done)
	//   Tier 3: read cookies straight from the user's browser via kooky.
	//   Tier 4: if Tier 3 didn't produce a working session AND the caller
	//           opted in (AuthConfig.BrowserEscapeHatch + WithInteractive ctx),
	//           open the preflight URL in the user's browser so they clear
	//           any JS/CAPTCHA challenge, then re-read cookies.
	// (Tier 2 — TLS-fingerprinted retry — is covered by the chrome HTTP
	// client selected in getOrCreateClient; it runs implicitly on every
	// request when cfg.TLS.Fingerprint == "chrome".)
	if needsBrowserCookieFallback(resp.StatusCode, extracted, pc.config.Auth.Extractions) {
		// Tier 3a: read cookies from user's browser (kooky).
		if tryBrowserCookieRetry(ctx, pc) {
			saveCachedCookies(pc.client, pc.config.Auth.PreflightURL)
			pc.authExpiry = time.Now().Add(authCacheDuration)
			return nil
		}
		// Tier 3b: run WAF challenge.js in sobek JS engine (pure Go).
		if tryWAFSolve(ctx, pc, resp.StatusCode, body) {
			saveCachedCookies(pc.client, pc.config.Auth.PreflightURL)
			pc.authExpiry = time.Now().Add(authCacheDuration)
			return nil
		}
		// Tier 4: last-resort escape hatch — open in browser.
		if pc.config.Auth.BrowserEscapeHatch && isInteractive(ctx) {
			if tryBrowserEscapeHatch(ctx, pc) {
				saveCachedCookies(pc.client, pc.config.Auth.PreflightURL)
				pc.authExpiry = time.Now().Add(authCacheDuration)
				return nil
			}
		}
	}

	// Tier 1 succeeded directly — persist cookies for future sessions.
	saveCachedCookies(pc.client, pc.config.Auth.PreflightURL)
	pc.authExpiry = time.Now().Add(authCacheDuration)
	return nil
}

// tryBrowserCookieRetry is Tier 3: read cookies from the user's disk-backed
// browser stores, seed them into the client jar, and retry preflight. Returns
// true on HTTP 2xx + successful extraction.
func tryBrowserCookieRetry(ctx context.Context, pc *providerClient) bool {
	if !applyBrowserCookies(pc.client, pc.config.Auth.PreflightURL) {
		return false
	}
	resp2, body2, err2 := doPreflightRequest(ctx, pc.client, pc.config.Auth)
	if err2 != nil || resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		return false
	}
	for k := range pc.authValues {
		delete(pc.authValues, k)
	}
	applyExtractions(pc.config.Auth.Extractions, resp2, body2, pc.authValues)
	return true
}

// tryWAFSolve is Tier 3b: if the preflight response looks like an AWS WAF
// challenge page (HTTP 202 with *.awswaf.com script refs), run challenge.js
// in the sobek JS engine to obtain an aws-waf-token cookie, then retry
// preflight. Returns true on success.
func tryWAFSolve(ctx context.Context, pc *providerClient, statusCode int, pageBody []byte) bool {
	// Only attempt on HTTP 202 (AWS WAF challenge) or 403 (some WAF variants).
	if statusCode != http.StatusAccepted && statusCode != http.StatusForbidden {
		return false
	}

	pageURL := pc.config.Auth.PreflightURL
	cookie, err := waf.SolveAWSWAF(ctx, pc.client, pageURL, string(pageBody), nil)
	if err != nil {
		slog.Debug("waf solver did not produce a token", "provider", pc.config.ID, "error", err.Error())
		return false
	}

	// Install the token cookie into the client jar.
	u, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	pc.client.Jar.SetCookies(u, []*http.Cookie{cookie})
	slog.Info("waf solver obtained aws-waf-token via JS engine", "provider", pc.config.ID)

	// Retry preflight with the fresh token.
	resp2, body2, err2 := doPreflightRequest(ctx, pc.client, pc.config.Auth)
	if err2 != nil || resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		return false
	}
	for k := range pc.authValues {
		delete(pc.authValues, k)
	}
	applyExtractions(pc.config.Auth.Extractions, resp2, body2, pc.authValues)
	return true
}

// tryBrowserEscapeHatch is Tier 4: open the preflight URL in the user's
// browser, wait for the cookie set to visibly change (meaning the WAF/JS
// challenge was solved), then retry preflight with the fresh cookies. Only
// fires when the caller has opted in both per-provider
// (AuthConfig.BrowserEscapeHatch) and per-call (WithInteractive context).
//
// When an ElicitConfirmFunc is present in the context (MCP sessions), the
// user is prompted before the browser opens — this replaces the old silent
// 15-second timeout that users never noticed.
func tryBrowserEscapeHatch(ctx context.Context, pc *providerClient) bool {
	targetURL := pc.config.Auth.PreflightURL
	browserPref := pc.config.Cookies.Browser

	// If elicitation is available, ask the user to confirm before opening
	// the browser. This turns a silent 15s timeout into an explicit user
	// action that actually succeeds.
	if elicit := getElicit(ctx); elicit != nil {
		msg := fmt.Sprintf(
			"%s needs a browser visit to refresh its WAF session. "+
				"I'll open %s in your browser — please complete any challenge "+
				"(CAPTCHA, cookie consent) and then confirm here.",
			pc.config.Name, targetURL,
		)
		confirmed, err := elicit(msg)
		if err != nil || !confirmed {
			slog.Info("browser escape hatch: user declined or elicitation failed",
				"provider", pc.config.ID)
			return false
		}
	}

	slog.Info("opening URL in browser to refresh WAF cookies, waiting up to 30s...",
		"provider", pc.config.ID,
		"url", targetURL,
		"browser", browserPref,
	)

	prev := browserCookiesForURL(targetURL)
	if err := openURLInBrowser(targetURL, browserPref); err != nil {
		slog.Warn("browser escape hatch: open failed",
			"provider", pc.config.ID, "error", err.Error())
		return false
	}

	// With elicitation the user explicitly confirmed they completed the
	// challenge, so extend the cookie-change wait to 30s. Without
	// elicitation, keep the original 15s.
	deadline := 15 * time.Second
	if getElicit(ctx) != nil {
		deadline = 30 * time.Second
	}

	fresh, changed := waitForFreshCookies(ctx, targetURL, prev, time.Second, deadline)
	if !changed {
		slog.Warn("browser escape hatch: no cookie change observed within deadline",
			"provider", pc.config.ID)
		return false
	}

	if pc.client == nil || pc.client.Jar == nil {
		return false
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	pc.client.Jar.SetCookies(u, fresh)

	resp2, body2, err2 := doPreflightRequest(ctx, pc.client, pc.config.Auth)
	if err2 != nil || resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		slog.Warn("browser escape hatch: preflight retry still failed",
			"provider", pc.config.ID)
		return false
	}
	for k := range pc.authValues {
		delete(pc.authValues, k)
	}
	applyExtractions(pc.config.Auth.Extractions, resp2, body2, pc.authValues)
	slog.Info("browser escape hatch: preflight recovered", "provider", pc.config.ID)
	return true
}

// doPreflightRequest issues the preflight request described by auth using
// the given client and returns the response plus body bytes. The caller does
// not need to close the body — it is consumed before returning.
func doPreflightRequest(ctx context.Context, client *http.Client, auth *AuthConfig) (*http.Response, []byte, error) {
	preflightBody := substituteEnvVars(auth.PreflightBody)

	method := auth.PreflightMethod
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if preflightBody != "" {
		bodyReader = strings.NewReader(preflightBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, auth.PreflightURL, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("preflight request: %w", err)
	}
	for k, v := range auth.PreflightHeaders {
		req.Header.Set(k, substituteEnvVars(v))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("preflight http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return resp, nil, fmt.Errorf("preflight read: %w", err)
	}
	return resp, body, nil
}

// applyExtractions runs each configured regex extraction against the response
// body or a named header, writing matches into authValues. Returns the number
// of extractions that matched.
func applyExtractions(extractions map[string]Extraction, resp *http.Response, body []byte, authValues map[string]string) int {
	matched := 0
	for name, extraction := range extractions {
		source := string(body)
		if extraction.Header != "" {
			source = resp.Header.Get(extraction.Header)
		}
		re, err := regexp.Compile(extraction.Pattern)
		if err != nil {
			slog.Warn("preflight regex compile failed", "name", name, "pattern", extraction.Pattern, "error", err.Error())
			continue
		}
		m := re.FindStringSubmatch(source)
		if len(m) >= 2 {
			varName := extraction.Variable
			if varName == "" {
				varName = name
			}
			authValues[varName] = m[1]
			matched++
		}
	}
	return matched
}

// needsBrowserCookieFallback reports whether the preflight outcome suggests a
// bot-detection block that browser cookies might bypass.
func needsBrowserCookieFallback(status, extracted int, extractions map[string]Extraction) bool {
	if status == http.StatusAccepted || status == http.StatusForbidden {
		return true
	}
	if len(extractions) > 0 && extracted == 0 {
		return true
	}
	return false
}

// applyBrowserCookies reads cookies from the user's browsers for the given
// URL and seeds them into the client's cookie jar. Returns true if any
// cookies were applied.
func applyBrowserCookies(client *http.Client, targetURL string) bool {
	if client == nil || client.Jar == nil {
		return false
	}
	cookies := browserCookiesForURL(targetURL)
	if len(cookies) == 0 {
		return false
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	client.Jar.SetCookies(u, cookies)
	slog.Debug("applied browser cookies to preflight client", "url", targetURL, "count", len(cookies))
	return true
}

// substituteVars replaces all ${var} placeholders in s with values from vars.
func substituteVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

// jsonPath walks a parsed JSON value using dot-notation.
// Supports nested objects and array indexing is not needed —
// arrays at the results level are returned as-is.
func jsonPath(data any, path string) any {
	if path == "" {
		return data
	}
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case []any:
			// If path segment applied to array, try each element
			// and return the first match. For results_path this
			// should not happen (arrays are the end).
			for _, elem := range v {
				if m, ok := elem.(map[string]any); ok {
					if val, exists := m[part]; exists {
						return val
					}
				}
			}
			return nil
		default:
			return nil
		}
	}
	return current
}

// mapHotelResult maps a raw JSON object to a HotelResult using field mappings.
func mapHotelResult(raw any, fields map[string]string) models.HotelResult {
	var h models.HotelResult
	for modelField, jsonField := range fields {
		val := jsonPath(raw, jsonField)
		if val == nil {
			continue
		}
		switch modelField {
		case "name":
			h.Name, _ = val.(string)
		case "hotel_id":
			h.HotelID = fmt.Sprintf("%v", val)
		case "rating":
			h.Rating = toFloat64(val)
		case "review_count":
			h.ReviewCount = toInt(val)
		case "stars":
			h.Stars = toInt(val)
		case "price":
			h.Price = toFloat64(val)
		case "currency":
			h.Currency, _ = val.(string)
		case "address":
			h.Address, _ = val.(string)
		case "lat":
			h.Lat = toFloat64(val)
		case "lon":
			h.Lon = toFloat64(val)
		case "booking_url":
			h.BookingURL, _ = val.(string)
		case "eco_certified":
			h.EcoCertified, _ = val.(bool)
		}
	}
	return h
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return f
		}
		// Strip currency symbols and whitespace (e.g. "€ 61" -> "61").
		cleaned := stripNonNumeric(n)
		if cleaned != "" {
			f, _ = strconv.ParseFloat(cleaned, 64)
			return f
		}
		return 0
	default:
		return 0
	}
}

// stripNonNumeric removes everything except digits, '.', and '-' from s.
// Used to extract a numeric value from currency-formatted strings like "€ 61".
func stripNonNumeric(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// substituteEnvVars replaces ${env.VAR_NAME} placeholders with values from
// the process environment. This allows provider configs to reference API keys
// stored in environment variables without hardcoding them.
func substituteEnvVars(s string) string {
	if !strings.Contains(s, "${env.") {
		return s
	}
	// Find all ${env.XXX} patterns and replace.
	for {
		start := strings.Index(s, "${env.")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			break
		}
		varName := s[start+6 : start+end] // skip "${env." prefix
		envVal := os.Getenv(varName)
		s = s[:start] + envVal + s[start+end+1:]
	}
	return s
}
