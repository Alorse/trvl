package flights

import (
	"context"
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// Leg is one segment of a multi-city itinerary. Origins and Destinations each
// hold one or more IATA airport codes (a city name expands to all its airports,
// all covered in a single request).
type Leg struct {
	Origins      []string
	Destinations []string
	Date         string
}

// ParseLeg parses an "ORIGIN:DEST:DATE" leg specification. Each of ORIGIN and
// DEST may be an IATA code or a city name (or a comma-separated list); city
// names are expanded to their airports via ParseFlightLocations. DATE must be a
// valid future YYYY-MM-DD date.
func ParseLeg(s string) (Leg, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return Leg{}, fmt.Errorf("invalid leg %q: expected ORIGIN:DEST:DATE (e.g. Paris:Tokyo:2026-09-01)", s)
	}

	origins := ParseFlightLocations(parts[0])
	destinations := ParseFlightLocations(parts[1])
	date := strings.TrimSpace(parts[2])

	if len(origins) == 0 {
		return Leg{}, fmt.Errorf("invalid leg %q: missing origin", s)
	}
	if len(destinations) == 0 {
		return Leg{}, fmt.Errorf("invalid leg %q: missing destination", s)
	}
	if err := models.ValidateDate(date); err != nil {
		return Leg{}, fmt.Errorf("invalid leg %q: %w", s, err)
	}

	return Leg{Origins: origins, Destinations: destinations, Date: date}, nil
}

// SearchMultiCity searches a multi-city itinerary (Google Flights tripType=3).
// Like a round-trip, Google returns options for the first leg priced at the
// combined itinerary total. Requires at least 2 legs. Google-only in v1.
func SearchMultiCity(ctx context.Context, legs []Leg, opts SearchOptions) (*models.FlightSearchResult, error) {
	opts.defaults()

	if len(legs) < 2 {
		return &models.FlightSearchResult{
			Error: "multi-city requires at least 2 legs",
		}, fmt.Errorf("multi-city requires at least 2 legs")
	}

	segments := make([]any, len(legs))
	for i, leg := range legs {
		if len(leg.Origins) == 0 || len(leg.Destinations) == 0 || leg.Date == "" {
			return &models.FlightSearchResult{
				Error: fmt.Sprintf("leg %d is missing origin, destination, or date", i+1),
			}, fmt.Errorf("leg %d is missing origin, destination, or date", i+1)
		}
		segments[i] = buildSegmentMulti(leg.Origins, leg.Destinations, leg.Date, opts)
	}

	// tripType 3 = multi-city.
	filters := buildFiltersFromSegments(segments, 3, opts)

	flights, err := runGoogleFlightSearch(ctx, DefaultClient(), filters, opts)
	if err != nil {
		return &models.FlightSearchResult{Error: err.Error()}, err
	}

	// Best-effort booking link: deep link for the first leg. Google's real
	// multi-city flow selects each leg in turn, so a single combined link
	// isn't available; the first-leg link is a useful starting point.
	first := legs[0]
	bookingURL := buildFlightBookingURL(first.Origins[0], first.Destinations[0], first.Date, "", opts.Currency)
	for i := range flights {
		flights[i].BookingURL = bookingURL
	}

	return &models.FlightSearchResult{
		Success:  true,
		Count:    len(flights),
		TripType: "multi_city",
		Flights:  flights,
	}, nil
}
