package flights

import (
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// --- filterFlightsByAirline ---

func TestFilterFlightsByAirline_MatchingAirline(t *testing.T) {
	flights := []models.FlightResult{
		{Provider: "google", Legs: []models.FlightLeg{{AirlineCode: "AY"}}},
		{Provider: "google", Legs: []models.FlightLeg{{AirlineCode: "LH"}}},
		{Provider: "google", Legs: []models.FlightLeg{{AirlineCode: "BA"}}},
	}
	got := filterFlightsByAirline(flights, []string{"AY", "ba"})
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
	for _, f := range got {
		code := f.Legs[0].AirlineCode
		if code != "AY" && code != "BA" {
			t.Errorf("unexpected airline %q in result", code)
		}
	}
}

func TestFilterFlightsByAirline_NoMatch(t *testing.T) {
	flights := []models.FlightResult{
		{Provider: "google", Legs: []models.FlightLeg{{AirlineCode: "LH"}}},
	}
	got := filterFlightsByAirline(flights, []string{"AY"})
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

func TestFilterFlightsByAirline_EmptyAirlines(t *testing.T) {
	flights := []models.FlightResult{
		{Provider: "google", Legs: []models.FlightLeg{{AirlineCode: "LH"}}},
	}
	// Empty airlines list means no filter (return all).
	got := filterFlightsByAirline(flights, []string{})
	if len(got) != 1 {
		t.Errorf("expected 1 result with empty filter, got %d", len(got))
	}
}

func TestFilterFlightsByAirline_EmptyFlights(t *testing.T) {
	got := filterFlightsByAirline(nil, []string{"AY"})
	if len(got) != 0 {
		t.Errorf("expected 0 results for nil input, got %d", len(got))
	}
}

func TestFilterFlightsByAirline_CaseInsensitive(t *testing.T) {
	flights := []models.FlightResult{
		{Legs: []models.FlightLeg{{AirlineCode: "AY"}}},
	}
	got := filterFlightsByAirline(flights, []string{"ay"})
	if len(got) != 1 {
		t.Errorf("expected 1 case-insensitive match, got %d", len(got))
	}
}

// --- flightArrival ---

func TestFlightArrival_NoLegs(t *testing.T) {
	f := models.FlightResult{}
	got := flightArrival(f)
	if !got.IsZero() {
		t.Errorf("expected zero time for no legs, got %v", got)
	}
}

func TestFlightArrival_SingleLeg(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{
			{ArrivalTime: "2026-07-01T14:30"},
		},
	}
	got := flightArrival(f)
	want, _ := time.Parse(flightTimeLayout, "2026-07-01T14:30")
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestFlightArrival_MultiLeg(t *testing.T) {
	// Arrival time is from the LAST leg.
	f := models.FlightResult{
		Legs: []models.FlightLeg{
			{ArrivalTime: "2026-07-01T10:00"},
			{ArrivalTime: "2026-07-01T14:30"},
		},
	}
	got := flightArrival(f)
	want, _ := time.Parse(flightTimeLayout, "2026-07-01T14:30")
	if !got.Equal(want) {
		t.Errorf("expected last leg arrival %v, got %v", want, got)
	}
}

func TestFlightArrival_InvalidTime(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{
			{ArrivalTime: "not-a-time"},
		},
	}
	got := flightArrival(f)
	if !got.IsZero() {
		t.Errorf("expected zero time for invalid arrival, got %v", got)
	}
}

// --- compareFlightTimes ---

func TestCompareFlightTimes_BothZero(t *testing.T) {
	got := compareFlightTimes(time.Time{}, time.Time{})
	if got != 0 {
		t.Errorf("expected 0 for two zero times, got %d", got)
	}
}

func TestCompareFlightTimes_LeftZero(t *testing.T) {
	t2 := time.Now()
	got := compareFlightTimes(time.Time{}, t2)
	if got != 1 {
		t.Errorf("expected 1 (zero is after non-zero), got %d", got)
	}
}

func TestCompareFlightTimes_RightZero(t *testing.T) {
	t1 := time.Now()
	got := compareFlightTimes(t1, time.Time{})
	if got != -1 {
		t.Errorf("expected -1 (non-zero is before zero), got %d", got)
	}
}

func TestCompareFlightTimes_LeftBefore(t *testing.T) {
	t1, _ := time.Parse(flightTimeLayout, "2026-07-01T06:00")
	t2, _ := time.Parse(flightTimeLayout, "2026-07-01T12:00")
	got := compareFlightTimes(t1, t2)
	if got != -1 {
		t.Errorf("expected -1, got %d", got)
	}
}

func TestCompareFlightTimes_LeftAfter(t *testing.T) {
	t1, _ := time.Parse(flightTimeLayout, "2026-07-01T18:00")
	t2, _ := time.Parse(flightTimeLayout, "2026-07-01T12:00")
	got := compareFlightTimes(t1, t2)
	if got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

func TestCompareFlightTimes_Equal(t *testing.T) {
	t1, _ := time.Parse(flightTimeLayout, "2026-07-01T12:00")
	t2, _ := time.Parse(flightTimeLayout, "2026-07-01T12:00")
	got := compareFlightTimes(t1, t2)
	if got != 0 {
		t.Errorf("expected 0 for equal times, got %d", got)
	}
}

// --- flightDepartsWithinWindow ---

func TestFlightDepartsWithinWindow_NoBounds(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{DepartureTime: "2026-07-01T06:00"}},
	}
	if !flightDepartsWithinWindow(f, "", "") {
		t.Error("expected true for no bounds")
	}
}

func TestFlightDepartsWithinWindow_AfterBound_Pass(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{DepartureTime: "2026-07-01T10:00"}},
	}
	if !flightDepartsWithinWindow(f, "08:00", "") {
		t.Error("expected true when departing after bound")
	}
}

func TestFlightDepartsWithinWindow_AfterBound_Fail(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{DepartureTime: "2026-07-01T06:00"}},
	}
	if flightDepartsWithinWindow(f, "08:00", "") {
		t.Error("expected false when departing before after-bound")
	}
}

func TestFlightDepartsWithinWindow_BeforeBound_Pass(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{DepartureTime: "2026-07-01T06:00"}},
	}
	if !flightDepartsWithinWindow(f, "", "08:00") {
		t.Error("expected true when departing before bound")
	}
}

func TestFlightDepartsWithinWindow_BeforeBound_Fail(t *testing.T) {
	f := models.FlightResult{
		Legs: []models.FlightLeg{{DepartureTime: "2026-07-01T22:00"}},
	}
	if flightDepartsWithinWindow(f, "", "20:00") {
		t.Error("expected false when departing after before-bound")
	}
}

func TestFlightDepartsWithinWindow_NoLegs(t *testing.T) {
	f := models.FlightResult{}
	if flightDepartsWithinWindow(f, "06:00", "22:00") {
		t.Error("expected false for flight with no legs")
	}
}

// --- tripTypeForSearch ---

func TestTripTypeForSearch_OneWay(t *testing.T) {
	opts := SearchOptions{}
	got := tripTypeForSearch(opts)
	if got != "one_way" {
		t.Errorf("expected 'one_way', got %q", got)
	}
}

func TestTripTypeForSearch_RoundTrip(t *testing.T) {
	opts := SearchOptions{ReturnDate: "2026-07-08"}
	got := tripTypeForSearch(opts)
	if got != "round_trip" {
		t.Errorf("expected 'round_trip', got %q", got)
	}
}

// --- kiwiEligibleOptions ---

func TestKiwiEligibleOptions_OneWayNoFilters(t *testing.T) {
	opts := SearchOptions{}
	if !kiwiEligibleOptions(opts) {
		t.Error("expected eligible for simple one-way with no filters")
	}
}

func TestKiwiEligibleOptions_RoundTrip(t *testing.T) {
	opts := SearchOptions{ReturnDate: "2026-07-08"}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible for round trip")
	}
}

func TestKiwiEligibleOptions_WithAirlines(t *testing.T) {
	opts := SearchOptions{Airlines: []string{"AY"}}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible with airline filter")
	}
}

func TestKiwiEligibleOptions_WithAlliances(t *testing.T) {
	opts := SearchOptions{Alliances: []string{"STAR_ALLIANCE"}}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible with alliance filter")
	}
}

func TestKiwiEligibleOptions_WithCarryOnBags(t *testing.T) {
	opts := SearchOptions{CarryOnBags: 1}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible with carry-on bag filter")
	}
}

func TestKiwiEligibleOptions_WithCheckedBags(t *testing.T) {
	opts := SearchOptions{CheckedBags: 1}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible with checked bag filter")
	}
}

func TestKiwiEligibleOptions_RequireCheckedBag(t *testing.T) {
	opts := SearchOptions{RequireCheckedBag: true}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible with RequireCheckedBag")
	}
}

func TestKiwiEligibleOptions_ExcludeBasic(t *testing.T) {
	opts := SearchOptions{ExcludeBasic: true}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible with ExcludeBasic")
	}
}

func TestKiwiEligibleOptions_LessEmissions(t *testing.T) {
	opts := SearchOptions{LessEmissions: true}
	if kiwiEligibleOptions(opts) {
		t.Error("expected not eligible with LessEmissions")
	}
}

// --- flightSearchCurrency ---

func TestFlightSearchCurrency_NilResult(t *testing.T) {
	got := flightSearchCurrency(nil)
	if got != "EUR" {
		t.Errorf("expected EUR for nil result, got %q", got)
	}
}

func TestFlightSearchCurrency_FromFlight(t *testing.T) {
	result := &models.FlightSearchResult{
		Flights: []models.FlightResult{
			{Currency: "USD"},
		},
	}
	got := flightSearchCurrency(result)
	if got != "USD" {
		t.Errorf("expected USD, got %q", got)
	}
}

func TestFlightSearchCurrency_EmptyFlights(t *testing.T) {
	result := &models.FlightSearchResult{}
	got := flightSearchCurrency(result)
	if got != "EUR" {
		t.Errorf("expected EUR for empty flights, got %q", got)
	}
}

// --- sortFlightResults (arrival sort) ---

func TestSortFlightResults_ByArrival(t *testing.T) {
	flights := []models.FlightResult{
		{
			Price: 200,
			Legs: []models.FlightLeg{
				{ArrivalTime: "2026-07-01T18:00"},
			},
		},
		{
			Price: 200,
			Legs: []models.FlightLeg{
				{ArrivalTime: "2026-07-01T10:00"},
			},
		},
	}
	sortFlightResults(flights, models.SortArrivalTime)
	if flights[0].Legs[0].ArrivalTime != "2026-07-01T10:00" {
		t.Errorf("expected earlier arrival first, got %q", flights[0].Legs[0].ArrivalTime)
	}
}

func TestSortFlightResults_ByDeparture(t *testing.T) {
	flights := []models.FlightResult{
		{
			Price: 200,
			Legs: []models.FlightLeg{
				{DepartureTime: "2026-07-01T20:00", ArrivalTime: "2026-07-01T22:00"},
			},
		},
		{
			Price: 200,
			Legs: []models.FlightLeg{
				{DepartureTime: "2026-07-01T08:00", ArrivalTime: "2026-07-01T10:00"},
			},
		},
	}
	sortFlightResults(flights, models.SortDepartureTime)
	if flights[0].Legs[0].DepartureTime != "2026-07-01T08:00" {
		t.Errorf("expected earlier departure first, got %q", flights[0].Legs[0].DepartureTime)
	}
}

// --- filterFlightsWithCheckedBag ---

func TestFilterFlightsWithCheckedBag_IncludesBag(t *testing.T) {
	bags := 1
	// filterFlightsWithCheckedBag reads the resolved estimate, which
	// filterFlightResults attaches; annotate the same way here.
	flights := []models.FlightResult{
		{CheckedBagsIncluded: &bags, Price: 200},
		{Price: 150}, // no bag info and no airline to fall back on
	}
	annotateBagEstimates(flights)
	got := filterFlightsWithCheckedBag(flights)
	if len(got) != 1 || got[0].Price != 200 {
		t.Fatalf("expected only the confirmed flight, got %+v", got)
	}
}

func TestFilterFlightsWithCheckedBag_ZeroBags(t *testing.T) {
	bags := 0
	flights := []models.FlightResult{
		{CheckedBagsIncluded: &bags, Price: 200},
	}
	got := filterFlightsWithCheckedBag(flights)
	if len(got) != 0 {
		t.Errorf("expected 0 flights with 0 bags, got %d", len(got))
	}
}
