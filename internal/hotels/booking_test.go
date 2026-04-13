package hotels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHasBookingKey(t *testing.T) {
	t.Setenv("BOOKING_API_KEY", "")
	if HasBookingKey() {
		t.Fatal("HasBookingKey should be false when BOOKING_API_KEY is empty")
	}

	t.Setenv("BOOKING_API_KEY", "secret")
	if !HasBookingKey() {
		t.Fatal("HasBookingKey should be true when BOOKING_API_KEY is set")
	}
}

func TestBookingSearchEligibleOptions(t *testing.T) {
	if !bookingSearchEligibleOptions(HotelSearchOptions{}) {
		t.Fatal("default search options should allow Booking")
	}
	if bookingSearchEligibleOptions(HotelSearchOptions{FreeCancellation: true}) {
		t.Fatal("free-cancellation filter should disable Booking provider participation")
	}
	if bookingSearchEligibleOptions(HotelSearchOptions{EcoCertified: true}) {
		t.Fatal("eco-certified filter should disable Booking provider participation")
	}
	if bookingSearchEligibleOptions(HotelSearchOptions{PropertyType: "hotel"}) {
		t.Fatal("property-type filter should disable Booking provider participation")
	}
}

func TestSearchBookingHotelsMockServer(t *testing.T) {
	origEnabled := bookingEnabled
	origBaseURL := bookingAPIBaseURL
	origClient := bookingHTTPClient
	origResolve := bookingResolveLocationFunc
	defer func() {
		bookingEnabled = origEnabled
		bookingAPIBaseURL = origBaseURL
		bookingHTTPClient = origClient
		bookingResolveLocationFunc = origResolve
		bookingResetTokenCache()
	}()

	bookingEnabled = true
	bookingResolveLocationFunc = func(context.Context, string) (float64, float64, error) {
		return 60.17, 24.94, nil
	}

	var authCalls int
	var searchCalls int
	var detailsCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			authCalls++
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.Form.Get("api_key"); got != "test-key" {
				t.Fatalf("api_key = %q, want test-key", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-123",
				"expires_in":   "3600",
			})
		case "/demand-api-v3-compatible/accommodations/search":
			searchCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("Authorization = %q", got)
			}
			var req bookingSearchRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode search request: %v", err)
			}
			if req.Booker.Platform != "DESKTOP" {
				t.Fatalf("platform = %q", req.Booker.Platform)
			}
			if req.Coords.Radius != bookingDefaultSearchRadiusKm {
				t.Fatalf("radius = %.1f, want %.1f", req.Coords.Radius, bookingDefaultSearchRadiusKm)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":       123,
						"currency": "EUR",
						"url":      "https://www.booking.com/hotel/fi/grand.html",
						"price": map[string]any{
							"book":  111,
							"total": 121,
						},
					},
				},
			})
		case "/demand-api-v3-compatible/accommodations/details":
			detailsCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("Authorization = %q", got)
			}
			var req bookingDetailsRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode details request: %v", err)
			}
			if len(req.Accommodations) != 1 || req.Accommodations[0] != 123 {
				t.Fatalf("details accommodations = %v", req.Accommodations)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": 123,
						"name": map[string]any{
							"translations": map[string]string{"en": "Grand Hotel"},
						},
						"url": "https://www.booking.com/hotel/fi/grand.html",
						"location": map[string]any{
							"address": map[string]any{
								"fallback": "Example Street 1",
							},
							"coordinates": map[string]any{
								"latitude":  60.17,
								"longitude": 24.94,
							},
						},
						"rating": map[string]any{
							"number_of_reviews": 200,
							"review_score":      8.4,
							"stars":             4,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	bookingAPIBaseURL = srv.URL
	bookingHTTPClient = srv.Client()
	bookingResetTokenCache()
	t.Setenv("BOOKING_API_KEY", "test-key")
	t.Setenv("TRVL_BOOKER_COUNTRY", "fi")

	opts := HotelSearchOptions{
		CheckIn:  "2026-07-01",
		CheckOut: "2026-07-04",
		Guests:   2,
		Currency: "EUR",
	}

	results, err := SearchBookingHotels(context.Background(), "Helsinki", opts)
	if err != nil {
		t.Fatalf("SearchBookingHotels returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	got := results[0]
	if got.Name != "Grand Hotel" {
		t.Fatalf("Name = %q", got.Name)
	}
	if got.Price != 121 {
		t.Fatalf("Price = %.0f, want 121", got.Price)
	}
	if got.Currency != "EUR" {
		t.Fatalf("Currency = %q", got.Currency)
	}
	if got.Rating != 4.2 {
		t.Fatalf("Rating = %.1f, want 4.2", got.Rating)
	}
	if got.Stars != 4 {
		t.Fatalf("Stars = %d, want 4", got.Stars)
	}
	if got.Address != "Example Street 1" {
		t.Fatalf("Address = %q", got.Address)
	}
	if got.BookingURL != "https://www.booking.com/hotel/fi/grand.html" {
		t.Fatalf("BookingURL = %q", got.BookingURL)
	}
	if len(got.Sources) != 1 || got.Sources[0].Provider != "booking" {
		t.Fatalf("Sources = %+v", got.Sources)
	}

	_, err = SearchBookingHotels(context.Background(), "Helsinki", opts)
	if err != nil {
		t.Fatalf("second SearchBookingHotels returned error: %v", err)
	}
	if authCalls != 1 {
		t.Fatalf("authCalls = %d, want 1", authCalls)
	}
	if searchCalls != 2 || detailsCalls != 2 {
		t.Fatalf("search/details calls = %d/%d, want 2/2", searchCalls, detailsCalls)
	}
}

func TestBookingBookerCountryFallsBackToLocale(t *testing.T) {
	t.Setenv("TRVL_BOOKER_COUNTRY", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "en_FI.UTF-8")

	if got := bookingBookerCountry(); got != "fi" {
		t.Fatalf("bookingBookerCountry() = %q, want fi", got)
	}
}

func TestMapBookingHotelsSkipsMissingNames(t *testing.T) {
	results := mapBookingHotels(
		[]bookingSearchHotel{{ID: 1, Currency: "EUR", Price: bookingPriceFields{Total: 100}}},
		[]bookingDetailsHotel{{ID: 1}},
		"EUR",
	)
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(results))
	}
}

func TestSummarizeBookingBody(t *testing.T) {
	got := summarizeBookingBody([]byte(strings.Repeat("a", 220)))
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("summarizeBookingBody should trim long strings, got %q", got)
	}
}

func TestBookingAccessTokenExpiryFallback(t *testing.T) {
	origEnabled := bookingEnabled
	origBaseURL := bookingAPIBaseURL
	origClient := bookingHTTPClient
	defer func() {
		bookingEnabled = origEnabled
		bookingAPIBaseURL = origBaseURL
		bookingHTTPClient = origClient
		bookingResetTokenCache()
	}()

	bookingEnabled = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-123",
		})
	}))
	defer srv.Close()

	bookingAPIBaseURL = srv.URL
	bookingHTTPClient = srv.Client()
	bookingResetTokenCache()
	t.Setenv("BOOKING_API_KEY", "test-key")

	token, err := bookingAccessToken(context.Background())
	if err != nil {
		t.Fatalf("bookingAccessToken returned error: %v", err)
	}
	if token != "token-123" {
		t.Fatalf("token = %q", token)
	}
	if time.Until(bookingTokenExpiry) <= 0 {
		t.Fatal("expected cached token expiry to be set")
	}
}
