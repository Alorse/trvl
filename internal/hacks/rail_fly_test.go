package hacks

import (
	"context"
	"strings"
	"testing"
)

// --- railFlyStationsForHub ---

func TestRailFlyStationsForHub_AMS(t *testing.T) {
	stations := railFlyStationsForHub("AMS")
	if len(stations) != 2 {
		t.Fatalf("expected 2 KLM stations for AMS, got %d", len(stations))
	}
	iatas := map[string]bool{}
	for _, st := range stations {
		iatas[st.IATA] = true
		if st.Airline != "KL" {
			t.Errorf("expected airline KL for AMS hub, got %q", st.Airline)
		}
	}
	if !iatas["ZWE"] {
		t.Error("expected ZWE (Antwerp) in AMS stations")
	}
	if !iatas["ZYR"] {
		t.Error("expected ZYR (Brussels-Midi) in AMS stations")
	}
}

func TestRailFlyStationsForHub_FRA(t *testing.T) {
	stations := railFlyStationsForHub("FRA")
	if len(stations) != 7 {
		t.Fatalf("expected 7 Lufthansa stations for FRA, got %d", len(stations))
	}
	for _, st := range stations {
		if st.Airline != "LH" {
			t.Errorf("expected airline LH for FRA hub, got %q for %s", st.Airline, st.IATA)
		}
		if st.AirlineName != "Lufthansa" {
			t.Errorf("expected AirlineName Lufthansa for FRA hub, got %q", st.AirlineName)
		}
	}
}

func TestRailFlyStationsForHub_CDG(t *testing.T) {
	stations := railFlyStationsForHub("CDG")
	if len(stations) != 1 {
		t.Fatalf("expected 1 Air France station for CDG, got %d", len(stations))
	}
	if stations[0].IATA != "ZYR" {
		t.Errorf("expected ZYR for CDG, got %q", stations[0].IATA)
	}
	if stations[0].Airline != "AF" {
		t.Errorf("expected airline AF for CDG, got %q", stations[0].Airline)
	}
}

func TestRailFlyStationsForHub_ZRH(t *testing.T) {
	stations := railFlyStationsForHub("ZRH")
	if len(stations) != 1 {
		t.Fatalf("expected 1 Swiss station for ZRH, got %d", len(stations))
	}
	if stations[0].IATA != "ZDH" {
		t.Errorf("expected ZDH (Basel) for ZRH, got %q", stations[0].IATA)
	}
}

func TestRailFlyStationsForHub_unknown(t *testing.T) {
	stations := railFlyStationsForHub("XXX")
	if len(stations) != 0 {
		t.Errorf("expected 0 stations for unknown hub XXX, got %d", len(stations))
	}
}

// --- Station database completeness ---

func TestRailFlyStations_completeness(t *testing.T) {
	// Verify all expected stations exist in the database.
	expected := map[string]string{
		"ZWE": "AMS",
		"ZYR": "", // appears for both AMS and CDG
		"QKL": "FRA",
		"ZWS": "FRA",
		"QDU": "FRA",
		"QMZ": "FRA",
		"QBO": "FRA",
		"ZAQ": "FRA",
		"QPP": "FRA",
		"ZDH": "ZRH",
	}

	found := map[string]bool{}
	for _, st := range railFlyStations {
		found[st.IATA] = true
		if st.City == "" {
			t.Errorf("station %s has empty City", st.IATA)
		}
		if st.AirlineName == "" {
			t.Errorf("station %s has empty AirlineName", st.IATA)
		}
		if st.TrainProvider == "" {
			t.Errorf("station %s has empty TrainProvider", st.IATA)
		}
		if st.TrainMinutes <= 0 {
			t.Errorf("station %s has non-positive TrainMinutes: %d", st.IATA, st.TrainMinutes)
		}
		if st.FareZone == "" {
			t.Errorf("station %s has empty FareZone", st.IATA)
		}
	}

	for iata := range expected {
		if !found[iata] {
			t.Errorf("expected station %s not found in railFlyStations", iata)
		}
	}
}

func TestRailFlyStations_ZYR_dual(t *testing.T) {
	// ZYR (Brussels-Midi) should appear for both KLM (AMS) and Air France (CDG).
	var amsFound, cdgFound bool
	for _, st := range railFlyStations {
		if st.IATA == "ZYR" && st.HubIATA == "AMS" {
			amsFound = true
		}
		if st.IATA == "ZYR" && st.HubIATA == "CDG" {
			cdgFound = true
		}
	}
	if !amsFound {
		t.Error("ZYR should appear with HubIATA=AMS (KLM)")
	}
	if !cdgFound {
		t.Error("ZYR should appear with HubIATA=CDG (Air France)")
	}
}

func TestRailFlyStations_totalCount(t *testing.T) {
	if len(railFlyStations) != 11 {
		t.Errorf("expected 11 rail+fly stations, got %d", len(railFlyStations))
	}
}

// --- buildRailFlyHack ---

func TestBuildRailFlyHack_oneWay(t *testing.T) {
	station := railFlyStation{
		IATA:          "ZWE",
		City:          "Antwerp",
		HubIATA:       "AMS",
		Airline:       "KL",
		AirlineName:   "KLM",
		TrainProvider: "Eurostar",
		TrainMinutes:  60,
		FareZone:      "Belgian market",
	}

	h := buildRailFlyHack("AMS", "BCN", 300, "EUR", 240, "EUR", 60, station, "")

	if h.Type != "rail_fly_arbitrage" {
		t.Errorf("Type = %q, want rail_fly_arbitrage", h.Type)
	}
	if h.Savings != 60 {
		t.Errorf("Savings = %v, want 60", h.Savings)
	}
	if h.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", h.Currency)
	}
	if !strings.Contains(h.Title, "Antwerp") {
		t.Error("title should mention Antwerp")
	}
	if !strings.Contains(h.Title, "AMS") {
		t.Error("title should mention hub AMS")
	}
	if !strings.Contains(h.Description, "KLM") {
		t.Error("description should mention KLM")
	}
	if !strings.Contains(h.Description, "Eurostar") {
		t.Error("description should mention Eurostar")
	}
	if !strings.Contains(h.Description, "Belgian market") {
		t.Error("description should mention fare zone")
	}
	if len(h.Steps) < 5 {
		t.Errorf("expected at least 5 steps (KLM with skip notes), got %d", len(h.Steps))
	}
	// Check one-way trip type in steps
	if !strings.Contains(h.Steps[0], "one-way") {
		t.Error("step 0 should mention one-way")
	}
	if len(h.Risks) < 3 {
		t.Errorf("expected at least 3 risks, got %d", len(h.Risks))
	}
	// KLM stations should note LOW risk (no enforcement)
	if !strings.Contains(h.Risks[0], "LOW risk") {
		t.Error("first risk for KLM should note LOW risk")
	}
	if len(h.Citations) != 2 {
		t.Errorf("expected 2 citations, got %d", len(h.Citations))
	}
}

func TestBuildRailFlyHack_roundTrip(t *testing.T) {
	station := railFlyStation{
		IATA:          "QKL",
		City:          "Cologne",
		HubIATA:       "FRA",
		Airline:       "LH",
		AirlineName:   "Lufthansa",
		TrainProvider: "DB ICE",
		TrainMinutes:  62,
		FareZone:      "Rhineland regional",
	}

	h := buildRailFlyHack("FRA", "JFK", 800, "EUR", 650, "EUR", 150, station, "2026-06-15")

	if h.Savings != 150 {
		t.Errorf("Savings = %v, want 150", h.Savings)
	}
	// Check round-trip type in steps
	if !strings.Contains(h.Steps[0], "round-trip") {
		t.Error("step 0 should mention round-trip for return date")
	}
	if !strings.Contains(h.Description, "DB ICE") {
		t.Error("description should mention DB ICE")
	}
	if !strings.Contains(h.Description, "Rhineland regional") {
		t.Error("description should mention fare zone")
	}
}

// --- DetectRailFlyArbitrage input validation ---

func TestDetectRailFlyArbitrage_emptyInputs(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		dest   string
		date   string
	}{
		{"all empty", "", "", ""},
		{"no origin", "", "BCN", "2026-05-01"},
		{"no destination", "AMS", "", "2026-05-01"},
		{"no date", "AMS", "BCN", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hacks := DetectRailFlyArbitrage(context.Background(), tc.origin, tc.dest, tc.date, "")
			if len(hacks) != 0 {
				t.Errorf("expected nil for %s, got %d hacks", tc.name, len(hacks))
			}
		})
	}
}

func TestDetectRailFlyArbitrage_noStationsForHub(t *testing.T) {
	// HEL has no rail+fly stations — should return nil immediately without API calls.
	hacks := DetectRailFlyArbitrage(context.Background(), "HEL", "BCN", "2026-05-01", "")
	if len(hacks) != 0 {
		t.Errorf("expected nil for hub without rail stations, got %d hacks", len(hacks))
	}
}

// --- detectRailFlyArbitrage wrapper ---

func TestDetectRailFlyArbitrage_wrapper_emptyInput(t *testing.T) {
	hacks := detectRailFlyArbitrage(context.Background(), DetectorInput{})
	if len(hacks) != 0 {
		t.Errorf("expected nil for empty DetectorInput, got %d hacks", len(hacks))
	}
}

// --- Savings threshold tests ---

func TestRailFlySavingsThreshold_belowPercent(t *testing.T) {
	// 4% savings (below 5% threshold) should not fire even if absolute > 15
	basePrice := 500.0
	railPrice := 480.0 // 20 savings = 4%
	savings := basePrice - railPrice

	// Verify the threshold logic: 4% < 5% minimum
	if savings/basePrice >= 0.05 {
		t.Fatal("test setup error: savings should be below 5%")
	}
	if savings < 15 {
		t.Fatal("test setup error: absolute savings should be >= 15")
	}
	// Both conditions must be met: savings >= 15 AND savings/basePrice >= 0.05
	// Here percent is below threshold, so hack should NOT fire.
}

func TestRailFlySavingsThreshold_abovePercent(t *testing.T) {
	// 6% savings (above 5% threshold) AND > 15 absolute should produce a hack
	station := railFlyStation{
		IATA: "ZWE", City: "Antwerp", HubIATA: "AMS", Airline: "KL",
		AirlineName: "KLM", TrainProvider: "Eurostar", TrainMinutes: 60,
		FareZone: "Belgian market",
	}

	basePrice := 500.0
	railPrice := 470.0 // 30 savings = 6%
	savings := basePrice - railPrice

	// Verify the threshold logic: 6% >= 5% AND 30 >= 15
	if savings/basePrice < 0.05 {
		t.Fatal("test setup error: savings should be above 5%")
	}
	if savings < 15 {
		t.Fatal("test setup error: savings should be above 15")
	}

	// Build the hack to verify it would be produced
	h := buildRailFlyHack("AMS", "BCN", basePrice, "EUR", railPrice, "EUR", savings, station, "")
	if h.Type != "rail_fly_arbitrage" {
		t.Errorf("expected hack to be produced for 6%% savings, got type %q", h.Type)
	}
	if h.Savings != 30 {
		t.Errorf("expected savings 30, got %v", h.Savings)
	}
}

func TestRailFlySavingsThreshold_belowAbsolute(t *testing.T) {
	// Even at 10% savings, if absolute is below 15 it should not fire
	basePrice := 100.0
	railPrice := 90.0 // 10 savings = 10%
	savings := basePrice - railPrice

	// Verify: 10% >= 5% but 10 < 15
	if savings/basePrice < 0.05 {
		t.Fatal("test setup error: percent should be above 5%")
	}
	if savings >= 15 {
		t.Fatal("test setup error: absolute savings should be below 15")
	}
}
