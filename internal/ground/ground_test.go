package ground

import (
	"context"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestFlixBusAutoComplete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cities, err := FlixBusAutoComplete(ctx, "Prague")
	if err != nil {
		t.Fatalf("FlixBusAutoComplete: %v", err)
	}
	if len(cities) == 0 {
		t.Fatal("expected at least one city for Prague")
	}
	if cities[0].Name != "Prague" {
		t.Errorf("first city = %q, want Prague", cities[0].Name)
	}
	if cities[0].ID == "" {
		t.Error("city ID should not be empty")
	}
	if cities[0].Country != "cz" {
		t.Errorf("country = %q, want cz", cities[0].Country)
	}
}

func TestFlixBusAutoComplete_NoResults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cities, err := FlixBusAutoComplete(ctx, "xyznonexistent12345")
	if err != nil {
		t.Fatalf("FlixBusAutoComplete: %v", err)
	}
	if len(cities) != 0 {
		t.Errorf("expected 0 cities, got %d", len(cities))
	}
}

func TestRegioJetAutoComplete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cities, err := RegioJetAutoComplete(ctx, "Prague")
	if err != nil {
		t.Fatalf("RegioJetAutoComplete: %v", err)
	}
	if len(cities) == 0 {
		t.Fatal("expected at least one city for Prague")
	}
	if cities[0].Name != "Prague" {
		t.Errorf("first city = %q, want Prague", cities[0].Name)
	}
}

func TestSearchFlixBus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Prague to Vienna — well-served route
	// First resolve city IDs
	fromCities, err := FlixBusAutoComplete(ctx, "Prague")
	if err != nil {
		t.Fatalf("resolve from: %v", err)
	}
	toCities, err := FlixBusAutoComplete(ctx, "Vienna")
	if err != nil {
		t.Fatalf("resolve to: %v", err)
	}
	if len(fromCities) == 0 || len(toCities) == 0 {
		t.Skip("could not resolve cities")
	}

	// Use a date 30 days from now to avoid past-date issues
	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	routes, err := SearchFlixBus(ctx, fromCities[0].ID, toCities[0].ID, date, "EUR")
	if err != nil {
		t.Fatalf("SearchFlixBus: %v", err)
	}
	if len(routes) == 0 {
		t.Skip("no FlixBus routes found (may be a timing issue)")
	}

	r := routes[0]
	if r.Provider != "flixbus" {
		t.Errorf("provider = %q, want flixbus", r.Provider)
	}
	if r.Price <= 0 {
		t.Errorf("price = %f, should be > 0", r.Price)
	}
	if r.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", r.Currency)
	}
	if r.Duration <= 0 {
		t.Errorf("duration = %d, should be > 0", r.Duration)
	}
	if r.BookingURL == "" {
		t.Error("booking URL should not be empty")
	}
	if r.Type != "bus" && r.Type != "train" && r.Type != "mixed" {
		t.Errorf("type = %q, should be bus/train/mixed", r.Type)
	}
}

func TestSearchRegioJet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Prague (10202003) to Vienna (10202052) — core RegioJet route
	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	routes, err := SearchRegioJet(ctx, 10202003, 10202052, date, "EUR")
	if err != nil {
		t.Fatalf("SearchRegioJet: %v", err)
	}
	if len(routes) == 0 {
		t.Skip("no RegioJet routes found")
	}

	// Find first route with a real price (some routes may have priceFrom=0 for sold-out)
	var r *models.GroundRoute
	for i := range routes {
		if routes[i].Price > 0 {
			r = &routes[i]
			break
		}
	}
	if r == nil {
		t.Skip("all RegioJet routes have price=0")
	}

	if r.Provider != "regiojet" {
		t.Errorf("provider = %q, want regiojet", r.Provider)
	}
	if r.Duration <= 0 {
		t.Errorf("duration = %d, should be > 0", r.Duration)
	}
	if r.BookingURL == "" {
		t.Error("booking URL should not be empty")
	}
}

func TestSearchByName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	result, err := SearchByName(ctx, "Prague", "Vienna", date, SearchOptions{Currency: "EUR"})
	if err != nil {
		t.Fatalf("SearchByName: %v", err)
	}
	if !result.Success {
		t.Skipf("no routes found: %s", result.Error)
	}
	if result.Count == 0 {
		t.Error("count should be > 0 when success is true")
	}
	if len(result.Routes) != result.Count {
		t.Errorf("routes length %d != count %d", len(result.Routes), result.Count)
	}

	// Routes should be sorted by price
	for i := 1; i < len(result.Routes); i++ {
		if result.Routes[i].Price < result.Routes[i-1].Price {
			t.Errorf("routes not sorted by price at index %d: %f < %f",
				i, result.Routes[i].Price, result.Routes[i-1].Price)
		}
	}
}

func TestSearchByName_TypeFilter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	result, err := SearchByName(ctx, "Prague", "Vienna", date, SearchOptions{
		Currency: "EUR",
		Type:     "train",
	})
	if err != nil {
		t.Fatalf("SearchByName: %v", err)
	}

	for _, r := range result.Routes {
		if r.Type != "train" {
			t.Errorf("expected type=train, got %q", r.Type)
		}
	}
}

func TestSearchByName_MaxPrice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	result, err := SearchByName(ctx, "Prague", "Vienna", date, SearchOptions{
		Currency: "EUR",
		MaxPrice: 20,
	})
	if err != nil {
		t.Fatalf("SearchByName: %v", err)
	}

	for _, r := range result.Routes {
		if r.Price > 20 {
			t.Errorf("price %f exceeds max 20", r.Price)
		}
	}
}

func TestSearchByName_UnknownCity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	result, err := SearchByName(ctx, "Xyznotacity99", "Vienna", date, SearchOptions{})
	if err != nil {
		t.Fatalf("SearchByName should not error: %v", err)
	}
	// Should gracefully return with error message, not crash
	if result.Success && result.Count > 0 {
		t.Error("expected no results for nonexistent city")
	}
}

func TestParseFlixBusAmenities(t *testing.T) {
	tests := []struct {
		name  string
		input []any
		want  int
	}{
		{"string amenities", []any{"WIFI", "POWER_SOCKETS"}, 2},
		{"object amenities", []any{map[string]any{"type": "WIFI"}, map[string]any{"type": "POWER_SOCKETS"}}, 2},
		{"empty", []any{}, 0},
		{"nil", nil, 0},
		{"mixed", []any{"WIFI", map[string]any{"type": "AC"}}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlixBusAmenities(tt.input)
			if len(got) != tt.want {
				t.Errorf("got %d amenities, want %d", len(got), tt.want)
			}
		})
	}
}

func TestComputeLegDuration(t *testing.T) {
	tests := []struct {
		dep  string
		arr  string
		want int
	}{
		{"2026-04-10T06:00:00+02:00", "2026-04-10T10:00:00+02:00", 240},
		{"2026-04-10T23:00:00+02:00", "2026-04-11T07:00:00+02:00", 480},
		{"invalid", "invalid", 0},
	}

	for _, tt := range tests {
		got := computeLegDuration(tt.dep, tt.arr)
		if got != tt.want {
			t.Errorf("computeLegDuration(%q, %q) = %d, want %d", tt.dep, tt.arr, got, tt.want)
		}
	}
}

func TestClassifyVehicleTypes(t *testing.T) {
	tests := []struct {
		types []string
		want  string
	}{
		{[]string{"BUS"}, "bus"},
		{[]string{"TRAIN"}, "train"},
		{[]string{"BUS", "TRAIN"}, "mixed"},
		{[]string{}, "bus"}, // default
	}

	for _, tt := range tests {
		got := classifyVehicleTypes(tt.types)
		if got != tt.want {
			t.Errorf("classifyVehicleTypes(%v) = %q, want %q", tt.types, got, tt.want)
		}
	}
}

func TestBuildFlixBusBookingURL(t *testing.T) {
	url := buildFlixBusBookingURL("abc-123", "def-456", "2026-07-01")
	if url == "" {
		t.Error("booking URL should not be empty")
	}
	if !contains(url, "flixbus.com") {
		t.Error("should contain flixbus.com")
	}
}

func TestBuildRegioJetBookingURL(t *testing.T) {
	url := buildRegioJetBookingURL(10202003, 10202052, "2026-07-01")
	if url == "" {
		t.Error("booking URL should not be empty")
	}
	if !contains(url, "regiojet.com") {
		t.Error("should contain regiojet.com")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
