package flights

import (
	"context"
	"errors"
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

// ParseLegs parses a list of "ORIGIN:DEST:DATE" specs into Legs. Shared by the
// CLI (--leg) and the MCP search_flights (legs param). The 2-leg minimum is
// enforced by SearchMultiCity, the single request entry point.
func ParseLegs(specs []string) ([]Leg, error) {
	legs := make([]Leg, 0, len(specs))
	for _, spec := range specs {
		leg, err := ParseLeg(spec)
		if err != nil {
			return nil, err
		}
		legs = append(legs, leg)
	}
	return legs, nil
}

// duffelSlicesForLegs maps multi-city legs to Duffel slices, using the first
// origin/destination airport of each leg (Duffel takes a single IATA code per
// endpoint, unlike Google which accepts an airport set).
func duffelSlicesForLegs(legs []Leg) []DuffelSlice {
	slices := make([]DuffelSlice, 0, len(legs))
	for _, leg := range legs {
		if len(leg.Origins) == 0 || len(leg.Destinations) == 0 {
			continue
		}
		slices = append(slices, DuffelSlice{
			Origin:        leg.Origins[0],
			Destination:   leg.Destinations[0],
			DepartureDate: leg.Date,
		})
	}
	return slices
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

	// Explicit provider allow-list: run only the listed providers (gating
	// bypassed). Empty list → default Google-primary / Duffel-fallback flow.
	if len(opts.Providers) > 0 {
		return searchMultiCityExplicit(ctx, legs, opts)
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
		// Google failed → try Duffel (paid fallback, native multi-city).
		if DuffelEnabled() {
			if duffelFlights, dErr := SearchDuffel(ctx, duffelSlicesForLegs(legs), opts); dErr == nil && len(duffelFlights) > 0 {
				return &models.FlightSearchResult{
					Success:  true,
					Count:    len(duffelFlights),
					TripType: "multi_city",
					Flights:  duffelFlights,
				}, nil
			}
		}
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

// searchMultiCityExplicit runs only the providers named in opts.Providers for a
// multi-city itinerary, merging their results. Google and Duffel both support
// native multi-city; Kiwi does not, so requesting only kiwi is an error.
func searchMultiCityExplicit(ctx context.Context, legs []Leg, opts SearchOptions) (*models.FlightSearchResult, error) {
	for i, leg := range legs {
		if len(leg.Origins) == 0 || len(leg.Destinations) == 0 || leg.Date == "" {
			return &models.FlightSearchResult{
				Error: fmt.Sprintf("leg %d is missing origin, destination, or date", i+1),
			}, fmt.Errorf("leg %d is missing origin, destination, or date", i+1)
		}
	}

	var combined []models.FlightResult
	var errs []error
	anySucceeded := false

	if providerListed(opts, "google") {
		segments := make([]any, len(legs))
		for i, leg := range legs {
			segments[i] = buildSegmentMulti(leg.Origins, leg.Destinations, leg.Date, opts)
		}
		filters := buildFiltersFromSegments(segments, 3, opts)
		if f, err := runGoogleFlightSearch(ctx, DefaultClient(), filters, opts); err != nil {
			errs = append(errs, fmt.Errorf("google: %w", err))
		} else {
			anySucceeded = true
			first := legs[0]
			bookingURL := buildFlightBookingURL(first.Origins[0], first.Destinations[0], first.Date, "", opts.Currency)
			for i := range f {
				f[i].BookingURL = bookingURL
			}
			combined = append(combined, f...)
		}
	}

	if providerListed(opts, "duffel") {
		if !DuffelEnabled() {
			errs = append(errs, fmt.Errorf("duffel: no API keys configured (set DUFFEL_API_KEYS or DUFFEL_API_KEY)"))
		} else if f, err := SearchDuffel(ctx, duffelSlicesForLegs(legs), opts); err != nil {
			errs = append(errs, fmt.Errorf("duffel: %w", err))
		} else {
			anySucceeded = true
			combined = append(combined, f...)
		}
	}

	if providerListed(opts, "kiwi") {
		errs = append(errs, fmt.Errorf("kiwi: multi-city search not supported"))
	}

	if anySucceeded {
		merged := mergeFlightResults(combined, nil, opts)
		return &models.FlightSearchResult{
			Success:  true,
			Count:    len(merged),
			TripType: "multi_city",
			Flights:  merged,
		}, nil
	}

	err := errors.Join(errs...)
	if err == nil {
		err = fmt.Errorf("no valid providers selected")
	}
	return &models.FlightSearchResult{Error: err.Error()}, err
}
