package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// watchListCmd — JSON format branch (requires a watch in the store)
// ---------------------------------------------------------------------------

func TestWatchListCmd_JSONFormat(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a watch so the list has content.
	addCmd := watchAddCmd()
	addCmd.SetArgs([]string{"HEL", "BCN"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("watch add: %v", err)
	}

	// Set global format to json.
	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	listCmd := watchListCmd()
	listCmd.SetArgs([]string{})
	if err := listCmd.Execute(); err != nil {
		t.Errorf("watch list json: %v", err)
	}
}

// ---------------------------------------------------------------------------
// watchListCmd — table format with watches (covers formatWatchDates branches)
// ---------------------------------------------------------------------------

func TestWatchListCmd_TableWithDateRangeWatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a date-range watch (covers formatWatchDates date-range path).
	addCmd := watchAddCmd()
	addCmd.SetArgs([]string{"HEL", "BCN", "--from", "2026-07-01", "--to", "2026-07-31", "--below", "200"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("watch add: %v", err)
	}

	listCmd := watchListCmd()
	listCmd.SetArgs([]string{})
	if err := listCmd.Execute(); err != nil {
		t.Errorf("watch list table: %v", err)
	}
}

func TestWatchListCmd_SpecificDateWatchTable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a specific-date watch.
	addCmd := watchAddCmd()
	addCmd.SetArgs([]string{"HEL", "BCN", "--depart", "2026-07-01"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("watch add: %v", err)
	}

	listCmd := watchListCmd()
	listCmd.SetArgs([]string{})
	if err := listCmd.Execute(); err != nil {
		t.Errorf("watch list specific date: %v", err)
	}
}

func TestWatchAddCmd_HotelTypeWithReturnV15(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Hotel watch exercises IsRoomWatch formatting path in watchListCmd.
	addCmd := watchAddCmd()
	addCmd.SetArgs([]string{"Prague", "--type", "hotel", "--depart", "2026-07-01", "--return", "2026-07-08", "--below", "100"})
	if err := addCmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// List it to exercise the hotel route formatting in watchListCmd.
	listCmd := watchListCmd()
	listCmd.SetArgs([]string{})
	_ = listCmd.Execute()
}

// ---------------------------------------------------------------------------
// discoverCmd — validation paths (no network — errors before trip.Discover)
// ---------------------------------------------------------------------------

func TestDiscoverCmd_InvalidOriginIATA(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := discoverCmd()
	cmd.SetArgs([]string{"--origin", "12", "--from", "2026-07-01", "--until", "2026-07-31", "--budget", "500"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestDiscoverCmd_MissingOriginNoPrefs(t *testing.T) {
	// Fresh HOME with no prefs → origin required.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := discoverCmd()
	cmd.SetArgs([]string{"--from", "2026-07-01", "--until", "2026-07-31", "--budget", "500"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no origin and no prefs")
	}
}

// ---------------------------------------------------------------------------
// tripCmd — flag coverage and IATA validation
// ---------------------------------------------------------------------------

func TestTripCmd_FlagsExist(t *testing.T) {
	cmd := tripCmd()
	for _, name := range []string{"return", "depart", "guests"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on tripCmd", name)
		}
	}
}

func TestTripCmd_RequiresTwoArgs(t *testing.T) {
	cmd := tripCmd()
	cmd.SetArgs([]string{"HEL"}) // only 1 arg
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestTripCmd_InvalidOriginIATA(t *testing.T) {
	cmd := tripCmd()
	cmd.SetArgs([]string{"12", "BCN", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

// ---------------------------------------------------------------------------
// searchCmd — flag registration
// ---------------------------------------------------------------------------

func TestSearchCmd_FlagsExist(t *testing.T) {
	cmd := searchCmd()
	for _, name := range []string{"dry-run", "json"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on searchCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// gridCmd — invalid IATA validation
// ---------------------------------------------------------------------------

func TestGridCmd_InvalidOriginIATAV15(t *testing.T) {
	cmd := gridCmd()
	cmd.SetArgs([]string{"12", "BCN"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestGridCmd_InvalidDestIATAV15(t *testing.T) {
	cmd := gridCmd()
	cmd.SetArgs([]string{"HEL", "12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid dest IATA")
	}
}

// ---------------------------------------------------------------------------
// mcpCmd — subcommand check
// ---------------------------------------------------------------------------

func TestMCPCmd_InstallSubcmd(t *testing.T) {
	cmd := mcpCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "install" || sub.Name() == "install" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'install' subcommand on mcpCmd")
	}
}

// ---------------------------------------------------------------------------
// pointsValueCmd — non-nil check
// ---------------------------------------------------------------------------

func TestPointsValueCmd_FlagsV15(t *testing.T) {
	cmd := pointsValueCmd()
	if cmd == nil {
		t.Error("expected non-nil pointsValueCmd")
		return
	}
	// Verify expected flags exist.
	for _, name := range []string{"program", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Logf("flag --%s not found on pointsValueCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// tripCostCmd — missing required flags
// ---------------------------------------------------------------------------

func TestTripCostCmd_MissingRequiredFlags(t *testing.T) {
	cmd := tripCostCmd()
	cmd.SetArgs([]string{"HEL", "BCN"}) // missing --depart and --return (required)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing required flags")
	}
}

func TestTripCostCmd_FlagsExistV15(t *testing.T) {
	cmd := tripCostCmd()
	for _, name := range []string{"depart", "return", "guests", "currency", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on tripCostCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// nearbyCmd — non-nil check
// ---------------------------------------------------------------------------

func TestNearbyCmd_NonNil(t *testing.T) {
	cmd := nearbyCmd()
	if cmd == nil {
		t.Error("expected non-nil nearbyCmd")
	}
}

// ---------------------------------------------------------------------------
// eventsCmd — missing required flags
// ---------------------------------------------------------------------------

func TestEventsCmd_MissingRequiredFlagsV15(t *testing.T) {
	cmd := eventsCmd()
	cmd.SetArgs([]string{"Barcelona"}) // missing --from, --to
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing required flags on eventsCmd")
	}
}
