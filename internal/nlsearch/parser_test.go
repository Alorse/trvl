package nlsearch

import "testing"

func TestHeuristic_HotelIntent(t *testing.T) {
	cases := []string{
		"hotels in Prague for 3 nights",
		"cheap hostel in Berlin",
		"need accommodation in Tokyo",
		"where to stay in Lisbon",
		"a room in Paris",
	}
	for _, q := range cases {
		p := Heuristic(q, "2026-04-12")
		if p.Intent != "hotel" {
			t.Errorf("query %q: intent = %q, want hotel", q, p.Intent)
		}
	}
}

func TestHeuristic_FlightIntent(t *testing.T) {
	cases := []string{
		"fly from Helsinki to Tokyo",
		"flying to Mumbai next week",
		"cheapest flight HEL-NRT",
		"airport transfer in Singapore", // matches "airport"
	}
	for _, q := range cases {
		p := Heuristic(q, "2026-04-12")
		if p.Intent != "flight" {
			t.Errorf("query %q: intent = %q, want flight", q, p.Intent)
		}
	}
}

func TestHeuristic_DealsIntent(t *testing.T) {
	for _, q := range []string{"travel deals to Asia", "looking for inspiration"} {
		p := Heuristic(q, "2026-04-12")
		if p.Intent != "deals" {
			t.Errorf("query %q: intent = %q, want deals", q, p.Intent)
		}
	}
}

func TestHeuristic_RouteFallback(t *testing.T) {
	// No keyword → defaults to route.
	p := Heuristic("HEL to BCN trip", "2026-04-12")
	if p.Intent != "route" {
		t.Errorf("intent = %q, want route", p.Intent)
	}
}

func TestHeuristic_FromToPattern(t *testing.T) {
	p := Heuristic("flight from HEL to NRT next weekend", "2026-04-12")
	if p.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", p.Origin)
	}
	if p.Destination != "NRT" {
		t.Errorf("Destination = %q, want NRT", p.Destination)
	}
	if p.Date == "" {
		t.Error("Date should be resolved from 'next weekend'")
	}
}

func TestHeuristic_BareIATA(t *testing.T) {
	// Two bare IATA codes — first is origin, second is destination.
	p := Heuristic("trvl flights HEL NRT 2026-06-15", "2026-04-12")
	if p.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", p.Origin)
	}
	if p.Destination != "NRT" {
		t.Errorf("Destination = %q, want NRT", p.Destination)
	}
	if p.Date != "2026-06-15" {
		t.Errorf("Date = %q, want 2026-06-15", p.Date)
	}
}

func TestHeuristic_ISODateRange(t *testing.T) {
	p := Heuristic("trip HEL BCN 2026-06-15 to 2026-06-22", "2026-04-12")
	if p.Date != "2026-06-15" {
		t.Errorf("Date = %q, want 2026-06-15", p.Date)
	}
	if p.ReturnDate != "2026-06-22" {
		t.Errorf("ReturnDate = %q, want 2026-06-22", p.ReturnDate)
	}
}

func TestHeuristic_NextWeekend(t *testing.T) {
	// 2026-04-12 is a Sunday. Next Saturday = 2026-04-18.
	p := Heuristic("anywhere next weekend", "2026-04-12")
	if p.Date != "2026-04-18" {
		t.Errorf("Date = %q, want 2026-04-18", p.Date)
	}
	if p.CheckIn != "2026-04-18" {
		t.Errorf("CheckIn = %q, want 2026-04-18", p.CheckIn)
	}
	if p.CheckOut != "2026-04-20" {
		t.Errorf("CheckOut = %q, want 2026-04-20", p.CheckOut)
	}
}

func TestHeuristic_NoWeekendKeyword(t *testing.T) {
	p := Heuristic("flights to Bali", "2026-04-12")
	if p.Date != "" {
		t.Errorf("Date should be empty when no date keyword, got %q", p.Date)
	}
}

func TestHeuristic_FilterFalsePositive(t *testing.T) {
	// "THE" and "AND" should NOT be treated as IATA codes.
	p := Heuristic("THE flight from HEL to NRT AND back", "2026-04-12")
	if p.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", p.Origin)
	}
	if p.Destination != "NRT" {
		t.Errorf("Destination = %q, want NRT", p.Destination)
	}
}

func TestHeuristic_HotelLocationFromIATA(t *testing.T) {
	p := Heuristic("hotel in BCN", "2026-04-12")
	if p.Intent != "hotel" {
		t.Fatalf("Intent = %q, want hotel", p.Intent)
	}
	if p.Location != "BCN" {
		t.Errorf("Location = %q, want BCN", p.Location)
	}
}
