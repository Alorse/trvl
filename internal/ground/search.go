package ground

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

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
	Currency  string // Default: EUR
	Providers []string // Filter to specific providers; empty = all
	MaxPrice  float64  // 0 = no limit
	Type      string   // "bus", "train", or empty for all
}

// SearchByName searches all providers for ground transport between two cities
// given by name. Resolves city names to provider-specific IDs automatically.
func SearchByName(ctx context.Context, from, to, date string, opts SearchOptions) (*models.GroundSearchResult, error) {
	if opts.Currency == "" {
		opts.Currency = "EUR"
	}

	type providerResult struct {
		routes []models.GroundRoute
		err    error
		name   string
	}

	var wg sync.WaitGroup
	results := make(chan providerResult, 3)

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
	// Try Snap (last-minute deals) first — if no Snap fares, fall back to regular.
	if (useProvider("eurostar") || useProvider("eurostar_snap")) && HasEurostarRoute(from, to) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Eurostar returns cheapest fares for a date range; use the single date
			// as both start and end to get that day's price.
			// Try Snap first (preferred — better value), fall back to regular.
			routes, err := SearchEurostar(ctx, from, to, date, date, opts.Currency, true)
			if err != nil || len(routes) == 0 {
				slog.Debug("no eurostar snap fares, trying regular", "err", err)
				routes, err = SearchEurostar(ctx, from, to, date, date, opts.Currency, false)
			}
			results <- providerResult{routes: routes, err: err, name: "eurostar"}
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

	// Filter out zero-price routes (sold-out routes from RegioJet)
	{
		filtered := allRoutes[:0]
		for _, r := range allRoutes {
			if r.Price > 0 {
				filtered = append(filtered, r)
			}
		}
		allRoutes = filtered
	}

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
	return result, nil
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
