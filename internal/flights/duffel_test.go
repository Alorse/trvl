package flights

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

const duffelFixture = `{"data":{"offers":[
 {"total_amount":"881.00","total_currency":"USD","total_emissions_kg":"1200","owner":{"iata_code":"BR","name":"EVA Air"},
  "slices":[
   {"origin":{"iata_code":"HAM","name":"Hamburg"},"destination":{"iata_code":"FUK","name":"Fukuoka"},"duration":"P1DT4H55M",
    "segments":[
     {"origin":{"iata_code":"HAM","name":"Hamburg"},"destination":{"iata_code":"MUC","name":"Munich"},
      "departing_at":"2026-09-15T06:25:00","arriving_at":"2026-09-15T07:45:00","duration":"PT1H20M",
      "marketing_carrier":{"iata_code":"EW","name":"Eurowings"},"marketing_carrier_flight_number":"7170",
      "aircraft":{"name":"Airbus A320"},
      "passengers":[{"baggages":[{"type":"checked","quantity":2},{"type":"carry_on","quantity":1}]}]},
     {"origin":{"iata_code":"MUC","name":"Munich"},"destination":{"iata_code":"FUK","name":"Fukuoka"},
      "departing_at":"2026-09-15T12:00:00","arriving_at":"2026-09-16T11:20:00","duration":"PT12H35M",
      "marketing_carrier":{"iata_code":"BR","name":"EVA Air"},"marketing_carrier_flight_number":"72",
      "aircraft":{"name":"Boeing 777-300ER"},
      "passengers":[{"baggages":[{"type":"checked","quantity":2},{"type":"carry_on","quantity":1}]}]}
    ]}
  ]}
]}}`

func TestSearchDuffel_MapsOffer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Errorf("missing Authorization header")
		}
		if r.Header.Get("Duffel-Version") != "v2" {
			t.Errorf("Duffel-Version = %q, want v2", r.Header.Get("Duffel-Version"))
		}
		// Assert the request body carries the slices we passed in.
		var req struct {
			Data struct {
				Slices []map[string]string `json:"slices"`
			} `json:"data"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
		if len(req.Data.Slices) != 1 {
			t.Errorf("slices in request = %d, want 1", len(req.Data.Slices))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(duffelFixture))
	}))
	defer srv.Close()

	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "test_key")
	restore := duffelSetEndpointForTest(srv.URL)
	defer restore()

	got, err := SearchDuffel(context.Background(),
		[]DuffelSlice{{Origin: "HAM", Destination: "FUK", DepartureDate: "2026-09-15"}},
		SearchOptions{Adults: 1, CabinClass: models.Economy})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("results = %d, want 1", len(got))
	}
	f := got[0]
	if f.Price != 881.00 {
		t.Errorf("price = %v, want 881.00", f.Price)
	}
	if f.Currency != "USD" {
		t.Errorf("currency = %q, want USD", f.Currency)
	}
	if f.Provider != "duffel" {
		t.Errorf("provider = %q, want duffel", f.Provider)
	}
	if f.BookingURL != "" {
		t.Errorf("booking_url = %q, want empty", f.BookingURL)
	}
	if len(f.Legs) != 2 {
		t.Fatalf("legs = %d, want 2", len(f.Legs))
	}
	if f.Stops != 1 { // 2 segments in 1 slice → 1 stop
		t.Errorf("stops = %d, want 1", f.Stops)
	}
	if f.Duration != parseISO8601Duration("P1DT4H55M") {
		t.Errorf("duration = %d, want %d", f.Duration, parseISO8601Duration("P1DT4H55M"))
	}
	if f.Emissions != 1200000 { // 1200 kg * 1000
		t.Errorf("emissions = %d, want 1200000", f.Emissions)
	}
	if f.CarryOnIncluded == nil || !*f.CarryOnIncluded {
		t.Errorf("carry_on_included = %v, want true", f.CarryOnIncluded)
	}
	if f.CheckedBagsIncluded == nil || *f.CheckedBagsIncluded != 2 {
		t.Errorf("checked_bags_included = %v, want 2", f.CheckedBagsIncluded)
	}
	if f.Legs[0].AirlineCode != "EW" || f.Legs[0].FlightNumber != "7170" {
		t.Errorf("leg0 carrier = %q%q, want EW7170", f.Legs[0].AirlineCode, f.Legs[0].FlightNumber)
	}
	if f.Legs[1].Aircraft != "Boeing 777-300ER" {
		t.Errorf("leg1 aircraft = %q", f.Legs[1].Aircraft)
	}
}

func TestSearchDuffel_DisabledWithoutKey(t *testing.T) {
	t.Setenv("DUFFEL_API_KEYS", "")
	t.Setenv("DUFFEL_API_KEY", "")
	if DuffelEnabled() {
		t.Fatalf("DuffelEnabled() = true with no keys")
	}
	_, err := SearchDuffel(context.Background(),
		[]DuffelSlice{{Origin: "HAM", Destination: "FUK", DepartureDate: "2026-09-15"}},
		SearchOptions{Adults: 1})
	if err == nil {
		t.Fatalf("expected error when no Duffel keys are set")
	}
}

func TestDuffelSlicesForSearch(t *testing.T) {
	oneway := duffelSlicesForSearch("HAM", "FUK", "2026-09-15", SearchOptions{})
	if len(oneway) != 1 {
		t.Fatalf("one-way slices = %d, want 1", len(oneway))
	}
	if oneway[0].Origin != "HAM" || oneway[0].Destination != "FUK" || oneway[0].DepartureDate != "2026-09-15" {
		t.Errorf("one-way slice = %+v", oneway[0])
	}

	rt := duffelSlicesForSearch("HAM", "FUK", "2026-09-15", SearchOptions{ReturnDate: "2026-09-28"})
	if len(rt) != 2 {
		t.Fatalf("round-trip slices = %d, want 2", len(rt))
	}
	if rt[1].Origin != "FUK" || rt[1].Destination != "HAM" || rt[1].DepartureDate != "2026-09-28" {
		t.Errorf("return slice = %+v", rt[1])
	}
}

func TestDuffelSlicesForLegs(t *testing.T) {
	legs := []Leg{
		{Origins: []string{"HAM"}, Destinations: []string{"FUK"}, Date: "2026-09-15"},
		{Origins: []string{"NRT"}, Destinations: []string{"HAM"}, Date: "2026-09-28"},
	}
	slices := duffelSlicesForLegs(legs)
	if len(slices) != 2 {
		t.Fatalf("slices = %d, want 2", len(slices))
	}
	if slices[0].Origin != "HAM" || slices[0].Destination != "FUK" || slices[0].DepartureDate != "2026-09-15" {
		t.Errorf("slice0 = %+v", slices[0])
	}
	if slices[1].Origin != "NRT" || slices[1].Destination != "HAM" || slices[1].DepartureDate != "2026-09-28" {
		t.Errorf("slice1 = %+v", slices[1])
	}
}

func TestSearchDuffel_FailoverToNextKey(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First key (whichever round-robin picks first) gets 401; retry with the
		// next key succeeds.
		if calls == 1 {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"errors":[{"title":"Unauthorized"}]}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(duffelFixture))
	}))
	defer srv.Close()

	t.Setenv("DUFFEL_API_KEYS", "bad,good")
	restore := duffelSetEndpointForTest(srv.URL)
	defer restore()

	got, err := SearchDuffel(context.Background(),
		[]DuffelSlice{{Origin: "HAM", Destination: "FUK", DepartureDate: "2026-09-15"}},
		SearchOptions{Adults: 1, CabinClass: models.Economy})
	if err != nil {
		t.Fatalf("expected failover to succeed, got error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("results after failover = %d, want 1", len(got))
	}
	if calls != 2 {
		t.Fatalf("server calls = %d, want 2 (1 failed key + 1 success)", calls)
	}
}
