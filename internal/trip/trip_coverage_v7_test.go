package trip

import (
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// ============================================================
// filterBreakfastSpots — pure function extracted from findBreakfastNearHotel
// ============================================================

func TestFilterBreakfastSpots_CafesAndRestaurants(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "Morning Cafe", Type: "cafe", Distance: 100, Cuisine: "coffee", Hours: "07:00-15:00", Website: "https://morning.test"},
			{Name: "Lunch Place", Type: "restaurant", Distance: 200, Cuisine: "italian"},
			{Name: "ATM", Type: "atm", Distance: 50},
			{Name: "Night Bar", Type: "bar", Distance: 150},
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 2 {
		t.Fatalf("expected 2 breakfast spots, got %d", len(spots))
	}
	// Sorted by distance: Morning Cafe (100m), Lunch Place (200m).
	if spots[0].Name != "Morning Cafe" {
		t.Errorf("first spot = %q, want Morning Cafe", spots[0].Name)
	}
	if spots[0].Type != "cafe" {
		t.Errorf("first type = %q, want cafe", spots[0].Type)
	}
	if spots[0].Distance != 100 {
		t.Errorf("first distance = %d, want 100", spots[0].Distance)
	}
	if spots[0].Cuisine != "coffee" {
		t.Errorf("first cuisine = %q, want coffee", spots[0].Cuisine)
	}
	if spots[0].Hours != "07:00-15:00" {
		t.Errorf("first hours = %q, want 07:00-15:00", spots[0].Hours)
	}
	if spots[0].Website != "https://morning.test" {
		t.Errorf("first website = %q, want https://morning.test", spots[0].Website)
	}
	if spots[1].Name != "Lunch Place" {
		t.Errorf("second spot = %q, want Lunch Place", spots[1].Name)
	}
}

func TestFilterBreakfastSpots_MergesRatedPlaces(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "OSM Cafe", Type: "cafe", Distance: 300},
		},
		RatedPlaces: []models.RatedPlace{
			{Name: "Google Restaurant", Distance: 400, Cuisine: "thai"},
			{Name: "Too Far", Distance: 700}, // beyond 600m, should be filtered
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 2 {
		t.Fatalf("expected 2 spots, got %d", len(spots))
	}
	if spots[0].Name != "OSM Cafe" {
		t.Errorf("first = %q, want OSM Cafe", spots[0].Name)
	}
	if spots[1].Name != "Google Restaurant" {
		t.Errorf("second = %q, want Google Restaurant", spots[1].Name)
	}
	if spots[1].Type != "restaurant" {
		t.Errorf("rated place type = %q, want restaurant", spots[1].Type)
	}
	if spots[1].Cuisine != "thai" {
		t.Errorf("rated place cuisine = %q, want thai", spots[1].Cuisine)
	}
}

func TestFilterBreakfastSpots_DeduplicatesByName(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "Cafe Central", Type: "cafe", Distance: 100},
			{Name: "cafe central", Type: "restaurant", Distance: 200}, // duplicate, different case
			{Name: "", Type: "cafe", Distance: 50},                    // empty name, skipped
		},
		RatedPlaces: []models.RatedPlace{
			{Name: "CAFE CENTRAL", Distance: 300}, // also duplicate
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 1 {
		t.Fatalf("expected 1 (deduped) spot, got %d", len(spots))
	}
	if spots[0].Name != "Cafe Central" {
		t.Errorf("spot = %q, want Cafe Central", spots[0].Name)
	}
}

func TestFilterBreakfastSpots_CapsAtFive(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "Spot1", Type: "cafe", Distance: 10},
			{Name: "Spot2", Type: "cafe", Distance: 20},
			{Name: "Spot3", Type: "restaurant", Distance: 30},
			{Name: "Spot4", Type: "cafe", Distance: 40},
			{Name: "Spot5", Type: "restaurant", Distance: 50},
			{Name: "Spot6", Type: "cafe", Distance: 60},
			{Name: "Spot7", Type: "restaurant", Distance: 70},
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 5 {
		t.Fatalf("expected 5 spots (capped), got %d", len(spots))
	}
	// Should be closest 5 sorted by distance.
	for i := 0; i < 5; i++ {
		wantDist := (i + 1) * 10
		if spots[i].Distance != wantDist {
			t.Errorf("spots[%d].Distance = %d, want %d", i, spots[i].Distance, wantDist)
		}
	}
}

func TestFilterBreakfastSpots_SortsByDistance(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "Far", Type: "cafe", Distance: 500},
			{Name: "Near", Type: "cafe", Distance: 50},
			{Name: "Mid", Type: "restaurant", Distance: 250},
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 3 {
		t.Fatalf("expected 3 spots, got %d", len(spots))
	}
	if spots[0].Name != "Near" || spots[1].Name != "Mid" || spots[2].Name != "Far" {
		t.Errorf("not sorted by distance: %s, %s, %s", spots[0].Name, spots[1].Name, spots[2].Name)
	}
}

func TestFilterBreakfastSpots_EmptyResult(t *testing.T) {
	result := &destinations.NearbyResult{}
	spots := filterBreakfastSpots(result)
	if len(spots) != 0 {
		t.Errorf("expected 0 spots for empty result, got %d", len(spots))
	}
}

func TestFilterBreakfastSpots_NoBreakfastTypes(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "ATM", Type: "atm", Distance: 10},
			{Name: "Pharmacy", Type: "pharmacy", Distance: 20},
			{Name: "Bar", Type: "bar", Distance: 30},
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 0 {
		t.Errorf("expected 0 spots for non-breakfast types, got %d", len(spots))
	}
}

func TestFilterBreakfastSpots_RatedPlacesAllBeyond600m(t *testing.T) {
	result := &destinations.NearbyResult{
		RatedPlaces: []models.RatedPlace{
			{Name: "Far1", Distance: 601},
			{Name: "Far2", Distance: 1000},
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 0 {
		t.Errorf("expected 0 spots for far rated places, got %d", len(spots))
	}
}

func TestFilterBreakfastSpots_RatedPlaceExactly600m(t *testing.T) {
	result := &destinations.NearbyResult{
		RatedPlaces: []models.RatedPlace{
			{Name: "Edge", Distance: 600, Cuisine: "sushi"},
		},
	}
	spots := filterBreakfastSpots(result)
	if len(spots) != 1 {
		t.Fatalf("expected 1 spot at 600m boundary, got %d", len(spots))
	}
	if spots[0].Name != "Edge" {
		t.Errorf("spot = %q, want Edge", spots[0].Name)
	}
}

// ============================================================
// assembleWeekendResults — pure function extracted from FindWeekendGetaways
// ============================================================

func TestAssembleWeekendResults_Basic(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Barcelona", AirportCode: "BCN", Price: 100, Stops: 0, AirlineName: "Vueling"},
		{CityName: "Rome", AirportCode: "FCO", Price: 200, Stops: 1, AirlineName: "Alitalia"},
	}
	hotelPrices := map[int]*weekendHotelResult{
		0: {perNight: 50, total: 100, name: "Hotel BCN"},
		1: {perNight: 80, total: 160, name: "Hotel Rome"},
	}
	results := assembleWeekendResults(dests, hotelPrices, 0, "EUR")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Sorted by total: BCN (200) < Rome (360).
	if results[0].Destination != "Barcelona" {
		t.Errorf("first = %q, want Barcelona", results[0].Destination)
	}
	if results[0].Total != 200 {
		t.Errorf("BCN total = %v, want 200", results[0].Total)
	}
	if results[0].FlightPrice != 100 {
		t.Errorf("BCN flight = %v, want 100", results[0].FlightPrice)
	}
	if results[0].HotelPrice != 100 {
		t.Errorf("BCN hotel = %v, want 100", results[0].HotelPrice)
	}
	if results[0].HotelName != "Hotel BCN" {
		t.Errorf("BCN hotel name = %q, want Hotel BCN", results[0].HotelName)
	}
	if results[0].Currency != "EUR" {
		t.Errorf("BCN currency = %q, want EUR", results[0].Currency)
	}
	if results[0].Stops != 0 {
		t.Errorf("BCN stops = %d, want 0", results[0].Stops)
	}
	if results[0].AirlineName != "Vueling" {
		t.Errorf("BCN airline = %q, want Vueling", results[0].AirlineName)
	}
	if results[1].Destination != "Rome" {
		t.Errorf("second = %q, want Rome", results[1].Destination)
	}
	if results[1].Total != 360 {
		t.Errorf("Rome total = %v, want 360", results[1].Total)
	}
}

func TestAssembleWeekendResults_SkipsMissingHotels(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Barcelona", AirportCode: "BCN", Price: 100},
		{CityName: "Rome", AirportCode: "FCO", Price: 200},
	}
	hotelPrices := map[int]*weekendHotelResult{
		1: {perNight: 80, total: 160, name: "Hotel Rome"},
		// index 0 (Barcelona) has no hotel data
	}
	results := assembleWeekendResults(dests, hotelPrices, 0, "EUR")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (BCN skipped), got %d", len(results))
	}
	if results[0].Destination != "Rome" {
		t.Errorf("result = %q, want Rome", results[0].Destination)
	}
}

func TestAssembleWeekendResults_BudgetFilter(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Tallinn", AirportCode: "TLL", Price: 50},
		{CityName: "Tokyo", AirportCode: "NRT", Price: 400},
	}
	hotelPrices := map[int]*weekendHotelResult{
		0: {total: 50, name: "Hostel"},
		1: {total: 200, name: "Luxury"},
	}
	results := assembleWeekendResults(dests, hotelPrices, 200, "EUR")
	if len(results) != 1 {
		t.Fatalf("expected 1 within budget, got %d", len(results))
	}
	if results[0].Destination != "Tallinn" {
		t.Errorf("result = %q, want Tallinn", results[0].Destination)
	}
}

func TestAssembleWeekendResults_NoBudgetLimit(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Sydney", AirportCode: "SYD", Price: 999},
	}
	hotelPrices := map[int]*weekendHotelResult{
		0: {total: 999, name: "Palace"},
	}
	// MaxBudget=0 means no limit.
	results := assembleWeekendResults(dests, hotelPrices, 0, "USD")
	if len(results) != 1 {
		t.Fatalf("expected 1 (no budget limit), got %d", len(results))
	}
	if results[0].Total != 1998 {
		t.Errorf("total = %v, want 1998", results[0].Total)
	}
}

func TestAssembleWeekendResults_EmptyDests(t *testing.T) {
	results := assembleWeekendResults(nil, nil, 500, "EUR")
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil dests, got %d", len(results))
	}
}

func TestAssembleWeekendResults_EmptyHotels(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Berlin", AirportCode: "BER", Price: 100},
	}
	results := assembleWeekendResults(dests, map[int]*weekendHotelResult{}, 500, "EUR")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty hotels, got %d", len(results))
	}
}

func TestAssembleWeekendResults_FallbackCityName(t *testing.T) {
	dests := []models.ExploreDestination{
		{AirportCode: "HEL", Price: 100}, // no CityName set
	}
	hotelPrices := map[int]*weekendHotelResult{
		0: {total: 50, name: "Hotel"},
	}
	results := assembleWeekendResults(dests, hotelPrices, 0, "EUR")
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	// models.LookupAirportName("HEL") should return a city name or "HEL".
	if results[0].Destination == "" {
		t.Error("destination should not be empty")
	}
}

func TestAssembleWeekendResults_SortedByTotal(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Tokyo", AirportCode: "NRT", Price: 300},
		{CityName: "Tallinn", AirportCode: "TLL", Price: 50},
		{CityName: "Stockholm", AirportCode: "ARN", Price: 150},
	}
	hotelPrices := map[int]*weekendHotelResult{
		0: {total: 100, name: "H1"},
		1: {total: 30, name: "H2"},
		2: {total: 70, name: "H3"},
	}
	results := assembleWeekendResults(dests, hotelPrices, 0, "EUR")
	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}
	// Sorted: Tallinn (80) < Stockholm (220) < Tokyo (400).
	if results[0].Destination != "Tallinn" {
		t.Errorf("first = %q, want Tallinn", results[0].Destination)
	}
	if results[1].Destination != "Stockholm" {
		t.Errorf("second = %q, want Stockholm", results[1].Destination)
	}
	if results[2].Destination != "Tokyo" {
		t.Errorf("third = %q, want Tokyo", results[2].Destination)
	}
}

// ============================================================
// rankDiscoverTrials — pure function extracted from Discover
// ============================================================

func TestRankDiscoverTrials_Basic(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC) // Friday
	sun := fri.AddDate(0, 0, 2)

	trials := []discoverTrial{
		{
			window: candidateWindow{start: fri, end: sun, nights: 2},
			dest:   models.ExploreDestination{CityName: "Barcelona", AirportCode: "BCN", Price: 100},
		},
	}
	hotelResults := map[discoverTrialKey]*discoverHotelInfo{
		{airport: "BCN", nights: 2}: {price: 75, total: 150, name: "Hotel BCN", rating: 4.2},
	}
	results := rankDiscoverTrials(trials, hotelResults, 500, "EUR", 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Destination != "Barcelona" {
		t.Errorf("destination = %q, want Barcelona", r.Destination)
	}
	if r.AirportCode != "BCN" {
		t.Errorf("airport = %q, want BCN", r.AirportCode)
	}
	if r.DepartDate != "2026-08-07" {
		t.Errorf("depart = %q, want 2026-08-07", r.DepartDate)
	}
	if r.ReturnDate != "2026-08-09" {
		t.Errorf("return = %q, want 2026-08-09", r.ReturnDate)
	}
	if r.Nights != 2 {
		t.Errorf("nights = %d, want 2", r.Nights)
	}
	if r.FlightPrice != 100 {
		t.Errorf("flight price = %v, want 100", r.FlightPrice)
	}
	if r.HotelPrice != 150 {
		t.Errorf("hotel price = %v, want 150", r.HotelPrice)
	}
	if r.HotelName != "Hotel BCN" {
		t.Errorf("hotel name = %q, want Hotel BCN", r.HotelName)
	}
	if r.HotelRating != 4.2 {
		t.Errorf("hotel rating = %v, want 4.2", r.HotelRating)
	}
	if r.Total != 250 {
		t.Errorf("total = %v, want 250", r.Total)
	}
	if r.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", r.Currency)
	}
	if r.BudgetSlack != 250 {
		t.Errorf("slack = %v, want 250", r.BudgetSlack)
	}
	if r.ValueScore <= 0 {
		t.Errorf("value score = %v, want > 0", r.ValueScore)
	}
	if r.Reasoning == "" {
		t.Error("reasoning should not be empty")
	}
}

func TestRankDiscoverTrials_ExceedsBudget(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	sun := fri.AddDate(0, 0, 2)

	trials := []discoverTrial{
		{
			window: candidateWindow{start: fri, end: sun, nights: 2},
			dest:   models.ExploreDestination{AirportCode: "BCN", Price: 400},
		},
	}
	hotelResults := map[discoverTrialKey]*discoverHotelInfo{
		{airport: "BCN", nights: 2}: {total: 200, name: "H"},
	}
	// Budget=500, total=600 -> exceeds.
	results := rankDiscoverTrials(trials, hotelResults, 500, "EUR", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for over-budget, got %d", len(results))
	}
}

func TestRankDiscoverTrials_NoHotelData(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	sun := fri.AddDate(0, 0, 2)

	trials := []discoverTrial{
		{
			window: candidateWindow{start: fri, end: sun, nights: 2},
			dest:   models.ExploreDestination{AirportCode: "BCN", Price: 100},
		},
	}
	// No hotel data for BCN.
	results := rankDiscoverTrials(trials, map[discoverTrialKey]*discoverHotelInfo{}, 500, "EUR", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for missing hotel, got %d", len(results))
	}
}

func TestRankDiscoverTrials_RankedByValueScore(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	trials := []discoverTrial{
		{
			window: candidateWindow{start: fri, end: fri.AddDate(0, 0, 2), nights: 2},
			dest:   models.ExploreDestination{CityName: "Tallinn", AirportCode: "TLL", Price: 50},
		},
		{
			window: candidateWindow{start: fri, end: fri.AddDate(0, 0, 2), nights: 2},
			dest:   models.ExploreDestination{CityName: "Tokyo", AirportCode: "NRT", Price: 400},
		},
	}
	hotelResults := map[discoverTrialKey]*discoverHotelInfo{
		{airport: "TLL", nights: 2}: {total: 50, name: "Hostel", rating: 4.5},
		{airport: "NRT", nights: 2}: {total: 50, name: "Hostel2", rating: 3.0},
	}
	results := rankDiscoverTrials(trials, hotelResults, 500, "EUR", 5)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	// Cheap trip (total=100) should have higher value score than expensive (total=450).
	if results[0].Destination != "Tallinn" {
		t.Errorf("highest value = %q, want Tallinn", results[0].Destination)
	}
	if results[0].ValueScore <= results[1].ValueScore {
		t.Errorf("Tallinn value (%v) should be > Tokyo value (%v)", results[0].ValueScore, results[1].ValueScore)
	}
}

func TestRankDiscoverTrials_TopCap(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	codes := []string{"BCN", "FCO", "CDG", "AMS", "BER", "VIE", "PRG", "WAW", "BUD", "ATH"}
	var trials []discoverTrial
	hotelResults := make(map[discoverTrialKey]*discoverHotelInfo)

	for i, code := range codes {
		trials = append(trials, discoverTrial{
			window: candidateWindow{start: fri, end: fri.AddDate(0, 0, 2), nights: 2},
			dest:   models.ExploreDestination{CityName: code, AirportCode: code, Price: float64(50 + i*10)},
		})
		hotelResults[discoverTrialKey{airport: code, nights: 2}] = &discoverHotelInfo{
			total: 50, name: "H", rating: 4.0,
		}
	}
	results := rankDiscoverTrials(trials, hotelResults, 1000, "EUR", 3)
	if len(results) != 3 {
		t.Errorf("expected 3 (top cap), got %d", len(results))
	}
}

func TestRankDiscoverTrials_ZeroRating(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	trials := []discoverTrial{
		{
			window: candidateWindow{start: fri, end: fri.AddDate(0, 0, 2), nights: 2},
			dest:   models.ExploreDestination{CityName: "Riga", AirportCode: "RIX", Price: 100},
		},
	}
	hotelResults := map[discoverTrialKey]*discoverHotelInfo{
		{airport: "RIX", nights: 2}: {total: 100, name: "Hotel", rating: 0},
	}
	results := rankDiscoverTrials(trials, hotelResults, 500, "EUR", 5)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	// Zero rating -> quality = 0.5 (base).
	if results[0].ValueScore <= 0 {
		t.Errorf("value score = %v, want > 0 even with zero rating", results[0].ValueScore)
	}
}

func TestRankDiscoverTrials_FallbackCityName(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	trials := []discoverTrial{
		{
			window: candidateWindow{start: fri, end: fri.AddDate(0, 0, 2), nights: 2},
			dest:   models.ExploreDestination{AirportCode: "HEL", Price: 100}, // no CityName
		},
	}
	hotelResults := map[discoverTrialKey]*discoverHotelInfo{
		{airport: "HEL", nights: 2}: {total: 100, name: "Hotel", rating: 4.0},
	}
	results := rankDiscoverTrials(trials, hotelResults, 500, "EUR", 5)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].Destination == "" {
		t.Error("destination should not be empty (airport name fallback)")
	}
}

func TestRankDiscoverTrials_EmptyTrials(t *testing.T) {
	results := rankDiscoverTrials(nil, nil, 500, "EUR", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 for nil trials, got %d", len(results))
	}
}

func TestRankDiscoverTrials_BudgetFitClampedAtZero(t *testing.T) {
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	// Total exactly at budget -> budgetFit=0 -> valueScore=0.
	trials := []discoverTrial{
		{
			window: candidateWindow{start: fri, end: fri.AddDate(0, 0, 2), nights: 2},
			dest:   models.ExploreDestination{CityName: "Lisbon", AirportCode: "LIS", Price: 400},
		},
	}
	hotelResults := map[discoverTrialKey]*discoverHotelInfo{
		{airport: "LIS", nights: 2}: {total: 100, name: "H", rating: 5.0},
	}
	results := rankDiscoverTrials(trials, hotelResults, 500, "EUR", 5)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].ValueScore != 0 {
		t.Errorf("value score = %v, want 0 (at budget)", results[0].ValueScore)
	}
	if results[0].BudgetSlack != 0 {
		t.Errorf("slack = %v, want 0", results[0].BudgetSlack)
	}
}

// ============================================================
// Discover validation — additional date edge cases
// ============================================================

func TestDiscover_UntilEqualsFrom_NoWindows(t *testing.T) {
	// When until == from, no windows can be formed (a Friday + MinNights
	// always exceeds until). Returns success with empty trips.
	result, err := Discover(t.Context(), DiscoverOptions{
		Origin: "HEL",
		Budget: 500,
		From:   "2026-08-07", // Friday
		Until:  "2026-08-07",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Trips) != 0 {
		t.Errorf("expected 0 trips, got %d", len(result.Trips))
	}
}

func TestDiscover_NoFridaysInRange(t *testing.T) {
	// Mon-Thu range with no Friday.
	result, err := Discover(t.Context(), DiscoverOptions{
		Origin: "HEL",
		Budget: 500,
		From:   "2026-08-10", // Monday
		Until:  "2026-08-13", // Thursday
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Success {
		t.Error("expected Success=true for empty-but-valid search")
	}
	if len(result.Trips) != 0 {
		t.Errorf("expected 0 trips (no Fridays), got %d", len(result.Trips))
	}
}

func TestDiscover_WindowNightsExceedUntil(t *testing.T) {
	// Friday is in range but MinNights=7 exceeds until.
	result, err := Discover(t.Context(), DiscoverOptions{
		Origin:    "HEL",
		Budget:    500,
		From:      "2026-08-07", // Friday
		Until:     "2026-08-10", // Monday, 3 days later
		MinNights: 7,
		MaxNights: 7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Trips) != 0 {
		t.Errorf("expected 0 trips (window exceeds until), got %d", len(result.Trips))
	}
}

// ============================================================
// buildReviewSnippets — pure function extracted from PlanTrip enrichment
// ============================================================

func TestBuildReviewSnippets_Basic(t *testing.T) {
	reviews := []models.HotelReview{
		{Rating: 4.5, Text: "Great hotel with excellent service", Author: "Alice", Date: "2026-01-15"},
		{Rating: 3.0, Text: "Decent but noisy", Author: "Bob", Date: "2026-02-01"},
	}
	snippets := buildReviewSnippets(reviews, "Grand Hotel")
	if len(snippets) != 2 {
		t.Fatalf("expected 2 snippets, got %d", len(snippets))
	}
	if snippets[0].Rating != 4.5 {
		t.Errorf("first rating = %v, want 4.5", snippets[0].Rating)
	}
	if snippets[0].Author != "Alice" {
		t.Errorf("first author = %q, want Alice", snippets[0].Author)
	}
	if snippets[0].HotelName != "Grand Hotel" {
		t.Errorf("first hotel = %q, want Grand Hotel", snippets[0].HotelName)
	}
	if snippets[0].Date != "2026-01-15" {
		t.Errorf("first date = %q, want 2026-01-15", snippets[0].Date)
	}
}

func TestBuildReviewSnippets_SkipsEmptyText(t *testing.T) {
	reviews := []models.HotelReview{
		{Rating: 5.0, Text: "", Author: "NoText"},
		{Rating: 4.0, Text: "Lovely stay", Author: "WithText"},
	}
	snippets := buildReviewSnippets(reviews, "Hotel")
	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet (empty text skipped), got %d", len(snippets))
	}
	if snippets[0].Author != "WithText" {
		t.Errorf("author = %q, want WithText", snippets[0].Author)
	}
}

func TestBuildReviewSnippets_CapsAtThree(t *testing.T) {
	reviews := []models.HotelReview{
		{Rating: 5.0, Text: "Review one"},
		{Rating: 4.0, Text: "Review two"},
		{Rating: 3.0, Text: "Review three"},
		{Rating: 2.0, Text: "Review four"},
		{Rating: 1.0, Text: "Review five"},
	}
	snippets := buildReviewSnippets(reviews, "Hotel")
	if len(snippets) != 3 {
		t.Errorf("expected 3 snippets (capped), got %d", len(snippets))
	}
}

func TestBuildReviewSnippets_Empty(t *testing.T) {
	snippets := buildReviewSnippets(nil, "Hotel")
	if len(snippets) != 0 {
		t.Errorf("expected 0 snippets for nil reviews, got %d", len(snippets))
	}
}

func TestBuildReviewSnippets_TruncatesLongText(t *testing.T) {
	longText := "This is a very long review that exceeds one hundred and eighty characters. " +
		"It goes on and on about the hotel amenities, the breakfast buffet, the pool, " +
		"the spa, the location, and many other aspects of the stay."
	reviews := []models.HotelReview{
		{Rating: 4.0, Text: longText},
	}
	snippets := buildReviewSnippets(reviews, "Hotel")
	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(snippets))
	}
	if len(snippets[0].Text) > 185 { // 180 + "..." suffix
		t.Errorf("text not truncated: len=%d", len(snippets[0].Text))
	}
}

// ============================================================
// buildDestinationContext — pure function extracted from PlanTrip enrichment
// ============================================================

func TestBuildDestinationContext_AllSections(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Location: "Barcelona",
		Summary:  "Barcelona is a vibrant Mediterranean city.",
		URL:      "https://en.wikivoyage.org/wiki/Barcelona",
		Sections: map[string]string{
			"When to go":  "Spring and autumn are the best times to visit.",
			"Get around":  "The metro system is excellent.",
			"See":         "Visit La Sagrada Familia.",
		},
	}
	ctx := buildDestinationContext(guide)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.Source != "https://en.wikivoyage.org/wiki/Barcelona" {
		t.Errorf("source = %q", ctx.Source)
	}
	if ctx.Summary != "Barcelona is a vibrant Mediterranean city." {
		t.Errorf("summary = %q", ctx.Summary)
	}
	if ctx.WhenToGo == "" {
		t.Error("WhenToGo should not be empty")
	}
	if ctx.GetAround == "" {
		t.Error("GetAround should not be empty")
	}
}

func TestBuildDestinationContext_SummaryOnly(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Summary:  "A short summary.",
		Sections: map[string]string{},
	}
	ctx := buildDestinationContext(guide)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.Summary != "A short summary." {
		t.Errorf("summary = %q", ctx.Summary)
	}
	if ctx.WhenToGo != "" {
		t.Errorf("WhenToGo = %q, want empty", ctx.WhenToGo)
	}
}

func TestBuildDestinationContext_ReturnsNilWhenEmpty(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Summary:  "",
		Sections: map[string]string{"See": "something"},
	}
	ctx := buildDestinationContext(guide)
	if ctx != nil {
		t.Error("expected nil context when no summary/whentogo/getaround")
	}
}

func TestBuildDestinationContext_AlternateKeys(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Summary: "Summary here.",
		Sections: map[string]string{
			"Understand":     "Important background info about the city.",
			"Getting around": "Taxis are cheap.",
		},
	}
	ctx := buildDestinationContext(guide)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.WhenToGo == "" {
		t.Error("WhenToGo should match 'Understand' fallback")
	}
	if ctx.GetAround == "" {
		t.Error("GetAround should match 'Getting around' fallback")
	}
}

func TestBuildDestinationContext_ClimateKey(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Summary: "A place.",
		Sections: map[string]string{
			"Climate": "Tropical climate all year round.",
		},
	}
	ctx := buildDestinationContext(guide)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.WhenToGo == "" {
		t.Error("WhenToGo should match 'Climate' fallback")
	}
}

// ============================================================
// applyOSMEnrichment — pure function extracted from PlanTrip enrichment
// ============================================================

func TestApplyOSMEnrichment_AllFields(t *testing.T) {
	hotel := &PlanHotel{Name: "Test Hotel"}
	extra := &destinations.HotelEnrichment{
		Stars:      4,
		Website:    "https://testhotel.com",
		Wheelchair: "yes",
	}
	applyOSMEnrichment(hotel, extra)
	if hotel.OSMStars != 4 {
		t.Errorf("OSMStars = %d, want 4", hotel.OSMStars)
	}
	if hotel.Website != "https://testhotel.com" {
		t.Errorf("Website = %q, want https://testhotel.com", hotel.Website)
	}
	if hotel.Wheelchair != "yes" {
		t.Errorf("Wheelchair = %q, want yes", hotel.Wheelchair)
	}
}

func TestApplyOSMEnrichment_DoesNotOverwriteExisting(t *testing.T) {
	hotel := &PlanHotel{
		Name:     "Test Hotel",
		OSMStars: 3,
		Website:  "https://existing.com",
	}
	extra := &destinations.HotelEnrichment{
		Stars:      5,
		Website:    "https://new.com",
		Wheelchair: "limited",
	}
	applyOSMEnrichment(hotel, extra)
	// Stars: existing=3 (non-zero), so keep existing.
	if hotel.OSMStars != 3 {
		t.Errorf("OSMStars = %d, want 3 (should not overwrite)", hotel.OSMStars)
	}
	// Website: existing non-empty, so keep existing.
	if hotel.Website != "https://existing.com" {
		t.Errorf("Website = %q, want existing (should not overwrite)", hotel.Website)
	}
	// Wheelchair: always applied.
	if hotel.Wheelchair != "limited" {
		t.Errorf("Wheelchair = %q, want limited", hotel.Wheelchair)
	}
}

func TestApplyOSMEnrichment_ZeroStars(t *testing.T) {
	hotel := &PlanHotel{Name: "Test"}
	extra := &destinations.HotelEnrichment{Stars: 0}
	applyOSMEnrichment(hotel, extra)
	if hotel.OSMStars != 0 {
		t.Errorf("OSMStars = %d, want 0 (extra has zero stars)", hotel.OSMStars)
	}
}

func TestApplyOSMEnrichment_EmptyExtra(t *testing.T) {
	hotel := &PlanHotel{Name: "Test"}
	extra := &destinations.HotelEnrichment{}
	applyOSMEnrichment(hotel, extra)
	if hotel.OSMStars != 0 {
		t.Errorf("OSMStars = %d, want 0", hotel.OSMStars)
	}
	if hotel.Website != "" {
		t.Errorf("Website = %q, want empty", hotel.Website)
	}
	if hotel.Wheelchair != "" {
		t.Errorf("Wheelchair = %q, want empty", hotel.Wheelchair)
	}
}

// ============================================================
// FindWeekendGetaways validation — additional edge cases
// ============================================================

func TestFindWeekendGetaways_NegativeNightsDefaultsTo2(t *testing.T) {
	// Should default to 2 even with negative input.
	_, err := FindWeekendGetaways(t.Context(), "HEL", WeekendOptions{
		Month:  "invalid-month",
		Nights: -5,
	})
	// Will fail on invalid month, but defaults() runs first.
	if err == nil {
		t.Error("expected error for invalid month")
	}
}
