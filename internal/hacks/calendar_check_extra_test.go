package hacks

import (
	"context"
	"testing"
)

// --- detectCalendarConflict ---

func TestDetectCalendarConflict_EmptyDate(t *testing.T) {
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: ""}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) != 0 {
		t.Errorf("expected nil for empty Date, got %d hacks", len(got))
	}
}

func TestDetectCalendarConflict_EmptyOrigin(t *testing.T) {
	in := DetectorInput{Origin: "", Destination: "BCN", Date: "2026-07-01"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) != 0 {
		t.Errorf("expected nil for empty Origin, got %d hacks", len(got))
	}
}

func TestDetectCalendarConflict_EmptyDestination(t *testing.T) {
	in := DetectorInput{Origin: "HEL", Destination: "", Date: "2026-07-01"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) != 0 {
		t.Errorf("expected nil for empty Destination, got %d hacks", len(got))
	}
}

func TestDetectCalendarConflict_InvalidDate(t *testing.T) {
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "not-a-date"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) != 0 {
		t.Errorf("expected nil for invalid date, got %d hacks", len(got))
	}
}

func TestDetectCalendarConflict_OffPeakDate(t *testing.T) {
	// November is not a peak period.
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-11-10"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) != 0 {
		t.Errorf("expected nil for off-peak date, got %d hacks (Nov 10)", len(got))
	}
}

func TestDetectCalendarConflict_SummerPeak(t *testing.T) {
	// July 1 falls in Summer holidays peak (Jun 22 – Aug 31).
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-25"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) == 0 {
		t.Error("expected hack for summer peak date Jul 25")
	}
	if got[0].Type != "calendar_conflict" {
		t.Errorf("expected type 'calendar_conflict', got %q", got[0].Type)
	}
	if got[0].Title == "" {
		t.Error("expected non-empty title")
	}
}

func TestDetectCalendarConflict_ChristmasPeak(t *testing.T) {
	// Dec 25 is in Christmas/New Year peak.
	in := DetectorInput{Origin: "HEL", Destination: "LON", Date: "2026-12-25"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) == 0 {
		t.Error("expected hack for Christmas peak date Dec 25")
	}
}

func TestDetectCalendarConflict_OctoberHalfTerm(t *testing.T) {
	// Oct 15 is in October half-term peak (Oct 12-23).
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-10-15"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) == 0 {
		t.Error("expected hack for October half-term date Oct 15")
	}
}

func TestDetectCalendarConflict_PeakReturnsAlternativeSuggestion(t *testing.T) {
	// Aug 15 is in summer peak — alternatives should exist outside peak.
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-08-15"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) == 0 {
		t.Fatal("expected at least one hack for Aug 15")
	}
	h := got[0]
	if len(h.Steps) == 0 {
		t.Error("expected Steps in the hack")
	}
	if h.Description == "" {
		t.Error("expected non-empty Description")
	}
}

func TestDetectCalendarConflict_PeakCurrencyDefault(t *testing.T) {
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-25"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) == 0 {
		t.Fatal("expected hack for summer peak")
	}
	// Currency defaults to EUR when not set.
	if got[0].Currency != "EUR" {
		t.Errorf("expected default EUR currency, got %q", got[0].Currency)
	}
}

func TestDetectCalendarConflict_PeakCustomCurrency(t *testing.T) {
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-25", Currency: "USD"}
	got := detectCalendarConflict(context.Background(), in)
	if len(got) == 0 {
		t.Fatal("expected hack for summer peak")
	}
	if got[0].Currency != "USD" {
		t.Errorf("expected USD currency, got %q", got[0].Currency)
	}
}

// --- findPeakPeriod ---

func TestFindPeakPeriod_SummerHolidays(t *testing.T) {
	// Jul 4 is in Summer holidays.
	t0 := dateOf(2026, 7, 4)
	if p := findPeakPeriod(t0); p != "Summer holidays" {
		t.Errorf("expected 'Summer holidays', got %q", p)
	}
}

func TestFindPeakPeriod_ChristmasNewYear(t *testing.T) {
	t0 := dateOf(2025, 12, 25)
	if p := findPeakPeriod(t0); p != "Christmas/New Year" {
		t.Errorf("expected 'Christmas/New Year', got %q", p)
	}
}

func TestFindPeakPeriod_NewYearJan3(t *testing.T) {
	t0 := dateOf(2026, 1, 3)
	if p := findPeakPeriod(t0); p != "Christmas/New Year" {
		t.Errorf("expected 'Christmas/New Year', got %q", p)
	}
}

func TestFindPeakPeriod_OffPeakNov(t *testing.T) {
	t0 := dateOf(2026, 11, 10)
	if p := findPeakPeriod(t0); p != "" {
		t.Errorf("expected empty (off-peak) for Nov 10, got %q", p)
	}
}

func TestFindPeakPeriod_FebSkiWeek(t *testing.T) {
	t0 := dateOf(2026, 2, 18)
	if p := findPeakPeriod(t0); p != "February ski week" {
		t.Errorf("expected 'February ski week', got %q", p)
	}
}

func TestFindPeakPeriod_MayDay(t *testing.T) {
	t0 := dateOf(2026, 5, 2)
	if p := findPeakPeriod(t0); p != "May Day / Ascension cluster" {
		t.Errorf("expected 'May Day / Ascension cluster', got %q", p)
	}
}

// --- computeEaster ---

func TestComputeEaster_2025(t *testing.T) {
	// Easter 2025 is April 20.
	e := computeEaster(2025)
	if e.Month() != 4 || e.Day() != 20 {
		t.Errorf("2025 Easter = %v, expected April 20", e)
	}
}

func TestComputeEaster_2026(t *testing.T) {
	// Easter 2026 is April 5.
	e := computeEaster(2026)
	if e.Month() != 4 || e.Day() != 5 {
		t.Errorf("2026 Easter = %v, expected April 5", e)
	}
}

func TestComputeEaster_2024(t *testing.T) {
	// Easter 2024 is March 31.
	e := computeEaster(2024)
	if e.Month() != 3 || e.Day() != 31 {
		t.Errorf("2024 Easter = %v, expected March 31", e)
	}
}

func TestFindPeakPeriod_Easter(t *testing.T) {
	// Easter 2026 is April 5 — April 3 is within ±10 days.
	t0 := dateOf(2026, 4, 3)
	if p := findPeakPeriod(t0); p != "Easter" {
		t.Errorf("expected 'Easter' near Easter 2026, got %q", p)
	}
}
