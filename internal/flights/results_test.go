package flights

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// TestFallbackSearchResultAppliesFilters covers the fallback providers (SerpApi,
// Duffel). Their results used to be returned raw, so max_price / max_stops /
// airlines / require_checked_bag were silently ignored whenever Google failed —
// the mirror image of the explicit --provider path, which did filter them.
func TestFallbackSearchResultAppliesFilters(t *testing.T) {
	flights := []models.FlightResult{
		{Price: 900, Provider: "google_serpapi", Stops: 0},
		{Price: 100, Provider: "google_serpapi", Stops: 3},
		{Price: 200, Provider: "google_serpapi", Stops: 0},
	}

	got := fallbackSearchResult(flights, SearchOptions{MaxPrice: 500, MaxStops: models.NonStop}, "one_way")

	if got == nil {
		t.Fatal("expected a result, got nil")
	}
	if len(got.Flights) != 1 || got.Flights[0].Price != 200 {
		t.Fatalf("expected only the 200 non-stop under budget, got %+v", got.Flights)
	}
	if got.Count != 1 {
		t.Errorf("Count = %d, want 1 (must match filtered length)", got.Count)
	}
	if !got.Success || got.TripType != "one_way" {
		t.Errorf("Success/TripType = %v/%q", got.Success, got.TripType)
	}

	// Everything filtered out → nil, so the caller keeps walking the chain.
	if r := fallbackSearchResult(flights, SearchOptions{MaxPrice: 1}, "one_way"); r != nil {
		t.Errorf("expected nil when all results are filtered out, got %+v", r)
	}
	if r := fallbackSearchResult(nil, SearchOptions{}, "one_way"); r != nil {
		t.Errorf("expected nil for empty input, got %+v", r)
	}
}

func TestMergeFlightResults_SortsCheapestAndFiltersStops(t *testing.T) {
	googleFlights := []models.FlightResult{
		{
			Price:    200,
			Currency: "EUR",
			Duration: 120,
			Stops:    0,
			Provider: "google_flights",
			Legs: []models.FlightLeg{
				{
					DepartureAirport: models.AirportInfo{Code: "HEL"},
					ArrivalAirport:   models.AirportInfo{Code: "DBV"},
					DepartureTime:    "2026-07-01T08:00",
					ArrivalTime:      "2026-07-01T10:00",
				},
			},
		},
	}
	kiwiFlights := []models.FlightResult{
		{
			Price:    150,
			Currency: "EUR",
			Duration: 300,
			Stops:    2,
			Provider: "kiwi",
			Legs: []models.FlightLeg{
				{DepartureAirport: models.AirportInfo{Code: "HEL"}, ArrivalAirport: models.AirportInfo{Code: "ARN"}, DepartureTime: "2026-07-01T06:00", ArrivalTime: "2026-07-01T07:00"},
				{DepartureAirport: models.AirportInfo{Code: "ARN"}, ArrivalAirport: models.AirportInfo{Code: "WAW"}, DepartureTime: "2026-07-01T08:00", ArrivalTime: "2026-07-01T09:00"},
				{DepartureAirport: models.AirportInfo{Code: "WAW"}, ArrivalAirport: models.AirportInfo{Code: "DBV"}, DepartureTime: "2026-07-01T10:00", ArrivalTime: "2026-07-01T11:00"},
			},
		},
		{
			Price:    175,
			Currency: "EUR",
			Duration: 180,
			Stops:    1,
			Provider: "kiwi",
			Legs: []models.FlightLeg{
				{DepartureAirport: models.AirportInfo{Code: "HEL"}, ArrivalAirport: models.AirportInfo{Code: "ARN"}, DepartureTime: "2026-07-01T07:00", ArrivalTime: "2026-07-01T08:00"},
				{DepartureAirport: models.AirportInfo{Code: "ARN"}, ArrivalAirport: models.AirportInfo{Code: "DBV"}, DepartureTime: "2026-07-01T09:00", ArrivalTime: "2026-07-01T10:00"},
			},
		},
	}

	merged := mergeFlightResults(googleFlights, kiwiFlights, SearchOptions{
		MaxStops: models.OneStop,
		SortBy:   models.SortCheapest,
	})

	if len(merged) != 2 {
		t.Fatalf("merged count = %d, want 2", len(merged))
	}
	if merged[0].Price != 175 {
		t.Fatalf("first price = %.0f, want 175", merged[0].Price)
	}
	if merged[1].Price != 200 {
		t.Fatalf("second price = %.0f, want 200", merged[1].Price)
	}
}

// TestBagEstimateDrivesTheFilter pins the resolution cascade end to end: every
// result carries a provenance-tagged verdict, and require_checked_bag acts on
// that verdict rather than on the raw provider field. Without the table step a
// Lufthansa long-haul — where Google states no allowance — would be dropped
// even though the airline includes a bag.
func TestBagEstimateDrivesTheFilter(t *testing.T) {
	zero := 0
	flights := []models.FlightResult{
		{Price: 100, Legs: []models.FlightLeg{{AirlineCode: "LH"}}},                             // silent → table says included
		{Price: 200, Legs: []models.FlightLeg{{AirlineCode: "FR"}}},                             // silent → table says none
		{Price: 300, Legs: []models.FlightLeg{{AirlineCode: "JU"}}},                             // silent → not covered
		{Price: 400, Legs: []models.FlightLeg{{AirlineCode: "LH"}}, CheckedBagsIncluded: &zero}, // provider overrides table
	}

	got := filterFlightResults(flights, SearchOptions{RequireCheckedBag: true})

	if len(got) != 1 || got[0].Price != 100 {
		t.Fatalf("expected only the Lufthansa fare without a provider verdict, got %+v", got)
	}
	if got[0].BagEstimate == nil {
		t.Fatal("every result must carry a bag estimate")
	}
	if got[0].BagEstimate.Source != models.BagSourceTableSourced {
		t.Errorf("source = %q, want the cited table entry to be credited", got[0].BagEstimate.Source)
	}

	// Unfiltered searches still get the annotation, so consumers can price it.
	all := filterFlightResults(flights, SearchOptions{})
	if len(all) != 4 {
		t.Fatalf("no filter means no drops, got %d", len(all))
	}
	for i, f := range all {
		if f.BagEstimate == nil {
			t.Fatalf("flight %d has no bag estimate", i)
		}
	}
	if all[2].BagEstimate.Source != models.BagSourceUnknown || all[2].BagEstimate.AmountMin != 0 {
		t.Errorf("uncovered airline must be unknown with no invented fee, got %+v", all[2].BagEstimate)
	}
	if all[3].BagEstimate.Source != models.BagSourceProvider {
		t.Errorf("provider verdict must win over the table, got %q", all[3].BagEstimate.Source)
	}
}

// TestAllInRangeFromBagEstimate covers the all-in cost: the fare plus what a
// checked bag would add, expressed as a range because published fees swing
// several-fold within one carrier. The floor is what a comparison should sort
// on; the ceiling is what the traveller might actually pay.
func TestAllInRangeFromBagEstimate(t *testing.T) {
	flights := []models.FlightResult{
		{Price: 129, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "LH"}}}, // bag included, and cited so it survives the staleness guard
		{Price: 87, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "FR"}}},  // EUR 9.49-60
		{Price: 114, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "U2"}}}, // range is GBP, fare is EUR
		{Price: 121, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "JU"}}}, // not in the table
	}

	annotateBagEstimates(flights, nil)

	// Bag included: all-in is just the fare, with no spread.
	if flights[0].AllInMin != 129 || flights[0].AllInMax != 129 {
		t.Errorf("included bag: all-in = %v-%v, want 129-129", flights[0].AllInMin, flights[0].AllInMax)
	}
	// Fee in the fare's own currency: added at both ends.
	if flights[1].AllInMin != 96.49 || flights[1].AllInMax != 147 {
		t.Errorf("Ryanair: all-in = %v-%v, want 96.49-147", flights[1].AllInMin, flights[1].AllInMax)
	}
	// A GBP fee cannot be added to a EUR fare without a rate we do not have.
	// Leaving it unset is honest; inventing a conversion is not.
	if flights[2].AllInMin != 0 || flights[2].AllInMax != 0 {
		t.Errorf("currency mismatch must not produce a total, got %v-%v", flights[2].AllInMin, flights[2].AllInMax)
	}
	// Unknown terms: no total either, so a consumer cannot mistake the bare
	// fare for a complete one.
	if flights[3].AllInMin != 0 || flights[3].AllInMax != 0 {
		t.Errorf("unknown terms must not produce a total, got %v-%v", flights[3].AllInMin, flights[3].AllInMax)
	}
}
