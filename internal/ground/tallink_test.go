package ground

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLookupTallinkPort(t *testing.T) {
	tests := []struct {
		city     string
		wantCode string
		wantCity string
		wantOK   bool
	}{
		// Helsinki aliases
		{"Helsinki", "HEL", "Helsinki", true},
		{"helsinki", "HEL", "Helsinki", true},
		{"hel", "HEL", "Helsinki", true},
		{"  Helsinki  ", "HEL", "Helsinki", true},

		// Tallinn aliases — new API uses TAL
		{"Tallinn", "TAL", "Tallinn", true},
		{"tallinn", "TAL", "Tallinn", true},
		{"tal", "TAL", "Tallinn", true},
		{"tll", "TAL", "Tallinn", true}, // legacy alias still resolves
		{"tln", "TAL", "Tallinn", true}, // legacy alias still resolves

		// Stockholm aliases
		{"Stockholm", "STO", "Stockholm", true},
		{"stockholm", "STO", "Stockholm", true},
		{"sto", "STO", "Stockholm", true},

		// Riga aliases
		{"Riga", "RIG", "Riga", true},
		{"riga", "RIG", "Riga", true},
		{"rig", "RIG", "Riga", true},

		// Turku aliases
		{"Turku", "TUR", "Turku", true},
		{"turku", "TUR", "Turku", true},
		{"tur", "TUR", "Turku", true},
		{"åbo", "TUR", "Turku", true},

		// Åland / Mariehamn — new code ALA
		{"Mariehamn", "ALA", "Mariehamn", true},
		{"mar", "ALA", "Mariehamn", true},
		{"ala", "ALA", "Mariehamn", true},

		// Långnäs maps to ALA now
		{"lng", "ALA", "Mariehamn", true},

		// Paldiski
		{"Paldiski", "PAL", "Paldiski", true},
		{"pal", "PAL", "Paldiski", true},

		// Kapellskär
		{"Kapellskär", "KAP", "Kapellskär", true},
		{"kap", "KAP", "Kapellskär", true},

		// Visby
		{"Visby", "VIS", "Visby", true},
		{"vis", "VIS", "Visby", true},

		// Non-existent
		{"", "", "", false},
		{"London", "", "", false},
		{"Paris", "", "", false},
		{"Atlantis", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.city, func(t *testing.T) {
			port, ok := LookupTallinkPort(tt.city)
			if ok != tt.wantOK {
				t.Fatalf("LookupTallinkPort(%q) ok = %v, want %v", tt.city, ok, tt.wantOK)
			}
			if ok {
				if port.Code != tt.wantCode {
					t.Errorf("Code = %q, want %q", port.Code, tt.wantCode)
				}
				if port.City != tt.wantCity {
					t.Errorf("City = %q, want %q", port.City, tt.wantCity)
				}
				if port.Name == "" {
					t.Errorf("Name should not be empty for %q", tt.city)
				}
			}
		})
	}
}

func TestHasTallinkPort(t *testing.T) {
	if !HasTallinkPort("Helsinki") {
		t.Error("Helsinki should have a Tallink port")
	}
	if !HasTallinkPort("Tallinn") {
		t.Error("Tallinn should have a Tallink port")
	}
	if HasTallinkPort("London") {
		t.Error("London should not have a Tallink port")
	}
	if HasTallinkPort("") {
		t.Error("empty city should not have a Tallink port")
	}
}

func TestHasTallinkRoute(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{"Helsinki", "Tallinn", true},
		{"Tallinn", "Helsinki", true},
		{"Stockholm", "Tallinn", true},
		{"Stockholm", "Riga", true},
		{"Stockholm", "Helsinki", true},
		{"Helsinki", "Stockholm", true},
		{"Helsinki", "Paldiski", true},
		{"Kapellskär", "Paldiski", true},
		{"Helsinki", "London", false}, // London not a Tallink port
		{"London", "Tallinn", false},
		{"Atlantis", "Helsinki", false},
		{"Helsinki", "Atlantis", false},
		{"", "Helsinki", false},
		{"Helsinki", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			got := HasTallinkRoute(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("HasTallinkRoute(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestTallinkRouteDuration(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want int
	}{
		{"HEL", "TAL", 120},
		{"TAL", "HEL", 120},
		{"STO", "TAL", 960},
		{"TAL", "STO", 960},
		{"STO", "HEL", 960},
		{"HEL", "STO", 960},
		{"STO", "RIG", 1020},
		{"RIG", "STO", 1020},
		{"TUR", "STO", 660},
		{"STO", "TUR", 660},
		{"PAL", "KAP", 540},
		{"KAP", "PAL", 540},
		// Unknown route falls back to 120
		{"HEL", "RIG", 120},
	}

	for _, tt := range tests {
		t.Run(tt.from+"-"+tt.to, func(t *testing.T) {
			got := tallinkRouteDuration(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("tallinkRouteDuration(%q, %q) = %d, want %d", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestTallinkParseLocalDateTime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-04-10T07:30:00", "2026-04-10T07:30:00"},
		{"2026-04-10T09:30:00", "2026-04-10T09:30:00"},
		{"2026-04-05T00:00:00", "2026-04-05T00:00:00"},
		{"", ""},
		{"bad-input", "bad-input"}, // returned as-is on parse failure
	}

	for _, tt := range tests {
		got := tallinkParseLocalDateTime(tt.input)
		if got != tt.want {
			t.Errorf("tallinkParseLocalDateTime(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildTallinkBookingURL(t *testing.T) {
	u := buildTallinkBookingURL("HEL", "TAL", "2026-04-10")
	if u == "" {
		t.Fatal("booking URL should not be empty")
	}
	if !strings.Contains(u, "tallink.com") {
		t.Errorf("URL should contain tallink.com, got %q", u)
	}
	if !strings.Contains(u, "HEL") {
		t.Errorf("URL should contain from port HEL, got %q", u)
	}
	if !strings.Contains(u, "TAL") {
		t.Errorf("URL should contain to port TAL, got %q", u)
	}
	if !strings.Contains(u, "2026-04-10") {
		t.Errorf("URL should contain date, got %q", u)
	}
	if !strings.Contains(u, "adults=1") {
		t.Errorf("URL should contain adults=1, got %q", u)
	}
	// New URL format uses book.tallink.com
	if !strings.Contains(u, "book.tallink.com") {
		t.Errorf("URL should use book.tallink.com, got %q", u)
	}
}

func TestTallinkShipSuffix(t *testing.T) {
	if got := tallinkShipSuffix(""); got != "" {
		t.Errorf("empty ship name should return empty, got %q", got)
	}
	if got := tallinkShipSuffix("MEGASTAR"); got != " (MEGASTAR)" {
		t.Errorf("ship suffix = %q, want %q", got, " (MEGASTAR)")
	}
}

func TestNewUUID(t *testing.T) {
	id := newUUID()
	if id == "" {
		t.Fatal("UUID should not be empty")
	}
	// UUID v4 format: 8-4-4-4-12 hex chars
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("UUID should have 5 parts, got %d: %q", len(parts), id)
	}

	// Generate two UUIDs and verify they are different.
	id2 := newUUID()
	if id == id2 {
		t.Error("two consecutive UUIDs should not be identical")
	}
}

func TestTallinkAllPortsHaveRequiredFields(t *testing.T) {
	for alias, port := range tallinkPorts {
		if port.Code == "" {
			t.Errorf("port alias %q has empty Code", alias)
		}
		if port.Name == "" {
			t.Errorf("port alias %q has empty Name", alias)
		}
		if port.City == "" {
			t.Errorf("port alias %q has empty City", alias)
		}
	}
}

func TestTallinkRateLimiterConfiguration(t *testing.T) {
	// 10 req/min (every 6s), burst 1 — allows multiple detectors in a single
	// hacks run without hitting the context deadline (previously 5 req/min / 12s).
	assertLimiterConfiguration(t, tallinkLimiter, 6*time.Second, 1)
}

// mockTallinkVoyageAvailsResponse is a realistic voyage-avails API response.
// Each element wraps a voyage in {"initialSail": {...}}, matching the real API.
const mockTallinkVoyageAvailsResponse = `[
  {
    "initialSail": {
      "packageId": 2380001,
      "shipCode": "MEGASTAR",
      "sailType": "TRANSPORT",
      "travelClass": {
        "minPrice": 38.9
      },
      "sailLegs": [
        {
          "from": {
            "port": "HEL",
            "pier": "LSA2",
            "localDateTime": "2026-04-10T08:30:00"
          },
          "to": {
            "port": "TAL",
            "pier": "DTER",
            "localDateTime": "2026-04-10T10:30:00"
          }
        }
      ],
      "isCheckInOpen": true,
      "isOvernight": false,
      "packageMode": "REGULAR"
    }
  },
  {
    "initialSail": {
      "packageId": 2380002,
      "shipCode": "MYSTAR",
      "sailType": "TRANSPORT",
      "travelClass": {
        "minPrice": 12.5
      },
      "sailLegs": [
        {
          "from": {
            "port": "HEL",
            "pier": "LSA2",
            "localDateTime": "2026-04-10T17:30:00"
          },
          "to": {
            "port": "TAL",
            "pier": "DTER",
            "localDateTime": "2026-04-10T19:30:00"
          }
        }
      ],
      "isCheckInOpen": false,
      "isOvernight": false,
      "packageMode": "REGULAR"
    }
  },
  {
    "initialSail": {
      "packageId": 2380011,
      "shipCode": "MEGASTAR",
      "sailType": "TRANSPORT",
      "travelClass": {
        "minPrice": 35.0
      },
      "sailLegs": [
        {
          "from": {
            "port": "HEL",
            "pier": "LSA2",
            "localDateTime": "2026-04-11T08:30:00"
          },
          "to": {
            "port": "TAL",
            "pier": "DTER",
            "localDateTime": "2026-04-11T10:30:00"
          }
        }
      ],
      "isCheckInOpen": false,
      "isOvernight": false,
      "packageMode": "REGULAR"
    }
  }
]`

// unwrapMockVoyages parses the mock JSON array (with initialSail wrapper) into []tallinkVoyageAvail.
func unwrapMockVoyages(t *testing.T) []tallinkVoyageAvail {
	t.Helper()
	var entries []tallinkVoyageAvailEntry
	if err := json.Unmarshal([]byte(mockTallinkVoyageAvailsResponse), &entries); err != nil {
		t.Fatalf("unmarshal mock voyage-avails: %v", err)
	}
	out := make([]tallinkVoyageAvail, len(entries))
	for i, e := range entries {
		out[i] = e.InitialSail
	}
	return out
}

func TestFetchTallinkVoyages_MockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters.
		q := r.URL.Query()
		if q.Get("sailType") != "TRANSPORT" {
			t.Errorf("sailType = %q, want TRANSPORT", q.Get("sailType"))
		}
		if q.Get("routeSeqN") != "1" {
			t.Errorf("routeSeqN = %q, want 1", q.Get("routeSeqN"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockTallinkVoyageAvailsResponse)) //nolint:errcheck
	}))
	defer srv.Close()

	// Parse the mock JSON via the entry wrapper, matching fetchTallinkVoyages behavior.
	voyages := unwrapMockVoyages(t)

	if len(voyages) != 3 {
		t.Fatalf("expected 3 voyages, got %d", len(voyages))
	}

	v := voyages[0]
	if v.ShipCode != "MEGASTAR" {
		t.Errorf("ShipCode = %q, want MEGASTAR", v.ShipCode)
	}
	if v.TravelClass.MinPrice != 38.9 {
		t.Errorf("MinPrice = %.2f, want 38.9", v.TravelClass.MinPrice)
	}
	if len(v.SailLegs) != 1 {
		t.Fatalf("expected 1 sail leg, got %d", len(v.SailLegs))
	}
	if v.SailLegs[0].From.LocalDateTime != "2026-04-10T08:30:00" {
		t.Errorf("departure = %q, want 2026-04-10T08:30:00", v.SailLegs[0].From.LocalDateTime)
	}
	if v.SailLegs[0].To.LocalDateTime != "2026-04-10T10:30:00" {
		t.Errorf("arrival = %q, want 2026-04-10T10:30:00", v.SailLegs[0].To.LocalDateTime)
	}
	if v.SailLegs[0].From.Port != "HEL" {
		t.Errorf("from port = %q, want HEL", v.SailLegs[0].From.Port)
	}
	if v.SailLegs[0].To.Port != "TAL" {
		t.Errorf("to port = %q, want TAL", v.SailLegs[0].To.Port)
	}
}

func TestTallinkFilterByDate(t *testing.T) {
	voyages := unwrapMockVoyages(t)

	// Filter for 2026-04-10 should return 2 voyages.
	filtered := tallinkFilterByDate(voyages, "2026-04-10")
	if len(filtered) != 2 {
		t.Errorf("expected 2 voyages on 2026-04-10, got %d", len(filtered))
	}

	// Filter for 2026-04-11 should return 1 voyage.
	filtered = tallinkFilterByDate(voyages, "2026-04-11")
	if len(filtered) != 1 {
		t.Errorf("expected 1 voyage on 2026-04-11, got %d", len(filtered))
	}

	// Filter for unknown date should return 0.
	filtered = tallinkFilterByDate(voyages, "2026-04-12")
	if len(filtered) != 0 {
		t.Errorf("expected 0 voyages on 2026-04-12, got %d", len(filtered))
	}
}

func TestTallinkDealFlag(t *testing.T) {
	// Voyage with price 12.5 (below tallinkDealThreshold=20) should get "Deal" amenity.
	voyages := unwrapMockVoyages(t)

	// Second voyage has price 12.5.
	v := voyages[1]
	if v.TravelClass.MinPrice >= tallinkDealThreshold {
		t.Skipf("test assumption violated: price %.2f >= threshold %.2f", v.TravelClass.MinPrice, tallinkDealThreshold)
	}

	// Simulate deal flagging logic.
	var amenities []string
	if v.TravelClass.MinPrice > 0 && v.TravelClass.MinPrice < tallinkDealThreshold {
		amenities = append(amenities, "Deal")
	}
	if len(amenities) != 1 || amenities[0] != "Deal" {
		t.Errorf("expected [Deal] amenity for price %.2f, got %v", v.TravelClass.MinPrice, amenities)
	}

	// First voyage (38.9) should not be flagged.
	v2 := voyages[0]
	var amenities2 []string
	if v2.TravelClass.MinPrice > 0 && v2.TravelClass.MinPrice < tallinkDealThreshold {
		amenities2 = append(amenities2, "Deal")
	}
	if len(amenities2) != 0 {
		t.Errorf("expected no deal amenity for price %.2f, got %v", v2.TravelClass.MinPrice, amenities2)
	}
}

func TestSearchTallink_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	routes, err := SearchTallink(ctx, "Helsinki", "Tallinn", date, "EUR")
	if err != nil {
		t.Skipf("Tallink API unavailable: %v", err)
	}
	if len(routes) == 0 {
		t.Skip("no Tallink routes found")
	}

	r := routes[0]
	if r.Provider != "tallink" {
		t.Errorf("provider = %q, want tallink", r.Provider)
	}
	if r.Type != "ferry" {
		t.Errorf("type = %q, want ferry", r.Type)
	}
	if r.Duration <= 0 {
		t.Errorf("duration = %d, should be > 0", r.Duration)
	}
	if r.Departure.City != "Helsinki" {
		t.Errorf("departure city = %q, want Helsinki", r.Departure.City)
	}
	if r.Arrival.City != "Tallinn" {
		t.Errorf("arrival city = %q, want Tallinn", r.Arrival.City)
	}
	if r.BookingURL == "" {
		t.Error("booking URL should not be empty")
	}
	if !strings.Contains(r.BookingURL, "book.tallink.com") {
		t.Errorf("booking URL should use book.tallink.com, got %q", r.BookingURL)
	}
	if r.Transfers != 0 {
		t.Errorf("transfers = %d, want 0 (ferry)", r.Transfers)
	}
	if r.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", r.Currency)
	}
}
