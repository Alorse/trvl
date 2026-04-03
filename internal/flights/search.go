package flights

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
)

var (
	defaultClient     *batchexec.Client
	defaultClientOnce sync.Once
)

// DefaultClient returns a shared batchexec.Client for the flights package.
// The client is created once and reused across all requests, enabling
// connection reuse and shared rate limiting.
func DefaultClient() *batchexec.Client {
	defaultClientOnce.Do(func() {
		defaultClient = batchexec.NewClient()
	})
	return defaultClient
}

// SearchOptions configures a flight search.
type SearchOptions struct {
	ReturnDate string           // Return date for round-trip (YYYY-MM-DD); empty = one-way
	CabinClass models.CabinClass // Cabin class (default: Economy)
	MaxStops   models.MaxStops   // Maximum stops filter
	SortBy     models.SortBy     // Result sort order
	Airlines   []string          // Restrict to these airline IATA codes
	Adults     int               // Number of adult passengers (default: 1)
}

// defaults fills in zero-value fields with sensible defaults.
func (o *SearchOptions) defaults() {
	if o.Adults <= 0 {
		o.Adults = 1
	}
	if o.CabinClass == 0 {
		o.CabinClass = models.Economy
	}
}

// SearchFlights searches for flights from origin to destination on the given date.
//
// origin and destination are IATA airport codes (e.g. "HEL", "NRT").
// date is the departure date as "YYYY-MM-DD".
//
// Returns a FlightSearchResult with parsed flight options, or an error.
// Uses a shared default client for connection reuse and rate limiting.
func SearchFlights(ctx context.Context, origin, destination, date string, opts SearchOptions) (*models.FlightSearchResult, error) {
	return SearchFlightsWithClient(ctx, DefaultClient(), origin, destination, date, opts)
}

// SearchFlightsWithClient is like SearchFlights but accepts a pre-built client,
// useful for reusing connections across multiple requests.
func SearchFlightsWithClient(ctx context.Context, client *batchexec.Client, origin, destination, date string, opts SearchOptions) (*models.FlightSearchResult, error) {
	opts.defaults()

	if origin == "" || destination == "" || date == "" {
		return &models.FlightSearchResult{
			Error: "origin, destination, and date are required",
		}, fmt.Errorf("origin, destination, and date are required")
	}

	filters := buildFilters(origin, destination, date, opts)

	encoded, err := batchexec.EncodeFlightFilters(filters)
	if err != nil {
		return &models.FlightSearchResult{
			Error: fmt.Sprintf("encode filters: %v", err),
		}, fmt.Errorf("encode filters: %w", err)
	}

	status, body, err := client.SearchFlights(ctx, encoded)
	if err != nil {
		return &models.FlightSearchResult{
			Error: fmt.Sprintf("request failed: %v", err),
		}, fmt.Errorf("request failed: %w", err)
	}

	if status == 403 {
		return &models.FlightSearchResult{
			Error: "blocked by Google (403)",
		}, batchexec.ErrBlocked
	}
	if status != 200 {
		return &models.FlightSearchResult{
			Error: fmt.Sprintf("unexpected status %d", status),
		}, fmt.Errorf("unexpected status %d", status)
	}

	inner, err := batchexec.DecodeFlightResponse(body)
	if err != nil {
		return &models.FlightSearchResult{
			Error: fmt.Sprintf("decode response: %v", err),
		}, fmt.Errorf("decode response: %w", err)
	}

	rawFlights, err := batchexec.ExtractFlightData(inner)
	if err != nil {
		return &models.FlightSearchResult{
			Error: fmt.Sprintf("extract flights: %v", err),
		}, fmt.Errorf("extract flights: %w", err)
	}

	flights := parseFlights(rawFlights)

	// Add booking URLs and convert currency to EUR if needed.
	for i := range flights {
		flights[i].BookingURL = buildFlightBookingURL(origin, destination, date)
		if flights[i].Price > 0 && flights[i].Currency != "" && flights[i].Currency != "EUR" {
			converted, _ := destinations.ConvertCurrency(ctx, flights[i].Price, flights[i].Currency, "EUR")
			flights[i].Price = math.Round(converted)
			flights[i].Currency = "EUR"
		}
	}

	tripType := "one_way"
	if opts.ReturnDate != "" {
		tripType = "round_trip"
	}

	return &models.FlightSearchResult{
		Success:  true,
		Count:    len(flights),
		TripType: tripType,
		Flights:  flights,
	}, nil
}

// buildFlightBookingURL constructs a Google Flights deep link for a route and date.
func buildFlightBookingURL(origin, destination, date string) string {
	return fmt.Sprintf("https://www.google.com/travel/flights?q=Flights+to+%s+from+%s+on+%s", destination, origin, date)
}

// buildFilters constructs the nested array structure for the flight search payload.
// This extends batchexec.BuildFlightFilters with support for cabin class, stops,
// round-trip, sort order, and airline filters.
func buildFilters(origin, destination, date string, opts SearchOptions) any {
	// Outbound segment
	outbound := buildSegment(origin, destination, date, opts)

	segments := []any{outbound}

	// Add return segment for round-trip
	if opts.ReturnDate != "" {
		ret := buildSegment(destination, origin, opts.ReturnDate, opts)
		segments = append(segments, ret)
	}

	// Trip type: 2 = one-way, 1 = round-trip
	tripType := 2
	if opts.ReturnDate != "" {
		tripType = 1
	}

	// Sort by: Google uses 1=best, 2=price, 3=duration, 4=departure, 5=arrival
	sortBy := 1 // default: best
	switch opts.SortBy {
	case models.SortCheapest:
		sortBy = 2
	case models.SortDuration:
		sortBy = 3
	case models.SortDepartureTime:
		sortBy = 4
	case models.SortArrivalTime:
		sortBy = 5
	}

	filters := []any{
		// outer[0]: empty array (flights mode)
		[]any{},
		// outer[1]: settings array
		[]any{
			nil,                                          // [0]
			nil,                                          // [1]
			tripType,                                     // [2] trip type
			nil,                                          // [3]
			[]any{},                                      // [4]
			int(opts.CabinClass),                         // [5] cabin class
			[]any{opts.Adults, 0, 0, 0},                  // [6] passengers
			nil,                                          // [7] price limit
			nil,                                          // [8]
			nil,                                          // [9]
			nil,                                          // [10] bags
			nil,                                          // [11]
			nil,                                          // [12]
			segments,                                     // [13] flight segments
			nil,                                          // [14]
			nil,                                          // [15]
			nil,                                          // [16]
			1,                                            // [17]
			nil,                                          // [18]
			nil,                                          // [19]
			nil,                                          // [20]
			nil,                                          // [21]
			nil,                                          // [22]
			nil,                                          // [23]
			nil,                                          // [24]
			nil,                                          // [25]
			nil,                                          // [26]
			nil,                                          // [27]
			0,                                            // [28] exclude basic economy
		},
		// outer[2]: sort by
		sortBy,
		// outer[3]: show all
		1,
		// outer[4]
		0,
		// outer[5]
		1,
	}

	return filters
}

// buildSegment constructs a single flight segment (one direction).
func buildSegment(from, to, date string, opts SearchOptions) any {
	// Build airlines filter
	var airlines any
	if len(opts.Airlines) > 0 {
		airlineEntries := make([]any, len(opts.Airlines))
		for i, code := range opts.Airlines {
			airlineEntries[i] = code
		}
		airlines = airlineEntries
	}

	// MaxStops: 0=any, 1=nonstop, 2=1stop, 3=2+stops
	stops := int(opts.MaxStops)

	return []any{
		// [0] departure airports
		[]any{[]any{[]any{from, 0}}},
		// [1] arrival airports
		[]any{[]any{[]any{to, 0}}},
		// [2] time restrictions
		nil,
		// [3] stops
		stops,
		// [4] airlines
		airlines,
		// [5]
		nil,
		// [6] date
		date,
		// [7] max duration
		nil,
		// [8] selected flight
		nil,
		// [9] layover airports
		nil,
		// [10]
		nil,
		// [11]
		nil,
		// [12] layover duration
		nil,
		// [13] emissions
		nil,
		// [14]
		3,
	}
}
