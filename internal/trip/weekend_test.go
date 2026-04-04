package trip

import (
	"fmt"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
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

func TestWeekendOptions_DefaultsNegativeNights(t *testing.T) {
	opts := WeekendOptions{Nights: -1}
	opts.defaults()
	if opts.Nights != 2 {
		t.Errorf("Nights = %d, want 2 for negative input", opts.Nights)
	}
}

func TestParseMonth_LowercaseLong(t *testing.T) {
	depart, ret, display, err := parseMonth("july-2026")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if display != "July 2026" {
		t.Errorf("display = %q, want July 2026", display)
	}
	if depart != "2026-07-03" {
		t.Errorf("depart = %q, want 2026-07-03", depart)
	}
	if ret != "2026-07-05" {
		t.Errorf("return = %q, want 2026-07-05", ret)
	}
}

func TestParseMonth_ShortName(t *testing.T) {
	_, _, display, err := parseMonth("Jan-2026")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if display != "January 2026" {
		t.Errorf("display = %q, want January 2026", display)
	}
}

func TestParseMonth_LowercaseShortName(t *testing.T) {
	_, _, display, err := parseMonth("jan-2026")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if display != "January 2026" {
		t.Errorf("display = %q, want January 2026", display)
	}
}

func TestParseMonth_FirstFridayWhenMonthStartsOnFriday(t *testing.T) {
	// January 2027 starts on a Friday.
	depart, ret, _, err := parseMonth("2027-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if depart != "2027-01-01" {
		t.Errorf("depart = %q, want 2027-01-01 (Jan 1 2027 is Friday)", depart)
	}
	if ret != "2027-01-03" {
		t.Errorf("return = %q, want 2027-01-03", ret)
	}
}

func TestParseMonth_FirstFridayWhenMonthStartsOnSaturday(t *testing.T) {
	// May 2027 starts on Saturday, so first Friday is May 7.
	depart, _, _, err := parseMonth("2027-05")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if depart != "2027-05-07" {
		t.Errorf("depart = %q, want 2027-05-07 (first Friday after Sat May 1)", depart)
	}
}

func TestBuildWeekendResult_BasicSort(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Expensive", AirportCode: "EXP", Price: 300, Stops: 1},
		{CityName: "Cheap", AirportCode: "CHP", Price: 30, Stops: 0},
		{CityName: "Medium", AirportCode: "MED", Price: 150, Stops: 1},
	}
	opts := WeekendOptions{Nights: 2}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, dests, "PLN")

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Count != 3 {
		t.Errorf("count = %d, want 3", result.Count)
	}
	if result.Origin != "HEL" {
		t.Errorf("origin = %q, want HEL", result.Origin)
	}
	if result.Month != "July 2026" {
		t.Errorf("month = %q, want July 2026", result.Month)
	}
	// Cheapest total should be first.
	if result.Destinations[0].AirportCode != "CHP" {
		t.Errorf("first dest = %q, want CHP (cheapest)", result.Destinations[0].AirportCode)
	}
	// Verify hotel estimate for cheap flight (30 EUR -> 40/night * 2 = 80).
	if result.Destinations[0].HotelEstimate != 80 {
		t.Errorf("hotel estimate = %v, want 80", result.Destinations[0].HotelEstimate)
	}
	// Total = 30 + 80 = 110.
	if result.Destinations[0].TotalEstimate != 110 {
		t.Errorf("total estimate = %v, want 110", result.Destinations[0].TotalEstimate)
	}
}

func TestBuildWeekendResult_BudgetFilter(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Cheap", AirportCode: "CHP", Price: 30, Stops: 0},
		{CityName: "Expensive", AirportCode: "EXP", Price: 500, Stops: 2},
	}
	opts := WeekendOptions{Nights: 2, MaxBudget: 200}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, dests, "PLN")

	if result.Count != 1 {
		t.Errorf("count = %d, want 1 (expensive filtered out)", result.Count)
	}
	if result.Destinations[0].AirportCode != "CHP" {
		t.Errorf("dest = %q, want CHP", result.Destinations[0].AirportCode)
	}
}

func TestBuildWeekendResult_EmptyDestinations(t *testing.T) {
	opts := WeekendOptions{Nights: 2}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, nil, "PLN")

	if !result.Success {
		t.Fatal("expected success even with empty destinations")
	}
	if result.Count != 0 {
		t.Errorf("count = %d, want 0", result.Count)
	}
}

func TestBuildWeekendResult_Max10(t *testing.T) {
	// Create 15 destinations; should only return top 10.
	var dests []models.ExploreDestination
	for i := 0; i < 15; i++ {
		dests = append(dests, models.ExploreDestination{
			CityName:    fmt.Sprintf("City%d", i),
			AirportCode: fmt.Sprintf("C%02d", i),
			Price:       float64(50 + i*10),
		})
	}
	opts := WeekendOptions{Nights: 2}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, dests, "PLN")

	if result.Count != 10 {
		t.Errorf("count = %d, want 10 (capped)", result.Count)
	}
}

func TestBuildWeekendResult_FallbackCityName(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "", AirportCode: "BCN", Price: 100},
	}
	opts := WeekendOptions{Nights: 2}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, dests, "PLN")

	// BCN should resolve to "Barcelona" via LookupAirportName.
	if result.Destinations[0].Destination != "Barcelona" {
		t.Errorf("destination = %q, want Barcelona (from airport lookup)", result.Destinations[0].Destination)
	}
}

func TestBuildWeekendResult_CurrencyIsEUR(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Test", AirportCode: "TST", Price: 100},
	}
	opts := WeekendOptions{Nights: 1}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, dests, "PLN")

	if result.Destinations[0].Currency != "PLN" {
		t.Errorf("currency = %q, want PLN", result.Destinations[0].Currency)
	}
}

func TestBuildWeekendResult_AirlineNamePreserved(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Test", AirportCode: "TST", Price: 100, AirlineName: "Finnair"},
	}
	opts := WeekendOptions{Nights: 2}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, dests, "PLN")

	if result.Destinations[0].AirlineName != "Finnair" {
		t.Errorf("airline = %q, want Finnair", result.Destinations[0].AirlineName)
	}
}

func TestBuildWeekendResult_BudgetFilterRemovesAll(t *testing.T) {
	dests := []models.ExploreDestination{
		{CityName: "Expensive", AirportCode: "EXP", Price: 500},
	}
	opts := WeekendOptions{Nights: 2, MaxBudget: 100}
	opts.defaults()

	result := buildWeekendResult("HEL", "July 2026", opts, dests, "PLN")

	if result.Count != 0 {
		t.Errorf("count = %d, want 0 (all filtered by budget)", result.Count)
	}
}

func TestEstimateHotelFromPriceLevel_BoundaryValues(t *testing.T) {
	tests := []struct {
		name        string
		flightPrice float64
		want        float64
	}{
		{"exactly 0", 0, 40},
		{"exactly 50", 50, 60},
		{"exactly 100", 100, 80},
		{"exactly 200", 200, 100},
		{"exactly 400", 400, 130},
		{"just below 50", 49.99, 40},
		{"just below 100", 99.99, 60},
		{"just below 200", 199.99, 80},
		{"just below 400", 399.99, 100},
		{"very expensive", 1000, 130},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateHotelFromPriceLevel(tt.flightPrice)
			if got != tt.want {
				t.Errorf("estimateHotel(%v) = %v, want %v", tt.flightPrice, got, tt.want)
			}
		})
	}
}
