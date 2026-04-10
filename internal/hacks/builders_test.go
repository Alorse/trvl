package hacks

import (
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestBuildHiddenCityHack(t *testing.T) {
	in := DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
		Date:        "2026-05-01",
	}
	h := buildHiddenCityHack(in, "PMI", 120, 180, "EUR", "VY")

	if h.Type != "hidden_city" {
		t.Errorf("Type = %q, want hidden_city", h.Type)
	}
	if h.Savings != 60 {
		t.Errorf("Savings = %v, want 60", h.Savings)
	}
	if h.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", h.Currency)
	}
	if !strings.Contains(h.Description, "PMI") {
		t.Error("description should mention beyond city")
	}
	if !strings.Contains(h.Description, "BCN") {
		t.Error("description should mention destination")
	}
	if len(h.Risks) < 3 {
		t.Errorf("expected at least 3 risks, got %d", len(h.Risks))
	}
	if len(h.Steps) < 3 {
		t.Errorf("expected at least 3 steps, got %d", len(h.Steps))
	}
	if len(h.Citations) == 0 {
		t.Error("expected at least 1 citation")
	}
}

func TestBuildNightHack(t *testing.T) {
	in := DetectorInput{
		Origin:      "PRG",
		Destination: "VIE",
		Date:        "2026-05-01",
		Currency:    "EUR",
	}
	r := models.GroundRoute{
		Provider:  "flixbus",
		Type:      "bus",
		Price:     15,
		Currency:  "EUR",
		Duration:  240,
		Departure: models.GroundStop{City: "Prague", Time: "2026-05-01T22:00"},
		Arrival:   models.GroundStop{City: "Vienna", Time: "2026-05-02T06:00"},
	}

	h := buildNightHack(in, r, 80)
	if h.Type != "night_transport" {
		t.Errorf("Type = %q, want night_transport", h.Type)
	}
	if h.Savings != 80 {
		t.Errorf("Savings = %v, want 80", h.Savings)
	}
	if !strings.Contains(h.Description, "Prague") {
		t.Error("description should mention departure city")
	}
	if !strings.Contains(h.Description, "Vienna") {
		t.Error("description should mention arrival city")
	}
	if len(h.Steps) < 2 {
		t.Errorf("expected at least 2 steps, got %d", len(h.Steps))
	}
}

func TestBuildStopoverHack(t *testing.T) {
	in := DetectorInput{
		Origin:      "HEL",
		Destination: "JFK",
		Date:        "2026-06-01",
		Currency:    "EUR",
	}
	prog := StopoverProgram{
		Airline:      "Icelandair",
		Hub:          "KEF",
		MaxNights:    7,
		Restrictions: "Must book directly",
		URL:          "https://www.icelandair.com/stopover/",
	}
	f := models.FlightResult{
		Price:    500,
		Currency: "EUR",
	}

	h := buildStopoverHack(in, prog, f, "KEF")
	if h.Type != "stopover" {
		t.Errorf("Type = %q, want stopover", h.Type)
	}
	if !strings.Contains(h.Description, "Reykjavik") {
		t.Error("description should mention hub city name")
	}
	if !strings.Contains(h.Description, "Icelandair") {
		t.Error("description should mention airline")
	}
	if len(h.Risks) < 3 {
		t.Errorf("expected at least 3 risks, got %d", len(h.Risks))
	}
}
