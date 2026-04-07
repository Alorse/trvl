package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
)

// --- ping ---

func TestPing(t *testing.T) {
	s := NewServer()
	resp := s.HandleRequest(&Request{JSONRPC: "2.0", ID: 1, Method: "ping"})
	if resp == nil {
		t.Fatal("ping should return a response")
	}
	if resp.Error != nil {
		t.Errorf("ping error: %v", resp.Error)
	}
}

// --- notifications/cancelled ---

func TestNotificationsCancelled(t *testing.T) {
	s := NewServer()
	resp := s.HandleRequest(&Request{JSONRPC: "2.0", Method: "notifications/cancelled"})
	if resp != nil {
		t.Error("should return nil")
	}
}

// --- logging/setLevel ---

func TestLoggingSetLevel(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]string{"level": "debug"})
	resp := s.HandleRequest(&Request{JSONRPC: "2.0", ID: 2, Method: "logging/setLevel", Params: params})
	if resp.Error != nil {
		t.Errorf("error: %v", resp.Error)
	}
	if logLevel != "debug" {
		t.Errorf("logLevel = %q, want debug", logLevel)
	}
	logLevel = "info"
}

func TestLoggingSetLevel_EmptyLevel(t *testing.T) {
	logLevel = "info"
	s := NewServer()
	params, _ := json.Marshal(map[string]string{"level": ""})
	s.HandleRequest(&Request{JSONRPC: "2.0", ID: 3, Method: "logging/setLevel", Params: params})
	if logLevel != "info" {
		t.Errorf("empty level changed logLevel to %q", logLevel)
	}
}

// --- completion/complete ---

func TestCompletionComplete_Airport(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]any{
		"ref":      map[string]any{"type": "ref/prompt", "name": "plan-trip"},
		"argument": map[string]any{"name": "origin", "value": "HE"},
	})
	resp := s.HandleRequest(&Request{JSONRPC: "2.0", ID: 4, Method: "completion/complete", Params: params})
	result := resp.Result.(map[string]any)
	completion := result["completion"].(map[string]any)
	values := completion["values"].([]string)
	found := false
	for _, v := range values {
		if v == "HEL" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected HEL in %v", values)
	}
}

func TestCompletionComplete_CabinClass(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]any{
		"ref":      map[string]any{"type": "ref/prompt", "name": "x"},
		"argument": map[string]any{"name": "cabin_class", "value": "b"},
	})
	resp := s.HandleRequest(&Request{JSONRPC: "2.0", ID: 5, Method: "completion/complete", Params: params})
	result := resp.Result.(map[string]any)
	completion := result["completion"].(map[string]any)
	values := completion["values"].([]string)
	if len(values) != 4 {
		t.Errorf("expected 4 cabin classes, got %d: %v", len(values), values)
	}
}

func TestCompletionComplete_Unknown(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]any{
		"ref":      map[string]any{"type": "ref/prompt", "name": "x"},
		"argument": map[string]any{"name": "unknown_field", "value": "x"},
	})
	resp := s.HandleRequest(&Request{JSONRPC: "2.0", ID: 6, Method: "completion/complete", Params: params})
	result := resp.Result.(map[string]any)
	completion := result["completion"].(map[string]any)
	values := completion["values"].([]string)
	if len(values) != 0 {
		t.Errorf("unknown field should return empty, got %v", values)
	}
}

// --- completeAirport ---

func TestCompleteAirport(t *testing.T) {
	tests := []struct{ prefix, expect string }{
		{"HEL", "HEL"}, {"hel", "HEL"}, {"AM", "AMS"}, {"PR", "PRG"}, {"", ""},
	}
	for _, tt := range tests {
		matches := completeAirport(tt.prefix)
		if tt.expect == "" {
			if len(matches) != 0 {
				t.Errorf("prefix %q: expected empty, got %v", tt.prefix, matches)
			}
			continue
		}
		found := false
		for _, m := range matches {
			if m == tt.expect {
				found = true
			}
		}
		if !found {
			t.Errorf("prefix %q: expected %s in %v", tt.prefix, tt.expect, matches)
		}
	}
}

// --- toUpper ---

func TestToUpper(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello", "HELLO"}, {"HEL", "HEL"}, {"", ""}, {"123abc", "123ABC"},
	}
	for _, tt := range tests {
		if got := toUpper(tt.in); got != tt.want {
			t.Errorf("toUpper(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- safeTimeSlice ---

func TestSafeTimeSlice(t *testing.T) {
	tests := []struct{ in, want string }{
		{"2026-04-10T14:25:00+02:00", "14:25"},
		{"2026-04-10T09:00:00Z", "09:00"},
		{"short", "short"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := safeTimeSlice(tt.in); got != tt.want {
			t.Errorf("safeTimeSlice(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- restaurantSummary ---

func TestRestaurantSummary_Empty(t *testing.T) {
	got := restaurantSummary(nil, "Helsinki")
	if !strings.Contains(got, "No restaurants") {
		t.Errorf("got %q", got)
	}
}

func TestRestaurantSummary_WithData(t *testing.T) {
	places := []models.RatedPlace{
		{Name: "Pizzeria", Rating: 4.5, Category: "Italian", Address: "Via Roma 1"},
	}
	got := restaurantSummary(places, "Rome")
	if !strings.Contains(got, "Pizzeria") || !strings.Contains(got, "4.5") || !strings.Contains(got, "Italian") {
		t.Errorf("got %q", got)
	}
}

// --- weekendSummary ---

func TestWeekendSummary(t *testing.T) {
	result := &trip.WeekendResult{
		Success: true, Origin: "HEL", Month: "july-2026", Nights: 2, Count: 1,
		Destinations: []trip.WeekendDestination{
			{Destination: "Tallinn", AirportCode: "TLL", FlightPrice: 89, HotelEstimate: 60, TotalEstimate: 149, Currency: "EUR"},
		},
	}
	got := weekendSummary(result)
	if !strings.Contains(got, "Tallinn") || !strings.Contains(got, "89") {
		t.Errorf("got %q", got)
	}
}

func TestWeekendSummary_Empty(t *testing.T) {
	result := &trip.WeekendResult{Success: true, Count: 0}
	got := weekendSummary(result)
	if !strings.Contains(got, "No weekend") {
		t.Errorf("got %q", got)
	}
}

// --- tripCostSummary ---

func TestTripCostSummary(t *testing.T) {
	result := &trip.TripCostResult{
		Success:   true,
		Flights:   trip.FlightCost{Outbound: 100, Return: 100, Currency: "EUR"},
		Hotels:    trip.HotelCost{PerNight: 50, Name: "Test Hotel", Currency: "EUR"},
		Total:     550,
		Currency:  "EUR",
		PerPerson: 275,
		PerDay:    78.5,
		Nights:    7,
	}
	got := tripCostSummary(result, "HEL", "BCN", 2)
	if !strings.Contains(got, "550") || !strings.Contains(got, "EUR") {
		t.Errorf("got %q", got)
	}
}

func TestTripCostSummary_Failed(t *testing.T) {
	result := &trip.TripCostResult{Success: false, Error: "no flights"}
	got := tripCostSummary(result, "HEL", "BCN", 1)
	if !strings.Contains(got, "failed") {
		t.Errorf("got %q", got)
	}
}

func TestTripCostSummary_PartialWarning(t *testing.T) {
	result := &trip.TripCostResult{
		Success:   true,
		Flights:   trip.FlightCost{Outbound: 100, Return: 120, Currency: "EUR"},
		Total:     220,
		Currency:  "EUR",
		PerPerson: 220,
		PerDay:    110,
		Nights:    2,
		Error:     "partial failure: hotels: hotel error",
	}

	got := tripCostSummary(result, "HEL", "BCN", 1)
	if !strings.Contains(got, "Warning: partial failure: hotels: hotel error") {
		t.Errorf("got %q", got)
	}
}

func TestTripCostSummary_UnavailableComponents(t *testing.T) {
	result := &trip.TripCostResult{
		Success:   true,
		Flights:   trip.FlightCost{Outbound: 100, Currency: "EUR"},
		Total:     100,
		Currency:  "EUR",
		PerPerson: 100,
		PerDay:    50,
		Nights:    2,
	}

	got := tripCostSummary(result, "HEL", "BCN", 1)
	if !strings.Contains(got, "Flights: outbound EUR 100, return unavailable") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "Hotel: unavailable") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "+ 0 return") {
		t.Fatalf("got %q, want unavailable wording", got)
	}
}

// --- nearbyPlacesSummary ---

func TestNearbyPlacesSummary(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "Sagrada Familia", Type: "attraction", Distance: 500},
		},
	}
	got := nearbyPlacesSummary(result, "attraction")
	if !strings.Contains(got, "Sagrada") {
		t.Errorf("got %q", got)
	}
}

func TestNearbyPlacesSummary_Empty(t *testing.T) {
	result := &destinations.NearbyResult{}
	got := nearbyPlacesSummary(result, "")
	if !strings.Contains(got, "0") {
		t.Errorf("got %q", got)
	}
}

// --- travelGuideSummary ---

func TestTravelGuideSummary(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Location: "Barcelona",
		Summary:  "Beautiful city",
		Sections: map[string]string{"Getting Around": "Metro is best."},
	}
	got := travelGuideSummary(guide)
	if !strings.Contains(got, "Barcelona") {
		t.Errorf("got %q", got)
	}
}

func TestTravelGuideSummary_Empty(t *testing.T) {
	guide := &models.WikivoyageGuide{Location: "Nowhere"}
	got := travelGuideSummary(guide)
	if got == "" {
		t.Error("should return something")
	}
}

// --- localEventsSummary ---

func TestLocalEventsSummary(t *testing.T) {
	events := []models.Event{
		{Name: "Jazz Fest", Date: "2026-07-01", Venue: "Square"},
	}
	got := localEventsSummary(events, "Prague", "2026-07-01", "2026-07-05")
	if !strings.Contains(got, "Jazz") {
		t.Errorf("got %q", got)
	}
}

func TestLocalEventsSummary_Empty(t *testing.T) {
	got := localEventsSummary(nil, "Nowhere", "2026-07-01", "2026-07-05")
	if !strings.Contains(got, "No events") && !strings.Contains(got, "0") {
		t.Errorf("got %q", got)
	}
}

// --- suggestDatesSummary ---

func TestSuggestDatesSummary(t *testing.T) {
	result := &trip.SmartDateResult{
		Success: true,
		CheapestDates: []trip.CheapDate{
			{Date: "2026-07-15", Price: 89, Currency: "EUR", DayOfWeek: "Tuesday"},
		},
		AveragePrice: 95,
		Currency:     "EUR",
	}
	got := suggestDatesSummary(result, "HEL", "BCN")
	if !strings.Contains(got, "HEL") || !strings.Contains(got, "BCN") {
		t.Errorf("got %q", got)
	}
}

// --- multiCitySummary ---

func TestMultiCitySummary(t *testing.T) {
	result := &trip.MultiCityResult{
		Success: true, TotalCost: 450, Currency: "EUR",
		Segments: []trip.Segment{
			{From: "HEL", To: "BCN", Price: 150},
			{From: "BCN", To: "HEL", Price: 300},
		},
	}
	got := multiCitySummary(result)
	if !strings.Contains(got, "450") {
		t.Errorf("got %q", got)
	}
}

// --- Unknown method ---

func TestUnknownMethod_Coverage(t *testing.T) {
	s := NewServer()
	resp := s.HandleRequest(&Request{JSONRPC: "2.0", ID: 99, Method: "nonexistent/method"})
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Errorf("expected -32601, got %v", resp.Error)
	}
}
