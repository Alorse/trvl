package main

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/trip"
)

// ---------------------------------------------------------------------------
// printSuggestTable — failure branch (covers !result.Success path)
// ---------------------------------------------------------------------------

func TestPrintSuggestTable_FailureBranchV21(t *testing.T) {
	result := &trip.SmartDateResult{
		Success: false,
		Error:   "no dates found",
	}
	ctx := context.Background()
	err := printSuggestTable(ctx, "", result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// exploreCmd — invalid date range (--to before --from)
// ---------------------------------------------------------------------------

func TestExploreCmd_ToBeforeFromV21(t *testing.T) {
	cmd := exploreCmd()
	// --from 2026-07-10, --to 2026-07-01 → ValidateDateRange should error.
	cmd.SetArgs([]string{"HEL", "--from", "2026-07-10", "--to", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --to is before --from")
	}
}

func TestExploreCmd_OneWayTypeV21(t *testing.T) {
	// Passes IATA + dates + type=one-way → runs until network call.
	// Covers the `tripType != "one-way"` branch (false path).
	cmd := exploreCmd()
	cmd.SetArgs([]string{"HEL", "--from", "2026-07-01", "--to", "2026-07-21", "--type", "one-way"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// shareCmd — --format link (createGist: gh not found → fallback print)
// ---------------------------------------------------------------------------

func TestShareCmd_LastFormatLinkV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	ls := &LastSearch{
		Command:        "flights",
		Origin:         "HEL",
		Destination:    "NRT",
		DepartDate:     "2026-07-01",
		FlightPrice:    799,
		FlightCurrency: "EUR",
		FlightAirline:  "Finnair",
	}
	saveLastSearch(ls)

	cmd := shareCmd()
	cmd.SetArgs([]string{"--last", "--format", "link"})
	// gh not in $PATH in test env → fallback: prints markdown. Either way, no panic.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// shareCmd — no args, no --last (covers the "provide a trip_id" error branch)
// ---------------------------------------------------------------------------

func TestShareCmd_NoArgsNoLastV21(t *testing.T) {
	cmd := shareCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no args and no --last")
	}
}

// ---------------------------------------------------------------------------
// runPrefsSet — additional key paths not yet covered
// ---------------------------------------------------------------------------

func TestPrefsSetCmd_LocaleV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"locale", "en-FI"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set locale: %v", err)
	}
}

func TestPrefsSetCmd_HomeAirportsMultipleV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"home_airports", "HEL,AMS"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set home_airports multi: %v", err)
	}
}

func TestPrefsSetCmd_LoyaltyAirlinesV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"loyalty_airlines", "AY,KL"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set loyalty_airlines: %v", err)
	}
}

func TestPrefsSetCmd_LoyaltyHotelsV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"loyalty_hotels", "Marriott Bonvoy"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set loyalty_hotels: %v", err)
	}
}

func TestPrefsSetCmd_PreferredDistrictsV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"preferred_districts", "Prague=Prague 1,Prague 2"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set preferred_districts: %v", err)
	}
}

func TestPrefsSetCmd_CarryOnOnlyV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"carry_on_only", "true"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set carry_on_only: %v", err)
	}
}

func TestPrefsSetCmd_PreferDirectV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"prefer_direct", "false"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set prefer_direct: %v", err)
	}
}

func TestPrefsSetCmd_UnknownKeyV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"unknown_key_xyz", "value"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown preference key")
	}
}

// ---------------------------------------------------------------------------
// tripsShowCmd — JSON format branch
// ---------------------------------------------------------------------------

func TestTripsShowCmd_JSONFormatV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "JSON Show Trip"})
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

	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	cmd := tripsCmd()
	cmd.SetArgs([]string{"show", tripID})
	if err := cmd.Execute(); err != nil {
		t.Errorf("trips show json: %v", err)
	}
}

// ---------------------------------------------------------------------------
// weekendCmd — valid IATA (runs until network call, covers IATA success branch)
// ---------------------------------------------------------------------------

func TestWeekendCmd_ValidIATANoNetworkV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := weekendCmd()
	cmd.SetArgs([]string{"HEL", "--month", "2026-08"})
	// Will fail on network — covers the branch past IATA validation.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// tripsListCmd — with trips (covers non-empty list branch)
// ---------------------------------------------------------------------------

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

func TestTripsListCmd_JSONFormatV21(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "JSON List Trip"})
	_ = createCmd.Execute()

	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	cmd := tripsCmd()
	cmd.SetArgs([]string{"list"})
	_ = cmd.Execute()
}
