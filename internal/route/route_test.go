package route

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/ground"
	"github.com/MikkoParkkola/trvl/internal/models"
)

func withSearchMocks(
	t *testing.T,
	flight func(context.Context, string, string, string, flights.SearchOptions) (*models.FlightSearchResult, error),
	groundSearch func(context.Context, string, string, string, ground.SearchOptions) (*models.GroundSearchResult, error),
	convert func(context.Context, float64, string, string) (float64, string),
) {
	t.Helper()

	prevFlight := searchFlightsFunc
	prevGround := searchGroundByNameFunc
	prevConvert := convertCurrencyFunc

	if flight != nil {
		searchFlightsFunc = flight
	}
	if groundSearch != nil {
		searchGroundByNameFunc = groundSearch
	}
	if convert != nil {
		convertCurrencyFunc = convert
	}

	t.Cleanup(func() {
		searchFlightsFunc = prevFlight
		searchGroundByNameFunc = prevGround
		convertCurrencyFunc = prevConvert
	})
}

func TestLookupHub(t *testing.T) {
	tests := []struct {
		city string
		want bool
	}{
		{"Helsinki", true},
		{"helsinki", true},
		{"Vienna", true},
		{"Dubrovnik", true},
		{"Nonexistent", false},
	}
	for _, tt := range tests {
		_, ok := LookupHub(tt.city)
		if ok != tt.want {
			t.Errorf("LookupHub(%q) = %v, want %v", tt.city, ok, tt.want)
		}
	}
}

func TestCityForAirport(t *testing.T) {
	tests := []struct {
		iata string
		want string
	}{
		{"HEL", "Helsinki"},
		{"VIE", "Vienna"},
		{"DBV", "Dubrovnik"},
		{"XXX", "XXX"}, // unknown
	}
	for _, tt := range tests {
		got := CityForAirport(tt.iata)
		if got != tt.want {
			t.Errorf("CityForAirport(%q) = %q, want %q", tt.iata, got, tt.want)
		}
	}
}

func TestAirportForCity(t *testing.T) {
	got := AirportForCity("Helsinki")
	if got != "HEL" {
		t.Errorf("AirportForCity(Helsinki) = %q, want HEL", got)
	}
	got = AirportForCity("Nonexistent")
	if got != "" {
		t.Errorf("AirportForCity(Nonexistent) = %q, want empty", got)
	}
}

func TestHaversineKm(t *testing.T) {
	// Helsinki to Tallinn: ~80km
	d := haversineKm(60.1699, 24.9384, 59.437, 24.7536)
	if d < 60 || d > 100 {
		t.Errorf("Helsinki→Tallinn distance = %.0f km, want 60-100", d)
	}
	// Helsinki to Dubrovnik: ~1800km
	d = haversineKm(60.1699, 24.9384, 42.6507, 18.0944)
	if d < 1700 || d > 2100 {
		t.Errorf("Helsinki→Dubrovnik distance = %.0f km, want 1700-2000", d)
	}
}

func TestCandidateHubs(t *testing.T) {
	hel, _ := LookupHub("Helsinki")
	dbv, _ := LookupHub("Dubrovnik")
	candidates := CandidateHubs(hel, dbv, 2.0)

	// Should include Vienna (on the path) but not Lisbon (too far west).
	foundVienna := false
	foundLisbon := false
	for _, h := range candidates {
		if h.City == "Vienna" {
			foundVienna = true
		}
		if h.City == "Lisbon" {
			foundLisbon = true
		}
	}
	if !foundVienna {
		t.Error("CandidateHubs(HEL→DBV) should include Vienna")
	}
	if foundLisbon {
		t.Error("CandidateHubs(HEL→DBV) should NOT include Lisbon")
	}
}

func TestMinConnectionTime(t *testing.T) {
	tests := []struct {
		prev, next string
		wantMin    int // minutes
	}{
		{"train", "train", 30},
		{"flight", "train", 120},
		{"ferry", "flight", 120},
		{"unknown", "unknown", 60},
	}
	for _, tt := range tests {
		got := MinConnectionTime(tt.prev, tt.next)
		gotMin := int(got.Minutes())
		if gotMin != tt.wantMin {
			t.Errorf("MinConnectionTime(%s→%s) = %d min, want %d", tt.prev, tt.next, gotMin, tt.wantMin)
		}
	}
}

func TestIsConnectionFeasible(t *testing.T) {
	// 3h gap between flight arrival and train departure — should be feasible.
	ok := IsConnectionFeasible("2026-04-10T10:00:00", "2026-04-10T13:00:00", "flight", "train")
	if !ok {
		t.Error("3h gap flight→train should be feasible")
	}

	// 30min gap between flight arrival and train departure — too short.
	ok = IsConnectionFeasible("2026-04-10T10:00:00", "2026-04-10T10:30:00", "flight", "train")
	if ok {
		t.Error("30min gap flight→train should NOT be feasible")
	}

	// 45min gap train→train — should be feasible (30min minimum).
	ok = IsConnectionFeasible("2026-04-10T10:00:00", "2026-04-10T10:45:00", "train", "train")
	if !ok {
		t.Error("45min gap train→train should be feasible")
	}
}

func TestParetoFilter(t *testing.T) {
	its := []models.RouteItinerary{
		{TotalPrice: 100, TotalDuration: 300, Transfers: 1}, // dominated by 0
		{TotalPrice: 80, TotalDuration: 280, Transfers: 0},  // Pareto optimal
		{TotalPrice: 50, TotalDuration: 600, Transfers: 2},  // Pareto optimal (cheapest)
		{TotalPrice: 200, TotalDuration: 250, Transfers: 3}, // Pareto optimal (not dominated)
	}
	filtered := paretoFilter(its)
	// Should have at least the non-dominated ones.
	if len(filtered) < 2 {
		t.Errorf("paretoFilter returned %d, want >= 2", len(filtered))
	}
	// The dominated one (100, 300, 1) should be removed.
	for _, it := range filtered {
		if it.TotalPrice == 100 && it.TotalDuration == 300 && it.Transfers == 1 {
			t.Error("paretoFilter kept a dominated itinerary (100, 300, 1)")
		}
	}
}

func TestSortItineraries(t *testing.T) {
	its := []models.RouteItinerary{
		{TotalPrice: 200, TotalDuration: 120},
		{TotalPrice: 50, TotalDuration: 600},
		{TotalPrice: 100, TotalDuration: 300},
	}

	sortItineraries(its, Options{SortBy: "price"})
	if its[0].TotalPrice != 50 {
		t.Errorf("sort by price: first = %.0f, want 50", its[0].TotalPrice)
	}

	sortItineraries(its, Options{SortBy: "duration"})
	if its[0].TotalDuration != 120 {
		t.Errorf("sort by duration: first = %d, want 120", its[0].TotalDuration)
	}
}

func TestSortItinerariesPreferMode(t *testing.T) {
	its := []models.RouteItinerary{
		{TotalPrice: 50, TotalDuration: 600, Legs: []models.RouteLeg{{Mode: "bus"}}},
		{TotalPrice: 200, TotalDuration: 120, Legs: []models.RouteLeg{{Mode: "train"}}},
	}

	sortItineraries(its, Options{SortBy: "price", Prefer: "train"})
	if got := its[0].Legs[0].Mode; got != "train" {
		t.Fatalf("preferred mode sort picked %q first, want train", got)
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{
		Prefer: " Train ",
		Avoid:  " BUS ",
	}

	opts.defaults()

	if opts.MaxTransfers != 3 {
		t.Fatalf("MaxTransfers = %d, want 3", opts.MaxTransfers)
	}
	if opts.Currency != "EUR" {
		t.Fatalf("Currency = %q, want EUR", opts.Currency)
	}
	if opts.MaxHubs != 1 {
		t.Fatalf("MaxHubs = %d, want 1", opts.MaxHubs)
	}
	if opts.SortBy != "price" {
		t.Fatalf("SortBy = %q, want price", opts.SortBy)
	}
	if opts.Prefer != "train" {
		t.Fatalf("Prefer = %q, want train", opts.Prefer)
	}
	if opts.Avoid != "bus" {
		t.Fatalf("Avoid = %q, want bus", opts.Avoid)
	}
}

func TestFilterItinerariesByConstraints(t *testing.T) {
	its := []models.RouteItinerary{
		{DepartTime: "2026-04-10T08:00:00", ArriveTime: "2026-04-10T12:00:00"},
		{DepartTime: "2026-04-10T10:00:00", ArriveTime: "2026-04-10T14:00:00"},
		{DepartTime: "2026-04-10T13:00:00", ArriveTime: "2026-04-10T18:00:00"},
	}

	filtered := filterItinerariesByConstraints(its, "2026-04-10", Options{
		DepartAfter: "09:00",
		ArriveBy:    "15:00",
	})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 itinerary after time filtering, got %d", len(filtered))
	}
	if got := filtered[0].DepartTime; got != "2026-04-10T10:00:00" {
		t.Fatalf("kept itinerary depart time = %q, want 2026-04-10T10:00:00", got)
	}
}

func TestSearchFlightLegConvertsCurrency(t *testing.T) {
	withSearchMocks(
		t,
		func(context.Context, string, string, string, flights.SearchOptions) (*models.FlightSearchResult, error) {
			return &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{
						Price:    100,
						Currency: "USD",
						Duration: 120,
						Stops:    0,
						Legs: []models.FlightLeg{
							{
								DepartureTime: "2026-07-01T08:00:00",
								ArrivalTime:   "2026-07-01T10:00:00",
								Airline:       "Finnair",
							},
						},
					},
				},
			}, nil
		},
		nil,
		func(context.Context, float64, string, string) (float64, string) {
			return 91.23, "EUR"
		},
	)

	legs := searchFlightLeg(context.Background(), Hub{City: "Helsinki", Airports: []string{"HEL"}}, Hub{City: "Vienna", Airports: []string{"VIE"}}, "2026-07-01", Options{Currency: "EUR"})
	if len(legs) != 1 {
		t.Fatalf("expected 1 leg, got %d", len(legs))
	}
	if legs[0].Price != 91.23 {
		t.Fatalf("converted price = %.2f, want 91.23", legs[0].Price)
	}
	if legs[0].Currency != "EUR" {
		t.Fatalf("currency = %q, want EUR", legs[0].Currency)
	}
	if legs[0].Provider != "Finnair" {
		t.Fatalf("provider = %q, want Finnair", legs[0].Provider)
	}
}

func TestSearchGroundLegPrefersPricedRoutesAndAppliesAvoid(t *testing.T) {
	withSearchMocks(
		t,
		nil,
		func(_ context.Context, from, to, date string, opts ground.SearchOptions) (*models.GroundSearchResult, error) {
			if from != "Helsinki" || to != "Vienna" || date != "2026-07-01" {
				t.Fatalf("unexpected search args: %q %q %q", from, to, date)
			}
			if !opts.AllowBrowserFallbacks {
				t.Fatal("expected AllowBrowserFallbacks to propagate to ground search")
			}
			return &models.GroundSearchResult{
				Success: true,
				Count:   3,
				Routes: []models.GroundRoute{
					{
						Provider: "transitous",
						Type:     "train",
						Price:    0,
						Currency: "EUR",
						Duration: 180,
						Departure: models.GroundStop{
							Time: "2026-07-01T09:00:00",
						},
						Arrival: models.GroundStop{
							Time: "2026-07-01T12:00:00",
						},
					},
					{
						Provider: "flixbus",
						Type:     "bus",
						Price:    15,
						Currency: "USD",
						Duration: 240,
						Departure: models.GroundStop{
							Time: "2026-07-01T08:00:00",
						},
						Arrival: models.GroundStop{
							Time: "2026-07-01T12:00:00",
						},
					},
					{
						Provider: "trainline",
						Type:     "train",
						Price:    20,
						Currency: "USD",
						Duration: 180,
						Departure: models.GroundStop{
							Time: "2026-07-01T09:00:00",
						},
						Arrival: models.GroundStop{
							Time: "2026-07-01T12:00:00",
						},
					},
				},
			}, nil
		},
		func(context.Context, float64, string, string) (float64, string) {
			return 18.5, "EUR"
		},
	)

	legs := searchGroundLeg(context.Background(), Hub{City: "Helsinki"}, Hub{City: "Vienna"}, "2026-07-01", Options{
		Currency:              "EUR",
		Avoid:                 "bus",
		AllowBrowserFallbacks: true,
	})
	if len(legs) != 1 {
		t.Fatalf("expected 1 ground leg after filtering, got %d", len(legs))
	}
	if legs[0].Mode != "train" {
		t.Fatalf("mode = %q, want train", legs[0].Mode)
	}
	if legs[0].Price != 18.5 {
		t.Fatalf("converted price = %.1f, want 18.5", legs[0].Price)
	}
	if legs[0].Currency != "EUR" {
		t.Fatalf("currency = %q, want EUR", legs[0].Currency)
	}
}

func TestSearchRouteReturnsNoMatchesError(t *testing.T) {
	withSearchMocks(
		t,
		func(context.Context, string, string, string, flights.SearchOptions) (*models.FlightSearchResult, error) {
			return &models.FlightSearchResult{Success: false}, nil
		},
		func(context.Context, string, string, string, ground.SearchOptions) (*models.GroundSearchResult, error) {
			return &models.GroundSearchResult{Success: false}, nil
		},
		nil,
	)

	result, err := SearchRoute(context.Background(), "alpha", "omega", "2026-07-01", Options{})
	if err != nil {
		t.Fatalf("SearchRoute: %v", err)
	}
	if result.Success {
		t.Fatal("expected no successful itineraries")
	}
	if result.Error == "" {
		t.Fatal("expected no-match error message")
	}
	if result.Origin != "Alpha" || result.Destination != "Omega" {
		t.Fatalf("unexpected normalized endpoints: %q -> %q", result.Origin, result.Destination)
	}
}

func TestSearchRouteReturnsDirectGroundResult(t *testing.T) {
	withSearchMocks(
		t,
		func(context.Context, string, string, string, flights.SearchOptions) (*models.FlightSearchResult, error) {
			return &models.FlightSearchResult{Success: false}, nil
		},
		func(_ context.Context, from, to, date string, opts ground.SearchOptions) (*models.GroundSearchResult, error) {
			if from != "Alpha" || to != "Omega" || date != "2026-07-01" {
				t.Fatalf("unexpected ground search args: %q %q %q", from, to, date)
			}
			if opts.Currency != "EUR" {
				t.Fatalf("currency = %q, want EUR", opts.Currency)
			}
			if !opts.AllowBrowserFallbacks {
				t.Fatal("expected AllowBrowserFallbacks to reach route search")
			}
			return &models.GroundSearchResult{
				Success: true,
				Count:   1,
				Routes: []models.GroundRoute{
					{
						Provider: "trainline",
						Type:     "train",
						Price:    25,
						Currency: "EUR",
						Duration: 180,
						Departure: models.GroundStop{
							Time: "2026-07-01T09:00:00",
						},
						Arrival: models.GroundStop{
							Time: "2026-07-01T12:00:00",
						},
					},
				},
			}, nil
		},
		nil,
	)

	result, err := SearchRoute(context.Background(), "alpha", "omega", "2026-07-01", Options{AllowBrowserFallbacks: true})
	if err != nil {
		t.Fatalf("SearchRoute: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected successful route search, got error %q", result.Error)
	}
	if result.Origin != "Alpha" || result.Destination != "Omega" {
		t.Fatalf("unexpected normalized endpoints: %q -> %q", result.Origin, result.Destination)
	}
	if len(result.Itineraries) != 1 {
		t.Fatalf("expected 1 itinerary, got %d", len(result.Itineraries))
	}
	it := result.Itineraries[0]
	if len(it.Legs) != 1 {
		t.Fatalf("expected direct itinerary, got %d legs", len(it.Legs))
	}
	if it.Legs[0].Mode != "train" {
		t.Fatalf("mode = %q, want train", it.Legs[0].Mode)
	}
}

func TestResolveCity(t *testing.T) {
	got := resolveCity("HEL")
	if got != "Helsinki" {
		t.Errorf("resolveCity(HEL) = %q, want Helsinki", got)
	}
	got = resolveCity("DBV")
	if got != "Dubrovnik" {
		t.Errorf("resolveCity(DBV) = %q, want Dubrovnik", got)
	}
	got = resolveCity("Helsinki")
	if got != "Helsinki" {
		t.Errorf("resolveCity(Helsinki) = %q, want Helsinki", got)
	}
}

func TestSingleLegItinerary(t *testing.T) {
	leg := models.RouteLeg{
		Mode: "flight", Price: 89, Currency: "EUR", Duration: 180,
		Departure: "2026-04-10T06:00:00", Arrival: "2026-04-10T09:00:00",
	}
	it := singleLegItinerary(leg)
	if it.TotalPrice != 89 {
		t.Errorf("TotalPrice = %.0f, want 89", it.TotalPrice)
	}
	if it.TotalDuration != 180 {
		t.Errorf("TotalDuration = %d, want 180", it.TotalDuration)
	}
	if it.Transfers != 0 {
		t.Errorf("Transfers = %d, want 0", it.Transfers)
	}
}

func TestComputeTotalDuration(t *testing.T) {
	l1 := models.RouteLeg{
		Departure: "2026-04-10T06:00:00", Arrival: "2026-04-10T08:00:00",
		Mode: "ferry", Duration: 120,
	}
	l2 := models.RouteLeg{
		Departure: "2026-04-10T12:00:00", Arrival: "2026-04-10T15:00:00",
		Mode: "flight", Duration: 180,
	}
	dur := computeTotalDuration(l1, l2)
	// 06:00 → 15:00 = 9 hours = 540 minutes.
	if dur != 540 {
		t.Errorf("computeTotalDuration = %d, want 540", dur)
	}
}

func TestBuildPaths(t *testing.T) {
	hel, _ := LookupHub("Helsinki")
	dbv, _ := LookupHub("Dubrovnik")
	paths := buildPaths(hel, dbv, Options{MaxHubs: 1})

	// Should have at least 2: direct + at least one hub.
	if len(paths) < 2 {
		t.Errorf("buildPaths(HEL→DBV) returned %d paths, want >= 2", len(paths))
	}

	// First should be direct.
	if len(paths[0].cities) != 2 {
		t.Errorf("first path should be direct (2 cities), got %d", len(paths[0].cities))
	}
}

func TestSelectDiverseLegs(t *testing.T) {
	flights := []models.RouteLeg{
		{Mode: "flight", Provider: "A"},
		{Mode: "flight", Provider: "B"},
		{Mode: "flight", Provider: "C"},
	}
	ground := []models.RouteLeg{
		{Mode: "train", Provider: "DB"},
	}

	selected := selectDiverseLegs(flights, ground, 2)
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected legs, got %d", len(selected))
	}
	if selected[0].Provider != "A" || selected[1].Provider != "B" || selected[2].Provider != "DB" {
		t.Fatalf("unexpected selected legs: %#v", selected)
	}
}

func TestCombineSegmentsBuildsFeasibleItinerary(t *testing.T) {
	p := path{
		cities: []Hub{
			{City: "Helsinki"},
			{City: "Vienna"},
			{City: "Dubrovnik"},
		},
	}
	segResults := map[string]*segResult{
		"Helsinki→Vienna": {
			flights: []models.RouteLeg{
				{
					Mode:      "flight",
					Provider:  "Finnair",
					Departure: "2026-07-01T08:00:00",
					Arrival:   "2026-07-01T10:00:00",
					Duration:  120,
					Price:     60,
					Currency:  "EUR",
				},
			},
		},
		"Vienna→Dubrovnik": {
			ground: []models.RouteLeg{
				{
					Mode:      "train",
					Provider:  "Nightjet",
					Departure: "2026-07-01T12:30:00",
					Arrival:   "2026-07-01T17:30:00",
					Duration:  300,
					Price:     70,
					Currency:  "EUR",
				},
			},
		},
	}

	itineraries := combineSegments(p, segResults, Options{MaxTransfers: 3})
	if len(itineraries) != 1 {
		t.Fatalf("expected 1 itinerary, got %d", len(itineraries))
	}

	it := itineraries[0]
	if it.TotalPrice != 130 {
		t.Fatalf("total price = %.0f, want 130", it.TotalPrice)
	}
	if it.TotalDuration != 570 {
		t.Fatalf("total duration = %d, want 570", it.TotalDuration)
	}
	if it.Transfers != 1 {
		t.Fatalf("transfers = %d, want 1", it.Transfers)
	}
}

func TestItineraryModeKeyAndDiverseFilter(t *testing.T) {
	flightCheap := models.RouteItinerary{
		Legs:       []models.RouteLeg{{Mode: "flight"}},
		TotalPrice: 50,
	}
	flightExpensive := models.RouteItinerary{
		Legs:       []models.RouteLeg{{Mode: "flight"}},
		TotalPrice: 80,
	}
	train := models.RouteItinerary{
		Legs:       []models.RouteLeg{{Mode: "train"}},
		TotalPrice: 55,
	}
	mixed := models.RouteItinerary{
		Legs:       []models.RouteLeg{{Mode: "flight"}, {Mode: "train"}},
		TotalPrice: 70,
	}

	if got := itineraryModeKey(mixed); got != "flight+train" {
		t.Fatalf("itineraryModeKey = %q, want flight+train", got)
	}

	filtered := diverseFilter([]models.RouteItinerary{
		flightExpensive,
		train,
		mixed,
		flightCheap,
	})
	if len(filtered) != 4 {
		t.Fatalf("expected 4 itineraries after diverse filter, got %d", len(filtered))
	}

	foundCheapFlight := false
	foundTrain := false
	foundMixed := false
	foundExtraFlight := false
	for _, it := range filtered {
		switch itineraryModeKey(it) {
		case "flight":
			if it.TotalPrice == 50 {
				foundCheapFlight = true
			}
			if it.TotalPrice == 80 {
				foundExtraFlight = true
			}
		case "train":
			foundTrain = true
		case "flight+train":
			foundMixed = true
		}
	}
	if !foundCheapFlight || !foundTrain || !foundMixed || !foundExtraFlight {
		t.Fatalf("unexpected diverse filter result: %#v", filtered)
	}
}
