package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/waf"
	"golang.org/x/time/rate"
)

// TestResult captures step-by-step diagnostics from a provider test.
type TestResult struct {
	Success           bool              `json:"success"`
	Step              string            `json:"step"`
	HTTPStatus        int               `json:"http_status,omitempty"`
	ResultsCount      int               `json:"results_count,omitempty"`
	Error             string            `json:"error,omitempty"`
	ExtractionResults map[string]string `json:"extraction_results,omitempty"`
	BodySnippet       string            `json:"body_snippet,omitempty"`
	SampleResult      map[string]any    `json:"sample_result,omitempty"`
	// AuthTier records which tier of the preflight cascade ultimately
	// succeeded: "direct" (Tier 1), "browser-cookies" (Tier 3), or
	// "browser-escape-hatch" (Tier 4). Empty if preflight was not run.
	AuthTier string `json:"auth_tier,omitempty"`
	// Suggestions contains auto-detected corrections when the config almost
	// works (e.g. HTTP 200 but wrong results_path or field mapping).
	Suggestions map[string]string `json:"suggestions,omitempty"`
}

// TestProvider runs a single search against the given provider config and
// returns structured diagnostics showing which step succeeded or failed.
func TestProvider(ctx context.Context, cfg *ProviderConfig, location string, lat, lon float64, checkin, checkout, currency string, guests int) *TestResult {
	result := &TestResult{Step: "init"}

	// Create a fresh client for testing. Always attach a cookie jar so the
	// browser-cookie fallback can seed session cookies when preflight hits
	// a JS bot-detection challenge.
	var httpClient *http.Client
	if cfg.TLS.Fingerprint == "chrome" {
		httpClient = batchexec.ChromeHTTPClient()
	} else {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if httpClient.Jar == nil {
		jar, _ := cookiejar.New(nil)
		httpClient.Jar = jar
	}

	rps := cfg.RateLimit.RequestsPerSecond
	if rps <= 0 {
		rps = 10 // generous for testing
	}

	pc := &providerClient{
		config:     cfg,
		client:     httpClient,
		limiter:    rate.NewLimiter(rate.Limit(rps), 1),
		authValues: make(map[string]string),
	}

	// Seed browser cookies unconditionally when configured, same as
	// searchProvider — carries JS sensor cookies for bot bypass.
	if cfg.Cookies.Source == "browser" {
		targetURL := cfg.Endpoint
		if cfg.Auth != nil && cfg.Auth.PreflightURL != "" {
			targetURL = cfg.Auth.PreflightURL
		}
		applyBrowserCookies(pc.client, targetURL)
	}

	// Step 1: Preflight auth.
	if cfg.Auth != nil && cfg.Auth.Type == "preflight" {
		result.Step = "preflight"

		if cfg.Auth.PreflightURL == "" {
			result.Error = "preflight: preflight_url is empty"
			return result
		}

		resp, body, err := doPreflightRequest(ctx, pc.client, cfg.Auth)
		if err != nil {
			result.Error = fmt.Sprintf("preflight: %v", err)
			return result
		}
		result.HTTPStatus = resp.StatusCode

		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		result.BodySnippet = snippet

		// Run extractions (attempt 1).
		result.Step = "auth_extraction"
		matched := applyExtractions(cfg.Auth.Extractions, resp, body, pc.authValues)
		matched += applyURLExtractions(ctx, pc.client, cfg.Auth.Extractions, pc.authValues)
		tier := "direct"

		// Fallback cascade: Tier 3 (browser cookies) then Tier 4
		// (escape hatch — open URL in browser and wait for fresh cookies).
		if needsBrowserCookieFallback(resp.StatusCode, matched, cfg.Auth.Extractions) {
			tier = ""
			if applied := applyBrowserCookies(pc.client, cfg.Auth.PreflightURL); applied {
				resp2, body2, err2 := doPreflightRequest(ctx, pc.client, cfg.Auth)
				if err2 == nil && resp2.StatusCode >= 200 && resp2.StatusCode < 300 && !isAkamaiChallenge(resp2.StatusCode, body2) {
					resp, body = resp2, body2
					result.HTTPStatus = resp.StatusCode
					snippet = string(body)
					if len(snippet) > 500 {
						snippet = snippet[:500]
					}
					result.BodySnippet = snippet
					for k := range pc.authValues {
						delete(pc.authValues, k)
					}
					matched = applyExtractions(cfg.Auth.Extractions, resp, body, pc.authValues)
					matched += applyURLExtractions(ctx, pc.client, cfg.Auth.Extractions, pc.authValues)
					tier = "browser-cookies"
				}
			}

			// Tier 3b: WAF JS solver (sobek).
			if tier == "" && (resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusForbidden) {
				cookie, wafErr := waf.SolveAWSWAF(ctx, pc.client, cfg.Auth.PreflightURL, string(body), nil)
				if wafErr == nil && cookie != nil {
					u, _ := url.Parse(cfg.Auth.PreflightURL)
					pc.client.Jar.SetCookies(u, []*http.Cookie{cookie})
					resp2, body2, err2 := doPreflightRequest(ctx, pc.client, cfg.Auth)
					if err2 == nil && resp2.StatusCode >= 200 && resp2.StatusCode < 300 && !isAkamaiChallenge(resp2.StatusCode, body2) {
						resp, body = resp2, body2
						result.HTTPStatus = resp.StatusCode
						snippet = string(body)
						if len(snippet) > 500 {
							snippet = snippet[:500]
						}
						result.BodySnippet = snippet
						for k := range pc.authValues {
							delete(pc.authValues, k)
						}
						matched = applyExtractions(cfg.Auth.Extractions, resp, body, pc.authValues)
						matched += applyURLExtractions(ctx, pc.client, cfg.Auth.Extractions, pc.authValues)
						tier = "waf-solver"
					}
				} else if wafErr != nil {
					slog.Debug("waf solver did not produce a token in test", "error", wafErr.Error())
				}
			}

			// Tier 4: only if the provider opted in and the caller marked
			// the context interactive. Non-interactive callers (this test
			// harness by default) never spawn a browser.
			if tier == "" && cfg.Auth.BrowserEscapeHatch && isInteractive(ctx) {
				if tryBrowserEscapeHatch(ctx, pc, cfg.Auth) {
					// tryBrowserEscapeHatch already wrote fresh values into
					// pc.authValues; re-issue preflight once more here only
					// to capture the body for diagnostics.
					resp2, body2, err2 := doPreflightRequest(ctx, pc.client, cfg.Auth)
					if err2 == nil && resp2.StatusCode >= 200 && resp2.StatusCode < 300 && !isAkamaiChallenge(resp2.StatusCode, body2) {
						resp, body = resp2, body2
						result.HTTPStatus = resp.StatusCode
						snippet = string(body)
						if len(snippet) > 500 {
							snippet = snippet[:500]
						}
						result.BodySnippet = snippet
					}
					matched = len(pc.authValues)
					tier = "browser-escape-hatch"
				}
			}
		}
		result.AuthTier = tier

		// Build the diagnostic report.
		result.ExtractionResults = make(map[string]string)
		for name, extraction := range cfg.Auth.Extractions {
			varName := extraction.Variable
			if varName == "" {
				varName = name
			}
			if v, ok := pc.authValues[varName]; ok {
				suffix := ""
				switch tier {
				case "browser-cookies":
					suffix = " [via browser cookies]"
				case "waf-solver":
					suffix = " [via WAF JS solver]"
				case "browser-escape-hatch":
					suffix = " [via browser escape hatch]"
				}
				result.ExtractionResults[name] = "ok (extracted " + strconv.Itoa(len(v)) + " chars)" + suffix
			} else {
				// Detect regex compile errors vs. plain no-match.
				if _, err := regexp.Compile(extraction.Pattern); err != nil {
					result.ExtractionResults[name] = fmt.Sprintf("regex error: %v", err)
				} else {
					result.ExtractionResults[name] = "no match"
				}
			}
		}

		// Check if any extraction failed.
		for name, v := range result.ExtractionResults {
			if !strings.HasPrefix(v, "ok") {
				result.Error = fmt.Sprintf("auth_extraction: %s: %s", name, v)
				return result
			}
		}
		_ = matched

		pc.authExpiry = time.Now().Add(authCacheDuration)
	}

	// Step 2: Build and send search request.
	result.Step = "request"

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

	// Resolve provider-specific city ID when the config provides a lookup
	// table (same behaviour as searchProvider so test_provider faithfully
	// mirrors live search URL construction).
	if id := resolveCityID(cfg.CityLookup, location); id != "" {
		vars["${city_id}"] = id
	}

	// Note: TestProvider does not receive filter params — it uses a fixed
	// set of test variables. Filter variable substitution is exercised via
	// the live searchProvider path and its unit tests.

	// Add auth-extracted variables.
	for k, v := range pc.authValues {
		vars["${"+k+"}"] = v
	}

	endpoint := stripUnresolvedPlaceholders(substituteVars(cfg.Endpoint, vars))

	if len(cfg.QueryParams) > 0 {
		u, err := url.Parse(endpoint)
		if err != nil {
			result.Error = fmt.Sprintf("request: parse endpoint: %v", err)
			return result
		}
		q := u.Query()
		for k, v := range cfg.QueryParams {
			resolved := substituteVars(v, vars)
			if strings.Contains(resolved, "${") {
				continue // skip unresolved optional filter vars
			}
			q.Set(k, resolved)
		}
		u.RawQuery = q.Encode()
		endpoint = u.String()
	}

	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	var bodyReader io.Reader
	if method == "POST" && cfg.BodyTemplate != "" {
		bodyReader = strings.NewReader(substituteVars(cfg.BodyTemplate, vars))
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		result.Error = fmt.Sprintf("request: create: %v", err)
		return result
	}

	for k, v := range cfg.Headers {
		req.Header.Set(k, substituteEnvVars(substituteVars(v, vars)))
	}

	resp, err := pc.client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("request: http: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.HTTPStatus = resp.StatusCode

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		result.Error = fmt.Sprintf("request: read body: %v", err)
		return result
	}

	// Detect Akamai/WAF challenge pages that use HTTP 202 (which is in the
	// 2xx success range but is actually an interstitial challenge page).
	if isAkamaiChallenge(resp.StatusCode, body) {
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		result.BodySnippet = snippet
		result.Error = fmt.Sprintf("request: http %d WAF/JS challenge page detected — provider needs browser cookie refresh", resp.StatusCode)
		return result
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		result.BodySnippet = snippet
		result.Error = fmt.Sprintf("request: http %d", resp.StatusCode)
		return result
	}

	// Step 3: Parse JSON response.
	result.Step = "response_parse"

	// If the provider embeds its API response inside an HTML body (SSR'd
	// Apollo cache etc.), apply the configured regex to pull the JSON blob
	// out first. Capture group 1 replaces `body` for JSON parsing.
	if pattern := cfg.ResponseMapping.BodyExtractPattern; pattern != "" {
		re, reErr := regexp.Compile(pattern)
		if reErr != nil {
			result.Error = fmt.Sprintf("response_parse: compile body_extract_pattern: %v", reErr)
			return result
		}
		m := re.FindSubmatch(body)
		if len(m) < 2 {
			snippet := string(body)
			if len(snippet) > 500 {
				snippet = snippet[:500]
			}
			result.BodySnippet = snippet
			result.Error = fmt.Sprintf("response_parse: body_extract_pattern %q did not match response body", pattern)
			return result
		}
		body = m[1]
	}

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		result.BodySnippet = snippet
		result.Error = fmt.Sprintf("response_parse: %v", err)
		return result
	}

	// Denormalize Apollo cache if detected (SSR providers like Booking.com).
	// Only denormalize ROOT_QUERY subtree to avoid seen-set poisoning.
	if cache, isMap := raw.(map[string]any); isMap {
		if rootQuery, hasRoot := cache["ROOT_QUERY"]; hasRoot {
			cache["ROOT_QUERY"] = denormalizeApollo(rootQuery, cache, nil)
		}
	}


	// Surface GraphQL-style {"errors":[...]} responses before complaining
	// about results_path — this makes stale persistedQuery hashes and WAF
	// denials diagnosable at a glance instead of hiding behind a generic
	// array-resolution failure.
	if topObj, isMap := raw.(map[string]any); isMap {
		if errs, hasErrs := topObj["errors"].([]any); hasErrs && len(errs) > 0 {
			if first, _ := errs[0].(map[string]any); first != nil {
				msg, _ := first["message"].(string)
				code := ""
				if ext, _ := first["extensions"].(map[string]any); ext != nil {
					code, _ = ext["code"].(string)
				}
				detail := msg
				if code != "" {
					detail = detail + " [" + code + "]"
				}
				if detail == "" {
					detail = "unknown graphql error"
				}
				// Keep a snippet of the full response body so the LLM can
				// inspect the extensions/data fields beyond the first error.
				snippet := string(body)
				if len(snippet) > 500 {
					snippet = snippet[:500]
				}
				result.BodySnippet = snippet
				result.Error = "response_parse: graphql error: " + detail
				return result
			}
		}
	}

	resultsRaw := jsonPath(raw, cfg.ResponseMapping.ResultsPath)
	arr, ok := resultsRaw.([]any)
	if !ok {
		result.Error = fmt.Sprintf("response_parse: results_path %q did not resolve to an array", cfg.ResponseMapping.ResultsPath)
		// Include a snippet of the actual API response so the LLM can see
		// what came back instead of guessing.
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		result.BodySnippet = snippet
		result.Suggestions = discoverArrayPaths(raw, "")
		if len(result.Suggestions) > 0 {
			result.Error += ". See suggestions for detected arrays."
		}
		return result
	}

	result.ResultsCount = len(arr)

	if len(arr) == 0 {
		// HTTP 200 + valid JSON but empty array — the path resolved but had
		// no results. Might be a legitimate empty search or a wrong path.
		sug := discoverArrayPaths(raw, cfg.ResponseMapping.ResultsPath)
		if len(sug) > 0 {
			result.Suggestions = sug
		}
	}

	// Step 4: Field mapping.
	result.Step = "field_mapping"

	if len(arr) > 0 {
		// Auto-suggest field names from the first result object.
		if obj, isMap := arr[0].(map[string]any); isMap {
			fieldSug := discoverFieldMappings(obj, "")
			if len(fieldSug) > 0 {
				if result.Suggestions == nil {
					result.Suggestions = make(map[string]string)
				}
				for k, v := range fieldSug {
					result.Suggestions[k] = v
				}
			}
		}
		// Map the first result as a sample.
		h := mapHotelResult(arr[0], cfg.ResponseMapping.Fields)
		sample := map[string]any{
			"name":     h.Name,
			"hotel_id": h.HotelID,
			"rating":   h.Rating,
			"price":    h.Price,
			"currency": h.Currency,
			"lat":      h.Lat,
			"lon":      h.Lon,
		}
		if h.Address != "" {
			sample["address"] = h.Address
		}
		result.SampleResult = sample
	}

	result.Step = "complete"
	result.Success = true
	return result
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		i, err := strconv.Atoi(n)
		if err == nil {
			return i
		}
		// Try the last integer in composite strings like "4.84 (25)" -> 25.
		if tok := lastIntToken(n); tok != "" {
			i, _ = strconv.Atoi(tok)
			return i
		}
		return 0
	default:
		return 0
	}
}
