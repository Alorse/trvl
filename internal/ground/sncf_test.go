package ground

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLookupSNCFStation(t *testing.T) {
	tests := []struct {
		city     string
		wantCode string
		wantOK   bool
	}{
		{"Paris", "FRPAR", true},
		{"paris", "FRPAR", true},
		{"PARIS", "FRPAR", true},
		{"  Paris  ", "FRPAR", true},
		{"Lyon", "FRLYS", true},
		{"Marseille", "FRMRS", true},
		{"Bordeaux", "FRBOJ", true},
		{"Nice", "FRNIC", true},
		{"Strasbourg", "FRSBG", true},
		{"Lille", "FRLIL", true},
		{"Toulouse", "FRTLS", true},
		{"Nantes", "FRNTE", true},
		{"Paris Nord", "FRPNO", true},
		{"Paris Gare de Lyon", "FRPLY", true},
		{"Paris Montparnasse", "FRPMO", true},
		{"Brussels", "BEBMI", true},
		{"Geneva", "CHGVA", true},
		{"Barcelona", "ESBCN", true},
		{"London", "GBSPX", true},
		{"Prague", "", false},
		{"", "", false},
		{"Nonexistent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.city, func(t *testing.T) {
			station, ok := LookupSNCFStation(tt.city)
			if ok != tt.wantOK {
				t.Fatalf("LookupSNCFStation(%q) ok = %v, want %v", tt.city, ok, tt.wantOK)
			}
			if ok && station.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", station.Code, tt.wantCode)
			}
		})
	}
}

func TestLookupSNCFStation_Metadata(t *testing.T) {
	station, ok := LookupSNCFStation("Lyon")
	if !ok {
		t.Fatal("expected Lyon to be found")
	}
	if station.Name != "Lyon Part-Dieu" {
		t.Errorf("Name = %q, want %q", station.Name, "Lyon Part-Dieu")
	}
	if station.City != "Lyon" {
		t.Errorf("City = %q, want %q", station.City, "Lyon")
	}
	if station.Country != "FR" {
		t.Errorf("Country = %q, want %q", station.Country, "FR")
	}
}

func TestHasSNCFRoute(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{"Paris", "Lyon", true},       // Both French
		{"Paris", "Brussels", true},   // One French
		{"Brussels", "Paris", true},   // One French (reversed)
		{"Brussels", "Geneva", false}, // Neither French
		{"Paris", "Prague", false},    // Prague not in station list
		{"", "Paris", false},
		{"Paris", "", false},
	}

	for _, tt := range tests {
		name := tt.from + "->" + tt.to
		t.Run(name, func(t *testing.T) {
			got := HasSNCFRoute(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("HasSNCFRoute(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestAllSNCFStationsHaveRequiredFields(t *testing.T) {
	for city, station := range sncfStations {
		if station.Code == "" {
			t.Errorf("station %q has empty Code", city)
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
		if len(station.Code) != 5 {
			t.Errorf("station %q Code %q should be 5 characters", city, station.Code)
		}
	}
}

func TestSNCFStationCodesAreUppercase(t *testing.T) {
	for city, station := range sncfStations {
		if station.Code != strings.ToUpper(station.Code) {
			t.Errorf("station %q Code %q should be uppercase", city, station.Code)
		}
	}
}

func TestBuildSNCFBookingURL(t *testing.T) {
	u := buildSNCFBookingURL("FRPAR", "FRLYS", "2026-04-10")
	if u == "" {
		t.Fatal("booking URL should not be empty")
	}
	if !strings.Contains(u, "sncf-connect.com") {
		t.Error("should contain sncf-connect.com")
	}
	if !strings.Contains(u, "FRPAR") {
		t.Error("should contain origin code")
	}
	if !strings.Contains(u, "FRLYS") {
		t.Error("should contain destination code")
	}
	if !strings.Contains(u, "2026-04-10") {
		t.Error("should contain date")
	}
}

func TestSNCFRateLimiterConfiguration(t *testing.T) {
	assertLimiterConfiguration(t, sncfLimiter, 6*time.Second, 1)
}

func TestSearchSNCF_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	routes, err := SearchSNCF(ctx, "Paris", "Lyon", date, "EUR")
	if err != nil {
		// The SNCF API may be behind Cloudflare or temporarily unavailable.
		t.Skipf("SNCF API unavailable (expected in CI): %v", err)
	}
	if len(routes) == 0 {
		t.Skip("no SNCF routes returned (may be outside booking window)")
	}

	r := routes[0]
	if r.Provider != "sncf" {
		t.Errorf("provider = %q, want sncf", r.Provider)
	}
	if r.Type != "train" {
		t.Errorf("type = %q, want train", r.Type)
	}
	if r.Price <= 0 {
		t.Errorf("price = %f, should be > 0", r.Price)
	}
	if r.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", r.Currency)
	}
	if r.BookingURL == "" {
		t.Error("booking URL should not be empty")
	}
}

func TestSearchSNCF_UnknownStation(t *testing.T) {
	ctx := context.Background()
	_, err := SearchSNCF(ctx, "Nonexistent", "Lyon", "2026-04-10", "EUR")
	if err == nil {
		t.Error("expected error for unknown station")
	}
}
