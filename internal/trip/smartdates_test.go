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

func TestBuildInsights_WeekendCheaperThanWeekday(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC) // Wednesday
	// Weekend prices lower than weekday prices to trigger the "weekend cheaper" branch.
	dates := []models.DatePriceResult{
		{Date: "2026-07-11", Price: 80, Currency: "EUR"},  // Saturday (weekend)
		{Date: "2026-07-12", Price: 90, Currency: "EUR"},  // Sunday (weekend)
		{Date: "2026-07-13", Price: 200, Currency: "EUR"}, // Monday (weekday)
		{Date: "2026-07-14", Price: 210, Currency: "EUR"}, // Tuesday (weekday)
	}

	insights := buildInsights(dates, target, 145)

	hasPattern := false
	for _, ins := range insights {
		if ins.Type == "pattern" {
			hasPattern = true
			if !strings.Contains(ins.Description, "Weekend flights average") {
				t.Errorf("expected weekend-cheaper pattern, got %q", ins.Description)
			}
		}
	}
	if !hasPattern {
		t.Error("expected a 'pattern' insight for weekend cheaper than weekday")
	}
}

func TestBuildInsights_ConsistentPrices(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	// All prices are very close, so pctSaving <= 5%.
	dates := []models.DatePriceResult{
		{Date: "2026-07-14", Price: 100, Currency: "EUR"}, // Tuesday
		{Date: "2026-07-15", Price: 102, Currency: "EUR"}, // Wednesday (target)
		{Date: "2026-07-16", Price: 104, Currency: "EUR"}, // Thursday
	}

	avgPrice := 102.0
	insights := buildInsights(dates, target, avgPrice)

	hasAvg := false
	for _, ins := range insights {
		if ins.Type == "average" {
			hasAvg = true
			if !strings.Contains(ins.Description, "fairly consistent") {
				t.Errorf("expected 'fairly consistent' for tight prices, got %q", ins.Description)
			}
		}
	}
	if !hasAvg {
		t.Error("expected an 'average' insight")
	}
}

func TestBuildInsights_TargetIsCheapest(t *testing.T) {
	target := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC) // Monday
	// Target date IS the cheapest, so savings should be 0 (no saving insight).
	dates := []models.DatePriceResult{
		{Date: "2026-07-13", Price: 100, Currency: "EUR"}, // Monday (target, cheapest)
		{Date: "2026-07-14", Price: 200, Currency: "EUR"}, // Tuesday
	}

	insights := buildInsights(dates, target, 150)

	for _, ins := range insights {
		if ins.Type == "saving" {
			t.Error("should not have a saving insight when target is cheapest")
		}
	}
}

func TestBuildInsights_TargetNotInDates(t *testing.T) {
	target := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) // Sunday (not in dates)
	dates := []models.DatePriceResult{
		{Date: "2026-07-14", Price: 100, Currency: "EUR"}, // Monday
		{Date: "2026-07-15", Price: 200, Currency: "EUR"}, // Tuesday
	}

	insights := buildInsights(dates, target, 150)

	for _, ins := range insights {
		if ins.Type == "saving" {
			t.Error("should not have a saving insight when target is not in dates")
		}
	}
}

func TestBuildInsights_OnlyWeekendDates(t *testing.T) {
	target := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC) // Saturday
	// All dates are weekends: no weekday vs weekend pattern should appear.
	dates := []models.DatePriceResult{
		{Date: "2026-07-10", Price: 100, Currency: "EUR"}, // Friday
		{Date: "2026-07-11", Price: 120, Currency: "EUR"}, // Saturday
		{Date: "2026-07-12", Price: 130, Currency: "EUR"}, // Sunday
	}

	insights := buildInsights(dates, target, 116.67)

	for _, ins := range insights {
		if ins.Type == "pattern" {
			t.Error("should not have a pattern insight when all dates are weekend")
		}
	}
}

func TestBuildInsights_OnlyWeekdayDates(t *testing.T) {
	target := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC) // Monday
	dates := []models.DatePriceResult{
		{Date: "2026-07-13", Price: 100, Currency: "EUR"}, // Monday
		{Date: "2026-07-14", Price: 120, Currency: "EUR"}, // Tuesday
		{Date: "2026-07-15", Price: 130, Currency: "EUR"}, // Wednesday
	}

	insights := buildInsights(dates, target, 116.67)

	for _, ins := range insights {
		if ins.Type == "pattern" {
			t.Error("should not have a pattern insight when all dates are weekday")
		}
	}
}

func TestBuildInsights_SingleDate(t *testing.T) {
	target := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	dates := []models.DatePriceResult{
		{Date: "2026-07-14", Price: 150, Currency: "EUR"},
	}

	insights := buildInsights(dates, target, 150)

	if len(insights) < 2 {
		t.Fatalf("expected at least 2 insights (cheapest + average), got %d", len(insights))
	}

	// Cheapest insight should exist.
	if insights[0].Type != "cheapest" {
		t.Errorf("first insight type = %q, want cheapest", insights[0].Type)
	}
}

func TestAssembleDateResult_Success(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{
		Success: true,
		Dates: []models.DatePriceResult{
			{Date: "2026-07-13", Price: 100, Currency: "EUR"},
			{Date: "2026-07-14", Price: 120, Currency: "EUR"},
			{Date: "2026-07-15", Price: 200, Currency: "EUR"},
			{Date: "2026-07-16", Price: 150, Currency: "EUR"},
		},
	}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Origin != "HEL" {
		t.Errorf("origin = %q, want HEL", result.Origin)
	}
	if result.Destination != "BCN" {
		t.Errorf("dest = %q, want BCN", result.Destination)
	}
	// Top 3 cheapest: 100, 120, 150.
	if len(result.CheapestDates) != 3 {
		t.Fatalf("cheapest dates = %d, want 3", len(result.CheapestDates))
	}
	if result.CheapestDates[0].Price != 100 {
		t.Errorf("cheapest price = %v, want 100", result.CheapestDates[0].Price)
	}
	if result.CheapestDates[0].DayOfWeek != "Monday" {
		t.Errorf("cheapest day = %q, want Monday", result.CheapestDates[0].DayOfWeek)
	}
	if result.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", result.Currency)
	}
	if len(result.Insights) == 0 {
		t.Error("expected insights")
	}
}

func TestAssembleDateResult_NoSuccess(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{Success: false}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if result.Success {
		t.Error("expected failure")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestAssembleDateResult_EmptyDates(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{Success: true, Dates: nil}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if result.Success {
		t.Error("expected failure for empty dates")
	}
}

func TestAssembleDateResult_AllZeroPrices(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{
		Success: true,
		Dates: []models.DatePriceResult{
			{Date: "2026-07-14", Price: 0, Currency: "EUR"},
			{Date: "2026-07-15", Price: 0, Currency: "EUR"},
		},
	}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if result.Success {
		t.Error("expected failure when all prices are zero")
	}
	if result.Error != "no valid prices found" {
		t.Errorf("error = %q, want 'no valid prices found'", result.Error)
	}
}

func TestAssembleDateResult_FewerThanThreeDates(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{
		Success: true,
		Dates: []models.DatePriceResult{
			{Date: "2026-07-14", Price: 100, Currency: "EUR"},
			{Date: "2026-07-15", Price: 200, Currency: "EUR"},
		},
	}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if !result.Success {
		t.Fatal("expected success")
	}
	if len(result.CheapestDates) != 2 {
		t.Errorf("cheapest dates = %d, want 2 (fewer than 3 available)", len(result.CheapestDates))
	}
}

func TestAssembleDateResult_SingleDate(t *testing.T) {
	target := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{
		Success: true,
		Dates: []models.DatePriceResult{
			{Date: "2026-07-14", Price: 150, Currency: "USD"},
		},
	}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if !result.Success {
		t.Fatal("expected success")
	}
	if len(result.CheapestDates) != 1 {
		t.Errorf("cheapest dates = %d, want 1", len(result.CheapestDates))
	}
	if result.Currency != "USD" {
		t.Errorf("currency = %q, want USD", result.Currency)
	}
	if result.AveragePrice != 150 {
		t.Errorf("average = %v, want 150", result.AveragePrice)
	}
}

func TestAssembleDateResult_MixedZeroAndValid(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{
		Success: true,
		Dates: []models.DatePriceResult{
			{Date: "2026-07-13", Price: 0, Currency: "EUR"},
			{Date: "2026-07-14", Price: 100, Currency: "EUR"},
			{Date: "2026-07-15", Price: 0, Currency: "EUR"},
			{Date: "2026-07-16", Price: 200, Currency: "EUR"},
		},
	}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if !result.Success {
		t.Fatal("expected success with some valid prices")
	}
	if len(result.CheapestDates) != 2 {
		t.Errorf("cheapest dates = %d, want 2 (only non-zero)", len(result.CheapestDates))
	}
}

func TestAssembleDateResult_ReturnDatePreserved(t *testing.T) {
	target := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	dateResult := &models.DateSearchResult{
		Success: true,
		Dates: []models.DatePriceResult{
			{Date: "2026-07-14", Price: 100, Currency: "EUR", ReturnDate: "2026-07-21"},
		},
	}

	result := assembleDateResult("HEL", "BCN", target, dateResult)

	if result.CheapestDates[0].ReturnDate != "2026-07-21" {
		t.Errorf("return date = %q, want 2026-07-21", result.CheapestDates[0].ReturnDate)
	}
}

func TestSmartDateOptions_DefaultsNonRoundTrip(t *testing.T) {
	opts := SmartDateOptions{RoundTrip: false}
	opts.defaults()
	// Duration should remain 0 when not round-trip.
	if opts.Duration != 0 {
		t.Errorf("Duration = %d, want 0 for non-round-trip", opts.Duration)
	}
}

func TestSmartDateOptions_DefaultsRoundTripPreservesDuration(t *testing.T) {
	opts := SmartDateOptions{RoundTrip: true, Duration: 14}
	opts.defaults()
	if opts.Duration != 14 {
		t.Errorf("Duration = %d, want 14", opts.Duration)
	}
}
