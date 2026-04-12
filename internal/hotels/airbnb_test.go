package hotels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ============================================================
// extractAirbnbScriptJSON
// ============================================================

func TestExtractAirbnbScriptJSON_Found(t *testing.T) {
	html := `<html><head></head><body>
<script id="data-deferred-state-0" type="application/json">{"key":"value"}</script>
</body></html>`

	got := extractAirbnbScriptJSON(html)
	if got != `{"key":"value"}` {
		t.Errorf("got %q, want %q", got, `{"key":"value"}`)
	}
}

func TestExtractAirbnbScriptJSON_NotFound(t *testing.T) {
	html := `<html><head></head><body><p>no script here</p></body></html>`
	got := extractAirbnbScriptJSON(html)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractAirbnbScriptJSON_EmptyContent(t *testing.T) {
	html := `<script id="data-deferred-state-0" type="application/json"></script>`
	got := extractAirbnbScriptJSON(html)
	if got != "" {
		t.Errorf("expected empty after trim, got %q", got)
	}
}

func TestExtractAirbnbScriptJSON_WithExtraAttributes(t *testing.T) {
	html := `<script data-foo="bar" id="data-deferred-state-0" type="application/json">{"ok":1}</script>`
	got := extractAirbnbScriptJSON(html)
	if got != `{"ok":1}` {
		t.Errorf("got %q, want {\"ok\":1}", got)
	}
}

// ============================================================
// extractAirbnbListings
// ============================================================

func TestExtractAirbnbListings_ValidPath(t *testing.T) {
	searchResult := map[string]any{
		"searchResults": []any{
			map[string]any{"listing": map[string]any{"id": "123", "name": "Cozy Studio"}},
		},
	}
	root := buildAirbnbRoot(searchResult)

	listings := extractAirbnbListings(root)
	if listings == nil {
		t.Fatal("expected non-nil listings")
	}
	if len(listings) != 1 {
		t.Errorf("expected 1 listing, got %d", len(listings))
	}
}

func TestExtractAirbnbListings_MissingNiobeClientData(t *testing.T) {
	root := map[string]any{"other": "data"}
	if extractAirbnbListings(root) != nil {
		t.Error("expected nil for missing niobeClientData")
	}
}

func TestExtractAirbnbListings_EmptyNiobeArray(t *testing.T) {
	root := map[string]any{"niobeClientData": []any{}}
	if extractAirbnbListings(root) != nil {
		t.Error("expected nil for empty niobeClientData")
	}
}

func TestExtractAirbnbListings_MissingStaysSearch(t *testing.T) {
	root := map[string]any{
		"niobeClientData": []any{
			[]any{
				nil,
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{},
					},
				},
			},
		},
	}
	if extractAirbnbListings(root) != nil {
		t.Error("expected nil for missing staysSearch")
	}
}

func TestExtractAirbnbListings_WrongType(t *testing.T) {
	root := map[string]any{"niobeClientData": "not an array"}
	if extractAirbnbListings(root) != nil {
		t.Error("expected nil for wrong type")
	}
}

// ============================================================
// mapAirbnbListing
// ============================================================

func TestMapAirbnbListing_BasicSuccess(t *testing.T) {
	item := map[string]any{
		"listing": map[string]any{
			"id":           "42",
			"name":         "Sunny Beach Cottage",
			"avgRating":    float64(4.9),
			"reviewsCount": float64(87),
		},
		"pricingQuote": map[string]any{
			"price": map[string]any{
				"total": map[string]any{
					"amount":   float64(300),
					"currency": "EUR",
				},
			},
		},
	}

	result, ok := mapAirbnbListing(item, 3)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result.Name != "Sunny Beach Cottage" {
		t.Errorf("name = %q", result.Name)
	}
	if result.Rating != 4.9 {
		t.Errorf("rating = %f, want 4.9", result.Rating)
	}
	if result.ReviewCount != 87 {
		t.Errorf("reviewCount = %d, want 87", result.ReviewCount)
	}
	if result.Stars != 0 {
		t.Errorf("stars = %d, want 0 (Airbnb has no star ratings)", result.Stars)
	}
	// 300 EUR / 3 nights = 100 EUR/night
	if result.Price != 100 {
		t.Errorf("price = %f, want 100", result.Price)
	}
	if result.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", result.Currency)
	}
	if result.BookingURL != "https://www.airbnb.com/rooms/42" {
		t.Errorf("bookingURL = %q", result.BookingURL)
	}
	if len(result.Sources) != 1 || result.Sources[0].Provider != "airbnb" {
		t.Errorf("sources = %+v, want one airbnb source", result.Sources)
	}
}

func TestMapAirbnbListing_MissingID(t *testing.T) {
	item := map[string]any{"listing": map[string]any{"name": "No ID Hotel"}}
	_, ok := mapAirbnbListing(item, 1)
	if ok {
		t.Error("expected ok=false when id is missing")
	}
}

func TestMapAirbnbListing_MissingName(t *testing.T) {
	item := map[string]any{"listing": map[string]any{"id": "99"}}
	_, ok := mapAirbnbListing(item, 1)
	if ok {
		t.Error("expected ok=false when name is missing")
	}
}

func TestMapAirbnbListing_NotAMap(t *testing.T) {
	_, ok := mapAirbnbListing("not a map", 1)
	if ok {
		t.Error("expected ok=false for non-map input")
	}
}

func TestMapAirbnbListing_NoPricingQuote(t *testing.T) {
	item := map[string]any{
		"listing": map[string]any{"id": "77", "name": "Price-less Place"},
	}
	result, ok := mapAirbnbListing(item, 2)
	if !ok {
		t.Fatal("expected ok=true even without price")
	}
	if result.Price != 0 {
		t.Errorf("price = %f, want 0 when pricingQuote is missing", result.Price)
	}
}

func TestMapAirbnbListing_SuperhostBadge(t *testing.T) {
	item := map[string]any{
		"listing": map[string]any{
			"id":     "11",
			"name":   "Superhost Villa",
			"badges": []any{"superhost"},
		},
	}
	result, ok := mapAirbnbListing(item, 1)
	if !ok {
		t.Fatal("expected ok=true")
	}
	found := false
	for _, a := range result.Amenities {
		if a == "superhost" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected superhost amenity, got %v", result.Amenities)
	}
}

func TestMapAirbnbListing_PropertyTypeTagged(t *testing.T) {
	item := map[string]any{
		"listing": map[string]any{
			"id":               "55",
			"name":             "Entire Home",
			"roomTypeCategory": "entire_home",
		},
	}
	result, ok := mapAirbnbListing(item, 1)
	if !ok {
		t.Fatal("expected ok=true")
	}
	found := false
	for _, a := range result.Amenities {
		if a == "type:apartment" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected type:apartment amenity, got %v", result.Amenities)
	}
}

// ============================================================
// extractAirbnbPrice
// ============================================================

func TestExtractAirbnbPrice_Path2TotalAmount(t *testing.T) {
	listing := map[string]any{
		"pricingQuote": map[string]any{
			"price": map[string]any{
				"total": map[string]any{
					"amount":   float64(450),
					"currency": "USD",
				},
			},
		},
	}
	price, cur := extractAirbnbPrice(listing, 3)
	if price != 150 {
		t.Errorf("price = %f, want 150", price)
	}
	if cur != "USD" {
		t.Errorf("currency = %q, want USD", cur)
	}
}

func TestExtractAirbnbPrice_Path1StructuredDisplay(t *testing.T) {
	listing := map[string]any{
		"pricingQuote": map[string]any{
			"structuredStayDisplayPrice": map[string]any{
				"primaryLine": map[string]any{
					"price": "USD 200",
				},
			},
		},
	}
	price, cur := extractAirbnbPrice(listing, 2)
	if price != 100 {
		t.Errorf("price = %f, want 100", price)
	}
	if cur != "USD" {
		t.Errorf("currency = %q, want USD", cur)
	}
}

func TestExtractAirbnbPrice_Path3PriceString(t *testing.T) {
	listing := map[string]any{
		"pricingQuote": map[string]any{
			"priceString": "EUR 180",
		},
	}
	price, cur := extractAirbnbPrice(listing, 1)
	if price != 180 {
		t.Errorf("price = %f, want 180", price)
	}
	if cur != "EUR" {
		t.Errorf("currency = %q, want EUR", cur)
	}
}

func TestExtractAirbnbPrice_NoPricingQuote(t *testing.T) {
	listing := map[string]any{}
	price, cur := extractAirbnbPrice(listing, 1)
	if price != 0 || cur != "" {
		t.Errorf("price=%f cur=%q, want 0/empty", price, cur)
	}
}

// ============================================================
// airbnbNights
// ============================================================

func TestAirbnbNights_Normal(t *testing.T) {
	if n := airbnbNights("2024-06-10", "2024-06-17"); n != 7 {
		t.Errorf("nights = %d, want 7", n)
	}
}

func TestAirbnbNights_OneNight(t *testing.T) {
	if n := airbnbNights("2024-06-10", "2024-06-11"); n != 1 {
		t.Errorf("nights = %d, want 1", n)
	}
}

func TestAirbnbNights_EmptyDates(t *testing.T) {
	if n := airbnbNights("", ""); n != 0 {
		t.Errorf("nights = %d, want 0", n)
	}
}

func TestAirbnbNights_InvalidDate(t *testing.T) {
	if n := airbnbNights("not-a-date", "2024-06-11"); n != 0 {
		t.Errorf("nights = %d, want 0", n)
	}
}

func TestAirbnbNights_Reversed(t *testing.T) {
	if n := airbnbNights("2024-06-17", "2024-06-10"); n != 0 {
		t.Errorf("nights = %d, want 0 for reversed dates", n)
	}
}

// ============================================================
// buildAirbnbURL
// ============================================================

func TestBuildAirbnbURL_ContainsLocation(t *testing.T) {
	u := buildAirbnbURL("Lisbon", HotelSearchOptions{
		CheckIn:  "2024-09-01",
		CheckOut: "2024-09-08",
		Guests:   2,
	})
	for _, want := range []string{"Lisbon", "checkin=2024-09-01", "checkout=2024-09-08", "adults=2"} {
		if !strings.Contains(u, want) {
			t.Errorf("URL %q missing %q", u, want)
		}
	}
}

func TestBuildAirbnbURL_PriceFilters(t *testing.T) {
	u := buildAirbnbURL("Barcelona", HotelSearchOptions{
		CheckIn:  "2024-10-01",
		CheckOut: "2024-10-03",
		MinPrice: 50,
		MaxPrice: 200,
	})
	if !strings.Contains(u, "price_min=50") {
		t.Errorf("URL %q missing price_min", u)
	}
	if !strings.Contains(u, "price_max=200") {
		t.Errorf("URL %q missing price_max", u)
	}
}

func TestBuildAirbnbURL_NoExtraPriceParams(t *testing.T) {
	u := buildAirbnbURL("Rome", HotelSearchOptions{
		CheckIn:  "2024-11-01",
		CheckOut: "2024-11-05",
	})
	if strings.Contains(u, "price_min") || strings.Contains(u, "price_max") {
		t.Errorf("URL %q should not contain price params when not set", u)
	}
}

// ============================================================
// mapAirbnbPropertyType
// ============================================================

func TestMapAirbnbPropertyType(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"entire_home", "apartment"},
		{"entire_home_apt", "apartment"},
		{"hotel_room", "hotel"},
		{"private_room", "hostel"},
		{"shared_room", "hostel"},
		{"mystery_type", ""},
	}
	for _, c := range cases {
		if got := mapAirbnbPropertyType(c.in); got != c.want {
			t.Errorf("mapAirbnbPropertyType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ============================================================
// parseAirbnbHTML — end-to-end with fixture HTML
// ============================================================

func TestParseAirbnbHTML_ValidFixture(t *testing.T) {
	listings := []any{
		map[string]any{
			"listing": map[string]any{
				"id":           "1001",
				"name":         "Charming Lisbon Flat",
				"avgRating":    float64(4.8),
				"reviewsCount": float64(120),
			},
			"pricingQuote": map[string]any{
				"price": map[string]any{
					"total": map[string]any{
						"amount":   float64(700),
						"currency": "EUR",
					},
				},
			},
		},
	}
	html := buildAirbnbFixtureHTML(t, listings)

	opts := HotelSearchOptions{CheckIn: "2024-09-01", CheckOut: "2024-09-08"}
	results, err := parseAirbnbHTML(html, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	h := results[0]
	if h.Name != "Charming Lisbon Flat" {
		t.Errorf("name = %q", h.Name)
	}
	if h.Rating != 4.8 {
		t.Errorf("rating = %f, want 4.8", h.Rating)
	}
	if h.Price != 100 { // 700/7 nights
		t.Errorf("price = %f, want 100", h.Price)
	}
	if h.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", h.Currency)
	}
	if len(h.Sources) != 1 || h.Sources[0].Provider != "airbnb" {
		t.Errorf("unexpected sources: %+v", h.Sources)
	}
}

func TestParseAirbnbHTML_NoScriptTag(t *testing.T) {
	html := `<html><body><p>Airbnb changed their HTML</p></body></html>`
	results, err := parseAirbnbHTML(html, HotelSearchOptions{CheckIn: "2024-09-01", CheckOut: "2024-09-08"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestParseAirbnbHTML_InvalidJSON(t *testing.T) {
	html := `<script id="data-deferred-state-0" type="application/json">NOT_JSON</script>`
	results, err := parseAirbnbHTML(html, HotelSearchOptions{CheckIn: "2024-09-01", CheckOut: "2024-09-08"})
	if err != nil {
		t.Fatalf("expected nil error on invalid JSON, got %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestParseAirbnbHTML_EmptyListings(t *testing.T) {
	html := buildAirbnbFixtureHTML(t, []any{})
	results, err := parseAirbnbHTML(html, HotelSearchOptions{CheckIn: "2024-09-01", CheckOut: "2024-09-08"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseAirbnbHTML_GracefulOnFormatChange(t *testing.T) {
	// Valid JSON but path doesn't exist — must not panic.
	html := `<script id="data-deferred-state-0" type="application/json">{"future":"format","niobeClientData":[]}</script>`
	results, err := parseAirbnbHTML(html, HotelSearchOptions{CheckIn: "2024-09-01", CheckOut: "2024-09-08"})
	if err != nil {
		t.Fatalf("expected nil error on format change, got %v", err)
	}
	_ = results
}

func TestParseAirbnbHTML_SkipsBadListings(t *testing.T) {
	listings := []any{
		map[string]any{"other": "data"}, // no "listing" key — should be skipped
		map[string]any{
			"listing": map[string]any{"id": "999", "name": "Valid Listing"},
		},
	}
	html := buildAirbnbFixtureHTML(t, listings)
	results, err := parseAirbnbHTML(html, HotelSearchOptions{CheckIn: "2024-09-01", CheckOut: "2024-09-02"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 valid result (skipping bad listing), got %d", len(results))
	}
}

// ============================================================
// SearchAirbnb — validation
// ============================================================

func TestSearchAirbnb_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := SearchAirbnb(ctx, "London", HotelSearchOptions{
		CheckIn:  "2024-09-01",
		CheckOut: "2024-09-08",
	})
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestSearchAirbnb_MissingDates(t *testing.T) {
	_, err := SearchAirbnb(context.Background(), "London", HotelSearchOptions{})
	if err == nil {
		t.Error("expected error when dates are missing")
	}
}

// ============================================================
// normalizeToPerNight
// ============================================================

func TestNormalizeToPerNight_Normal(t *testing.T) {
	if got := normalizeToPerNight(300, 3); got != 100 {
		t.Errorf("got %f, want 100", got)
	}
}

func TestNormalizeToPerNight_ZeroNights(t *testing.T) {
	if got := normalizeToPerNight(300, 0); got != 300 {
		t.Errorf("got %f, want 300 (passthrough)", got)
	}
}

// ============================================================
// Test helpers
// ============================================================

// buildAirbnbRoot assembles the full niobeClientData structure expected by
// extractAirbnbListings, wrapping the provided searchResult map.
func buildAirbnbRoot(searchResult map[string]any) any {
	return map[string]any{
		"niobeClientData": []any{
			[]any{
				nil,
				map[string]any{
					"data": map[string]any{
						"presentation": map[string]any{
							"staysSearch": map[string]any{
								"results": searchResult,
							},
						},
					},
				},
			},
		},
	}
}

// buildAirbnbFixtureHTML creates minimal Airbnb-like HTML embedding listings
// in the expected JSON structure under data-deferred-state-0.
func buildAirbnbFixtureHTML(t *testing.T, listings []any) string {
	t.Helper()
	root := buildAirbnbRoot(map[string]any{"searchResults": listings})
	b, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("buildAirbnbFixtureHTML: marshal failed: %v", err)
	}
	return fmt.Sprintf(
		`<!DOCTYPE html><html><head></head><body>`+
			`<script id="data-deferred-state-0" type="application/json">%s</script>`+
			`</body></html>`,
		string(b),
	)
}
