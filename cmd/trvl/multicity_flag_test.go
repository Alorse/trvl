package main

import "testing"

const futureDate = "2999-01-01" // valid/future, avoids past-date rejection

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
