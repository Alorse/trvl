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
	// Same hotel, slightly different coordinates (within 200m).
	a := []HotelResult{{Name: "Hilton Barcelona", Price: 150, Currency: "EUR", Lat: 41.3851, Lon: 2.1734, Sources: []PriceSource{{Provider: "google_hotels", Price: 150, Currency: "EUR"}}}}
	b := []HotelResult{{Name: "hilton barcelona", Price: 140, Currency: "EUR", Lat: 41.3852, Lon: 2.1735, Sources: []PriceSource{{Provider: "trivago", Price: 140, Currency: "EUR"}}}}

	result := MergeHotelResults(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged hotel (same location), got %d", len(result))
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
