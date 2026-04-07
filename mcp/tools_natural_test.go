package mcp

import (
	"strings"
	"testing"
)

// TestHeuristicParse_RouteIntent verifies that a generic route query produces
// intent="route" (the default) and leaves origin/destination blank when not mentioned.
func TestHeuristicParse_RouteIntent(t *testing.T) {
	p := heuristicParse("cheapest way from Helsinki to Tallinn next weekend", "2026-05-01")
	if p.Intent != "route" {
		t.Errorf("intent = %q, want route", p.Intent)
	}
	// Heuristic parser does not extract city names — that is intentional.
	// Origin/Destination resolution is left to the caller.
}

// TestHeuristicParse_FlightIntent verifies flight keyword detection.
func TestHeuristicParse_FlightIntent(t *testing.T) {
	cases := []string{
		"find me a flight from HEL to PRG",
		"flying to Tokyo next month",
		"cheapest airport deal",
	}
	for _, q := range cases {
		p := heuristicParse(q, "2026-05-01")
		if p.Intent != "flight" {
			t.Errorf("query %q: intent = %q, want flight", q, p.Intent)
		}
	}
}

// TestHeuristicParse_HotelIntent verifies hotel keyword detection.
func TestHeuristicParse_HotelIntent(t *testing.T) {
	cases := []string{
		"hotels in Prague for 3 nights",
		"find accommodation in Berlin",
		"where to stay in Lisbon",
		"book a room in Amsterdam",
	}
	for _, q := range cases {
		p := heuristicParse(q, "2026-05-01")
		if p.Intent != "hotel" {
			t.Errorf("query %q: intent = %q, want hotel", q, p.Intent)
		}
	}
}

// TestHeuristicParse_NextWeekend verifies relative date resolution.
func TestHeuristicParse_NextWeekend(t *testing.T) {
	// 2026-05-01 is a Friday; next Saturday = 2026-05-02, Monday = 2026-05-04.
	p := heuristicParse("route next weekend", "2026-05-01")
	if p.Date == "" {
		t.Error("Date should be set for 'next weekend' query")
	}
	if !strings.HasPrefix(p.Date, "2026-") {
		t.Errorf("Date = %q, expected a YYYY-MM-DD date in 2026", p.Date)
	}
	if p.CheckIn == "" || p.CheckOut == "" {
		t.Error("CheckIn/CheckOut should be set for weekend query")
	}
}

// TestHeuristicParse_DealsIntent verifies deals keyword detection.
func TestHeuristicParse_DealsIntent(t *testing.T) {
	p := heuristicParse("show me travel deals to anywhere", "2026-05-01")
	if p.Intent != "deals" {
		t.Errorf("intent = %q, want deals", p.Intent)
	}
}

// TestHeuristicParse_EmptyQuery verifies that an empty query returns a default struct.
func TestHeuristicParse_EmptyQuery(t *testing.T) {
	p := heuristicParse("", "2026-05-01")
	// Should not panic; intent defaults to "route".
	if p.Intent != "route" {
		t.Errorf("intent = %q, want route for empty query", p.Intent)
	}
}
