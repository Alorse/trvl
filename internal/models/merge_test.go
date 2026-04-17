package models

import (
	"testing"
)

func TestMergeHotelResults_NoDuplicates(t *testing.T) {
	a := []HotelResult{{Name: "Hotel A", Price: 100, Currency: "EUR", Sources: []PriceSource{{Provider: "google_hotels", Price: 100, Currency: "EUR"}}}}
	b := []HotelResult{{Name: "Hotel B", Price: 200, Currency: "EUR", Sources: []PriceSource{{Provider: "trivago", Price: 200, Currency: "EUR"}}}}

	result := MergeHotelResults(a, b)
	if len(result) != 2 {
		t.Fatalf("expected 2 hotels, got %d", len(result))
	}
}

func TestMergeHotelResults_MergesSameHotel(t *testing.T) {
	a := []HotelResult{{Name: "Hilton Barcelona", Price: 150, Currency: "EUR", Sources: []PriceSource{{Provider: "google_hotels", Price: 150, Currency: "EUR"}}}}
	b := []HotelResult{{Name: "hilton barcelona", Price: 128, Currency: "EUR", Sources: []PriceSource{{Provider: "trivago", Price: 128, Currency: "EUR"}}}}

	result := MergeHotelResults(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged hotel, got %d", len(result))
	}
	if result[0].Price != 128 {
		t.Errorf("expected lowest price 128, got %.0f", result[0].Price)
	}
	if len(result[0].Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(result[0].Sources))
	}
}

func TestMergeHotelResults_KeepsLowestPrice(t *testing.T) {
	a := []HotelResult{{Name: "Test Hotel", Price: 200, Currency: "EUR", Sources: []PriceSource{{Provider: "google_hotels", Price: 200, Currency: "EUR"}}}}
	b := []HotelResult{{Name: "Test Hotel", Price: 180, Currency: "EUR", Sources: []PriceSource{{Provider: "trivago", Price: 180, Currency: "EUR"}}}}
	c := []HotelResult{{Name: "Test Hotel", Price: 195, Currency: "EUR", Sources: []PriceSource{{Provider: "airbnb", Price: 195, Currency: "EUR"}}}}

	result := MergeHotelResults(a, b, c)
	if len(result) != 1 {
		t.Fatalf("expected 1 hotel, got %d", len(result))
	}
	if result[0].Price != 180 {
		t.Errorf("expected lowest price 180, got %.0f", result[0].Price)
	}
	if len(result[0].Sources) != 3 {
		t.Errorf("expected 3 sources, got %d", len(result[0].Sources))
	}
}

func TestMergeHotelResults_GeoDisambiguation(t *testing.T) {
	// Same name but different cities (>200m apart).
	a := []HotelResult{{Name: "Hilton", Price: 150, Currency: "EUR", Lat: 41.3851, Lon: 2.1734, Address: "Barcelona", Sources: []PriceSource{{Provider: "google_hotels", Price: 150, Currency: "EUR"}}}}
	b := []HotelResult{{Name: "Hilton", Price: 200, Currency: "EUR", Lat: 48.8566, Lon: 2.3522, Address: "Paris", Sources: []PriceSource{{Provider: "trivago", Price: 200, Currency: "EUR"}}}}

	result := MergeHotelResults(a, b)
	if len(result) != 2 {
		t.Fatalf("expected 2 hotels (different cities), got %d", len(result))
	}
}

func TestMergeHotelResults_GeoProximityMerges(t *testing.T) {
	// Same hotel, slightly different coordinates (within 500m).
	a := []HotelResult{{Name: "Hilton Barcelona", Price: 150, Currency: "EUR", Lat: 41.3851, Lon: 2.1734, Sources: []PriceSource{{Provider: "google_hotels", Price: 150, Currency: "EUR"}}}}
	b := []HotelResult{{Name: "hilton barcelona", Price: 140, Currency: "EUR", Lat: 41.3852, Lon: 2.1735, Sources: []PriceSource{{Provider: "trivago", Price: 140, Currency: "EUR"}}}}

	result := MergeHotelResults(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged hotel (same location), got %d", len(result))
	}
}

func TestMergeHotelResults_AddressMatchOverridesGeoDrift(t *testing.T) {
	a := []HotelResult{{
		Name:     "Grand Hotel",
		Price:    150,
		Currency: "EUR",
		Lat:      60.1699,
		Lon:      24.9384,
		Address:  "Example Street 1",
		Sources:  []PriceSource{{Provider: "google_hotels", Price: 150, Currency: "EUR"}},
	}}
	b := []HotelResult{{
		Name:     "grand hotel",
		Price:    120,
		Currency: "EUR",
		Lat:      60.1760,
		Lon:      24.9384,
		Address:  "Example Street 1",
		Sources:  []PriceSource{{Provider: "booking", Price: 120, Currency: "EUR"}},
	}}

	result := MergeHotelResults(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged hotel, got %d", len(result))
	}
	if result[0].Price != 120 {
		t.Fatalf("expected merged price 120, got %.0f", result[0].Price)
	}
}

func TestMergeHotelResults_PreservesHotelIDFromLaterSource(t *testing.T) {
	a := []HotelResult{{
		Name:       "Grand Hotel",
		Price:      120,
		Currency:   "EUR",
		BookingURL: "https://www.booking.com/hotel/fi/grand.html",
		Sources:    []PriceSource{{Provider: "booking", Price: 120, Currency: "EUR"}},
	}}
	b := []HotelResult{{
		Name:       "Grand Hotel",
		HotelID:    "/g/123",
		Price:      130,
		Currency:   "EUR",
		BookingURL: "https://www.google.com/travel/hotels/example",
		Sources:    []PriceSource{{Provider: "google_hotels", Price: 130, Currency: "EUR"}},
	}}

	result := MergeHotelResults(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged hotel, got %d", len(result))
	}
	if result[0].HotelID != "/g/123" {
		t.Fatalf("hotel_id = %q, want /g/123", result[0].HotelID)
	}
}

func TestMergeHotelResults_DifferentAddressesDoNotMergeWithoutCoordinates(t *testing.T) {
	a := []HotelResult{{Name: "Hilton", Price: 150, Currency: "EUR", Address: "Barcelona", Sources: []PriceSource{{Provider: "google_hotels", Price: 150, Currency: "EUR"}}}}
	b := []HotelResult{{Name: "Hilton", Price: 200, Currency: "EUR", Address: "Paris", Sources: []PriceSource{{Provider: "booking", Price: 200, Currency: "EUR"}}}}

	result := MergeHotelResults(a, b)
	if len(result) != 2 {
		t.Fatalf("expected 2 hotels, got %d", len(result))
	}
}

func TestMergeHotelResults_EmptyInputs(t *testing.T) {
	result := MergeHotelResults(nil, nil)
	if len(result) != 0 {
		t.Fatalf("expected 0 hotels, got %d", len(result))
	}
}

func TestMergeHotelResults_SingleSource(t *testing.T) {
	a := []HotelResult{
		{Name: "Hotel A", Price: 100, Currency: "EUR"},
		{Name: "Hotel B", Price: 200, Currency: "EUR"},
	}
	result := MergeHotelResults(a)
	if len(result) != 2 {
		t.Fatalf("expected 2 hotels, got %d", len(result))
	}
}

// TestMergeHotelResults_GeoProximityDedup verifies that hotels with different
// name variants but the same physical location are merged across providers.
// This is the core cross-provider dedup scenario: Google Hotels calls it
// "Holiday Inn Express Amsterdam - Arena Towers" and Booking calls it
// "Holiday Inn Express Amsterdam Arena Towers by IHG" — different names
// but within 100m of each other.
func TestMergeHotelResults_GeoProximityDedup(t *testing.T) {
	google := []HotelResult{
		{
			Name: "Holiday Inn Express Amsterdam - Arena Towers", Price: 120, Currency: "EUR",
			Rating: 4.3, ReviewCount: 5000, Lat: 52.3096, Lon: 4.9418,
			Sources: []PriceSource{{Provider: "google_hotels", Price: 120, Currency: "EUR"}},
		},
	}
	booking := []HotelResult{
		{
			Name: "Holiday Inn Express Amsterdam Arena Towers by IHG", Price: 110, Currency: "EUR",
			Rating: 0, ReviewCount: 0, Lat: 52.3096, Lon: 4.9419, // ~10m away
			Sources: []PriceSource{{Provider: "booking", Price: 110, Currency: "EUR"}},
		},
	}
	result := MergeHotelResults(google, booking)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged hotel, got %d", len(result))
	}
	h := result[0]
	// Should have Google's rating + Booking's lower price.
	if h.Rating != 4.3 {
		t.Errorf("rating = %v, want 4.3 (from Google)", h.Rating)
	}
	if h.Price != 110 {
		t.Errorf("price = %v, want 110 (cheapest)", h.Price)
	}
	if len(h.Sources) != 2 {
		t.Errorf("sources = %d, want 2 (both providers)", len(h.Sources))
	}
}

func TestMergeHotelResults_MergesMissingFields(t *testing.T) {
	a := []HotelResult{{Name: "Hotel X", Price: 100, Currency: "EUR", Rating: 4.5, Stars: 4}}
	b := []HotelResult{{Name: "hotel x", Price: 90, Currency: "EUR", Address: "123 Main St", ReviewCount: 500}}

	result := MergeHotelResults(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 hotel, got %d", len(result))
	}
	h := result[0]
	if h.Rating != 4.5 {
		t.Errorf("expected rating 4.5, got %f", h.Rating)
	}
	if h.Stars != 4 {
		t.Errorf("expected stars 4, got %d", h.Stars)
	}
	if h.Address != "123 Main St" {
		t.Errorf("expected address from second source, got %q", h.Address)
	}
	if h.ReviewCount != 500 {
		t.Errorf("expected review count 500, got %d", h.ReviewCount)
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"  Hilton  Barcelona  ", "hilton barcelona"},
		{"HOTEL ABC", "hotel abc"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeAddress(t *testing.T) {
	tests := []struct{ input, want string }{
		{" Example Street 1 ", "example street 1"},
		{"Rue-de-Paris, 5", "rue de paris 5"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeAddress(tt.input)
		if got != tt.want {
			t.Errorf("normalizeAddress(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHasExternalProviderSource(t *testing.T) {
	tests := []struct {
		name  string
		hotel HotelResult
		want  bool
	}{
		{
			name:  "google only",
			hotel: HotelResult{Sources: []PriceSource{{Provider: "google_hotels"}}},
			want:  false,
		},
		{
			name:  "trivago only",
			hotel: HotelResult{Sources: []PriceSource{{Provider: "trivago"}}},
			want:  false,
		},
		{
			name:  "hostelworld",
			hotel: HotelResult{Sources: []PriceSource{{Provider: "hostelworld"}}},
			want:  true,
		},
		{
			name:  "airbnb",
			hotel: HotelResult{Sources: []PriceSource{{Provider: "airbnb"}}},
			want:  true,
		},
		{
			name: "mixed google and booking",
			hotel: HotelResult{Sources: []PriceSource{
				{Provider: "google_hotels"},
				{Provider: "booking"},
			}},
			want: true,
		},
		{
			name:  "no sources",
			hotel: HotelResult{},
			want:  false,
		},
		{
			name:  "empty provider string",
			hotel: HotelResult{Sources: []PriceSource{{Provider: ""}}},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasExternalProviderSource(tt.hotel); got != tt.want {
				t.Errorf("HasExternalProviderSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeHotelResults_DeduplicatesSources(t *testing.T) {
	// Simulate Google Hotels returning the same hotel from multiple sort
	// orders / pagination pages — all with identical provider+price+currency.
	pages := make([][]HotelResult, 9)
	for i := range pages {
		pages[i] = []HotelResult{{
			Name:     "Clarion Hotel Helsinki",
			Price:    105,
			Currency: "EUR",
			Sources:  []PriceSource{{Provider: "google_hotels", Price: 105, Currency: "EUR"}},
		}}
	}

	result := MergeHotelResults(pages...)
	if len(result) != 1 {
		t.Fatalf("expected 1 hotel, got %d", len(result))
	}
	if len(result[0].Sources) != 1 {
		t.Errorf("expected 1 deduplicated source, got %d", len(result[0].Sources))
	}
}

func TestMergeHotelResults_KeepsDifferentPricesSameProvider(t *testing.T) {
	// Same provider but different prices should be kept (e.g. price changed
	// between pages, or different room types).
	a := []HotelResult{{
		Name:     "Clarion Hotel Helsinki",
		Price:    105,
		Currency: "EUR",
		Sources:  []PriceSource{{Provider: "google_hotels", Price: 105, Currency: "EUR"}},
	}}
	b := []HotelResult{{
		Name:     "Clarion Hotel Helsinki",
		Price:    115,
		Currency: "EUR",
		Sources:  []PriceSource{{Provider: "google_hotels", Price: 115, Currency: "EUR"}},
	}}

	result := MergeHotelResults(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 hotel, got %d", len(result))
	}
	if len(result[0].Sources) != 2 {
		t.Errorf("expected 2 sources (different prices), got %d", len(result[0].Sources))
	}
	if result[0].Price != 105 {
		t.Errorf("expected lowest price 105, got %.0f", result[0].Price)
	}
}

func TestDeduplicateSources(t *testing.T) {
	sources := []PriceSource{
		{Provider: "google_hotels", Price: 105, Currency: "EUR"},
		{Provider: "google_hotels", Price: 105, Currency: "EUR"},
		{Provider: "google_hotels", Price: 105, Currency: "EUR"},
		{Provider: "booking", Price: 110, Currency: "EUR"},
		{Provider: "booking", Price: 110, Currency: "EUR"},
		{Provider: "google_hotels", Price: 120, Currency: "EUR"},
	}
	got := deduplicateSources(sources)
	if len(got) != 3 {
		t.Errorf("expected 3 unique sources, got %d", len(got))
	}
}

func TestHaversineMeters(t *testing.T) {
	// Helsinki to Tallinn ≈ 80km.
	dist := haversineMeters(60.1699, 24.9384, 59.4370, 24.7536)
	if dist < 70000 || dist > 90000 {
		t.Errorf("Helsinki-Tallinn expected ~80km, got %.0fm", dist)
	}

	// Same point = 0.
	dist = haversineMeters(60.17, 24.94, 60.17, 24.94)
	if dist != 0 {
		t.Errorf("same point expected 0m, got %.0fm", dist)
	}
}
