package trip

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// OptimizeTripDatesInput configures a trip date optimization search.
type OptimizeTripDatesInput struct {
	Origin      string // IATA code
	Destination string // IATA code or city name
	FromDate    string // start of search range (YYYY-MM-DD)
	ToDate      string // end of search range (YYYY-MM-DD)
	TripLength  int    // trip length in nights
	Guests      int    // number of guests
	Currency    string // target currency
}

// DateOption represents one possible departure date with its estimated cost.
type DateOption struct {
	DepartDate string  `json:"depart_date"`
	ReturnDate string  `json:"return_date"`
	FlightCost float64 `json:"flight_cost"`
	HotelCost  float64 `json:"hotel_cost,omitempty"`
	TotalCost  float64 `json:"total_cost"`
	Currency   string  `json:"currency"`
	Savings    float64 `json:"savings"` // savings vs most expensive date
}

// OptimizeTripDatesResult is the response for a trip date optimization.
type OptimizeTripDatesResult struct {
	Success    bool         `json:"success"`
	BestDate   *DateOption  `json:"best_date,omitempty"`
	Options    []DateOption `json:"options"`
	MaxSavings float64      `json:"max_savings"` // savings of best vs worst
	Currency   string       `json:"currency"`
	Error      string       `json:"error,omitempty"`
}

// OptimizeTripDates finds the cheapest departure date within a date range
// by searching flight prices across the range using the calendar API, then
// ranking by total flight cost.
func OptimizeTripDates(ctx context.Context, input OptimizeTripDatesInput) (*OptimizeTripDatesResult, error) {
	if input.Origin == "" || input.Destination == "" {
		return nil, fmt.Errorf("origin and destination are required")
	}
	if input.FromDate == "" || input.ToDate == "" {
		return nil, fmt.Errorf("from_date and to_date are required")
	}
	if input.TripLength <= 0 {
		return nil, fmt.Errorf("trip_length must be at least 1")
	}
	if input.Guests <= 0 {
		input.Guests = 1
	}

	fromDate, err := models.ParseDate(input.FromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid from_date %q: %w", input.FromDate, err)
	}
	toDate, err := models.ParseDate(input.ToDate)
	if err != nil {
		return nil, fmt.Errorf("invalid to_date %q: %w", input.ToDate, err)
	}
	if !toDate.After(fromDate) {
		return nil, fmt.Errorf("to_date must be after from_date")
	}

	// Use SearchCalendar for round-trip flight prices across the range.
	calResult, err := flights.SearchCalendar(ctx, input.Origin, input.Destination, flights.CalendarOptions{
		FromDate:   input.FromDate,
		ToDate:     input.ToDate,
		TripLength: input.TripLength,
		RoundTrip:  true,
		Adults:     1, // per-person pricing
	})
	if err != nil {
		return nil, fmt.Errorf("calendar search: %w", err)
	}
	if calResult == nil || len(calResult.Dates) == 0 {
		return &OptimizeTripDatesResult{
			Success: false,
			Error:   "no flight prices found for the date range",
		}, nil
	}

	// Build date options from calendar prices.
	options := buildDateOptions(calResult.Dates, input)

	if len(options) == 0 {
		return &OptimizeTripDatesResult{
			Success: false,
			Error:   "no valid date options found",
		}, nil
	}

	// Sort by flight cost to find cheapest.
	sort.Slice(options, func(i, j int) bool {
		return options[i].FlightCost < options[j].FlightCost
	})

	// Cap to top 10 for presentation.
	if len(options) > 10 {
		options = options[:10]
	}

	// Calculate savings vs the most expensive option in our top-10.
	maxCost := options[len(options)-1].FlightCost
	for i := range options {
		options[i].TotalCost = options[i].FlightCost
		options[i].Savings = math.Round(maxCost - options[i].FlightCost)
	}

	result := &OptimizeTripDatesResult{
		Success:    true,
		BestDate:   &options[0],
		Options:    options,
		MaxSavings: math.Round(maxCost - options[0].FlightCost),
		Currency:   options[0].Currency,
	}
	return result, nil
}

// buildDateOptions converts calendar DatePriceResult entries into DateOption
// entries, filtering out zero-price dates and computing per-guest costs.
func buildDateOptions(dates []models.DatePriceResult, input OptimizeTripDatesInput) []DateOption {
	options := make([]DateOption, 0, len(dates))
	for _, dp := range dates {
		if dp.Price <= 0 {
			continue
		}
		departDate, parseErr := models.ParseDate(dp.Date)
		if parseErr != nil {
			continue
		}
		returnDate := departDate.AddDate(0, 0, input.TripLength)

		currency := input.Currency
		if currency == "" {
			currency = dp.Currency
		}

		options = append(options, DateOption{
			DepartDate: dp.Date,
			ReturnDate: returnDate.Format("2006-01-02"),
			FlightCost: dp.Price * float64(input.Guests),
			Currency:   currency,
		})
	}
	return options
}
