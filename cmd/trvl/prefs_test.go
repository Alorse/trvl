package main

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/preferences"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "simple", input: "HEL,AMS,CDG", want: []string{"HEL", "AMS", "CDG"}},
		{name: "with spaces", input: " HEL , AMS , CDG ", want: []string{"HEL", "AMS", "CDG"}},
		{name: "empty items filtered", input: "HEL,,AMS,,", want: []string{"HEL", "AMS"}},
		{name: "single value", input: "HEL", want: []string{"HEL"}},
		{name: "all empty", input: ",,", want: []string{}},
		{name: "whitespace only items", input: " , , ", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitAndTrim(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("splitAndTrim(%q)[%d] = %q, want %q", tt.input, i, g, tt.want[i])
				}
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	trueValues := []string{"true", "TRUE", "True", "yes", "YES", "1", "on", "ON"}
	falseValues := []string{"false", "FALSE", "False", "no", "NO", "0", "off", "OFF"}
	invalidValues := []string{"maybe", "yep", "nope", "2", ""}

	for _, v := range trueValues {
		t.Run("true_"+v, func(t *testing.T) {
			got, err := parseBool(v)
			if err != nil {
				t.Fatalf("parseBool(%q) error: %v", v, err)
			}
			if !got {
				t.Errorf("parseBool(%q) = false, want true", v)
			}
		})
	}

	for _, v := range falseValues {
		t.Run("false_"+v, func(t *testing.T) {
			got, err := parseBool(v)
			if err != nil {
				t.Fatalf("parseBool(%q) error: %v", v, err)
			}
			if got {
				t.Errorf("parseBool(%q) = true, want false", v)
			}
		})
	}

	for _, v := range invalidValues {
		t.Run("invalid_"+v, func(t *testing.T) {
			_, err := parseBool(v)
			if err == nil {
				t.Errorf("parseBool(%q) should return error", v)
			}
		})
	}
}

func TestParseBool_Whitespace(t *testing.T) {
	got, err := parseBool("  true  ")
	if err != nil {
		t.Fatalf("parseBool with whitespace: %v", err)
	}
	if !got {
		t.Error("parseBool('  true  ') should return true")
	}
}

func TestFormatRating(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{name: "zero", input: 0, want: "0"},
		{name: "whole number", input: 4.0, want: "4.0"},
		{name: "decimal", input: 4.5, want: "4.5"},
		{name: "long decimal", input: 3.14159, want: "3.1"},
		{name: "max", input: 5.0, want: "5.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRating(tt.input)
			if got != tt.want {
				t.Errorf("formatRating(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestApplyPreference_HomeAirports(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "home_airports", "HEL,AMS"); err != nil {
		t.Fatal(err)
	}
	if len(p.HomeAirports) != 2 || p.HomeAirports[0] != "HEL" {
		t.Errorf("home_airports: got %v", p.HomeAirports)
	}
}

func TestApplyPreference_HomeCities(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "home_cities", "Helsinki, Amsterdam"); err != nil {
		t.Fatal(err)
	}
	if len(p.HomeCities) != 2 || p.HomeCities[0] != "Helsinki" {
		t.Errorf("home_cities: got %v", p.HomeCities)
	}
}

func TestApplyPreference_Booleans(t *testing.T) {
	boolKeys := []string{"carry_on_only", "prefer_direct", "no_dormitories", "ensuite_only", "fast_wifi_needed"}
	for _, key := range boolKeys {
		t.Run(key, func(t *testing.T) {
			p := preferences.Default()
			if err := applyPreference(p, key, "true"); err != nil {
				t.Fatalf("applyPreference(%s, true): %v", key, err)
			}
		})
	}
}

func TestApplyPreference_MinHotelStars(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "min_hotel_stars", "3"); err != nil {
		t.Fatal(err)
	}
	if p.MinHotelStars != 3 {
		t.Errorf("min_hotel_stars: got %d, want 3", p.MinHotelStars)
	}

	// Invalid value.
	if err := applyPreference(p, "min_hotel_stars", "7"); err == nil {
		t.Error("min_hotel_stars=7 should fail")
	}
}

func TestApplyPreference_MinHotelRating(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "min_hotel_rating", "4.0"); err != nil {
		t.Fatal(err)
	}
	if p.MinHotelRating != 4.0 {
		t.Errorf("min_hotel_rating: got %v, want 4.0", p.MinHotelRating)
	}

	// Invalid value.
	if err := applyPreference(p, "min_hotel_rating", "6.0"); err == nil {
		t.Error("min_hotel_rating=6.0 should fail")
	}
}

func TestApplyPreference_DisplayCurrency(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "display_currency", "usd"); err != nil {
		t.Fatal(err)
	}
	if p.DisplayCurrency != "USD" {
		t.Errorf("display_currency: got %q, want USD", p.DisplayCurrency)
	}

	// Invalid length.
	if err := applyPreference(p, "display_currency", "US"); err == nil {
		t.Error("display_currency=US should fail")
	}
}

func TestApplyPreference_Locale(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "locale", "en-FI"); err != nil {
		t.Fatal(err)
	}
	if p.Locale != "en-FI" {
		t.Errorf("locale: got %q", p.Locale)
	}
}

func TestApplyPreference_LoyaltyAirlines(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "loyalty_airlines", "KL,AY"); err != nil {
		t.Fatal(err)
	}
	if len(p.LoyaltyAirlines) != 2 {
		t.Errorf("loyalty_airlines: got %v", p.LoyaltyAirlines)
	}
}

func TestApplyPreference_PreferredDistricts(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "preferred_districts", "Prague=Prague 1,Prague 2"); err != nil {
		t.Fatal(err)
	}
	if len(p.PreferredDistricts["Prague"]) != 2 {
		t.Errorf("preferred_districts: got %v", p.PreferredDistricts)
	}

	// Delete districts.
	if err := applyPreference(p, "preferred_districts", "Prague="); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.PreferredDistricts["Prague"]; ok {
		t.Error("Prague districts should be deleted")
	}

	// Missing = sign.
	if err := applyPreference(p, "preferred_districts", "PragueDistrict1"); err == nil {
		t.Error("missing = should fail")
	}

	// Missing city name.
	if err := applyPreference(p, "preferred_districts", "=District1"); err == nil {
		t.Error("empty city should fail")
	}
}

func TestApplyPreference_Unknown(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "nonexistent_key", "value"); err == nil {
		t.Error("unknown key should return error")
	}
}

func TestApplyPreference_InvalidBool(t *testing.T) {
	p := preferences.Default()
	if err := applyPreference(p, "carry_on_only", "maybe"); err == nil {
		t.Error("invalid bool value should return error")
	}
}
