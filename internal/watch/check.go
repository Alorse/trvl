package watch

import (
	"context"
	"fmt"
	"time"
)

// PriceChecker retrieves the current cheapest price for a route.
// Implementations bridge to flights.SearchFlights or hotels.SearchHotels
// without creating an import dependency from the watch package.
type PriceChecker interface {
	// CheckPrice returns the cheapest price and currency for the given watch.
	// For date-range and route watches, also returns the cheapest date found.
	// Returns 0 price if no results are found (not an error).
	CheckPrice(ctx context.Context, w Watch) (price float64, currency string, cheapestDate string, err error)
}

// CheckResult holds the outcome of checking a single watch.
type CheckResult struct {
	Watch        Watch
	NewPrice     float64
	Currency     string
	PrevPrice    float64
	BelowGoal    bool    // price dropped below threshold
	PriceDrop    float64 // negative = price decreased (good)
	CheapestDate string  // for range/route watches: which date was cheapest
	Error        error
}

// CheckAll checks all watches using the provided price checker and records
// results in the store. Pauses 3 seconds between checks to respect API rate limits.
// Returns a result for each watch.
func CheckAll(ctx context.Context, store *Store, checker PriceChecker) []CheckResult {
	watches := store.List()
	results := make([]CheckResult, 0, len(watches))

	for i, w := range watches {
		r := checkOne(ctx, store, checker, w)
		results = append(results, r)

		// Pause between checks to respect rate limits (skip after last).
		if i < len(watches)-1 {
			select {
			case <-ctx.Done():
				return results
			case <-time.After(3 * time.Second):
			}
		}
	}
	return results
}

// checkOne performs a price check for a single watch.
func checkOne(ctx context.Context, store *Store, checker PriceChecker, w Watch) CheckResult {
	price, currency, cheapestDate, err := checker.CheckPrice(ctx, w)
	if err != nil {
		return CheckResult{Watch: w, Error: err}
	}

	result := CheckResult{
		Watch:        w,
		NewPrice:     price,
		Currency:     currency,
		PrevPrice:    w.LastPrice,
		CheapestDate: cheapestDate,
	}

	if price > 0 {
		// Calculate price change.
		if w.LastPrice > 0 {
			result.PriceDrop = price - w.LastPrice
		}

		// Check threshold.
		if w.BelowPrice > 0 && price <= w.BelowPrice {
			result.BelowGoal = true
		}

		// Update watch state.
		w.LastCheck = time.Now()
		w.LastPrice = price
		w.Currency = currency
		if cheapestDate != "" {
			w.CheapestDate = cheapestDate
		}
		if w.LowestPrice == 0 || price < w.LowestPrice {
			w.LowestPrice = price
		}

		// Persist updates.
		if err := store.UpdateWatch(w); err != nil {
			result.Error = fmt.Errorf("update watch: %w", err)
			return result
		}

		if err := store.RecordPrice(w.ID, price, currency); err != nil {
			result.Error = fmt.Errorf("record price: %w", err)
			return result
		}

		// Update the result's watch to reflect saved state.
		result.Watch = w
	}

	return result
}
