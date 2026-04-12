package main

import (
	"strings"
	"testing"
)

func TestCalendarCmd_NonNil(t *testing.T) {
	if cmd := calendarCmd(); cmd == nil {
		t.Fatal("calendarCmd() returned nil")
	}
}

func TestCalendarCmd_Use(t *testing.T) {
	cmd := calendarCmd()
	if !strings.HasPrefix(cmd.Use, "calendar") {
		t.Errorf("Use = %q, want to start with 'calendar'", cmd.Use)
	}
}

func TestCalendarCmd_Flags(t *testing.T) {
	cmd := calendarCmd()
	for _, name := range []string{"output", "last"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag --%s", name)
		}
	}
}

func TestCalendarCmd_RequiresArgOrLast(t *testing.T) {
	cmd := calendarCmd()
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Error("expected error with no args and no --last")
	}
}

func TestLastSearchToTrip_FlightOnly(t *testing.T) {
	ls := &LastSearch{
		Command:        "flights",
		Origin:         "HEL",
		Destination:    "NRT",
		DepartDate:     "2026-06-15",
		FlightPrice:    650,
		FlightCurrency: "EUR",
		FlightAirline:  "Finnair",
	}
	trip := lastSearchToTrip(ls)
	if trip == nil {
		t.Fatal("lastSearchToTrip returned nil")
	}
	if len(trip.Legs) != 1 {
		t.Errorf("Legs len = %d, want 1", len(trip.Legs))
	}
	if trip.Legs[0].Type != "flight" {
		t.Errorf("Type = %q, want flight", trip.Legs[0].Type)
	}
	if trip.Legs[0].Provider != "Finnair" {
		t.Errorf("Provider = %q, want Finnair", trip.Legs[0].Provider)
	}
	if !strings.Contains(trip.Name, "HEL") {
		t.Errorf("Name = %q, want to contain HEL", trip.Name)
	}
}

func TestLastSearchToTrip_RoundTripWithHotel(t *testing.T) {
	ls := &LastSearch{
		Command:        "trip",
		Origin:         "HEL",
		Destination:    "BCN",
		DepartDate:     "2026-07-01",
		ReturnDate:     "2026-07-05",
		FlightPrice:    250,
		FlightCurrency: "EUR",
		HotelPrice:     400,
		HotelCurrency:  "EUR",
		HotelName:      "Hotel Stary",
	}
	trip := lastSearchToTrip(ls)

	// Outbound + return + hotel = 3 legs.
	if got := len(trip.Legs); got != 3 {
		t.Fatalf("Legs len = %d, want 3", got)
	}
	if trip.Legs[0].Type != "flight" || trip.Legs[1].Type != "flight" || trip.Legs[2].Type != "hotel" {
		t.Errorf("expected flight,flight,hotel; got %s,%s,%s",
			trip.Legs[0].Type, trip.Legs[1].Type, trip.Legs[2].Type)
	}
	// Return leg reverses origin/destination.
	if trip.Legs[1].From != "BCN" || trip.Legs[1].To != "HEL" {
		t.Errorf("return leg: from=%q to=%q, want from=BCN to=HEL", trip.Legs[1].From, trip.Legs[1].To)
	}
}

func TestLastSearchToTrip_NoData(t *testing.T) {
	ls := &LastSearch{Command: "discover"}
	trip := lastSearchToTrip(ls)
	if len(trip.Legs) != 0 {
		t.Errorf("expected 0 legs, got %d", len(trip.Legs))
	}
}

func TestLastSearchName_WithRoute(t *testing.T) {
	ls := &LastSearch{Origin: "HEL", Destination: "NRT", DepartDate: "2026-06-15"}
	name := lastSearchName(ls)
	if !strings.Contains(name, "HEL") || !strings.Contains(name, "NRT") {
		t.Errorf("Name = %q, want to contain HEL and NRT", name)
	}
}

func TestLastSearchName_Empty(t *testing.T) {
	ls := &LastSearch{Command: "discover"}
	name := lastSearchName(ls)
	if name == "" {
		t.Error("Name should never be empty")
	}
}
