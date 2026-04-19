package hacks

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

func TestGoogleFlightsURL(t *testing.T) {
	tests := []struct {
		dest, origin, date string
		want               string
	}{
		{
			dest: "HEL", origin: "AMS", date: "2026-05-01",
			want: "https://www.google.com/travel/flights?q=Flights+to+HEL+from+AMS+on+2026-05-01",
		},
		{
			dest: "BCN", origin: "LHR", date: "2026-12-25",
			want: "https://www.google.com/travel/flights?q=Flights+to+BCN+from+LHR+on+2026-12-25",
		},
	}

	for _, tt := range tests {
		got := googleFlightsURL(tt.dest, tt.origin, tt.date)
		if got != tt.want {
			t.Errorf("googleFlightsURL(%q, %q, %q) = %q, want %q", tt.dest, tt.origin, tt.date, got, tt.want)
		}
	}
}

func TestDetectorInputValid(t *testing.T) {
	tests := []struct {
		name string
		in   DetectorInput
		want bool
	}{
		{name: "both set", in: DetectorInput{Origin: "AMS", Destination: "HEL"}, want: true},
		{name: "origin empty", in: DetectorInput{Origin: "", Destination: "HEL"}, want: false},
		{name: "destination empty", in: DetectorInput{Origin: "AMS", Destination: ""}, want: false},
		{name: "both empty", in: DetectorInput{Origin: "", Destination: ""}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.valid()
			if got != tt.want {
				t.Errorf("valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMinFlightPrice(t *testing.T) {
	tests := []struct {
		name string
		r    *models.FlightSearchResult
		want float64
	}{
		{
			name: "nil result",
			r:    nil,
			want: 0,
		},
		{
			name: "unsuccessful",
			r:    &models.FlightSearchResult{Success: false},
			want: 0,
		},
		{
			name: "empty flights",
			r:    &models.FlightSearchResult{Success: true, Flights: []models.FlightResult{}},
			want: 0,
		},
		{
			name: "single flight",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{{Price: 99.50}},
			},
			want: 99.50,
		},
		{
			name: "multiple flights picks cheapest",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Price: 200},
					{Price: 50},
					{Price: 150},
				},
			},
			want: 50,
		},
		{
			name: "zero price ignored",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Price: 0},
					{Price: 100},
				},
			},
			want: 100,
		},
		{
			name: "negative price ignored",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Price: -10},
					{Price: 80},
				},
			},
			want: 80,
		},
		{
			name: "all zero returns 0",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{{Price: 0}, {Price: 0}},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minFlightPrice(tt.r)
			if got != tt.want {
				t.Errorf("minFlightPrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlightCurrency(t *testing.T) {
	tests := []struct {
		name     string
		r        *models.FlightSearchResult
		fallback string
		want     string
	}{
		{
			name:     "nil result",
			r:        nil,
			fallback: "USD",
			want:     "USD",
		},
		{
			name:     "unsuccessful",
			r:        &models.FlightSearchResult{Success: false},
			fallback: "EUR",
			want:     "EUR",
		},
		{
			name: "empty flights",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{},
			},
			fallback: "GBP",
			want:     "GBP",
		},
		{
			name: "picks first flight currency",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Currency: "SEK"},
					{Currency: "EUR"},
				},
			},
			fallback: "USD",
			want:     "SEK",
		},
		{
			name: "empty currency falls back",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{{Currency: ""}},
			},
			fallback: "EUR",
			want:     "EUR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flightCurrency(tt.r, tt.fallback)
			if got != tt.want {
				t.Errorf("flightCurrency() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseHour(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "HH:MM", input: "14:30", want: 14},
		{name: "midnight", input: "00:00", want: 0},
		{name: "ISO datetime", input: "2026-04-10T09:15", want: 9},
		{name: "evening ISO", input: "2026-04-10T22:45", want: 22},
		{name: "invalid", input: "not-a-time", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hour int
			_, err := parseHour(tt.input, &hour)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hour != tt.want {
				t.Errorf("parseHour(%q) hour = %d, want %d", tt.input, hour, tt.want)
			}
		})
	}
}

func TestParseHourError(t *testing.T) {
	e := &parseHourError{s: "bad"}
	got := e.Error()
	if got != "cannot parse hour from: bad" {
		t.Errorf("Error() = %q", got)
	}
}

func TestIsOvernightRoute_HHMMEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		dep  string
		arr  string
		want bool
	}{
		{name: "HH:MM early morning depart", dep: "01:30", arr: "07:00", want: true},
		{name: "HH:MM afternoon", dep: "14:00", arr: "18:00", want: false},
		{name: "invalid strings", dep: "xyz", arr: "abc", want: false},
		{name: "empty strings", dep: "", arr: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOvernightRoute(tt.dep, tt.arr)
			if got != tt.want {
				t.Errorf("isOvernightRoute(%q, %q) = %v, want %v", tt.dep, tt.arr, got, tt.want)
			}
		})
	}
}

func TestIsLowCostFlight(t *testing.T) {
	tests := []struct {
		name string
		f    models.FlightResult
		want bool
	}{
		{
			name: "ryanair",
			f: models.FlightResult{
				Legs: []models.FlightLeg{{AirlineCode: "FR"}},
			},
			want: true,
		},
		{
			name: "easyjet",
			f: models.FlightResult{
				Legs: []models.FlightLeg{{AirlineCode: "U2"}},
			},
			want: true,
		},
		{
			name: "wizz air",
			f: models.FlightResult{
				Legs: []models.FlightLeg{{AirlineCode: "W6"}},
			},
			want: true,
		},
		{
			name: "legacy carrier",
			f: models.FlightResult{
				Legs: []models.FlightLeg{{AirlineCode: "BA"}},
			},
			want: false,
		},
		{
			name: "no legs",
			f:    models.FlightResult{},
			want: false,
		},
		{
			name: "mixed legs second is lcc",
			f: models.FlightResult{
				Legs: []models.FlightLeg{
					{AirlineCode: "KL"},
					{AirlineCode: "HV"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLowCostFlight(tt.f)
			if got != tt.want {
				t.Errorf("isLowCostFlight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLccName(t *testing.T) {
	tests := []struct {
		name string
		f    models.FlightResult
		want string
	}{
		{
			name: "ryanair",
			f:    models.FlightResult{Legs: []models.FlightLeg{{AirlineCode: "FR"}}},
			want: "Ryanair",
		},
		{
			name: "no match",
			f:    models.FlightResult{Legs: []models.FlightLeg{{AirlineCode: "BA"}}},
			want: "LCC",
		},
		{
			name: "empty",
			f:    models.FlightResult{},
			want: "LCC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lccName(tt.f)
			if got != tt.want {
				t.Errorf("lccName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoyaltyConflictNote(t *testing.T) {
	tests := []struct {
		name   string
		result *models.FlightSearchResult
		prefs  *preferences.Preferences
		want   bool // true = non-empty warning
	}{
		{
			name:   "nil prefs",
			result: &models.FlightSearchResult{Success: true},
			prefs:  nil,
			want:   false,
		},
		{
			name:   "empty loyalty list",
			result: &models.FlightSearchResult{Success: true},
			prefs:  &preferences.Preferences{LoyaltyAirlines: []string{}},
			want:   false,
		},
		{
			name: "loyalty conflict detected",
			result: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Legs: []models.FlightLeg{{AirlineCode: "KL", Airline: "KLM"}}},
				},
			},
			prefs: &preferences.Preferences{LoyaltyAirlines: []string{"KL"}},
			want:  true,
		},
		{
			name: "no conflict",
			result: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Legs: []models.FlightLeg{{AirlineCode: "BA", Airline: "British Airways"}}},
				},
			},
			prefs: &preferences.Preferences{LoyaltyAirlines: []string{"KL"}},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loyaltyConflictNote(tt.result, tt.prefs)
			if tt.want && got == "" {
				t.Error("expected non-empty warning, got empty")
			}
			if !tt.want && got != "" {
				t.Errorf("expected empty, got %q", got)
			}
		})
	}
}

func TestRoutesToughDestination(t *testing.T) {
	tests := []struct {
		name string
		r    *models.FlightSearchResult
		dest string
		want bool
	}{
		{
			name: "nil result",
			r:    nil,
			dest: "BCN",
			want: false,
		},
		{
			name: "unsuccessful",
			r:    &models.FlightSearchResult{Success: false},
			dest: "BCN",
			want: false,
		},
		{
			name: "single leg cannot be hidden city",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Legs: []models.FlightLeg{
						{ArrivalAirport: models.AirportInfo{Code: "BCN"}},
					}},
				},
			},
			dest: "BCN",
			want: true, // optimistic fallback: has flights but no multi-leg match
		},
		{
			name: "multi-leg with dest as intermediate",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Legs: []models.FlightLeg{
						{ArrivalAirport: models.AirportInfo{Code: "BCN"}},
						{ArrivalAirport: models.AirportInfo{Code: "PMI"}},
					}},
				},
			},
			dest: "BCN",
			want: true,
		},
		{
			name: "multi-leg without dest intermediate",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Legs: []models.FlightLeg{
						{ArrivalAirport: models.AirportInfo{Code: "MAD"}},
						{ArrivalAirport: models.AirportInfo{Code: "PMI"}},
					}},
				},
			},
			dest: "BCN",
			want: true, // optimistic: has flights
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := routesThroughDestination(tt.r, tt.dest)
			if got != tt.want {
				t.Errorf("routesThroughDestination() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrimaryAirlineCode(t *testing.T) {
	tests := []struct {
		name string
		r    *models.FlightSearchResult
		want string
	}{
		{
			name: "nil",
			r:    nil,
			want: "",
		},
		{
			name: "unsuccessful",
			r:    &models.FlightSearchResult{Success: false},
			want: "",
		},
		{
			name: "first airline code",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Legs: []models.FlightLeg{
						{AirlineCode: "KL"},
						{AirlineCode: "AF"},
					}},
				},
			},
			want: "KL",
		},
		{
			name: "empty code skipped",
			r: &models.FlightSearchResult{
				Success: true,
				Flights: []models.FlightResult{
					{Legs: []models.FlightLeg{
						{AirlineCode: ""},
						{AirlineCode: "BA"},
					}},
				},
			},
			want: "BA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primaryAirlineCode(tt.r)
			if got != tt.want {
				t.Errorf("primaryAirlineCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHubCityName(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"HEL", "Helsinki"},
		{"KEF", "Reykjavik"},
		{"IST", "Istanbul"},
		{"DOH", "Doha"},
		{"DXB", "Dubai"},
		{"SIN", "Singapore"},
		{"ZZZ", "ZZZ"}, // unknown fallback
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := hubCityName(tt.code)
			if got != tt.want {
				t.Errorf("hubCityName(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}
