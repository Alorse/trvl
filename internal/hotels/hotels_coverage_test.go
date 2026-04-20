package hotels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ---------------------------------------------------------------------------
// fetchBookingPage via httptest
// ---------------------------------------------------------------------------

func TestFetchBookingPage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html>mock booking page</html>")
	}))
	defer srv.Close()

	body, err := fetchBookingPage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "mock booking page") {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestFetchBookingPage_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := fetchBookingPage(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "status 403") {
		t.Errorf("expected status 403 error, got: %v", err)
	}
}

func TestFetchBookingPage_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	_, err := fetchBookingPage(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ---------------------------------------------------------------------------
// FetchBookingRooms via httptest
// ---------------------------------------------------------------------------

func TestCov_FetchBookingRooms_EmptyURL(t *testing.T) {
	_, err := FetchBookingRooms(context.Background(), "", "2026-07-01", "2026-07-05", "EUR")
	if err == nil || !strings.Contains(err.Error(), "booking URL is required") {
		t.Errorf("expected 'booking URL is required', got: %v", err)
	}
}

func TestFetchBookingRooms_WithJSONLD(t *testing.T) {
	hotelData := map[string]any{
		"@type": "Hotel",
		"makesOffer": []any{
			map[string]any{
				"name":        "Superior Double Room with Sea View",
				"description": "Spacious 35 m\u00b2 room, sleeps 2 adults, with balcony and minibar",
				"priceSpecification": map[string]any{
					"price":         "189.50",
					"priceCurrency": "EUR",
				},
			},
			map[string]any{
				"name":        "Standard Single Room",
				"description": "Cozy 18 sqm room with free wifi",
				"priceSpecification": map[string]any{
					"price":         99.0,
					"priceCurrency": "EUR",
				},
			},
		},
	}
	jsonLD, _ := json.Marshal(hotelData)
	page := fmt.Sprintf("<html><head>\n"+
		`<script type="application/ld+json">%s</script>`+
		"\n</head><body>hotel page</body></html>", string(jsonLD))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	rooms, err := FetchBookingRooms(context.Background(), srv.URL, "2026-07-01", "2026-07-05", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(rooms))
	}
	r := rooms[0]
	if r.Name != "Superior Double Room with Sea View" {
		t.Errorf("Name = %q", r.Name)
	}
	if r.Price != 189.50 {
		t.Errorf("Price = %v, want 189.50", r.Price)
	}
	if r.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", r.Currency)
	}
	if r.Provider != "Booking.com" {
		t.Errorf("Provider = %q, want Booking.com", r.Provider)
	}
	if r.SizeM2 != 35 {
		t.Errorf("SizeM2 = %v, want 35", r.SizeM2)
	}
	if r.MaxGuests != 2 {
		t.Errorf("MaxGuests = %d, want 2", r.MaxGuests)
	}
}

func TestFetchBookingRooms_FallsBackToSSR(t *testing.T) {
	page := "<html><body>\n" +
		`{"room_name":"Deluxe King Room with Balcony","price_breakdown":{"gross_amount":{"value":250.00}}}` + "\n" +
		`{"room_name":"Economy Twin Room","price_breakdown":{"gross_amount":{"value":120.00}}}` + "\n" +
		"</body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	rooms, err := FetchBookingRooms(context.Background(), srv.URL, "2026-07-01", "2026-07-05", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) < 2 {
		t.Fatalf("expected at least 2 rooms from SSR, got %d", len(rooms))
	}
	if rooms[0].Name != "Deluxe King Room with Balcony" {
		t.Errorf("first room Name = %q", rooms[0].Name)
	}
}

func TestFetchBookingRooms_NoOffers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body>empty page</body></html>")
	}))
	defer srv.Close()
	_, err := FetchBookingRooms(context.Background(), srv.URL, "2026-07-01", "2026-07-05", "USD")
	if err == nil || !strings.Contains(err.Error(), "no room offers") {
		t.Errorf("expected 'no room offers' error, got: %v", err)
	}
}

func TestFetchBookingRooms_FetchError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	_, err := FetchBookingRooms(ctx, srv.URL, "2026-07-01", "2026-07-05", "USD")
	if err == nil || !strings.Contains(err.Error(), "fetch booking detail page") {
		t.Errorf("expected fetch error, got: %v", err)
	}
}

func TestFetchBookingRooms_CurrencyFallback(t *testing.T) {
	hotelData := map[string]any{
		"@type":      "Hotel",
		"makesOffer": []any{map[string]any{"name": "Basic Room", "price": 50.0}},
	}
	jsonLD, _ := json.Marshal(hotelData)
	page := fmt.Sprintf("<html><head>\n"+
		`<script type="application/ld+json">%s</script>`+
		"\n</head></html>", string(jsonLD))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	rooms, err := FetchBookingRooms(context.Background(), srv.URL, "2026-07-01", "2026-07-05", "GBP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 1 || rooms[0].Currency != "GBP" {
		t.Errorf("expected GBP fallback, got %+v", rooms)
	}
}

// ---------------------------------------------------------------------------
// parseBookingApolloRooms / extractRoomNamesFromSSR
// ---------------------------------------------------------------------------

func TestParseBookingApolloRooms_WithRooms(t *testing.T) {
	page := `blah "room_name":"Deluxe Suite" blah "room_name":"Standard Room with Garden View" blah
"price_breakdown":{"gross_amount":{"value":300.00}} more
"price_breakdown":{"gross_amount":{"value":150.00}}`
	offers := parseBookingApolloRooms(page)
	if len(offers) < 2 {
		t.Fatalf("expected at least 2 offers, got %d", len(offers))
	}
	if offers[0].Price != 300.00 {
		t.Errorf("first offer Price = %v, want 300", offers[0].Price)
	}
}

func TestParseBookingApolloRooms_Empty(t *testing.T) {
	if offers := parseBookingApolloRooms("<html>no room data</html>"); offers != nil {
		t.Errorf("expected nil, got %d offers", len(offers))
	}
}

func TestExtractRoomNamesFromSSR_Dedup(t *testing.T) {
	offers := extractRoomNamesFromSSR(`"room_name":"Deluxe Room" and "room_name":"Deluxe Room" again`)
	if len(offers) != 1 {
		t.Errorf("expected 1 after dedup, got %d", len(offers))
	}
}

func TestExtractRoomNamesFromSSR_NoPrices(t *testing.T) {
	offers := extractRoomNamesFromSSR(`"room_name":"King Suite with View" and "room_name":"Twin Room"`)
	if len(offers) != 2 {
		t.Fatalf("expected 2, got %d", len(offers))
	}
	for _, o := range offers {
		if o.Price != 0 {
			t.Errorf("expected 0 price for %q, got %v", o.Name, o.Price)
		}
	}
}

// ---------------------------------------------------------------------------
// ldFloat type branches
// ---------------------------------------------------------------------------

func TestLdFloat_Branches(t *testing.T) {
	cases := []struct {
		name string
		obj  map[string]any
		key  string
		want float64
	}{
		{"int", map[string]any{"v": 42}, "v", 42},
		{"json_number", map[string]any{"v": json.Number("3.14")}, "v", 3.14},
		{"string", map[string]any{"v": "  99.99 "}, "v", 99.99},
		{"invalid_string", map[string]any{"v": "nope"}, "v", 0},
		{"missing_key", map[string]any{"other": 1.0}, "v", 0},
		{"bool_type", map[string]any{"v": true}, "v", 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ldFloat(tt.obj, tt.key); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseBookingJSONLD edge cases
// ---------------------------------------------------------------------------

func TestParseBookingJSONLD_ArrayWrapper(t *testing.T) {
	hotelData := []map[string]any{{
		"@type":      "Hotel",
		"makesOffer": []any{map[string]any{"name": "Junior Suite", "price": 220.0}},
	}}
	jsonLD, _ := json.Marshal(hotelData)
	page := fmt.Sprintf(`<script type="application/ld+json">%s</script>`, string(jsonLD))
	offers, err := parseBookingJSONLD(page)
	if err != nil || len(offers) != 1 || offers[0].Name != "Junior Suite" {
		t.Errorf("err=%v offers=%+v", err, offers)
	}
}

func TestCov_ParseBookingJSONLD_GraphArray(t *testing.T) {
	data := map[string]any{
		"@graph": []any{map[string]any{
			"@type":      "Hotel",
			"makesOffer": []any{map[string]any{"name": "Penthouse Suite", "price": 500.0}},
		}},
	}
	jsonLD, _ := json.Marshal(data)
	page := fmt.Sprintf(`<script type="application/ld+json">%s</script>`, string(jsonLD))
	offers, err := parseBookingJSONLD(page)
	if err != nil || len(offers) != 1 {
		t.Errorf("err=%v len=%d", err, len(offers))
	}
}

func TestParseBookingJSONLD_NoBlocks(t *testing.T) {
	if _, err := parseBookingJSONLD("<html>no json-ld</html>"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCov_ParseBookingJSONLD_NoOffers(t *testing.T) {
	data := map[string]any{"@type": "Organization", "name": "SomeOrg"}
	jsonLD, _ := json.Marshal(data)
	if _, err := parseBookingJSONLD(fmt.Sprintf(`<script type="application/ld+json">%s</script>`, string(jsonLD))); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseBookingJSONLD_InvalidJSON(t *testing.T) {
	if _, err := parseBookingJSONLD(`<script type="application/ld+json">{bad json</script>`); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// parseOfferObject branches
// ---------------------------------------------------------------------------

func TestParseOfferObject_PriceCurrencyFallback(t *testing.T) {
	room := parseOfferObject(map[string]any{"name": "Budget Room", "price": 75.0, "priceCurrency": "CHF"})
	if room.Price != 75.0 || room.Currency != "CHF" {
		t.Errorf("Price=%v Currency=%q", room.Price, room.Currency)
	}
}

func TestParseOfferObject_BedTypeFromName(t *testing.T) {
	if room := parseOfferObject(map[string]any{"name": "King Suite"}); room.BedType != "1 king bed" {
		t.Errorf("BedType = %q", room.BedType)
	}
}

func TestParseOfferObject_PriceSpecCurrencyKey(t *testing.T) {
	offer := map[string]any{
		"name":               "Test Room",
		"priceSpecification": map[string]any{"price": 100.0, "currency": "SEK"},
	}
	if room := parseOfferObject(offer); room.Currency != "SEK" {
		t.Errorf("Currency = %q", room.Currency)
	}
}

// ---------------------------------------------------------------------------
// extractMakesOffer edge cases
// ---------------------------------------------------------------------------

func TestExtractMakesOffer_SingleObject(t *testing.T) {
	hotel := map[string]any{"@type": "Hotel", "makesOffer": map[string]any{"name": "Solo Room", "price": 80.0}}
	if offers := extractMakesOffer(hotel); len(offers) != 1 {
		t.Fatalf("expected 1, got %d", len(offers))
	}
}

func TestExtractMakesOffer_NoMakesOffer(t *testing.T) {
	if offers := extractMakesOffer(map[string]any{"@type": "Hotel"}); offers != nil {
		t.Errorf("expected nil, got %d", len(offers))
	}
}

func TestExtractMakesOffer_InvalidType(t *testing.T) {
	if offers := extractMakesOffer(map[string]any{"@type": "Hotel", "makesOffer": 42}); offers != nil {
		t.Errorf("expected nil, got %d", len(offers))
	}
}

func TestExtractMakesOffer_EmptyNameSkipped(t *testing.T) {
	hotel := map[string]any{
		"@type": "Hotel",
		"makesOffer": []any{
			map[string]any{"name": "", "price": 50.0},
			map[string]any{"name": "Valid Room", "price": 100.0},
		},
	}
	if offers := extractMakesOffer(hotel); len(offers) != 1 {
		t.Fatalf("expected 1, got %d", len(offers))
	}
}

// ---------------------------------------------------------------------------
// deduplicateOffers edge cases
// ---------------------------------------------------------------------------

func TestDeduplicateOffers_ReplacementChain(t *testing.T) {
	// First: price=0, desc="". Second: price=200, replaces (price>0, existing==0).
	// Third: desc set, replaces (desc!="" && existing.Description=="").
	offers := []bookingRoomOffer{
		{Name: "Deluxe Room", Price: 0},
		{Name: "Deluxe Room", Price: 200},
		{Name: "deluxe room", Description: "A nice room"},
	}
	result := deduplicateOffers(offers)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Description != "A nice room" {
		t.Errorf("desc = %q", result[0].Description)
	}
}

func TestDeduplicateOffers_PreferWithPriceOnly(t *testing.T) {
	offers := []bookingRoomOffer{
		{Name: "Standard Room", Price: 0},
		{Name: "standard room", Price: 150},
	}
	result := deduplicateOffers(offers)
	if len(result) != 1 || result[0].Price != 150 {
		t.Errorf("got %+v", result)
	}
}

func TestDeduplicateOffers_EmptyNameSkipped(t *testing.T) {
	offers := []bookingRoomOffer{
		{Name: "", Price: 100},
		{Name: "  ", Price: 200},
		{Name: "Real Room", Price: 300},
	}
	result := deduplicateOffers(offers)
	if len(result) != 1 || result[0].Name != "Real Room" {
		t.Errorf("got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// mergeStringSlices edge cases
// ---------------------------------------------------------------------------

func TestMergeStringSlices_BothEmpty(t *testing.T) {
	if result := mergeStringSlices(nil, nil); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestMergeStringSlices_AEmpty(t *testing.T) {
	result := mergeStringSlices(nil, []string{"a"})
	if len(result) != 1 || result[0] != "a" {
		t.Errorf("got %v", result)
	}
}

func TestMergeStringSlices_BEmpty(t *testing.T) {
	result := mergeStringSlices([]string{"a"}, nil)
	if len(result) != 1 || result[0] != "a" {
		t.Errorf("got %v", result)
	}
}

func TestMergeStringSlices_CaseInsensitiveDedup(t *testing.T) {
	result := mergeStringSlices([]string{"WiFi"}, []string{"wifi", "Pool"})
	if len(result) != 2 {
		t.Errorf("expected 2, got %d: %v", len(result), result)
	}
}

// ---------------------------------------------------------------------------
// mergeRoomTypes branches
// ---------------------------------------------------------------------------

func TestMergeRoomTypes_EnrichExisting(t *testing.T) {
	google := []RoomType{{Name: "Deluxe Room", Price: 200, Currency: "EUR"}}
	booking := []RoomType{{
		Name: "Deluxe Room", Price: 190, Currency: "EUR", Provider: "Booking.com",
		Description: "A spacious room", BedType: "1 king bed", SizeM2: 35,
		MaxGuests: 2, Amenities: []string{"WiFi", "Minibar"},
	}}
	merged := mergeRoomTypes(google, booking)
	if len(merged) != 1 {
		t.Fatalf("expected 1, got %d", len(merged))
	}
	m := merged[0]
	if m.Price != 200 {
		t.Errorf("Price = %v, want 200", m.Price)
	}
	if m.Description != "A spacious room" || m.BedType != "1 king bed" || m.SizeM2 != 35 {
		t.Errorf("enrichment failed: desc=%q bed=%q size=%v", m.Description, m.BedType, m.SizeM2)
	}
	if len(m.Amenities) == 0 {
		t.Error("Amenities not enriched")
	}
}

func TestMergeRoomTypes_BookingOnlyRooms(t *testing.T) {
	google := []RoomType{{Name: "Standard Room", Price: 100}}
	booking := []RoomType{{Name: "Penthouse Suite", Price: 500, Provider: "Booking.com"}}
	if merged := mergeRoomTypes(google, booking); len(merged) != 2 {
		t.Fatalf("expected 2, got %d", len(merged))
	}
}

func TestMergeRoomTypes_GoogleZeroPriceEnriched(t *testing.T) {
	google := []RoomType{{Name: "Test Room", Price: 0, Currency: "EUR"}}
	booking := []RoomType{{Name: "Test Room", Price: 150, Currency: "EUR", Provider: "Booking.com"}}
	merged := mergeRoomTypes(google, booking)
	if len(merged) != 1 || merged[0].Price != 150 {
		t.Errorf("got %+v", merged)
	}
}

// ---------------------------------------------------------------------------
// GetRoomAvailabilityWithOpts validation
// ---------------------------------------------------------------------------

func TestGetRoomAvailabilityWithOpts_EmptyHotelID(t *testing.T) {
	_, err := GetRoomAvailabilityWithOpts(context.Background(), RoomSearchOptions{
		CheckIn: "2026-07-01", CheckOut: "2026-07-05",
	})
	if err == nil || !strings.Contains(err.Error(), "hotel ID is required") {
		t.Errorf("got: %v", err)
	}
}

func TestGetRoomAvailabilityWithOpts_NoDates(t *testing.T) {
	_, err := GetRoomAvailabilityWithOpts(context.Background(), RoomSearchOptions{HotelID: "test-id"})
	if err == nil || !strings.Contains(err.Error(), "dates are required") {
		t.Errorf("got: %v", err)
	}
}

func TestGetRoomAvailability_Delegates(t *testing.T) {
	if _, err := GetRoomAvailability(context.Background(), "", "2026-07-01", "2026-07-05", "USD"); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// parseRoomsFromPage
// ---------------------------------------------------------------------------

func TestParseRoomsFromPage_EmptyPage(t *testing.T) {
	rooms, name := parseRoomsFromPage("", "USD")
	if rooms != nil || name != "" {
		t.Errorf("expected nil/empty for empty page")
	}
}

func TestParseRoomsFromPage_WithCallbacks(t *testing.T) {
	roomData := []any{[]any{
		[]any{"Deluxe Suite", 299.0, "EUR", "Booking.com"},
		[]any{"Standard Room", 149.0, "EUR", "Expedia"},
	}}
	jsonBytes, _ := json.Marshal(roomData)
	page := fmt.Sprintf("AF_initDataCallback({key: 'ds:1', data:%s});", string(jsonBytes))
	rooms, _ := parseRoomsFromPage(page, "EUR")
	if len(rooms) < 2 {
		t.Errorf("expected at least 2 rooms, got %d", len(rooms))
	}
}

func TestParseRoomsFromPage_WithHotelName(t *testing.T) {
	data := []any{[]any{"The Grand Hotel Budapest"}}
	jsonBytes, _ := json.Marshal(data)
	page := fmt.Sprintf("AF_initDataCallback({key: 'ds:0', data:%s});", string(jsonBytes))
	if _, name := parseRoomsFromPage(page, "USD"); name != "The Grand Hotel Budapest" {
		t.Errorf("name = %q", name)
	}
}

// ---------------------------------------------------------------------------
// extractLocationFromPage
// ---------------------------------------------------------------------------

func TestExtractLocationFromPage_WithLocation(t *testing.T) {
	data := []any{[]any{nil, "Helsinki", "0x46920b0af7b76d4f"}}
	jsonBytes, _ := json.Marshal(data)
	page := fmt.Sprintf("AF_initDataCallback({key: 'ds:0', data:%s});", string(jsonBytes))
	if loc := extractLocationFromPage(page); loc != "Helsinki" {
		t.Errorf("location = %q", loc)
	}
}

func TestExtractLocationFromPage_NoCallbacks(t *testing.T) {
	if loc := extractLocationFromPage("<html>no callbacks</html>"); loc != "" {
		t.Errorf("expected empty, got %q", loc)
	}
}

func TestExtractLocationFromPage_NoTriplet(t *testing.T) {
	data := []any{"foo", "bar", "baz"}
	jsonBytes, _ := json.Marshal(data)
	page := fmt.Sprintf("AF_initDataCallback({key: 'ds:0', data:%s});", string(jsonBytes))
	if loc := extractLocationFromPage(page); loc != "" {
		t.Errorf("expected empty, got %q", loc)
	}
}

// ---------------------------------------------------------------------------
// extractReviewsFromText
// ---------------------------------------------------------------------------

func TestExtractReviewsFromText_WithReviews(t *testing.T) {
	page := "<html>\n<script>\n" +
		`{"reviewRating":{"ratingValue":"4.5"},"reviewBody":"Great hotel with amazing views","author":{"name":"John Smith"},"datePublished":"2026-01-15"}` +
		"\n</script>\n</html>"
	reviews := extractReviewsFromText(page)
	if len(reviews) < 1 {
		t.Fatalf("expected at least 1 review, got %d", len(reviews))
	}
	if reviews[0].Rating != 4.5 {
		t.Errorf("rating = %v, want 4.5", reviews[0].Rating)
	}
}

func TestExtractReviewsFromText_NoReviews(t *testing.T) {
	if reviews := extractReviewsFromText("<html>no reviews here</html>"); len(reviews) != 0 {
		t.Errorf("expected 0, got %d", len(reviews))
	}
}

func TestExtractReviewsFromText_InvalidJSON(t *testing.T) {
	if reviews := extractReviewsFromText(`prefix "reviewRating" but {invalid json: here} end`); len(reviews) != 0 {
		t.Errorf("expected 0, got %d", len(reviews))
	}
}

func TestExtractReviewsFromText_NoBraceBeforeKeyword(t *testing.T) {
	page := strings.Repeat("x", 3000) + `"reviewRating":{"ratingValue":"3.0"}`
	_ = extractReviewsFromText(page) // must not panic
}

// ---------------------------------------------------------------------------
// extractReviewFromJSON branches
// ---------------------------------------------------------------------------

func TestExtractReviewFromJSON_StringAuthor(t *testing.T) {
	obj := map[string]any{
		"reviewBody": "Nice place", "author": "Direct Author",
		"reviewRating": map[string]any{"ratingValue": 4.0}, "datePublished": "2026-03-01",
	}
	r := extractReviewFromJSON(obj)
	if r.Author != "Direct Author" || r.Date != "2026-03-01" {
		t.Errorf("Author=%q Date=%q", r.Author, r.Date)
	}
}

func TestExtractReviewFromJSON_RatingAsString(t *testing.T) {
	obj := map[string]any{"reviewBody": "Okay", "reviewRating": map[string]any{"ratingValue": "2.5"}}
	if r := extractReviewFromJSON(obj); r.Rating != 2.5 {
		t.Errorf("Rating = %v", r.Rating)
	}
}

// ---------------------------------------------------------------------------
// findReviewEntries
// ---------------------------------------------------------------------------

func TestFindReviewEntries_MapTraversal(t *testing.T) {
	data := map[string]any{"reviews": []any{
		[]any{"Nice place to stay with beautiful scenery", 4.0, "John", "2 weeks ago"},
		[]any{"Excellent service and friendly staff here", 5.0, "Jane", "1 month ago"},
	}}
	if reviews := findReviewEntries(data, 0); len(reviews) < 1 {
		t.Errorf("expected reviews, got %d", len(reviews))
	}
}

func TestFindReviewEntries_DepthLimit(t *testing.T) {
	if reviews := findReviewEntries([]any{[]any{"Long review text here with detail", 4.0}}, 13); reviews != nil {
		t.Error("expected nil at depth limit")
	}
}

// ---------------------------------------------------------------------------
// findHotelNameInData branches
// ---------------------------------------------------------------------------

func TestFindHotelNameInData_FoundAtIndex1(t *testing.T) {
	if name := findHotelNameInData([]any{nil, "Hilton Garden Inn"}, 0); name != "Hilton Garden Inn" {
		t.Errorf("got %q", name)
	}
}

func TestFindHotelNameInData_DepthLimit(t *testing.T) {
	if name := findHotelNameInData([]any{nil, "Hotel Name"}, 7); name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}

func TestFindHotelNameInData_MapTraversal(t *testing.T) {
	if name := findHotelNameInData(map[string]any{"info": []any{nil, "Map Hotel"}}, 0); name != "Map Hotel" {
		t.Errorf("got %q", name)
	}
}

func TestFindHotelNameInData_SkipsTooShort(t *testing.T) {
	if name := findHotelNameInData([]any{nil, "ab"}, 0); name != "" {
		t.Errorf("got %q", name)
	}
}

// ---------------------------------------------------------------------------
// findSummaryData
// ---------------------------------------------------------------------------

func TestFindSummaryData_RatingBreakdown(t *testing.T) {
	var s models.ReviewSummary
	findSummaryData([]any{5.0, 10.0, 20.0, 50.0, 100.0}, &s, 0)
	if s.RatingBreakdown == nil || s.RatingBreakdown["5"] != 100 {
		t.Errorf("breakdown: %+v", s.RatingBreakdown)
	}
}

func TestFindSummaryData_AvgAndCount(t *testing.T) {
	var s models.ReviewSummary
	findSummaryData([]any{4.2, 500.0}, &s, 0)
	if s.AverageRating != 4.2 || s.TotalReviews != 500 {
		t.Errorf("avg=%v total=%d", s.AverageRating, s.TotalReviews)
	}
}

func TestFindSummaryData_MapTraversal(t *testing.T) {
	var s models.ReviewSummary
	findSummaryData(map[string]any{"summary": []any{3.8, 200.0}}, &s, 0)
	if s.AverageRating != 3.8 {
		t.Errorf("avg=%v", s.AverageRating)
	}
}

// ---------------------------------------------------------------------------
// isHotelType
// ---------------------------------------------------------------------------

func TestIsHotelType_AllTypes(t *testing.T) {
	for _, typ := range []string{"Hotel", "LodgingBusiness", "Motel", "Hostel",
		"Resort", "BedAndBreakfast", "CampingPitch", "Apartment"} {
		if !isHotelType(map[string]any{"@type": typ}) {
			t.Errorf("isHotelType(%q) = false", typ)
		}
	}
}

func TestIsHotelType_NonHotel(t *testing.T) {
	for _, typ := range []string{"Organization", "Restaurant", ""} {
		if isHotelType(map[string]any{"@type": typ}) {
			t.Errorf("isHotelType(%q) = true", typ)
		}
	}
}

// ---------------------------------------------------------------------------
// Validation early-return paths
// ---------------------------------------------------------------------------

func TestCov_GetHotelReviews_EmptyID(t *testing.T) {
	_, err := GetHotelReviews(context.Background(), "", ReviewOptions{})
	if err == nil || !strings.Contains(err.Error(), "hotel ID is required") {
		t.Errorf("got: %v", err)
	}
}

func TestGetHotelPrices_EmptyID(t *testing.T) {
	if _, err := GetHotelPrices(context.Background(), "", "2026-07-01", "2026-07-05", "USD"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetHotelPrices_NoDates(t *testing.T) {
	if _, err := GetHotelPrices(context.Background(), "test-id", "", "", "USD"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchHotelAmenities_EmptyID(t *testing.T) {
	if _, err := FetchHotelAmenities(context.Background(), ""); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// parseReviewsFromPage edge cases
// ---------------------------------------------------------------------------

func TestCov_ParseReviewsFromPage_NoCallbacks(t *testing.T) {
	if _, err := parseReviewsFromPage("<html>nothing</html>", "test-id", ReviewOptions{Limit: 10, Sort: "newest"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseReviewsFromPage_WithReviewText(t *testing.T) {
	page := "AF_initDataCallback({key: 'ds:0', data:[\"no review data here\"]});\n" +
		"<script>" +
		`{"reviewRating":{"ratingValue":"4.0"},"reviewBody":"Excellent location and very clean rooms","author":{"name":"Test Author"}}` +
		"</script>"
	result, err := parseReviewsFromPage(page, "test-id", ReviewOptions{Limit: 10, Sort: "newest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Reviews) < 1 {
		t.Errorf("expected at least 1 review, got %d", len(result.Reviews))
	}
}

func TestParseReviewsFromPage_ComputesSummary(t *testing.T) {
	page := "AF_initDataCallback({key: 'ds:0', data:[\"empty\"]});\n" +
		`<script>{"reviewRating":{"ratingValue":"5.0"},"reviewBody":"Perfect stay with amazing breakfast and pool","author":{"name":"A"}}</script>` + "\n" +
		`<script>{"reviewRating":{"ratingValue":"3.0"},"reviewBody":"Average experience nothing special but ok","author":{"name":"B"}}</script>`
	result, err := parseReviewsFromPage(page, "test-id", ReviewOptions{Limit: 10, Sort: "newest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Summary.TotalReviews == 0 {
		t.Error("expected computed summary")
	}
}

func TestParseReviewsFromRawEntries_Empty(t *testing.T) {
	if _, err := parseReviewsFromRawEntries([]any{}, "test-id", ReviewOptions{Limit: 10, Sort: "newest"}); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// findRoomData map traversal
// ---------------------------------------------------------------------------

func TestFindRoomData_MapTraversal(t *testing.T) {
	data := map[string]any{"rooms": []any{
		[]any{"Deluxe Double Room", 199.0},
		[]any{"Standard Twin", 129.0},
	}}
	if rooms := findRoomData(data, "EUR", 0); len(rooms) < 2 {
		t.Errorf("expected 2+, got %d", len(rooms))
	}
}

// ---------------------------------------------------------------------------
// extractSizeM2 / extractMaxGuests
// ---------------------------------------------------------------------------

func TestExtractSizeM2_Variants(t *testing.T) {
	for _, tt := range []struct {
		text string
		want float64
	}{
		{"Room is 35 m\u00b2", 35},
		{"28m2 room", 28},
		{"40 sqm space", 40},
		{"50 sq m floor", 50},
		{"no size info", 0},
	} {
		if got := extractSizeM2(tt.text); got != tt.want {
			t.Errorf("extractSizeM2(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestExtractMaxGuests_Variants(t *testing.T) {
	for _, tt := range []struct {
		text string
		want int
	}{
		{"max 4 guests", 4},
		{"sleeps 6 adults", 6},
		{"for 2 people", 2},
		{"accommodates 3 persons", 3},
		{"up to 5 guests", 5},
		{"no guest info", 0},
		{"maximum 25 guests", 0},
	} {
		if got := extractMaxGuests(tt.text); got != tt.want {
			t.Errorf("extractMaxGuests(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestExtractRoomAmenities_Empty(t *testing.T) {
	if result := extractRoomAmenities(""); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// DefaultProvider
// ---------------------------------------------------------------------------

func TestDefaultProvider_SearchHotels_CancelledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &DefaultProvider{}
	// Cancelled context may or may not error; key is delegation works and no panic.
	_, _ = p.SearchHotels(ctx, "Paris", models.HotelSearchOptions{
		CheckIn: "2026-07-01", CheckOut: "2026-07-05", Guests: 2, Currency: "USD",
	})
}

// ---------------------------------------------------------------------------
// sortReviews
// ---------------------------------------------------------------------------

func TestCov_SortReviews_Highest(t *testing.T) {
	reviews := []models.HotelReview{{Rating: 3.0}, {Rating: 5.0}, {Rating: 1.0}}
	sortReviews(reviews, "highest")
	if reviews[0].Rating != 5.0 {
		t.Errorf("first = %v", reviews[0].Rating)
	}
}

func TestCov_SortReviews_Lowest(t *testing.T) {
	reviews := []models.HotelReview{{Rating: 3.0}, {Rating: 5.0}, {Rating: 1.0}}
	sortReviews(reviews, "lowest")
	if reviews[0].Rating != 1.0 {
		t.Errorf("first = %v", reviews[0].Rating)
	}
}

// ---------------------------------------------------------------------------
// SearchHotelsByName / SearchHotelByName validation
// ---------------------------------------------------------------------------

func TestSearchHotelsByName_EmptyName(t *testing.T) {
	if _, err := SearchHotelsByName(context.Background(), "", "Paris", "2026-07-01", "2026-07-05", "EUR"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchHotelsByName_NoDates(t *testing.T) {
	if _, err := SearchHotelsByName(context.Background(), "Hotel", "Paris", "", "", "EUR"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchHotelByName_EmptyQuery(t *testing.T) {
	if _, err := SearchHotelByName(context.Background(), "", "2026-07-01", "2026-07-05"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchHotelByName_NoDates(t *testing.T) {
	if _, err := SearchHotelByName(context.Background(), "Hotel", "", ""); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// isDateString
// ---------------------------------------------------------------------------

func TestIsDateString_AdditionalCases(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  bool
	}{
		{"2 weeks ago", true},
		{"3 months ago", true},
		{"a month ago", true},
		{"last year", true},
		{"January 2026", true},
		{"Mar 15", true},
		{"2026-01-15", true},
		{"1 day ago", true},
		{"1 hour ago", true},
		{"decent hotel room", false},
		{"not a date at all", false},
		{"", false},
		{strings.Repeat("x", 51), false},
		{"a week ago", true},
	} {
		if got := isDateString(tt.input); got != tt.want {
			t.Errorf("isDateString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// buildNameQuery
// ---------------------------------------------------------------------------

func TestBuildNameQuery_LocationAlreadyInName(t *testing.T) {
	if got := buildNameQuery("Hotel Ritz Paris", "Paris"); got != "Hotel Ritz Paris" {
		t.Errorf("got %q", got)
	}
}

func TestBuildNameQuery_AppendLocation(t *testing.T) {
	if got := buildNameQuery("Hotel Ritz", "Paris"); got != "Hotel Ritz, Paris" {
		t.Errorf("got %q", got)
	}
}

func TestBuildNameQuery_EmptyLocation(t *testing.T) {
	if got := buildNameQuery("Hotel Ritz", ""); got != "Hotel Ritz" {
		t.Errorf("got %q", got)
	}
}

// ---------------------------------------------------------------------------
// filterByNameMatch / normalizeWords
// ---------------------------------------------------------------------------

func TestFilterByNameMatch_EmptySearchName(t *testing.T) {
	hotels := []models.HotelResult{{Name: "Hotel A"}}
	if result := filterByNameMatch(hotels, "a"); len(result) != 1 {
		t.Errorf("expected 1, got %d", len(result))
	}
}

func TestNormalizeWords_StripsPunctuation(t *testing.T) {
	for _, w := range normalizeWords("Hotel's (Best) room!") {
		if strings.ContainsAny(w, "'()!") {
			t.Errorf("word %q has punctuation", w)
		}
	}
}

// ---------------------------------------------------------------------------
// propertyTypeCode
// ---------------------------------------------------------------------------

func TestPropertyTypeCode_AllTypes(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"hotel", "2"}, {"apartment", "3"}, {"hostel", "4"}, {"resort", "5"},
		{"bnb", "7"}, {"bed_and_breakfast", "7"}, {"bed and breakfast", "7"},
		{"villa", "8"}, {"", ""}, {"unrecognized", ""}, {" Hotel ", "2"},
	} {
		if got := propertyTypeCode(tt.input); got != tt.want {
			t.Errorf("propertyTypeCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// filterHotels
// ---------------------------------------------------------------------------

func TestFilterHotels_AllFilters(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Cheap", Price: 50, Stars: 2, Rating: 3.0, Lat: 60.17, Lon: 24.93, Amenities: []string{"WiFi"}},
		{Name: "Mid", Price: 150, Stars: 4, Rating: 8.0, Lat: 60.18, Lon: 24.94, Amenities: []string{"WiFi", "Pool"}},
		{Name: "Expensive", Price: 500, Stars: 5, Rating: 9.5, Lat: 60.19, Lon: 24.95, Amenities: []string{"WiFi", "Pool", "Gym"}},
		{Name: "Far Away", Price: 100, Stars: 3, Rating: 7.0, Lat: 61.50, Lon: 25.00, Amenities: []string{"WiFi"}},
	}
	opts := HotelSearchOptions{
		MinPrice: 100, MaxPrice: 300, Stars: 3, MinRating: 5.0,
		Amenities: []string{"WiFi"}, Brand: "Mid",
		CenterLat: 60.17, CenterLon: 24.93, MaxDistanceKm: 5,
	}
	result := filterHotels(hotels, opts)
	found := false
	for _, h := range result {
		if h.Name == "Mid" {
			found = true
		}
		if h.Name == "Far Away" || h.Name == "Cheap" || h.Name == "Expensive" {
			t.Errorf("%s should have been filtered", h.Name)
		}
	}
	if !found {
		t.Error("Mid should survive")
	}
}

func TestFilterHotels_ExternalProviderNoRating(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Airbnb Place", Price: 100, Rating: 0, Sources: []models.PriceSource{{Provider: "airbnb"}}},
		{Name: "Google Hotel", Price: 100, Rating: 0, Sources: []models.PriceSource{{Provider: "google_hotels"}}},
	}
	result := filterHotels(hotels, HotelSearchOptions{MinRating: 5.0})
	foundAirbnb := false
	for _, h := range result {
		if h.Name == "Airbnb Place" {
			foundAirbnb = true
		}
		if h.Name == "Google Hotel" {
			t.Error("Google Hotel without rating should be filtered")
		}
	}
	if !foundAirbnb {
		t.Error("Airbnb Place should survive (external provider)")
	}
}

// ---------------------------------------------------------------------------
// parseReviewsFromRawEntries — with actual review data
// ---------------------------------------------------------------------------

func TestParseReviewsFromRawEntries_WithReviews(t *testing.T) {
	// Build entries that contain review-like arrays.
	entries := []any{
		[]any{
			[]any{
				[]any{"This hotel was absolutely wonderful and amazing", 5.0, "Alice", "2 weeks ago"},
				[]any{"Terrible experience, would not recommend this", 1.0, "Bob", "3 days ago"},
			},
		},
	}
	result, err := parseReviewsFromRawEntries(entries, "test-id", ReviewOptions{Limit: 10, Sort: "highest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Reviews) < 1 {
		t.Errorf("expected reviews, got %d", len(result.Reviews))
	}
}

func TestParseReviewsFromRawEntries_LimitApplied(t *testing.T) {
	entries := []any{
		[]any{
			[]any{
				[]any{"Great place to stay for a family vacation", 4.0, "One", "1 week ago"},
				[]any{"Another amazing hotel right on the beach", 5.0, "Two", "2 weeks ago"},
				[]any{"Not bad but could be better with service", 3.0, "Three", "1 month ago"},
			},
		},
	}
	result, err := parseReviewsFromRawEntries(entries, "test-id", ReviewOptions{Limit: 1, Sort: "newest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Reviews) > 1 {
		t.Errorf("expected limit=1, got %d", len(result.Reviews))
	}
}

// ---------------------------------------------------------------------------
// extractTotalAvailable — more data patterns
// ---------------------------------------------------------------------------

func TestCov_ExtractTotalAvailable_Key416343588(t *testing.T) {
	// Build nested structure matching the expected path.
	data := []any{
		[]any{
			[]any{
				[]any{nil, []any{
					[]any{nil, map[string]any{
						"416343588": []any{42.0},
					}},
				}},
			},
		},
	}
	got := extractTotalAvailable(data)
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestCov_ExtractTotalAvailable_Key410579159(t *testing.T) {
	data := []any{
		[]any{
			[]any{
				[]any{nil, []any{
					[]any{nil, map[string]any{
						"410579159": []any{"cursor", "", 100.0, 1.0, 20.0},
					}},
				}},
			},
		},
	}
	got := extractTotalAvailable(data)
	if got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
}

func TestExtractTotalAvailable_NilData(t *testing.T) {
	if got := extractTotalAvailable(nil); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// tryParseOneReview — nested rating and sub-array author
// ---------------------------------------------------------------------------

func TestTryParseOneReview_NestedRating(t *testing.T) {
	arr := []any{
		"Author Name",
		"This is a really wonderful hotel with amazing views",
		[]any{4.0}, // nested rating
		"2 weeks ago",
	}
	r, ok := tryParseOneReview(arr)
	if !ok {
		t.Fatal("expected to parse review")
	}
	if r.Rating != 4.0 {
		t.Errorf("Rating = %v, want 4.0", r.Rating)
	}
}

func TestCov_TryParseOneReview_TooShort(t *testing.T) {
	arr := []any{"short", 3.0}
	_, ok := tryParseOneReview(arr)
	if ok {
		t.Error("should reject array with < 3 elements")
	}
}

func TestTryParseOneReview_NoTextNoRating(t *testing.T) {
	arr := []any{42, 43, 44}
	_, ok := tryParseOneReview(arr)
	if ok {
		t.Error("should reject array without text and rating")
	}
}

// ---------------------------------------------------------------------------
// extractHotelNameFromCallback — deeper paths
// ---------------------------------------------------------------------------

func TestExtractHotelNameFromCallback_DeepPath(t *testing.T) {
	// Hotel name at [0][0][0].
	data := []any{
		[]any{
			[]any{
				"Deep Nested Hotel Name Here",
			},
		},
	}
	name := extractHotelNameFromCallback(data)
	if name != "Deep Nested Hotel Name Here" {
		t.Errorf("got %q", name)
	}
}

func TestExtractHotelNameFromCallback_NoValidName(t *testing.T) {
	// Only short strings and numbers.
	data := []any{[]any{[]any{42}}}
	name := extractHotelNameFromCallback(data)
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}

// ---------------------------------------------------------------------------
// extractOrganicPrice — additional patterns
// ---------------------------------------------------------------------------

func TestExtractOrganicPrice_NilInner(t *testing.T) {
	got, cur := extractOrganicPrice(nil)
	if got != 0 || cur != "" {
		t.Errorf("expected (0, \"\"), got (%v, %q)", got, cur)
	}
}

func TestCov_ExtractOrganicPrice_NotArray(t *testing.T) {
	got, _ := extractOrganicPrice("not-array")
	if got != 0 {
		t.Errorf("expected 0, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// parseReviewsFromBatchResponse — with valid payload
// ---------------------------------------------------------------------------

func TestParseReviewsFromBatchResponse_FallbackToRaw(t *testing.T) {
	// Entries without "ocp93e" marker — falls back to parseReviewsFromRawEntries.
	entries := []any{
		[]any{
			[]any{
				[]any{"Really great experience staying at this hotel", 5.0, "Guest", "1 week ago"},
				[]any{"Could have been better but overall it was ok", 3.0, "Visitor", "2 months ago"},
			},
		},
	}
	result, err := parseReviewsFromBatchResponse(entries, "test-id", ReviewOptions{Limit: 10, Sort: "newest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

// ---------------------------------------------------------------------------
// parseReviewsFromPage — limit and sort branches
// ---------------------------------------------------------------------------

func TestParseReviewsFromPage_LimitApplied(t *testing.T) {
	// Multiple reviews via text extraction, limit should truncate.
	page := "AF_initDataCallback({key: 'ds:0', data:[\"empty\"]});\n" +
		`<script>{"reviewRating":{"ratingValue":"5.0"},"reviewBody":"Wonderful hotel with great amenities and pool","author":{"name":"A"}}</script>` + "\n" +
		`<script>{"reviewRating":{"ratingValue":"4.0"},"reviewBody":"Good location near the city center and shops","author":{"name":"B"}}</script>` + "\n" +
		`<script>{"reviewRating":{"ratingValue":"3.0"},"reviewBody":"Average stay nothing special about this place","author":{"name":"C"}}</script>`
	result, err := parseReviewsFromPage(page, "test-id", ReviewOptions{Limit: 1, Sort: "highest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Reviews) > 1 {
		t.Errorf("expected limit=1, got %d", len(result.Reviews))
	}
}

// ---------------------------------------------------------------------------
// findRoomData — single room entry (not list)
// ---------------------------------------------------------------------------

func TestFindRoomData_SingleEntry(t *testing.T) {
	data := []any{
		"Deluxe King Room", 299.0, "EUR", "Booking.com",
	}
	rooms := findRoomData(data, "EUR", 0)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room from single entry, got %d", len(rooms))
	}
}

// ---------------------------------------------------------------------------
// extractLocationFromSearchData — placeID without 0x prefix
// ---------------------------------------------------------------------------

func TestExtractLocationFromSearchData_NonHexPlaceID(t *testing.T) {
	// placeID without "0x" prefix should NOT match.
	data := []any{nil, "SomeCity", "notahexid"}
	loc := extractLocationFromSearchData(data, 0)
	if loc != "" {
		t.Errorf("expected empty for non-hex placeID, got %q", loc)
	}
}

func TestExtractLocationFromSearchData_ShortCity(t *testing.T) {
	// City name with 1 char should not match (< 2 chars).
	data := []any{nil, "X", "0xabc"}
	loc := extractLocationFromSearchData(data, 0)
	if loc != "" {
		t.Errorf("expected empty for short city, got %q", loc)
	}
}

// ---------------------------------------------------------------------------
// isGoogleConsentPage
// ---------------------------------------------------------------------------

func TestIsGoogleConsentPage_Positive(t *testing.T) {
	for _, marker := range []string{
		"consent.google.com",
		`action="https://consent.google."`,
		`id="SOCS"`,
		"SOCS blah consentheading",
	} {
		page := []byte("<html>" + marker + "</html>")
		if !isGoogleConsentPage(page) {
			t.Errorf("expected consent page for marker: %s", marker)
		}
	}
}

func TestIsGoogleConsentPage_Negative(t *testing.T) {
	page := []byte("<html>normal hotel search results</html>")
	if isGoogleConsentPage(page) {
		t.Error("should not be consent page")
	}
}

// ---------------------------------------------------------------------------
// buildHotelBookingURL
// ---------------------------------------------------------------------------

func TestBuildHotelBookingURL(t *testing.T) {
	url := buildHotelBookingURL("Paris", "2026-07-01", "2026-07-05")
	if !strings.Contains(url, "Paris") || !strings.Contains(url, "2026-07-01") {
		t.Errorf("unexpected URL: %s", url)
	}
}

// ---------------------------------------------------------------------------
// Haversine
// ---------------------------------------------------------------------------

func TestCov_Haversine_SamePoint(t *testing.T) {
	d := Haversine(60.17, 24.93, 60.17, 24.93)
	if d != 0 {
		t.Errorf("expected 0 for same point, got %v", d)
	}
}

func TestHaversine_KnownDistance(t *testing.T) {
	// Helsinki to Tallinn is approximately 80km.
	d := Haversine(60.17, 24.93, 59.43, 24.75)
	if d < 50 || d > 120 {
		t.Errorf("Helsinki-Tallinn should be ~80km, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// parseTrivagoAccommodations — fallback key paths
// ---------------------------------------------------------------------------

func TestParseTrivagoAccommodations_NestedKey(t *testing.T) {
	// Accommodations under "hotels" key instead of "accommodations".
	raw := json.RawMessage(`{"hotels":[{"accommodation_name":"Nested Hotel","price_per_night":"150","currency":"EUR"}]}`)
	results, err := parseTrivagoAccommodations(raw, "EUR")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].Name != "Nested Hotel" {
		t.Errorf("Name = %q", results[0].Name)
	}
}

func TestParseTrivagoAccommodations_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	_, err := parseTrivagoAccommodations(raw, "EUR")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseTrivagoAccommodations_EmptyAccommodations(t *testing.T) {
	raw := json.RawMessage(`{"accommodations":[]}`)
	results, err := parseTrivagoAccommodations(raw, "EUR")
	// Empty accommodations = empty slice, no error (obscure location).
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty, got %d results", len(results))
	}
}

// ---------------------------------------------------------------------------
// tryAmenityGroup — edge cases
// ---------------------------------------------------------------------------

func TestTryAmenityGroup_ValidGroup(t *testing.T) {
	arr := []any{
		"Popular",
		[]any{
			[]any{"Free WiFi"},
			[]any{"Pool"},
			[]any{"Air conditioning"},
		},
	}
	names := tryAmenityGroup(arr)
	if len(names) != 3 {
		t.Errorf("expected 3, got %d: %v", len(names), names)
	}
}

func TestTryAmenityGroup_NotGroupName(t *testing.T) {
	arr := []any{"SomeRandomString", []any{[]any{"WiFi"}}}
	names := tryAmenityGroup(arr)
	if names != nil {
		t.Errorf("expected nil for non-group name, got %v", names)
	}
}

func TestTryAmenityGroup_TooShort(t *testing.T) {
	if names := tryAmenityGroup([]any{"Popular"}); names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestTryAmenityGroup_SecondNotArray(t *testing.T) {
	arr := []any{"Popular", "not-an-array"}
	if names := tryAmenityGroup(arr); names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// tryAmenityCodePairs — edge cases
// ---------------------------------------------------------------------------

func TestTryAmenityCodePairs_ValidPairs(t *testing.T) {
	// Pairs of [available, code] where available=1 means present.
	arr := []any{
		[]any{1.0, 7.0},  // Pool
		[]any{1.0, 9.0},  // Gym
		[]any{0.0, 12.0}, // Not available
	}
	names := tryAmenityCodePairs(arr)
	if len(names) < 1 {
		t.Errorf("expected amenity names, got %d: %v", len(names), names)
	}
}

func TestTryAmenityCodePairs_TooFewElements(t *testing.T) {
	if names := tryAmenityCodePairs([]any{[]any{1.0, 7.0}, []any{1.0, 9.0}}); names != nil {
		t.Errorf("expected nil for < 3 elements, got %v", names)
	}
}

func TestCov_TryAmenityCodePairs_NotPairs(t *testing.T) {
	// Elements are not 2-element arrays.
	arr := []any{[]any{1.0, 7.0, "extra"}, []any{1.0}, []any{1.0, 9.0}}
	if names := tryAmenityCodePairs(arr); names != nil {
		t.Errorf("expected nil for invalid pairs, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// tryFlatAmenityList — edge cases
// ---------------------------------------------------------------------------

func TestTryFlatAmenityList_ValidList(t *testing.T) {
	arr := []any{
		[]any{"Free WiFi"},
		[]any{"Pool"},
		[]any{"Spa"},
		[]any{"Gym"},
	}
	names := tryFlatAmenityList(arr)
	if len(names) != 4 {
		t.Errorf("expected 4, got %d: %v", len(names), names)
	}
}

func TestTryFlatAmenityList_NotAmenityLike(t *testing.T) {
	arr := []any{
		[]any{"Random String One"},
		[]any{"Random String Two"},
		[]any{"Random String Three"},
	}
	names := tryFlatAmenityList(arr)
	if names != nil {
		t.Errorf("expected nil for non-amenity strings, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// parseDetailAmenities — page with amenity groups
// ---------------------------------------------------------------------------

func TestParseDetailAmenities_WithGroups(t *testing.T) {
	// Build a page with callbacks containing amenity group data.
	data := []any{
		"Popular",
		[]any{
			[]any{"Free WiFi"},
			[]any{"Pool"},
			[]any{"Breakfast included"},
		},
	}
	jsonBytes, _ := json.Marshal([]any{data})
	page := fmt.Sprintf("AF_initDataCallback({key: 'ds:0', data:%s});", string(jsonBytes))
	amenities, err := parseDetailAmenities(page)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(amenities) < 3 {
		t.Errorf("expected at least 3 amenities, got %d: %v", len(amenities), amenities)
	}
}

func TestCov_ParseDetailAmenities_NoCallbacks(t *testing.T) {
	if _, err := parseDetailAmenities("<html>nothing</html>"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCov_ParseDetailAmenities_NoAmenities(t *testing.T) {
	page := "AF_initDataCallback({key: 'ds:0', data:[\"not amenity data\"]});"
	if _, err := parseDetailAmenities(page); err == nil {
		t.Fatal("expected error for no amenities found")
	}
}

// ---------------------------------------------------------------------------
// mapTrivagoAccommodations — edge cases
// ---------------------------------------------------------------------------

func TestMapTrivagoAccommodations_MixedData(t *testing.T) {
	accoms := []trivagoAccommodation{
		{
			AccommodationName: "Good Hotel",
			PricePerNight:     "\u20ac200",
			Currency:          "EUR",
			HotelRating:       4,
			ReviewRating:      "8.5",
			ReviewCount:       150,
			Lat:               48.85,
			Lon:               2.35,
			AccommodationURL:  "https://trivago.com/book/good",
		},
		{
			AccommodationName: "No Price Hotel",
			PricePerNight:     "",
			AccommodationURL:  "https://trivago.com/book/noprice",
		},
	}
	results := mapTrivagoAccommodations(accoms, "EUR")
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	if results[0].Name != "Good Hotel" {
		t.Errorf("Name = %q", results[0].Name)
	}
}

// ---------------------------------------------------------------------------
// extractSponsoredAmenities — edge cases
// ---------------------------------------------------------------------------

func TestCov_ExtractSponsoredAmenities_Nil(t *testing.T) {
	got := extractSponsoredAmenities(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExtractSponsoredAmenities_Valid(t *testing.T) {
	// Amenities are flat float64 codes that map to amenityCodeMap entries.
	// 7=fitness_center, 4=pool, 2=free_wifi
	data := []any{float64(7), float64(4), float64(2)}
	got := extractSponsoredAmenities(data)
	if len(got) < 1 {
		t.Errorf("expected amenities, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// extractSponsoredHotels — edge cases for coverage
// ---------------------------------------------------------------------------

func TestExtractSponsoredHotels_NilData(t *testing.T) {
	got := extractSponsoredHotels(nil, "USD")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// parseHotelsFromPageFull — sponsored section
// ---------------------------------------------------------------------------

func TestParseHotelsFromPageFull_EmptyPage(t *testing.T) {
	result := parseHotelsFromPageFull("", "USD")
	if len(result.Hotels) != 0 {
		t.Errorf("expected 0 hotels, got %d", len(result.Hotels))
	}
}

// ---------------------------------------------------------------------------
// sortHotels — distance sort
// ---------------------------------------------------------------------------

func TestCov_SortHotels_Distance(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Far", Lat: 61.0, Lon: 25.0},
		{Name: "Near", Lat: 60.18, Lon: 24.94},
	}
	sortHotels(hotels, "distance", 60.17, 24.93)
	if hotels[0].Name != "Near" {
		t.Errorf("expected Near first, got %q", hotels[0].Name)
	}
}

func TestCov_SortHotels_Stars(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "3star", Stars: 3},
		{Name: "5star", Stars: 5},
	}
	sortHotels(hotels, "stars", 0, 0)
	if hotels[0].Name != "5star" {
		t.Errorf("expected 5star first, got %q", hotels[0].Name)
	}
}
