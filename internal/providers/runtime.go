// Package providers accesses third-party provider APIs on behalf of the
// local user for personal, noncommercial travel search. It is licensed
// under PolyForm Noncommercial 1.0.0 (see LICENSE at repo root).
// Commercial use, redistribution of scraped data, or operation as a
// service is prohibited by this license.
//
// Rate limits are intentionally conservative (0.5 req/s default per
// provider) to make request patterns behaviorally indistinguishable
// from manual human browsing. Cookie persistence is capped at 24h.
// Per-provider browser escape hatches require explicit opt-in via
// AuthConfig.BrowserEscapeHatch AND WithInteractive context.
package providers

import (
	"bytes"
	"compress/gzip"
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
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/waf"
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/time/rate"
)

const (
	defaultRPS         = 0.5
	defaultBurst       = 1
	authCacheDuration  = 10 * time.Minute
	boundingBoxOffset  = 0.15
	maxResponseBytes   = 10 * 1024 * 1024 // 10 MB
)

// HotelFilterParams carries search filter values that should be passed through
// to external provider URL templates and query parameters via ${var} substitution.
type HotelFilterParams struct {
	MinPrice         float64
	MaxPrice         float64
	PropertyType     string   // normalized: "hotel", "apartment", "hostel", etc.
	Sort             string   // "price", "rating", "distance", "stars"
	Stars            int      // minimum star rating, 0 = no filter
	MinRating        float64  // minimum guest rating, 0 = no filter
	Amenities        []string // required amenities
	FreeCancellation bool

	// Extended filters — wired to providers that support them.
	MinBedrooms      int    // minimum bedrooms (Airbnb)
	MinBathrooms     int    // minimum bathrooms (Airbnb)
	MinBeds          int    // minimum beds (Airbnb)
	RoomType         string // "entire_home", "private_room", "shared_room" (Airbnb)
	Superhost        bool   // Superhost-only filter (Airbnb)
	InstantBook      bool   // instant-bookable only (Airbnb)
	MaxDistanceM     int    // max distance from center in meters (Booking)
	Sustainable      bool   // eco/sustainable properties (Booking)
	MealPlan         bool   // breakfast/meal included (Booking)
	IncludeSoldOut   bool   // include sold-out properties in results (Booking)
}

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
	// lastPreflightURL tracks the fully-resolved preflight URL used for the
	// current auth cache entry. When the preflight URL contains ${city_id} or
	// other search-specific vars, switching cities produces a different URL
	// and the auth cache must be invalidated — WAF cookies obtained for one
	// dest_id are rejected for a different one.
	lastPreflightURL string
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
	if cfg.TLS.Fingerprint == "chrome" && cfg.Cookies.Source != "browser" {
		// Use fhttp-based client that sends Chrome-like HTTP/2 SETTINGS,
		// WINDOW_UPDATE, and PRIORITY frames. Combined with utls Chrome146
		// TLS fingerprint, this makes requests indistinguishable from Chrome
		// at both the TLS and HTTP/2 layers — bypassing Akamai bot detection
		// that flags Go's x/net/http2 framing as "b_bot".
		//
		// When cookies.source is "browser", the real browser session cookies
		// already authenticate the request and the standard Go TLS transport
		// produces better results — some providers (Booking.com) SSR fewer
		// results through the fhttp/utls pipeline despite identical cookies,
		// likely due to subtle HTTP/2 framing differences that trigger a
		// different server-side rendering path.
		httpClient = newChromeH2Client()
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
// the LLM for autonomous diagnosis. The optional filters parameter passes
// search filters (price, property type, stars, etc.) through to provider URL
// templates. A nil filters value is safe and means no filter vars are set.
func (rt *Runtime) SearchHotels(ctx context.Context, location string, lat, lon float64, checkin, checkout, currency string, guests int, filters *HotelFilterParams) ([]models.HotelResult, []models.ProviderStatus, error) {
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
			hotels, err := rt.searchProvider(ctx, cfg, location, lat, lon, checkin, checkout, currency, guests, filters)
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

func (rt *Runtime) searchProvider(ctx context.Context, cfg *ProviderConfig, location string, lat, lon float64, checkin, checkout, currency string, guests int, filters *HotelFilterParams) ([]models.HotelResult, error) {
	// Pick up on-disk edits without an MCP restart. If the file mtime has
	// advanced since we last parsed it, ReloadIfChanged swaps in the fresh
	// config; we then drop the cached providerClient so its HTTP client,
	// rate limiter and auth cache are rebuilt from the new config.
	var oldJar http.CookieJar
	if fresh := rt.registry.ReloadIfChanged(cfg.ID); fresh != nil && fresh != cfg {
		// Preserve the cookie jar so WAF tokens and session cookies survive
		// config reloads. The jar is installed on the new client below.
		rt.mu.Lock()
		if old := rt.clients[cfg.ID]; old != nil && old.client != nil {
			oldJar = old.client.Jar
		}
		delete(rt.clients, cfg.ID)
		rt.mu.Unlock()
		cfg = fresh
	}
	pc := rt.getOrCreateClient(cfg)
	if oldJar != nil && pc.client != nil {
		pc.client.Jar = oldJar
	}

	// Rate limit.
	if err := pc.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	// Build variable map early — the preflight URL may contain ${city_id}
	// or other search-specific placeholders that must be resolved before
	// the preflight request fires. Without this, Booking's WAF rejects
	// requests because cookies obtained for one dest_id (e.g. Paris) are
	// tied to that city and fail when the actual search targets another.
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

	// Resolve provider-specific city ID. First check the static lookup
	// table; if not found, fall back to the dynamic city_resolver API.
	if id := resolveCityID(cfg.CityLookup, location); id != "" {
		vars["${city_id}"] = id
	} else if cfg.CityResolver != nil {
		if id, err := resolveCityIDDynamic(ctx, cfg, pc.client, location, rt.registry); err != nil {
			slog.Warn("city_resolver failed, continuing without city_id",
				"provider", cfg.ID, "location", location, "error", err.Error())
		} else {
			vars["${city_id}"] = id
		}
	}

	// When cookies.source is "browser", unconditionally seed the client's
	// cookie jar with the user's real browser cookies BEFORE preflight.
	// This carries JS-written sensor cookies (Akamai bm_sz, PerimeterX
	// _pxhd) that bot-detection systems validate server-side. Without
	// them, providers like Booking.com classify the request as b_bot and
	// strip review scores from the SSR response.
	browserCookiesApplied := false
	if cfg.Cookies.Source == "browser" {
		endpointURL := cfg.Endpoint
		if cfg.Auth != nil && cfg.Auth.PreflightURL != "" {
			endpointURL = substituteVars(cfg.Auth.PreflightURL, vars)
		}
		browserCookiesApplied = applyBrowserCookies(pc.client, endpointURL, cfg.Cookies.Browser)
	}

	// Preflight auth if needed. The preflight URL is resolved with
	// search-specific vars so that ${city_id} etc. produce a city-specific
	// WAF session rather than reusing a hardcoded one.
	//
	// When browser cookies were successfully loaded AND the auth config has
	// no extractions (i.e. preflight's only purpose is cookie seeding), skip
	// the preflight entirely. Running preflight with a non-fingerprinted HTTP
	// client causes the server to set new session cookies (via Set-Cookie) that
	// overwrite the browser's authenticated cookies in the jar — replacing a
	// real-user session with a bot-classified one. This is the root cause of
	// Booking.com returning 0 results despite having valid browser cookies.
	if cfg.Auth != nil && cfg.Auth.Type == "preflight" {
		skipPreflight := browserCookiesApplied && len(cfg.Auth.Extractions) == 0
		if skipPreflight {
			slog.Info("skipping preflight: browser cookies already loaded, no extractions needed",
				"provider", cfg.ID)
		} else if err := rt.runPreflight(ctx, pc, vars); err != nil {
			return nil, fmt.Errorf("preflight: %w", err)
		}
	}

	// Add filter variables when provided. These allow provider URL
	// templates and query params to reference ${min_price}, ${max_price},
	// ${property_type}, ${sort}, ${stars}, ${min_rating}, ${amenities},
	// and ${free_cancellation}.
	if filters != nil {
		if filters.MinPrice > 0 {
			vars["${min_price}"] = strconv.FormatFloat(filters.MinPrice, 'f', -1, 64)
		}
		if filters.MaxPrice > 0 {
			vars["${max_price}"] = strconv.FormatFloat(filters.MaxPrice, 'f', -1, 64)
		}
		if filters.PropertyType != "" {
			// Resolve to provider-specific ID if a lookup table exists.
			if resolved := resolvePropertyType(cfg.PropertyTypeLookup, filters.PropertyType); resolved != "" {
				vars["${property_type}"] = resolved
			} else {
				vars["${property_type}"] = filters.PropertyType
			}
		}
		if filters.Sort != "" {
			if resolved, ok := cfg.SortLookup[strings.ToLower(filters.Sort)]; ok && resolved != "" {
				vars["${sort}"] = resolved
			} else {
				vars["${sort}"] = filters.Sort
			}
		}
		if filters.Stars > 0 {
			vars["${stars}"] = strconv.Itoa(filters.Stars)
		}
		if filters.MinRating > 0 {
			vars["${min_rating}"] = strconv.FormatFloat(filters.MinRating, 'f', 1, 64)
		}
		if len(filters.Amenities) > 0 {
			vars["${amenities}"] = strings.Join(filters.Amenities, ",")
			// Resolve amenity names to provider-specific IDs.
			if len(cfg.AmenityLookup) > 0 {
				var resolved []string
				for _, a := range filters.Amenities {
					if id, ok := cfg.AmenityLookup[strings.ToLower(a)]; ok && id != "" {
						resolved = append(resolved, id)
					}
				}
				if len(resolved) > 0 {
					vars["${amenity_ids}"] = strings.Join(resolved, ",")
				}
			}
		}
		if filters.FreeCancellation {
			vars["${free_cancellation}"] = "1"
			vars["${flexible_cancellation}"] = "true"
		}
		// Build composite price_range var for providers like Booking that
		// encode price filters as "currency-min-max-1" (e.g. "EUR-50-200-1").
		if filters.MinPrice > 0 || filters.MaxPrice > 0 {
			minS := "0"
			maxS := "9999"
			if filters.MinPrice > 0 {
				minS = strconv.FormatFloat(filters.MinPrice, 'f', 0, 64)
			}
			if filters.MaxPrice > 0 {
				maxS = strconv.FormatFloat(filters.MaxPrice, 'f', 0, 64)
			}
			vars["${price_range}"] = currency + "-" + minS + "-" + maxS + "-1"
		}

		// Extended filter vars.
		if filters.MinBedrooms > 0 {
			vars["${min_bedrooms}"] = strconv.Itoa(filters.MinBedrooms)
		}
		if filters.MinBathrooms > 0 {
			vars["${min_bathrooms}"] = strconv.Itoa(filters.MinBathrooms)
		}
		if filters.MinBeds > 0 {
			vars["${min_beds}"] = strconv.Itoa(filters.MinBeds)
		}
		if filters.RoomType != "" {
			// Map canonical names to Airbnb room_types[] values.
			switch strings.ToLower(filters.RoomType) {
			case "entire_home", "entire home", "entire":
				vars["${room_type}"] = "Entire home/apt"
			case "private_room", "private room", "private":
				vars["${room_type}"] = "Private room"
			case "shared_room", "shared room", "shared":
				vars["${room_type}"] = "Shared room"
			case "hotel_room", "hotel room", "hotel":
				vars["${room_type}"] = "Hotel room"
			default:
				vars["${room_type}"] = filters.RoomType
			}
		}
		if filters.Superhost {
			vars["${superhost}"] = "true"
		}
		if filters.InstantBook {
			vars["${instant_book}"] = "true"
		}
		if filters.MaxDistanceM > 0 {
			vars["${max_distance_m}"] = strconv.Itoa(filters.MaxDistanceM)
		}
		if filters.Sustainable {
			vars["${sustainable}"] = "1"
		}
		if filters.MealPlan {
			vars["${meal_plan}"] = "1"
		}
		if filters.IncludeSoldOut {
			vars["${include_sold_out}"] = "1"
		}
	}

	// Build composite filter parameters (e.g. Booking's nflt) from
	// individual filter vars. Only active (non-empty) parts are joined.
	if fc := cfg.FilterComposite; fc != nil && fc.TargetVar != "" {
		var parts []string
		for filterVar, prefix := range fc.Parts {
			if val := vars["${"+filterVar+"}"]; val != "" {
				// Apply scale if defined (e.g. min_rating × 10 for Booking's 0-100 scale).
				if scale, hasScale := fc.Scales[filterVar]; hasScale && scale != 0 {
					if f, err := strconv.ParseFloat(val, 64); err == nil {
						val = strconv.Itoa(int(f * scale))
					}
				}
				// Multi-value support: if the value contains commas (e.g.
				// amenity_ids "107,433"), expand to separate prefix+id parts
				// so Booking gets hotelfacility%3D107%3Bhotelfacility%3D433.
				if strings.Contains(val, ",") {
					for _, sub := range strings.Split(val, ",") {
						sub = strings.TrimSpace(sub)
						if sub != "" {
							parts = append(parts, prefix+sub)
						}
					}
				} else {
					parts = append(parts, prefix+val)
				}
			}
		}
		vars["${"+fc.TargetVar+"}"] = strings.Join(parts, fc.Separator)
	}

	// Add auth-extracted variables.
	pc.authMu.RLock()
	for k, v := range pc.authValues {
		vars["${"+k+"}"] = v
	}
	pc.authMu.RUnlock()

	// Build endpoint URL. After substitution, strip any remaining ${...}
	// placeholders and their preceding &/? separators so optional filter
	// params that weren't set don't produce malformed URLs (e.g.
	// "&nflt=${nflt}" → removed entirely when no filters are active).
	endpoint := substituteVars(cfg.Endpoint, vars)
	endpoint = stripUnresolvedPlaceholders(endpoint)

	// Build query params.
	if len(cfg.QueryParams) > 0 {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("parse endpoint: %w", err)
		}
		q := u.Query()
		for k, v := range cfg.QueryParams {
			resolved := substituteVars(v, vars)
			// Skip query params whose value still contains an unresolved
			// ${placeholder} — this happens when an optional filter (e.g.
			// ${property_type}, ${min_price}) was not set by the caller.
			// Sending a literal "${property_type}" as a query value would
			// confuse the provider's API.
			if strings.Contains(resolved, "${") {
				continue
			}
			// Array params (e.g. "amenities[]"): if the key ends in [] and
			// the value contains commas, add each value as a separate param
			// so Airbnb gets amenities[]=4&amenities[]=7 instead of amenities[]=4,7.
			if strings.HasSuffix(k, "[]") && strings.Contains(resolved, ",") {
				for _, sub := range strings.Split(resolved, ",") {
					sub = strings.TrimSpace(sub)
					if sub != "" {
						q.Add(k, sub)
					}
				}
				continue
			}
			q.Set(k, resolved)
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

	// Add headers in deterministic order when header_order is configured.
	// WAF/bot-detection systems (Booking.com, Akamai) fingerprint header
	// ordering. Go's map iteration is random, so without explicit ordering
	// every request has a different header sequence — a bot fingerprint.
	if len(cfg.HeaderOrder) > 0 {
		added := make(map[string]bool, len(cfg.HeaderOrder))
		for _, k := range cfg.HeaderOrder {
			if v, ok := cfg.Headers[k]; ok {
				req.Header.Set(k, substituteEnvVars(substituteVars(v, vars)))
				added[k] = true
			}
		}
		// Append any headers not listed in the order (safety net).
		for k, v := range cfg.Headers {
			if !added[k] {
				req.Header.Set(k, substituteEnvVars(substituteVars(v, vars)))
			}
		}
	} else {
		for k, v := range cfg.Headers {
			req.Header.Set(k, substituteEnvVars(substituteVars(v, vars)))
		}
	}

	// Log jar cookie count at debug level for diagnostics.
	if pc.client.Jar != nil {
		if u2, err2 := url.Parse(endpoint); err2 == nil {
			slog.Debug("jar cookies before search request",
				"provider", cfg.ID,
				"cookie_count", len(pc.client.Jar.Cookies(u2)))
		}
	}

	// Transparency header: identify the tool to the operator without
	// concealing its nature. Providers who object can block on this
	// header; providers who don't are implicitly tolerating personal-use
	// access. Note: this does not remove any User-Agent header the
	// config sets (some providers require a browser UA to avoid WAF
	// blocks), it adds alongside.
	//
	// Skip this header for browser-cookie providers: adding a non-standard
	// header breaks the browser-identical request fingerprint that makes
	// the session cookies valid. Booking.com's WAF correlates the session
	// cookie with the original request fingerprint — an unknown header
	// causes it to serve a degraded response (0 hotel results in the SSR
	// Apollo cache despite HTTP 200).
	if cfg.Cookies.Source != "browser" {
		req.Header.Set("X-Personal-Use", "trvl personal noncommercial https://github.com/MikkoParkkola/trvl")
	}

	// Send request.
	resp, err := pc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := decompressBody(resp, maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	slog.Debug("search response", "provider", cfg.ID, "status", resp.StatusCode, "body_len", len(body),
		"content_encoding", resp.Header.Get("Content-Encoding"),
		"is_challenge", isAkamaiChallenge(resp.StatusCode, body))

	// Detect Akamai/AWS WAF challenge pages. HTTP 202 is in the 2xx range so
	// the generic status check below would accept it, but the body is an HTML
	// challenge page — not the real API response. When detected, run the same
	// Tier 3/4 escape-hatch cascade that runPreflight uses: browser cookies →
	// WAF JS solver → browser escape hatch. If any tier succeeds, retry the
	// main request with the fresh cookies.
	if isAkamaiChallenge(resp.StatusCode, body) {
		slog.Info("search response is an Akamai/WAF challenge page, attempting cookie recovery",
			"provider", cfg.ID, "status", resp.StatusCode)

		recovered := false

		// Tier 3a: re-read cookies from the user's browser.
		if applyBrowserCookies(pc.client, endpoint, cfg.Cookies.Browser) {
			resp2, body2, err2 := doSearchRequest(ctx, pc.client, req)
			if err2 == nil && !isAkamaiChallenge(resp2.StatusCode, body2) && resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
				resp, body = resp2, body2
				recovered = true
				slog.Info("search challenge bypassed via browser cookies", "provider", cfg.ID)
			}
		}

		// Tier 3b: WAF JS solver.
		if !recovered {
			cookie, wafErr := waf.SolveAWSWAF(ctx, pc.client, endpoint, string(body), nil)
			if wafErr == nil && cookie != nil {
				if u, parseErr := url.Parse(endpoint); parseErr == nil {
					pc.client.Jar.SetCookies(u, []*http.Cookie{cookie})
				}
				resp2, body2, err2 := doSearchRequest(ctx, pc.client, req)
				if err2 == nil && !isAkamaiChallenge(resp2.StatusCode, body2) && resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
					resp, body = resp2, body2
					recovered = true
					slog.Info("search challenge bypassed via WAF JS solver", "provider", cfg.ID)
				}
			}
		}

		// Tier 4: browser escape hatch.
		if !recovered && cfg.Auth != nil && cfg.Auth.BrowserEscapeHatch && isInteractive(ctx) {
			if tryBrowserEscapeHatch(ctx, pc, cfg.Auth) {
				resp2, body2, err2 := doSearchRequest(ctx, pc.client, req)
				if err2 == nil && !isAkamaiChallenge(resp2.StatusCode, body2) && resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
					resp, body = resp2, body2
					recovered = true
					slog.Info("search challenge bypassed via browser escape hatch", "provider", cfg.ID)
				}
			}
		}

		if !recovered {
			return nil, fmt.Errorf("http %d: WAF/JS challenge page — all cookie recovery tiers failed (provider %s)", resp.StatusCode, cfg.ID)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	// If the provider embeds its API response inside an HTML body (e.g.
	// Booking SSR'd Apollo cache), apply the configured regex to pull the
	// JSON blob out first. Capture group 1 replaces `body` for JSON parsing.
	if pattern := cfg.ResponseMapping.BodyExtractPattern; pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile body_extract_pattern: %w", err)
		}
		m := re.FindSubmatch(body)
		if len(m) < 2 {
			slog.Debug("body_extract_pattern did not match",
				"provider", cfg.ID,
				"body_len", len(body),
				"body_prefix", string(body[:min(len(body), 300)]))
			return nil, fmt.Errorf("body_extract_pattern %q did not match response body", pattern)
		}
		slog.Debug("body_extract_pattern matched", "provider", cfg.ID, "extract_len", len(m[1]))
		body = m[1]
	}

	// Parse JSON.
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	// Unwrap Airbnb Niobe SSR cache: {"niobeClientData":[[key, {data:...}]]}
	// into the inner payload so results_path can resolve normally.
	raw = unwrapNiobe(raw)

	// If the parsed JSON is an Apollo normalized cache (detected by a
	// top-level ROOT_QUERY key), resolve __ref pointers so that jsonPath
	// can traverse the data as a plain denormalized tree. This is required
	// for SSR-extracted providers like Booking.com where nested objects
	// (reviewScore, location, pricing) are stored as separate cache entries
	// linked via {"__ref": "BasicPropertyData:12345"}.
	if cache, ok := raw.(map[string]any); ok {
		if rootQuery, hasRoot := cache["ROOT_QUERY"]; hasRoot {
			// Only denormalize the ROOT_QUERY subtree, using the full cache
			// as the ref-lookup source. Denormalizing the entire top-level
			// cache would poison the `seen` set (cycle guard) with refs
			// encountered via different top-level keys, causing legitimate
			// multi-use refs (e.g. ReviewScore:42 used by both the top-level
			// entity AND the ROOT_QUERY chain) to appear circular.
			cache["ROOT_QUERY"] = denormalizeApollo(rootQuery, cache, nil)
		}
	}

	// If the response carries a top-level "errors" field (GraphQL convention),
	// surface the first error message before attempting to map results. This
	// makes stale persistedQuery hashes, CSRF mismatches, and WAF denials
	// immediately diagnosable instead of hiding behind a generic results_path
	// failure.
	if topObj, ok := raw.(map[string]any); ok {
		if errs, hasErrs := topObj["errors"].([]any); hasErrs && len(errs) > 0 {
			if firstErr, _ := errs[0].(map[string]any); firstErr != nil {
				msg, _ := firstErr["message"].(string)
				code := ""
				if ext, _ := firstErr["extensions"].(map[string]any); ext != nil {
					code, _ = ext["code"].(string)
				}
				if msg == "" && code == "" {
					msg = "unknown graphql error"
				}
				return nil, fmt.Errorf("graphql error: %s%s", msg, func() string {
					if code != "" {
						return " [" + code + "]"
					}
					return ""
				}())
			}
		}
	}

	// Extract results array.
	resultsRaw := jsonPath(raw, cfg.ResponseMapping.ResultsPath)
	arr, ok := resultsRaw.([]any)
	slog.Debug("results_path resolution", "provider", cfg.ID,
		"path", cfg.ResponseMapping.ResultsPath,
		"resolved_type", fmt.Sprintf("%T", resultsRaw),
		"is_array", ok,
		"count", func() int { if ok { return len(arr) }; return -1 }())
	// For Apollo-cache providers (e.g. Booking), log empty-results at debug
	// level so operators can diagnose SSR-vs-CSR rendering issues.
	if ok && len(arr) == 0 {
		slog.Debug("results_path resolved to empty array",
			"provider", cfg.ID, "body_len", len(body),
			"path", cfg.ResponseMapping.ResultsPath)
	}
	// The block below was a temporary debug dump; replaced with the
	// concise empty-array log above. Guard placeholder for the deleted
	// booking-specific debug block — this dead code will be cleaned up
	// in the next commit.
	if false {
		_ = cfg.ID
		_ = raw
		_ = fmt.Sprintf //nolint:staticcheck
		_ = slog.Debug
		_ = min(0, 0)
	}
	if !ok {
		// Include a body snippet + detected top-level keys so the LLM (and
		// human) can see what actually came back. This is the difference
		// between "mystery failure" and "ah, persistedQueryNotFound".
		snippet := string(body)
		if len(snippet) > 400 {
			snippet = snippet[:400] + "..."
		}
		var topKeys string
		if topObj, ok := raw.(map[string]any); ok {
			keys := make([]string, 0, len(topObj))
			for k := range topObj {
				keys = append(keys, k)
			}
			topKeys = fmt.Sprintf(" (top-level keys: %v)", keys)
		}
		return nil, fmt.Errorf("results_path %q did not resolve to an array%s; body: %s",
			cfg.ResponseMapping.ResultsPath, topKeys, snippet)
	}

	// Map each element to HotelResult and tag with provider source.
	hotels := make([]models.HotelResult, 0, len(arr))
	for _, item := range arr {
		h := mapHotelResult(item, cfg.ResponseMapping.Fields)
		src := models.PriceSource{
			Provider: cfg.ID,
			Price:    h.Price,
			Currency: h.Currency,
		}
		// Extract room-level price spread from Booking-style "blocks" array.
		if maxP, roomCt := extractBlocksPriceSpread(item); roomCt > 0 {
			src.MaxPrice = maxP
			src.RoomCount = roomCt
		}

		// Construct booking URL from pageName + countryCode when available.
		// Booking.com SSR results contain basicPropertyData.pageName (e.g.
		// "aix-europe") and basicPropertyData.location.countryCode (e.g. "fr")
		// which combine into the canonical hotel URL:
		// https://www.booking.com/hotel/{cc}/{pageName}.html
		if h.BookingURL == "" {
			if pageName, _ := jsonPath(item, "basicPropertyData.pageName").(string); pageName != "" {
				cc, _ := jsonPath(item, "basicPropertyData.location.countryCode").(string)
				if cc == "" {
					cc = "xx" // fallback — Booking will redirect
				}
				h.BookingURL = "https://www.booking.com/hotel/" + cc + "/" + pageName + ".html"
				src.BookingURL = h.BookingURL
			}
		}

		h.Sources = []models.PriceSource{src}

		// Normalize top-level price to the requested currency so
		// cross-provider comparison works (e.g. USD Booking vs EUR Google).
		// Airbnb returns prices in the requested currency but leaves the
		// currency field empty — treat empty as already-correct.
		srcCurrency := h.Currency
		if srcCurrency == "" {
			srcCurrency = currency // assume price is in the requested currency
		}
		h.Price = normalizePrice(h.Price, srcCurrency, currency)
		h.Currency = currency

		// Update source currency too — it was captured before the fallback.
		if len(h.Sources) > 0 && h.Sources[0].Currency == "" {
			h.Sources[0].Currency = currency
		}

		hotels = append(hotels, h)
	}

	// Rating enrichment: when hotels have a BookingURL but rating=0, fetch
	// the detail page to extract the JSON-LD aggregateRating. This only
	// fires for providers that produce booking URLs (currently Booking.com).
	// Capped at 5 enrichments per search to limit latency.
	enrichRatings(ctx, pc.client, hotels, cfg)

	return hotels, nil
}

// enrichRatings fetches hotel detail pages for results with rating=0 and a
// booking URL, extracting the aggregateRating from JSON-LD. This compensates
// for Booking.com's SSR response sometimes omitting review scores from the
// search results Apollo cache. Maximum 5 enrichments per call.
func enrichRatings(ctx context.Context, client *http.Client, hotels []models.HotelResult, cfg *ProviderConfig) {
	const maxEnrichments = 5
	enriched := 0

	for i := range hotels {
		if enriched >= maxEnrichments {
			break
		}
		if hotels[i].Rating > 0 || hotels[i].BookingURL == "" {
			continue
		}

		rating, reviewCount, err := fetchJSONLDRating(ctx, client, hotels[i].BookingURL)
		if err != nil {
			slog.Debug("rating enrichment failed", "url", hotels[i].BookingURL, "error", err.Error())
			continue
		}
		if rating > 0 {
			hotels[i].Rating = rating
			if reviewCount > 0 && hotels[i].ReviewCount == 0 {
				hotels[i].ReviewCount = reviewCount
			}
			slog.Debug("rating enriched from detail page",
				"hotel", hotels[i].Name, "rating", rating, "reviews", reviewCount)
		}
		enriched++
	}
	if enriched > 0 {
		slog.Info("enriched hotel ratings from detail pages",
			"provider", cfg.ID, "count", enriched)
	}
}

// fetchJSONLDRating fetches a hotel detail page and extracts the
// aggregateRating from the JSON-LD structured data. Booking.com embeds
// a <script type="application/ld+json"> block with the hotel's
// aggregateRating.ratingValue and aggregateRating.reviewCount.
func fetchJSONLDRating(ctx context.Context, client *http.Client, hotelURL string) (rating float64, reviewCount int, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", hotelURL, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("http %d", resp.StatusCode)
	}

	body, err := decompressBody(resp, maxResponseBytes)
	if err != nil {
		return 0, 0, err
	}

	// Extract JSON-LD blocks from the HTML.
	re := regexp.MustCompile(`<script[^>]*type="application/ld\+json"[^>]*>([\s\S]*?)</script>`)
	matches := re.FindAllSubmatch(body, -1)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		var ld map[string]any
		if err := json.Unmarshal(m[1], &ld); err != nil {
			continue
		}
		// Look for aggregateRating in the top level or within @graph.
		if r, rc := extractAggregateRating(ld); r > 0 {
			return r, rc, nil
		}
		// Check @graph array.
		if graph, ok := ld["@graph"].([]any); ok {
			for _, item := range graph {
				if obj, ok := item.(map[string]any); ok {
					if r, rc := extractAggregateRating(obj); r > 0 {
						return r, rc, nil
					}
				}
			}
		}
	}

	return 0, 0, fmt.Errorf("no aggregateRating in JSON-LD")
}

// extractAggregateRating extracts ratingValue and reviewCount from a JSON-LD
// object that has an "aggregateRating" property.
func extractAggregateRating(obj map[string]any) (float64, int) {
	ar, ok := obj["aggregateRating"].(map[string]any)
	if !ok {
		return 0, 0
	}
	rating := toFloat64(ar["ratingValue"])
	count := toInt(ar["reviewCount"])
	return rating, count
}

// runPreflight performs a GET to the preflight URL and extracts auth values.
// The vars map allows search-specific placeholders (e.g. ${city_id}) to be
// resolved in the preflight URL, so WAF cookies are obtained for the actual
// target city rather than a hardcoded default. When the resolved URL differs
// from the last preflight (city changed), the auth cache is invalidated.
func (rt *Runtime) runPreflight(ctx context.Context, pc *providerClient, vars map[string]string) error {
	if pc.config.Auth == nil || pc.config.Auth.PreflightURL == "" {
		return nil
	}

	// Resolve search-specific vars in the preflight URL so that ${city_id}
	// etc. produce a city-specific WAF session.
	resolvedURL := substituteVars(pc.config.Auth.PreflightURL, vars)

	pc.authMu.RLock()
	cacheValid := time.Now().Before(pc.authExpiry) && pc.lastPreflightURL == resolvedURL
	pc.authMu.RUnlock()
	if cacheValid {
		return nil
	}

	pc.authMu.Lock()
	defer pc.authMu.Unlock()

	// Double-check after lock.
	if time.Now().Before(pc.authExpiry) && pc.lastPreflightURL == resolvedURL {
		return nil
	}

	// Build a shallow copy of the auth config with the resolved URL so that
	// doPreflightRequest, cookie helpers, and WAF solver all see the
	// city-specific URL without mutating the shared config.
	resolvedAuth := *pc.config.Auth
	resolvedAuth.PreflightURL = resolvedURL

	// Tier 0: try loading persisted cookies from a previous successful session.
	// This makes browser escape hatch a one-time setup rather than per-search.
	loadCachedCookies(pc.client, resolvedURL)

	resp, body, err := doPreflightRequest(ctx, pc.client, &resolvedAuth)
	if err != nil {
		return err
	}

	extracted := applyExtractions(resolvedAuth.Extractions, resp, body, pc.authValues)
	// Stage 2: fetch any URL-based extractions (e.g. JS bundle for
	// persisted-query sha256Hash) using the now-populated cookie jar.
	extracted += applyURLExtractions(ctx, pc.client, resolvedAuth.Extractions, pc.authValues)

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
	if needsBrowserCookieFallback(resp.StatusCode, extracted, resolvedAuth.Extractions) {
		// Tier 3a: read cookies from user's browser (kooky).
		if tryBrowserCookieRetry(ctx, pc, &resolvedAuth) {
			saveCachedCookies(pc.client, resolvedURL)
			pc.lastPreflightURL = resolvedURL
			pc.authExpiry = time.Now().Add(authCacheDuration)
			return nil
		}
		// Tier 3b: run WAF challenge.js in sobek JS engine (pure Go).
		if tryWAFSolve(ctx, pc, &resolvedAuth, resp.StatusCode, body) {
			saveCachedCookies(pc.client, resolvedURL)
			pc.lastPreflightURL = resolvedURL
			pc.authExpiry = time.Now().Add(authCacheDuration)
			return nil
		}
		// Tier 4: last-resort escape hatch — open in browser.
		if resolvedAuth.BrowserEscapeHatch && isInteractive(ctx) {
			if tryBrowserEscapeHatch(ctx, pc, &resolvedAuth) {
				saveCachedCookies(pc.client, resolvedURL)
				pc.lastPreflightURL = resolvedURL
				pc.authExpiry = time.Now().Add(authCacheDuration)
				return nil
			}
		}
	}

	// Tier 1 succeeded directly — persist cookies for future sessions.
	saveCachedCookies(pc.client, resolvedURL)
	pc.lastPreflightURL = resolvedURL
	pc.authExpiry = time.Now().Add(authCacheDuration)
	return nil
}

// tryBrowserCookieRetry is Tier 3: read cookies from the user's disk-backed
// browser stores, seed them into the client jar, and retry preflight. Returns
// true on HTTP 2xx + successful extraction. The auth parameter carries the
// resolved (city-specific) preflight URL.
func tryBrowserCookieRetry(ctx context.Context, pc *providerClient, auth *AuthConfig) bool {
	if !applyBrowserCookies(pc.client, auth.PreflightURL, pc.config.Cookies.Browser) {
		return false
	}
	resp2, body2, err2 := doPreflightRequest(ctx, pc.client, auth)
	if err2 != nil || resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		return false
	}
	// Reject 202 challenge pages — they are in the 2xx range but are WAF
	// interstitials, not real responses.
	if isAkamaiChallenge(resp2.StatusCode, body2) {
		return false
	}
	for k := range pc.authValues {
		delete(pc.authValues, k)
	}
	applyExtractions(auth.Extractions, resp2, body2, pc.authValues)
	applyURLExtractions(ctx, pc.client, auth.Extractions, pc.authValues)
	return true
}

// tryWAFSolve is Tier 3b: if the preflight response looks like an AWS WAF
// challenge page (HTTP 202 with *.awswaf.com script refs), run challenge.js
// in the sobek JS engine to obtain an aws-waf-token cookie, then retry
// preflight. Returns true on success. The auth parameter carries the
// resolved (city-specific) preflight URL.
func tryWAFSolve(ctx context.Context, pc *providerClient, auth *AuthConfig, statusCode int, pageBody []byte) bool {
	// Only attempt on HTTP 202 (AWS WAF challenge) or 403 (some WAF variants).
	if statusCode != http.StatusAccepted && statusCode != http.StatusForbidden {
		return false
	}

	pageURL := auth.PreflightURL
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
	resp2, body2, err2 := doPreflightRequest(ctx, pc.client, auth)
	if err2 != nil || resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		return false
	}
	// Reject 202 challenge pages — still a WAF interstitial despite being 2xx.
	if isAkamaiChallenge(resp2.StatusCode, body2) {
		return false
	}
	for k := range pc.authValues {
		delete(pc.authValues, k)
	}
	applyExtractions(auth.Extractions, resp2, body2, pc.authValues)
	applyURLExtractions(ctx, pc.client, auth.Extractions, pc.authValues)
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
// 15-second timeout that users never noticed. The auth parameter carries the
// resolved (city-specific) preflight URL.
func tryBrowserEscapeHatch(ctx context.Context, pc *providerClient, auth *AuthConfig) bool {
	targetURL := auth.PreflightURL
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

	resp2, body2, err2 := doPreflightRequest(ctx, pc.client, auth)
	if err2 != nil || resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		slog.Warn("browser escape hatch: preflight retry still failed",
			"provider", pc.config.ID)
		return false
	}
	// Reject 202 challenge pages — still a WAF interstitial despite being 2xx.
	if isAkamaiChallenge(resp2.StatusCode, body2) {
		slog.Warn("browser escape hatch: preflight retry returned another challenge page",
			"provider", pc.config.ID)
		return false
	}
	for k := range pc.authValues {
		delete(pc.authValues, k)
	}
	applyExtractions(auth.Extractions, resp2, body2, pc.authValues)
	applyURLExtractions(ctx, pc.client, auth.Extractions, pc.authValues)
	slog.Info("browser escape hatch: preflight recovered", "provider", pc.config.ID)
	return true
}

// doSearchRequest clones the given request, executes it via client, reads the
// response body, and returns (resp, body, err). Used to retry the main search
// request after recovering cookies from the escape hatch. The original request
// body (if any) is not consumed by this helper — req.GetBody is used to obtain
// a fresh reader. The returned *http.Response must NOT be used for streaming;
// the body is already consumed and closed.
func doSearchRequest(ctx context.Context, client *http.Client, orig *http.Request) (*http.Response, []byte, error) {
	var bodyReader io.Reader
	if orig.GetBody != nil {
		b, err := orig.GetBody()
		if err != nil {
			return nil, nil, fmt.Errorf("search retry: get body: %w", err)
		}
		bodyReader = b
	}
	req, err := http.NewRequestWithContext(ctx, orig.Method, orig.URL.String(), bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("search retry: create request: %w", err)
	}
	req.Header = orig.Header.Clone()

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("search retry: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := decompressBody(resp, maxResponseBytes)
	if err != nil {
		return resp, nil, fmt.Errorf("search retry: read body: %w", err)
	}
	return resp, body, nil
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

	body, err := decompressBody(resp, maxResponseBytes)
	if err != nil {
		return resp, nil, fmt.Errorf("preflight read: %w", err)
	}
	return resp, body, nil
}

// applyExtractions runs each configured regex extraction against the response
// body or a named header, writing matches into authValues. Returns the number
// of extractions that matched. Extractions with a non-empty URL are skipped
// here — they require a second HTTP request and are handled by
// applyURLExtractions, which the caller should invoke after this one.
func applyExtractions(extractions map[string]Extraction, resp *http.Response, body []byte, authValues map[string]string) int {
	matched := 0
	for name, extraction := range extractions {
		if extraction.URL != "" {
			continue // deferred to applyURLExtractions
		}
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
		} else if extraction.Default != "" {
			varName := extraction.Variable
			if varName == "" {
				varName = name
			}
			authValues[varName] = extraction.Default
			matched++
			slog.Debug("extraction no match; using default",
				"name", name, "pattern", extraction.Pattern)
		}
	}
	return matched
}

// applyURLExtractions handles the second-stage extractions: those whose URL
// field is set. Each URL is fetched with the provided HTTP client (reusing
// its cookie jar — critical, since bundled JS is usually served under the
// provider's own origin with the same WAF cookies as the HTML page) and the
// pattern is matched against the response body. ${var} placeholders in the
// URL are resolved from authValues so a stage-2 URL can be derived from a
// stage-1 extraction (e.g. "bundle_url" extracted from HTML → fetched as
// stage 2). Returns the number of new variables matched.
func applyURLExtractions(ctx context.Context, client *http.Client, extractions map[string]Extraction, authValues map[string]string) int {
	if client == nil {
		return 0
	}
	// Build substitution map once from already-extracted values.
	vars := make(map[string]string, len(authValues))
	for k, v := range authValues {
		vars["${"+k+"}"] = v
	}

	matched := 0
	for name, extraction := range extractions {
		if extraction.URL == "" {
			continue
		}
		resolvedURL := substituteVars(extraction.URL, vars)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL, nil)
		if err != nil {
			slog.Warn("stage-2 extraction: build request failed",
				"name", name, "url", resolvedURL, "error", err.Error())
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			slog.Warn("stage-2 extraction: fetch failed",
				"name", name, "url", resolvedURL, "error", err.Error())
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close()
		if err != nil {
			slog.Warn("stage-2 extraction: read failed",
				"name", name, "url", resolvedURL, "error", err.Error())
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			slog.Warn("stage-2 extraction: non-2xx",
				"name", name, "url", resolvedURL, "status", resp.StatusCode)
			continue
		}

		re, err := regexp.Compile(extraction.Pattern)
		if err != nil {
			slog.Warn("stage-2 extraction: regex compile failed",
				"name", name, "pattern", extraction.Pattern, "error", err.Error())
			continue
		}
		m := re.FindStringSubmatch(string(body))
		varName := extraction.Variable
		if varName == "" {
			varName = name
		}
		if len(m) >= 2 {
			authValues[varName] = m[1]
			// Make the newly-extracted value available to subsequent URL
			// substitutions in this same pass (enables N-stage chains).
			vars["${"+varName+"}"] = m[1]
			matched++
		} else if extraction.Default != "" {
			authValues[varName] = extraction.Default
			vars["${"+varName+"}"] = extraction.Default
			matched++
			slog.Warn("stage-2 extraction: no match; using default",
				"name", name, "url", resolvedURL, "pattern", extraction.Pattern)
		} else {
			slog.Warn("stage-2 extraction: no match",
				"name", name, "url", resolvedURL, "pattern", extraction.Pattern)
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

// isAkamaiChallenge reports whether an HTTP response looks like an Akamai (or
// AWS WAF) JavaScript challenge page. These are characterised by HTTP 202
// status paired with body markers such as "window.aws", "reportChallengeError",
// or "challenge.js" script references. An HTTP 202 WITHOUT these markers is
// treated as a legitimate response (some APIs use 202 Accepted).
func isAkamaiChallenge(statusCode int, body []byte) bool {
	if statusCode != http.StatusAccepted {
		return false
	}
	// Short-circuit: if the body parses as valid JSON with no challenge markers,
	// it is a real 202 Accepted response (e.g. async job acknowledgement).
	// Challenge pages are always HTML, never JSON.
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return false
	}
	// Look for challenge page signatures in HTML.
	return bytes.Contains(body, []byte("challenge.js")) ||
		bytes.Contains(body, []byte("window.aws")) ||
		bytes.Contains(body, []byte("reportChallengeError")) ||
		bytes.Contains(body, []byte("awswaf"))
}

// applyBrowserCookies reads cookies from the user's browsers for the given
// URL and seeds them into the client's cookie jar. When browserHint is
// non-empty, reads only from that specific browser to avoid cross-browser
// cookie contamination. Returns true if any cookies were applied.
func applyBrowserCookies(client *http.Client, targetURL, browserHint string) bool {
	if client == nil || client.Jar == nil {
		return false
	}
	cookies := browserCookiesForURLWithHint(targetURL, browserHint)
	slog.Debug("applyBrowserCookies", "url", targetURL, "browser", browserHint, "count", len(cookies))
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

// decompressBody reads and decompresses the response body based on the
// Content-Encoding header. When the request explicitly sets Accept-Encoding
// (e.g. "gzip, deflate, br, zstd" to match Chrome), Go's http.Transport
// does NOT auto-decompress — it assumes the caller handles decompression.
// This function handles gzip, br (Brotli), and zstd transparently.
//
// When the transport (or an intermediate CDN/proxy) already decompressed the
// body but left the Content-Encoding header intact, the declared encoding
// won't match the actual payload. The gzip path buffers the body and falls
// back to raw bytes on header mismatch — this is the most common case in
// practice (e.g. Airbnb preflight via fhttp Chrome-fingerprinted transport).
func decompressBody(resp *http.Response, limit int64) ([]byte, error) {
	// When the transport already decompressed the body (e.g. Go's default
	// gzip handling), Uncompressed is true and the Content-Encoding header
	// may still be present. Reading raw is correct.
	if resp.Uncompressed {
		return io.ReadAll(io.LimitReader(resp.Body, limit))
	}

	encoding := resp.Header.Get("Content-Encoding")
	reader := io.LimitReader(resp.Body, limit)

	switch encoding {
	case "br":
		br := brotli.NewReader(reader)
		return io.ReadAll(br)
	case "gzip":
		// Buffer the body so we can fall back to raw bytes if the payload
		// is not actually gzip-encoded. This happens when the transport or
		// a CDN decompressed the body but left the Content-Encoding header,
		// or when the server advertises gzip but sends identity/Brotli.
		raw, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("gzip read raw: %w", err)
		}
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			// Not valid gzip — return the raw bytes as-is.
			slog.Debug("Content-Encoding says gzip but body is not gzip, using raw",
				"error", err.Error(), "body_len", len(raw))
			return raw, nil
		}
		defer gr.Close()
		decoded, err := io.ReadAll(gr)
		if err != nil {
			// Gzip header valid but decompression failed mid-stream.
			slog.Debug("gzip decompression failed mid-stream, using raw",
				"error", err.Error(), "body_len", len(raw))
			return raw, nil
		}
		return decoded, nil
	case "zstd":
		zr, err := zstd.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("zstd reader: %w", err)
		}
		defer zr.Close()
		return io.ReadAll(zr)
	default:
		// No encoding or "identity" — read raw.
		return io.ReadAll(reader)
	}
}

// substituteVars replaces all ${var} placeholders in s with values from vars.
