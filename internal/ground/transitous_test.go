package ground

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClassifyTransitousType(t *testing.T) {
	tests := []struct {
		name string
		legs []transitousLeg
		want string
	}{
		{
			"train only",
			[]transitousLeg{{Mode: "WALK"}, {Mode: "RAIL"}, {Mode: "WALK"}},
			"train",
		},
		{
			"bus only",
			[]transitousLeg{{Mode: "BUS"}},
			"bus",
		},
		{
			"tram only",
			[]transitousLeg{{Mode: "TRAM"}},
			"tram",
		},
		{
			"mixed train and bus",
			[]transitousLeg{{Mode: "RAIL"}, {Mode: "BUS"}},
			"mixed",
		},
		{
			"regional rail",
			[]transitousLeg{{Mode: "REGIONAL_RAIL"}},
			"train",
		},
		{
			"subway",
			[]transitousLeg{{Mode: "SUBWAY"}},
			"train",
		},
		{
			"highspeed rail",
			[]transitousLeg{{Mode: "HIGHSPEED_RAIL"}},
			"train",
		},
		{
			"walk only returns transit",
			[]transitousLeg{{Mode: "WALK"}},
			"transit",
		},
		{
			"coach is bus",
			[]transitousLeg{{Mode: "COACH"}},
			"bus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			itin := transitousItinerary{Legs: tt.legs}
			got := classifyTransitousType(itin)
			if got != tt.want {
				t.Errorf("classifyTransitousType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMotisModeTo(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{"BUS", "bus"},
		{"COACH", "bus"},
		{"TRAM", "tram"},
		{"CABLE_CAR", "tram"},
		{"GONDOLA", "tram"},
		{"FUNICULAR", "tram"},
		{"SUBWAY", "metro"},
		{"METRO", "metro"},
		{"RAIL", "train"},
		{"REGIONAL_RAIL", "train"},
		{"HIGHSPEED_RAIL", "train"},
		{"SUBURBAN", "train"},
		{"UNKNOWN_MODE", "train"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := motisModeTo(tt.mode)
			if got != tt.want {
				t.Errorf("motisModeTo(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestIsWalkOnly(t *testing.T) {
	tests := []struct {
		name string
		legs []transitousLeg
		want bool
	}{
		{"all walk", []transitousLeg{{Mode: "WALK"}, {Mode: "WALK"}}, true},
		{"mixed", []transitousLeg{{Mode: "WALK"}, {Mode: "RAIL"}}, false},
		{"no walk", []transitousLeg{{Mode: "BUS"}}, false},
		{"empty", []transitousLeg{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			itin := transitousItinerary{Legs: tt.legs}
			got := isWalkOnly(itin)
			if got != tt.want {
				t.Errorf("isWalkOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransitousLegProvider(t *testing.T) {
	tests := []struct {
		name string
		leg  transitousLeg
		want string
	}{
		{
			"with agency",
			transitousLeg{Route: &transitousRoute{Agency: "SNCF"}},
			"SNCF",
		},
		{
			"without route",
			transitousLeg{},
			"transitous",
		},
		{
			"empty agency",
			transitousLeg{Route: &transitousRoute{Agency: ""}},
			"transitous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transitousLegProvider(tt.leg)
			if got != tt.want {
				t.Errorf("transitousLegProvider() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildTransitousURL(t *testing.T) {
	u := BuildTransitousURL(48.8566, 2.3522, 50.8503, 4.3517)
	if u == "" {
		t.Fatal("URL should not be empty")
	}
	if !strings.Contains(u, "transitous.org") {
		t.Error("should contain transitous.org")
	}
	if !strings.Contains(u, "48.856600") {
		t.Error("should contain from latitude")
	}
	if !strings.Contains(u, "50.850300") {
		t.Error("should contain to latitude")
	}
}

func TestTransitousRateLimiterConfiguration(t *testing.T) {
	assertLimiterConfiguration(t, transitousLimiter, 6*time.Second, 1)
}

func TestSearchTransitous_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	// Paris to Brussels — well-served transit route.
	routes, err := SearchTransitous(ctx, 48.8566, 2.3522, 50.8503, 4.3517, date)
	if err != nil {
		t.Skipf("Transitous API unavailable: %v", err)
	}
	if len(routes) == 0 {
		t.Skip("no Transitous routes returned")
	}

	r := routes[0]
	if r.Provider != "transitous" {
		t.Errorf("provider = %q, want transitous", r.Provider)
	}
	if r.Duration <= 0 {
		t.Errorf("duration = %d, should be > 0", r.Duration)
	}
	if r.Departure.Time == "" {
		t.Error("departure time should not be empty")
	}
	if r.Arrival.Time == "" {
		t.Error("arrival time should not be empty")
	}
	// Transitous does not provide pricing.
	if r.Price != 0 {
		t.Errorf("price = %f, should be 0 (transitous has no pricing)", r.Price)
	}
	validTypes := map[string]bool{"train": true, "bus": true, "tram": true, "metro": true, "mixed": true, "transit": true}
	if !validTypes[r.Type] {
		t.Errorf("type = %q, not a valid transit type", r.Type)
	}
}

func TestGeocodeCity_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	coord, err := geocodeCity(ctx, "Paris")
	if err != nil {
		t.Skipf("Nominatim unavailable: %v", err)
	}

	// Paris should be roughly at 48.85, 2.35.
	if coord.lat < 48.5 || coord.lat > 49.2 {
		t.Errorf("lat = %f, expected ~48.85", coord.lat)
	}
	if coord.lon < 1.5 || coord.lon > 3.0 {
		t.Errorf("lon = %f, expected ~2.35", coord.lon)
	}
}

func TestGeocodeCity_Cache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// First call populates cache.
	coord1, err := geocodeCity(ctx, "Vienna")
	if err != nil {
		t.Skipf("Nominatim unavailable: %v", err)
	}

	// Second call should use cache.
	coord2, err := geocodeCity(ctx, "Vienna")
	if err != nil {
		t.Fatalf("cached call failed: %v", err)
	}

	if coord1.lat != coord2.lat || coord1.lon != coord2.lon {
		t.Error("cached result should match")
	}
}

func TestGeocodeCity_UnknownCity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := geocodeCity(ctx, "xyznonexistent12345zzz")
	if err == nil {
		t.Error("expected error for nonexistent city")
	}
}
