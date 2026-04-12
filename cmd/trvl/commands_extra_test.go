package main

import "testing"

// ---------------------------------------------------------------------------
// Command constructors — exercise flag registration and defaults.
// Each test creates the command, which runs the init code and flag setup.
// ---------------------------------------------------------------------------

func TestFlightsCmd_NonNil(t *testing.T) {
	cmd := flightsCmd()
	if cmd == nil {
		t.Fatal("flightsCmd() returned nil")
	}
	if f := cmd.Flags().Lookup("return"); f == nil {
		t.Error("flightsCmd missing --return flag")
	}
}

func TestDatesCmd_NonNil(t *testing.T) {
	cmd := datesCmd()
	if cmd == nil {
		t.Fatal("datesCmd() returned nil")
	}
}

func TestExploreCmd_NonNil(t *testing.T) {
	cmd := exploreCmd()
	if cmd == nil {
		t.Fatal("exploreCmd() returned nil")
	}
}

func TestGridCmd_NonNil(t *testing.T) {
	cmd := gridCmd()
	if cmd == nil {
		t.Fatal("gridCmd() returned nil")
	}
}

func TestDealsCmd_NonNil(t *testing.T) {
	cmd := dealsCmd()
	if cmd == nil {
		t.Fatal("dealsCmd() returned nil")
	}
}

func TestDiscoverCmd_NonNil(t *testing.T) {
	cmd := discoverCmd()
	if cmd == nil {
		t.Fatal("discoverCmd() returned nil")
	}
}

func TestMultiCityCmd_NonNil(t *testing.T) {
	cmd := multiCityCmd()
	if cmd == nil {
		t.Fatal("multiCityCmd() returned nil")
	}
}

func TestSuggestCmd_NonNil(t *testing.T) {
	cmd := suggestCmd()
	if cmd == nil {
		t.Fatal("suggestCmd() returned nil")
	}
}

func TestWeatherCmd_NonNil(t *testing.T) {
	cmd := weatherCmd()
	if cmd == nil {
		t.Fatal("weatherCmd() returned nil")
	}
}

func TestWeekendCmd_NonNil(t *testing.T) {
	cmd := weekendCmd()
	if cmd == nil {
		t.Fatal("weekendCmd() returned nil")
	}
}

func TestWhenCmd_NonNil(t *testing.T) {
	cmd := whenCmd()
	if cmd == nil {
		t.Fatal("whenCmd() returned nil")
	}
	// Check key flags exist.
	for _, flag := range []string{"from", "to", "until", "min-nights", "max-nights", "budget"} {
		if f := cmd.Flags().Lookup(flag); f == nil {
			t.Errorf("whenCmd missing --%s flag", flag)
		}
	}
}

func TestHacksCmd_NonNil(t *testing.T) {
	cmd := hacksCmd()
	if cmd == nil {
		t.Fatal("hacksCmd() returned nil")
	}
}

func TestTripCostCmd_NonNil(t *testing.T) {
	cmd := tripCostCmd()
	if cmd == nil {
		t.Fatal("tripCostCmd() returned nil")
	}
}

func TestAirportTransferCmd_NonNil(t *testing.T) {
	cmd := airportTransferCmd()
	if cmd == nil {
		t.Fatal("airportTransferCmd() returned nil")
	}
}

func TestPrefsCmd_NonNil(t *testing.T) {
	cmd := prefsCmd()
	if cmd == nil {
		t.Fatal("prefsCmd() returned nil")
	}
	// Should have subcommands.
	if len(cmd.Commands()) == 0 {
		t.Error("prefsCmd should have subcommands")
	}
}

func TestPrefsSetCmd_NonNil(t *testing.T) {
	cmd := prefsSetCmd()
	if cmd == nil {
		t.Fatal("prefsSetCmd() returned nil")
	}
}

func TestPrefsEditCmd_NonNil(t *testing.T) {
	cmd := prefsEditCmd()
	if cmd == nil {
		t.Fatal("prefsEditCmd() returned nil")
	}
}

func TestPrefsInitCmd_NonNil(t *testing.T) {
	cmd := prefsInitCmd()
	if cmd == nil {
		t.Fatal("prefsInitCmd() returned nil")
	}
}

func TestPrefsAddFamilyMemberCmd_NonNil(t *testing.T) {
	cmd := prefsAddFamilyMemberCmd()
	if cmd == nil {
		t.Fatal("prefsAddFamilyMemberCmd() returned nil")
	}
}

func TestTripsCmd_NonNil(t *testing.T) {
	cmd := tripsCmd()
	if cmd == nil {
		t.Fatal("tripsCmd() returned nil")
	}
	if len(cmd.Commands()) == 0 {
		t.Error("tripsCmd should have subcommands")
	}
}

func TestTripsListCmd_NonNil(t *testing.T) {
	cmd := tripsListCmd()
	if cmd == nil {
		t.Fatal("tripsListCmd() returned nil")
	}
}

func TestTripsCreateCmd_NonNil(t *testing.T) {
	cmd := tripsCreateCmd()
	if cmd == nil {
		t.Fatal("tripsCreateCmd() returned nil")
	}
}

func TestTripsShowCmd_NonNil(t *testing.T) {
	cmd := tripsShowCmd()
	if cmd == nil {
		t.Fatal("tripsShowCmd() returned nil")
	}
}

func TestTripsDeleteCmd_NonNil(t *testing.T) {
	cmd := tripsDeleteCmd()
	if cmd == nil {
		t.Fatal("tripsDeleteCmd() returned nil")
	}
}

func TestTripsBookCmd_NonNil(t *testing.T) {
	cmd := tripsBookCmd()
	if cmd == nil {
		t.Fatal("tripsBookCmd() returned nil")
	}
}

func TestTripsAddLegCmd_NonNil(t *testing.T) {
	cmd := tripsAddLegCmd()
	if cmd == nil {
		t.Fatal("tripsAddLegCmd() returned nil")
	}
}

func TestTripsStatusCmd_NonNil(t *testing.T) {
	cmd := tripsStatusCmd()
	if cmd == nil {
		t.Fatal("tripsStatusCmd() returned nil")
	}
}

func TestTripsAlertsCmd_NonNil(t *testing.T) {
	cmd := tripsAlertsCmd()
	if cmd == nil {
		t.Fatal("tripsAlertsCmd() returned nil")
	}
}

func TestWatchAddCmd_NonNil(t *testing.T) {
	cmd := watchAddCmd()
	if cmd == nil {
		t.Fatal("watchAddCmd() returned nil")
	}
}

func TestWatchListCmd_NonNil(t *testing.T) {
	cmd := watchListCmd()
	if cmd == nil {
		t.Fatal("watchListCmd() returned nil")
	}
}

func TestWatchRemoveCmd_NonNil(t *testing.T) {
	cmd := watchRemoveCmd()
	if cmd == nil {
		t.Fatal("watchRemoveCmd() returned nil")
	}
}

func TestWatchCheckCmd_NonNil(t *testing.T) {
	cmd := watchCheckCmd()
	if cmd == nil {
		t.Fatal("watchCheckCmd() returned nil")
	}
}

func TestWatchHistoryCmd_NonNil(t *testing.T) {
	cmd := watchHistoryCmd()
	if cmd == nil {
		t.Fatal("watchHistoryCmd() returned nil")
	}
}

func TestMcpInstallCmd_NonNil(t *testing.T) {
	cmd := mcpInstallCmd()
	if cmd == nil {
		t.Fatal("mcpInstallCmd() returned nil")
	}
	for _, flag := range []string{"client", "force", "dry-run"} {
		if f := cmd.Flags().Lookup(flag); f == nil {
			t.Errorf("mcpInstallCmd missing --%s flag", flag)
		}
	}
}

func TestDestinationCmd_NonNil(t *testing.T) {
	cmd := destinationCmd()
	if cmd == nil {
		t.Fatal("destinationCmd() returned nil")
	}
}

func TestGuideCmd_NonNil(t *testing.T) {
	cmd := guideCmd()
	if cmd == nil {
		t.Fatal("guideCmd() returned nil")
	}
}

func TestEventsCmd_NonNil(t *testing.T) {
	cmd := eventsCmd()
	if cmd == nil {
		t.Fatal("eventsCmd() returned nil")
	}
}

func TestPointsValueCmd_NonNil(t *testing.T) {
	cmd := pointsValueCmd()
	if cmd == nil {
		t.Fatal("pointsValueCmd() returned nil")
	}
}
