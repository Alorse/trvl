package preferences

import "testing"

func TestDefaultDistrictsFor_KnownCities(t *testing.T) {
	cities := []string{
		"Amsterdam", "Paris", "Prague", "Krakow", "Kraków",
		"Madrid", "Helsinki", "Vienna", "Barcelona", "Lisbon",
		"Rome", "Berlin", "Lisboa",
	}

	for _, city := range cities {
		t.Run(city, func(t *testing.T) {
			got := DefaultDistrictsFor(city)
			if got == nil || len(got) == 0 {
				t.Errorf("DefaultDistrictsFor(%q) returned nil/empty, want non-empty", city)
			}
		})
	}
}

func TestDefaultDistrictsFor_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"amsterdam"},
		{"AMSTERDAM"},
		{"Amsterdam"},
		{"paris"},
		{"PARIS"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DefaultDistrictsFor(tt.input)
			if got == nil {
				t.Errorf("DefaultDistrictsFor(%q) should match case-insensitively", tt.input)
			}
		})
	}
}

func TestDefaultDistrictsFor_CityWithCountry(t *testing.T) {
	// "Paris, France" should match via first-word extraction.
	got := DefaultDistrictsFor("Paris, France")
	if got == nil {
		t.Error("DefaultDistrictsFor('Paris, France') should match via first word")
	}

	got = DefaultDistrictsFor("Amsterdam, Netherlands")
	if got == nil {
		t.Error("DefaultDistrictsFor('Amsterdam, Netherlands') should match via first word")
	}
}

func TestDefaultDistrictsFor_UnknownCity(t *testing.T) {
	unknown := []string{"Tokyo", "Nairobi", "Honolulu", "Smalltown"}
	for _, city := range unknown {
		t.Run(city, func(t *testing.T) {
			got := DefaultDistrictsFor(city)
			if got != nil {
				t.Errorf("DefaultDistrictsFor(%q) should return nil for unknown city, got %v", city, got)
			}
		})
	}
}

func TestDefaultDistrictsFor_Empty(t *testing.T) {
	if got := DefaultDistrictsFor(""); got != nil {
		t.Errorf("DefaultDistrictsFor('') should return nil, got %v", got)
	}
}

func TestDefaultDistrictsFor_WhitespaceOnly(t *testing.T) {
	if got := DefaultDistrictsFor("   "); got != nil {
		t.Errorf("DefaultDistrictsFor('   ') should return nil, got %v", got)
	}
}

func TestDefaultDistrictsFor_DistrictContent(t *testing.T) {
	// Verify that known districts are present for a few cities.
	amsterdam := DefaultDistrictsFor("Amsterdam")
	found := false
	for _, d := range amsterdam {
		if d == "jordaan" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Amsterdam defaults should include 'jordaan'")
	}

	paris := DefaultDistrictsFor("Paris")
	found = false
	for _, d := range paris {
		if d == "le marais" || d == "marais" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Paris defaults should include 'marais' or 'le marais'")
	}
}
