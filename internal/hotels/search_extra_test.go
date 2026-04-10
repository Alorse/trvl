package hotels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/jsonutil"
	"github.com/MikkoParkkola/trvl/internal/models"
)

func longCallbackPreamble() string {
	return strings.Repeat("meta:'xxxxxxxxxx',", 40)
}

// --- parsePriceString ---

func TestParsePriceString(t *testing.T) {
	tests := []struct {
		input   string
		wantAmt float64
		wantCur string
	}{
		{"PLN 420", 420, "PLN"},
		{"USD 150.50", 150.50, "USD"},
		{"EUR 89", 89, "EUR"},
		{"GBP 200", 200, "GBP"},
		{"420 PLN", 420, "PLN"},       // amount first
		{"150.50 USD", 150.50, "USD"}, // amount first
		{"1,234 EUR", 1234, "EUR"},    // comma in number
		{"", 0, ""},                   // empty
		{"single", 0, ""},             // single token
		{"notnum notcur", 0, ""},      // neither is a number
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			amt, cur := parsePriceString(tt.input)
			if amt != tt.wantAmt {
				t.Errorf("amount = %v, want %v", amt, tt.wantAmt)
			}
			if cur != tt.wantCur {
				t.Errorf("currency = %q, want %q", cur, tt.wantCur)
			}
		})
	}
}

func TestParsePriceString_InvalidCurrency(t *testing.T) {
	// "usd" is lowercase — not a valid 3-letter uppercase code.
	amt, cur := parsePriceString("100 usd")
	if amt != 100 {
		t.Errorf("amount = %v, want 100", amt)
	}
	if cur != "" {
		t.Errorf("currency = %q, want empty (lowercase not valid)", cur)
	}
}

func TestParsePriceString_SymbolPrefix(t *testing.T) {
	// "$123" — single token, not enough parts.
	amt, _ := parsePriceString("$123")
	if amt != 0 {
		t.Errorf("amount = %v, want 0 for single-token price", amt)
	}
}

// --- deduplicateHotels ---

func TestDeduplicateHotels(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Hotel A", Price: 100},
		{Name: "Hotel B", Price: 200},
		{Name: "hotel a", Price: 150}, // duplicate (case-insensitive)
		{Name: "Hotel C", Price: 300},
		{Name: "HOTEL B", Price: 250}, // duplicate
	}

	result := deduplicateHotels(hotels)
	if len(result) != 3 {
		t.Fatalf("expected 3 hotels after dedup, got %d", len(result))
	}

	// First occurrence should be kept (100, not 150).
	if result[0].Price != 100 {
		t.Errorf("first hotel price = %v, want 100", result[0].Price)
	}
}

func TestDeduplicateHotels_Empty(t *testing.T) {
	result := deduplicateHotels(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

// --- navigateArray ---

func TestNavigateArray(t *testing.T) {
	data := []any{
		[]any{
			[]any{
				"deep value",
			},
		},
	}

	result := jsonutil.NavigateArray(data, 0, 0, 0)
	if result != "deep value" {
		t.Errorf("got %v, want %q", result, "deep value")
	}
}

func TestNavigateArray_OutOfBounds(t *testing.T) {
	data := []any{[]any{"only one"}}

	result := jsonutil.NavigateArray(data, 0, 5)
	if result != nil {
		t.Errorf("expected nil for out-of-bounds, got %v", result)
	}
}

func TestNavigateArray_NotArray(t *testing.T) {
	result := jsonutil.NavigateArray("not an array", 0)
	if result != nil {
		t.Errorf("expected nil for non-array, got %v", result)
	}
}

func TestNavigateArray_NoIndices(t *testing.T) {
	data := []any{1, 2, 3}
	result := jsonutil.NavigateArray(data)
	// With no indices, should return the value itself.
	arr, ok := result.([]any)
	if !ok || len(arr) != 3 {
		t.Errorf("expected original array back, got %v", result)
	}
}

// --- safeString ---

func TestSafeString(t *testing.T) {
	if jsonutil.StringValue("hello") != "hello" {
		t.Error("expected 'hello'")
	}
	if jsonutil.StringValue(nil) != "" {
		t.Error("expected empty for nil")
	}
	if jsonutil.StringValue(42) != "" {
		t.Error("expected empty for int")
	}
	if jsonutil.StringValue(3.14) != "" {
		t.Error("expected empty for float")
	}
}

// --- toFloat64 ---

func TestToFloat64(t *testing.T) {
	f, ok := jsonutil.ToFloat(float64(42.5))
	if !ok || f != 42.5 {
		t.Errorf("toFloat64(42.5) = (%v, %v)", f, ok)
	}

	f, ok = jsonutil.ToFloat(json.Number("99.9"))
	if !ok || f != 99.9 {
		t.Errorf("toFloat64(json.Number 99.9) = (%v, %v)", f, ok)
	}

	_, ok = jsonutil.ToFloat(nil)
	if ok {
		t.Error("expected false for nil")
	}

	_, ok = jsonutil.ToFloat("string")
	if ok {
		t.Error("expected false for string")
	}
}

// --- buildTravelURL ---

func TestBuildTravelURL(t *testing.T) {
	opts := HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Guests:   2,
		Currency: "USD",
	}

	url := buildTravelURL("Helsinki", opts)

	if !strings.Contains(url, "google.com/travel/hotels/") {
		t.Errorf("URL missing google.com base: %s", url)
	}
	if !strings.Contains(url, "Helsinki") {
		t.Errorf("URL missing location: %s", url)
	}
	if !strings.Contains(url, "dates=2026-06-15") {
		t.Errorf("URL missing dates: %s", url)
	}
	if !strings.Contains(url, "currency=USD") {
		t.Errorf("URL missing currency: %s", url)
	}
	if !strings.Contains(url, "adults=2") {
		t.Errorf("URL missing adults: %s", url)
	}
	if !strings.Contains(url, "hl=en") {
		t.Errorf("URL missing hl: %s", url)
	}
}

func TestBuildTravelURL_SpecialChars(t *testing.T) {
	url := buildTravelURL("New York City", HotelSearchOptions{
		CheckIn:  "2026-12-25",
		CheckOut: "2026-12-28",
		Guests:   3,
		Currency: "EUR",
	})

	if !strings.Contains(url, "New%20York%20City") {
		t.Errorf("URL should escape spaces: %s", url)
	}
}

// --- filterByStars ---

func TestFilterByStars_All(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "A", Stars: 5},
		{Name: "B", Stars: 4},
		{Name: "C", Stars: 3},
	}

	// Filter >= 3 should keep all.
	result := filterByStars(hotels, 3)
	if len(result) != 3 {
		t.Errorf("expected 3, got %d", len(result))
	}
}

func TestFilterByStars_None(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "A", Stars: 2},
		{Name: "B", Stars: 1},
	}

	result := filterByStars(hotels, 5)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestFilterByStars_ZeroStars(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Unknown Stars", Stars: 0},
		{Name: "Five Star", Stars: 5},
	}

	result := filterByStars(hotels, 3)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Name != "Five Star" {
		t.Errorf("expected Five Star, got %q", result[0].Name)
	}
}

// --- sortHotels ---

func TestSortHotels_EmptySlice(t *testing.T) {
	var hotels []models.HotelResult
	sortHotels(hotels, "cheapest", 0, 0) // should not panic
}

func TestSortHotels_SingleElement(t *testing.T) {
	hotels := []models.HotelResult{{Name: "Only", Price: 100}}
	sortHotels(hotels, "cheapest", 0, 0)
	if hotels[0].Name != "Only" {
		t.Errorf("single element changed")
	}
}

func TestSortHotels_UnknownSort(t *testing.T) {
	// Default sort (cheapest) should apply for unknown sort key.
	hotels := []models.HotelResult{
		{Name: "B", Price: 200},
		{Name: "A", Price: 100},
	}
	sortHotels(hotels, "unknown_sort", 0, 0)
	// "unknown_sort" doesn't match any case — no sorting happens.
	// The original order is preserved.
	if hotels[0].Name != "B" {
		t.Errorf("expected no sorting for unknown sort, but order changed")
	}
}

func TestSortHotels_PriceWithZeros(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Zero1", Price: 0},
		{Name: "Cheap", Price: 50},
		{Name: "Zero2", Price: 0},
		{Name: "Mid", Price: 150},
	}
	sortHotels(hotels, "cheapest", 0, 0)

	// Priced hotels first (ascending), zeros at end.
	if hotels[0].Name != "Cheap" {
		t.Errorf("first = %q, want Cheap", hotels[0].Name)
	}
	if hotels[1].Name != "Mid" {
		t.Errorf("second = %q, want Mid", hotels[1].Name)
	}
}

func TestSortHotels_RatingDescending(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "A", Rating: 3.0},
		{Name: "B", Rating: 4.5},
		{Name: "C", Rating: 4.0},
		{Name: "D", Rating: 5.0},
	}
	sortHotels(hotels, "rating", 0, 0)

	if hotels[0].Name != "D" {
		t.Errorf("first = %q, want D (5.0)", hotels[0].Name)
	}
	if hotels[1].Name != "B" {
		t.Errorf("second = %q, want B (4.5)", hotels[1].Name)
	}
}

// --- lessPrice ---

func TestLessPrice(t *testing.T) {
	tests := []struct {
		name string
		a, b models.HotelResult
		want bool
	}{
		{"a < b", models.HotelResult{Price: 100}, models.HotelResult{Price: 200}, true},
		{"a > b", models.HotelResult{Price: 200}, models.HotelResult{Price: 100}, false},
		{"a == b", models.HotelResult{Price: 100}, models.HotelResult{Price: 100}, false},
		{"a=0", models.HotelResult{Price: 0}, models.HotelResult{Price: 100}, false},
		{"b=0", models.HotelResult{Price: 100}, models.HotelResult{Price: 0}, true},
		{"both=0", models.HotelResult{Price: 0}, models.HotelResult{Price: 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lessPrice(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("lessPrice(%.0f, %.0f) = %v, want %v", tt.a.Price, tt.b.Price, got, tt.want)
			}
		})
	}
}

// --- extractCallbacks ---

func TestExtractCallbacks_Empty(t *testing.T) {
	results := extractCallbacks("")
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty page, got %d", len(results))
	}
}

func TestExtractCallbacks_NoCallbacks(t *testing.T) {
	results := extractCallbacks("<html><body>no callbacks here</body></html>")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestExtractCallbacks_ValidCallback(t *testing.T) {
	page := `AF_initDataCallback({key: 'ds:0', data:[1,2,3], sideChannel: {}});`
	results := extractCallbacks(page)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestExtractCallbacks_MultipleCallbacks(t *testing.T) {
	page := `AF_initDataCallback({key: 'ds:0', data:[1,2,3]});
	AF_initDataCallback({key: 'ds:1', data:[4,5,6]});`
	results := extractCallbacks(page)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestExtractCallbacks_MalformedJSON(t *testing.T) {
	page := `AF_initDataCallback({key: 'ds:0', data:{not valid json array});`
	results := extractCallbacks(page)
	// The data: starts with '{' not '[', so it should be skipped.
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-array data, got %d", len(results))
	}
}

func TestExtractCallbacks_LongCallbackPreamble(t *testing.T) {
	page := `AF_initDataCallback({key: 'ds:0', ` + longCallbackPreamble() + `data:[1,2,3], sideChannel: {}});`
	results := extractCallbacks(page)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	arr, ok := results[0].([]any)
	if !ok {
		t.Fatalf("expected parsed array result, got %T", results[0])
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 array elements, got %d", len(arr))
	}
}

func TestExtractCallbacks_DoesNotReachIntoNextCallback(t *testing.T) {
	page := `AF_initDataCallback({key: 'ds:0', sideChannel: {}});` +
		`AF_initDataCallback({key: 'ds:1', data:[4,5,6]});`
	results := extractCallbacks(page)
	if len(results) != 1 {
		t.Fatalf("expected 1 result from the second callback only, got %d", len(results))
	}
}

// --- parseOrganicHotel ---

func TestParseOrganicHotel_MinimalEntry(t *testing.T) {
	entry := make([]any, 3)
	entry[1] = "Minimal Hotel"
	h := parseOrganicHotel(entry, "USD")

	if h.Name != "Minimal Hotel" {
		t.Errorf("Name = %q", h.Name)
	}
	if h.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", h.Currency)
	}
}

func TestParseOrganicHotel_NilName(t *testing.T) {
	entry := make([]any, 3)
	entry[1] = nil
	h := parseOrganicHotel(entry, "EUR")
	if h.Name != "" {
		t.Errorf("Name = %q, want empty", h.Name)
	}
}

func TestParseOrganicHotel_WithDescription(t *testing.T) {
	entry := make([]any, 12)
	entry[1] = "Hotel With Desc"
	entry[11] = []any{"Main Street 42, Helsinki"}
	h := parseOrganicHotel(entry, "EUR")
	if h.Address != "Main Street 42, Helsinki" {
		t.Errorf("Address = %q", h.Address)
	}
}

// --- parseSponsoredHotel ---

func TestParseSponsoredHotel(t *testing.T) {
	entry := make([]any, 7)
	entry[0] = "Sponsored Hotel"
	entry[2] = "EUR 299"
	entry[4] = float64(500)
	entry[5] = float64(4.3)

	h := parseSponsoredHotel(entry, "USD")
	if h.Name != "Sponsored Hotel" {
		t.Errorf("Name = %q", h.Name)
	}
	if h.Price != 299 {
		t.Errorf("Price = %v, want 299", h.Price)
	}
	if h.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", h.Currency)
	}
	if h.ReviewCount != 500 {
		t.Errorf("ReviewCount = %d, want 500", h.ReviewCount)
	}
	if h.Rating != 4.3 {
		t.Errorf("Rating = %v, want 4.3", h.Rating)
	}
}

func TestParseSponsoredHotel_EmptyPrice(t *testing.T) {
	entry := make([]any, 7)
	entry[0] = "Hotel No Price"
	entry[2] = ""
	h := parseSponsoredHotel(entry, "USD")
	if h.Price != 0 {
		t.Errorf("Price = %v, want 0", h.Price)
	}
}

// --- extractOrganicPrice ---

func TestExtractOrganicPrice_Nil(t *testing.T) {
	price, cur := extractOrganicPrice(nil)
	if price != 0 || cur != "" {
		t.Errorf("expected (0, \"\"), got (%v, %q)", price, cur)
	}
}

func TestExtractOrganicPrice_NotArray(t *testing.T) {
	price, cur := extractOrganicPrice("not array")
	if price != 0 || cur != "" {
		t.Errorf("expected (0, \"\"), got (%v, %q)", price, cur)
	}
}

func TestExtractOrganicPrice_Valid(t *testing.T) {
	raw := []any{nil, []any{[]any{189.0, 0.0}, nil, nil, "EUR"}}
	price, cur := extractOrganicPrice(raw)
	if price != 189 {
		t.Errorf("price = %v, want 189", price)
	}
	if cur != "EUR" {
		t.Errorf("currency = %q, want EUR", cur)
	}
}

// --- looksLikeProviderEntry / looksLikePriceList ---

func TestLooksLikeProviderEntry(t *testing.T) {
	valid := []any{"Booking.com", 189.0, "USD"}
	if !looksLikeProviderEntry(valid) {
		t.Error("expected true for valid provider entry")
	}

	noName := []any{nil, 189.0}
	if looksLikeProviderEntry(noName) {
		t.Error("expected false for entry without name")
	}

	noPrice := []any{"Booking.com"}
	if looksLikeProviderEntry(noPrice) {
		t.Error("expected false for entry without price")
	}

	empty := []any{}
	if looksLikeProviderEntry(empty) {
		t.Error("expected false for empty")
	}
}

func TestLooksLikePriceList(t *testing.T) {
	list := []any{
		[]any{"Booking.com", 189.0},
		[]any{"Expedia", 195.0},
	}
	if !looksLikePriceList(list) {
		t.Error("expected true for valid price list")
	}

	empty := []any{}
	if looksLikePriceList(empty) {
		t.Error("expected false for empty list")
	}
}

// --- parseOneProvider ---

func TestParseOneProvider_WithSubArray(t *testing.T) {
	entry := []any{
		"Hotels.com",
		[]any{210.0, "EUR"},
		"https://example.com",
	}

	p := parseOneProvider(entry)
	if p.Provider != "Hotels.com" {
		t.Errorf("Provider = %q", p.Provider)
	}
	if p.Price != 210 {
		t.Errorf("Price = %v, want 210", p.Price)
	}
	if p.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", p.Currency)
	}
}

func TestParseOneProvider_SkipsURLs(t *testing.T) {
	entry := []any{
		"https://booking.com/...",
		"Booking.com",
		189.0,
	}

	p := parseOneProvider(entry)
	// The URL should be skipped; provider should be "Booking.com".
	if p.Provider != "Booking.com" {
		t.Errorf("Provider = %q, want Booking.com", p.Provider)
	}
}

// --- Geocode mock ---

func TestResolveLocation_MockServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"lat":"60.1695","lon":"24.9354","display_name":"Helsinki"}]`))
	}))
	defer ts.Close()

	// Test cache hit (primed above in another test, but let's prime fresh).
	geoCache.Lock()
	geoCache.entries["MockCity"] = geoEntry{lat: 51.5, lon: -0.12}
	geoCache.Unlock()

	lat, lon, err := ResolveLocation(context.Background(), "MockCity")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if lat != 51.5 || lon != -0.12 {
		t.Errorf("got (%v, %v), want (51.5, -0.12)", lat, lon)
	}

	// Clean up.
	geoCache.Lock()
	delete(geoCache.entries, "MockCity")
	geoCache.Unlock()
}

// --- SearchHotels validation ---

func TestSearchHotels_MissingDates(t *testing.T) {
	_, err := SearchHotels(context.Background(), "Helsinki", HotelSearchOptions{})
	if err == nil {
		t.Error("expected error for missing dates")
	}
}

func TestSearchHotels_BadCheckInDate(t *testing.T) {
	_, err := SearchHotels(context.Background(), "Helsinki", HotelSearchOptions{
		CheckIn:  "not-a-date",
		CheckOut: "2026-06-18",
	})
	if err == nil {
		t.Error("expected error for bad check-in date")
	}
}

func TestSearchHotels_BadCheckOutDate(t *testing.T) {
	_, err := SearchHotels(context.Background(), "Helsinki", HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "invalid",
	})
	if err == nil {
		t.Error("expected error for bad check-out date")
	}
}

// --- GetHotelPrices validation ---

func TestGetHotelPrices_EmptyHotelID(t *testing.T) {
	_, err := GetHotelPrices(context.Background(), "", "2026-06-15", "2026-06-18", "USD")
	if err == nil {
		t.Error("expected error for empty hotel ID")
	}
}

func TestGetHotelPrices_EmptyDates(t *testing.T) {
	_, err := GetHotelPrices(context.Background(), "/g/123", "", "2026-06-18", "USD")
	if err == nil {
		t.Error("expected error for empty check-in")
	}

	_, err = GetHotelPrices(context.Background(), "/g/123", "2026-06-15", "", "USD")
	if err == nil {
		t.Error("expected error for empty check-out")
	}
}

func TestGetHotelPrices_BadDate(t *testing.T) {
	_, err := GetHotelPrices(context.Background(), "/g/123", "bad", "2026-06-18", "USD")
	if err == nil {
		t.Error("expected error for bad check-in date")
	}

	_, err = GetHotelPrices(context.Background(), "/g/123", "2026-06-15", "bad", "USD")
	if err == nil {
		t.Error("expected error for bad check-out date")
	}
}

func TestGetHotelPrices_DefaultCurrency(t *testing.T) {
	// Can't easily test the full flow without a real server,
	// but verify it doesn't panic with empty currency.
	// The function will fail at the HTTP request level, which is fine.
	_, err := GetHotelPrices(context.Background(), "/g/123", "2026-06-15", "2026-06-18", "")
	// Will fail because it tries to hit google.com — that's expected.
	if err == nil {
		t.Log("Unexpectedly succeeded (maybe network is available)")
	}
}

// --- parseHotelsFromPage ---

func TestParseHotelsFromPage_NoCallbacks(t *testing.T) {
	_, err := parseHotelsFromPage("<html><body>empty</body></html>", "USD")
	if err == nil {
		t.Error("expected error for page with no callbacks")
	}
}

func TestParseHotelsFromPage_CallbackNoHotels(t *testing.T) {
	page := `AF_initDataCallback({key: 'ds:0', data:[1,2,3]});`
	_, err := parseHotelsFromPage(page, "USD")
	if err == nil {
		t.Error("expected error for page with no hotel data")
	}
}

// --- ParseHotelSearchResponse ---

func TestParseHotelSearchResponse_EmptyEntries(t *testing.T) {
	_, err := ParseHotelSearchResponse([]any{}, "USD")
	if err == nil {
		t.Error("expected error for empty entries")
	}
}

func TestParseHotelSearchResponse_InvalidJSON(t *testing.T) {
	entries := []any{
		[]any{
			[]any{"wrb.fr", "AtySUc", "not valid json", nil},
		},
	}

	_, err := ParseHotelSearchResponse(entries, "USD")
	if err == nil {
		t.Error("expected error for invalid JSON in payload")
	}
}

// --- ParseHotelPriceResponse ---

func TestParseHotelPriceResponse_EmptyEntries(t *testing.T) {
	_, err := ParseHotelPriceResponse([]any{})
	if err == nil {
		t.Error("expected error for empty entries")
	}
}

func TestParseHotelPriceResponse_NoPrices(t *testing.T) {
	// Valid batch response but no price-like entries.
	inner := `[null, "no prices here"]`
	entries := []any{
		[]any{
			[]any{"wrb.fr", "yY52ce", inner, nil},
		},
	}

	_, err := ParseHotelPriceResponse(entries)
	if err == nil {
		t.Error("expected error for response with no prices")
	}
}

// --- extractBatchPayload edge cases ---

func TestExtractBatchPayload_DirectEntries(t *testing.T) {
	// Entries where the batch array is directly at the entry level.
	entries := []any{
		[]any{"wrb.fr", "TestRPC", `[1,2,3]`, nil},
	}

	payload, err := extractBatchPayload(entries, "TestRPC")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	arr, ok := payload.([]any)
	if !ok {
		t.Fatalf("payload not array, got %T", payload)
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr))
	}
}

// --- findHotelEntries edge cases ---

func TestFindHotelEntries_DeepNesting(t *testing.T) {
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "Deep Hotel"
	hotel[2] = []any{[]any{60.168, 24.941}}

	// Wrap hotel in many layers.
	var data any = hotel
	for range 5 {
		data = []any{data}
	}

	found := findHotelEntries(data, 0)
	if len(found) != 1 {
		t.Errorf("expected 1 hotel in deep nesting, got %d", len(found))
	}
}

func TestFindHotelEntries_MaxDepth(t *testing.T) {
	// Create nesting deeper than max depth (10).
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "Too Deep Hotel"
	hotel[2] = []any{[]any{60.168, 24.941}}

	var data any = hotel
	for range 12 {
		data = []any{data}
	}

	found := findHotelEntries(data, 0)
	if len(found) != 0 {
		t.Errorf("expected 0 hotels beyond max depth, got %d", len(found))
	}
}

func TestFindHotelEntries_MapValue(t *testing.T) {
	hotel := make([]any, 12)
	hotel[0] = nil
	hotel[1] = "Map Hotel"
	hotel[2] = []any{[]any{60.168, 24.941}}

	data := map[string]any{
		"hotels": []any{hotel},
	}

	found := findHotelEntries(data, 0)
	if len(found) != 1 {
		t.Errorf("expected 1 hotel in map, got %d", len(found))
	}
}

// --- parseDateArray ---

func TestParseDateArray_EdgeCases(t *testing.T) {
	// Valid edge dates.
	got, err := parseDateArray("2000-01-01")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != [3]int{2000, 1, 1} {
		t.Errorf("got %v", got)
	}

	got, err = parseDateArray("2099-12-31")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != [3]int{2099, 12, 31} {
		t.Errorf("got %v", got)
	}
}

func TestParseDateArray_InvalidDate(t *testing.T) {
	_, err := parseDateArray("not-a-date")
	if err == nil {
		t.Error("expected error for invalid date")
	}
}

// --- buildHotelBookingURL ---

func TestBuildHotelBookingURL_Basic(t *testing.T) {
	url := buildHotelBookingURL("Helsinki", "2026-06-15", "2026-06-18")
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	if !strings.Contains(url, "google.com/travel/hotels") {
		t.Errorf("URL missing google.com/travel/hotels: %s", url)
	}
	if !strings.Contains(url, "Helsinki") {
		t.Errorf("URL missing location Helsinki: %s", url)
	}
	if !strings.Contains(url, "2026-06-15") {
		t.Errorf("URL missing check-in date: %s", url)
	}
	if !strings.Contains(url, "2026-06-18") {
		t.Errorf("URL missing check-out date: %s", url)
	}
}

func TestBuildHotelBookingURL_Format(t *testing.T) {
	url := buildHotelBookingURL("Tokyo", "2026-07-01", "2026-07-05")
	if !strings.Contains(url, "dates=2026-07-01,2026-07-05") {
		t.Errorf("URL date format incorrect: %s", url)
	}
}

func TestBuildHotelBookingURL_SpecialChars(t *testing.T) {
	url := buildHotelBookingURL("New York City", "2026-12-25", "2026-12-28")
	// URL should contain escaped location.
	if !strings.Contains(url, "New") {
		t.Errorf("URL missing location parts: %s", url)
	}
	// Path-escaped and query-escaped should both be present.
	if !strings.Contains(url, "hotels/") {
		t.Errorf("URL missing hotels path: %s", url)
	}
}

func TestBuildHotelBookingURL_DifferentLocations(t *testing.T) {
	tests := []struct {
		location, checkIn, checkOut string
	}{
		{"Barcelona", "2026-08-01", "2026-08-05"},
		{"London", "2026-09-10", "2026-09-15"},
		{"Singapore", "2027-01-01", "2027-01-03"},
	}
	for _, tt := range tests {
		url := buildHotelBookingURL(tt.location, tt.checkIn, tt.checkOut)
		if !strings.Contains(url, tt.location) {
			t.Errorf("URL for %s missing location: %s", tt.location, url)
		}
		if !strings.Contains(url, tt.checkIn) {
			t.Errorf("URL for %s missing check-in: %s", tt.location, url)
		}
		if !strings.Contains(url, tt.checkOut) {
			t.Errorf("URL for %s missing check-out: %s", tt.location, url)
		}
	}
}

// --- SearchHotels defaults ---

func TestSearchHotels_DefaultGuests(t *testing.T) {
	// Verify defaults by calling SearchHotels with 0 guests.
	// It will fail at the HTTP layer, but we can confirm defaults don't panic.
	_, err := SearchHotels(context.Background(), "Helsinki", HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Guests:   0, // should default to 2
	})
	// Will fail because it tries to contact google.com — expected.
	if err == nil {
		t.Log("Unexpectedly succeeded")
	}
}

func TestSearchHotels_DefaultCurrency(t *testing.T) {
	_, err := SearchHotels(context.Background(), "Helsinki", HotelSearchOptions{
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Currency: "", // should default to "USD"
	})
	if err == nil {
		t.Log("Unexpectedly succeeded")
	}
}

// --- Haversine ---

func TestHaversine_SamePoint(t *testing.T) {
	d := Haversine(60.17, 24.94, 60.17, 24.94)
	if d != 0 {
		t.Errorf("same point distance = %v, want 0", d)
	}
}

func TestHaversine_HelsinkiToTallinn(t *testing.T) {
	// Helsinki (60.17, 24.94) to Tallinn (59.44, 24.75) ~80 km
	d := Haversine(60.17, 24.94, 59.44, 24.75)
	if d < 70 || d > 90 {
		t.Errorf("Helsinki-Tallinn distance = %.1f km, expected ~80 km", d)
	}
}

func TestHaversine_HelsinkiToTokyo(t *testing.T) {
	// Helsinki (60.17, 24.94) to Tokyo (35.68, 139.69) ~7800 km
	d := Haversine(60.17, 24.94, 35.68, 139.69)
	if d < 7500 || d > 8200 {
		t.Errorf("Helsinki-Tokyo distance = %.0f km, expected ~7800 km", d)
	}
}

func TestHaversine_Antipodal(t *testing.T) {
	// North pole to south pole ~20000 km
	d := Haversine(90, 0, -90, 0)
	if d < 19900 || d > 20100 {
		t.Errorf("pole-to-pole distance = %.0f km, expected ~20015 km", d)
	}
}

// --- filterHotels ---

func TestFilterHotels_NoFilters(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "A", Price: 100, Rating: 3.5, Stars: 3},
		{Name: "B", Price: 200, Rating: 4.5, Stars: 4},
	}
	result := filterHotels(hotels, HotelSearchOptions{})
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestFilterHotels_MinPrice(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Cheap", Price: 50},
		{Name: "Mid", Price: 150},
		{Name: "Pricey", Price: 300},
		{Name: "No Price", Price: 0}, // price=0 should NOT be filtered out
	}
	result := filterHotels(hotels, HotelSearchOptions{MinPrice: 100})
	if len(result) != 3 {
		t.Errorf("expected 3 (Mid + Pricey + No Price), got %d", len(result))
	}
	for _, h := range result {
		if h.Name == "Cheap" {
			t.Error("Cheap should be filtered out by MinPrice=100")
		}
	}
}

func TestFilterHotels_MaxPrice(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Cheap", Price: 50},
		{Name: "Mid", Price: 150},
		{Name: "Pricey", Price: 300},
		{Name: "No Price", Price: 0},
	}
	result := filterHotels(hotels, HotelSearchOptions{MaxPrice: 200})
	if len(result) != 3 {
		t.Errorf("expected 3 (Cheap + Mid + No Price), got %d", len(result))
	}
	for _, h := range result {
		if h.Name == "Pricey" {
			t.Error("Pricey should be filtered out by MaxPrice=200")
		}
	}
}

func TestFilterHotels_MinRating(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Low", Rating: 3.0},
		{Name: "Mid", Rating: 4.0},
		{Name: "High", Rating: 4.8},
		{Name: "No Rating", Rating: 0}, // SHOULD be filtered out — unrated
	}
	result := filterHotels(hotels, HotelSearchOptions{MinRating: 4.0})
	// Unrated properties are now excluded when MinRating is set. They are
	// typically private rooms, new listings, or apartment units without
	// enough guest reviews to establish quality — exactly what a serious
	// traveler does NOT want when asking for "at least 4 stars".
	if len(result) != 2 {
		t.Errorf("expected 2 (Mid + High), got %d", len(result))
	}
	for _, h := range result {
		if h.Name == "Low" {
			t.Error("Low should be filtered out by MinRating=4.0")
		}
		if h.Name == "No Rating" {
			t.Error("No Rating should be filtered out by MinRating=4.0")
		}
	}
}

func TestFilterHotels_Stars(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Two", Stars: 2},
		{Name: "Four", Stars: 4},
		{Name: "Five", Stars: 5},
	}
	result := filterHotels(hotels, HotelSearchOptions{Stars: 4})
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestFilterHotels_MaxDistance(t *testing.T) {
	// Helsinki center: 60.17, 24.94
	hotels := []models.HotelResult{
		{Name: "Close", Lat: 60.17, Lon: 24.94},  // ~0 km
		{Name: "Medium", Lat: 60.20, Lon: 24.94}, // ~3.3 km
		{Name: "Far", Lat: 60.50, Lon: 24.94},    // ~36.7 km
		{Name: "No Coords", Lat: 0, Lon: 0},      // no coords, should NOT be filtered
	}
	result := filterHotels(hotels, HotelSearchOptions{
		MaxDistanceKm: 5,
		CenterLat:     60.17,
		CenterLon:     24.94,
	})
	if len(result) != 3 {
		t.Errorf("expected 3 (Close + Medium + No Coords), got %d", len(result))
	}
	for _, h := range result {
		if h.Name == "Far" {
			t.Error("Far should be filtered out by MaxDistanceKm=5")
		}
	}
}

func TestFilterHotels_MaxDistanceNoCenterCoords(t *testing.T) {
	// If center coords are 0, distance filter should not remove anything.
	hotels := []models.HotelResult{
		{Name: "A", Lat: 60.17, Lon: 24.94},
		{Name: "B", Lat: 35.68, Lon: 139.69},
	}
	result := filterHotels(hotels, HotelSearchOptions{
		MaxDistanceKm: 1,
		CenterLat:     0,
		CenterLon:     0,
	})
	if len(result) != 2 {
		t.Errorf("expected 2 (no filtering without center), got %d", len(result))
	}
}

func TestFilterHotels_Combined(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Perfect", Price: 150, Rating: 4.5, Stars: 4, Lat: 60.17, Lon: 24.94},
		{Name: "Cheap Bad", Price: 50, Rating: 2.0, Stars: 2, Lat: 60.17, Lon: 24.94},
		{Name: "Expensive", Price: 500, Rating: 4.8, Stars: 5, Lat: 60.17, Lon: 24.94},
		{Name: "Far Good", Price: 120, Rating: 4.2, Stars: 4, Lat: 60.50, Lon: 24.94},
	}
	result := filterHotels(hotels, HotelSearchOptions{
		Stars:         3,
		MinPrice:      80,
		MaxPrice:      300,
		MinRating:     4.0,
		MaxDistanceKm: 5,
		CenterLat:     60.17,
		CenterLon:     24.94,
	})
	if len(result) != 1 {
		t.Errorf("expected 1 (Perfect only), got %d", len(result))
		for _, h := range result {
			t.Logf("  %s: price=%.0f rating=%.1f stars=%d", h.Name, h.Price, h.Rating, h.Stars)
		}
	}
	if len(result) > 0 && result[0].Name != "Perfect" {
		t.Errorf("expected Perfect, got %q", result[0].Name)
	}
}

// --- sortHotels stars and distance ---

func TestSortHotels_Stars(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Two", Stars: 2},
		{Name: "Five", Stars: 5},
		{Name: "Three", Stars: 3},
		{Name: "Four", Stars: 4},
	}
	sortHotels(hotels, "stars", 0, 0)
	if hotels[0].Name != "Five" {
		t.Errorf("first = %q, want Five", hotels[0].Name)
	}
	if hotels[1].Name != "Four" {
		t.Errorf("second = %q, want Four", hotels[1].Name)
	}
}

func TestSortHotels_Distance(t *testing.T) {
	// Helsinki center: 60.17, 24.94
	hotels := []models.HotelResult{
		{Name: "Far", Lat: 60.50, Lon: 24.94},
		{Name: "Close", Lat: 60.17, Lon: 24.94},
		{Name: "Medium", Lat: 60.20, Lon: 24.94},
	}
	sortHotels(hotels, "distance", 60.17, 24.94)
	if hotels[0].Name != "Close" {
		t.Errorf("first = %q, want Close", hotels[0].Name)
	}
	if hotels[1].Name != "Medium" {
		t.Errorf("second = %q, want Medium", hotels[1].Name)
	}
	if hotels[2].Name != "Far" {
		t.Errorf("third = %q, want Far", hotels[2].Name)
	}
}

func TestSortHotels_DistanceNoCenter(t *testing.T) {
	// With center=(0,0), distance sort should still not panic.
	hotels := []models.HotelResult{
		{Name: "A", Lat: 60.17, Lon: 24.94},
		{Name: "B", Lat: 35.68, Lon: 139.69},
	}
	sortHotels(hotels, "distance", 0, 0)
	// No crash is the success condition.
	if len(hotels) != 2 {
		t.Errorf("expected 2 hotels, got %d", len(hotels))
	}
}
