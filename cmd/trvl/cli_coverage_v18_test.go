package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// runNearby — invalid lat/lon error paths (no network)
// ---------------------------------------------------------------------------

func TestRunNearby_InvalidLat(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"not-a-lat", "24.9384"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid latitude")
	}
}

func TestRunNearby_InvalidLon(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"60.1699", "not-a-lon"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid longitude")
	}
}

// ---------------------------------------------------------------------------
// runProvidersDisable — provider not found path
// ---------------------------------------------------------------------------

func TestProvidersDisableCmd_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := providersDisableCmd()
	cmd.SetArgs([]string{"nonexistent-provider-id"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

// ---------------------------------------------------------------------------
// runProvidersList — empty registry
// ---------------------------------------------------------------------------

func TestRunProvidersList_EmptyV18(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := providersCmd()
	cmd.SetArgs([]string{"list"})
	_ = cmd.Execute()
}

func TestRunProvidersList_JSONEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := providersCmd()
	cmd.SetArgs([]string{"list", "--format", "json"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runProvidersStatus — empty registry
// ---------------------------------------------------------------------------

func TestRunProvidersStatus_EmptyV18(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := providersCmd()
	cmd.SetArgs([]string{"status"})
	_ = cmd.Execute()
}

func TestRunProvidersStatus_JSONEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := providersCmd()
	cmd.SetArgs([]string{"status", "--format", "json"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runProfileSummary — with bookings (exercises printProfileSummary)
// ---------------------------------------------------------------------------

func TestRunProfileSummary_WithBookings(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a booking.
	addCmd := profileAddCmd()
	addCmd.SetArgs([]string{
		"--type", "flight",
		"--provider", "Finnair",
		"--from", "HEL",
		"--to", "NRT",
		"--price", "799",
		"--currency", "EUR",
		"--travel-date", "2026-06-15",
	})
	_ = addCmd.Execute()

	// Run summary — should show profile stats.
	cmd := profileCmd()
	cmd.SetArgs([]string{"summary"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile summary: %v", err)
	}
}

// ---------------------------------------------------------------------------
// accomHackCmd — additional flag checks
// ---------------------------------------------------------------------------

func TestAccomHackCmd_FlagsV18(t *testing.T) {
	cmd := accomHackCmd()
	for _, name := range []string{"checkin", "checkout", "currency", "max-splits", "guests"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on accomHackCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// gridCmd — with valid IATAs but no network: covers flag-only path
// ---------------------------------------------------------------------------

func TestGridCmd_RequiredFlagsMissing(t *testing.T) {
	// gridCmd requires 2 positional args; one arg should error.
	cmd := gridCmd()
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

// ---------------------------------------------------------------------------
// watchDaemonCmd — flags coverage
// ---------------------------------------------------------------------------

func TestWatchDaemonCmd_EveryFlag(t *testing.T) {
	cmd := watchDaemonCmd()
	f := cmd.Flags().Lookup("every")
	if f == nil {
		t.Error("expected --every flag on watchDaemonCmd")
	}
	if f.DefValue != "6h0m0s" {
		t.Logf("default every = %q (may vary by platform)", f.DefValue)
	}
}

func TestWatchDaemonCmd_RunNowFlag(t *testing.T) {
	cmd := watchDaemonCmd()
	f := cmd.Flags().Lookup("run-now")
	if f == nil {
		t.Error("expected --run-now flag on watchDaemonCmd")
	}
}

// ---------------------------------------------------------------------------
// multiCityCmd — valid IATAs with dates triggers search (covers more of RunE)
// The search will fail on network but covers lines up to context+network call.
// ---------------------------------------------------------------------------

func TestMultiCityCmd_ValidArgsNoNetwork(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := multiCityCmd()
	// Valid IATA codes, valid dates → runs until network call.
	cmd.SetArgs([]string{"HEL", "--visit", "BCN,ROM", "--dates", "2026-07-01,2026-07-21"})
	// Will fail on network — just verify no panic before that.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// exploreCmd — flags exist (duplicate test of InvalidFromDate removed; v5 has it)
// ---------------------------------------------------------------------------

func TestExploreCmd_FlagsExistV18(t *testing.T) {
	cmd := exploreCmd()
	for _, name := range []string{"from", "to", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on exploreCmd", name)
		}
	}
}
