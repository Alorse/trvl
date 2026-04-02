package trip

import (
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestSmartDateOptions_Defaults(t *testing.T) {
	opts := SmartDateOptions{}
	opts.defaults()

	if opts.FlexDays != 7 {
		t.Errorf("FlexDays = %d, want 7", opts.FlexDays)
	}
}

func TestSmartDateOptions_DefaultsRoundTrip(t *testing.T) {
	opts := SmartDateOptions{RoundTrip: true}
	opts.defaults()

	if opts.Duration != 7 {
		t.Errorf("Duration = %d, want 7", opts.Duration)
	}
}

func TestSmartDateOptions_DefaultsPreserve(t *testing.T) {
	opts := SmartDateOptions{FlexDays: 14, RoundTrip: true, Duration: 10}
	opts.defaults()

	if opts.FlexDays != 14 {
		t.Errorf("FlexDays = %d, want 14", opts.FlexDays)
	}
	if opts.Duration != 10 {
		t.Errorf("Duration = %d, want 10", opts.Duration)
	}
}

func TestSuggestDates_EmptyOrigin(t *testing.T) {
	_, err := SuggestDates(t.Context(), "", "BCN", SmartDateOptions{TargetDate: "2026-07-15"})
	if err == nil {
		t.Error("expected error for empty origin")
	}
}

func TestSuggestDates_EmptyDest(t *testing.T) {
	_, err := SuggestDates(t.Context(), "HEL", "", SmartDateOptions{TargetDate: "2026-07-15"})
	if err == nil {
		t.Error("expected error for empty destination")
	}
}

func TestSuggestDates_EmptyTargetDate(t *testing.T) {
	_, err := SuggestDates(t.Context(), "HEL", "BCN", SmartDateOptions{})
	if err == nil {
		t.Error("expected error for empty target date")
	}
}

func TestSuggestDates_InvalidTargetDate(t *testing.T) {
	_, err := SuggestDates(t.Context(), "HEL", "BCN", SmartDateOptions{TargetDate: "invalid"})
	if err == nil {
		t.Error("expected error for invalid target date")
	}
}

func TestBuildInsights_Empty(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	insights := buildInsights(nil, target, 0)
	if insights != nil {
		t.Error("expected nil insights for empty dates")
	}
}

func TestBuildInsights_WithDates(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC) // Wednesday
	dates := []models.DatePriceResult{
		{Date: "2026-07-13", Price: 100, Currency: "EUR"}, // Monday
		{Date: "2026-07-14", Price: 120, Currency: "EUR"}, // Tuesday
		{Date: "2026-07-15", Price: 200, Currency: "EUR"}, // Wednesday (target)
		{Date: "2026-07-18", Price: 250, Currency: "EUR"}, // Saturday
	}

	insights := buildInsights(dates, target, 167.5)

	if len(insights) == 0 {
		t.Fatal("expected at least one insight")
	}

	// Should contain a cheapest insight.
	hasCheapest := false
	for _, ins := range insights {
		if ins.Type == "cheapest" {
			hasCheapest = true
			if !strings.Contains(ins.Description, "100") {
				t.Errorf("cheapest insight should mention price 100, got %q", ins.Description)
			}
		}
	}
	if !hasCheapest {
		t.Error("expected a 'cheapest' insight")
	}

	// Should contain a saving insight (target is 200 vs cheapest 100).
	hasSaving := false
	for _, ins := range insights {
		if ins.Type == "saving" {
			hasSaving = true
			if ins.Savings != 100 {
				t.Errorf("savings = %v, want 100", ins.Savings)
			}
		}
	}
	if !hasSaving {
		t.Error("expected a 'saving' insight")
	}
}

func TestAvg(t *testing.T) {
	tests := []struct {
		input []float64
		want  float64
	}{
		{nil, 0},
		{[]float64{}, 0},
		{[]float64{10}, 10},
		{[]float64{10, 20, 30}, 20},
	}
	for _, tt := range tests {
		got := avg(tt.input)
		if got != tt.want {
			t.Errorf("avg(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
