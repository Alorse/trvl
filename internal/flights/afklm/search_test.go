package afklm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestAvailableOffersHappyPath(t *testing.T) {
	fixture, err := os.ReadFile("testdata/available_offers_ams_prg.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		w.WriteHeader(200)
		w.Write(fixture)
	}))
	defer srv.Close()

	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	client := newTestClient(t, srv, func() time.Time { return now })

	req := AvailableOffersRequest{
		BookingFlow:  "LEISURE",
		Passengers:   []Passenger{{ID: 1, Type: "ADT"}},
		RequestedConnections: []RequestedConnection{{
			DepartureDate: "2026-05-15",
			Origin:        Place{Type: "AIRPORT", Code: "AMS"},
			Destination:   Place{Type: "AIRPORT", Code: "PRG"},
		}},
		Currency: "EUR",
	}

	resp, stale, err := client.AvailableOffers(context.Background(), req)
	if err != nil {
		t.Fatalf("AvailableOffers: %v", err)
	}
	if stale {
		t.Error("expected non-stale first call")
	}
	if len(resp.Recommendations) != 2 {
		t.Errorf("expected 2 recommendations, got %d", len(resp.Recommendations))
	}
	// Verify parsed price.
	price := resp.Recommendations[0].FlightProducts[0].Connections[0].Price.DisplayPrice
	if price != 89.0 {
		t.Errorf("expected price 89.0, got %v", price)
	}
}

func TestAvailableOffersServedFromCache(t *testing.T) {
	fixture, _ := os.ReadFile("testdata/available_offers_ams_prg.json")
	var serverCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "application/hal+json")
		w.WriteHeader(200)
		w.Write(fixture)
	}))
	defer srv.Close()

	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	client := newTestClient(t, srv, func() time.Time { return now })

	req := AvailableOffersRequest{
		BookingFlow:  "LEISURE",
		Passengers:   []Passenger{{ID: 1, Type: "ADT"}},
		RequestedConnections: []RequestedConnection{{
			DepartureDate: "2026-05-15",
			Origin:        Place{Type: "AIRPORT", Code: "AMS"},
			Destination:   Place{Type: "AIRPORT", Code: "PRG"},
		}},
	}

	client.AvailableOffers(context.Background(), req)
	client.AvailableOffers(context.Background(), req) // should hit cache

	if serverCalls != 1 {
		t.Errorf("expected 1 server call (2nd should be cached), got %d", serverCalls)
	}
}

func TestDaysUntilDepartureTTLTable(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		daysStr string
		days    int
		wantTTL time.Duration
	}{
		{"2026-06-05", 45, 72 * time.Hour}, // 45 days
		{"2026-05-11", 20, 24 * time.Hour}, // 20 days
		{"2026-05-01", 10, 12 * time.Hour}, // 10 days
		{"2026-04-26", 5, 6 * time.Hour},   // 5 days
		{"2026-04-22", 1, 2 * time.Hour},   // 1 day
	}

	for _, tc := range cases {
		days := daysFromISO(tc.daysStr, now)
		if days != tc.days {
			t.Errorf("daysFromISO(%s): got %d, want %d", tc.daysStr, days, tc.days)
		}
		ttl := DepArrTTL(days)
		if ttl != tc.wantTTL {
			t.Errorf("TTL for %s (%d days): got %v, want %v", tc.daysStr, days, ttl, tc.wantTTL)
		}
	}
}
