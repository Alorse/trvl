package models

import (
	"testing"
)

func TestLookupAirportName_Known(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"HEL", "Helsinki"},
		{"JFK", "New York JFK"},
		{"NRT", "Tokyo Narita"},
		{"BCN", "Barcelona"},
		{"DBV", "Dubrovnik"},
		{"SIN", "Singapore"},
	}
	for _, tt := range tests {
		got := LookupAirportName(tt.code)
		if got != tt.want {
			t.Errorf("LookupAirportName(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestLookupAirportName_Unknown(t *testing.T) {
	got := LookupAirportName("ZZZ")
	if got != "ZZZ" {
		t.Errorf("LookupAirportName(ZZZ) = %q, want ZZZ", got)
	}
}

func TestAirportNames_NotEmpty(t *testing.T) {
	if len(AirportNames) < 100 {
		t.Errorf("AirportNames has %d entries, want >= 100", len(AirportNames))
	}
}

func TestAirportNames_AllThreeLetterCodes(t *testing.T) {
	for code := range AirportNames {
		if len(code) != 3 {
			t.Errorf("airport code %q is not 3 characters", code)
		}
	}
}
