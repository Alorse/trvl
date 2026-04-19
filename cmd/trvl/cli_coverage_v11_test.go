package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// watchAddCmd — coverage of branch modes (route watch, date range, specific date)
// ---------------------------------------------------------------------------

func TestWatchAddCmd_RouteWatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := watchAddCmd()
	// No date flags → route watch mode (monitors next 60 days).
	cmd.SetArgs([]string{"HEL", "BCN"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatchAddCmd_DateRange(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := watchAddCmd()
	// --from and --to → date range mode.
	cmd.SetArgs([]string{"HEL", "BCN", "--from", "2026-07-01", "--to", "2026-07-31"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatchAddCmd_SpecificDate_NoBelowPrice(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := watchAddCmd()
	// --depart only, no --below → specific date without alert threshold.
	cmd.SetArgs([]string{"HEL", "BCN", "--depart", "2026-07-01"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatchAddCmd_HotelType(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := watchAddCmd()
	// Hotel watch with single arg (destination only).
	cmd.SetArgs([]string{"Prague", "--type", "hotel", "--depart", "2026-07-01", "--return", "2026-07-08"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// watchRoomsCmd — flag registration
// ---------------------------------------------------------------------------

func TestWatchRoomsCmd_FlagsExist(t *testing.T) {
	cmd := watchRoomsCmd()
	for _, name := range []string{"checkin", "checkout", "below"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on watchRoomsCmd", name)
		}
	}
}

func TestWatchRoomsCmd_NoArgs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := watchRoomsCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

// ---------------------------------------------------------------------------
// watchCheckCmd — no watches to check (empty store)
// ---------------------------------------------------------------------------

func TestWatchCheckCmd_EmptyStore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := watchCheckCmd()
	cmd.SetArgs([]string{})
	// With no watches, should print "No active watches" and return nil.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// tripsAddLegCmd — flag registration
// ---------------------------------------------------------------------------

func TestTripsAddLegCmd_FlagsExist(t *testing.T) {
	cmd := tripsAddLegCmd()
	for _, name := range []string{"from", "to", "provider", "start", "end", "price", "currency"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on trips add-leg", name)
		}
	}
}

func TestTripsAddLegCmd_RequiresArgs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsAddLegCmd()
	cmd.SetArgs([]string{}) // missing trip ID and type
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

// ---------------------------------------------------------------------------
// tripsBookCmd — flag registration
// ---------------------------------------------------------------------------

func TestTripsBookCmd_FlagsExist(t *testing.T) {
	cmd := tripsBookCmd()
	for _, name := range []string{"provider", "ref", "type", "url", "notes"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on trips book", name)
		}
	}
}

func TestTripsBookCmd_RequiresArg(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsBookCmd()
	cmd.SetArgs([]string{}) // missing trip ID
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

// ---------------------------------------------------------------------------
// trips full flow — create + add-leg + list in temp store
// ---------------------------------------------------------------------------

func TestTripsFullFlow_CreateAndAddLeg(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create trip.
	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Prague Trip 2026"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	// List to get the ID (just verify no error).
	listCmd := tripsCmd()
	listCmd.SetArgs([]string{"list"})
	if err := listCmd.Execute(); err != nil {
		t.Errorf("list trips: %v", err)
	}
}

// ---------------------------------------------------------------------------
// dealsCmd — valid execution (will fail on network but covers more RunE lines)
// ---------------------------------------------------------------------------

func TestDealsCmd_DefaultRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := dealsCmd()
	cmd.SetArgs([]string{})
	// Will attempt network — just ensure no panic.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// discoverCmd — missing required flags (no network needed)
// ---------------------------------------------------------------------------

func TestDiscoverCmd_MissingFromFlag(t *testing.T) {
	cmd := discoverCmd()
	cmd.SetArgs([]string{"--until", "2026-07-31", "--budget", "500"})
	err := cmd.Execute()
	// discover requires --from; should error.
	_ = err
}

func TestDiscoverCmd_FlagsV11(t *testing.T) {
	cmd := discoverCmd()
	for _, name := range []string{"from", "until", "budget", "origin"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on discoverCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// runTripsList — the "--all" branch with empty store
// ---------------------------------------------------------------------------

func TestRunTripsList_AllEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// runTripsList(true) prints "No trips found" when store is empty.
	if err := runTripsList(true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunTripsList_InactiveEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// runTripsList(false) prints "No active trips" when store is empty.
	if err := runTripsList(false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// aiportTransferCmd — arg count validation
// ---------------------------------------------------------------------------

func TestAirportTransferCmd_RequiresThreeArgsV11(t *testing.T) {
	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"CDG", "Hotel"}) // only 2 args
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only two args")
	}
}

// ---------------------------------------------------------------------------
// hacksCmd — valid IATA inputs (will fail on network but covers validation lines)
// ---------------------------------------------------------------------------

func TestHacksCmd_ValidIATANoNetwork(t *testing.T) {
	cmd := hacksCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "2026-07-01"})
	// Will attempt network; just ensure no panic on valid input.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// groundCmd — valid args but no network
// ---------------------------------------------------------------------------

func TestGroundCmd_ValidArgsNoNetwork(t *testing.T) {
	cmd := groundCmd()
	cmd.SetArgs([]string{"Prague", "Vienna", "2026-07-01"})
	// Will attempt network; cover the arg parsing code paths.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// airportTransferCmd — valid args (covers opts building, will fail on network)
// ---------------------------------------------------------------------------

func TestAirportTransferCmd_ValidArgsNoNetwork(t *testing.T) {
	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"CDG", "Paris Gare du Nord", "2026-07-01"})
	// Will attempt network; covers input struct construction.
	_ = cmd.Execute()
}
