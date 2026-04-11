package flights

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
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

	// Server-side filters passed to Google Flights batchexecute.
	MaxPrice      int    // Max price in whole currency units (0 = no limit)
	MaxDuration   int    // Max total flight duration in minutes (0 = no limit)
	CarryOnBags   int    // Number of carry-on bags (0 = no filter, 1 = require carry-on included)
	// Note: Google Flights UI only supports carry-on bag filter. CheckedBags
	// was removed after research confirmed no server-side checked bag filter exists.
	ExcludeBasic  bool   // Exclude basic economy fares
	Alliances     []string // Alliance filter; e.g. ["STAR_ALLIANCE", "ONEWORLD", "SKYTEAM"]
	DepartAfter   string // Earliest departure time "HH:MM" (e.g. "06:00")
	DepartBefore  string // Latest departure time "HH:MM" (e.g. "22:00")
	LessEmissions bool   // Only show flights with less emissions

	// Client-side post-filters (applied after server response).
	RequireCheckedBag bool // Only show flights with ≥1 free checked bag
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

	// Add booking URLs. Prices are in the API's native currency (IP-based).
	// Currency conversion, if needed, happens in the CLI display layer.
	for i := range flights {
		flights[i].BookingURL = buildFlightBookingURL(origin, destination, date)
	}

	// Client-side post-filter: require checked bag included.
	// This is a unique feature — Google returns bag data in responses but
	// doesn't offer a server-side checked bag filter.
	if opts.RequireCheckedBag {
		flights = filterFlightsWithCheckedBag(flights)
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
			priceLimit(opts.MaxPrice),                     // [7] max price (nil or int)
			nil,                                          // [8]
			nil,                                          // [9]
			bagsFilter(opts.CarryOnBags),                    // [10] carry-on bags filter
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
			alliancesFilter(opts.Alliances),              // [25] alliance filter
			nil,                                          // [26]
			nil,                                          // [27]
			excludeBasicEconomy(opts.ExcludeBasic),        // [28] exclude basic economy
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
		// [2] departure time window [startHour, endHour] or nil
		departTimeWindow(opts.DepartAfter, opts.DepartBefore),
		// [3] stops
		stops,
		// [4] airlines
		airlines,
		// [5]
		nil,
		// [6] date
		date,
		// [7] max duration in minutes
		durationLimit(opts.MaxDuration),
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
		// [13] emissions filter (1 = less emissions only)
		emissionsFilter(opts.LessEmissions),
		// [14]
		3,
	}
}

// priceLimit returns the max price for the batchexecute filter, or nil if unset.
func priceLimit(maxPrice int) any {
	if maxPrice <= 0 {
		return nil
	}
	return maxPrice
}

// bagsFilter returns the carry-on bags count for the batchexecute filter, or nil if unset.
// Google Flights only supports carry-on bag filtering (verified: UI toggle + SerpAPI).
func bagsFilter(carryOn int) any {
	if carryOn <= 0 {
		return nil
	}
	return carryOn
}

// filterFlightsWithCheckedBag returns only flights that include at least one
// free checked bag. This is a client-side post-filter since Google's
// batchexecute API has no server-side checked bag filter.
func filterFlightsWithCheckedBag(flights []models.FlightResult) []models.FlightResult {
	filtered := flights[:0]
	for _, f := range flights {
		if f.CheckedBagsIncluded != nil && *f.CheckedBagsIncluded >= 1 {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// durationLimit returns the max duration in minutes, or nil if unset.
func durationLimit(maxDuration int) any {
	if maxDuration <= 0 {
		return nil
	}
	return maxDuration
}

// excludeBasicEconomy returns the flag for the batchexecute filter.
func excludeBasicEconomy(exclude bool) int {
	if exclude {
		return 1
	}
	return 0
}

// alliancesFilter returns the alliances array for the batchexecute filter,
// or nil if no alliances are specified.
//
// Accepted alliance names (case-insensitive): STAR_ALLIANCE, ONEWORLD, SKYTEAM.
// Unknown values are passed through as-is to avoid silently dropping filters.
func alliancesFilter(alliances []string) any {
	if len(alliances) == 0 {
		return nil
	}
	entries := make([]any, len(alliances))
	for i, a := range alliances {
		entries[i] = strings.ToUpper(strings.TrimSpace(a))
	}
	return entries
}

// departTimeWindow parses "HH:MM" strings and returns the segment [2] value
// []any{startHour, endHour}, or nil when neither bound is set.
// Malformed values are silently ignored (treated as unset).
func departTimeWindow(after, before string) any {
	start := parseHour(after)
	end := parseHour(before)
	if start < 0 && end < 0 {
		return nil
	}
	// Use 0 for an unset lower bound and 24 for an unset upper bound so the
	// API sees a well-formed window even when only one side is specified.
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 24
	}
	return []any{start, end}
}

// parseHour parses a strict "HH:MM" string (exactly 5 characters) and returns
// the hour as an integer [0, 23]. Returns -1 on any parse error or out-of-range value.
func parseHour(hhmm string) int {
	// Must be exactly "HH:MM" — 5 characters, colon at index 2.
	if len(hhmm) != 5 || hhmm[2] != ':' {
		return -1
	}
	h0, h1 := hhmm[0], hhmm[1]
	if h0 < '0' || h0 > '9' || h1 < '0' || h1 > '9' {
		return -1
	}
	m0, m1 := hhmm[3], hhmm[4]
	if m0 < '0' || m0 > '9' || m1 < '0' || m1 > '9' {
		return -1
	}
	hour := int(h0-'0')*10 + int(h1-'0')
	if hour > 23 {
		return -1
	}
	return hour
}

// emissionsFilter returns 1 when the less-emissions filter is active, nil otherwise.
func emissionsFilter(less bool) any {
	if less {
		return 1
	}
	return nil
}
