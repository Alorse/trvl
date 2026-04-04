package ground

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLookupNSStation(t *testing.T) {
	tests := []struct {
		city    string
		wantUIC string
		wantOK  bool
	}{
		{"Amsterdam", "8400058", true},
		{"amsterdam", "8400058", true},
		{"AMSTERDAM", "8400058", true},
		{"  Amsterdam  ", "8400058", true},
		{"Rotterdam", "8400530", true},
		{"Den Haag", "8400390", true},
		{"den haag", "8400390", true},
		{"The Hague", "8400390", true},
		{"the hague", "8400390", true},
		{"Utrecht", "8400621", true},
		{"Eindhoven", "8400206", true},
		{"Groningen", "8400263", true},
		{"Maastricht", "8400382", true},
		{"Arnhem", "8400071", true},
		{"Breda", "8400126", true},
		{"Brussels", "8814001", true},
		{"Antwerp", "8821006", true},
		{"Berlin", "8011160", true},
		{"London", "7015400", true},
		{"", "", false},
		{"Nonexistent", "", false},
		{"Tokyo", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.city, func(t *testing.T) {
			station, ok := LookupNSStation(tt.city)
			if ok != tt.wantOK {
				t.Fatalf("LookupNSStation(%q) ok = %v, want %v", tt.city, ok, tt.wantOK)
			}
			if ok && station.UIC != tt.wantUIC {
				t.Errorf("UIC = %q, want %q", station.UIC, tt.wantUIC)
			}
		})
	}
}

func TestLookupNSStation_Metadata(t *testing.T) {
	station, ok := LookupNSStation("Amsterdam")
	if !ok {
		t.Fatal("expected Amsterdam to be found")
	}
	if station.Name != "Amsterdam Centraal" {
		t.Errorf("Name = %q, want %q", station.Name, "Amsterdam Centraal")
	}
	if station.City != "Amsterdam" {
		t.Errorf("City = %q, want %q", station.City, "Amsterdam")
	}
	if station.Country != "NL" {
		t.Errorf("Country = %q, want %q", station.Country, "NL")
	}
	if station.Code != "ASD" {
		t.Errorf("Code = %q, want %q", station.Code, "ASD")
	}
}

func TestLookupNSStation_DenHaagAlias(t *testing.T) {
	// Den Haag and The Hague should resolve to the same station.
	s1, ok1 := LookupNSStation("den haag")
	s2, ok2 := LookupNSStation("the hague")
	if !ok1 || !ok2 {
		t.Fatal("both 'den haag' and 'the hague' should be found")
	}
	if s1.UIC != s2.UIC {
		t.Errorf("UIC mismatch: den haag=%q, the hague=%q", s1.UIC, s2.UIC)
	}
}

func TestHasNSStation(t *testing.T) {
	if !HasNSStation("Amsterdam") {
		t.Error("Amsterdam should have an NS station")
	}
	if HasNSStation("Atlantis") {
		t.Error("Atlantis should not have an NS station")
	}
}

func TestAllNSStationsHaveRequiredFields(t *testing.T) {
	for city, station := range nsStations {
		if station.UIC == "" {
			t.Errorf("station %q has empty UIC", city)
		}
		if station.Name == "" {
			t.Errorf("station %q has empty Name", city)
		}
		if station.City == "" {
			t.Errorf("station %q has empty City", city)
		}
		if station.Country == "" {
			t.Errorf("station %q has empty Country", city)
		}
		if len(station.Country) != 2 {
			t.Errorf("station %q Country %q should be 2 letters", city, station.Country)
		}
		if len(station.UIC) < 7 {
			t.Errorf("station %q UIC %q should be at least 7 digits", city, station.UIC)
		}
	}
}

func TestNSRateLimiterConfiguration(t *testing.T) {
	assertLimiterConfiguration(t, nsLimiter, 12*time.Second, 1)
}

func TestBuildNSBookingURL(t *testing.T) {
	u := buildNSBookingURL("Amsterdam Centraal", "Rotterdam Centraal", "2026-06-18")
	if u == "" {
		t.Fatal("booking URL should not be empty")
	}
	if !strings.Contains(u, "ns.nl") {
		t.Error("should contain ns.nl")
	}
	if !strings.Contains(u, "Amsterdam") {
		t.Error("should contain Amsterdam")
	}
	if !strings.Contains(u, "Rotterdam") {
		t.Error("should contain Rotterdam")
	}
	if !strings.Contains(u, "2026-06-18") {
		t.Error("should contain date")
	}
}

func TestParseNSTrips_Empty(t *testing.T) {
	from := nsStation{UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"}
	to := nsStation{UIC: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL"}
	routes := parseNSTrips(nil, from, to, "EUR")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}
}

func TestParseNSTrips_NoLegs(t *testing.T) {
	from := nsStation{UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"}
	to := nsStation{UIC: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL"}
	// A trip with no legs should be skipped.
	trips := []nsTrip{{Legs: nil}}
	routes := parseNSTrips(trips, from, to, "EUR")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for trip with no legs, got %d", len(routes))
	}
}

func TestParseNSTrips_Basic(t *testing.T) {
	from := nsStation{UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"}
	to := nsStation{UIC: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL"}

	trips := []nsTrip{
		{
			Legs: []nsTripLeg{
				{
					Origin:                   nsStop{Name: "Amsterdam Centraal", UICCode: "8400058"},
					Destination:              nsStop{Name: "Rotterdam Centraal", UICCode: "8400530"},
					TrainCategory:            "Intercity",
					PlannedDepartureDateTime: "2026-06-18T08:02:00+0200",
					PlannedArrivalDateTime:   "2026-06-18T09:07:00+0200",
				},
			},
			OptimalPrice: &nsPrice{TotalPriceInCents: 1590},
			Transfers:    0,
		},
	}

	routes := parseNSTrips(trips, from, to, "EUR")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	r := routes[0]
	if r.Provider != "ns" {
		t.Errorf("Provider = %q, want %q", r.Provider, "ns")
	}
	if r.Type != "train" {
		t.Errorf("Type = %q, want %q", r.Type, "train")
	}
	if r.Currency != "EUR" {
		t.Errorf("Currency = %q, want %q", r.Currency, "EUR")
	}
	if r.Price != 15.90 {
		t.Errorf("Price = %.2f, want 15.90", r.Price)
	}
	if r.Transfers != 0 {
		t.Errorf("Transfers = %d, want 0", r.Transfers)
	}
	if r.Departure.City != "Amsterdam" {
		t.Errorf("Departure.City = %q, want %q", r.Departure.City, "Amsterdam")
	}
	if r.Arrival.City != "Rotterdam" {
		t.Errorf("Arrival.City = %q, want %q", r.Arrival.City, "Rotterdam")
	}
	if r.BookingURL == "" {
		t.Error("BookingURL should not be empty")
	}
	if len(r.Legs) != 1 {
		t.Errorf("Legs = %d, want 1", len(r.Legs))
	}
}

func TestParseNSTrips_MultiLeg(t *testing.T) {
	from := nsStation{UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"}
	to := nsStation{UIC: "8400382", Name: "Maastricht", City: "Maastricht", Country: "NL"}

	trips := []nsTrip{
		{
			Legs: []nsTripLeg{
				{
					Origin:                   nsStop{Name: "Amsterdam Centraal"},
					Destination:              nsStop{Name: "Utrecht Centraal"},
					TrainCategory:            "Intercity",
					PlannedDepartureDateTime: "2026-06-18T08:02:00+0200",
					PlannedArrivalDateTime:   "2026-06-18T08:32:00+0200",
				},
				{
					Origin:                   nsStop{Name: "Utrecht Centraal"},
					Destination:              nsStop{Name: "Maastricht"},
					TrainCategory:            "Intercity Direct",
					PlannedDepartureDateTime: "2026-06-18T08:44:00+0200",
					PlannedArrivalDateTime:   "2026-06-18T10:04:00+0200",
				},
			},
			OptimalPrice: &nsPrice{TotalPriceInCents: 2890},
			Transfers:    1,
		},
	}

	routes := parseNSTrips(trips, from, to, "EUR")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	r := routes[0]
	if r.Transfers != 1 {
		t.Errorf("Transfers = %d, want 1", r.Transfers)
	}
	if len(r.Legs) != 2 {
		t.Errorf("Legs = %d, want 2", len(r.Legs))
	}
	if r.Price != 28.90 {
		t.Errorf("Price = %.2f, want 28.90", r.Price)
	}
}

func TestParseNSTrips_NoPrice(t *testing.T) {
	from := nsStation{UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"}
	to := nsStation{UIC: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL"}

	trips := []nsTrip{
		{
			Legs: []nsTripLeg{
				{
					Origin:                   nsStop{Name: "Amsterdam Centraal"},
					Destination:              nsStop{Name: "Rotterdam Centraal"},
					PlannedDepartureDateTime: "2026-06-18T08:02:00+0200",
					PlannedArrivalDateTime:   "2026-06-18T09:07:00+0200",
				},
			},
			OptimalPrice: nil, // no price
		},
	}

	routes := parseNSTrips(trips, from, to, "EUR")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Price != 0 {
		t.Errorf("Price = %.2f, want 0", routes[0].Price)
	}
}

func TestParseNSTrips_FallbackToPriceInCents(t *testing.T) {
	from := nsStation{UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"}
	to := nsStation{UIC: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL"}

	trips := []nsTrip{
		{
			Legs: []nsTripLeg{
				{
					Origin:                   nsStop{Name: "Amsterdam Centraal"},
					Destination:              nsStop{Name: "Rotterdam Centraal"},
					PlannedDepartureDateTime: "2026-06-18T08:02:00+0200",
					PlannedArrivalDateTime:   "2026-06-18T09:07:00+0200",
				},
			},
			// TotalPriceInCents is 0, fall back to PriceInCents.
			OptimalPrice: &nsPrice{TotalPriceInCents: 0, PriceInCents: 1590},
		},
	}

	routes := parseNSTrips(trips, from, to, "EUR")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Price != 15.90 {
		t.Errorf("Price = %.2f, want 15.90", routes[0].Price)
	}
}

func TestParseNSTrips_CurrencyUppercased(t *testing.T) {
	from := nsStation{UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL"}
	to := nsStation{UIC: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL"}

	trips := []nsTrip{
		{
			Legs: []nsTripLeg{
				{
					Origin:                   nsStop{Name: "Amsterdam Centraal"},
					Destination:              nsStop{Name: "Rotterdam Centraal"},
					PlannedDepartureDateTime: "2026-06-18T08:02:00+0200",
					PlannedArrivalDateTime:   "2026-06-18T09:07:00+0200",
				},
			},
			OptimalPrice: &nsPrice{TotalPriceInCents: 1000},
		},
	}

	routes := parseNSTrips(trips, from, to, "eur")
	if routes[0].Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", routes[0].Currency)
	}
}

func TestDate_ExtractFromISO(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-06-18T08:02:00+0200", "2026-06-18"},
		{"2026-06-18", "2026-06-18"},
		{"2026-06-18T08:02:00Z", "2026-06-18"},
		{"short", "short"},
		{"", ""},
	}

	for _, tt := range tests {
		got := date(tt.input)
		if got != tt.want {
			t.Errorf("date(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSearchNS_UnknownFrom(t *testing.T) {
	ctx := context.Background()
	_, err := SearchNS(ctx, "Nonexistent", "Amsterdam", "2026-06-18", "EUR")
	if err == nil {
		t.Error("expected error for unknown from station")
	}
}

func TestSearchNS_UnknownTo(t *testing.T) {
	ctx := context.Background()
	_, err := SearchNS(ctx, "Amsterdam", "Nonexistent", "2026-06-18", "EUR")
	if err == nil {
		t.Error("expected error for unknown to station")
	}
}

func TestSearchNS_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 2, 0).Format("2006-01-02")

	routes, err := SearchNS(ctx, "Amsterdam", "Rotterdam", date, "EUR")
	if err != nil {
		t.Skipf("NS API unavailable (expected in CI): %v", err)
	}
	if len(routes) == 0 {
		t.Skip("no NS routes found (may be outside booking window)")
	}

	r := routes[0]
	if r.Provider != "ns" {
		t.Errorf("provider = %q, want ns", r.Provider)
	}
	if r.Type != "train" {
		t.Errorf("type = %q, want train", r.Type)
	}
	if r.Duration <= 0 {
		t.Errorf("duration = %d, should be > 0", r.Duration)
	}
	if r.Departure.City != "Amsterdam" {
		t.Errorf("departure city = %q, want Amsterdam", r.Departure.City)
	}
	if r.Arrival.City != "Rotterdam" {
		t.Errorf("arrival city = %q, want Rotterdam", r.Arrival.City)
	}
	if r.BookingURL == "" {
		t.Error("booking URL should not be empty")
	}
}
