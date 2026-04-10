package models

import "testing"

func TestResolveHotelCity(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Airport codes that should resolve to broader city.
		{name: "CDG -> Paris", input: "CDG", want: "Paris"},
		{name: "ORY -> Paris", input: "ORY", want: "Paris"},
		{name: "LHR -> London", input: "LHR", want: "London"},
		{name: "LGW -> London", input: "LGW", want: "London"},
		{name: "STN -> London", input: "STN", want: "London"},
		{name: "FCO -> Rome", input: "FCO", want: "Rome"},
		{name: "MXP -> Milan", input: "MXP", want: "Milan"},
		{name: "JFK -> New York", input: "JFK", want: "New York"},
		{name: "NRT -> Tokyo", input: "NRT", want: "Tokyo"},
		{name: "ICN -> Seoul", input: "ICN", want: "Seoul"},
		{name: "BKK -> Bangkok", input: "BKK", want: "Bangkok"},
		{name: "PEK -> Beijing", input: "PEK", want: "Beijing"},
		{name: "SAW -> Istanbul", input: "SAW", want: "Istanbul"},

		// Codes not in airportSearchCities but in AirportNames -> fallback.
		{name: "AMS -> Amsterdam", input: "AMS", want: "Amsterdam"},
		{name: "HEL -> Helsinki", input: "HEL", want: "Helsinki"},
		{name: "BCN -> Barcelona", input: "BCN", want: "Barcelona"},
		{name: "PRG -> Prague", input: "PRG", want: "Prague"},

		// Unknown code -> passthrough.
		{name: "unknown code", input: "ZZZ", want: "ZZZ"},

		// City name (not IATA code) -> passthrough.
		{name: "city name", input: "Prague", want: "Prague"},
		{name: "lowercase", input: "paris", want: "paris"},

		// Edge cases.
		{name: "empty string", input: "", want: ""},
		{name: "whitespace", input: "  ", want: ""},
		{name: "trimmed code", input: " CDG ", want: "Paris"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveHotelCity(tt.input); got != tt.want {
				t.Errorf("ResolveHotelCity(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
