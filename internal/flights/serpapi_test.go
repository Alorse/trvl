package flights

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

const serpOneWayFixture = `{
  "best_flights": [{
    "price": 767,
    "type": "One way",
    "total_duration": 1160,
    "carbon_emissions": {"this_flight": 936000},
    "layovers": [{"duration": 185, "name": "Dubai International Airport", "id": "DXB"}],
    "flights": [
      {"departure_airport": {"name": "Frankfurt Airport", "id": "FRA", "time": "2026-08-15 15:15"},
       "arrival_airport": {"name": "Dubai International Airport", "id": "DXB", "time": "2026-08-15 23:35"},
       "duration": 380, "airline": "Emirates", "flight_number": "EK 46", "airplane": "Airbus A380", "travel_class": "Economy"},
      {"departure_airport": {"name": "Dubai International Airport", "id": "DXB", "time": "2026-08-16 02:40"},
       "arrival_airport": {"name": "Narita International Airport", "id": "NRT", "time": "2026-08-16 17:35"},
       "duration": 595, "airline": "Emirates", "flight_number": "EK 318", "airplane": "Airbus A380", "travel_class": "Economy"}
    ]
  }],
  "other_flights": []
}`

func TestSearchSerpApiParsesFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(serpOneWayFixture))
	}))
	defer srv.Close()

	origBase, origKey := serpAPIBaseURL, serpKeyFunc
	defer func() { serpAPIBaseURL, serpKeyFunc = origBase, origKey }()
	serpAPIBaseURL = srv.URL
	serpKeyFunc = func(context.Context) (string, error) { return "K", nil }

	res, err := SearchSerpApi(context.Background(),
		[]DuffelSlice{{Origin: "FRA", Destination: "NRT", DepartureDate: "2026-08-15"}},
		SearchOptions{Currency: "EUR"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 option, got %d", len(res))
	}
	f := res[0]
	if f.Price != 767 || f.Currency != "EUR" {
		t.Fatalf("price/currency: %v %q", f.Price, f.Currency)
	}
	if f.Provider != "google_serpapi" {
		t.Fatalf("provider: %q", f.Provider)
	}
	if f.Duration != 1160 || f.Stops != 1 {
		t.Fatalf("duration/stops: %d %d", f.Duration, f.Stops)
	}
	if f.Emissions != 936000 {
		t.Fatalf("emissions (grams): %d", f.Emissions)
	}
	if len(f.Legs) != 2 {
		t.Fatalf("legs: %d", len(f.Legs))
	}
	if f.Legs[0].AirlineCode != "EK" || f.Legs[0].FlightNumber != "EK 46" {
		t.Fatalf("leg0 airline: %q %q", f.Legs[0].AirlineCode, f.Legs[0].FlightNumber)
	}
	if f.Legs[1].LayoverMinutes != 185 {
		t.Fatalf("leg1 layover: %d", f.Legs[1].LayoverMinutes)
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
		_, _ = w.Write([]byte(serpOneWayFixture))
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
	if len(res) != 1 || res[0].Provider != "google_serpapi" {
		t.Fatalf("unexpected results: %+v", res)
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
		_, _ = w.Write([]byte(serpOneWayFixture))
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
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if keyCalls.Load() != 2 {
		t.Fatalf("expected 2 serp-key calls (rotated), got %d", keyCalls.Load())
	}
}
