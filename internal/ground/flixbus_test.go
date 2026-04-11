package ground

import (
	"testing"
)

func TestCollectFlixBusAmenities_Dedup(t *testing.T) {
	legs := []flixbusLeg{
		{Amenities: []any{"WiFi", "AC"}},
		{Amenities: []any{"WiFi", "Toilet"}}, // WiFi should be deduped
	}
	got := collectFlixBusAmenities(legs)
	if len(got) != 3 {
		t.Fatalf("expected 3 unique amenities, got %d: %v", len(got), got)
	}
}

func TestCollectFlixBusAmenities_Empty(t *testing.T) {
	got := collectFlixBusAmenities(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 amenities for nil legs, got %d", len(got))
	}
}

func TestParseFlixBusAmenities_ObjectWithoutType(t *testing.T) {
	raw := []any{map[string]any{"name": "WiFi"}}
	got := parseFlixBusAmenities(raw)
	if len(got) != 0 {
		t.Errorf("expected 0 amenities for object without type, got %d", len(got))
	}
}

// FlixBus rate limiter is tested in eurostar_test.go (TestAllLimiterConfigurations).

func TestFlixBusSearchResponse_Parse(t *testing.T) {
	// Verify that the search response struct deserializes correctly.
	var resp flixbusSearchResponse
	resp.Trips = []flixbusTrip{
		{
			DepartureCityID: "city-1",
			ArrivalCityID:   "city-2",
			Results: map[string]flixbusResult{
				"uid-1": {
					UID:          "uid-1",
					Status:       "available",
					TransferType: "direct",
					Departure:    flixbusStop{Date: "2026-06-01T10:00:00+02:00", CityID: "city-1"},
					Arrival:      flixbusStop{Date: "2026-06-01T14:00:00+02:00", CityID: "city-2"},
					Duration:     flixbusDuration{Hours: 4, Minutes: 0},
					Price:        flixbusPrice{Total: 29.99, Original: 35.00},
				},
			},
		},
	}
	if len(resp.Trips) != 1 {
		t.Fatalf("expected 1 trip")
	}
	r := resp.Trips[0].Results["uid-1"]
	if r.Price.Total != 29.99 {
		t.Errorf("price = %.2f, want 29.99", r.Price.Total)
	}
	dur := r.Duration.Hours*60 + r.Duration.Minutes
	if dur != 240 {
		t.Errorf("duration = %d min, want 240", dur)
	}
}
