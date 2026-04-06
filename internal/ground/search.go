package ground

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/cache"
	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

// groundCache caches ground transport search results.
var groundCache = cache.New()

// groundCacheTTL is the TTL for cached ground transport results.
const groundCacheTTL = 10 * time.Minute

// httpClient is a shared HTTP client with sensible timeouts for FlixBus/RegioJet.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// Shared rate limiters for FlixBus and RegioJet (used by the shared httpClient).
var (
	flixbusLimiter  = rate.NewLimiter(rate.Limit(10), 1) // 10 req/s
	regiojetLimiter = rate.NewLimiter(rate.Limit(10), 1) // 10 req/s
)

// rateLimitedDo executes an HTTP request through the shared client after
// waiting on the provided rate limiter.
func rateLimitedDo(ctx context.Context, limiter *rate.Limiter, req *http.Request) (*http.Response, error) {
	if err := limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}
	return httpClient.Do(req)
}

// SearchOptions configures a ground transport search.
type SearchOptions struct {
	Currency  string   // Default: EUR
	Providers []string // Filter to specific providers; empty = all
	MaxPrice  float64  // 0 = no limit
	Type      string   // "bus", "train", or empty for all
	NoCache   bool     // bypass response cache
}

// SearchByName searches all providers for ground transport between two cities
// given by name. Resolves city names to provider-specific IDs automatically.
func SearchByName(ctx context.Context, from, to, date string, opts SearchOptions) (*models.GroundSearchResult, error) {
	if opts.Currency == "" {
		opts.Currency = "EUR"
	}

	// Build cache key from search parameters.
	providerKey := "all"
	if len(opts.Providers) > 0 {
		sorted := make([]string, len(opts.Providers))
		copy(sorted, opts.Providers)
		sort.Strings(sorted)
		providerKey = strings.Join(sorted, ",")
	}
	cacheKey := cache.Key("ground", fmt.Sprintf("%s|%s|%s|%s|%s|%.2f|%s", from, to, date, opts.Currency, providerKey, opts.MaxPrice, opts.Type))

	// Check cache unless bypassed.
	if !opts.NoCache {
		if data, ok := groundCache.Get(cacheKey); ok {
			var cached models.GroundSearchResult
			if err := json.Unmarshal(data, &cached); err == nil {
				slog.Debug("ground cache hit", "from", from, "to", to, "date", date)
				return &cached, nil
			}
		}
	}

	type providerResult struct {
		routes []models.GroundRoute
		err    error
		name   string
	}

	var wg sync.WaitGroup
	results := make(chan providerResult, 17)

	useProvider := func(name string) bool {
		if len(opts.Providers) == 0 {
			return true
		}
		for _, p := range opts.Providers {
			if strings.EqualFold(p, name) {
				return true
			}
		}
		return false
	}

	// Distribusion — ground transport GDS covering bus, ferry, train, airport transfers.
	// Placed first (before individual providers) since it aggregates 2,000+ carriers.
	// Requires DISTRIBUSION_API_KEY to be set; silently skipped otherwise.
	if useProvider("distribusion") && HasDistribusionKey() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchDistribusion(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "distribusion"}
		}()
	}

	// FlixBus
	if useProvider("flixbus") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := searchFlixBusByName(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "flixbus"}
		}()
	}

	// RegioJet
	if useProvider("regiojet") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := searchRegioJetByName(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "regiojet"}
		}()
	}

	// Eurostar — only if both cities have Eurostar stations.
	// Search both Snap (last-minute deals) and regular fares in parallel so the
	// user sees both options (e.g. "eurostar snap GBP 39" and "eurostar GBP 130").
	if (useProvider("eurostar") || useProvider("eurostar snap")) && HasEurostarRoute(from, to) {
		// Eurostar cheapestFaresSearch needs a date range (not a single day).
		// Use the requested date as start, +7 days as end.
		endDate := date // fallback
		if t, err := time.Parse("2006-01-02", date); err == nil {
			endDate = t.AddDate(0, 0, 7).Format("2006-01-02")
		}

		// Snap fares goroutine.
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchEurostar(ctx, from, to, date, endDate, opts.Currency, true)
			results <- providerResult{routes: routes, err: err, name: "eurostar snap"}
		}()

		// Regular fares goroutine.
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchEurostar(ctx, from, to, date, endDate, opts.Currency, false)
			results <- providerResult{routes: routes, err: err, name: "eurostar"}
		}()
	}

	// NS (Dutch Railways) — only if at least one city has an NS station.
	if useProvider("ns") && (HasNSStation(from) || HasNSStation(to)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchNS(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "ns"}
		}()
	}

	// Deutsche Bahn — if at least one city has a DB station (covers most European rail).
	if useProvider("db") && (HasDBStation(from) || HasDBStation(to)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchDeutscheBahn(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "db"}
		}()
	}

	// SNCF — only if at least one city is French.
	if useProvider("sncf") && HasSNCFRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchSNCF(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "sncf"}
		}()
	}

	// Trainline — train aggregator (covers SNCF, Eurostar, DB, Trenitalia, etc.)
	if useProvider("trainline") && HasTrainlineStation(from) && HasTrainlineStation(to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchTrainline(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "trainline"}
		}()
	}

	// ÖBB (Austrian Federal Railways) — Austria and neighbouring countries.
	if useProvider("oebb") && HasOebbRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchOebb(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "oebb"}
		}()
	}

	// Digitransit (VR Finnish Railways) — only if at least one city has a Finnish station.
	if (useProvider("digitransit") || useProvider("vr")) && (HasDigitransitStation(from) || HasDigitransitStation(to)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchDigitransit(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "vr"}
		}()
	}

	// Renfe (Spain) — only if at least one city has a Renfe station (Spanish rail).
	if useProvider("renfe") && HasRenfeRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchRenfe(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "renfe"}
		}()
	}

	// Tallink/Silja Line — ferry routes in the Baltic Sea.
	if useProvider("tallink") && HasTallinkRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchTallink(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "tallink"}
		}()
	}

	// Stena Line — ferry routes across the North Sea and Baltic Sea.
	if useProvider("stenaline") && HasStenaLineRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchStenaLine(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "stenaline"}
		}()
	}

	// DFDS — ferry routes across the North Sea and Baltic Sea.
	if useProvider("dfds") && HasDFDSRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchDFDS(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "dfds"}
		}()
	}

	// Viking Line — ferry routes in the Baltic Sea (Helsinki–Tallinn, Helsinki–Stockholm,
	// Turku–Stockholm, Stockholm–Mariehamn).
	if useProvider("vikingline") && HasVikingLineRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchVikingLine(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "vikingline"}
		}()
	}

	// Eckerö Line — Helsinki ↔ Tallinn ferry (M/S Finlandia).
	if useProvider("eckeroline") && HasEckeroLineRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := SearchEckeroLine(ctx, from, to, date, opts.Currency)
			results <- providerResult{routes: routes, err: err, name: "eckeroline"}
		}()
	}

	// Transitous — coordinate-based, always available as a fallback.
	// Requires geocoding city names to coordinates; skipped if geocoding fails.
	if useProvider("transitous") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			routes, err := searchTransitousByName(ctx, from, to, date)
			results <- providerResult{routes: routes, err: err, name: "transitous"}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allRoutes []models.GroundRoute
	var errors []string
	for r := range results {
		if r.err != nil {
			slog.Warn("ground provider error", "provider", r.name, "error", r.err)
			errors = append(errors, fmt.Sprintf("%s: %v", r.name, r.err))
			continue
		}
		allRoutes = append(allRoutes, r.routes...)
	}

	// Filter out zero-price routes and deduplicate (same provider+time+price).
	allRoutes = filterUnavailableGroundRoutes(allRoutes)
	allRoutes = deduplicateGroundRoutes(allRoutes)

	// Apply filters
	if opts.MaxPrice > 0 {
		filtered := allRoutes[:0]
		for _, r := range allRoutes {
			if r.Price <= opts.MaxPrice {
				filtered = append(filtered, r)
			}
		}
		allRoutes = filtered
	}
	if opts.Type != "" {
		filtered := allRoutes[:0]
		for _, r := range allRoutes {
			if strings.EqualFold(r.Type, opts.Type) {
				filtered = append(filtered, r)
			}
		}
		allRoutes = filtered
	}

	// Sort by price
	sort.Slice(allRoutes, func(i, j int) bool {
		return allRoutes[i].Price < allRoutes[j].Price
	})

	result := &models.GroundSearchResult{
		Success: len(allRoutes) > 0,
		Count:   len(allRoutes),
		Routes:  allRoutes,
	}
	if len(allRoutes) == 0 && len(errors) > 0 {
		result.Error = strings.Join(errors, "; ")
	}

	// Cache successful results.
	if result.Success && !opts.NoCache {
		if data, err := json.Marshal(result); err == nil {
			groundCache.Set(cacheKey, data, groundCacheTTL)
		}
	}

	return result, nil
}

func deduplicateGroundRoutes(routes []models.GroundRoute) []models.GroundRoute {
	seen := make(map[string]bool)
	result := routes[:0]
	for _, r := range routes {
		key := fmt.Sprintf("%s|%s|%.2f|%s", r.Provider, r.Departure.Time, r.Price, r.Arrival.Time)
		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}
	return result
}

// scheduleOnlyProviders is the set of providers whose results are kept even
// when price is 0 (they provide schedule data without live pricing).
var scheduleOnlyProviders = map[string]bool{
	"distribusion": true, "transitous": true, "db": true, "ns": true,
	"oebb": true, "vr": true, "tallink": true, "stenaline": true,
	"dfds": true, "vikingline": true, "eckeroline": true,
}

func filterUnavailableGroundRoutes(routes []models.GroundRoute) []models.GroundRoute {
	filtered := routes[:0]
	for _, route := range routes {
		if route.Price > 0 || scheduleOnlyProviders[strings.ToLower(route.Provider)] {
			filtered = append(filtered, route)
		}
	}
	return filtered
}

// searchFlixBusByName resolves city names and searches FlixBus.
func searchFlixBusByName(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromCities, err := FlixBusAutoComplete(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("resolve from city: %w", err)
	}
	if len(fromCities) == 0 {
		return nil, fmt.Errorf("no FlixBus city found for %q", from)
	}

	toCities, err := FlixBusAutoComplete(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("resolve to city: %w", err)
	}
	if len(toCities) == 0 {
		return nil, fmt.Errorf("no FlixBus city found for %q", to)
	}

	routes, err := SearchFlixBus(ctx, fromCities[0].ID, toCities[0].ID, date, currency)
	if err != nil {
		return nil, err
	}

	// Enrich city names
	for i := range routes {
		if routes[i].Departure.City == "" {
			routes[i].Departure.City = fromCities[0].Name
		}
		if routes[i].Arrival.City == "" {
			routes[i].Arrival.City = toCities[0].Name
		}
	}

	return routes, nil
}

// searchRegioJetByName resolves city names and searches RegioJet.
func searchRegioJetByName(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromCities, err := RegioJetAutoComplete(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("resolve from city: %w", err)
	}
	if len(fromCities) == 0 {
		return nil, fmt.Errorf("no RegioJet city found for %q", from)
	}

	toCities, err := RegioJetAutoComplete(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("resolve to city: %w", err)
	}
	if len(toCities) == 0 {
		return nil, fmt.Errorf("no RegioJet city found for %q", to)
	}

	return SearchRegioJet(ctx, fromCities[0].ID, toCities[0].ID, date, currency)
}

// searchTransitousByName geocodes city names to coordinates and searches Transitous.
func searchTransitousByName(ctx context.Context, from, to, date string) ([]models.GroundRoute, error) {
	fromGeo, err := geocodeCity(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("geocode from city: %w", err)
	}
	toGeo, err := geocodeCity(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("geocode to city: %w", err)
	}
	return SearchTransitous(ctx, fromGeo.lat, fromGeo.lon, toGeo.lat, toGeo.lon, date)
}

// geoCoord holds a latitude/longitude pair from geocoding.
type geoCoord struct {
	lat float64
	lon float64
}

// geoCityCache caches city name to coordinate lookups.
var geoCityCache = struct {
	sync.RWMutex
	entries map[string]geoCoord
}{entries: make(map[string]geoCoord)}

// geocodeCity resolves a city name to coordinates using Nominatim.
func geocodeCity(ctx context.Context, city string) (geoCoord, error) {
	key := strings.ToLower(strings.TrimSpace(city))

	geoCityCache.RLock()
	if entry, ok := geoCityCache.entries[key]; ok {
		geoCityCache.RUnlock()
		return entry, nil
	}
	geoCityCache.RUnlock()

	params := url.Values{
		"q":      {city},
		"format": {"json"},
		"limit":  {"1"},
	}
	apiURL := "https://nominatim.openstreetmap.org/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return geoCoord{}, err
	}
	req.Header.Set("User-Agent", "trvl/1.0 (travel agent; github.com/MikkoParkkola/trvl)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return geoCoord{}, fmt.Errorf("nominatim: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return geoCoord{}, fmt.Errorf("nominatim: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return geoCoord{}, fmt.Errorf("nominatim read: %w", err)
	}

	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.Unmarshal(body, &results); err != nil {
		return geoCoord{}, fmt.Errorf("nominatim decode: %w", err)
	}
	if len(results) == 0 {
		return geoCoord{}, fmt.Errorf("no geocoding results for %q", city)
	}

	var lat, lon float64
	if _, err := fmt.Sscanf(results[0].Lat, "%f", &lat); err != nil {
		return geoCoord{}, fmt.Errorf("parse lat %q: %w", results[0].Lat, err)
	}
	if _, err := fmt.Sscanf(results[0].Lon, "%f", &lon); err != nil {
		return geoCoord{}, fmt.Errorf("parse lon %q: %w", results[0].Lon, err)
	}

	coord := geoCoord{lat: lat, lon: lon}
	geoCityCache.Lock()
	geoCityCache.entries[key] = coord
	geoCityCache.Unlock()

	return coord, nil
}
