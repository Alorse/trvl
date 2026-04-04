package trip

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// MultiCityOptions configures a multi-city trip optimization.
type MultiCityOptions struct {
	DepartDate string // YYYY-MM-DD start date
	ReturnDate string // YYYY-MM-DD end date (return to home)
}

// Segment represents a flight between two cities.
type Segment struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

// MultiCityResult is the top-level response for a multi-city optimization.
type MultiCityResult struct {
	Success       bool      `json:"success"`
	HomeAirport   string    `json:"home_airport"`
	OptimalOrder  []string  `json:"optimal_order"`
	Segments      []Segment `json:"segments"`
	TotalCost     float64   `json:"total_cost"`
	Currency      string    `json:"currency"`
	WorstCost     float64   `json:"worst_cost"`
	Savings       float64   `json:"savings"`
	Permutations  int       `json:"permutations_checked"`
	Error         string    `json:"error,omitempty"`
}

// OptimizeMultiCity finds the cheapest routing order for visiting multiple cities.
//
// Given a home airport and a list of cities to visit (max 6), it tries all
// permutations of visit order and returns the one with the lowest total flight
// cost. The route is: home -> city1 -> city2 -> ... -> cityN -> home.
//
// For each leg, it fetches the cheapest one-way flight price. For N cities,
// there are N! permutations (720 for N=6), each requiring N+1 price lookups.
// To avoid excessive API calls, prices are cached in a map.
func OptimizeMultiCity(ctx context.Context, homeAirport string, cities []string, opts MultiCityOptions) (*MultiCityResult, error) {
	if homeAirport == "" {
		return nil, fmt.Errorf("home airport is required")
	}
	if len(cities) == 0 {
		return nil, fmt.Errorf("at least one city is required")
	}
	if len(cities) > 6 {
		return nil, fmt.Errorf("maximum 6 cities supported (got %d); N! permutations grow too fast", len(cities))
	}
	if opts.DepartDate == "" {
		return nil, fmt.Errorf("depart_date is required")
	}

	// Pre-fetch all needed prices.
	// Legs needed: home->each city, each city->each other city, each city->home.
	allCodes := append([]string{homeAirport}, cities...)
	priceCache := make(map[string]float64)
	currencyCache := make(map[string]string)

	for _, from := range allCodes {
		for _, to := range allCodes {
			if from == to {
				continue
			}
			key := from + "->" + to
			if _, ok := priceCache[key]; ok {
				continue
			}
			pc := fetchCheapestPrice(ctx, from, to, opts.DepartDate)
			priceCache[key] = pc.price
			if pc.currency != "" {
				currencyCache[key] = pc.currency
			}
		}
	}

	return optimizeRoute(homeAirport, cities, priceCache, currencyCache), nil
}

// optimizeRoute finds the cheapest routing order given pre-fetched prices.
func optimizeRoute(homeAirport string, cities []string, priceCache map[string]float64, currencyCache map[string]string) *MultiCityResult {
	perms := permutations(cities)

	var bestOrder []string
	bestCost := math.MaxFloat64
	worstCost := 0.0

	for _, perm := range perms {
		cost := routeCost(homeAirport, perm, priceCache)
		if cost < bestCost {
			bestCost = cost
			bestOrder = perm
		}
		if cost > worstCost {
			worstCost = cost
		}
	}

	// Build segments for the best route.
	route := append([]string{homeAirport}, bestOrder...)
	route = append(route, homeAirport)

	// Build segments — use the actual currency from each flight search.
	var segments []Segment
	detectedCurrency := ""
	for i := 0; i < len(route)-1; i++ {
		key := route[i] + "->" + route[i+1]
		cur := currencyCache[key]
		if cur != "" && detectedCurrency == "" {
			detectedCurrency = cur
		}
		segments = append(segments, Segment{
			From:     route[i],
			To:       route[i+1],
			Price:    priceCache[key],
			Currency: cur,
		})
	}

	return &MultiCityResult{
		Success:      true,
		HomeAirport:  homeAirport,
		OptimalOrder: bestOrder,
		Segments:     segments,
		TotalCost:    bestCost,
		Currency:     detectedCurrency,
		WorstCost:    worstCost,
		Savings:      worstCost - bestCost,
		Permutations: len(perms),
	}
}

// routeCost calculates the total cost of a route: home -> perm[0] -> ... -> perm[N-1] -> home.
func routeCost(home string, perm []string, prices map[string]float64) float64 {
	total := 0.0
	route := append([]string{home}, perm...)
	route = append(route, home)

	for i := 0; i < len(route)-1; i++ {
		key := route[i] + "->" + route[i+1]
		total += prices[key]
	}
	return total
}

// fetchCheapestPrice searches for the cheapest one-way flight between two airports.
// Returns 9999 if the search fails (so the route is deprioritized but not excluded).
// priceAndCurrency holds a price with its source currency.
type priceAndCurrency struct {
	price    float64
	currency string
}

func fetchCheapestPrice(ctx context.Context, from, to, date string) priceAndCurrency {
	result, err := flights.SearchFlights(ctx, from, to, date, flights.SearchOptions{
		SortBy: models.SortCheapest,
		Adults: 1,
	})
	if err != nil || !result.Success || len(result.Flights) == 0 {
		return priceAndCurrency{price: 9999}
	}

	cheapest := result.Flights[0]
	for _, f := range result.Flights[1:] {
		if f.Price > 0 && f.Price < cheapest.Price {
			cheapest = f
		}
	}
	if cheapest.Price <= 0 {
		return priceAndCurrency{price: 9999}
	}
	return priceAndCurrency{price: cheapest.Price, currency: cheapest.Currency}
}

// permutations generates all permutations of a string slice.
func permutations(input []string) [][]string {
	if len(input) <= 1 {
		return [][]string{append([]string{}, input...)}
	}

	var result [][]string
	for i, elem := range input {
		rest := make([]string, 0, len(input)-1)
		rest = append(rest, input[:i]...)
		rest = append(rest, input[i+1:]...)

		subPerms := permutations(rest)
		for _, sp := range subPerms {
			perm := append([]string{elem}, sp...)
			result = append(result, perm)
		}
	}
	return result
}

// SortSegmentsByPrice sorts segments by price for display.
func SortSegmentsByPrice(segments []Segment) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Price < segments[j].Price
	})
}
