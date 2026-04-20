package flights

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// SearchMultiAirport searches flights across multiple origin and destination airports.
// Runs all origin×destination combinations in parallel (max 5 concurrent) and merges
// results sorted by price. Each flight already contains departure/arrival airport codes.
func SearchMultiAirport(ctx context.Context, origins, destinations []string, date string, opts SearchOptions) (*models.FlightSearchResult, error) {
	client := DefaultClient()
	opts.defaults()

	if len(origins) == 0 || len(destinations) == 0 || date == "" {
		return nil, fmt.Errorf("origins, destinations, and date are required")
	}

	sem := make(chan struct{}, 5) // max 5 concurrent searches
	var mu sync.Mutex
	var allFlights []models.FlightResult
	var wg sync.WaitGroup

	for _, orig := range origins {
		for _, dest := range destinations {
			if orig == dest {
				continue
			}
			wg.Add(1)
			go func(o, d string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				result, err := SearchFlightsWithClient(ctx, client, o, d, date, opts)
				if err != nil || !result.Success {
					return // skip failed combos silently
				}

				mu.Lock()
				allFlights = append(allFlights, result.Flights...)
				mu.Unlock()
			}(orig, dest)
		}
	}

	wg.Wait()

	sortFlightResults(allFlights, opts.SortBy)

	return &models.FlightSearchResult{
		Success:  len(allFlights) > 0,
		Count:    len(allFlights),
		TripType: tripTypeForSearch(opts),
		Flights:  allFlights,
	}, nil
}

// ParseAirports splits a comma-separated airport string into a slice.
// Trims whitespace and uppercases each code.
func ParseAirports(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToUpper(p))
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ParseFlightLocations extends ParseAirports with city-name resolution.
// Each comma-separated token is treated as:
//   - An IATA code (exactly 3 uppercase ASCII letters) → kept as-is
//   - A known city name → expanded to all airports serving that city
//   - Anything else → kept as-is (unknown code passthrough)
//
// Returned slice contains no duplicates and preserves encounter order.
func ParseFlightLocations(s string) []string {
	tokens := ParseAirports(s)
	if len(tokens) == 0 {
		return tokens
	}
	seen := make(map[string]bool, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if models.IsIATACode(token) {
			if !seen[token] {
				seen[token] = true
				out = append(out, token)
			}
			continue
		}
		airports := models.ResolveCityToAirports(token)
		if len(airports) == 0 {
			// Unknown token — pass through as-is (may be a valid IATA not in our map).
			if !seen[token] {
				seen[token] = true
				out = append(out, token)
			}
			continue
		}
		for _, code := range airports {
			if !seen[code] {
				seen[code] = true
				out = append(out, code)
			}
		}
	}
	return out
}
