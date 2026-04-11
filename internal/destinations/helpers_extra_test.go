package destinations

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ============================================================
// advisoryText — boundary values matching actual implementation
// ============================================================

func TestAdvisoryText_BoundaryValues(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.0, "Exercise normal caution"},
		{2.5, "Exercise normal caution"},
		{2.6, "Exercise increased caution"},
		{3.5, "Exercise increased caution"},
		{3.6, "Reconsider travel"},
		{4.0, "Reconsider travel"},
		{4.1, "Do not travel"},
		{5.0, "Do not travel"},
	}
	for _, tt := range tests {
		got := advisoryText(tt.score)
		if got != tt.want {
			t.Errorf("advisoryText(%.1f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

// ============================================================
// filterHolidays — edge cases
// ============================================================

func TestFilterHolidays_EmptyInput(t *testing.T) {
	result := filterHolidays(nil, "2026-06-15", "2026-06-18")
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestFilterHolidays_OnlyStartDate(t *testing.T) {
	holidays := []models.Holiday{
		{Date: "2026-06-14", Name: "Before"},
		{Date: "2026-06-15", Name: "OnStart"},
		{Date: "2026-06-20", Name: "After"},
	}
	result := filterHolidays(holidays, "2026-06-15", "")
	// With empty end date, should return all.
	if len(result) != 3 {
		t.Errorf("expected 3 (no end date filter), got %d", len(result))
	}
}

func TestFilterHolidays_ExactBoundary(t *testing.T) {
	holidays := []models.Holiday{
		{Date: "2026-06-15", Name: "Start"},
		{Date: "2026-06-18", Name: "End"},
	}
	result := filterHolidays(holidays, "2026-06-15", "2026-06-18")
	if len(result) != 2 {
		t.Errorf("expected 2 (both boundaries inclusive), got %d", len(result))
	}
}

// ============================================================
// ProximityMatch — additional cases
// ============================================================

func TestProximityMatch_ZeroThreshold(t *testing.T) {
	got := ProximityMatch(60.17, 24.94, 60.17, 24.94, 0)
	if !got {
		t.Error("expected true for same point with zero threshold")
	}
}

func TestProximityMatch_NegativeCoords(t *testing.T) {
	got := ProximityMatch(-33.8688, 151.2093, -33.8688, 151.2093, 100)
	if !got {
		t.Error("expected true for same point in southern hemisphere")
	}
}

// ============================================================
// weatherCodeDescription — all known codes
// ============================================================

func TestWeatherCodeDescription_AllKnownCodes(t *testing.T) {
	knownCodes := map[int]string{
		0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
		45: "Fog", 48: "Fog",
		51: "Drizzle", 53: "Drizzle", 55: "Drizzle",
		61: "Rain", 63: "Rain", 65: "Rain",
		71: "Snow", 73: "Snow", 75: "Snow",
		80: "Rain showers", 81: "Rain showers", 82: "Rain showers",
		95: "Thunderstorm", 96: "Thunderstorm", 99: "Thunderstorm",
	}
	for code, want := range knownCodes {
		got := weatherCodeDescription(code)
		if got != want {
			t.Errorf("weatherCodeDescription(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestWeatherCodeDescription_UnknownCode(t *testing.T) {
	got := weatherCodeDescription(500)
	if got != "Unknown" {
		t.Errorf("weatherCodeDescription(500) = %q, want Unknown", got)
	}
}

// ============================================================
// nameMatchScore — note: case-sensitive Contains, not case-insensitive
// ============================================================

func TestNameMatchScore_ExactWords(t *testing.T) {
	score := nameMatchScore("Grand Hotel", "Grand Hotel Prague")
	if score < 2 {
		t.Errorf("expected score >= 2, got %d", score)
	}
}

func TestNameMatchScore_BothEmpty(t *testing.T) {
	score := nameMatchScore("", "")
	if score != 0 {
		t.Errorf("expected 0 for both empty, got %d", score)
	}
}

func TestNameMatchScore_NoOverlap(t *testing.T) {
	score := nameMatchScore("Hilton", "Marriott Resort")
	if score != 0 {
		t.Errorf("expected 0 for no overlap, got %d", score)
	}
}
