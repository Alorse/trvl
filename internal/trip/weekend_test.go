package trip

import (
	"testing"
)

func TestWeekendOptions_Defaults(t *testing.T) {
	opts := WeekendOptions{}
	opts.defaults()

	if opts.Nights != 2 {
		t.Errorf("Nights = %d, want 2", opts.Nights)
	}
}

func TestWeekendOptions_DefaultsPreserve(t *testing.T) {
	opts := WeekendOptions{Nights: 3}
	opts.defaults()

	if opts.Nights != 3 {
		t.Errorf("Nights = %d, want 3", opts.Nights)
	}
}

func TestParseMonth_LongFormat(t *testing.T) {
	depart, ret, display, err := parseMonth("July-2026")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if display != "July 2026" {
		t.Errorf("display = %q, want July 2026", display)
	}
	// First Friday of July 2026 is July 3.
	if depart != "2026-07-03" {
		t.Errorf("depart = %q, want 2026-07-03", depart)
	}
	if ret != "2026-07-05" {
		t.Errorf("return = %q, want 2026-07-05", ret)
	}
}

func TestParseMonth_ShortFormat(t *testing.T) {
	_, _, display, err := parseMonth("2026-08")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if display != "August 2026" {
		t.Errorf("display = %q, want August 2026", display)
	}
}

func TestParseMonth_Invalid(t *testing.T) {
	_, _, _, err := parseMonth("not-a-month")
	if err == nil {
		t.Error("expected error for invalid month")
	}
}

func TestEstimateHotelFromPriceLevel(t *testing.T) {
	tests := []struct {
		flightPrice float64
		want        float64
	}{
		{30, 40},
		{80, 60},
		{150, 80},
		{300, 100},
		{600, 130},
	}
	for _, tt := range tests {
		got := estimateHotelFromPriceLevel(tt.flightPrice)
		if got != tt.want {
			t.Errorf("estimateHotel(%v) = %v, want %v", tt.flightPrice, got, tt.want)
		}
	}
}

func TestFindWeekendGetaways_EmptyOrigin(t *testing.T) {
	_, err := FindWeekendGetaways(t.Context(), "", WeekendOptions{Month: "july-2026"})
	if err == nil {
		t.Error("expected error for empty origin")
	}
}

func TestFindWeekendGetaways_InvalidMonth(t *testing.T) {
	_, err := FindWeekendGetaways(t.Context(), "HEL", WeekendOptions{Month: "invalid"})
	if err == nil {
		t.Error("expected error for invalid month")
	}
}
