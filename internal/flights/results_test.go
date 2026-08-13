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

	defer stubFXRates(t, map[string]float64{"GBP": 0.80}, "2026-08-12")()

	annotateBagEstimates(flights, nil, 1)

	// Bag included: all-in is just the fare, with no spread.
	if flights[0].AllInMin != 129 || flights[0].AllInMax != 129 {
		t.Errorf("included bag: all-in = %v-%v, want 129-129", flights[0].AllInMin, flights[0].AllInMax)
	}
	// Fee in the fare's own currency: added at both ends.
	if flights[1].AllInMin != 96.49 || flights[1].AllInMax != 147 {
		t.Errorf("Ryanair: all-in = %v-%v, want 96.49-147", flights[1].AllInMin, flights[1].AllInMax)
	}
	// A GBP fee against a EUR fare is converted rather than dropped. Dropping
	// was the old behaviour and it was the expensive one: no all-in means the
	// flight leaves price comparison, and something dearer takes its place.
	// easyJet publishes GBP 6.99-60; at 1 GBP = 1.25 EUR that is EUR 8.7375-75.
	if !approx(flights[2].AllInMin, 114+8.7375) || !approx(flights[2].AllInMax, 114+75) {
		t.Errorf("converted fee: all-in = %v-%v, want %v-%v",
			flights[2].AllInMin, flights[2].AllInMax, 114+8.7375, 114+75)
	}
	// The conversion has to be auditable: a derived number must say so.
	est := flights[2].BagEstimate
	if est.ConversionRate != 1.25 {
		t.Errorf("conversion rate = %v, want the 1.25 the stub published", est.ConversionRate)
	}
	if est.ConversionAsOf != "2026-08-12" {
		t.Errorf("conversion date = %q, want the day the rate was published", est.ConversionAsOf)
	}
	if est.Currency != "GBP" || est.AmountMin != 6.99 {
		t.Errorf("the published figure must survive in the airline's own currency, got %s %v", est.Currency, est.AmountMin)
	}
	// Unknown terms: no total either, so a consumer cannot mistake the bare
	// fare for a complete one.
	if flights[3].AllInMin != 0 || flights[3].AllInMax != 0 {
		t.Errorf("unknown terms must not produce a total, got %v-%v", flights[3].AllInMin, flights[3].AllInMax)
	}
}

// TestAllInChargesTheBagPerDirection pins that a bag fee is multiplied by the
// number of directions flown. Airlines charge it per direction — Finnair and
// SWISS both state this on their own fee pages — so a round trip without an
// included bag pays twice. Charging once understated every round-trip total by
// roughly half, in the direction that gets cached and never corrected.
//
// An included bag is not multiplied: a fare that carries one carries it both
// ways.
func TestAllInChargesTheBagPerDirection(t *testing.T) {
	oneWay := []models.FlightResult{
		{Price: 1000, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "LX"}}}, // EUR 70-105
	}
	annotateBagEstimates(oneWay, nil, 1)
	if oneWay[0].AllInMin != 1070 || oneWay[0].AllInMax != 1105 {
		t.Errorf("one way: all-in = %v-%v, want 1070-1105", oneWay[0].AllInMin, oneWay[0].AllInMax)
	}

	roundTrip := []models.FlightResult{
		{Price: 1000, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "LX"}}},
	}
	annotateBagEstimates(roundTrip, nil, 2)
	if roundTrip[0].AllInMin != 1140 || roundTrip[0].AllInMax != 1210 {
		t.Errorf("round trip: all-in = %v-%v, want 1140-1210 (the bag paid each way)",
			roundTrip[0].AllInMin, roundTrip[0].AllInMax)
	}

	// An included bag costs nothing extra however many directions are flown.
	included := []models.FlightResult{
		{Price: 900, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "LH"}}},
	}
	annotateBagEstimates(included, nil, 3)
	if included[0].AllInMin != 900 || included[0].AllInMax != 900 {
		t.Errorf("included bag: all-in = %v-%v, want 900-900", included[0].AllInMin, included[0].AllInMax)
	}
}

// TestAllInWithoutAnyRateStaysSilent pins the limit of the conversion. Being
// able to convert most currencies is not a licence to invent the rest: when no
// rate exists for the pair, the total stays unset exactly as it did before,
// and the flight carries its published fee in the airline's own currency for a
// consumer to interpret.
func TestAllInWithoutAnyRateStaysSilent(t *testing.T) {
	// The stub publishes a EUR table without GBP, so easyJet's GBP fee has no
	// route to EUR.
	defer stubFXRates(t, map[string]float64{"USD": 1.10}, "2026-08-12")()

	flights := []models.FlightResult{
		{Price: 114, Currency: "EUR", Legs: []models.FlightLeg{{AirlineCode: "U2"}}},
	}
	annotateBagEstimates(flights, nil, 1)

	if flights[0].AllInMin != 0 || flights[0].AllInMax != 0 {
		t.Errorf("no rate for GBP→EUR: all-in = %v-%v, want no total rather than a guessed one",
			flights[0].AllInMin, flights[0].AllInMax)
	}
	est := flights[0].BagEstimate
	if est.ConversionRate != 0 || est.ConversionAsOf != "" {
		t.Errorf("nothing was converted, so no conversion may be reported: %+v", est)
	}
	if est.AmountMin != 6.99 || est.Currency != "GBP" {
		t.Errorf("the published fee must still be reported, got %v %s", est.AmountMin, est.Currency)
	}
}
