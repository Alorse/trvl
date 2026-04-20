package flights

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseAirports(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"HEL", []string{"HEL"}},
		{"hel", []string{"HEL"}},
		{"HEL, ARN, CPH", []string{"HEL", "ARN", "CPH"}},
		{"  hel , arn  ", []string{"HEL", "ARN"}},
		{"", nil},
		{",,,", nil},
		{"HEL,,ARN", []string{"HEL", "ARN"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseAirports(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseAirports(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseAirports(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseFlightLocations_IATACodes(t *testing.T) {
	// IATA codes pass through unchanged.
	tests := []struct {
		input string
		want  []string
	}{
		{"HEL", []string{"HEL"}},
		{"AMS,EIN", []string{"AMS", "EIN"}},
		{" JFK , LGA ", []string{"JFK", "LGA"}},
	}
	for _, tt := range tests {
		got := ParseFlightLocations(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseFlightLocations(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseFlightLocations_CityNames(t *testing.T) {
	// City names expand to their airports.
	tests := []struct {
		input string
		want  []string // sorted
	}{
		{"Paris", []string{"CDG", "ORY"}},
		{"paris", []string{"CDG", "ORY"}},
		{"Tokyo", []string{"HND", "NRT"}},
		{"Helsinki", []string{"HEL"}},
	}
	for _, tt := range tests {
		got := ParseFlightLocations(tt.input)
		sort.Strings(got)
		want := make([]string, len(tt.want))
		copy(want, tt.want)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseFlightLocations(%q) = %v, want %v", tt.input, got, want)
		}
	}
}

func TestParseFlightLocations_UnknownPassthrough(t *testing.T) {
	// Unknown tokens (not IATA, not city) pass through unchanged.
	got := ParseFlightLocations("BOM")
	if !reflect.DeepEqual(got, []string{"BOM"}) {
		t.Errorf("ParseFlightLocations(BOM) = %v, want [BOM]", got)
	}
}

func TestParseFlightLocations_Mixed(t *testing.T) {
	// Mix of IATA code and city name in comma list.
	got := ParseFlightLocations("BCN,Paris")
	sort.Strings(got)
	want := []string{"BCN", "CDG", "ORY"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseFlightLocations(BCN,Paris) = %v, want %v", got, want)
	}
}

func TestParseFlightLocations_Empty(t *testing.T) {
	got := ParseFlightLocations("")
	if len(got) != 0 {
		t.Errorf("ParseFlightLocations(\"\") = %v, want []", got)
	}
}
