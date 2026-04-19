package ground

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

// ============================================================
// fetchDBBestPrice via httptest (was 0%)
// ============================================================

func TestFetchDBBestPrice_MockHappyPath(t *testing.T) {
	origClient := dbClient
	origLimiter := dbLimiter
	t.Cleanup(func() {
		dbClient = origClient
		dbLimiter = origLimiter
	})
	dbLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbBestPriceResponse{
			TagesbestPreisIntervalle: []dbBestPriceInterval{
				{AngebotsPreis: &dbPreis{Betrag: 29.90, Waehrung: "eur"}},
				{AngebotsPreis: &dbPreis{Betrag: 39.90, Waehrung: "eur"}},
			},
		})
	}))
	defer srv.Close()

	dbClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	price, cur := fetchDBBestPrice(context.Background(), "8011160", "8010205", "2026-07-01")
	if price != 29.90 {
		t.Errorf("price = %v, want 29.90", price)
	}
	if cur != "EUR" {
		t.Errorf("currency = %q, want EUR", cur)
	}
}

func TestFetchDBBestPrice_MockNoPrices(t *testing.T) {
	origClient := dbClient
	origLimiter := dbLimiter
	t.Cleanup(func() {
		dbClient = origClient
		dbLimiter = origLimiter
	})
	dbLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbBestPriceResponse{
			TagesbestPreisIntervalle: []dbBestPriceInterval{
				{AngebotsPreis: nil},
			},
		})
	}))
	defer srv.Close()

	dbClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	price, cur := fetchDBBestPrice(context.Background(), "8011160", "8010205", "2026-07-01")
	if price != 0 {
		t.Errorf("price = %v, want 0", price)
	}
	if cur != "" {
		t.Errorf("currency = %q, want empty", cur)
	}
}

func TestFetchDBBestPrice_MockHTTPError(t *testing.T) {
	origClient := dbClient
	origLimiter := dbLimiter
	t.Cleanup(func() {
		dbClient = origClient
		dbLimiter = origLimiter
	})
	dbLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dbClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	price, _ := fetchDBBestPrice(context.Background(), "8011160", "8010205", "2026-07-01")
	if price != 0 {
		t.Errorf("price = %v, want 0 for HTTP error", price)
	}
}

// ============================================================
// SearchDeutscheBahn via httptest (was 0%)
// ============================================================

func TestSearchDeutscheBahn_MockHappyPath(t *testing.T) {
	origClient := dbClient
	origLimiter := dbLimiter
	t.Cleanup(func() {
		dbClient = origClient
		dbLimiter = origLimiter
	})
	dbLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbJourneysResponse{
			Verbindungen: []dbVerbindung{
				{
					VerbindungsAbschnitte: []dbAbschnitt{
						{
							AbgangsDatum:   "2026-07-01T08:00:00",
							AnkunftsDatum:  "2026-07-01T12:30:00",
							ProduktGattung: "ICE",
							Kurztext:       "ICE 123",
						},
					},
					AngebotsPreis: &dbPreis{Betrag: 45.00, Waehrung: "EUR"},
				},
			},
		})
	}))
	defer srv.Close()

	dbClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routes, err := SearchDeutscheBahn(context.Background(), "Berlin", "Munich", "2026-07-01", "EUR")
	if err != nil {
		t.Fatalf("SearchDeutscheBahn: %v", err)
	}
	if len(routes) == 0 {
		t.Fatal("expected at least 1 route")
	}
	r := routes[0]
	if r.Provider != "db" {
		t.Errorf("provider = %q, want db", r.Provider)
	}
	if r.Type != "train" {
		t.Errorf("type = %q, want train", r.Type)
	}
	if r.Price != 45.00 {
		t.Errorf("price = %v, want 45.00", r.Price)
	}
}

func TestSearchDeutscheBahn_MockInvalidDate(t *testing.T) {
	_, err := SearchDeutscheBahn(context.Background(), "Berlin", "Munich", "not-a-date", "EUR")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestSearchDeutscheBahn_MockUnknownCity(t *testing.T) {
	_, err := SearchDeutscheBahn(context.Background(), "NoSuchCityXYZ", "Munich", "2026-07-01", "EUR")
	if err == nil {
		t.Fatal("expected error for unknown city")
	}
}

func TestSearchDeutscheBahn_MockAPIError(t *testing.T) {
	origClient := dbClient
	origLimiter := dbLimiter
	t.Cleanup(func() {
		dbClient = origClient
		dbLimiter = origLimiter
	})
	dbLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbJourneysResponse{
			FehlerNachricht: &dbError{
				Code:         "SERVICE_UNAVAILABLE",
				Ueberschrift: "Service temporarily unavailable",
			},
		})
	}))
	defer srv.Close()

	dbClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := SearchDeutscheBahn(context.Background(), "Berlin", "Munich", "2026-07-01", "EUR")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "SERVICE_UNAVAILABLE") {
		t.Errorf("error should mention SERVICE_UNAVAILABLE, got: %v", err)
	}
}

func TestSearchDeutscheBahn_MockDefaultCurrency(t *testing.T) {
	origClient := dbClient
	origLimiter := dbLimiter
	t.Cleanup(func() {
		dbClient = origClient
		dbLimiter = origLimiter
	})
	dbLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbJourneysResponse{
			Verbindungen: []dbVerbindung{},
		})
	}))
	defer srv.Close()

	dbClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	// Empty currency should default to EUR without error.
	routes, err := SearchDeutscheBahn(context.Background(), "Berlin", "Munich", "2026-07-01", "")
	if err != nil {
		t.Fatalf("SearchDeutscheBahn: %v", err)
	}
	_ = routes
}

func TestSearchDeutscheBahn_MockHTTP500(t *testing.T) {
	origClient := dbClient
	origLimiter := dbLimiter
	t.Cleanup(func() {
		dbClient = origClient
		dbLimiter = origLimiter
	})
	dbLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	dbClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := SearchDeutscheBahn(context.Background(), "Berlin", "Munich", "2026-07-01", "EUR")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

// ============================================================
// fetchDFDSAvailability via httptest — additional cases (was 0%)
// ============================================================

func TestFetchDFDSAvailability_MockDateDisabled(t *testing.T) {
	origClient := dfdsClient
	t.Cleanup(func() { dfdsClient = origClient })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := dfdsAvailabilityResponse{
			DisabledDates: []string{"2026-07-01"},
		}
		resp.Dates.FromDate = "2026-06-01"
		resp.Dates.ToDate = "2026-09-30"
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dfdsClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routeInfo := dfdsRouteInfo{RouteCode: "CPHO-OSFO", SalesOwner: 19}
	avail, isOffer, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-07-01")
	if err != nil {
		t.Fatalf("fetchDFDSAvailability: %v", err)
	}
	if avail {
		t.Error("expected unavailable for disabled date")
	}
	if isOffer {
		t.Error("expected not offer for disabled date")
	}
}

func TestFetchDFDSAvailability_MockDateBeforeRange(t *testing.T) {
	origClient := dfdsClient
	t.Cleanup(func() { dfdsClient = origClient })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := dfdsAvailabilityResponse{}
		resp.Dates.FromDate = "2026-06-01"
		resp.Dates.ToDate = "2026-09-30"
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dfdsClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routeInfo := dfdsRouteInfo{RouteCode: "CPHO-OSFO", SalesOwner: 19}
	avail, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-05-01")
	if err != nil {
		t.Fatalf("fetchDFDSAvailability: %v", err)
	}
	if avail {
		t.Error("expected unavailable for date before range")
	}
}

func TestFetchDFDSAvailability_MockDateAfterRange(t *testing.T) {
	origClient := dfdsClient
	t.Cleanup(func() { dfdsClient = origClient })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := dfdsAvailabilityResponse{}
		resp.Dates.FromDate = "2026-06-01"
		resp.Dates.ToDate = "2026-09-30"
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dfdsClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routeInfo := dfdsRouteInfo{RouteCode: "CPHO-OSFO", SalesOwner: 19}
	avail, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-10-15")
	if err != nil {
		t.Fatalf("fetchDFDSAvailability: %v", err)
	}
	if avail {
		t.Error("expected unavailable for date after range")
	}
}

func TestFetchDFDSAvailability_MockInactive(t *testing.T) {
	origClient := dfdsClient
	t.Cleanup(func() { dfdsClient = origClient })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := dfdsAvailabilityResponse{}
		resp.Dates.FromDate = ""
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dfdsClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routeInfo := dfdsRouteInfo{RouteCode: "CPHO-OSFO", SalesOwner: 19}
	avail, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-07-01")
	if err != nil {
		t.Fatalf("fetchDFDSAvailability: %v", err)
	}
	if avail {
		t.Error("expected unavailable for inactive route")
	}
}

func TestFetchDFDSAvailability_MockDecodeError(t *testing.T) {
	origClient := dfdsClient
	t.Cleanup(func() { dfdsClient = origClient })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	dfdsClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routeInfo := dfdsRouteInfo{RouteCode: "CPHO-OSFO", SalesOwner: 19}
	avail, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-07-01")
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}
	// Decode error is non-fatal; returns available=true.
	if !avail {
		t.Error("expected available=true on decode error (non-fatal)")
	}
}

// ============================================================
// fetchFinnlinesTimetables via httptest (was 0%)
// ============================================================

func TestFetchFinnlinesTimetables_MockHappyPath(t *testing.T) {
	origClient := finnlinesClient
	origLimiter := finnlinesLimiter
	t.Cleanup(func() {
		finnlinesClient = origClient
		finnlinesLimiter = origLimiter
	})
	finnlinesLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		charge := 8500
		json.NewEncoder(w).Encode(finnlinesGraphQLResponse{
			Data: struct {
				ListTimeTableAvailability []finnlinesTimetableEntry `json:"listTimeTableAvailability"`
			}{
				ListTimeTableAvailability: []finnlinesTimetableEntry{
					{
						SailingCode:   "HT-2026-07-01",
						DepartureDate: "2026-07-01",
						DepartureTime: "17:00",
						ArrivalDate:   "2026-07-02",
						ArrivalTime:   "21:00",
						DeparturePort: "FIHEL",
						ArrivalPort:   "DETRV",
						IsAvailable:   true,
						ShipName:      "Finnmaid",
						CrossingTime:  "28:00",
						ChargeTotal:   &charge,
					},
				},
			},
		})
	}))
	defer srv.Close()

	finnlinesClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	entries, err := fetchFinnlinesTimetables(context.Background(), "FIHEL", "DETRV", "2026-07-01")
	if err != nil {
		t.Fatalf("fetchFinnlinesTimetables: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ShipName != "Finnmaid" {
		t.Errorf("ship = %q, want Finnmaid", entries[0].ShipName)
	}
}

func TestFetchFinnlinesTimetables_MockGraphQLError(t *testing.T) {
	origClient := finnlinesClient
	origLimiter := finnlinesLimiter
	t.Cleanup(func() {
		finnlinesClient = origClient
		finnlinesLimiter = origLimiter
	})
	finnlinesLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{
				{"message": "Validation error"},
			},
		})
	}))
	defer srv.Close()

	finnlinesClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := fetchFinnlinesTimetables(context.Background(), "FIHEL", "DETRV", "2026-07-01")
	if err == nil {
		t.Fatal("expected error for GraphQL error response")
	}
}

func TestFetchFinnlinesTimetables_MockHTTP500(t *testing.T) {
	origClient := finnlinesClient
	origLimiter := finnlinesLimiter
	t.Cleanup(func() {
		finnlinesClient = origClient
		finnlinesLimiter = origLimiter
	})
	finnlinesLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer srv.Close()

	finnlinesClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := fetchFinnlinesTimetables(context.Background(), "FIHEL", "DETRV", "2026-07-01")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

// ============================================================
// fetchFinnlinesProducts via httptest (was 0%)
// ============================================================

func TestFetchFinnlinesProducts_MockHappyPath(t *testing.T) {
	origClient := finnlinesClient
	origLimiter := finnlinesLimiter
	t.Cleanup(func() {
		finnlinesClient = origClient
		finnlinesLimiter = origLimiter
	})
	finnlinesLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(finnlinesProductResponse{
			Data: struct {
				ListProductsAvailability []finnlinesProduct `json:"listProductsAvailability"`
			}{
				ListProductsAvailability: []finnlinesProduct{
					{Code: "BII", Type: "ACCOMMODATION", Name: "B-Inside cabin", MaxPeople: 2, Available: true, ChargePerUnit: 12900},
					{Code: "SE", Type: "SEAT", Name: "Seat", MaxPeople: 1, Available: true, ChargePerUnit: 0},
				},
			},
		})
	}))
	defer srv.Close()

	finnlinesClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	products, err := fetchFinnlinesProducts(context.Background(), "FIHEL", "DETRV", "2026-07-01", "17:00")
	if err != nil {
		t.Fatalf("fetchFinnlinesProducts: %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}
}

func TestFetchFinnlinesProducts_MockHTTP500(t *testing.T) {
	origClient := finnlinesClient
	origLimiter := finnlinesLimiter
	t.Cleanup(func() {
		finnlinesClient = origClient
		finnlinesLimiter = origLimiter
	})
	finnlinesLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer srv.Close()

	finnlinesClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := fetchFinnlinesProducts(context.Background(), "FIHEL", "DETRV", "2026-07-01", "17:00")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestFetchFinnlinesProducts_MockGraphQLError(t *testing.T) {
	origClient := finnlinesClient
	origLimiter := finnlinesLimiter
	t.Cleanup(func() {
		finnlinesClient = origClient
		finnlinesLimiter = origLimiter
	})
	finnlinesLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "product error"}},
		})
	}))
	defer srv.Close()

	finnlinesClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := fetchFinnlinesProducts(context.Background(), "FIHEL", "DETRV", "2026-07-01", "17:00")
	if err == nil {
		t.Fatal("expected error for GraphQL error")
	}
}

// ============================================================
// SearchEuropeanSleeper via httptest (additional cases, was 0%)
// ============================================================

func TestSearchEuropeanSleeper_MockHappyPath2(t *testing.T) {
	origClient := europeanSleeperClient
	origLimiter := europeanSleeperLimiter
	t.Cleanup(func() {
		europeanSleeperClient = origClient
		europeanSleeperLimiter = origLimiter
	})
	europeanSleeperLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(europeanSleeperTripsResponse{
			Trips: []europeanSleeperTrip{
				{
					DepartureTime: "2026-07-01T19:22:00",
					ArrivalTime:   "2026-07-02T07:30:00",
					Duration:      728,
					Prices: []europeanSleeperPrice{
						{Amount: 49.0, Currency: "EUR", Class: "seat"},
						{Amount: 99.0, Currency: "EUR", Class: "couchette"},
					},
					Segments: []europeanSleeperSegment{
						{
							DepartureTime: "2026-07-01T19:22:00",
							ArrivalTime:   "2026-07-02T07:30:00",
							Origin:        europeanSleeperTripStop{Name: "Brussels-Midi"},
							Destination:   europeanSleeperTripStop{Name: "Prague hl.n."},
							TrainNumber:   "NJ 40421",
							Operator:      "European Sleeper",
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	europeanSleeperClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routes, err := SearchEuropeanSleeper(context.Background(), "Brussels", "Prague", "2026-07-01", "EUR")
	if err != nil {
		t.Fatalf("SearchEuropeanSleeper: %v", err)
	}
	if len(routes) == 0 {
		t.Fatal("expected at least 1 route")
	}
	r := routes[0]
	if r.Provider != "european_sleeper" {
		t.Errorf("provider = %q, want european_sleeper", r.Provider)
	}
	if r.Price != 49.0 {
		t.Errorf("price = %v, want 49.0", r.Price)
	}
}

func TestSearchEuropeanSleeper_MockNon200(t *testing.T) {
	origClient := europeanSleeperClient
	origLimiter := europeanSleeperLimiter
	t.Cleanup(func() {
		europeanSleeperClient = origClient
		europeanSleeperLimiter = origLimiter
	})
	europeanSleeperLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	europeanSleeperClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routes, err := SearchEuropeanSleeper(context.Background(), "Brussels", "Prague", "2026-07-01", "EUR")
	if err != nil {
		t.Fatalf("expected nil error for non-200 (graceful), got: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("expected empty routes for non-200, got %d", len(routes))
	}
}

func TestSearchEuropeanSleeper_UnknownStation2(t *testing.T) {
	_, err := SearchEuropeanSleeper(context.Background(), "NoSuchCityABC", "Prague", "2026-07-01", "EUR")
	if err == nil {
		t.Fatal("expected error for unknown station")
	}
}

func TestSearchEuropeanSleeper_MockDefaultCurrency(t *testing.T) {
	origClient := europeanSleeperClient
	origLimiter := europeanSleeperLimiter
	t.Cleanup(func() {
		europeanSleeperClient = origClient
		europeanSleeperLimiter = origLimiter
	})
	europeanSleeperLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "currency=EUR") {
			t.Errorf("expected currency=EUR in query, got: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(europeanSleeperTripsResponse{Trips: nil})
	}))
	defer srv.Close()

	europeanSleeperClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := SearchEuropeanSleeper(context.Background(), "Brussels", "Prague", "2026-07-01", "")
	if err != nil {
		t.Fatalf("SearchEuropeanSleeper: %v", err)
	}
}

func TestSearchEuropeanSleeper_MockEmptyTrips(t *testing.T) {
	origClient := europeanSleeperClient
	origLimiter := europeanSleeperLimiter
	t.Cleanup(func() {
		europeanSleeperClient = origClient
		europeanSleeperLimiter = origLimiter
	})
	europeanSleeperLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(europeanSleeperTripsResponse{Trips: nil})
	}))
	defer srv.Close()

	europeanSleeperClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routes, err := SearchEuropeanSleeper(context.Background(), "Brussels", "Prague", "2026-07-01", "EUR")
	if err != nil {
		t.Fatalf("SearchEuropeanSleeper: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for empty trips, got %d", len(routes))
	}
}

// ============================================================
// SearchDigitransit — additional coverage (was 16.7%)
// ============================================================

func TestSearchDigitransit_MockHappyPath2(t *testing.T) {
	origClient := httpClient
	origLimiter := digitransitLimiter
	t.Cleanup(func() {
		httpClient = origClient
		digitransitLimiter = origLimiter
	})
	digitransitLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"plan": map[string]any{
					"itineraries": []any{
						map[string]any{
							"duration":    5400,
							"startTime":   float64(time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC).UnixMilli()),
							"endTime":     float64(time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC).UnixMilli()),
							"walkDistance": 500.0,
							"waitingTime": 0,
							"transfers":   0,
							"legs": []any{
								map[string]any{
									"mode":       "RAIL",
									"startTime":  float64(time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC).UnixMilli()),
									"endTime":    float64(time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC).UnixMilli()),
									"duration":   5400,
									"from":       map[string]any{"name": "Helsinki", "stop": map[string]any{"code": "HEL"}},
									"to":         map[string]any{"name": "Tampere", "stop": map[string]any{"code": "TRE"}},
									"route":      map[string]any{"shortName": "IC 123", "agency": map[string]any{"name": "VR"}},
									"transitLeg": true,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	httpClient = &http.Client{
		Transport: &redirectTransport{target: srv.URL},
		Timeout:   5 * time.Second,
	}

	routes, err := SearchDigitransit(context.Background(), "Helsinki", "Tampere", "2026-07-01", "EUR")
	if err != nil {
		t.Fatalf("SearchDigitransit: %v", err)
	}
	if len(routes) == 0 {
		t.Fatal("expected at least 1 route")
	}
	if routes[0].Provider != "vr" {
		t.Errorf("provider = %q, want vr", routes[0].Provider)
	}
}

// ============================================================
// Additional edge case coverage
// ============================================================

func TestIsProviderNotApplicable_ContextDeadline(t *testing.T) {
	if !isProviderNotApplicable(context.DeadlineExceeded) {
		t.Error("expected true for context.DeadlineExceeded")
	}
}

func TestIsProviderNotApplicable_Nil2(t *testing.T) {
	if isProviderNotApplicable(nil) {
		t.Error("expected false for nil error")
	}
}

func TestDeduplicateGroundRoutes_SameProviderDifferentTimes(t *testing.T) {
	routes := []models.GroundRoute{
		{Provider: "db", Departure: models.GroundStop{Time: "08:00"}, Arrival: models.GroundStop{Time: "12:00"}, Price: 45},
		{Provider: "db", Departure: models.GroundStop{Time: "10:00"}, Arrival: models.GroundStop{Time: "14:00"}, Price: 45},
	}
	result := deduplicateGroundRoutes(routes)
	if len(result) != 2 {
		t.Errorf("expected 2 routes (different times), got %d", len(result))
	}
}

func TestFilterUnavailableGroundRoutes_MixedProviders(t *testing.T) {
	routes := []models.GroundRoute{
		{Provider: "flixbus", Price: 0},
		{Provider: "transitous", Price: 0},
		{Provider: "db", Price: 25},
	}
	result := filterUnavailableGroundRoutes(routes)
	if len(result) != 2 {
		t.Errorf("expected 2 routes, got %d", len(result))
	}
}

func TestSearchResultBufferCapacity_NonZero(t *testing.T) {
	cap := searchResultBufferCapacity()
	if cap <= 0 {
		t.Errorf("searchResultBufferCapacity() = %d, want > 0", cap)
	}
}
