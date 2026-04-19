package profile

import "testing"

// --- DetectTravelMode ---

func TestDetectTravelModeNil(t *testing.T) {
	if m := DetectTravelMode(nil, 0); m != nil {
		t.Errorf("want nil for nil profile, got %+v", m)
	}
}

func TestDetectTravelModeEmpty(t *testing.T) {
	p := &TravelProfile{}
	if m := DetectTravelMode(p, 0); m != nil {
		t.Errorf("want nil for empty profile, got %+v", m)
	}
}

func TestDetectTravelModeExactMatch(t *testing.T) {
	p := &TravelProfile{
		TravelModes: []TravelMode{
			{Name: "solo_remote", Companions: 0},
			{Name: "with_partner", Companions: 1},
			{Name: "with_kids", Companions: 3},
		},
	}
	m := DetectTravelMode(p, 1)
	if m == nil || m.Name != "with_partner" {
		t.Errorf("exact match companions=1: want with_partner, got %+v", m)
	}
}

func TestDetectTravelModeClosest(t *testing.T) {
	p := &TravelProfile{
		TravelModes: []TravelMode{
			{Name: "solo", Companions: 0},
			{Name: "with_kids", Companions: 3},
		},
	}
	// companions=2 is closer to 3 than to 0
	m := DetectTravelMode(p, 2)
	if m == nil || m.Name != "with_kids" {
		t.Errorf("closest companions=2: want with_kids, got %+v", m)
	}
}

func TestDetectTravelModeFallbackFirst(t *testing.T) {
	p := &TravelProfile{
		TravelModes: []TravelMode{
			{Name: "weekend_break", Companions: 1},
		},
	}
	m := DetectTravelMode(p, 5)
	if m == nil || m.Name != "weekend_break" {
		t.Errorf("single mode fallback: want weekend_break, got %+v", m)
	}
}

// --- FlightHints ---

func TestFlightHintsNilProfile(t *testing.T) {
	h := FlightHints(nil, "HEL", "NRT")
	if len(h.PreferredAirlines) != 0 {
		t.Errorf("nil profile: want no airlines, got %v", h.PreferredAirlines)
	}
	if h.MaxPrice != 0 {
		t.Errorf("nil profile: want MaxPrice=0, got %d", h.MaxPrice)
	}
}

func TestFlightHintsTopAirlines(t *testing.T) {
	p := &TravelProfile{
		TopAirlines: []AirlineStats{
			{Code: "AY", Flights: 20},
			{Code: "LH", Flights: 15},
			{Code: "BA", Flights: 10},
			{Code: "FR", Flights: 5}, // 4th — should be excluded
		},
	}
	h := FlightHints(p, "HEL", "LHR")
	if len(h.PreferredAirlines) != 3 {
		t.Errorf("want 3 airlines, got %d: %v", len(h.PreferredAirlines), h.PreferredAirlines)
	}
	if h.PreferredAirlines[0] != "AY" {
		t.Errorf("first airline: want AY, got %s", h.PreferredAirlines[0])
	}
}

func TestFlightHintsMaxPriceFromAvg(t *testing.T) {
	p := &TravelProfile{AvgFlightPrice: 200}
	h := FlightHints(p, "HEL", "AMS")
	if h.MaxPrice != 300 { // 200 * 1.5
		t.Errorf("MaxPrice: want 300, got %d", h.MaxPrice)
	}
}

func TestFlightHintsRouteOverridesAvg(t *testing.T) {
	p := &TravelProfile{
		AvgFlightPrice: 200,
		TopRoutes: []RouteStats{
			{From: "HEL", To: "NRT", Count: 5, AvgPrice: 800},
		},
	}
	h := FlightHints(p, "HEL", "NRT")
	if h.MaxPrice != 1200 { // 800 * 1.5
		t.Errorf("route override: want 1200, got %d", h.MaxPrice)
	}
}

func TestFlightHintsCabinClassLuxury(t *testing.T) {
	p := &TravelProfile{BudgetTier: "luxury"}
	h := FlightHints(p, "JFK", "LHR")
	if h.CabinClass != 3 {
		t.Errorf("luxury CabinClass: want 3 (business), got %d", h.CabinClass)
	}
}

func TestFlightHintsAlliancePassthrough(t *testing.T) {
	p := &TravelProfile{PreferredAlliance: "STAR_ALLIANCE"}
	h := FlightHints(p, "HEL", "AMS")
	if h.PreferredAlliance != "STAR_ALLIANCE" {
		t.Errorf("alliance: want STAR_ALLIANCE, got %q", h.PreferredAlliance)
	}
}

// --- HotelHints ---

func TestHotelHintsNilProfile(t *testing.T) {
	h := HotelHints(nil, "Helsinki")
	if h.MinStars != 0 || h.MaxPrice != 0 || h.PropertyType != "" {
		t.Errorf("nil profile should produce zero hints, got %+v", h)
	}
}

func TestHotelHintsStarsFromAvg(t *testing.T) {
	p := &TravelProfile{AvgStarRating: 3.8}
	h := HotelHints(p, "Helsinki")
	if h.MinStars != 3 {
		t.Errorf("MinStars from 3.8 avg: want 3, got %d", h.MinStars)
	}
}

func TestHotelHintsStarsExact(t *testing.T) {
	p := &TravelProfile{AvgStarRating: 4.0}
	h := HotelHints(p, "Helsinki")
	if h.MinStars != 4 {
		t.Errorf("MinStars from 4.0 avg: want 4, got %d", h.MinStars)
	}
}

func TestHotelHintsMaxPriceWithFlex(t *testing.T) {
	p := &TravelProfile{
		AvgNightlyRate: 100,
		TravelModes:    []TravelMode{{Name: "solo", Companions: 0, BudgetFlex: 1.5}},
	}
	h := HotelHints(p, "Berlin")
	if h.MaxPrice != 150 {
		t.Errorf("MaxPrice 100*1.5: want 150, got %.0f", h.MaxPrice)
	}
}

func TestHotelHintsLuxuryTier(t *testing.T) {
	p := &TravelProfile{
		AvgNightlyRate: 100,
		BudgetTier:     "luxury",
		AvgStarRating:  3.0,
	}
	h := HotelHints(p, "Paris")
	// Luxury: no price ceiling, min stars forced to 4.
	if h.MaxPrice != 0 {
		t.Errorf("luxury: want MaxPrice=0, got %.0f", h.MaxPrice)
	}
	if h.MinStars < 4 {
		t.Errorf("luxury: want MinStars>=4, got %d", h.MinStars)
	}
}

func TestHotelHintsCityNeighbourhoods(t *testing.T) {
	p := &TravelProfile{
		CityIntelligence: []CityIntelligence{
			{City: "Barcelona", Neighbourhoods: []string{"Eixample", "Gracia"}},
			{City: "Tokyo", Neighbourhoods: []string{"Shinjuku"}},
		},
	}
	h := HotelHints(p, "Barcelona")
	if len(h.PreferredNeighbourhoods) != 2 {
		t.Fatalf("Barcelona neighbourhoods: want 2, got %d", len(h.PreferredNeighbourhoods))
	}
	if h.PreferredNeighbourhoods[0] != "Eixample" {
		t.Errorf("first neighbourhood: want Eixample, got %s", h.PreferredNeighbourhoods[0])
	}
}

func TestHotelHintsElasticityLocationMultiplier(t *testing.T) {
	p := &TravelProfile{
		AvgNightlyRate: 100,
		PriceElasticity: []PreferenceElasticity{
			{Factor: "location", Impact: "will_pay_more", PriceDelta: 1.4},
		},
	}
	h := HotelHints(p, "Rome")
	// default flex 1.3 × 1.4 = 182
	if h.MaxPrice < 180 || h.MaxPrice > 185 {
		t.Errorf("elasticity location: want ~182, got %.1f", h.MaxPrice)
	}
}

// --- GroundHints ---

func TestGroundHintsNilProfile(t *testing.T) {
	h := GroundHints(nil, "Prague", "Vienna")
	if h.PreferredType != "" {
		t.Errorf("nil profile: want empty type, got %q", h.PreferredType)
	}
}

func TestGroundHintsPickTopMode(t *testing.T) {
	p := &TravelProfile{
		TopGroundModes: []ModeStats{
			{Mode: "bus", Count: 10},
			{Mode: "train", Count: 25},
			{Mode: "ferry", Count: 3},
		},
	}
	h := GroundHints(p, "Stockholm", "Helsinki")
	if h.PreferredType != "train" {
		t.Errorf("top mode: want train, got %q", h.PreferredType)
	}
}

func TestGroundHintsUnknownModeDropped(t *testing.T) {
	p := &TravelProfile{
		TopGroundModes: []ModeStats{
			{Mode: "rideshare", Count: 99},
		},
	}
	h := GroundHints(p, "A", "B")
	if h.PreferredType != "" {
		t.Errorf("unknown mode should be dropped, got %q", h.PreferredType)
	}
}
