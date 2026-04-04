package ground

import (
	"context"
	"strings"
	"testing"
)

func TestEstimateTaxiTransfer(t *testing.T) {
	route, err := EstimateTaxiTransfer(context.Background(), TaxiEstimateInput{
		FromName:    "Paris Charles de Gaulle Airport",
		ToName:      "Hotel Lutetia Paris",
		FromLat:     49.0097,
		FromLon:     2.5479,
		ToLat:       48.8510,
		ToLon:       2.3259,
		CountryCode: "FR",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Provider != "taxi" {
		t.Fatalf("provider = %q, want taxi", route.Provider)
	}
	if route.Type != "taxi" {
		t.Fatalf("type = %q, want taxi", route.Type)
	}
	if route.Currency != "EUR" {
		t.Fatalf("currency = %q, want EUR", route.Currency)
	}
	if route.Price <= 0 {
		t.Fatalf("price = %.2f, want > 0", route.Price)
	}
	if route.PriceMax <= route.Price {
		t.Fatalf("price range = %.2f-%.2f, want max > min", route.Price, route.PriceMax)
	}
	if route.Duration <= 0 {
		t.Fatalf("duration = %d, want > 0", route.Duration)
	}
	if route.Transfers != 0 {
		t.Fatalf("transfers = %d, want 0", route.Transfers)
	}
	if !strings.Contains(route.BookingURL, "travelmode=driving") {
		t.Fatalf("booking URL = %q, want Google Maps driving URL", route.BookingURL)
	}
	if !strings.Contains(strings.Join(route.Amenities, ","), "estimated fare") {
		t.Fatalf("amenities = %v, want estimated fare note", route.Amenities)
	}
}

func TestEstimateTaxiTransfer_RequiresDistinctCoordinates(t *testing.T) {
	_, err := EstimateTaxiTransfer(context.Background(), TaxiEstimateInput{
		FromName: "A",
		ToName:   "B",
		FromLat:  48.8566,
		FromLon:  2.3522,
		ToLat:    48.8566,
		ToLon:    2.3522,
	})
	if err == nil || !strings.Contains(err.Error(), "distinct coordinates") {
		t.Fatalf("expected distinct coordinates error, got %v", err)
	}
}

func TestTaxiFareMultiplier(t *testing.T) {
	if got := taxiFareMultiplier("CH"); got <= 1.0 {
		t.Fatalf("CH multiplier = %.2f, want > 1", got)
	}
	if got := taxiFareMultiplier("CZ"); got >= 1.0 {
		t.Fatalf("CZ multiplier = %.2f, want < 1", got)
	}
	if got := taxiFareMultiplier("ZZ"); got != 1.0 {
		t.Fatalf("unknown country multiplier = %.2f, want 1", got)
	}
}

func TestEstimateTaxiDurationMinutes_GrowsWithDistance(t *testing.T) {
	short := estimateTaxiDurationMinutes(5)
	long := estimateTaxiDurationMinutes(30)
	if short <= 0 {
		t.Fatalf("short duration = %d, want > 0", short)
	}
	if long <= short {
		t.Fatalf("duration should grow with distance, got short=%d long=%d", short, long)
	}
}
