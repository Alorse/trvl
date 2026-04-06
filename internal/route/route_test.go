package route

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

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
		{TotalPrice: 200, TotalDuration: 250, Transfers: 3},  // Pareto optimal (not dominated)
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

	sortItineraries(its, "price")
	if its[0].TotalPrice != 50 {
		t.Errorf("sort by price: first = %.0f, want 50", its[0].TotalPrice)
	}

	sortItineraries(its, "duration")
	if its[0].TotalDuration != 120 {
		t.Errorf("sort by duration: first = %d, want 120", its[0].TotalDuration)
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
