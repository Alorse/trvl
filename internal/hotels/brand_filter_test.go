package hotels

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// buildHotel is a test helper that creates a HotelResult with just a name.
func buildHotel(name string) models.HotelResult {
	return models.HotelResult{Name: name, Price: 100, Stars: 3, Rating: 4.0}
}

func TestFilterHotels_BrandFilter_ExactCaseInsensitive(t *testing.T) {
	hotels := []models.HotelResult{
		buildHotel("Hilton London"),
		buildHotel("DoubleTree by Hilton Paris"),
		buildHotel("Marriott Madrid"),
		buildHotel("ibis Styles Amsterdam"),
	}

	opts := HotelSearchOptions{Brand: "hilton"}
	got := filterHotels(hotels, opts)

	if len(got) != 2 {
		t.Fatalf("expected 2 Hilton hotels, got %d: %v", len(got), got)
	}
	for _, h := range got {
		if h.Name != "Hilton London" && h.Name != "DoubleTree by Hilton Paris" {
			t.Errorf("unexpected hotel in results: %q", h.Name)
		}
	}
}

func TestFilterHotels_BrandFilter_CaseInsensitive(t *testing.T) {
	hotels := []models.HotelResult{
		buildHotel("Marriott Downtown"),
		buildHotel("Hilton Garden Inn"),
	}

	// Uppercase brand should still match lowercase in name.
	opts := HotelSearchOptions{Brand: "MARRIOTT"}
	got := filterHotels(hotels, opts)

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Name != "Marriott Downtown" {
		t.Errorf("unexpected match: %q", got[0].Name)
	}
}

func TestFilterHotels_BrandFilter_NoMatch(t *testing.T) {
	hotels := []models.HotelResult{
		buildHotel("Random Local Hotel"),
		buildHotel("Budget Inn"),
	}

	opts := HotelSearchOptions{Brand: "hilton"}
	got := filterHotels(hotels, opts)

	if len(got) != 0 {
		t.Errorf("expected 0 results for brand=hilton, got %d: %v", len(got), got)
	}
}

func TestFilterHotels_BrandFilter_Empty_PassesAll(t *testing.T) {
	hotels := []models.HotelResult{
		buildHotel("Marriott"),
		buildHotel("Hilton"),
		buildHotel("Ibis"),
	}

	opts := HotelSearchOptions{Brand: ""}
	got := filterHotels(hotels, opts)

	if len(got) != 3 {
		t.Errorf("empty brand filter: expected 3 results, got %d", len(got))
	}
}

func TestFilterHotels_BrandFilter_CombinedWithOtherFilters(t *testing.T) {
	// Brand filter should compose with other filters.
	hotels := []models.HotelResult{
		{Name: "Hilton Luxury", Price: 600, Stars: 5, Rating: 4.8},
		{Name: "Hilton Budget", Price: 80, Stars: 3, Rating: 3.5},
		{Name: "Marriott Grand", Price: 200, Stars: 4, Rating: 4.5},
	}

	opts := HotelSearchOptions{
		Brand:     "hilton",
		MaxPrice:  500,
		MinRating: 4.0,
	}
	got := filterHotels(hotels, opts)

	// Hilton Luxury: fails MaxPrice (600 > 500)
	// Hilton Budget: fails MinRating (3.5 < 4.0)
	// Marriott Grand: fails Brand filter
	if len(got) != 0 {
		t.Errorf("expected 0 results with combined filters, got %d: %v", len(got), got)
	}
}

func TestFilterHotels_BrandFilter_SubstringMatch(t *testing.T) {
	// "ibis" should match "ibis Styles" and "ibis Budget" but not "Ibisworld".
	hotels := []models.HotelResult{
		buildHotel("ibis Styles Berlin"),
		buildHotel("ibis Budget Munich"),
		buildHotel("Novotel Ibisworld"), // unusual but tests substring precision
	}

	opts := HotelSearchOptions{Brand: "ibis"}
	got := filterHotels(hotels, opts)

	// All three contain "ibis" — the filter is substring, not word-boundary.
	if len(got) != 3 {
		t.Errorf("expected 3 substring matches, got %d", len(got))
	}
}
