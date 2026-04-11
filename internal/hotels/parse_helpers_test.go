package hotels

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ============================================================
// parsePriceString
// ============================================================

func TestParsePriceString_CurrencyFirst(t *testing.T) {
	price, cur := parsePriceString("PLN 420")
	if price != 420 {
		t.Errorf("price = %f, want 420", price)
	}
	if cur != "PLN" {
		t.Errorf("currency = %q, want PLN", cur)
	}
}

func TestParsePriceString_AmountFirst(t *testing.T) {
	price, cur := parsePriceString("420 PLN")
	if price != 420 {
		t.Errorf("price = %f, want 420", price)
	}
	if cur != "PLN" {
		t.Errorf("currency = %q, want PLN", cur)
	}
}

func TestParsePriceString_SymbolAttached(t *testing.T) {
	price, cur := parsePriceString("$150")
	if price != 150 {
		t.Errorf("price = %f, want 150", price)
	}
	if cur != "" {
		t.Errorf("currency = %q, want empty for symbol-attached", cur)
	}
}

func TestParsePriceString_EuroSymbol(t *testing.T) {
	price, _ := parsePriceString("\u20ac98")
	if price != 98 {
		t.Errorf("price = %f, want 98", price)
	}
}

func TestParsePriceString_WithComma(t *testing.T) {
	price, cur := parsePriceString("USD 1,250")
	if price != 1250 {
		t.Errorf("price = %f, want 1250", price)
	}
	if cur != "USD" {
		t.Errorf("currency = %q, want USD", cur)
	}
}

func TestParsePriceString_Empty(t *testing.T) {
	price, cur := parsePriceString("")
	if price != 0 {
		t.Errorf("price = %f, want 0", price)
	}
	if cur != "" {
		t.Errorf("currency = %q, want empty", cur)
	}
}

func TestParsePriceString_NoParseable(t *testing.T) {
	price, _ := parsePriceString("free")
	if price != 0 {
		t.Errorf("price = %f, want 0 for non-parseable input", price)
	}
}

func TestParsePriceString_InvalidCurrencyCode(t *testing.T) {
	price, cur := parsePriceString("eu 100")
	if price != 100 {
		t.Errorf("price = %f, want 100", price)
	}
	if cur != "" {
		t.Errorf("currency = %q, want empty for invalid code", cur)
	}
}

func TestParsePriceString_Float(t *testing.T) {
	price, cur := parsePriceString("USD 150.50")
	if price != 150.50 {
		t.Errorf("price = %f, want 150.50", price)
	}
	if cur != "USD" {
		t.Errorf("currency = %q, want USD", cur)
	}
}

// ============================================================
// deduplicateHotels
// ============================================================

func TestDeduplicateHotels_NoDupes(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hotel A"}, {Name: "Hotel B"}, {Name: "Hotel C"},
	}
	result := deduplicateHotels(hotels)
	if len(result) != 3 {
		t.Errorf("expected 3, got %d", len(result))
	}
}

func TestDeduplicateHotels_CaseInsensitive(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hotel Alpha", Price: 100},
		{Name: "HOTEL ALPHA", Price: 200},
		{Name: "Hotel Beta"},
	}
	result := deduplicateHotels(hotels)
	if len(result) != 2 {
		t.Errorf("expected 2 unique hotels, got %d", len(result))
	}
	if result[0].Price != 100 {
		t.Errorf("expected first occurrence to win, got price %f", result[0].Price)
	}
}

// ============================================================
// enrichAmenitiesFromDescription — unique tests
// ============================================================

func TestEnrichAmenitiesFromDescription_AddsNewFromParse(t *testing.T) {
	existing := []string{"air_conditioning"}
	result := enrichAmenitiesFromDescription(existing, "This hotel features a lovely pool and free wifi")
	found := map[string]bool{}
	for _, a := range result {
		found[a] = true
	}
	if !found["pool"] {
		t.Error("expected pool to be added from description")
	}
	if !found["free_wifi"] {
		t.Error("expected free_wifi to be added from description")
	}
	if !found["air_conditioning"] {
		t.Error("expected existing amenity to be preserved")
	}
}

func TestEnrichAmenitiesFromDescription_EmptyDescPreservesExisting(t *testing.T) {
	existing := []string{"pool"}
	result := enrichAmenitiesFromDescription(existing, "")
	if len(result) != 1 {
		t.Errorf("expected 1, got %d", len(result))
	}
}

// ============================================================
// extractSponsoredAmenities — unique tests
// ============================================================

func TestExtractSponsoredAmenities_DedupAccessible(t *testing.T) {
	// codes 11 and 54 both map to "accessible"
	raw := []any{float64(11), float64(54)}
	result := extractSponsoredAmenities(raw)
	if len(result) != 1 {
		t.Errorf("expected 1 (deduped accessible), got %d", len(result))
	}
}

func TestExtractSponsoredAmenities_NilInputFromParse(t *testing.T) {
	result := extractSponsoredAmenities(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}
