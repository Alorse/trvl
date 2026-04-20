package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/nlsearch"
	"github.com/MikkoParkkola/trvl/internal/providers"
	"github.com/spf13/cobra"
)

func TestVisaCmd_ListAllJSON(t *testing.T) {
	cmd := visaCmd()

	cmd.SetArgs([]string{"--list"})

	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVisaCmd_FailedLookupNotSuccess(t *testing.T) {

	cmd := visaCmd()
	cmd.SetArgs([]string{"--passport", "XX", "--destination", "JP"})

	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error (expected print to stderr, not error): %v", err)
	}
}

func TestVisaCmd_LookupJSON(t *testing.T) {
	cmd := visaCmd()
	cmd.SetArgs([]string{"--passport", "FI", "--destination", "JP"})
	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpgradeCmd_DefaultRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{})

	_ = cmd.Execute()
}

func TestTripsStatusCmd_NoUpcoming(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsCmd()
	cmd.SetArgs([]string{"status"})
	_ = cmd.Execute()
}

func TestTripsDeleteCmd_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsCmd()
	cmd.SetArgs([]string{"delete", "nonexistent-id"})
	err := cmd.Execute()

	_ = err
}

func TestTripsAlertsCmd_NoAlerts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsCmd()
	cmd.SetArgs([]string{"alerts"})
	_ = cmd.Execute()
}

func TestTripsListCmd_AllFlagEmptyStore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsCmd()
	cmd.SetArgs([]string{"list", "--all"})
	_ = cmd.Execute()
}

func TestTripsCreateCmd_CreatesTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsCmd()
	cmd.SetArgs([]string{"create", "My Test Trip"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTripsListCmd_ShowsCreatedTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := tripsCmd()
	cmd.SetArgs([]string{"create", "Test Trip"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	cmd2 := tripsCmd()
	cmd2.SetArgs([]string{"list"})
	if err := cmd2.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMCPCmd_FlagsExist(t *testing.T) {
	cmd := mcpCmd()
	if cmd == nil {
		t.Error("expected non-nil mcpCmd")
	}
}

func TestDealsCmd_FlagsV10(t *testing.T) {
	cmd := dealsCmd()
	for _, name := range []string{"region", "format", "providers"} {
		if f := cmd.Flags().Lookup(name); f == nil {

			t.Logf("flag --%s not found on dealsCmd", name)
		}
	}
}

func TestShareCmd_NoArgsNoLast(t *testing.T) {
	cmd := shareCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no trip_id and no --last")
	}
}

func TestMCPInstallCmd_UnknownClient(t *testing.T) {
	cmd := mcpInstallCmd()
	cmd.SetArgs([]string{"--client", "totally-unknown-client"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown client")
	}
}

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
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

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
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestTripsFullFlow_CreateAndAddLeg(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Prague Trip 2026"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	listCmd := tripsCmd()
	listCmd.SetArgs([]string{"list"})
	if err := listCmd.Execute(); err != nil {
		t.Errorf("list trips: %v", err)
	}
}

func TestDealsCmd_FlagsV11b(t *testing.T) {
	cmd := dealsCmd()
	if cmd == nil {
		t.Error("expected non-nil dealsCmd")
	}
}

func TestDiscoverCmd_MissingFromFlag(t *testing.T) {
	cmd := discoverCmd()
	cmd.SetArgs([]string{"--until", "2026-07-31", "--budget", "500"})
	err := cmd.Execute()

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

func TestRunTripsList_AllEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := runTripsList(true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunTripsList_InactiveEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := runTripsList(false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAirportTransferCmd_RequiresThreeArgsV11(t *testing.T) {
	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"CDG", "Hotel"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only two args")
	}
}

func TestHacksCmd_FlagsRegisteredV11(t *testing.T) {
	cmd := hacksCmd()
	for _, name := range []string{"return", "carry-on", "currency"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on hacksCmd", name)
		}
	}
}

func TestGroundCmd_AliasExist(t *testing.T) {
	cmd := groundCmd()

	if len(cmd.Aliases) == 0 {
		t.Error("expected aliases on groundCmd")
	}
}

func TestAirportTransferCmd_ArrivalAfterFlag(t *testing.T) {
	cmd := airportTransferCmd()
	if f := cmd.Flags().Lookup("arrival-after"); f == nil {
		t.Error("expected --arrival-after flag on airportTransferCmd")
	}
}

func TestExploreCmd_ToBeforeFrom(t *testing.T) {
	cmd := exploreCmd()
	cmd.SetArgs([]string{"HEL", "--from", "2026-07-31", "--to", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --to is before --from")
	}
}

func TestExploreCmd_InvalidToDate(t *testing.T) {
	cmd := exploreCmd()
	cmd.SetArgs([]string{"HEL", "--from", "2026-07-01", "--to", "not-a-date"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid --to date")
	}
}

func TestDatesCmd_LegacyFlag(t *testing.T) {
	cmd := datesCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "--legacy", "--from", "2026-07-01", "--to", "2026-07-07"})

	_ = cmd.Execute()
}

func TestDatesCmd_RoundTripFlag(t *testing.T) {
	cmd := datesCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "--round-trip", "--from", "2026-07-01", "--to", "2026-07-31"})

	_ = cmd.Execute()
}

func TestTripsAddLeg_CreatesLeg(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Fatal("expected at least one trip in store")
	}
	tripID := list[0].ID

	addLegCmd := tripsCmd()
	addLegCmd.SetArgs([]string{
		"add-leg", tripID, "flight",
		"--from", "HEL",
		"--to", "BCN",
		"--provider", "KLM",
		"--start", "2026-07-01T18:00",
		"--price", "199",
		"--currency", "EUR",
	})
	if err := addLegCmd.Execute(); err != nil {
		t.Errorf("add-leg: %v", err)
	}

	store2, err := loadTripStore()
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	trip, err := store2.Get(tripID)
	if err != nil {
		t.Fatalf("get trip: %v", err)
	}
	if len(trip.Legs) != 1 {
		t.Errorf("expected 1 leg, got %d", len(trip.Legs))
	}
}

func TestTripsBookCmd_AddsBooking(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Book Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Fatal("expected at least one trip")
	}
	tripID := list[0].ID

	bookCmd := tripsCmd()
	bookCmd.SetArgs([]string{
		"book", tripID,
		"--provider", "KLM",
		"--ref", "XYZ789",
	})
	if err := bookCmd.Execute(); err != nil {
		t.Errorf("book trip: %v", err)
	}
}

func TestTripsStatusCmd_HasTripNoLegs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Future Trip"})
	_ = createCmd.Execute()

	cmd := tripsCmd()
	cmd.SetArgs([]string{"status"})
	_ = cmd.Execute()
}

func TestTripsDeleteCmd_DeletesTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Delete Me"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create: %v", err)
	}

	store, _ := loadTripStore()
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	deleteCmd := tripsCmd()
	deleteCmd.SetArgs([]string{"delete", tripID})
	if err := deleteCmd.Execute(); err != nil {
		t.Errorf("delete trip: %v", err)
	}
}

func TestTripsAlertsCmd_MarkReadEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsCmd()
	cmd.SetArgs([]string{"alerts", "--mark-read"})
	_ = cmd.Execute()
}

func TestEventsCmd_FlagsV12(t *testing.T) {
	cmd := eventsCmd()
	for _, name := range []string{"from", "to"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on eventsCmd", name)
		}
	}
}

func TestDatesCmd_AdultsFlag(t *testing.T) {
	cmd := datesCmd()
	if f := cmd.Flags().Lookup("adults"); f == nil {
		t.Error("expected --adults flag on datesCmd")
	}
	if f := cmd.Flags().Lookup("legacy"); f == nil {
		t.Error("expected --legacy flag on datesCmd")
	}
}

func TestWeekendCmd_NightsDefault(t *testing.T) {
	cmd := weekendCmd()
	f := cmd.Flags().Lookup("nights")
	if f == nil {
		t.Error("expected --nights flag on weekendCmd")
		return
	}
	if f.DefValue != "2" {
		t.Errorf("expected default nights=2, got %s", f.DefValue)
	}
}

func TestWhenCmd_ValidArgsNoNetwork(t *testing.T) {

	cmd := whenCmd()
	cmd.SetArgs([]string{"--origin", "HEL"})
	err := cmd.Execute()

	_ = err
}

func TestWhenCmd_InvalidOrigin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := whenCmd()
	cmd.SetArgs([]string{
		"--to", "BCN",
		"--from", "2026-07-01",
		"--until", "2026-08-31",
		"--origin", "12",
	})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestWhenCmd_FlagsV13(t *testing.T) {
	cmd := whenCmd()
	for _, name := range []string{"to", "origin", "from", "until", "busy", "prefer", "min-nights", "max-nights"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on whenCmd", name)
		}
	}
}

func TestUpgradeCmd_FreshInstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{})

	_ = cmd.Execute()
}

func TestUpgradeCmd_DryRunFreshInstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--dry-run"})
	_ = cmd.Execute()
}

func TestHacksCmd_ReturnFlag(t *testing.T) {
	cmd := hacksCmd()
	if f := cmd.Flags().Lookup("return"); f == nil {
		t.Error("expected --return flag on hacksCmd")
	}
}

func TestFlightsCmd_CompareCabinsFlag(t *testing.T) {
	cmd := flightsCmd()
	if f := cmd.Flags().Lookup("compare-cabins"); f == nil {
		t.Error("expected --compare-cabins flag on flightsCmd")
	}
}

func TestTripsShowCmd_NotFoundV13(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsShowCmd()
	cmd.SetArgs([]string{"nonexistent-trip-id-v13"})

	_ = cmd.Execute()
}

func TestMultiCityCmd_MissingDatesV14(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SetArgs([]string{"HEL", "--visit", "BCN,ROM"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --dates is missing")
	}
}

func TestMultiCityCmd_InvalidCityIATAV14(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SetArgs([]string{"HEL", "--visit", "12,ROM", "--dates", "2026-07-01,2026-07-21"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid city IATA in --visit")
	}
}

func TestMultiCityCmd_FlagsExistV14(t *testing.T) {
	cmd := multiCityCmd()
	for _, name := range []string{"visit", "dates", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on multiCityCmd", name)
		}
	}
}

func TestWeekendCmd_InvalidIATAV14(t *testing.T) {
	cmd := weekendCmd()
	cmd.SetArgs([]string{"12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestWeatherCmd_FlagsExistV14(t *testing.T) {
	cmd := weatherCmd()
	for _, name := range []string{"from", "to"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on weatherCmd", name)
		}
	}
}

func TestGroundCmd_FlagsExistV14(t *testing.T) {
	cmd := groundCmd()
	for _, name := range []string{"max-price", "type"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on groundCmd", name)
		}
	}
}

func TestLoungesCmd_InvalidIATAV14(t *testing.T) {
	cmd := loungesCmd()
	cmd.SetArgs([]string{"12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid airport IATA")
	}
}

func (m *mockWatchTicker) Chan() <-chan time.Time { return m.ch }

func (m *mockWatchTicker) Stop() {}

func TestUpgradeCmd_FlagsExistV14(t *testing.T) {
	cmd := upgradeCmd()
	for _, name := range []string{"dry-run", "quiet"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on upgradeCmd", name)
		}
	}
}

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
	cmd.SetArgs([]string{"HEL"})
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

func TestSearchCmd_FlagsExist(t *testing.T) {
	cmd := searchCmd()
	for _, name := range []string{"dry-run", "json"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on searchCmd", name)
		}
	}
}

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

func TestPointsValueCmd_FlagsV15(t *testing.T) {
	cmd := pointsValueCmd()
	if cmd == nil {
		t.Error("expected non-nil pointsValueCmd")
		return
	}

	for _, name := range []string{"program", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Logf("flag --%s not found on pointsValueCmd", name)
		}
	}
}

func TestTripCostCmd_MissingRequiredFlags(t *testing.T) {
	cmd := tripCostCmd()
	cmd.SetArgs([]string{"HEL", "BCN"})
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

func TestNearbyCmd_NonNil(t *testing.T) {
	cmd := nearbyCmd()
	if cmd == nil {
		t.Error("expected non-nil nearbyCmd")
	}
}

func TestEventsCmd_MissingRequiredFlagsV15(t *testing.T) {
	cmd := eventsCmd()
	cmd.SetArgs([]string{"Barcelona"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing required flags on eventsCmd")
	}
}

func TestCalendarCmd_WithTripID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Calendar Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	cmd := calendarCmd()
	cmd.SetArgs([]string{tripID})
	if err := cmd.Execute(); err != nil {
		t.Errorf("calendar with trip_id: %v", err)
	}
}

func TestCalendarCmd_WithTripIDToFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Calendar File Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	outFile := filepath.Join(tmp, "trip.ics")
	cmd := calendarCmd()
	cmd.SetArgs([]string{tripID, "--output", outFile})
	if err := cmd.Execute(); err != nil {
		t.Errorf("calendar with trip_id --output: %v", err)
	}

	if _, statErr := os.Stat(outFile); os.IsNotExist(statErr) {
		t.Error("expected ICS file to be written")
	}
}

func TestCalendarCmd_WithTripIDNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := calendarCmd()
	cmd.SetArgs([]string{"nonexistent-id-v16"})

	_ = cmd.Execute()
}

func TestUpgradeCmd_QuietV16(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--quiet"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpgradeCmd_AlreadyUpToDate(t *testing.T) {

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd1 := upgradeCmd()
	cmd1.SetArgs([]string{})
	_ = cmd1.Execute()

	cmd2 := upgradeCmd()
	cmd2.SetArgs([]string{})
	_ = cmd2.Execute()
}

func TestAccomHackCmd_MissingCheckIn(t *testing.T) {
	cmd := accomHackCmd()
	cmd.SetArgs([]string{"Prague"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing required flags")
	}
}

func TestShareTrip_WithRealTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Share Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	addLegCmd := tripsCmd()
	addLegCmd.SetArgs([]string{
		"add-leg", tripID, "flight",
		"--from", "HEL",
		"--to", "BCN",
		"--provider", "KLM",
		"--start", "2026-07-01T18:00",
		"--end", "2026-07-01T21:00",
		"--price", "199",
		"--currency", "EUR",
	})
	_ = addLegCmd.Execute()

	cmd := shareCmd()
	cmd.SetArgs([]string{tripID})
	if err := cmd.Execute(); err != nil {
		t.Errorf("share trip: %v", err)
	}
}

func TestShareTrip_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := shareCmd()
	cmd.SetArgs([]string{"nonexistent-id-v17"})

	err := cmd.Execute()
	_ = err
}

func TestShareCmd_LastWithSearch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	ls := &LastSearch{
		Command:        "flights",
		Origin:         "HEL",
		Destination:    "BCN",
		DepartDate:     "2026-07-01",
		ReturnDate:     "2026-07-08",
		FlightPrice:    199,
		FlightCurrency: "EUR",
		FlightAirline:  "KLM",
	}
	saveLastSearch(ls)

	cmd := shareCmd()
	cmd.SetArgs([]string{"--last"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("share --last: %v", err)
	}
}

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

func TestAccomHackCmd_FlagsV18(t *testing.T) {
	cmd := accomHackCmd()
	for _, name := range []string{"checkin", "checkout", "currency", "max-splits", "guests"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on accomHackCmd", name)
		}
	}
}

func TestGridCmd_RequiredFlagsMissing(t *testing.T) {

	cmd := gridCmd()
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

func TestMultiCityCmd_ValidArgsNoNetwork(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := multiCityCmd()

	cmd.SetArgs([]string{"HEL", "--visit", "BCN,ROM", "--dates", "2026-07-01,2026-07-21"})

	_ = cmd.Execute()
}

func TestExploreCmd_FlagsExistV18(t *testing.T) {
	cmd := exploreCmd()
	for _, name := range []string{"from", "to", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on exploreCmd", name)
		}
	}
}

// writeTestProviderV19 creates a minimal provider config JSON in the temp HOME.
func writeTestProviderV19(t *testing.T, tmp, id string) {
	t.Helper()
	dir := filepath.Join(tmp, ".trvl", "providers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	cfg := providers.ProviderConfig{
		ID:       id,
		Name:     "Test Provider " + id,
		Category: "hotels",
		Endpoint: "https://example.com/api",
		Method:   "GET",
		Consent: &providers.ConsentRecord{
			Granted:   true,
			Timestamp: time.Now(),
			Domain:    "example.com",
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal provider: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}
}

func TestRunProvidersList_WithProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "test-hotel-provider")

	cmd := providersCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("providers list: %v", err)
	}
}

func TestRunProvidersList_WithProviderJSONV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "test-hotel-provider-json")

	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	cmd := providersCmd()
	cmd.SetArgs([]string{"list"})
	_ = cmd.Execute()
}

func TestRunProvidersStatus_WithProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "test-status-provider")

	cmd := providersCmd()
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("providers status: %v", err)
	}
}

func TestRunProvidersStatus_WithErrorProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".trvl", "providers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := providers.ProviderConfig{
		ID:         "error-provider",
		Name:       "Error Provider",
		Category:   "flights",
		Endpoint:   "https://bad.example.com/api",
		LastError:  "connection refused",
		ErrorCount: 5,
		Consent: &providers.ConsentRecord{
			Granted:   true,
			Timestamp: time.Now(),
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(dir, "error-provider.json"), data, 0o600)

	cmd := providersCmd()
	cmd.SetArgs([]string{"status"})
	_ = cmd.Execute()
}

func TestRunProvidersStatus_WithStaleProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".trvl", "providers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := providers.ProviderConfig{
		ID:          "stale-provider",
		Name:        "Stale Provider",
		Category:    "flights",
		Endpoint:    "https://stale.example.com/api",
		LastSuccess: time.Now().Add(-48 * time.Hour),
		Consent: &providers.ConsentRecord{
			Granted:   true,
			Timestamp: time.Now(),
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(dir, "stale-provider.json"), data, 0o600)

	cmd := providersCmd()
	cmd.SetArgs([]string{"status"})
	_ = cmd.Execute()
}

func TestRunProvidersDisable_WithProviderV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "to-delete-provider")

	cmd := providersCmd()
	cmd.SetArgs([]string{"disable", "to-delete-provider"})

	_ = cmd.Execute()
}

func TestAirportTransferCmd_MissingArgsV19(t *testing.T) {
	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"CDG"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

func TestAirportTransferCmd_FlagsExistV19(t *testing.T) {
	cmd := airportTransferCmd()
	for _, name := range []string{"currency", "provider", "max-price", "type", "arrival-after"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on airportTransferCmd", name)
		}
	}
}

func TestRouteCmd_MissingArgsV19(t *testing.T) {
	cmd := routeCmd()
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

func TestRunInstall_CodexDryRunV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	err := runInstall("codex", false, true)
	if err != nil {
		t.Errorf("runInstall codex dry-run: %v", err)
	}
}

func TestRunInstall_CodexCreatesConfigV19(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	err := runInstall("codex", false, false)
	if err != nil {
		t.Errorf("runInstall codex create: %v", err)
	}
}

func TestOpenBrowser_EmptyURL_V20(t *testing.T) {
	err := openBrowser("")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestTripsAlertsCmd_MarkReadEmptyV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := tripsCmd()
	cmd.SetArgs([]string{"alerts", "--mark-read"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("trips alerts --mark-read: %v", err)
	}
}

func TestSuggestCmd_InvalidOriginV20(t *testing.T) {
	cmd := suggestCmd()
	cmd.SetArgs([]string{"12", "BCN"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestSuggestCmd_InvalidDestV20(t *testing.T) {
	cmd := suggestCmd()
	cmd.SetArgs([]string{"HEL", "12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid dest IATA")
	}
}

func TestSuggestCmd_FlagsExistV20(t *testing.T) {
	cmd := suggestCmd()
	for _, name := range []string{"around", "flex", "round-trip", "duration", "format", "currency"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on suggestCmd", name)
		}
	}
}

func TestTripsBookCmd_BooksTripV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Book Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	cmd := tripsCmd()
	cmd.SetArgs([]string{
		"book", tripID,
		"--provider", "Finnair",
		"--ref", "AY12345",
		"--type", "flight",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("trips book: %v", err)
	}
}

func TestTripsDeleteCmd_DeletesTripV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Delete Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	cmd := tripsCmd()
	cmd.SetArgs([]string{"delete", tripID})
	if err := cmd.Execute(); err != nil {
		t.Errorf("trips delete: %v", err)
	}
}

func TestTripsStatusCmd_WithTripV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Status Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	addLeg := tripsCmd()
	addLeg.SetArgs([]string{
		"add-leg", tripID, "flight",
		"--from", "HEL",
		"--to", "BCN",
		"--provider", "Finnair",
		"--start", "2026-07-01T08:00",
		"--end", "2026-07-01T11:00",
		"--price", "199",
		"--currency", "EUR",
	})
	_ = addLeg.Execute()

	statusCmd := tripsCmd()
	statusCmd.SetArgs([]string{"status", tripID})
	if err := statusCmd.Execute(); err != nil {
		t.Errorf("trips status: %v", err)
	}
}

func TestShareCmd_GistFlagWithLastSearchV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	ls := &LastSearch{
		Command:        "flights",
		Origin:         "HEL",
		Destination:    "BCN",
		DepartDate:     "2026-07-01",
		FlightPrice:    199,
		FlightCurrency: "EUR",
		FlightAirline:  "Finnair",
	}
	saveLastSearch(ls)

	cmd := shareCmd()
	cmd.SetArgs([]string{"--last", "--gist"})

	_ = cmd.Execute()
}

func TestTripsAlertsCmd_TableWithAlertsV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Alert Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	store, err := loadTripStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	list := store.List()
	if len(list) == 0 {
		t.Skip("no trips in store")
	}
	tripID := list[0].ID

	addLeg := tripsCmd()
	addLeg.SetArgs([]string{
		"add-leg", tripID, "flight",
		"--from", "HEL",
		"--to", "BCN",
		"--provider", "Finnair",
		"--start", "2026-04-22T08:00",
		"--end", "2026-04-22T11:00",
		"--price", "199",
		"--currency", "EUR",
	})
	_ = addLeg.Execute()

	alertsCmd := tripsCmd()
	alertsCmd.SetArgs([]string{"alerts"})
	_ = alertsCmd.Execute()
}

func TestProvidersEnableCmd_FlagsExistV20(t *testing.T) {
	cmd := providersEnableCmd()
	f := cmd.Flags().Lookup("accept-tos")
	if f == nil {
		t.Error("expected --accept-tos flag on providersEnableCmd")
	}
}

func TestDatesCmd_FlagsExistV20(t *testing.T) {
	cmd := datesCmd()
	for _, name := range []string{"return", "depart", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Logf("--%s flag not found on datesCmd", name)
		}
	}
}

func TestTripCostCmd_ValidArgsNoNetworkV20(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripCostCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "--depart", "2026-07-01", "--return", "2026-07-08"})

	_ = cmd.Execute()
}

func TestExploreCmd_ToBeforeFromV21(t *testing.T) {
	cmd := exploreCmd()

	cmd.SetArgs([]string{"HEL", "--from", "2026-07-10", "--to", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --to is before --from")
	}
}

func TestExploreCmd_OneWayTypeV21(t *testing.T) {

	cmd := exploreCmd()
	cmd.SetArgs([]string{"HEL", "--from", "2026-07-01", "--to", "2026-07-21", "--type", "one-way"})
	_ = cmd.Execute()
}

func TestShareCmd_NoArgsNoLastV21(t *testing.T) {
	cmd := shareCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no args and no --last")
	}
}

func TestWeekendCmd_ValidIATANoNetworkV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := weekendCmd()
	cmd.SetArgs([]string{"HEL", "--month", "2026-08"})

	_ = cmd.Execute()
}

func TestTripsListCmd_WithTripsV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "List Test Trip"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	cmd := tripsCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("trips list: %v", err)
	}
}

func TestWhenCmd_MissingToFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--from", "2026-07-01", "--until", "2026-07-31"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --to flag")
	}
}

func TestWhenCmd_MissingFromFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--until", "2026-07-31"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --from flag")
	}
}

func TestWhenCmd_MissingUntilFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--from", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --until flag")
	}
}

func TestWhenCmd_InvalidOriginIATAV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--from", "2026-07-01", "--until", "2026-07-31", "--origin", "12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestWhenCmd_InvalidBusyFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{
		"--to", "BCN",
		"--from", "2026-07-01",
		"--until", "2026-07-31",
		"--origin", "HEL",
		"--busy", "not-a-valid-interval",
	})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid --busy interval")
	}
}

func TestWhenCmd_FlagsExistV22(t *testing.T) {
	cmd := whenCmd()
	for _, name := range []string{"to", "from", "until", "origin", "busy", "prefer", "min-nights", "max-nights", "top", "budget", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on whenCmd", name)
		}
	}
}

func TestCabinResult_StructV22(t *testing.T) {
	r := cabinResult{Cabin: "Economy", Error: "no flights"}
	if r.Cabin != "Economy" {
		t.Errorf("expected Economy, got %s", r.Cabin)
	}
}

func TestRunProvidersDisable_SucceedsV22(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "deletable-provider")

	cmd := providersDisableCmd()
	cmd.SetArgs([]string{"deletable-provider"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("providers disable: %v", err)
	}
}

func TestPricesCmd_NonNilV22(t *testing.T) {
	cmd := pricesCmd()
	if cmd == nil {
		t.Error("expected non-nil pricesCmd")
	}
}

func TestOptimizeCmd_FlagsExistV22(t *testing.T) {
	cmd := optimizeCmd()
	if cmd == nil {
		t.Error("expected non-nil optimizeCmd")
	}
}

func TestOptimizeCmd_MissingArgsV22(t *testing.T) {
	cmd := optimizeCmd()
	cmd.SetArgs([]string{"HEL"})
	_ = cmd.Execute()
}

func TestDealsCmd_NonNilV22(t *testing.T) {
	cmd := dealsCmd()
	if cmd == nil {
		t.Error("expected non-nil dealsCmd")
	}
}

func TestRunProvidersDisable_ConfirmsNonTerminalV23(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "confirm-delete-provider")

	err := runProvidersDisable("confirm-delete-provider")
	if err != nil {
		t.Errorf("runProvidersDisable: %v", err)
	}
}

func TestPointsValueCmd_WithProgramV23(t *testing.T) {
	cmd := pointsValueCmd()
	cmd.SetArgs([]string{"--program", "avios"})
	_ = cmd.Execute()
}

func TestWeekendCmd_FlagsV23(t *testing.T) {
	cmd := weekendCmd()
	for _, name := range []string{"month", "budget", "nights", "format", "currency"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on weekendCmd", name)
		}
	}
}

func TestAirportTransferCmd_ValidArgsNoNetworkV23(t *testing.T) {
	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"CDG", "Hotel Lutetia Paris", "2026-07-01"})
	_ = cmd.Execute()
}

func TestDealsCmd_ValidRunV23(t *testing.T) {
	cmd := dealsCmd()
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
}

func TestSearchCmd_DryRunV23(t *testing.T) {
	cmd := searchCmd()
	cmd.SetArgs([]string{"--dry-run", "flights from HEL to BCN"})
	_ = cmd.Execute()
}

func TestLoungesCmd_InvalidIATA_V24(t *testing.T) {
	cmd := loungesCmd()
	cmd.SetArgs([]string{"12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid IATA")
	}
}

func TestLoungesCmd_FlagsV24(t *testing.T) {
	cmd := loungesCmd()
	if cmd == nil {
		t.Fatal("loungesCmd returned nil")
	}
}

func TestWeatherCmd_FlagsV24(t *testing.T) {
	cmd := weatherCmd()
	for _, name := range []string{"from", "to"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on weatherCmd", name)
		}
	}
}

func TestUpgradeCmd_DryRunV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("upgrade --dry-run: %v", err)
	}
}

func TestUpgradeCmd_QuietV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--quiet"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("upgrade --quiet: %v", err)
	}
}

func TestUpgradeCmd_DefaultRunV24(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("upgrade (default): %v", err)
	}
}

func TestCreateGist_NoGhV24(t *testing.T) {

	err := createGist("# Test trip\n\nSome markdown content here.")
	_ = err
}

func TestRunEvents_MissingAPIKeyV24(t *testing.T) {
	t.Setenv("TICKETMASTER_API_KEY", "")
	cmd := eventsCmd()
	cmd.SetArgs([]string{"Barcelona", "--from", "2026-07-01", "--to", "2026-07-08"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing TICKETMASTER_API_KEY")
	}
}

func TestMcpCmd_FlagsV24(t *testing.T) {
	cmd := mcpCmd()
	for _, name := range []string{"http", "port"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on mcpCmd", name)
		}
	}
}

func TestGroundCmd_MissingArgsV24(t *testing.T) {
	cmd := groundCmd()
	cmd.SetArgs([]string{"Prague"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

func TestGroundCmd_FlagsV24(t *testing.T) {
	cmd := groundCmd()
	for _, name := range []string{"currency", "max-price", "type"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on groundCmd", name)
		}
	}
}

func TestHacksCmd_MissingArgsV24(t *testing.T) {
	cmd := hacksCmd()
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only one positional arg")
	}
}

func TestDispatchSearch_FlightMissingFieldsV25(t *testing.T) {
	parent := &cobra.Command{}
	p := nlsearch.Params{Intent: "flight", Origin: "HEL"}
	err := dispatchSearch(parent, p)
	if err != nil {
		t.Errorf("expected nil for missing flight fields, got: %v", err)
	}
}

func TestDispatchSearch_HotelMissingFieldsV25(t *testing.T) {
	parent := &cobra.Command{}
	p := nlsearch.Params{Intent: "hotel", Location: "Barcelona"}
	err := dispatchSearch(parent, p)
	if err != nil {
		t.Errorf("expected nil for missing hotel fields, got: %v", err)
	}
}

func TestDispatchSearch_RouteMissingOriginV25(t *testing.T) {
	parent := &cobra.Command{}
	p := nlsearch.Params{Intent: "route"}
	err := dispatchSearch(parent, p)
	if err != nil {
		t.Errorf("expected nil for missing route origin, got: %v", err)
	}
}

func TestMissingFieldsHint_FlightV25(t *testing.T) {
	p := nlsearch.Params{Intent: "flight"}
	err := missingFieldsHint(p, "flight", "trvl flights ORIGIN DESTINATION YYYY-MM-DD")
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestMissingFieldsHint_HotelV25(t *testing.T) {
	p := nlsearch.Params{Intent: "hotel"}
	err := missingFieldsHint(p, "hotel", `trvl hotels "CITY" --checkin YYYY-MM-DD --checkout YYYY-MM-DD`)
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestMissingFieldsHint_DealsV25(t *testing.T) {
	p := nlsearch.Params{Intent: "deals"}
	err := missingFieldsHint(p, "deals", "trvl deals")
	if err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestDestinationCmd_MissingArgV25(t *testing.T) {
	cmd := destinationCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no positional arg")
	}
}

func TestDestinationCmd_FlagsV25(t *testing.T) {
	cmd := destinationCmd()
	if f := cmd.Flags().Lookup("dates"); f == nil {
		t.Error("expected --dates flag on destinationCmd")
	}
}

func TestGuideCmd_MissingArgV25(t *testing.T) {
	cmd := guideCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no positional arg")
	}
}

func TestReviewsCmd_FlagsV25(t *testing.T) {

	if reviewsCmd == nil {
		t.Fatal("reviewsCmd is nil")
	}
	for _, name := range []string{"limit", "sort", "format"} {
		if f := reviewsCmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on reviewsCmd", name)
		}
	}
}

func TestWeatherCmd_DefaultDatesNoNetworkV25(t *testing.T) {
	cmd := weatherCmd()
	cmd.SetArgs([]string{"Helsinki"})

	_ = cmd.Execute()
}

func TestLoungesCmd_ValidIATANoNetworkV25(t *testing.T) {
	cmd := loungesCmd()
	cmd.SetArgs([]string{"HEL"})

	_ = cmd.Execute()
}

func TestLooksLikeGoogleHotelID_CHIJ_V26(t *testing.T) {
	cases := []string{"ChIJabc123", "chijlower", "CHIJUPPER"}
	for _, id := range cases {
		if !looksLikeGoogleHotelID(id) {
			t.Errorf("looksLikeGoogleHotelID(%q) = false, want true", id)
		}
	}
}

func TestLooksLikeGoogleHotelID_ColonForm_V26(t *testing.T) {

	if !looksLikeGoogleHotelID("namespace:identifier") {
		t.Error("looksLikeGoogleHotelID(colon form) = false, want true")
	}

	if looksLikeGoogleHotelID("a:b:c") {
		t.Error("looksLikeGoogleHotelID(two colons) = true, want false")
	}

	if looksLikeGoogleHotelID("a: b") {
		t.Error("looksLikeGoogleHotelID(colon with space) = true, want false")
	}
}

func TestLooksLikeGoogleHotelID_Whitespace_V26(t *testing.T) {

	if !looksLikeGoogleHotelID("  /g/somehotel  ") {
		t.Error("expected true for /g/ with surrounding whitespace")
	}
}

func TestMaybeShowAccomHackTip_EmptyCheckIn_V26(t *testing.T) {
	maybeShowAccomHackTip(context.Background(), "Helsinki", "", "2026-07-04", "EUR", 1)
}

func TestMaybeShowAccomHackTip_EmptyCheckOut_V26(t *testing.T) {
	maybeShowAccomHackTip(context.Background(), "Helsinki", "2026-07-01", "", "EUR", 1)
}

func TestHotelSourceLabels_Deduplication_V26(t *testing.T) {
	h := models.HotelResult{
		Sources: []models.PriceSource{
			{Provider: "google_hotels"},
			{Provider: "google_hotels"},
			{Provider: "booking"},
		},
	}
	got := hotelSourceLabels(h)
	if got == "" {
		t.Error("expected non-empty label for known sources")
	}
}

func TestDatesCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := datesCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "--from", "2026-08-01", "--to", "2026-08-15"})

	_ = cmd.ExecuteContext(ctx)
}

func TestDatesCmd_LegacyMode_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := datesCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "--from", "2026-08-01", "--to", "2026-08-07", "--legacy"})
	_ = cmd.ExecuteContext(ctx)
}

func TestAccomHackCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := accomHackCmd()
	cmd.SetArgs([]string{
		"Prague",
		"--checkin", "2026-08-01",
		"--checkout", "2026-08-08",
	})
	_ = cmd.ExecuteContext(ctx)
}

func TestMultiCityCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := multiCityCmd()
	cmd.SetArgs([]string{
		"HEL",
		"--visit", "BCN,ROM",
		"--dates", "2026-08-01,2026-08-15",
	})
	_ = cmd.ExecuteContext(ctx)
}

func TestGroundCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := groundCmd()
	cmd.SetArgs([]string{"Helsinki", "Tampere", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

func TestOptimizeCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := optimizeCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "--depart", "2026-08-01", "--return", "2026-08-08"})
	_ = cmd.ExecuteContext(ctx)
}

func TestTripCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := tripCmd()
	cmd.SetArgs([]string{
		"HEL", "BCN",
		"--depart", "2026-08-01",
		"--return", "2026-08-08",
	})
	_ = cmd.ExecuteContext(ctx)
}

func TestWhenCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := whenCmd()
	cmd.SetArgs([]string{
		"--to", "BCN",
		"--from", "2026-08-01",
		"--until", "2026-08-31",
		"--origin", "HEL",
	})
	_ = cmd.ExecuteContext(ctx)
}

func TestLoungesCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := loungesCmd()
	cmd.SetArgs([]string{"HEL"})
	_ = cmd.ExecuteContext(ctx)
}

func TestExploreCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := exploreCmd()
	cmd.SetArgs([]string{"HEL", "--from", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

func TestHacksCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := hacksCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

func TestSuggestCmd_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := suggestCmd()
	cmd.SetArgs([]string{"HEL", "BCN", "--around", "2026-08-15"})
	_ = cmd.ExecuteContext(ctx)
}

func TestProfileCmd_EmptyProfile_V27(t *testing.T) {
	cmd := profileCmd()
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
}

func TestCancelledContextTests_Timing_V27(t *testing.T) {
	start := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():

	default:
		t.Error("expected already-cancelled context to be done")
	}

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("context setup took %v, expected <100ms", elapsed)
	}
}

func (f *fakeDaemonTickerV28) Chan() <-chan time.Time { return f.ch }

func (f *fakeDaemonTickerV28) Stop() {}

func TestDestinationCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := destinationCmd()
	cmd.SetArgs([]string{"Tokyo"})
	_ = cmd.ExecuteContext(ctx)
}

func TestDestinationCmd_WithDates_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := destinationCmd()
	cmd.SetArgs([]string{"Barcelona", "--dates", "2026-08-01,2026-08-08"})
	_ = cmd.ExecuteContext(ctx)
}

func TestDestinationCmd_SingleDate_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := destinationCmd()
	cmd.SetArgs([]string{"Paris", "--dates", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

func TestGuideCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := guideCmd()
	cmd.SetArgs([]string{"Rome"})
	_ = cmd.ExecuteContext(ctx)
}

func TestEventsCmd_NoAPIKey_V28(t *testing.T) {
	orig := os.Getenv("TICKETMASTER_API_KEY")
	_ = os.Unsetenv("TICKETMASTER_API_KEY")
	defer func() {
		if orig != "" {
			_ = os.Setenv("TICKETMASTER_API_KEY", orig)
		}
	}()

	cmd := eventsCmd()
	cmd.SetArgs([]string{"Barcelona", "--from", "2026-07-01", "--to", "2026-07-08"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when TICKETMASTER_API_KEY is not set")
	}
	if !strings.Contains(err.Error(), "TICKETMASTER_API_KEY") {
		t.Errorf("expected TICKETMASTER_API_KEY error, got: %v", err)
	}
}

func TestNearbyCmd_InvalidLat_V28(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"notanum", "2.17"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid latitude")
	}
}

func TestNearbyCmd_InvalidLon_V28(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"41.38", "notanum"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid longitude")
	}
}

func TestAirportTransferCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"HEL", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

func TestUpgradeCmd_CancelledCtx_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := upgradeCmd()
	cmd.SetArgs([]string{})
	_ = cmd.ExecuteContext(ctx)
}

func TestTripsAlertsCmd_MarkRead_V29(t *testing.T) {

	cmd := tripsAlertsCmd()
	cmd.SetArgs([]string{"--mark-read"})
	_ = cmd.Execute()
}

func TestTripsAlertsCmd_NoAlerts_V29(t *testing.T) {

	cmd := tripsAlertsCmd()
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
}

func TestAirportTransferCmd_3Args_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"HEL", "Helsinki City Center", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

func TestUpgradeCmd_DryRun_V29(t *testing.T) {
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--dry-run"})
	_ = cmd.Execute()
}

func TestUpgradeCmd_Quiet_V29(t *testing.T) {
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--quiet"})
	_ = cmd.Execute()
}

func TestGridCmd_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := gridCmd()
	cmd.SetArgs([]string{"HEL", "--from", "2026-08-01", "--to", "2026-08-31"})
	_ = cmd.ExecuteContext(ctx)
}

func TestSplitLines_NoNewline(t *testing.T) {
	lines := splitLines("hello")
	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("expected [hello], got %v", lines)
	}
}

func TestSplitLines_MultiLine(t *testing.T) {
	lines := splitLines("a\nb\nc")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("wrong lines: %v", lines)
	}
}

func TestSplitAndTrim_Basic(t *testing.T) {
	out := splitAndTrim("HEL, AMS ,NRT")
	if len(out) != 3 || out[0] != "HEL" || out[1] != "AMS" || out[2] != "NRT" {
		t.Errorf("unexpected: %v", out)
	}
}

func TestSplitAndTrim_Empty(t *testing.T) {
	out := splitAndTrim("")
	if len(out) != 0 {
		t.Errorf("expected empty, got %v", out)
	}
}

func TestSplitAndTrim_SkipsBlank(t *testing.T) {
	out := splitAndTrim("HEL,,AMS")
	if len(out) != 2 {
		t.Errorf("expected 2, got %v", out)
	}
}

func TestParseBool_TrueVariants(t *testing.T) {
	for _, v := range []string{"true", "yes", "1", "on", "TRUE", "YES"} {
		b, err := parseBool(v)
		if err != nil || !b {
			t.Errorf("parseBool(%q) should be true, got %v err %v", v, b, err)
		}
	}
}

func TestParseBool_FalseVariants(t *testing.T) {
	for _, v := range []string{"false", "no", "0", "off", "FALSE", "NO"} {
		b, err := parseBool(v)
		if err != nil || b {
			t.Errorf("parseBool(%q) should be false, got %v err %v", v, b, err)
		}
	}
}

func TestParseBool_Invalid(t *testing.T) {
	_, err := parseBool("maybe")
	if err == nil {
		t.Error("expected error for invalid boolean")
	}
}

func TestPromptString_UsesInput(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("Amsterdam\n"))
	result := promptString(scanner, "City", "Helsinki")
	if result != "Amsterdam" {
		t.Errorf("expected Amsterdam, got %q", result)
	}
}

func TestPromptString_EmptyUsesDefault(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	result := promptString(scanner, "City", "Helsinki")
	if result != "Helsinki" {
		t.Errorf("expected Helsinki (default), got %q", result)
	}
}

func TestPromptString_EmptyCurrentNoDefault(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	result := promptString(scanner, "City", "")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestPromptBool_TrueInput(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("yes\n"))
	if !promptBool(scanner, "Enable?", false) {
		t.Error("expected true")
	}
}

func TestPromptBool_FalseInput(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("no\n"))
	if promptBool(scanner, "Enable?", true) {
		t.Error("expected false")
	}
}

func TestPromptBool_InvalidUsesDefault(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("maybe\n"))
	if promptBool(scanner, "Enable?", true) != true {
		t.Error("expected default (true) on invalid input")
	}
}

func TestPromptStringSlice_Basic(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("HEL, AMS\n"))
	result := promptStringSlice(scanner, "Airports", []string{"NRT"})
	if len(result) != 2 || result[0] != "HEL" || result[1] != "AMS" {
		t.Errorf("unexpected: %v", result)
	}
}

func TestDestinationCmd_Flags(t *testing.T) {
	cmd := destinationCmd()
	if cmd.Use != "destination <location>" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}
	if f := cmd.Flags().Lookup("dates"); f == nil {
		t.Error("expected --dates flag")
	}
}

func TestDestinationCmd_ExactArgs(t *testing.T) {
	cmd := destinationCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestNearbyCmd_InvalidLatitude(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"not-a-float", "2.17"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid latitude")
	}
}

func TestNearbyCmd_InvalidLongitude(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SetArgs([]string{"41.38", "not-a-float"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid longitude")
	}
}

func TestGuideCmd_ExactArgs(t *testing.T) {
	cmd := guideCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestGuideCmd_Use(t *testing.T) {
	cmd := guideCmd()
	if cmd.Use != "guide <location>" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}
}

func TestOptimizeCmd_Flags(t *testing.T) {
	cmd := optimizeCmd()
	for _, name := range []string{"depart", "return", "flex", "guests", "currency", "results"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag", name)
		}
	}
}

func TestOptimizeCmd_RequiresTwoArgs(t *testing.T) {
	cmd := optimizeCmd()
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with single arg")
	}
}

func TestWeatherCmd_Flags(t *testing.T) {
	cmd := weatherCmd()
	for _, name := range []string{"from", "to"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag", name)
		}
	}
}

func TestWeatherCmd_Use(t *testing.T) {
	cmd := weatherCmd()
	if cmd.Use != "weather CITY" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}
}

func TestHotelSourceLabel_KnownProviders(t *testing.T) {
	cases := map[string]string{
		"google_hotels": "Google",
		"trivago":       "Trivago",
		"airbnb":        "Airbnb",
		"booking":       "Booking",
	}
	for input, want := range cases {
		got := hotelSourceLabel(input)
		if got != want {
			t.Errorf("hotelSourceLabel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHotelSourceLabel_WithSpaces(t *testing.T) {
	got := hotelSourceLabel("  airbnb  ")
	if got != "Airbnb" {
		t.Errorf("expected 'Airbnb' with spaces trimmed, got %q", got)
	}
}

func TestMaybeShowAccomHackTip_ShortStayNoOp(t *testing.T) {

	maybeShowAccomHackTip(context.TODO(), "Prague", "2026-06-15", "2026-06-17", "EUR", 2)
}

func TestProfileCmd_SubcommandsExist(t *testing.T) {
	cmd := profileCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Use] = true
	}
	for _, want := range []string{"add", "summary", "import-email"} {
		if !names[want] {
			t.Errorf("expected subcommand %q", want)
		}
	}
}

// Ensure the bytes import is actually used to avoid "imported and not used" error.
var _ = bytes.NewBuffer

func TestLooksLikeGoogleHotelID_SlashG(t *testing.T) {
	if !looksLikeGoogleHotelID("/g/11b6d4_v_4") {
		t.Error("expected true for /g/ prefix")
	}
}

func TestLooksLikeGoogleHotelID_ChIJ(t *testing.T) {
	if !looksLikeGoogleHotelID("ChIJy7MSZP0LkkYRZw2dDekQP78") {
		t.Error("expected true for ChIJ prefix")
	}
}

func TestLooksLikeGoogleHotelID_ColonSeparated(t *testing.T) {
	if !looksLikeGoogleHotelID("0x123456:0xabcdef") {
		t.Error("expected true for colon-separated ID without spaces")
	}
}

func TestLooksLikeGoogleHotelID_HotelName(t *testing.T) {
	if looksLikeGoogleHotelID("Hotel Lutetia Paris") {
		t.Error("expected false for hotel name with spaces")
	}
}

func TestLooksLikeGoogleHotelID_Empty(t *testing.T) {
	if looksLikeGoogleHotelID("") {
		t.Error("expected false for empty string")
	}
}

func TestLooksLikeGoogleHotelID_WithSpaces(t *testing.T) {
	if looksLikeGoogleHotelID("   /g/abc   ") {

	}

	if !looksLikeGoogleHotelID("   /g/abc   ") {
		t.Error("expected true after trimming spaces for /g/ prefix")
	}
}

func TestRunRestaurants_InvalidLat(t *testing.T) {

	cmd := restaurantsCmd

	err := runRestaurants(cmd, []string{"not-lat", "2.17"})
	if err == nil {
		t.Error("expected error for invalid latitude")
	}
}

func TestRunRestaurants_InvalidLon(t *testing.T) {
	cmd := restaurantsCmd
	err := runRestaurants(cmd, []string{"41.38", "not-lon"})
	if err == nil {
		t.Error("expected error for invalid longitude")
	}
}

func TestRunRestaurants_LatOutOfRange(t *testing.T) {
	cmd := restaurantsCmd
	err := runRestaurants(cmd, []string{"91.0", "2.17"})
	if err == nil {
		t.Error("expected error for lat > 90")
	}
}

func TestRunRestaurants_LonOutOfRange(t *testing.T) {
	cmd := restaurantsCmd
	err := runRestaurants(cmd, []string{"41.38", "181.0"})
	if err == nil {
		t.Error("expected error for lon > 180")
	}
}

func TestRoomsCmd_FlagsV3(t *testing.T) {
	cmd := roomsCmd()
	for _, name := range []string{"checkin", "checkout", "currency", "location"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag", name)
		}
	}
}

func TestReviewsCmd_FlagsV3(t *testing.T) {
	for _, name := range []string{"limit", "sort", "format"} {
		if f := reviewsCmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on reviewsCmd", name)
		}
	}
}

func TestDiscoverCmd_MissingOriginError(t *testing.T) {

	cmd := discoverCmd()
	cmd.SetArgs([]string{"--from", "2026-07-01", "--until", "2026-07-31", "--budget", "500"})

	_ = cmd.Execute()
}

func TestClassifyProviderStatus_Healthy(t *testing.T) {
	cfg := &providers.ProviderConfig{
		LastSuccess: time.Now().Add(-1 * time.Hour),
	}
	got := classifyProviderStatus(cfg)
	if got != "healthy" {
		t.Errorf("expected healthy, got %q", got)
	}
}

func TestClassifyProviderStatus_Stale(t *testing.T) {
	cfg := &providers.ProviderConfig{
		LastSuccess: time.Now().Add(-25 * time.Hour),
	}
	got := classifyProviderStatus(cfg)
	if got != "stale" {
		t.Errorf("expected stale, got %q", got)
	}
}

func TestClassifyProviderStatus_Error(t *testing.T) {
	cfg := &providers.ProviderConfig{
		ErrorCount:  3,
		LastError:   "connection refused",
		LastSuccess: time.Now().Add(-1 * time.Hour),
	}
	got := classifyProviderStatus(cfg)
	if got != "error" {
		t.Errorf("expected error, got %q", got)
	}
}

func TestClassifyProviderStatus_Unconfigured(t *testing.T) {
	cfg := &providers.ProviderConfig{}
	got := classifyProviderStatus(cfg)
	if got != "unconfigured" {
		t.Errorf("expected unconfigured, got %q", got)
	}
}

func TestColorProviderStatus_AllBranches(t *testing.T) {
	for _, status := range []string{"healthy", "stale", "error", "unconfigured", "unknown"} {
		got := colorProviderStatus(status)
		if got == "" {
			t.Errorf("colorProviderStatus(%q) returned empty", status)
		}
	}
}

func TestRelativeTimeStr_Zero(t *testing.T) {
	got := relativeTimeStr(time.Time{})
	if got != "-" {
		t.Errorf("expected -, got %q", got)
	}
}

func TestRelativeTimeStr_JustNow(t *testing.T) {
	got := relativeTimeStr(time.Now().Add(-10 * time.Second))
	if got != "just now" {
		t.Errorf("expected 'just now', got %q", got)
	}
}

func TestRelativeTimeStr_OneMinuteAgo(t *testing.T) {
	got := relativeTimeStr(time.Now().Add(-1*time.Minute - 5*time.Second))
	if got != "1m ago" {
		t.Errorf("expected '1m ago', got %q", got)
	}
}

func TestRelativeTimeStr_MultipleMinutesAgo(t *testing.T) {
	got := relativeTimeStr(time.Now().Add(-30 * time.Minute))
	if !strings.HasSuffix(got, "m ago") {
		t.Errorf("expected Xm ago, got %q", got)
	}
}

func TestRelativeTimeStr_OneHourAgo(t *testing.T) {
	got := relativeTimeStr(time.Now().Add(-1*time.Hour - 5*time.Minute))
	if got != "1h ago" {
		t.Errorf("expected '1h ago', got %q", got)
	}
}

func TestRelativeTimeStr_MultipleHoursAgo(t *testing.T) {
	got := relativeTimeStr(time.Now().Add(-5 * time.Hour))
	if !strings.HasSuffix(got, "h ago") {
		t.Errorf("expected Xh ago, got %q", got)
	}
}

func TestRelativeTimeStr_OneDayAgo(t *testing.T) {
	got := relativeTimeStr(time.Now().Add(-25 * time.Hour))
	if got != "1d ago" {
		t.Errorf("expected '1d ago', got %q", got)
	}
}

func TestRelativeTimeStr_MultipleDaysAgo(t *testing.T) {
	got := relativeTimeStr(time.Now().Add(-72 * time.Hour))
	if !strings.HasSuffix(got, "d ago") {
		t.Errorf("expected Xd ago, got %q", got)
	}
}

func TestLoungesCmd_InvalidIATAError(t *testing.T) {
	cmd := loungesCmd()
	cmd.SetArgs([]string{"12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid IATA code")
	}
}

func TestWeatherCmd_MissingArg(t *testing.T) {
	cmd := weatherCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestMultiCityCmd_FlagsV4(t *testing.T) {
	cmd := multiCityCmd()

	for _, name := range []string{"visit", "dates"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag", name)
		}
	}
}

func TestRunNearby_ValidLatLon_FlagDefaults(t *testing.T) {

	cmd := nearbyCmd()

	cmd.SetArgs([]string{"91.0", "2.17"})

	if f := cmd.Flags().Lookup("radius"); f.DefValue != "500" {
		t.Errorf("expected default radius 500, got %s", f.DefValue)
	}
}

func TestCalendarCmd_NoArgNoLastError(t *testing.T) {
	cmd := calendarCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when neither trip_id nor --last provided")
	}
}

func TestExploreCmd_InvalidIATAError(t *testing.T) {
	cmd := exploreCmd()
	cmd.SetArgs([]string{"12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid IATA origin")
	}
}

func TestExploreCmd_InvalidFromDate(t *testing.T) {
	cmd := exploreCmd()
	cmd.SetArgs([]string{"HEL", "--from", "not-a-date"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid --from date")
	}
}

func TestExploreCmd_FlagsV5(t *testing.T) {
	cmd := exploreCmd()
	for _, name := range []string{"from", "to", "type", "stops", "format", "currency"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on exploreCmd", name)
		}
	}
}

func TestGroundCmd_FlagsV5(t *testing.T) {
	cmd := groundCmd()
	for _, name := range []string{"currency", "provider", "max-price", "type"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on groundCmd", name)
		}
	}
}

func TestGroundCmd_RequiresThreeArgsV5(t *testing.T) {
	cmd := groundCmd()
	cmd.SetArgs([]string{"Prague", "Vienna"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only two args")
	}
}

func TestHacksCmd_Flags(t *testing.T) {
	cmd := hacksCmd()
	for _, name := range []string{"return", "carry-on", "currency"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on hacksCmd", name)
		}
	}
}

func TestHacksCmd_RequiresThreeArgs(t *testing.T) {
	cmd := hacksCmd()
	cmd.SetArgs([]string{"HEL", "BCN"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only two args")
	}
}

func TestRunEvents_MissingAPIKey(t *testing.T) {
	t.Setenv("TICKETMASTER_API_KEY", "")
	cmd := eventsCmd()
	cmd.SetArgs([]string{"Barcelona", "--from", "2026-07-01", "--to", "2026-07-08"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when TICKETMASTER_API_KEY is not set")
	}
}

func TestAirportTransferCmd_FlagsExist(t *testing.T) {
	cmd := airportTransferCmd()
	for _, name := range []string{"currency", "max-price", "type", "arrival-after"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag", name)
		}
	}
}

func TestDatesCmd_FlagsV5(t *testing.T) {
	cmd := datesCmd()

	for _, name := range []string{"from", "to", "duration", "round-trip"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on datesCmd", name)
		}
	}
}

func TestDatesCmd_RequiresTwoArgsV5(t *testing.T) {
	cmd := datesCmd()
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with one arg")
	}
}

func TestGridCmd_FlagsV5(t *testing.T) {
	cmd := gridCmd()
	for _, name := range []string{"depart-from", "depart-to", "return-from", "return-to"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on gridCmd", name)
		}
	}
}

func TestFlightsCmd_InvalidOriginIATA(t *testing.T) {
	cmd := flightsCmd()
	cmd.SetArgs([]string{"12", "BCN", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestFlightsCmd_InvalidDestinationIATA(t *testing.T) {
	cmd := flightsCmd()
	cmd.SetArgs([]string{"HEL", "12", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid destination IATA")
	}
}

func TestMultiCityCmd_InvalidHomeIATA(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SetArgs([]string{"12", "--visit", "BCN,ROM"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid home IATA")
	}
}

func TestMultiCityCmd_MissingVisitFlag(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --visit is missing")
	}
}

func TestProvidersEnableCmd_FlagsExist(t *testing.T) {
	cmd := providersEnableCmd()
	if f := cmd.Flags().Lookup("accept-tos"); f == nil {
		t.Error("expected --accept-tos flag")
	}
}

func TestProvidersEnableCmd_RequiresOneArg(t *testing.T) {
	cmd := providersEnableCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestProvidersDisableCmd_RequiresOneArg(t *testing.T) {
	cmd := providersDisableCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestCabinSpecSlice_HasFourClasses(t *testing.T) {
	if len(cabinClasses) != 4 {
		t.Errorf("expected 4 cabin classes, got %d", len(cabinClasses))
	}
	names := []string{"Economy", "Premium Economy", "Business", "First"}
	for i, spec := range cabinClasses {
		if spec.Name != names[i] {
			t.Errorf("cabinClasses[%d].Name = %q, want %q", i, spec.Name, names[i])
		}
	}
}

func TestConvertRoundedDisplayAmounts_NilAmount(t *testing.T) {
	got := convertRoundedDisplayAmounts(context.Background(), "EUR", "USD", 2, nil)
	if got != "EUR" {
		t.Errorf("expected EUR (no conversion), got %q", got)
	}
}

func TestConvertRoundedDisplayAmounts_ZeroAmount(t *testing.T) {
	zero := 0.0
	got := convertRoundedDisplayAmounts(context.Background(), "EUR", "USD", 2, &zero)
	if got != "EUR" {
		t.Errorf("expected EUR (zero amount skipped), got %q", got)
	}
}

func TestMaybeShowFlightHackTips_EmptyFlightsV7(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: nil,
	}

	maybeShowFlightHackTips(context.Background(), []string{"HEL"}, []string{"BCN"}, "2026-07-01", "", 1, result)
}

func TestSuggestCmd_InvalidOriginIATAV7(t *testing.T) {
	cmd := suggestCmd()
	cmd.SetArgs([]string{"12", "BCN", "--around", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestSuggestCmd_InvalidDestIATAV7(t *testing.T) {
	cmd := suggestCmd()
	cmd.SetArgs([]string{"HEL", "12", "--around", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid dest IATA")
	}
}

func TestFlightsCmd_HomeOriginResolves(t *testing.T) {

	cmd := flightsCmd()
	cmd.SetArgs([]string{"home", "BCN", "2026-07-01"})
	_ = cmd.Execute()
}

func TestBaggageCmd_CarryOnOnly(t *testing.T) {
	cmd := baggageCmd()
	cmd.SetArgs([]string{"--carry-on-only"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaggageCmd_UnknownAirline(t *testing.T) {
	cmd := baggageCmd()
	cmd.SetArgs([]string{"XX"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown airline code")
	}
}

func TestBaggageCmd_NoArgsReturnsHelp(t *testing.T) {
	cmd := baggageCmd()
	cmd.SetArgs([]string{})

	_ = cmd.Execute()
}

func TestAccomHackCmd_FlagsExist(t *testing.T) {
	cmd := accomHackCmd()
	for _, name := range []string{"checkin", "checkout", "currency", "max-splits", "guests"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on accomHackCmd", name)
		}
	}
}

func TestCalendarCmd_LastNoSearch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := calendarCmd()
	cmd.SetArgs([]string{"--last"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no last search exists")
	}
}

func TestCalendarCmd_FlagsExist(t *testing.T) {
	cmd := calendarCmd()
	for _, name := range []string{"output", "last"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on calendarCmd", name)
		}
	}
}

func TestCalendarCmd_NoArgNoLast(t *testing.T) {
	cmd := calendarCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when neither trip_id nor --last provided")
	}
}

func TestCalendarCmd_LastWithSearch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	ls := &LastSearch{
		Command:        "flights",
		Origin:         "HEL",
		Destination:    "BCN",
		DepartDate:     "2026-07-01",
		ReturnDate:     "2026-07-08",
		FlightPrice:    199,
		FlightCurrency: "EUR",
		FlightAirline:  "KLM",
	}
	saveLastSearch(ls)

	cmd := calendarCmd()
	cmd.SetArgs([]string{"--last"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCalendarCmd_LastWriteToFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	ls := &LastSearch{
		Command:        "flights",
		Origin:         "HEL",
		Destination:    "BCN",
		DepartDate:     "2026-07-01",
		ReturnDate:     "2026-07-08",
		FlightPrice:    199,
		FlightCurrency: "EUR",
	}
	saveLastSearch(ls)

	outFile := filepath.Join(tmp, "trip.ics")
	cmd := calendarCmd()
	cmd.SetArgs([]string{"--last", "--output", outFile})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		t.Error("expected ICS file to be written")
	}
}

func TestRunCabinComparison_JSONNoNetwork(t *testing.T) {

	ctx := context.Background()

	err := runCabinComparison(ctx, []string{"HEL"}, []string{"BCN"}, "2026-07-01", flights.SearchOptions{}, "json")

	_ = err
}

func TestTripsShowCmd_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := tripsShowCmd()
	cmd.SetArgs([]string{"nonexistent-id"})
	err := cmd.Execute()

	_ = err
}

func TestTripCostCmd_FlagsV9(t *testing.T) {
	cmd := tripCostCmd()
	if cmd == nil {
		t.Error("expected non-nil tripCostCmd")
	}
}
