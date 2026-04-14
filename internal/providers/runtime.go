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
		jar, _ := cookiejar.New(nil)
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		}
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

// SearchHotels queries all hotel-category providers and returns combined results.
func (rt *Runtime) SearchHotels(ctx context.Context, location string, lat, lon float64, checkin, checkout, currency string, guests int) ([]models.HotelResult, error) {
	providers := rt.registry.ListByCategory("hotel")
	if len(providers) == 0 {
		return nil, nil
	}

	type result struct {
		hotels []models.HotelResult
		err    error
		id     string
	}

	results := make(chan result, len(providers))
	var wg sync.WaitGroup

	for _, cfg := range providers {
		wg.Add(1)
		go func(cfg *ProviderConfig) {
			defer wg.Done()
			hotels, err := rt.searchProvider(ctx, cfg, location, lat, lon, checkin, checkout, currency, guests)
			results <- result{hotels: hotels, err: err, id: cfg.ID}
		}(cfg)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var combined []models.HotelResult
	var firstErr error
	for r := range results {
		if r.err != nil {
			slog.Warn("provider error", "provider", r.id, "error", r.err.Error())
			rt.registry.MarkError(r.id, r.err.Error())
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		rt.registry.MarkSuccess(r.id)
		combined = append(combined, r.hotels...)
	}

	if len(combined) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return combined, nil
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

	// Map each element to HotelResult.
	hotels := make([]models.HotelResult, 0, len(arr))
	for _, item := range arr {
		h := mapHotelResult(item, cfg.ResponseMapping.Fields)
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

	// Substitute environment variables in preflight body (e.g. ${env.BOOKING_API_KEY}).
	preflightBody := pc.config.Auth.PreflightBody
	preflightBody = substituteEnvVars(preflightBody)

	method := pc.config.Auth.PreflightMethod
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if preflightBody != "" {
		bodyReader = strings.NewReader(preflightBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, pc.config.Auth.PreflightURL, bodyReader)
	if err != nil {
		return fmt.Errorf("preflight request: %w", err)
	}
	for k, v := range pc.config.Auth.PreflightHeaders {
		req.Header.Set(k, substituteEnvVars(v))
	}

	resp, err := pc.client.Do(req)
	if err != nil {
		return fmt.Errorf("preflight http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("preflight read: %w", err)
	}

	for name, extraction := range pc.config.Auth.Extractions {
		source := string(body)
		if extraction.Header != "" {
			source = resp.Header.Get(extraction.Header)
		}
		re, err := regexp.Compile(extraction.Pattern)
		if err != nil {
			slog.Warn("preflight regex compile failed", "name", name, "pattern", extraction.Pattern, "error", err.Error())
			continue
		}
		matches := re.FindStringSubmatch(source)
		if len(matches) >= 2 {
			varName := extraction.Variable
			if varName == "" {
				varName = name
			}
			pc.authValues[varName] = matches[1]
		}
	}

	pc.authExpiry = time.Now().Add(authCacheDuration)
	return nil
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
}

// TestProvider runs a single search against the given provider config and
// returns structured diagnostics showing which step succeeded or failed.
func TestProvider(ctx context.Context, cfg *ProviderConfig, location string, lat, lon float64, checkin, checkout, currency string, guests int) *TestResult {
	result := &TestResult{Step: "init"}

	// Create a fresh client for testing.
	var httpClient *http.Client
	if cfg.TLS.Fingerprint == "chrome" {
		httpClient = batchexec.ChromeHTTPClient()
	} else {
		jar, _ := cookiejar.New(nil)
		httpClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
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

	// Step 1: Preflight auth.
	if cfg.Auth != nil && cfg.Auth.Type == "preflight" {
		result.Step = "preflight"

		if cfg.Auth.PreflightURL == "" {
			result.Error = "preflight: preflight_url is empty"
			return result
		}

		preflightBody := substituteEnvVars(cfg.Auth.PreflightBody)
		method := cfg.Auth.PreflightMethod
		if method == "" {
			method = "GET"
		}

		var bodyReader io.Reader
		if preflightBody != "" {
			bodyReader = strings.NewReader(preflightBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, cfg.Auth.PreflightURL, bodyReader)
		if err != nil {
			result.Error = fmt.Sprintf("preflight: create request: %v", err)
			return result
		}
		for k, v := range cfg.Auth.PreflightHeaders {
			req.Header.Set(k, substituteEnvVars(v))
		}

		resp, err := pc.client.Do(req)
		if err != nil {
			result.Error = fmt.Sprintf("preflight: http: %v", err)
			return result
		}
		defer resp.Body.Close()

		result.HTTPStatus = resp.StatusCode

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		if err != nil {
			result.Error = fmt.Sprintf("preflight: read body: %v", err)
			return result
		}

		// Capture body snippet.
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		result.BodySnippet = snippet

		// Run extractions.
		result.Step = "auth_extraction"
		result.ExtractionResults = make(map[string]string)
		for name, extraction := range cfg.Auth.Extractions {
			source := string(body)
			if extraction.Header != "" {
				source = resp.Header.Get(extraction.Header)
			}
			re, err := regexp.Compile(extraction.Pattern)
			if err != nil {
				result.ExtractionResults[name] = fmt.Sprintf("regex error: %v", err)
				continue
			}
			matches := re.FindStringSubmatch(source)
			if len(matches) >= 2 {
				varName := extraction.Variable
				if varName == "" {
					varName = name
				}
				pc.authValues[varName] = matches[1]
				result.ExtractionResults[name] = "ok (extracted " + strconv.Itoa(len(matches[1])) + " chars)"
			} else {
				result.ExtractionResults[name] = "no match"
			}
		}

		// Check if any extraction failed.
		for name, v := range result.ExtractionResults {
			if v != "ok" && !strings.HasPrefix(v, "ok ") {
				result.Error = fmt.Sprintf("auth_extraction: %s: %s", name, v)
				return result
			}
		}

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

	// Add auth-extracted variables.
	for k, v := range pc.authValues {
		vars["${"+k+"}"] = v
	}

	endpoint := substituteVars(cfg.Endpoint, vars)

	if len(cfg.QueryParams) > 0 {
		u, err := url.Parse(endpoint)
		if err != nil {
			result.Error = fmt.Sprintf("request: parse endpoint: %v", err)
			return result
		}
		q := u.Query()
		for k, v := range cfg.QueryParams {
			q.Set(k, substituteVars(v, vars))
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

	resultsRaw := jsonPath(raw, cfg.ResponseMapping.ResultsPath)
	arr, ok := resultsRaw.([]any)
	if !ok {
		result.Error = fmt.Sprintf("response_parse: results_path %q did not resolve to an array", cfg.ResponseMapping.ResultsPath)
		return result
	}

	result.ResultsCount = len(arr)

	// Step 4: Field mapping.
	result.Step = "field_mapping"

	if len(arr) > 0 {
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
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}
