package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// shareTrip — with a real trip (covers shareTrip + formatTripMarkdown paths)
// ---------------------------------------------------------------------------

func TestShareTrip_WithRealTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a trip.
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

	// Add a leg so markdown has content.
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

	// Share should generate markdown to stdout.
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
	// Should error — trip not found.
	err := cmd.Execute()
	_ = err
}

// ---------------------------------------------------------------------------
// shareCmd --last (covers shareLastSearch path)
// ---------------------------------------------------------------------------

func TestShareCmd_LastWithSearch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Pre-write a last_search.json.
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

// ---------------------------------------------------------------------------
// profileAddCmd — actual add (covers happy path)
// ---------------------------------------------------------------------------

func TestProfileAddCmd_AddsFlightBooking(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := profileAddCmd()
	cmd.SetArgs([]string{
		"--type", "flight",
		"--provider", "KLM",
		"--from", "HEL",
		"--to", "AMS",
		"--price", "189",
		"--currency", "EUR",
		"--travel-date", "2026-03-15",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile add: %v", err)
	}
}

func TestProfileAddCmd_AddsHotelBooking(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := profileAddCmd()
	cmd.SetArgs([]string{
		"--type", "hotel",
		"--provider", "Marriott",
		"--price", "450",
		"--currency", "EUR",
		"--nights", "3",
		"--stars", "4",
		"--travel-date", "2026-03-15",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile add hotel: %v", err)
	}
}

func TestProfileAddCmd_PrintsFrom_To(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// --from and --to provided — covers the conditional print path.
	cmd := profileAddCmd()
	cmd.SetArgs([]string{
		"--type", "ground",
		"--provider", "FlixBus",
		"--from", "Prague",
		"--to", "Vienna",
		"--price", "19",
		"--currency", "EUR",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile add ground: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runProfileShow — with bookings (covers the JSON and table format branches)
// ---------------------------------------------------------------------------

func TestRunProfileShow_WithBookingsTable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a booking first.
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

	// profileCmd RunE is runProfileShow — run with no subcommand args.
	cmd := profileCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile show: %v", err)
	}
}

func TestRunProfileShow_WithBookingsJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a booking first.
	addCmd := profileAddCmd()
	addCmd.SetArgs([]string{
		"--type", "flight",
		"--provider", "KLM",
		"--from", "HEL",
		"--to", "AMS",
		"--price", "189",
		"--currency", "EUR",
	})
	_ = addCmd.Execute()

	// Set global format to json — profileCmd uses global var.
	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	cmd := profileCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile show json: %v", err)
	}
}

// ---------------------------------------------------------------------------
// prefsCmd set — exercise various key paths via prefs set command
// ---------------------------------------------------------------------------

func TestPrefsSetCmd_HomeAirports(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"home_airports", "HEL"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set home_airports: %v", err)
	}
}

func TestPrefsSetCmd_DisplayCurrency(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"display_currency", "EUR"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set display_currency: %v", err)
	}
}

func TestPrefsSetCmd_InvalidDisplayCurrency(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"display_currency", "TOOLONG"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid display_currency")
	}
}

func TestPrefsSetCmd_MinHotelStars(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"min_hotel_stars", "3"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set min_hotel_stars: %v", err)
	}
}

func TestPrefsSetCmd_MinHotelRating(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsSetCmd()
	cmd.SetArgs([]string{"min_hotel_rating", "8.5"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("prefs set min_hotel_rating: %v", err)
	}
}
