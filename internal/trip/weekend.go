package trip

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/explore"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// WeekendOptions configures a weekend getaway search.
type WeekendOptions struct {
	Month     string  // Month to search in, e.g. "july-2026" or "2026-07"
	MaxBudget float64 // Maximum total budget (flight + hotel estimate)
	Nights    int     // Number of nights (default: 2)
}

// WeekendDestination is a single destination in the weekend getaway results.
type WeekendDestination struct {
	Destination   string  `json:"destination"`
	AirportCode   string  `json:"airport_code"`
	FlightPrice   float64 `json:"flight_price"`
	HotelEstimate float64 `json:"hotel_estimate"`
	TotalEstimate float64 `json:"total_estimate"`
	Currency      string  `json:"currency"`
	Stops         int     `json:"stops"`
	AirlineName   string  `json:"airline_name,omitempty"`
}

// WeekendResult is the top-level response for a weekend getaway search.
type WeekendResult struct {
	Success      bool                 `json:"success"`
	Origin       string               `json:"origin"`
	Month        string               `json:"month"`
	Nights       int                  `json:"nights"`
	Count        int                  `json:"count"`
	Destinations []WeekendDestination `json:"destinations"`
	Error        string               `json:"error,omitempty"`
}

// defaults fills in zero-value fields.
func (o *WeekendOptions) defaults() {
	if o.Nights <= 0 {
		o.Nights = 2
	}
}

// parseMonth parses month strings like "july-2026", "2026-07", "jul-2026".
// Returns the first Friday of the month as departure date and the following
// Sunday as return date.
func parseMonth(month string) (departDate, returnDate string, displayMonth string, err error) {
	formats := []string{
		"January-2006", "january-2006",
		"Jan-2006", "jan-2006",
		"2006-01",
	}

	var t time.Time
	for _, f := range formats {
		t, err = time.Parse(f, month)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", "", "", fmt.Errorf("invalid month %q: use format like july-2026 or 2026-07", month)
	}

	displayMonth = t.Format("January 2006")

	// Find first Friday of the month.
	d := t
	for d.Weekday() != time.Friday {
		d = d.AddDate(0, 0, 1)
	}
	departDate = d.Format("2006-01-02")
	returnDate = d.AddDate(0, 0, 2).Format("2006-01-02") // Sunday

	return departDate, returnDate, displayMonth, nil
}

// estimateHotelFromPriceLevel estimates a per-night hotel cost based on
// the flight price level. Destinations with cheap flights tend to have
// cheaper hotels. This is a rough heuristic.
func estimateHotelFromPriceLevel(flightPrice float64) float64 {
	switch {
	case flightPrice < 50:
		return 40 // very cheap destination
	case flightPrice < 100:
		return 60
	case flightPrice < 200:
		return 80
	case flightPrice < 400:
		return 100
	default:
		return 130
	}
}

// buildWeekendResult assembles a WeekendResult from explore destinations.
// It sorts by price, estimates hotels, filters by budget, and returns the result.
func buildWeekendResult(origin, displayMonth string, opts WeekendOptions, dests []models.ExploreDestination, apiCurrency string) *WeekendResult {
	// Sort by price and take top 10.
	sort.Slice(dests, func(i, j int) bool {
		return dests[i].Price < dests[j].Price
	})
	if len(dests) > 10 {
		dests = dests[:10]
	}

	// Build weekend destinations with hotel estimates.
	var results []WeekendDestination
	for _, d := range dests {
		hotelPerNight := estimateHotelFromPriceLevel(d.Price)
		hotelTotal := hotelPerNight * float64(opts.Nights)
		total := d.Price + hotelTotal // flight price is round-trip from explore

		if opts.MaxBudget > 0 && total > opts.MaxBudget {
			continue
		}

		cityName := d.CityName
		if cityName == "" {
			cityName = models.LookupAirportName(d.AirportCode)
		}

		results = append(results, WeekendDestination{
			Destination:   cityName,
			AirportCode:   d.AirportCode,
			FlightPrice:   d.Price,
			HotelEstimate: hotelTotal,
			TotalEstimate: total,
			Currency:      apiCurrency,
			Stops:         d.Stops,
			AirlineName:   d.AirlineName,
		})
	}

	// Sort by total estimate.
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalEstimate < results[j].TotalEstimate
	})

	return &WeekendResult{
		Success:      true,
		Origin:       origin,
		Month:        displayMonth,
		Nights:       opts.Nights,
		Count:        len(results),
		Destinations: results,
	}
}

// FindWeekendGetaways searches for cheap weekend getaway destinations from an origin.
//
// It uses the explore API to find the cheapest destinations, then estimates
// hotel costs based on flight price levels. Results are ranked by total
// estimated cost (flight round-trip + hotel).
func FindWeekendGetaways(ctx context.Context, origin string, opts WeekendOptions) (*WeekendResult, error) {
	opts.defaults()

	if origin == "" {
		return nil, fmt.Errorf("origin airport is required")
	}

	departDate, returnDate, displayMonth, err := parseMonth(opts.Month)
	if err != nil {
		return nil, err
	}

	client := batchexec.NewClient()

	exploreOpts := explore.ExploreOptions{
		DepartureDate: departDate,
		ReturnDate:    returnDate,
		Adults:        1,
	}

	exploreResult, err := explore.SearchExplore(ctx, client, origin, exploreOpts)
	if err != nil {
		return nil, fmt.Errorf("explore destinations: %w", err)
	}

	// Detect the actual API currency (explore returns prices without labels).
	apiCurrency := ""
	if len(exploreResult.Destinations) > 0 {
		apiCurrency = flights.DetectSourceCurrency(ctx, origin, exploreResult.Destinations[0].AirportCode)
	}

	return buildWeekendResult(origin, displayMonth, opts, exploreResult.Destinations, apiCurrency), nil
}
