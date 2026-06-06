package main

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/flights"
)

const futureDate = "2999-01-01" // valid/future, avoids past-date rejection

func TestMultiCityRouteLabel_OpenJaw(t *testing.T) {
	// Open-jaw: leg 2 starts at ZNZ, not NBO (leg 1's destination). The label
	// must show both legs honestly, not collapse to "FRA → NBO → FRA".
	legs := []flights.Leg{
		{Origins: []string{"FRA"}, Destinations: []string{"NBO"}, Date: futureDate},
		{Origins: []string{"ZNZ"}, Destinations: []string{"FRA"}, Date: futureDate},
	}
	got := multiCityRouteLabel(legs)
	want := "FRA→NBO, ZNZ→FRA"
	if got != want {
		t.Errorf("multiCityRouteLabel = %q, want %q", got, want)
	}
}

func TestMultiCityRouteLabel_Connected(t *testing.T) {
	legs := []flights.Leg{
		{Origins: []string{"CDG"}, Destinations: []string{"HND"}, Date: futureDate},
		{Origins: []string{"HND"}, Destinations: []string{"ICN"}, Date: futureDate},
		{Origins: []string{"ICN"}, Destinations: []string{"CDG"}, Date: futureDate},
	}
	got := multiCityRouteLabel(legs)
	want := "CDG→HND, HND→ICN, ICN→CDG"
	if got != want {
		t.Errorf("multiCityRouteLabel = %q, want %q", got, want)
	}
}

func TestFlightsCmd_MultiCity_RejectsReturnFlag(t *testing.T) {
	cmd := flightsCmd()
	cmd.SetArgs([]string{"--leg", "CDG:HND:" + futureDate, "--leg", "HND:ICN:" + futureDate, "--return", futureDate})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error combining --leg with --return")
	}
}

func TestFlightsCmd_MultiCity_RejectsPositionalArgs(t *testing.T) {
	cmd := flightsCmd()
	cmd.SetArgs([]string{"CDG", "HND", futureDate, "--leg", "CDG:HND:" + futureDate})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error combining positional args with --leg")
	}
}

func TestFlightsCmd_MultiCity_RequiresTwoLegs(t *testing.T) {
	cmd := flightsCmd()
	cmd.SetArgs([]string{"--leg", "CDG:HND:" + futureDate})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for single --leg (min 2 required)")
	}
}

func TestFlightsCmd_MultiCity_RejectsBadLegFormat(t *testing.T) {
	cmd := flightsCmd()
	cmd.SetArgs([]string{"--leg", "CDG-HND-bad", "--leg", "HND:ICN:" + futureDate})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for malformed leg spec")
	}
}
