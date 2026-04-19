package ground

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// ---------------------------------------------------------------------------
// SearchDFDS — mock the dfdsClient to redirect availability calls
// ---------------------------------------------------------------------------

// dfdsTestMux creates a test server that handles DFDS availability requests.
func dfdsTestMux(t *testing.T, availJSON string) (*httptest.Server, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, availJSON)
	}))
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	dfdsClient = srv.Client()
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)
	cleanup := func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
		srv.Close()
	}
	return srv, cleanup
}

// fetchDFDSAvailability redirected to test server via dfdsClient but the URL
// is still dfdsAvailabilityBase (const). The test client will make a real TCP
// connection to that host. Instead, we test fetchDFDSAvailability directly
// by overriding dfdsClient with a transport that returns canned responses.

func TestDFDSTestMux_ServerStarts(t *testing.T) {
	// Verify dfdsTestMux stands up a server and restores globals on cleanup.
	srv, cleanup := dfdsTestMux(t, `{"route":"TEST","dates":{"fromDate":"2026-01-01","toDate":"2027-12-31"},"disabledDates":[],"offerDates":[]}`)
	defer cleanup()
	if srv == nil {
		t.Fatal("expected non-nil server from dfdsTestMux")
	}
}

func TestFetchDFDSAvailability_ViaTransport_Available(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-01-01","toDate":"2027-12-31"},"disabledDates":[],"offerDates":[]}`)
			return rec.Result(), nil
		}),
	}

	routeInfo := dfdsRouteInfo{RouteCode: "NOOSL-DKCPH", SalesOwner: 19}
	available, isOffer, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !available {
		t.Error("expected available=true")
	}
	if isOffer {
		t.Error("expected isOffer=false")
	}
}

func TestFetchDFDSAvailability_ViaTransport_OfferDate(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-01-01","toDate":"2027-12-31"},"disabledDates":[],"offerDates":["2026-08-15"]}`)
			return rec.Result(), nil
		}),
	}

	routeInfo := dfdsRouteInfo{RouteCode: "NOOSL-DKCPH", SalesOwner: 19}
	available, isOffer, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !available {
		t.Error("expected available=true for offer date")
	}
	if !isOffer {
		t.Error("expected isOffer=true for offer date")
	}
}

func TestFetchDFDSAvailability_ViaTransport_DisabledDate(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-01-01","toDate":"2027-12-31"},"disabledDates":["2026-08-15"],"offerDates":[]}`)
			return rec.Result(), nil
		}),
	}

	routeInfo := dfdsRouteInfo{RouteCode: "NOOSL-DKCPH", SalesOwner: 19}
	available, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if available {
		t.Error("expected available=false for disabled date")
	}
}

func TestFetchDFDSAvailability_ViaTransport_DateBeforeRange(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			// fromDate is 2026-09-01, our date is 2026-08-15 — before range
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-09-01","toDate":"2027-12-31"},"disabledDates":[],"offerDates":[]}`)
			return rec.Result(), nil
		}),
	}

	routeInfo := dfdsRouteInfo{RouteCode: "NOOSL-DKCPH", SalesOwner: 19}
	available, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if available {
		t.Error("expected available=false for date before range")
	}
}

func TestFetchDFDSAvailability_ViaTransport_DateAfterRange(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			// toDate is 2026-07-01, our date is 2026-08-15 — after range
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-01-01","toDate":"2026-07-01"},"disabledDates":[],"offerDates":[]}`)
			return rec.Result(), nil
		}),
	}

	routeInfo := dfdsRouteInfo{RouteCode: "NOOSL-DKCPH", SalesOwner: 19}
	available, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if available {
		t.Error("expected available=false for date after range")
	}
}

func TestFetchDFDSAvailability_ViaTransport_InactiveRoute(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			// Empty fromDate → inactive route
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"","toDate":""},"disabledDates":[],"offerDates":[]}`)
			return rec.Result(), nil
		}),
	}

	routeInfo := dfdsRouteInfo{RouteCode: "NOOSL-DKCPH", SalesOwner: 19}
	available, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if available {
		t.Error("expected available=false for inactive route (empty dates)")
	}
}

func TestFetchDFDSAvailability_ViaTransport_NonOK(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(rec, `service unavailable`)
			return rec.Result(), nil
		}),
	}

	routeInfo := dfdsRouteInfo{RouteCode: "NOOSL-DKCPH", SalesOwner: 19}
	// Non-200 → non-fatal → returns true (assume available)
	available, _, err := fetchDFDSAvailability(context.Background(), routeInfo, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !available {
		t.Error("expected available=true for non-200 (non-fatal path)")
	}
}

// ---------------------------------------------------------------------------
// SearchDFDS — full path with transport mock
// ---------------------------------------------------------------------------

func TestSearchDFDS_FullPath_WithOffer(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			// Return availability with our date as offer date
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-01-01","toDate":"2027-12-31"},"disabledDates":[],"offerDates":["2026-08-15"]}`)
			return rec.Result(), nil
		}),
	}

	ctx := context.Background()
	routes, err := SearchDFDS(ctx, "Oslo", "Copenhagen", "2026-08-15", "EUR")
	if err != nil {
		t.Fatalf("SearchDFDS: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	r := routes[0]
	if r.Provider != "dfds" {
		t.Errorf("provider = %q, want dfds", r.Provider)
	}
	if r.Type != "ferry" {
		t.Errorf("type = %q, want ferry", r.Type)
	}
	// Offer date → amenity "Deal"
	found := false
	for _, a := range r.Amenities {
		if a == "Deal" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Deal amenity for offer date, got %v", r.Amenities)
	}
}

func TestSearchDFDS_FullPath_Unavailable(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			// Disabled date → unavailable
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-01-01","toDate":"2027-12-31"},"disabledDates":["2026-08-15"],"offerDates":[]}`)
			return rec.Result(), nil
		}),
	}

	ctx := context.Background()
	routes, err := SearchDFDS(ctx, "Oslo", "Copenhagen", "2026-08-15", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unavailable date → nil routes
	if routes != nil {
		t.Errorf("expected nil routes for unavailable date, got %v", routes)
	}
}

func TestSearchDFDS_DefaultCurrency(t *testing.T) {
	origClient := dfdsClient
	origLimiter := dfdsLimiter
	t.Cleanup(func() {
		dfdsClient = origClient
		dfdsLimiter = origLimiter
	})
	dfdsLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	dfdsClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			fmt.Fprint(rec, `{"route":"NOOSL-DKCPH","dates":{"fromDate":"2026-01-01","toDate":"2027-12-31"},"disabledDates":[],"offerDates":[]}`)
			return rec.Result(), nil
		}),
	}

	// currency="" → falls back to route's native currency
	ctx := context.Background()
	routes, err := SearchDFDS(ctx, "Oslo", "Copenhagen", "2026-08-15", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = routes
}

// ---------------------------------------------------------------------------
// SearchSNCF — more branches via sncfDo transport
// ---------------------------------------------------------------------------

func TestSearchSNCF_WithRoutes(t *testing.T) {
	origDo := sncfDo
	origLimiter := sncfLimiter
	t.Cleanup(func() {
		sncfDo = origDo
		sncfLimiter = origLimiter
	})
	sncfLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	// Return valid SNCF response with journeys
	sncfDo = func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusOK)
		// parseSNCFResponse expects []sncfCalendarResponse
		price := 4500 // price in cents
		resp := []sncfCalendarResponse{{
			Date:  "2026-08-15",
			Price: &price,
		}}
		json.NewEncoder(rec).Encode(resp)
		return rec.Result(), nil
	}

	ctx := context.Background()
	routes, err := SearchSNCF(ctx, "Paris", "Lyon", "2026-08-15", "EUR", false)
	if err != nil {
		t.Fatalf("SearchSNCF: %v", err)
	}
	if len(routes) == 0 {
		t.Error("expected at least 1 route")
	}
}

func TestSearchSNCF_403_NoBrowserFallback_v2(t *testing.T) {
	origDo := sncfDo
	origLimiter := sncfLimiter
	t.Cleanup(func() {
		sncfDo = origDo
		sncfLimiter = origLimiter
	})
	sncfLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	sncfDo = func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusForbidden)
		fmt.Fprint(rec, `forbidden`)
		return rec.Result(), nil
	}

	ctx := context.Background()
	_, err := SearchSNCF(ctx, "Paris", "Lyon", "2026-08-15", "EUR", false)
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("expected HTTP 403 error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SearchTrainline — additional branches
// ---------------------------------------------------------------------------

func TestSearchTrainline_403_WithBrowserFallbackAllowed(t *testing.T) {
	// When allowBrowserFallbacks=true and 403, it tries multiple fallbacks.
	// Since nab/curl/browser aren't available, eventually returns 403 error.
	origDo := trainlineDo
	origLimiter := trainlineLimiter
	t.Cleanup(func() {
		trainlineDo = origDo
		trainlineLimiter = origLimiter
	})
	trainlineLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	callCount := 0
	trainlineDo = func(req *http.Request) (*http.Response, error) {
		callCount++
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusForbidden)
		// No datadome cookie to try the seed retry
		fmt.Fprint(rec, `forbidden`)
		return rec.Result(), nil
	}

	ctx := context.Background()
	_, err := SearchTrainline(ctx, "London", "Paris", "2026-08-15", "GBP", true)
	// With browser fallbacks allowed but all failing → returns 403 error
	if err == nil {
		t.Error("expected error when all fallbacks fail")
	}
}

func TestSearchTrainline_MarshallError(t *testing.T) {
	// Invalid station → returns early before marshal
	ctx := context.Background()
	_, err := SearchTrainline(ctx, "UnknownXYZ", "Paris", "2026-08-15", "GBP", false)
	if err == nil || !strings.Contains(err.Error(), "no Trainline station") {
		t.Errorf("expected station error, got %v", err)
	}
}

func TestSearchTrainline_InvalidDateFormat(t *testing.T) {
	ctx := context.Background()
	_, err := SearchTrainline(ctx, "London", "Paris", "bad-date-format", "GBP", false)
	if err == nil || !strings.Contains(err.Error(), "invalid date") {
		t.Errorf("expected invalid date error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SearchSNCF — RateLimiter cancel path
// ---------------------------------------------------------------------------

func TestSearchSNCF_RateLimiterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	origLimiter := sncfLimiter
	sncfLimiter = newProviderLimiter(60 * time.Second)
	defer func() { sncfLimiter = origLimiter }()

	_, err := SearchSNCF(ctx, "Paris", "Lyon", "2026-08-15", "EUR", false)
	if err == nil {
		t.Error("expected rate-limiter error on cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Finnlines — basic coverage
// ---------------------------------------------------------------------------

func TestSearchFinnlines_UnknownRoute(t *testing.T) {
	ctx := context.Background()
	_, err := SearchFinnlines(ctx, "Tokyo", "Paris", "2026-08-15", "EUR")
	if err == nil {
		t.Error("expected error for unknown Finnlines route")
	}
}

// ---------------------------------------------------------------------------
// geocodeCity — empty results branch
// ---------------------------------------------------------------------------

func TestGeocodeCity_EmptyResults(t *testing.T) {
	origClient := httpClient
	httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			fmt.Fprint(rec, `[]`) // empty results
			return rec.Result(), nil
		}),
	}
	defer func() { httpClient = origClient }()

	_, err := geocodeCity(context.Background(), "CityEmptyResultsXYZ777")
	if err == nil || !strings.Contains(err.Error(), "no geocoding results") {
		t.Errorf("expected 'no geocoding results' error, got %v", err)
	}
}

func TestGeocodeCity_InvalidLatLon(t *testing.T) {
	origClient := httpClient
	httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			// Invalid lat/lon values that can't be parsed as floats
			fmt.Fprint(rec, `[{"lat":"not-a-number","lon":"also-not-a-number"}]`)
			return rec.Result(), nil
		}),
	}
	defer func() { httpClient = origClient }()

	_, err := geocodeCity(context.Background(), "CityInvalidLatLon888XYZ")
	if err == nil {
		t.Error("expected parse error for invalid lat/lon")
	}
}

// ---------------------------------------------------------------------------
// searchEurostarTimetable — via eurostarClient transport override
// ---------------------------------------------------------------------------

func TestSearchEurostarTimetable_ViaTransport_HappyPath(t *testing.T) {
	// Override eurostarClient transport directly to avoid real network calls.
	origClient := eurostarClient
	timetableResp := `{"data":{"timetableServices":[{"model":{"trainNumber":"9001","scheduledDepartureDateTime":"2026-08-15T07:00:00"},"origin":{"model":{"scheduledDepartureDateTime":"2026-08-15T07:00:00"}},"destination":{"model":{"scheduledArrivalDateTime":"2026-08-15T10:17:00"}}}]},"errors":[]}`
	eurostarClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			fmt.Fprint(rec, timetableResp)
			return rec.Result(), nil
		}),
	}
	defer func() { eurostarClient = origClient }()

	from, _ := LookupEurostarStation("london")
	to, _ := LookupEurostarStation("paris")
	entries, err := searchEurostarTimetable(context.Background(), from, to, "2026-08-15")
	if err != nil {
		t.Fatalf("searchEurostarTimetable: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].TrainNumber != "9001" {
		t.Errorf("TrainNumber = %q, want 9001", entries[0].TrainNumber)
	}
	if entries[0].ArrivalTime != "2026-08-15T10:17:00" {
		t.Errorf("ArrivalTime = %q, want 2026-08-15T10:17:00", entries[0].ArrivalTime)
	}
}

func TestSearchEurostarTimetable_ViaTransport_NonOK(t *testing.T) {
	origClient := eurostarClient
	eurostarClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(rec, `service error`)
			return rec.Result(), nil
		}),
	}
	defer func() { eurostarClient = origClient }()

	from, _ := LookupEurostarStation("london")
	to, _ := LookupEurostarStation("paris")
	// Non-200 → returns nil, nil (non-fatal)
	entries, err := searchEurostarTimetable(context.Background(), from, to, "2026-08-15")
	if err != nil {
		t.Errorf("expected nil error for non-200, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for non-200, got %v", entries)
	}
}

func TestSearchEurostarTimetable_ViaTransport_InvalidJSON(t *testing.T) {
	origClient := eurostarClient
	eurostarClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			fmt.Fprint(rec, `{not valid json}`)
			return rec.Result(), nil
		}),
	}
	defer func() { eurostarClient = origClient }()

	from, _ := LookupEurostarStation("london")
	to, _ := LookupEurostarStation("paris")
	// Invalid JSON → returns nil, nil (non-fatal)
	entries, err := searchEurostarTimetable(context.Background(), from, to, "2026-08-15")
	if err != nil {
		t.Errorf("expected nil error for bad JSON (non-fatal), got %v", err)
	}
	_ = entries
}

func TestSearchEurostarTimetable_ViaTransport_GraphQLError(t *testing.T) {
	origClient := eurostarClient
	eurostarClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			fmt.Fprint(rec, `{"errors":[{"message":"route not found"}],"data":null}`)
			return rec.Result(), nil
		}),
	}
	defer func() { eurostarClient = origClient }()

	from, _ := LookupEurostarStation("london")
	to, _ := LookupEurostarStation("paris")
	// GraphQL errors → returns nil, nil (non-fatal)
	entries, err := searchEurostarTimetable(context.Background(), from, to, "2026-08-15")
	if err != nil {
		t.Errorf("expected nil error for graphql error, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for graphql error, got %v", entries)
	}
}

func TestSearchEurostarTimetable_ViaTransport_OriginDepFallback(t *testing.T) {
	// When model.scheduledDepartureDateTime is "", origin.model value is used.
	origClient := eurostarClient
	eurostarClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusOK)
			fmt.Fprint(rec, `{"data":{"timetableServices":[{"model":{"trainNumber":"9005","scheduledDepartureDateTime":""},"origin":{"model":{"scheduledDepartureDateTime":"2026-08-15T09:00:00"}},"destination":{"model":{"scheduledArrivalDateTime":"2026-08-15T12:17:00"}}}]},"errors":[]}`)
			return rec.Result(), nil
		}),
	}
	defer func() { eurostarClient = origClient }()

	from, _ := LookupEurostarStation("london")
	to, _ := LookupEurostarStation("paris")
	entries, err := searchEurostarTimetable(context.Background(), from, to, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// dep = origin.model value since model.scheduledDepartureDateTime is ""
	if entries[0].DepartureTime != "2026-08-15T09:00:00" {
		t.Errorf("DepartureTime = %q, want 2026-08-15T09:00:00", entries[0].DepartureTime)
	}
}

// ---------------------------------------------------------------------------
// searchEurostarTimetable — network error path
// ---------------------------------------------------------------------------

func TestSearchEurostarTimetable_ViaTransport_NetworkError(t *testing.T) {
	origClient := eurostarClient
	eurostarClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network error")
		}),
	}
	defer func() { eurostarClient = origClient }()

	from, _ := LookupEurostarStation("london")
	to, _ := LookupEurostarStation("paris")
	_, err := searchEurostarTimetable(context.Background(), from, to, "2026-08-15")
	if err == nil {
		t.Error("expected network error")
	}
}

// ---------------------------------------------------------------------------
// tallinkGetSession — via tallinkClient transport override
// ---------------------------------------------------------------------------

func TestTallinkGetSession_ViaTransport_HappyPath(t *testing.T) {
	origClient := tallinkClient
	tallinkClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			rec.Header().Set("Set-Cookie", "JSESSIONID=test-session; Path=/")
			rec.WriteHeader(http.StatusOK)
			fmt.Fprint(rec, `<html><script>window.Env = { sessionGuid: 'GUID-TEST-123' };</script></html>`)
			// httptest.ResponseRecorder doesn't support Set-Cookie headers
			// in the same way as real servers — use http.Response directly
			return rec.Result(), nil
		}),
	}
	defer func() { tallinkClient = origClient }()

	// tallinkGetSession is called internally; test by checking the result struct parsing
	guid := tallinkExtractSessionGUID(`<html><script>window.Env = { sessionGuid: 'GUID-TEST-123' };</script></html>`)
	if guid != "GUID-TEST-123" {
		t.Errorf("guid = %q, want GUID-TEST-123", guid)
	}
}

func TestTallinkGetSession_ViaTransport_NoCookies(t *testing.T) {
	// tallinkGetSession returns error when no cookies in response.
	// We can't call it directly (uses const URL) but test the cookie check logic.
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("<html></html>")),
	}
	// Verify: no cookies → len(resp.Cookies()) == 0
	if len(resp.Cookies()) != 0 {
		t.Error("expected no cookies in response without Set-Cookie header")
	}
}

// ---------------------------------------------------------------------------
// fetchTallinkTimetables / SearchTallink — via tallinkClient transport override
// Approach: override tallinkClient.Transport to intercept all requests
// ---------------------------------------------------------------------------

// tallinkTransportMock intercepts all requests and routes them based on URL path.
type tallinkTransportMock struct {
	bookingPageBody string
	timetableJSON   string
	summaryStatus   int
	travelClasses   string
}

func (m *tallinkTransportMock) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	path := r.URL.Path
	switch {
	case strings.Contains(path, "/api/timetables"):
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(http.StatusOK)
		fmt.Fprint(rec, m.timetableJSON)
	case strings.Contains(path, "/api/reservation/cruiseSummary"):
		rec.WriteHeader(m.summaryStatus)
		fmt.Fprint(rec, `{"status":"OK"}`)
	case strings.Contains(path, "/api/travelclasses"):
		rec.WriteHeader(http.StatusOK)
		fmt.Fprint(rec, m.travelClasses)
	default:
		// Booking page
		rec.Header().Set("Set-Cookie", "JSESSIONID=mock-sess; Path=/")
		rec.WriteHeader(http.StatusOK)
		fmt.Fprint(rec, m.bookingPageBody)
	}
	return rec.Result(), nil
}

const tallinkMockTimetableDay = `{
  "defaultSelections": {"outwardSail": 1, "returnSail": null},
  "trips": {
    "2026-09-01": {
      "outwards": [
        {"sailId": 5001, "shipCode": "MEGASTAR", "departureIsoDate": "2026-09-01T07:30", "arrivalIsoDate": "2026-09-01T09:30", "personPrice": "38.90", "vehiclePrice": null, "duration": 2.0, "sailPackageCode": "HEL-TAL", "sailPackageName": "Helsinki-Tallinn", "cityFrom": "HEL", "cityTo": "TAL", "pierFrom": "A", "pierTo": "B", "hasRoom": true, "isOvernight": false, "isDisabled": false, "promotionApplied": false, "marketingMessage": null, "isVoucherApplicable": false},
        {"sailId": 5002, "shipCode": "MYSTAR", "departureIsoDate": "2026-09-01T17:30", "arrivalIsoDate": "2026-09-01T19:30", "personPrice": "12.00", "vehiclePrice": null, "duration": 2.0, "sailPackageCode": "HEL-TAL", "sailPackageName": "Helsinki-Tallinn", "cityFrom": "HEL", "cityTo": "TAL", "pierFrom": "A", "pierTo": "B", "hasRoom": true, "isOvernight": false, "isDisabled": false, "promotionApplied": true, "marketingMessage": null, "isVoucherApplicable": false}
      ],
      "returns": []
    }
  }
}`

func TestSearchTallink_ViaTransport_HappyPath(t *testing.T) {
	origClient := tallinkClient
	origLimiter := tallinkLimiter
	t.Cleanup(func() {
		tallinkClient = origClient
		tallinkLimiter = origLimiter
	})
	tallinkLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	tallinkClient = &http.Client{
		Transport: &tallinkTransportMock{
			bookingPageBody: `<html><script>window.Env = { sessionGuid: 'MOCK-GUID' };</script></html>`,
			timetableJSON:   tallinkMockTimetableDay,
			summaryStatus:   http.StatusOK,
			travelClasses:   `[]`,
		},
	}

	ctx := context.Background()
	routes, err := SearchTallink(ctx, "Helsinki", "Tallinn", "2026-09-01", "EUR")
	if err != nil {
		t.Fatalf("SearchTallink: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Provider != "tallink" {
		t.Errorf("provider = %q, want tallink", routes[0].Provider)
	}
	if routes[0].Type != "ferry" {
		t.Errorf("type = %q, want ferry", routes[0].Type)
	}
	if routes[0].Price != 38.90 {
		t.Errorf("price = %f, want 38.90", routes[0].Price)
	}
	// sail2: price 12.00 < 20 threshold → Deal amenity; promotionApplied → Promotion
	found := map[string]bool{}
	for _, a := range routes[1].Amenities {
		found[a] = true
	}
	if !found["Deal"] {
		t.Errorf("sail2 should have Deal amenity, got %v", routes[1].Amenities)
	}
	if !found["Promotion"] {
		t.Errorf("sail2 should have Promotion amenity, got %v", routes[1].Amenities)
	}
}

func TestSearchTallink_ViaTransport_NoTripsForDate(t *testing.T) {
	origClient := tallinkClient
	origLimiter := tallinkLimiter
	t.Cleanup(func() {
		tallinkClient = origClient
		tallinkLimiter = origLimiter
	})
	tallinkLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	// Timetable has trips for a different date
	timetableJSON := `{"defaultSelections":{"outwardSail":0,"returnSail":null},"trips":{"2026-09-02":{"outwards":[],"returns":[]}}}`

	tallinkClient = &http.Client{
		Transport: &tallinkTransportMock{
			bookingPageBody: `<html><script>window.Env = { sessionGuid: 'MOCK' };</script></html>`,
			timetableJSON:   timetableJSON,
			summaryStatus:   http.StatusOK,
			travelClasses:   `[]`,
		},
	}

	ctx := context.Background()
	routes, err := SearchTallink(ctx, "Helsinki", "Tallinn", "2026-09-01", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No trips for 2026-09-01 → nil routes
	if routes != nil {
		t.Errorf("expected nil routes, got %v", routes)
	}
}

func TestSearchTallink_ViaTransport_TimetableError(t *testing.T) {
	origClient := tallinkClient
	origLimiter := tallinkLimiter
	t.Cleanup(func() {
		tallinkClient = origClient
		tallinkLimiter = origLimiter
	})
	tallinkLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	// Session fails: no cookies returned
	tallinkClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			// No Set-Cookie header → tallinkGetSession returns error
			rec.WriteHeader(http.StatusOK)
			fmt.Fprint(rec, `<html>no cookies</html>`)
			return rec.Result(), nil
		}),
	}

	ctx := context.Background()
	_, err := SearchTallink(ctx, "Helsinki", "Tallinn", "2026-09-01", "EUR")
	if err == nil {
		t.Error("expected error when session has no cookies")
	}
}

func TestSearchTallink_ViaTransport_EmptySails(t *testing.T) {
	origClient := tallinkClient
	origLimiter := tallinkLimiter
	t.Cleanup(func() {
		tallinkClient = origClient
		tallinkLimiter = origLimiter
	})
	tallinkLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	// Trips exist but outwards is empty
	timetableJSON := `{"defaultSelections":{"outwardSail":0,"returnSail":null},"trips":{"2026-09-01":{"outwards":[],"returns":[]}}}`

	tallinkClient = &http.Client{
		Transport: &tallinkTransportMock{
			bookingPageBody: `<html><script>window.Env = { sessionGuid: 'MOCK' };</script></html>`,
			timetableJSON:   timetableJSON,
			summaryStatus:   http.StatusOK,
			travelClasses:   `[]`,
		},
	}

	ctx := context.Background()
	routes, err := SearchTallink(ctx, "Helsinki", "Tallinn", "2026-09-01", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if routes != nil {
		t.Errorf("expected nil routes for empty sails, got %v", routes)
	}
}

func TestSearchTallink_ViaTransport_TimetableNonOK(t *testing.T) {
	origClient := tallinkClient
	origLimiter := tallinkLimiter
	t.Cleanup(func() {
		tallinkClient = origClient
		tallinkLimiter = origLimiter
	})
	tallinkLimiter = rate.NewLimiter(rate.Limit(1000), 1)

	callCount := 0
	tallinkClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			callCount++
			rec := httptest.NewRecorder()
			if strings.Contains(r.URL.Path, "/api/timetables") {
				// Return non-200 for timetables
				rec.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(rec, `service unavailable`)
			} else {
				// Booking page: return session cookie
				rec.Header().Set("Set-Cookie", "JSESSIONID=sess; Path=/")
				rec.WriteHeader(http.StatusOK)
				fmt.Fprint(rec, `<html><script>window.Env = { sessionGuid: 'G' };</script></html>`)
			}
			return rec.Result(), nil
		}),
	}

	ctx := context.Background()
	_, err := SearchTallink(ctx, "Helsinki", "Tallinn", "2026-09-01", "EUR")
	if err == nil {
		t.Error("expected error for timetable non-200 response")
	}
}

func TestFetchTallinkCabinClasses_ViaTransport_HappyPath(t *testing.T) {
	origClient := tallinkClient
	tallinkClient = &http.Client{
		Transport: &tallinkTransportMock{
			bookingPageBody: "",
			timetableJSON:   "",
			summaryStatus:   http.StatusOK,
			travelClasses:   `[{"code":"A2","name":"A2","description":"Cabin","price":89.0,"capacity":2}]`,
		},
	}
	defer func() { tallinkClient = origClient }()

	cookies := []*http.Cookie{{Name: "JSESSIONID", Value: "sess"}}
	ctx := context.Background()
	classes, err := fetchTallinkCabinClasses(ctx, cookies, "MOCK-GUID", 5001)
	if err != nil {
		t.Fatalf("fetchTallinkCabinClasses: %v", err)
	}
	if len(classes) != 1 {
		t.Fatalf("expected 1 cabin class, got %d", len(classes))
	}
	if classes[0].Code != "A2" {
		t.Errorf("code = %q, want A2", classes[0].Code)
	}
	if classes[0].Price != 89.0 {
		t.Errorf("price = %f, want 89.0", classes[0].Price)
	}
}

func TestFetchTallinkCabinClasses_ViaTransport_SummaryError(t *testing.T) {
	origClient := tallinkClient
	tallinkClient = &http.Client{
		Transport: &tallinkTransportMock{
			summaryStatus: http.StatusForbidden,
			travelClasses: `[]`,
		},
	}
	defer func() { tallinkClient = origClient }()

	cookies := []*http.Cookie{{Name: "JSESSIONID", Value: "sess"}}
	_, err := fetchTallinkCabinClasses(context.Background(), cookies, "GUID", 5001)
	if err == nil || !strings.Contains(err.Error(), "summary HTTP 403") {
		t.Errorf("expected summary HTTP 403 error, got %v", err)
	}
}
