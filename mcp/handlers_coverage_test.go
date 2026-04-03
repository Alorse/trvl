package mcp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
)

// ============================================================
// handleSearchGround error paths
// ============================================================

func TestHandleSearchGround_MissingAll(t *testing.T) {
	_, _, err := handleSearchGround(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing from/to/date")
	}
}

func TestHandleSearchGround_MissingFrom(t *testing.T) {
	_, _, err := handleSearchGround(map[string]any{
		"to":   "Vienna",
		"date": "2026-07-01",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing from")
	}
}

func TestHandleSearchGround_MissingTo(t *testing.T) {
	_, _, err := handleSearchGround(map[string]any{
		"from": "Prague",
		"date": "2026-07-01",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing to")
	}
}

func TestHandleSearchGround_MissingDate(t *testing.T) {
	_, _, err := handleSearchGround(map[string]any{
		"from": "Prague",
		"to":   "Vienna",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing date")
	}
}

func TestHandleSearchGround_NilArgs(t *testing.T) {
	_, _, err := handleSearchGround(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}
}

// ============================================================
// handleTripCost error paths
// ============================================================

func TestHandleTripCost_MissingOriginDest(t *testing.T) {
	_, _, err := handleTripCost(map[string]any{
		"depart_date": "2026-07-01",
		"return_date": "2026-07-08",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing origin and destination")
	}
}

func TestHandleTripCost_MissingDates(t *testing.T) {
	_, _, err := handleTripCost(map[string]any{
		"origin":      "HEL",
		"destination": "BCN",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing dates")
	}
}

func TestHandleTripCost_MissingReturnDate(t *testing.T) {
	_, _, err := handleTripCost(map[string]any{
		"origin":      "HEL",
		"destination": "BCN",
		"depart_date": "2026-07-01",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing return_date")
	}
}

func TestHandleTripCost_InvalidOriginIATA(t *testing.T) {
	_, _, err := handleTripCost(map[string]any{
		"origin":      "XX",
		"destination": "BCN",
		"depart_date": "2026-07-01",
		"return_date": "2026-07-08",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestHandleTripCost_InvalidDestIATA(t *testing.T) {
	_, _, err := handleTripCost(map[string]any{
		"origin":      "HEL",
		"destination": "12",
		"depart_date": "2026-07-01",
		"return_date": "2026-07-08",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for invalid destination IATA")
	}
}

func TestHandleTripCost_NilArgs(t *testing.T) {
	_, _, err := handleTripCost(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}
}

// ============================================================
// handleNearbyPlaces error paths
// ============================================================

func TestHandleNearbyPlaces_MissingCoords(t *testing.T) {
	_, _, err := handleNearbyPlaces(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing lat/lon")
	}
}

func TestHandleNearbyPlaces_NilArgs(t *testing.T) {
	_, _, err := handleNearbyPlaces(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}
}

func TestHandleNearbyPlaces_ZeroCoords(t *testing.T) {
	_, _, err := handleNearbyPlaces(map[string]any{
		"lat": float64(0),
		"lon": float64(0),
	}, nil, nil)
	if err == nil {
		t.Error("expected error for zero lat/lon")
	}
}

// ============================================================
// handleTravelGuide error paths
// ============================================================

func TestHandleTravelGuide_MissingLocation(t *testing.T) {
	_, _, err := handleTravelGuide(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing location")
	}
}

func TestHandleTravelGuide_NilArgs(t *testing.T) {
	_, _, err := handleTravelGuide(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}
}

func TestHandleTravelGuide_EmptyLocation(t *testing.T) {
	_, _, err := handleTravelGuide(map[string]any{"location": ""}, nil, nil)
	if err == nil {
		t.Error("expected error for empty location")
	}
}

// ============================================================
// handleLocalEvents error paths
// ============================================================

func TestHandleLocalEvents_MissingAll(t *testing.T) {
	_, _, err := handleLocalEvents(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing location")
	}
}

func TestHandleLocalEvents_MissingDates(t *testing.T) {
	_, _, err := handleLocalEvents(map[string]any{
		"location": "Barcelona",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing dates")
	}
}

func TestHandleLocalEvents_MissingEndDate(t *testing.T) {
	_, _, err := handleLocalEvents(map[string]any{
		"location":   "Barcelona",
		"start_date": "2026-07-01",
	}, nil, nil)
	if err == nil {
		t.Error("expected error for missing end_date")
	}
}

func TestHandleLocalEvents_NilArgs(t *testing.T) {
	_, _, err := handleLocalEvents(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}
}

func TestHandleLocalEvents_NoAPIKey(t *testing.T) {
	// Without TICKETMASTER_API_KEY set, should return content with message.
	t.Setenv("TICKETMASTER_API_KEY", "")
	content, result, err := handleLocalEvents(map[string]any{
		"location":   "Barcelona",
		"start_date": "2026-07-01",
		"end_date":   "2026-07-05",
	}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected content blocks")
	}
	if !strings.Contains(content[0].Text, "TICKETMASTER_API_KEY") {
		t.Errorf("expected message about API key, got %q", content[0].Text)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

// ============================================================
// handleSearchRestaurants error paths
// ============================================================

func TestHandleSearchRestaurants_MissingLocation(t *testing.T) {
	_, _, err := handleSearchRestaurants(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing location")
	}
}

func TestHandleSearchRestaurants_NilArgs(t *testing.T) {
	_, _, err := handleSearchRestaurants(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}
}

func TestHandleSearchRestaurants_EmptyLocation(t *testing.T) {
	_, _, err := handleSearchRestaurants(map[string]any{"location": ""}, nil, nil)
	if err == nil {
		t.Error("expected error for empty location")
	}
}

// ============================================================
// curateHotelsViaSampling
// ============================================================

func TestCurateHotelsViaSampling_SuccessfulSampling(t *testing.T) {
	result := &models.HotelSearchResult{
		Success: true,
		Count:   3,
		Hotels: []models.HotelResult{
			{Name: "Budget Inn", Price: 80, Currency: "EUR", Rating: 3.5, Stars: 2, Address: "Street 1"},
			{Name: "Grand Hotel", Price: 250, Currency: "EUR", Rating: 4.8, Stars: 5, Address: "Avenue 2"},
			{Name: "Family Suite", Price: 150, Currency: "EUR", Rating: 4.2, Stars: 3, Address: "Road 3"},
		},
	}

	mockSampling := func(messages []SamplingMessage, maxTokens int) (string, error) {
		return "Budget: Budget Inn - Best value\nLuxury: Grand Hotel - Top rated\nFamily: Family Suite - Great for kids", nil
	}

	got := curateHotelsViaSampling(result, "Helsinki", mockSampling)
	if got == "" {
		t.Error("expected non-empty curation result")
	}
	if !strings.Contains(got, "Budget Inn") {
		t.Errorf("curation missing Budget Inn: %q", got)
	}
}

func TestCurateHotelsViaSampling_SamplingError(t *testing.T) {
	result := &models.HotelSearchResult{
		Success: true,
		Count:   2,
		Hotels: []models.HotelResult{
			{Name: "Hotel A", Price: 100},
			{Name: "Hotel B", Price: 200},
		},
	}

	errorSampling := func(messages []SamplingMessage, maxTokens int) (string, error) {
		return "", fmt.Errorf("sampling unavailable")
	}

	got := curateHotelsViaSampling(result, "Helsinki", errorSampling)
	if got != "" {
		t.Errorf("expected empty string on sampling error, got %q", got)
	}
}

func TestCurateHotelsViaSampling_LargeResultSet(t *testing.T) {
	// Test with more than 30 hotels to exercise the limit cap.
	hotels := make([]models.HotelResult, 40)
	for i := range 40 {
		hotels[i] = models.HotelResult{
			Name:     fmt.Sprintf("Hotel %d", i+1),
			Price:    float64(50 + i*10),
			Currency: "EUR",
			Rating:   float64(3 + (i%20)/10.0),
		}
	}
	result := &models.HotelSearchResult{
		Success: true,
		Count:   40,
		Hotels:  hotels,
	}

	mockSampling := func(messages []SamplingMessage, maxTokens int) (string, error) {
		if maxTokens != 300 {
			t.Errorf("maxTokens = %d, want 300", maxTokens)
		}
		return "Budget: Hotel 1\nLuxury: Hotel 40\nFamily: Hotel 20", nil
	}

	got := curateHotelsViaSampling(result, "Test City", mockSampling)
	if got == "" {
		t.Error("expected non-empty curation result")
	}
}

// ============================================================
// suggestDatesSummary coverage (failure path)
// ============================================================

func TestSuggestDatesSummary_FailureWithError(t *testing.T) {
	result := &trip.SmartDateResult{
		Success: false,
		Error:   "no calendar data",
	}
	got := suggestDatesSummary(result, "HEL", "BCN")
	if !strings.Contains(got, "failed") {
		t.Errorf("expected 'failed' in summary, got %q", got)
	}
	if !strings.Contains(got, "no calendar data") {
		t.Errorf("expected error message in summary, got %q", got)
	}
}

func TestSuggestDatesSummary_FailureNoError(t *testing.T) {
	result := &trip.SmartDateResult{
		Success: false,
	}
	got := suggestDatesSummary(result, "HEL", "BCN")
	if !strings.Contains(got, "Could not find") {
		t.Errorf("expected 'Could not find' in summary, got %q", got)
	}
}

func TestSuggestDatesSummary_SuccessWithInsights(t *testing.T) {
	result := &trip.SmartDateResult{
		Success:      true,
		AveragePrice: 150,
		Currency:     "EUR",
		Insights: []trip.DateInsight{
			{Type: "cheapest", Description: "Tuesdays are cheapest"},
			{Type: "pattern", Description: "Prices drop mid-week"},
		},
	}
	got := suggestDatesSummary(result, "HEL", "BCN")
	if !strings.Contains(got, "Tuesdays") {
		t.Errorf("expected insight in summary, got %q", got)
	}
	if !strings.Contains(got, "Prices drop") {
		t.Errorf("expected second insight in summary, got %q", got)
	}
}

// ============================================================
// multiCitySummary coverage (failure path)
// ============================================================

func TestMultiCitySummary_Failure(t *testing.T) {
	result := &trip.MultiCityResult{
		Success: false,
		Error:   "insufficient routes",
	}
	got := multiCitySummary(result)
	if !strings.Contains(got, "failed") || !strings.Contains(got, "insufficient") {
		t.Errorf("expected failure message, got %q", got)
	}
}

// ============================================================
// handleHotelReviews error paths
// ============================================================

func TestHandleHotelReviews_NilArgs(t *testing.T) {
	s := NewServer()
	handler := s.handlers["hotel_reviews"]
	_, _, err := handler(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil args")
	}
}

func TestHandleHotelReviews_MissingHotelID(t *testing.T) {
	s := NewServer()
	handler := s.handlers["hotel_reviews"]
	_, _, err := handler(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing hotel_id")
	}
}

// ============================================================
// Handler via HandleRequest to test missing-arg error flows
// ============================================================

func TestHandleToolsCall_SearchGround_MissingArgs(t *testing.T) {
	s := NewServer()
	handler := s.handlers["search_ground"]
	_, _, err := handler(map[string]any{"from": "Prague"}, nil, nil)
	if err == nil {
		t.Error("expected error for partial args")
	}
}

func TestHandleToolsCall_TripCost_MissingArgs(t *testing.T) {
	s := NewServer()
	handler := s.handlers["calculate_trip_cost"]
	_, _, err := handler(map[string]any{"origin": "HEL"}, nil, nil)
	if err == nil {
		t.Error("expected error for partial args")
	}
}

func TestHandleToolsCall_NearbyPlaces_MissingArgs(t *testing.T) {
	s := NewServer()
	handler := s.handlers["nearby_places"]
	_, _, err := handler(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing lat/lon")
	}
}

func TestHandleToolsCall_TravelGuide_MissingArgs(t *testing.T) {
	s := NewServer()
	handler := s.handlers["travel_guide"]
	_, _, err := handler(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing location")
	}
}

func TestHandleToolsCall_LocalEvents_MissingArgs(t *testing.T) {
	s := NewServer()
	handler := s.handlers["local_events"]
	_, _, err := handler(map[string]any{"location": "Prague"}, nil, nil)
	if err == nil {
		t.Error("expected error for missing dates")
	}
}

func TestHandleToolsCall_Restaurants_MissingArgs(t *testing.T) {
	s := NewServer()
	handler := s.handlers["search_restaurants"]
	_, _, err := handler(map[string]any{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing location")
	}
}
