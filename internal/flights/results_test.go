package flights

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// TestRequireCheckedBagKeepsUnknownMarked pins the require_checked_bag
// semantics: a nil CheckedBagsIncluded means the provider never reported the
// field (SerpApi on routes where Google omits it), not "no bag". Dropping those
// silently emptied every SerpApi result set. They are kept and marked instead.
func TestRequireCheckedBagKeepsUnknownMarked(t *testing.T) {
	zero, one := 0, 1
	flights := []models.FlightResult{
		{Price: 100, Provider: "google_serpapi", CheckedBagsIncluded: &zero},
		{Price: 200, Provider: "google_flights", CheckedBagsIncluded: &one},
		{Price: 300, Provider: "google_serpapi", CheckedBagsIncluded: nil},
	}

	got := filterFlightResults(flights, SearchOptions{RequireCheckedBag: true})

	if len(got) != 2 {
		t.Fatalf("kept %d flights, want 2 (the included one + the unknown one)", len(got))
	}
	if got[0].Price != 200 {
		t.Errorf("first kept price = %.0f, want 200 (bag included)", got[0].Price)
	}
	if got[1].Price != 300 {
		t.Fatalf("second kept price = %.0f, want 300 (bag unknown)", got[1].Price)
	}
	if len(got[1].Warnings) == 0 {
		t.Error("unknown-bag flight must be marked with a warning")
	}
	if len(got[0].Warnings) != 0 {
		t.Errorf("known-bag flight must not be marked, got %v", got[0].Warnings)
	}
}

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
