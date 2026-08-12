package flights

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSerpKeyFuncInjection(t *testing.T) {
	orig := serpKeyFunc
	defer func() { serpKeyFunc = orig }()

	serpKeyFunc = func(context.Context) (string, error) { return "KEY123", nil }
	got, err := serpKeyFunc(context.Background())
	if err != nil || got != "KEY123" {
		t.Fatalf("expected KEY123, got %q err=%v", got, err)
	}

	serpKeyFunc = func(context.Context) (string, error) { return "", errors.New("no keys") }
	if _, err := serpKeyFunc(context.Background()); err == nil {
		t.Fatal("expected error when serp-key fails")
	}
}

func TestSerpKeyCmdDefault(t *testing.T) {
	t.Setenv("TRVL_SERP_KEY_CMD", "")
	if got := serpKeyCmd(); got != "serp-key" {
		t.Fatalf("expected default serp-key, got %q", got)
	}
	t.Setenv("TRVL_SERP_KEY_CMD", "/custom/serp-key")
	if got := serpKeyCmd(); got != "/custom/serp-key" {
		t.Fatalf("expected override, got %q", got)
	}
}

func TestBuildSerpQueryTripTypes(t *testing.T) {
	// One-way → type=2
	q, err := buildSerpQuery([]DuffelSlice{{Origin: "FRA", Destination: "NRT", DepartureDate: "2026-08-15"}}, "K", SearchOptions{Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := url.ParseQuery(q)
	if v.Get("type") != "2" || v.Get("departure_id") != "FRA" || v.Get("arrival_id") != "NRT" || v.Get("outbound_date") != "2026-08-15" {
		t.Fatalf("one-way query wrong: %s", q)
	}
	if v.Get("engine") != "google_flights" || v.Get("currency") != "EUR" {
		t.Fatalf("missing engine/currency: %s", q)
	}

	// Round-trip (mirror slices) → type=1 with return_date
	q, _ = buildSerpQuery([]DuffelSlice{
		{Origin: "FRA", Destination: "NRT", DepartureDate: "2026-08-15"},
		{Origin: "NRT", Destination: "FRA", DepartureDate: "2026-08-29"},
	}, "K", SearchOptions{Currency: "EUR"})
	v, _ = url.ParseQuery(q)
	if v.Get("type") != "1" || v.Get("return_date") != "2026-08-29" {
		t.Fatalf("round-trip query wrong: %s", q)
	}

	// Multi-city (3 legs) → type=3 with multi_city_json
	q, _ = buildSerpQuery([]DuffelSlice{
		{Origin: "FRA", Destination: "NRT", DepartureDate: "2026-08-15"},
		{Origin: "NRT", Destination: "ICN", DepartureDate: "2026-08-20"},
		{Origin: "ICN", Destination: "FRA", DepartureDate: "2026-08-29"},
	}, "K", SearchOptions{Currency: "EUR"})
	v, _ = url.ParseQuery(q)
	if v.Get("type") != "3" || !strings.Contains(v.Get("multi_city_json"), `"departure_id":"FRA"`) {
		t.Fatalf("multi-city query wrong: %s", q)
	}
}

func TestBuildSerpQueryTwoLegNonMirrorIsMultiCity(t *testing.T) {
	q, err := buildSerpQuery([]DuffelSlice{
		{Origin: "FRA", Destination: "NRT", DepartureDate: "2026-08-15"},
		{Origin: "NRT", Destination: "ICN", DepartureDate: "2026-08-20"},
	}, "K", SearchOptions{Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := url.ParseQuery(q)
	if v.Get("type") != "3" {
		t.Fatalf("expected multi-city type=3 for non-mirror 2-leg, got type=%q (query=%s)", v.Get("type"), q)
	}
	if v.Get("return_date") != "" {
		t.Fatalf("non-mirror 2-leg must not set return_date, got %q", v.Get("return_date"))
	}
	if !strings.Contains(v.Get("multi_city_json"), `"departure_id":"NRT"`) {
		t.Fatalf("expected NRT leg in multi_city_json, got %s", v.Get("multi_city_json"))
	}
}

func TestSerpApiUsesPointToPointSlices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back that we received a round-trip query for FRA->NRT.
		if r.URL.Query().Get("type") != "1" || r.URL.Query().Get("return_date") == "" {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write(mustReadRawFixture(t))
	}))
	defer srv.Close()

	origBase, origKey := serpAPIBaseURL, serpKeyFunc
	defer func() { serpAPIBaseURL, serpKeyFunc = origBase, origKey }()
	serpAPIBaseURL = srv.URL
	serpKeyFunc = func(context.Context) (string, error) { return "K", nil }

	slices := duffelSlicesForSearch("FRA", "NRT", "2026-08-15", SearchOptions{ReturnDate: "2026-08-29"})
	res, err := SearchSerpApi(context.Background(), slices, SearchOptions{Currency: "EUR", ReturnDate: "2026-08-29"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res) == 0 || res[0].Provider != "google_serpapi" {
		t.Fatalf("unexpected results: %+v", res)
	}
}

// mustReadRawFixture loads the captured raw Google payload used across the
// SerpApi tests (JFK-LAX 2026-09-15, one-way, USD).
func mustReadRawFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "serpapi_raw_jfklax.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestBuildSerpQuerySendsBagAndFareParams checks the two SerpApi request
// parameters we can actually steer. Per SerpApi docs `bags` is carry-on only
// ("Parameter defines the number of carry-on bags"); there is no checked-bag
// request parameter, so opts.CheckedBags has nothing to map to.
func TestBuildSerpQuerySendsBagAndFareParams(t *testing.T) {
	q, err := buildSerpQuery(
		[]DuffelSlice{{Origin: "JFK", Destination: "LAX", DepartureDate: "2026-09-15"}},
		"K", SearchOptions{Currency: "USD", CarryOnBags: 1, ExcludeBasic: true})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := url.ParseQuery(q)
	if v.Get("bags") != "1" {
		t.Errorf("expected bags=1 from CarryOnBags, got %q (query=%s)", v.Get("bags"), q)
	}
	if v.Get("exclude_basic") != "true" {
		t.Errorf("expected exclude_basic=true, got %q (query=%s)", v.Get("exclude_basic"), q)
	}

	// Unset options must not emit the params at all.
	q, _ = buildSerpQuery(
		[]DuffelSlice{{Origin: "JFK", Destination: "LAX", DepartureDate: "2026-09-15"}},
		"K", SearchOptions{Currency: "USD"})
	v, _ = url.ParseQuery(q)
	if v.Has("bags") || v.Has("exclude_basic") {
		t.Errorf("unset options must not emit params, got %s", q)
	}
}

// TestSearchSerpApiParsesRawGooglePayload covers the output=html path: SerpApi
// returns Google's own batchexecute payload unparsed, so it goes through the
// same decoder the google_flights provider uses. This is what gives SerpApi
// results the bag allowance — SerpApi's normalized JSON drops that field.
//
// Fixture is a real capture (JFK-LAX 2026-09-15, one-way, USD) whose baggage
// terms were confirmed via Google's booking options: no free checked bag.
func TestSearchSerpApiParsesRawGooglePayload(t *testing.T) {
	fixture := mustReadRawFixture(t)
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	origBase, origKey := serpAPIBaseURL, serpKeyFunc
	defer func() { serpAPIBaseURL, serpKeyFunc = origBase, origKey }()
	serpAPIBaseURL = srv.URL
	serpKeyFunc = func(context.Context) (string, error) { return "K", nil }

	res, err := SearchSerpApi(context.Background(),
		[]DuffelSlice{{Origin: "JFK", Destination: "LAX", DepartureDate: "2026-09-15"}},
		SearchOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if gotQuery.Get("output") != "html" {
		t.Errorf("must request the raw payload, got output=%q", gotQuery.Get("output"))
	}
	if len(res) == 0 {
		t.Fatal("expected flights parsed from the raw payload")
	}
	for i, f := range res {
		if f.Provider != "google_serpapi" {
			t.Fatalf("flight %d provider = %q", i, f.Provider)
		}
		if f.Currency == "" {
			t.Fatalf("flight %d has no currency", i)
		}
	}
	// The whole point: bag data now survives.
	if res[0].CheckedBagsIncluded == nil {
		t.Fatal("checked bag allowance must be populated from the raw payload")
	}
	if *res[0].CheckedBagsIncluded != 0 {
		t.Errorf("fixture has no free checked bag, got %d", *res[0].CheckedBagsIncluded)
	}
	if res[0].CarryOnIncluded == nil || !*res[0].CarryOnIncluded {
		t.Errorf("fixture includes a carry-on, got %v", res[0].CarryOnIncluded)
	}
}

// TestSearchSerpApiSurfacesAPIError checks that SerpApi's JSON error envelope is
// still recognised on the output=html path, where success bodies are not JSON.
func TestSearchSerpApiSurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error": "Your account has run out of searches."}`))
	}))
	defer srv.Close()

	origBase, origKey := serpAPIBaseURL, serpKeyFunc
	defer func() { serpAPIBaseURL, serpKeyFunc = origBase, origKey }()
	serpAPIBaseURL = srv.URL
	serpKeyFunc = func(context.Context) (string, error) { return "K", nil }

	_, err := SearchSerpApi(context.Background(),
		[]DuffelSlice{{Origin: "JFK", Destination: "LAX", DepartureDate: "2026-09-15"}},
		SearchOptions{Currency: "USD"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "run out of searches") {
		t.Errorf("error must surface the API message, got %v", err)
	}
}

func TestSearchSerpApiRetriesWithRotatedKey(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.WriteHeader(500) // first attempt fails
			return
		}
		_, _ = w.Write(mustReadRawFixture(t))
	}))
	defer srv.Close()

	origBase, origKey := serpAPIBaseURL, serpKeyFunc
	defer func() { serpAPIBaseURL, serpKeyFunc = origBase, origKey }()
	serpAPIBaseURL = srv.URL
	var keyCalls atomic.Int32
	serpKeyFunc = func(context.Context) (string, error) {
		return fmt.Sprintf("KEY-%d", keyCalls.Add(1)), nil
	}

	res, err := SearchSerpApi(context.Background(),
		[]DuffelSlice{{Origin: "FRA", Destination: "NRT", DepartureDate: "2026-08-15"}},
		SearchOptions{Currency: "EUR"})
	if err != nil {
		t.Fatalf("expected success on retry, got %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected results after the retry")
	}
	if keyCalls.Load() != 2 {
		t.Fatalf("expected 2 serp-key calls (rotated), got %d", keyCalls.Load())
	}
}
