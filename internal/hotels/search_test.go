package hotels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// TestParseDateArray validates date string to [year, month, day] conversion.
func TestParseDateArray(t *testing.T) {
	tests := []struct {
		input   string
		want    [3]int
		wantErr bool
	}{
		{"2026-06-15", [3]int{2026, 6, 15}, false},
		{"2026-01-01", [3]int{2026, 1, 1}, false},
		{"2026-12-31", [3]int{2026, 12, 31}, false},
		{"bad-date", [3]int{}, true},
		{"", [3]int{}, true},
		{"2026/06/15", [3]int{}, true},
	}

	for _, tt := range tests {
		got, err := parseDateArray(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseDateArray(%q) expected error, got %v", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDateArray(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseDateArray(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseHotelSearchResponse tests parsing of mock hotel search data.
func TestParseHotelSearchResponse(t *testing.T) {
	// Simulated batchexecute response structure with hotel entries.
	// This mimics the structure: [["wrb.fr","AtySUc","<json>", ...]]
	hotelData := []any{
		[]any{
			"Hotel Kamp",                  // [0] name
			"/g/11b6d4_v_4",              // [1] hotel ID
			4.6,                           // [2] rating
			1523.0,                        // [3] review count
			5.0,                           // [4] stars
			[]any{189.0, "USD"},          // [5] price info
			"Pohjoisesplanadi 29, Helsinki", // [6] address
			[]any{60.168, 24.941},         // [7] coordinates
			[]any{"WiFi", "Pool", "Spa"},  // [8] amenities
		},
		[]any{
			"Scandic Grand Central Helsinki",
			"/g/11c6rk8_qb",
			4.3,
			892.0,
			4.0,
			[]any{129.0, "USD"},
			"Vilhonkatu 13, Helsinki",
			[]any{60.170, 24.943},
			[]any{"WiFi", "Restaurant"},
		},
	}

	inner, _ := json.Marshal(hotelData)
	// Wrap in the batchexecute response format.
	entries := []any{
		[]any{
			[]any{"wrb.fr", "AtySUc", string(inner), nil, nil, nil, "generic"},
		},
	}

	hotels, err := ParseHotelSearchResponse(entries, "USD")
	if err != nil {
		t.Fatalf("ParseHotelSearchResponse error: %v", err)
	}

	if len(hotels) < 2 {
		t.Fatalf("expected at least 2 hotels, got %d", len(hotels))
	}

	// Verify first hotel fields.
	h := hotels[0]
	if h.Name != "Hotel Kamp" {
		t.Errorf("hotel[0].Name = %q, want %q", h.Name, "Hotel Kamp")
	}
	if h.HotelID != "/g/11b6d4_v_4" {
		t.Errorf("hotel[0].HotelID = %q, want %q", h.HotelID, "/g/11b6d4_v_4")
	}
	if h.Rating != 4.6 {
		t.Errorf("hotel[0].Rating = %v, want 4.6", h.Rating)
	}
}

// TestParseHotelPriceResponse tests parsing of mock provider price data.
func TestParseHotelPriceResponse(t *testing.T) {
	priceData := []any{
		[]any{
			"Booking.com",
			189.0,
			"USD",
			"https://www.booking.com/...",
		},
		[]any{
			"Hotels.com",
			195.0,
			"USD",
			"https://www.hotels.com/...",
		},
		[]any{
			"Expedia",
			192.0,
			"USD",
			"https://www.expedia.com/...",
		},
	}

	inner, _ := json.Marshal(priceData)
	entries := []any{
		[]any{
			[]any{"wrb.fr", "yY52ce", string(inner), nil, nil, nil, "generic"},
		},
	}

	prices, err := ParseHotelPriceResponse(entries)
	if err != nil {
		t.Fatalf("ParseHotelPriceResponse error: %v", err)
	}

	if len(prices) < 3 {
		t.Fatalf("expected at least 3 providers, got %d", len(prices))
	}

	if prices[0].Provider != "Booking.com" {
		t.Errorf("prices[0].Provider = %q, want %q", prices[0].Provider, "Booking.com")
	}
	if prices[0].Price != 189.0 {
		t.Errorf("prices[0].Price = %v, want 189.0", prices[0].Price)
	}
	if prices[0].Currency != "USD" {
		t.Errorf("prices[0].Currency = %q, want %q", prices[0].Currency, "USD")
	}
}

// TestSortHotels verifies sorting by price and rating.
func TestSortHotels(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Expensive", Price: 300},
		{Name: "Cheap", Price: 100},
		{Name: "Mid", Price: 200},
		{Name: "No Price", Price: 0},
	}

	sortHotels(hotels, "cheapest")
	if hotels[0].Name != "Cheap" {
		t.Errorf("cheapest sort: first hotel = %q, want %q", hotels[0].Name, "Cheap")
	}
	if hotels[1].Name != "Mid" {
		t.Errorf("cheapest sort: second hotel = %q, want %q", hotels[1].Name, "Mid")
	}
	// Hotels with price=0 should be at the end.
	if hotels[3].Name != "No Price" {
		t.Errorf("cheapest sort: last hotel = %q, want %q", hotels[3].Name, "No Price")
	}

	// Rating sort.
	hotels2 := []models.HotelResult{
		{Name: "Low", Rating: 3.5},
		{Name: "High", Rating: 4.8},
		{Name: "Mid", Rating: 4.2},
	}
	sortHotels(hotels2, "rating")
	if hotels2[0].Name != "High" {
		t.Errorf("rating sort: first hotel = %q, want %q", hotels2[0].Name, "High")
	}
}

// TestFilterByStars verifies star rating filtering.
func TestFilterByStars(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Two Star", Stars: 2},
		{Name: "Four Star", Stars: 4},
		{Name: "Five Star", Stars: 5},
		{Name: "Three Star", Stars: 3},
	}

	filtered := filterByStars(hotels, 4)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 hotels with >= 4 stars, got %d", len(filtered))
	}
	for _, h := range filtered {
		if h.Stars < 4 {
			t.Errorf("hotel %q has %d stars, expected >= 4", h.Name, h.Stars)
		}
	}
}

// TestGeocodeCache verifies that the geocode cache works.
func TestGeocodeCache(t *testing.T) {
	// Manually prime the cache.
	geoCache.Lock()
	geoCache.entries["TestCity"] = geoEntry{lat: 60.17, lon: 24.94}
	geoCache.Unlock()

	lat, lon, err := ResolveLocation(context.Background(), "TestCity")
	if err != nil {
		t.Fatalf("ResolveLocation from cache error: %v", err)
	}
	if lat != 60.17 || lon != 24.94 {
		t.Errorf("got (%v, %v), want (60.17, 24.94)", lat, lon)
	}

	// Clean up.
	geoCache.Lock()
	delete(geoCache.entries, "TestCity")
	geoCache.Unlock()
}

// TestNominatimLookup tests the Nominatim API integration with a mock server.
func TestNominatimLookup(t *testing.T) {
	// Create a mock Nominatim server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"lat":"60.1695","lon":"24.9354","display_name":"Helsinki, Finland"}]`))
	}))
	defer ts.Close()

	// We can't easily mock the global nominatimURL, so we test the parsing
	// via ParseHotelSearchResponse and ResolveLocation cache path instead.
	// The mock server test is here for documentation of the expected API format.
	t.Logf("Mock Nominatim server running at %s", ts.URL)
}

// TestExtractBatchPayload tests the batchexecute response extraction.
func TestExtractBatchPayload(t *testing.T) {
	inner := `[["Hotel A","/g/123",4.5]]`
	entries := []any{
		[]any{
			[]any{"wrb.fr", "AtySUc", inner, nil, nil, nil, "generic"},
		},
	}

	payload, err := extractBatchPayload(entries, "AtySUc")
	if err != nil {
		t.Fatalf("extractBatchPayload error: %v", err)
	}

	arr, ok := payload.([]any)
	if !ok {
		t.Fatalf("payload not array, got %T", payload)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(arr))
	}
}

// TestExtractBatchPayload_NotFound tests error on missing rpcid.
func TestExtractBatchPayload_NotFound(t *testing.T) {
	entries := []any{
		[]any{
			[]any{"wrb.fr", "OtherRPC", `[]`, nil},
		},
	}

	_, err := extractBatchPayload(entries, "AtySUc")
	if err == nil {
		t.Error("expected error for missing rpcid, got nil")
	}
}

// TestLooksLikeHotelEntry verifies the hotel entry heuristic.
func TestLooksLikeHotelEntry(t *testing.T) {
	tests := []struct {
		name string
		arr  []any
		want bool
	}{
		{
			name: "valid hotel",
			arr:  []any{"Hotel Kamp", "/g/123", 4.5, 1000.0, 5.0},
			want: true,
		},
		{
			name: "too short",
			arr:  []any{"Hotel"},
			want: false,
		},
		{
			name: "no name string",
			arr:  []any{1.0, 2.0, 3.0, 4.0, 5.0},
			want: false,
		},
		{
			name: "slash prefix not name",
			arr:  []any{"/g/123", nil, nil, nil, nil},
			want: false,
		},
	}

	for _, tt := range tests {
		got := looksLikeHotelEntry(tt.arr)
		if got != tt.want {
			t.Errorf("looksLikeHotelEntry(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestSearchHotels_ValidationErrors tests input validation.
func TestSearchHotels_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	// Missing dates.
	_, err := SearchHotels(ctx, "Helsinki", HotelSearchOptions{})
	if err == nil {
		t.Error("expected error for missing dates, got nil")
	}

	// Bad date format.
	_, err = SearchHotels(ctx, "Helsinki", HotelSearchOptions{
		CheckIn:  "bad",
		CheckOut: "2026-06-18",
	})
	if err == nil {
		t.Error("expected error for bad date, got nil")
	}
}

// TestGetHotelPrices_ValidationErrors tests price lookup validation.
func TestGetHotelPrices_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	// Missing hotel ID.
	_, err := GetHotelPrices(ctx, "", "2026-06-15", "2026-06-18", "USD")
	if err == nil {
		t.Error("expected error for empty hotel ID, got nil")
	}

	// Missing dates.
	_, err = GetHotelPrices(ctx, "/g/123", "", "2026-06-18", "USD")
	if err == nil {
		t.Error("expected error for missing check-in, got nil")
	}

	// Bad date.
	_, err = GetHotelPrices(ctx, "/g/123", "not-a-date", "2026-06-18", "USD")
	if err == nil {
		t.Error("expected error for bad date, got nil")
	}
}

// TestParseOneHotel verifies single hotel entry parsing.
func TestParseOneHotel(t *testing.T) {
	entry := []any{
		"Grand Hotel",
		"/g/11abc",
		4.2,
		500.0,
		3.0,
		[]any{150.0, "EUR"},
		"Main Street 1, City, Country",
		[]any{51.5074, -0.1278},
		[]any{"WiFi", "Parking", "Restaurant"},
	}

	h := parseOneHotel(entry, "EUR")

	if h.Name != "Grand Hotel" {
		t.Errorf("Name = %q, want %q", h.Name, "Grand Hotel")
	}
	if h.HotelID != "/g/11abc" {
		t.Errorf("HotelID = %q, want %q", h.HotelID, "/g/11abc")
	}
	if h.Rating != 4.2 {
		t.Errorf("Rating = %v, want 4.2", h.Rating)
	}
	if h.Lat == 0 || h.Lon == 0 {
		t.Error("expected non-zero coordinates")
	}
}

// TestParseOneProvider verifies single provider entry parsing.
func TestParseOneProvider(t *testing.T) {
	entry := []any{"Booking.com", 189.0, "USD", "https://booking.com/..."}

	p := parseOneProvider(entry)

	if p.Provider != "Booking.com" {
		t.Errorf("Provider = %q, want %q", p.Provider, "Booking.com")
	}
	if p.Price != 189.0 {
		t.Errorf("Price = %v, want 189.0", p.Price)
	}
	if p.Currency != "USD" {
		t.Errorf("Currency = %q, want %q", p.Currency, "USD")
	}
}
