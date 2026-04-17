package main

import (
	"context"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ---------------------------------------------------------------------------
// formatPrice
// ---------------------------------------------------------------------------

func TestFormatPrice_Normal(t *testing.T) {
	got := formatPrice(199, "EUR")
	if got != "EUR 199" {
		t.Errorf("formatPrice(199, EUR) = %q, want %q", got, "EUR 199")
	}
}

func TestFormatPrice_Zero(t *testing.T) {
	got := formatPrice(0, "EUR")
	if got != "-" {
		t.Errorf("formatPrice(0, EUR) = %q, want %q", got, "-")
	}
}

func TestFormatPrice_LargeAmount(t *testing.T) {
	got := formatPrice(12345, "JPY")
	if got != "JPY 12345" {
		t.Errorf("formatPrice(12345, JPY) = %q, want %q", got, "JPY 12345")
	}
}

// ---------------------------------------------------------------------------
// formatDuration
// ---------------------------------------------------------------------------

func TestFormatDuration_HoursAndMinutes(t *testing.T) {
	got := formatDuration(150)
	if got != "2h 30m" {
		t.Errorf("formatDuration(150) = %q, want %q", got, "2h 30m")
	}
}

func TestFormatDuration_MinutesOnly(t *testing.T) {
	got := formatDuration(45)
	if got != "45m" {
		t.Errorf("formatDuration(45) = %q, want %q", got, "45m")
	}
}

func TestFormatDuration_Zero(t *testing.T) {
	got := formatDuration(0)
	if got != "-" {
		t.Errorf("formatDuration(0) = %q, want %q", got, "-")
	}
}

func TestFormatDuration_ExactHours(t *testing.T) {
	got := formatDuration(120)
	if got != "2h 0m" {
		t.Errorf("formatDuration(120) = %q, want %q", got, "2h 0m")
	}
}

// ---------------------------------------------------------------------------
// formatStops
// ---------------------------------------------------------------------------

func TestFormatStops_Direct(t *testing.T) {
	if got := formatStops(0); got != "Direct" {
		t.Errorf("formatStops(0) = %q, want %q", got, "Direct")
	}
}

func TestFormatStops_One(t *testing.T) {
	if got := formatStops(1); got != "1 stop" {
		t.Errorf("formatStops(1) = %q, want %q", got, "1 stop")
	}
}

func TestFormatStops_Multiple(t *testing.T) {
	if got := formatStops(2); got != "2 stops" {
		t.Errorf("formatStops(2) = %q, want %q", got, "2 stops")
	}
	if got := formatStops(3); got != "3 stops" {
		t.Errorf("formatStops(3) = %q, want %q", got, "3 stops")
	}
}

// ---------------------------------------------------------------------------
// flightRoute
// ---------------------------------------------------------------------------

func TestFlightRoute_Direct(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{
			{
				DepartureAirport: models.AirportInfo{Code: "HEL"},
				ArrivalAirport:   models.AirportInfo{Code: "NRT"},
			},
		},
	}
	got := flightRoute(f)
	if got != "HEL -> NRT" {
		t.Errorf("flightRoute = %q, want %q", got, "HEL -> NRT")
	}
}

func TestFlightRoute_Connection(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{
			{
				DepartureAirport: models.AirportInfo{Code: "HEL"},
				ArrivalAirport:   models.AirportInfo{Code: "FRA"},
			},
			{
				DepartureAirport: models.AirportInfo{Code: "FRA"},
				ArrivalAirport:   models.AirportInfo{Code: "NRT"},
			},
		},
	}
	got := flightRoute(f)
	if got != "HEL -> FRA -> NRT" {
		t.Errorf("flightRoute = %q, want %q", got, "HEL -> FRA -> NRT")
	}
}

func TestFlightRoute_Empty(t *testing.T) {
	f := models.FlightResult{}
	if got := flightRoute(f); got != "" {
		t.Errorf("flightRoute(empty) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// colorizeStops
// ---------------------------------------------------------------------------

func TestColorizeStops_Direct(t *testing.T) {
	models.UseColor = false
	got := colorizeStops(0)
	if got != "Direct" {
		t.Errorf("colorizeStops(0) = %q, want %q", got, "Direct")
	}
}

func TestColorizeStops_One(t *testing.T) {
	models.UseColor = false
	got := colorizeStops(1)
	if got != "1 stop" {
		t.Errorf("colorizeStops(1) = %q, want %q", got, "1 stop")
	}
}

func TestColorizeStops_Two(t *testing.T) {
	models.UseColor = false
	got := colorizeStops(2)
	if got != "2 stops" {
		t.Errorf("colorizeStops(2) = %q, want %q", got, "2 stops")
	}
}

// ---------------------------------------------------------------------------
// colorizeRating
// ---------------------------------------------------------------------------

func TestColorizeRating_High(t *testing.T) {
	models.UseColor = false
	got := colorizeRating(9.5, "9.5")
	if got != "9.5" {
		t.Errorf("colorizeRating(9.5) = %q, want %q", got, "9.5")
	}
}

func TestColorizeRating_Medium(t *testing.T) {
	models.UseColor = false
	got := colorizeRating(7.5, "7.5")
	if got != "7.5" {
		t.Errorf("colorizeRating(7.5) = %q, want %q", got, "7.5")
	}
}

func TestColorizeRating_Low(t *testing.T) {
	models.UseColor = false
	got := colorizeRating(6.0, "6.0")
	if got != "6.0" {
		t.Errorf("colorizeRating(6.0) = %q, want %q", got, "6.0")
	}
}

func TestColorizeRating_Zero(t *testing.T) {
	models.UseColor = false
	got := colorizeRating(0, "0")
	if got != "0" {
		t.Errorf("colorizeRating(0) = %q, want %q", got, "0")
	}
}

// ---------------------------------------------------------------------------
// priceScale
// ---------------------------------------------------------------------------

func TestPriceScale_WithMultiple(t *testing.T) {
	var ps priceScale
	ps = ps.With(100)
	ps = ps.With(200)
	ps = ps.With(150)

	if ps.min != 100 {
		t.Errorf("min = %f, want 100", ps.min)
	}
	if ps.max != 200 {
		t.Errorf("max = %f, want 200", ps.max)
	}
}

func TestPriceScale_SkipsZero(t *testing.T) {
	var ps priceScale
	ps = ps.With(0)
	if ps.ok {
		t.Error("should not be ok after adding zero")
	}
}

func TestPriceScale_Apply(t *testing.T) {
	models.UseColor = false
	var ps priceScale
	ps = ps.With(100)
	ps = ps.With(300)

	// Cheapest gets green (no color when UseColor=false, just text).
	got := ps.Apply(100, "EUR 100")
	if got != "EUR 100" {
		t.Errorf("Apply(min) = %q, want %q", got, "EUR 100")
	}

	got = ps.Apply(300, "EUR 300")
	if got != "EUR 300" {
		t.Errorf("Apply(max) = %q, want %q", got, "EUR 300")
	}

	got = ps.Apply(200, "EUR 200")
	if got != "EUR 200" {
		t.Errorf("Apply(mid) = %q, want %q", got, "EUR 200")
	}
}

func TestPriceScale_ApplySinglePrice(t *testing.T) {
	models.UseColor = false
	var ps priceScale
	ps = ps.With(100)

	// min == max, no coloring.
	got := ps.Apply(100, "EUR 100")
	if got != "EUR 100" {
		t.Errorf("Apply(single) = %q, want %q", got, "EUR 100")
	}
}

// ---------------------------------------------------------------------------
// formatGroundTime
// ---------------------------------------------------------------------------

func TestFormatGroundTime_Full(t *testing.T) {
	got := formatGroundTime("2026-07-01T08:30:00+02:00")
	if got != "08:30" {
		t.Errorf("formatGroundTime = %q, want %q", got, "08:30")
	}
}

func TestFormatGroundTime_Short(t *testing.T) {
	got := formatGroundTime("08:30")
	if got != "08:30" {
		t.Errorf("formatGroundTime(short) = %q, want %q", got, "08:30")
	}
}

func TestFormatGroundTime_Empty(t *testing.T) {
	got := formatGroundTime("")
	if got != "" {
		t.Errorf("formatGroundTime(empty) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// formatDealAge
// ---------------------------------------------------------------------------

func TestFormatDealAge_Minutes(t *testing.T) {
	got := formatDealAge(time.Now().Add(-30 * time.Minute))
	if got != "30m ago" {
		// Allow +/- 1 minute for test timing.
		if got != "29m ago" && got != "31m ago" && got != "30m ago" {
			t.Errorf("formatDealAge(30m) = %q", got)
		}
	}
}

func TestFormatDealAge_Hours(t *testing.T) {
	got := formatDealAge(time.Now().Add(-5 * time.Hour))
	if got != "5h ago" {
		t.Errorf("formatDealAge(5h) = %q, want %q", got, "5h ago")
	}
}

func TestFormatDealAge_Days(t *testing.T) {
	got := formatDealAge(time.Now().Add(-48 * time.Hour))
	if got != "2d ago" {
		t.Errorf("formatDealAge(48h) = %q, want %q", got, "2d ago")
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncate_Short(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("truncate(short) = %q, want %q", got, "hello")
	}
}

func TestTruncate_Exact(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("truncate(exact) = %q, want %q", got, "hello")
	}
}

func TestTruncate_Long(t *testing.T) {
	got := truncate("hello world from trvl", 10)
	if got != "hello w..." {
		t.Errorf("truncate(long) = %q, want %q", got, "hello w...")
	}
}

func TestTruncate_VeryShort(t *testing.T) {
	got := truncate("hello", 3)
	if got != "hel" {
		t.Errorf("truncate(<=3) = %q, want %q", got, "hel")
	}
}

// ---------------------------------------------------------------------------
// splitLines
// ---------------------------------------------------------------------------

func TestSplitLines_Multiple(t *testing.T) {
	lines := splitLines("a\nb\nc")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("splitLines = %v", lines)
	}
}

func TestSplitLines_Single(t *testing.T) {
	lines := splitLines("hello")
	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("splitLines(single) = %v", lines)
	}
}

func TestSplitLines_Empty(t *testing.T) {
	lines := splitLines("")
	if len(lines) != 0 {
		t.Errorf("splitLines(empty) = %v, want empty", lines)
	}
}

func TestSplitLines_TrailingNewline(t *testing.T) {
	lines := splitLines("a\nb\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
}

// ---------------------------------------------------------------------------
// printWrapped
// ---------------------------------------------------------------------------

func TestPrintWrapped_SimpleText(t *testing.T) {
	models.UseColor = false
	out := captureStdout(t, func() {
		printWrapped("line one\nline two", 72, "  ")
	})
	if out == "" {
		t.Error("printWrapped produced no output")
	}
}

func TestPrintWrapped_EmptyLine(t *testing.T) {
	models.UseColor = false
	out := captureStdout(t, func() {
		printWrapped("before\n\nafter", 72, "  ")
	})
	if out == "" {
		t.Error("printWrapped produced no output")
	}
}

// ---------------------------------------------------------------------------
// shouldUseColor
// ---------------------------------------------------------------------------

type mockStdout struct {
	fd uintptr
}

func (m mockStdout) Fd() uintptr { return m.fd }

func TestShouldUseColor_NoColor(t *testing.T) {
	got := shouldUseColor(
		mockStdout{1},
		func(int) bool { return true },
		func(key string) string {
			if key == "NO_COLOR" {
				return "1"
			}
			return ""
		},
	)
	if got {
		t.Error("should return false when NO_COLOR is set")
	}
}

func TestShouldUseColor_ForceColor(t *testing.T) {
	got := shouldUseColor(
		mockStdout{1},
		func(int) bool { return false },
		func(key string) string {
			if key == "FORCE_COLOR" {
				return "1"
			}
			return ""
		},
	)
	if !got {
		t.Error("should return true when FORCE_COLOR is set")
	}
}

func TestShouldUseColor_DumbTerminal(t *testing.T) {
	got := shouldUseColor(
		mockStdout{1},
		func(int) bool { return true },
		func(key string) string {
			if key == "TERM" {
				return "dumb"
			}
			return ""
		},
	)
	if got {
		t.Error("should return false for dumb terminal")
	}
}

// ---------------------------------------------------------------------------
// prepareGroundRows (helper coverage)
// ---------------------------------------------------------------------------

func TestPrepareGroundRows_Basic(t *testing.T) {
	models.UseColor = false

	routes := []models.GroundRoute{
		{
			Provider:  "FlixBus",
			Type:      "bus",
			Price:     15,
			Currency:  "EUR",
			Duration:  240,
			Departure: models.GroundStop{City: "Prague", Time: "2026-07-01T08:00:00"},
			Arrival:   models.GroundStop{City: "Vienna", Time: "2026-07-01T12:00:00"},
		},
		{
			Provider:  "DB",
			Type:      "train",
			Price:     30,
			Currency:  "EUR",
			Duration:  180,
			Departure: models.GroundStop{City: "Prague", Time: "2026-07-01T10:00:00"},
			Arrival:   models.GroundStop{City: "Vienna", Time: "2026-07-01T13:00:00"},
		},
	}

	provCount, provList, rows := prepareGroundRows(context.Background(), "", routes)
	if provCount != 2 {
		t.Errorf("expected 2 providers, got %d", provCount)
	}
	if len(provList) != 2 {
		t.Errorf("expected 2 provider names, got %d", len(provList))
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

func TestPrepareGroundRows_PriceRange(t *testing.T) {
	models.UseColor = false

	routes := []models.GroundRoute{
		{
			Provider:  "RegioJet",
			Type:      "bus",
			Price:     12,
			PriceMax:  18,
			Currency:  "EUR",
			Duration:  300,
			Departure: models.GroundStop{Time: "2026-07-01T09:00:00"},
			Arrival:   models.GroundStop{Time: "2026-07-01T14:00:00"},
		},
	}

	_, _, rows := prepareGroundRows(context.Background(), "", routes)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// Price column (first) should contain the range.
	priceCol := rows[0][0]
	if priceCol == "" {
		t.Error("price column should not be empty for price range")
	}
}

func TestPrepareGroundRows_SeatsLow(t *testing.T) {
	models.UseColor = false

	seats := 2
	routes := []models.GroundRoute{
		{
			Provider:  "FlixBus",
			Type:      "bus",
			Price:     10,
			Currency:  "EUR",
			Duration:  120,
			Departure: models.GroundStop{Time: "2026-07-01T08:00:00"},
			Arrival:   models.GroundStop{Time: "2026-07-01T10:00:00"},
			SeatsLeft: &seats,
		},
	}

	_, _, rows := prepareGroundRows(context.Background(), "", routes)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}
