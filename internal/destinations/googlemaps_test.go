package destinations

import (
	"context"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/jsonutil"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// TestParseMapsResponse_ValidJSON tests parsing a Google Maps JSON response
// with the anti-XSSI prefix stripped and place data at expected indices.
func TestParseMapsResponse_ValidJSON(t *testing.T) {
	// Simulated Maps JSON response with the )]}'  prefix.
	// Place entries have name at index 11, rating at index 4,
	// coordinates at index 9 ([nil, nil, lat, lon]).
	response := `)]}'
[
  "search",
  null,
  [
    [null, null, null, null, 4.3, null, null, null, null,
     [null, null, 41.3851, 2.1734],
     null, "Can Paixano", null, null, "Tapas Bar", null, null, null, "Carrer de la Reina Cristina 7"],
    [null, null, null, null, 4.7, null, null, null, null,
     [null, null, 41.3825, 2.1769],
     null, "El Xampanyet", null, null, "Wine Bar", null, null, null, "Carrer de Montcada 22"],
    [null, null, null, null, 4.1, null, null, null, null,
     [null, null, 41.3790, 2.1700],
     null, "La Boqueria", null, null, "Market", null, null, null, "La Rambla 91"]
  ]
]`

	places, err := parseMapsResponse([]byte(response), 41.38, 2.17, 10)
	if err != nil {
		t.Fatalf("parseMapsResponse: %v", err)
	}

	if len(places) != 3 {
		t.Fatalf("expected 3 places, got %d", len(places))
	}

	// Verify first place.
	if places[0].Name != "Can Paixano" {
		t.Errorf("places[0].Name = %q, want Can Paixano", places[0].Name)
	}
	if places[0].Rating != 4.3 {
		t.Errorf("places[0].Rating = %.1f, want 4.3", places[0].Rating)
	}
	if places[0].Category != "Tapas Bar" {
		t.Errorf("places[0].Category = %q, want Tapas Bar", places[0].Category)
	}
	if places[0].Address != "Carrer de la Reina Cristina 7" {
		t.Errorf("places[0].Address = %q, want Carrer de la Reina Cristina 7", places[0].Address)
	}

	// Verify distance is computed (should be > 0 for different coordinates).
	if places[0].Distance == 0 {
		t.Error("places[0].Distance should be > 0")
	}

	// Verify second place.
	if places[1].Name != "El Xampanyet" {
		t.Errorf("places[1].Name = %q, want El Xampanyet", places[1].Name)
	}
	if places[1].Rating != 4.7 {
		t.Errorf("places[1].Rating = %.1f, want 4.7", places[1].Rating)
	}
}

// TestParseMapsResponse_Limit tests that the limit parameter caps results.
func TestParseMapsResponse_Limit(t *testing.T) {
	response := `)]}'
[
  "search",
  null,
  [
    [null, null, null, null, 4.3, null, null, null, null,
     [null, null, 41.385, 2.173],
     null, "Place A", null, null, "Restaurant"],
    [null, null, null, null, 4.7, null, null, null, null,
     [null, null, 41.383, 2.177],
     null, "Place B", null, null, "Cafe"],
    [null, null, null, null, 4.1, null, null, null, null,
     [null, null, 41.379, 2.170],
     null, "Place C", null, null, "Bar"]
  ]
]`

	places, err := parseMapsResponse([]byte(response), 41.38, 2.17, 2)
	if err != nil {
		t.Fatalf("parseMapsResponse: %v", err)
	}

	if len(places) != 2 {
		t.Errorf("expected 2 places (limited), got %d", len(places))
	}
}

// TestParseMapsResponse_EmptyJSON tests handling of valid but empty JSON.
func TestParseMapsResponse_EmptyJSON(t *testing.T) {
	response := `)]}'
["search", null, []]`

	places, err := parseMapsResponse([]byte(response), 41.38, 2.17, 10)
	if err != nil {
		t.Fatalf("parseMapsResponse: %v", err)
	}

	if len(places) != 0 {
		t.Errorf("expected 0 places for empty response, got %d", len(places))
	}
}

// TestParseMapsResponse_InvalidJSON tests error handling for malformed JSON.
func TestParseMapsResponse_InvalidJSON(t *testing.T) {
	response := `)]}'
this is not json`

	_, err := parseMapsResponse([]byte(response), 41.38, 2.17, 10)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestParseMapsResponse_NoPrefix tests that responses without the prefix still work.
func TestParseMapsResponse_NoPrefix(t *testing.T) {
	response := `[
  "search", null,
  [
    [null, null, null, null, 3.9, null, null, null, null,
     [null, null, 41.385, 2.173],
     null, "Solo Place", null, null, "Diner"]
  ]
]`

	places, err := parseMapsResponse([]byte(response), 41.38, 2.17, 10)
	if err != nil {
		t.Fatalf("parseMapsResponse: %v", err)
	}

	if len(places) != 1 {
		t.Errorf("expected 1 place, got %d", len(places))
	}
	if places[0].Name != "Solo Place" {
		t.Errorf("Name = %q, want Solo Place", places[0].Name)
	}
}

// TestTryExtractPlace_Valid tests extracting a well-formed place entry.
func TestTryExtractPlace_Valid(t *testing.T) {
	entry := make([]any, 19)
	entry[4] = 4.5
	entry[9] = []any{nil, nil, 41.385, 2.173}
	entry[11] = "Test Restaurant"
	entry[14] = "Italian"
	entry[18] = "Via Roma 1"

	place, ok := tryExtractPlace(entry, 41.38, 2.17)
	if !ok {
		t.Fatal("expected place extraction to succeed")
	}
	if place.Name != "Test Restaurant" {
		t.Errorf("Name = %q", place.Name)
	}
	if place.Rating != 4.5 {
		t.Errorf("Rating = %.1f", place.Rating)
	}
	if place.Category != "Italian" {
		t.Errorf("Category = %q", place.Category)
	}
	if place.Address != "Via Roma 1" {
		t.Errorf("Address = %q", place.Address)
	}
	if place.Distance == 0 {
		t.Error("Distance should be > 0")
	}
}

// TestTryExtractPlace_TooShort tests rejection of arrays that are too short.
func TestTryExtractPlace_TooShort(t *testing.T) {
	entry := make([]any, 5)
	_, ok := tryExtractPlace(entry, 41.38, 2.17)
	if ok {
		t.Error("should reject array shorter than 12 elements")
	}
}

// TestTryExtractPlace_NoName tests rejection when name is missing.
func TestTryExtractPlace_NoName(t *testing.T) {
	entry := make([]any, 12)
	entry[4] = 4.0
	entry[11] = "" // empty name

	_, ok := tryExtractPlace(entry, 41.38, 2.17)
	if ok {
		t.Error("should reject entry with empty name")
	}
}

// TestTryExtractPlace_BadRating tests rejection when rating is out of range.
func TestTryExtractPlace_BadRating(t *testing.T) {
	entry := make([]any, 12)
	entry[4] = 7.5 // out of 1-5 range
	entry[11] = "Bad Rating Place"

	_, ok := tryExtractPlace(entry, 41.38, 2.17)
	if ok {
		t.Error("should reject entry with rating > 5.0")
	}
}

// TestToFloat tests the JSON number conversion helper.
func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{3.14, 3.14, true},
		{float64(42), 42.0, true},
		{"not a number", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tt := range tests {
		got, ok := jsonutil.ToFloat(tt.input)
		if ok != tt.ok {
			t.Errorf("toFloat(%v) ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Errorf("toFloat(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestFormatCoords tests coordinate string formatting.
func TestFormatCoords(t *testing.T) {
	got := formatCoords(41.3851, 2.1734)
	want := "41.3851,2.1734"
	if got != want {
		t.Errorf("formatCoords = %q, want %q", got, want)
	}
}

// TestSearchGoogleMapsPlaces_Caching tests that results are cached.
func TestSearchGoogleMapsPlaces_Caching(t *testing.T) {
	clearAllCaches()

	// Pre-populate cache.
	mapsCache.Lock()
	mapsCache.entries["41.3800,2.1700,restaurants,10"] = mapsCacheEntry{
		places: []models.RatedPlace{
			{Name: "Cached Place", Rating: 4.0},
		},
		fetched: time.Now(),
	}
	mapsCache.Unlock()

	ctx := context.Background()
	places, err := SearchGoogleMapsPlaces(ctx, 41.38, 2.17, "restaurants", 10)
	if err != nil {
		t.Fatalf("SearchGoogleMapsPlaces: %v", err)
	}

	if len(places) != 1 || places[0].Name != "Cached Place" {
		t.Errorf("expected cached result, got %v", places)
	}
}
