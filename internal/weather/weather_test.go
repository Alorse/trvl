package weather

import (
	"testing"
)

func TestDescribeWeatherCode(t *testing.T) {
	tests := []struct {
		code   int
		precip float64
		want   string
	}{
		{0, 0, "Sunny"},
		{1, 0, "Mostly sunny"},
		{2, 0, "Partly cloudy"},
		{3, 0, "Overcast"},
		{45, 0, "Fog"},
		{48, 0, "Fog"},
		{51, 0.5, "Drizzle"},
		{61, 1, "Rain"},
		{61, 10, "Heavy rain"},
		{71, 3, "Snow"},
		{80, 4, "Rain showers"},
		{95, 0, "Thunderstorm"},
		{96, 0, "Thunderstorm with hail"},
		{999, 0, "Partly cloudy"}, // unknown code, no precip
		{999, 1, "Light rain"},    // unknown code, light precip
		{999, 6, "Rain"},          // unknown code, heavy precip
	}

	for _, tc := range tests {
		got := describeWeatherCode(tc.code, tc.precip)
		if got != tc.want {
			t.Errorf("describeWeatherCode(%d, %.1f) = %q, want %q", tc.code, tc.precip, got, tc.want)
		}
	}
}

func TestWeatherEmoji(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"Sunny", "☀️"},
		{"Mostly sunny", "☀️"},
		{"Partly cloudy", "⛅"},
		{"Overcast", "☁️"},
		{"Fog", "🌫️"},
		{"Thunderstorm", "⛈️"},
		{"Thunderstorm with hail", "⛈️"},
		{"Snow", "❄️"},
		{"Snow showers", "❄️"},
		{"Rain", "🌧️"},
		{"Heavy rain", "🌧️"},
		{"Drizzle", "🌧️"},
		{"Rain showers", "🌧️"},
		{"Something else", "🌤️"},
	}

	for _, tc := range tests {
		got := WeatherEmoji(tc.desc)
		if got != tc.want {
			t.Errorf("WeatherEmoji(%q) = %q, want %q", tc.desc, got, tc.want)
		}
	}
}

func TestFormatDateShort(t *testing.T) {
	tests := []struct {
		date string
		want string
	}{
		{"2026-04-12", "Apr 12"},
		{"2026-01-01", "Jan 01"},
		{"2026-12-31", "Dec 31"},
		{"invalid", "invalid"}, // fallback to raw
	}
	for _, tc := range tests {
		got := FormatDateShort(tc.date)
		if got != tc.want {
			t.Errorf("FormatDateShort(%q) = %q, want %q", tc.date, got, tc.want)
		}
	}
}

func TestDayOfWeek(t *testing.T) {
	// 2026-04-12 is a Sunday
	got := DayOfWeek("2026-04-12")
	if got != "Sun" {
		t.Errorf("DayOfWeek(2026-04-12) = %q, want Sun", got)
	}
	// 2026-04-13 is Monday
	got = DayOfWeek("2026-04-13")
	if got != "Mon" {
		t.Errorf("DayOfWeek(2026-04-13) = %q, want Mon", got)
	}
	// invalid
	got = DayOfWeek("bad")
	if got != "" {
		t.Errorf("DayOfWeek(bad) = %q, want empty", got)
	}
}

func TestParseForecasts(t *testing.T) {
	raw := openMeteoResponse{}
	raw.Daily.Time = []string{"2026-04-12", "2026-04-13"}
	raw.Daily.TemperatureMax = []float64{18.5, 15.0}
	raw.Daily.TemperatureMin = []float64{8.0, 9.5}
	raw.Daily.PrecipitationSum = []float64{0.0, 3.5}
	raw.Daily.WeatherCode = []int{0, 61}

	forecasts := parseForecasts("Prague", raw)

	if len(forecasts) != 2 {
		t.Fatalf("expected 2 forecasts, got %d", len(forecasts))
	}

	f0 := forecasts[0]
	if f0.City != "Prague" {
		t.Errorf("City = %q, want Prague", f0.City)
	}
	if f0.Date != "2026-04-12" {
		t.Errorf("Date = %q, want 2026-04-12", f0.Date)
	}
	if f0.TempMax != 18.5 {
		t.Errorf("TempMax = %v, want 18.5", f0.TempMax)
	}
	if f0.TempMin != 8.0 {
		t.Errorf("TempMin = %v, want 8.0", f0.TempMin)
	}
	if f0.Precipitation != 0.0 {
		t.Errorf("Precipitation = %v, want 0.0", f0.Precipitation)
	}
	if f0.Description != "Sunny" {
		t.Errorf("Description = %q, want Sunny", f0.Description)
	}

	f1 := forecasts[1]
	if f1.Description != "Rain" {
		t.Errorf("Description = %q, want Rain", f1.Description)
	}
}

func TestParseForecastsEmpty(t *testing.T) {
	raw := openMeteoResponse{}
	forecasts := parseForecasts("Nowhere", raw)
	if len(forecasts) != 0 {
		t.Errorf("expected empty forecasts for empty response, got %d", len(forecasts))
	}
}

func TestSafeHelpers(t *testing.T) {
	ss := []string{"a", "b"}
	if safeStringAt(ss, 5) != "" {
		t.Error("out of bounds string should be empty")
	}
	if safeStringAt(ss, 0) != "a" {
		t.Error("index 0 should be 'a'")
	}

	fs := []float64{1.0, 2.0}
	if safeFloat64At(fs, 5) != 0 {
		t.Error("out of bounds float should be 0")
	}
	if safeFloat64At(fs, 1) != 2.0 {
		t.Error("index 1 should be 2.0")
	}

	is := []int{10, 20}
	if safeIntAt(is, 5) != 0 {
		t.Error("out of bounds int should be 0")
	}
	if safeIntAt(is, 0) != 10 {
		t.Error("index 0 should be 10")
	}
}
