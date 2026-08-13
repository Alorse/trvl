package flights

import (
	"sort"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

const flightTimeLayout = "2006-01-02T15:04"

func mergeFlightResults(googleFlights, kiwiFlights []models.FlightResult, opts SearchOptions) []models.FlightResult {
	merged := make([]models.FlightResult, 0, len(googleFlights)+len(kiwiFlights))
	merged = append(merged, googleFlights...)
	merged = append(merged, kiwiFlights...)
	merged = filterFlightResults(merged, opts)
	sortFlightResults(merged, opts.SortBy)
	return merged
}

// fallbackSearchResult wraps results from a fallback provider (SerpApi, Duffel).
// Returns nil when nothing survives, so the caller keeps walking the provider
// chain instead of returning an empty success.
func fallbackSearchResult(flights []models.FlightResult, opts SearchOptions, tripType string) *models.FlightSearchResult {
	return searchResultFrom(flights, opts, tripType)
}

// searchResultFrom applies the filters, bag annotation and sort that every
// search path owes its results, then wraps the survivors. Every provider and
// every branch goes through here: the paths that skipped it returned unsorted,
// unfiltered results with no bag verdict, and each was found separately.
func searchResultFrom(flights []models.FlightResult, opts SearchOptions, tripType string) *models.FlightSearchResult {
	merged := mergeFlightResults(flights, nil, opts)
	if len(merged) == 0 {
		return nil
	}
	return &models.FlightSearchResult{
		Success:  true,
		Count:    len(merged),
		TripType: tripType,
		Flights:  merged,
	}
}

// annotateBagEstimates resolves every result's checked-bag situation from the
// best evidence available and records which evidence that was, so consumers can
// price a bag and see how far to trust the figure. Runs on every search, not
// only bag-filtered ones — trvl estimates trip cost, and a fare without its bag
// terms understates it.
//
// The airline is taken from the first leg, matching how the rest of the
// codebase attributes a flight. That is the wrong rule for interline
// itineraries, where the governing carrier may be neither the first nor the one
// the user sees, but changing it belongs with the other first-leg call sites.
func annotateBagEstimates(flights []models.FlightResult, ffStatuses []baggage.FFStatus, directions int) {
	for i := range flights {
		code := ""
		if len(flights[i].Legs) > 0 {
			code = flights[i].Legs[0].AirlineCode
		}
		est := baggage.ResolveCheckedBag(flights[i].CheckedBagsIncluded, code, ffStatuses)
		flights[i].BagEstimate = &est
		flights[i].AllInMin, flights[i].AllInMax = allInRange(flights[i].Price, flights[i].Currency, est, directions)
	}
}

// allInRange bounds the fare plus a checked bag, charged once per direction.
//
// Returns zeroes when no total can be stated honestly: an airline whose terms
// nobody reports, a carrier that publishes no figure, or a fee quoted in a
// currency we cannot convert into the fare's. Emitting the bare fare in those
// cases would be the same mistake the baggage table used to make — an unpriced
// bag reading as a free one, and the flight ranking cheaper than it is.
func allInRange(price float64, currency string, est models.BagEstimate, directions int) (float64, float64) {
	if price <= 0 {
		return 0, 0
	}
	if est.HasBag() {
		return price, price
	}
	if est.AmountMin <= 0 {
		return 0, 0 // fee varies, or the airline is not covered at all
	}
	if est.Currency != "" && currency != "" && !strings.EqualFold(est.Currency, currency) {
		return 0, 0
	}
	max := est.AmountMax
	if max < est.AmountMin {
		max = est.AmountMin
	}
	// The fee is charged per direction — Finnair and SWISS both state this on
	// their own fee pages — so a round trip without an included bag pays twice.
	// An included bag is not multiplied: a fare that carries one carries it
	// both ways, which is why this only applies below the Included branch.
	if directions < 1 {
		directions = 1
	}
	n := float64(directions)
	return price + est.AmountMin*n, price + max*n
}

func filterFlightResults(flights []models.FlightResult, opts SearchOptions) []models.FlightResult {
	if len(flights) == 0 {
		return nil
	}
	annotateBagEstimates(flights, opts.FFStatuses, opts.directionsFlown())

	filtered := make([]models.FlightResult, 0, len(flights))
	for _, f := range flights {
		if opts.MaxPrice > 0 && f.Price > float64(opts.MaxPrice) {
			continue
		}
		if opts.MaxDuration > 0 && f.Duration > opts.MaxDuration {
			continue
		}
		if opts.MaxStops == models.NonStop && f.Stops > 0 {
			continue
		}
		if opts.MaxStops == models.OneStop && f.Stops > 1 {
			continue
		}
		if !flightDepartsWithinWindow(f, opts.DepartAfter, opts.DepartBefore) {
			continue
		}
		filtered = append(filtered, f)
	}

	if len(opts.Airlines) > 0 {
		filtered = filterFlightsByAirline(filtered, opts.Airlines)
	}
	if opts.RequireCheckedBag {
		filtered = filterFlightsWithCheckedBag(filtered)
	}
	if len(opts.Alliances) > 0 {
		filtered = filterFlightsByAlliance(filtered, opts.Alliances)
	}

	return filtered
}

func filterFlightsByAirline(flights []models.FlightResult, airlines []string) []models.FlightResult {
	if len(flights) == 0 {
		return nil
	}

	want := make(map[string]bool, len(airlines))
	for _, airline := range airlines {
		code := strings.TrimSpace(strings.ToUpper(airline))
		if code != "" {
			want[code] = true
		}
	}
	if len(want) == 0 {
		return flights
	}

	filtered := flights[:0]
	for _, f := range flights {
		matched := false
		for _, leg := range f.Legs {
			if want[strings.ToUpper(leg.AirlineCode)] {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func flightDepartsWithinWindow(f models.FlightResult, after, before string) bool {
	if after == "" && before == "" {
		return true
	}
	if len(f.Legs) == 0 || len(f.Legs[0].DepartureTime) < len("2006-01-02T15:04") {
		return false
	}

	clock := f.Legs[0].DepartureTime[len("2006-01-02T"):]
	if after != "" && clock < after {
		return false
	}
	if before != "" && clock > before {
		return false
	}
	return true
}

func sortFlightResults(flights []models.FlightResult, sortBy models.SortBy) {
	sort.SliceStable(flights, func(i, j int) bool {
		left := flights[i]
		right := flights[j]

		switch sortBy {
		case models.SortDuration:
			if left.Duration != right.Duration {
				return left.Duration < right.Duration
			}
		case models.SortDepartureTime:
			if cmp := compareFlightTimes(flightDeparture(left), flightDeparture(right)); cmp != 0 {
				return cmp < 0
			}
		case models.SortArrivalTime:
			if cmp := compareFlightTimes(flightArrival(left), flightArrival(right)); cmp != 0 {
				return cmp < 0
			}
		default:
			if cmp := compareFlightPrices(left.Price, right.Price); cmp != 0 {
				return cmp < 0
			}
		}

		if cmp := compareFlightPrices(left.Price, right.Price); cmp != 0 {
			return cmp < 0
		}
		if left.Duration != right.Duration {
			return left.Duration < right.Duration
		}
		if cmp := compareFlightTimes(flightDeparture(left), flightDeparture(right)); cmp != 0 {
			return cmp < 0
		}
		if routeCmp := strings.Compare(flightSortKey(left), flightSortKey(right)); routeCmp != 0 {
			return routeCmp < 0
		}
		return strings.Compare(left.Provider, right.Provider) < 0
	})
}

func compareFlightPrices(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareFlightTimes(left, right time.Time) int {
	switch {
	case left.IsZero() && right.IsZero():
		return 0
	case left.IsZero():
		return 1
	case right.IsZero():
		return -1
	case left.Before(right):
		return -1
	case left.After(right):
		return 1
	default:
		return 0
	}
}

func flightDeparture(f models.FlightResult) time.Time {
	if len(f.Legs) == 0 {
		return time.Time{}
	}
	t, _ := time.Parse(flightTimeLayout, f.Legs[0].DepartureTime)
	return t
}

func flightArrival(f models.FlightResult) time.Time {
	if len(f.Legs) == 0 {
		return time.Time{}
	}
	t, _ := time.Parse(flightTimeLayout, f.Legs[len(f.Legs)-1].ArrivalTime)
	return t
}

func flightSortKey(f models.FlightResult) string {
	if len(f.Legs) == 0 {
		return ""
	}

	parts := []string{f.Legs[0].DepartureAirport.Code}
	for _, leg := range f.Legs {
		parts = append(parts, leg.ArrivalAirport.Code)
	}
	return strings.Join(parts, "->")
}

func flightSearchCurrency(result *models.FlightSearchResult) string {
	if result != nil {
		for _, f := range result.Flights {
			if f.Currency != "" {
				return f.Currency
			}
		}
	}
	return "EUR"
}

func tripTypeForSearch(opts SearchOptions) string {
	if opts.ReturnDate != "" {
		return "round_trip"
	}
	return "one_way"
}

func kiwiSearchEligible(client *batchexec.Client, opts SearchOptions) bool {
	if client == nil || client != batchexec.SharedClient() {
		return false
	}
	return kiwiEligibleOptions(opts)
}

func kiwiEligibleOptions(opts SearchOptions) bool {
	if opts.ReturnDate != "" {
		return false
	}
	if len(opts.Airlines) > 0 || len(opts.Alliances) > 0 {
		return false
	}
	if opts.CarryOnBags > 0 || opts.CheckedBags > 0 {
		return false
	}
	if opts.RequireCheckedBag || opts.ExcludeBasic || opts.LessEmissions {
		return false
	}
	return true
}
