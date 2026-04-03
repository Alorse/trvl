package main

import (
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Root command
// ---------------------------------------------------------------------------

func TestRootCmd_HasExpectedSubcommands(t *testing.T) {
	expected := []string{
		"flights", "dates", "hotels", "prices", "reviews",
		"explore", "grid", "destination", "trip-cost", "weekend",
		"suggest", "multi-city", "guide", "nearby", "events",
		"restaurants", "ground", "watch", "mcp", "version",
	}
	for _, name := range expected {
		found := false
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("root command missing subcommand %q", name)
		}
	}
}

func TestRootCmd_FormatFlag(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("format")
	if f == nil {
		t.Fatal("root missing persistent --format flag")
	}
	if f.DefValue != "table" {
		t.Errorf("--format default = %q, want %q", f.DefValue, "table")
	}
}

func TestRootCmd_NoCacheFlag(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("no-cache")
	if f == nil {
		t.Fatal("root missing persistent --no-cache flag")
	}
	if f.DefValue != "false" {
		t.Errorf("--no-cache default = %q, want %q", f.DefValue, "false")
	}
}

func TestRootCmd_HelpContainsExamples(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	// Execute returns nil for help; capture long desc.
	long := rootCmd.Long
	if !strings.Contains(long, "trvl flights") {
		t.Error("root long desc should contain example usage")
	}
}

// ---------------------------------------------------------------------------
// flights command
// ---------------------------------------------------------------------------

func TestFlightsCmd_RequiresThreeArgs(t *testing.T) {
	cmd := flightsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestFlightsCmd_TooFewArgsTwo(t *testing.T) {
	cmd := flightsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL", "NRT"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 2 args")
	}
}

func TestFlightsCmd_Flags(t *testing.T) {
	cmd := flightsCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"return", ""},
		{"cabin", "economy"},
		{"stops", "any"},
		{"sort", ""},
		{"airline", "[]"},
		{"adults", "1"},
		{"format", "table"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("flights missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("flights --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

func TestFlightsCmd_UseLine(t *testing.T) {
	cmd := flightsCmd()
	if cmd.Use != "flights ORIGIN DESTINATION DATE" {
		t.Errorf("flights Use = %q", cmd.Use)
	}
}

// ---------------------------------------------------------------------------
// hotels command
// ---------------------------------------------------------------------------

func TestHotelsCmd_RequiresOneArg(t *testing.T) {
	cmd := hotelsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestHotelsCmd_RequiresCheckinCheckout(t *testing.T) {
	cmd := hotelsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	// Provide the positional arg but omit required flags.
	cmd.SetArgs([]string{"Helsinki"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --checkin/--checkout missing")
	}
}

func TestHotelsCmd_Flags(t *testing.T) {
	cmd := hotelsCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"checkin", ""},
		{"checkout", ""},
		{"guests", "2"},
		{"stars", "0"},
		{"sort", "cheapest"},
		{"currency", "USD"},
		{"min-price", "0"},
		{"max-price", "0"},
		{"min-rating", "0"},
		{"max-distance", "0"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("hotels missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("hotels --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// ground command
// ---------------------------------------------------------------------------

func TestGroundCmd_RequiresThreeArgs(t *testing.T) {
	cmd := groundCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"Prague"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestGroundCmd_Aliases(t *testing.T) {
	cmd := groundCmd()
	aliases := cmd.Aliases
	want := map[string]bool{"bus": true, "train": true}
	for _, a := range aliases {
		delete(want, a)
	}
	for missing := range want {
		t.Errorf("ground missing alias %q", missing)
	}
}

func TestGroundCmd_Flags(t *testing.T) {
	cmd := groundCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"currency", "EUR"},
		{"provider", ""},
		{"max-price", "0"},
		{"type", ""},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("ground missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("ground --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// deals command
// ---------------------------------------------------------------------------

func TestDealsCmd_NoRequiredArgs(t *testing.T) {
	// deals takes no positional args; it should not fail on arg count.
	cmd := dealsCmd()
	if cmd.Args != nil {
		// Cobra nil means ArbitraryArgs (0+). ExactArgs would be a function.
		// We just verify the Use line has no ARG placeholder in uppercase.
		if strings.Contains(cmd.Use, "ORIGIN") || strings.Contains(cmd.Use, "DESTINATION") {
			t.Error("deals should not require positional args")
		}
	}
}

func TestDealsCmd_Flags(t *testing.T) {
	cmd := dealsCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"from", ""},
		{"max-price", "0"},
		{"type", ""},
		{"hours", "48"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("deals missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("deals --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// watch command + subcommands
// ---------------------------------------------------------------------------

func TestWatchCmd_HasSubcommands(t *testing.T) {
	cmd := watchCmd()
	expected := []string{"add", "list", "remove", "check", "history"}
	names := make(map[string]bool)
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("watch missing subcommand %q", name)
		}
	}
}

func TestWatchAddCmd_RequiresTwoArgs(t *testing.T) {
	cmd := watchAddCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestWatchAddCmd_RequiresDepartFlag(t *testing.T) {
	cmd := watchAddCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	// Two args but missing required --depart.
	cmd.SetArgs([]string{"HEL", "BCN"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --depart missing")
	}
}

func TestWatchAddCmd_Flags(t *testing.T) {
	cmd := watchAddCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"depart", ""},
		{"return", ""},
		{"from", ""},
		{"to", ""},
		{"below", "0"},
		{"currency", "EUR"},
		{"type", "flight"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("watch add missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("watch add --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

func TestWatchRemoveCmd_RequiresOneArg(t *testing.T) {
	cmd := watchRemoveCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestWatchHistoryCmd_RequiresOneArg(t *testing.T) {
	cmd := watchHistoryCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestWatchListCmd_NoArgs(t *testing.T) {
	cmd := watchListCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"extra"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with unexpected args")
	}
}

func TestWatchCheckCmd_NoArgs(t *testing.T) {
	cmd := watchCheckCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"extra"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with unexpected args")
	}
}

// ---------------------------------------------------------------------------
// dates command
// ---------------------------------------------------------------------------

func TestDatesCmd_RequiresTwoArgs(t *testing.T) {
	cmd := datesCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestDatesCmd_Flags(t *testing.T) {
	cmd := datesCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"from", ""},
		{"to", ""},
		{"duration", "7"},
		{"round-trip", "false"},
		{"adults", "1"},
		{"format", "table"},
		{"legacy", "false"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("dates missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("dates --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// explore command
// ---------------------------------------------------------------------------

func TestExploreCmd_RequiresOneArg(t *testing.T) {
	cmd := exploreCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestExploreCmd_Flags(t *testing.T) {
	cmd := exploreCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"from", ""},
		{"to", ""},
		{"type", "round-trip"},
		{"stops", "any"},
		{"format", "table"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("explore missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("explore --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// grid command
// ---------------------------------------------------------------------------

func TestGridCmd_RequiresTwoArgs(t *testing.T) {
	cmd := gridCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestGridCmd_Flags(t *testing.T) {
	cmd := gridCmd()
	flags := []string{"depart-from", "depart-to", "return-from", "return-to", "format"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("grid missing --%s flag", name)
		}
	}
}

// ---------------------------------------------------------------------------
// destination command
// ---------------------------------------------------------------------------

func TestDestinationCmd_RequiresOneArg(t *testing.T) {
	cmd := destinationCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestDestinationCmd_HasDatesFlag(t *testing.T) {
	cmd := destinationCmd()
	if cmd.Flags().Lookup("dates") == nil {
		t.Error("destination missing --dates flag")
	}
}

// ---------------------------------------------------------------------------
// prices command
// ---------------------------------------------------------------------------

func TestPricesCmd_RequiresOneArg(t *testing.T) {
	cmd := pricesCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestPricesCmd_RequiresCheckinCheckout(t *testing.T) {
	cmd := pricesCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"/g/11test"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --checkin/--checkout missing")
	}
}

func TestPricesCmd_Flags(t *testing.T) {
	cmd := pricesCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"checkin", ""},
		{"checkout", ""},
		{"currency", "USD"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("prices missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("prices --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// reviews command
// ---------------------------------------------------------------------------

func TestReviewsCmd_UseLine(t *testing.T) {
	if reviewsCmd.Use != "reviews <hotel_id>" {
		t.Errorf("reviews Use = %q, want %q", reviewsCmd.Use, "reviews <hotel_id>")
	}
}

func TestReviewsCmd_ArgsIsExactOne(t *testing.T) {
	// reviewsCmd uses cobra.ExactArgs(1); verify by testing the Args validator.
	if reviewsCmd.Args == nil {
		t.Fatal("reviews Args validator is nil")
	}
	if err := reviewsCmd.Args(reviewsCmd, []string{}); err == nil {
		t.Error("expected error with 0 args")
	}
	if err := reviewsCmd.Args(reviewsCmd, []string{"id1"}); err != nil {
		t.Errorf("unexpected error with 1 arg: %v", err)
	}
	if err := reviewsCmd.Args(reviewsCmd, []string{"id1", "id2"}); err == nil {
		t.Error("expected error with 2 args")
	}
}

func TestReviewsCmd_Flags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"limit", "10"},
		{"sort", "newest"},
		{"format", "table"},
	}
	for _, tt := range flags {
		f := reviewsCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("reviews missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("reviews --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// suggest command
// ---------------------------------------------------------------------------

func TestSuggestCmd_RequiresTwoArgs(t *testing.T) {
	cmd := suggestCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestSuggestCmd_Flags(t *testing.T) {
	cmd := suggestCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"around", ""},
		{"flex", "7"},
		{"round-trip", "false"},
		{"duration", "7"},
		{"format", "table"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("suggest missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("suggest --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// multi-city command
// ---------------------------------------------------------------------------

func TestMultiCityCmd_RequiresOneArg(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestMultiCityCmd_RequiresVisitAndDates(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --visit/--dates missing")
	}
}

func TestMultiCityCmd_Flags(t *testing.T) {
	cmd := multiCityCmd()
	flags := []string{"visit", "dates", "format"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("multi-city missing --%s flag", name)
		}
	}
}

// ---------------------------------------------------------------------------
// guide command
// ---------------------------------------------------------------------------

func TestGuideCmd_RequiresOneArg(t *testing.T) {
	cmd := guideCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

// ---------------------------------------------------------------------------
// nearby command
// ---------------------------------------------------------------------------

func TestNearbyCmd_RequiresTwoArgs(t *testing.T) {
	cmd := nearbyCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"41.38"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestNearbyCmd_Flags(t *testing.T) {
	cmd := nearbyCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"category", "all"},
		{"radius", "500"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("nearby missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("nearby --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// events command
// ---------------------------------------------------------------------------

func TestEventsCmd_RequiresOneArg(t *testing.T) {
	cmd := eventsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestEventsCmd_RequiresFromTo(t *testing.T) {
	cmd := eventsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"Barcelona"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --from/--to missing")
	}
}

// ---------------------------------------------------------------------------
// restaurants command
// ---------------------------------------------------------------------------

func TestRestaurantsCmd_ArgsIsExactTwo(t *testing.T) {
	if restaurantsCmd.Args == nil {
		t.Fatal("restaurants Args validator is nil")
	}
	if err := restaurantsCmd.Args(restaurantsCmd, []string{"41.38"}); err == nil {
		t.Error("expected error with only 1 arg")
	}
	if err := restaurantsCmd.Args(restaurantsCmd, []string{"41.38", "2.17"}); err != nil {
		t.Errorf("unexpected error with 2 args: %v", err)
	}
}

func TestRestaurantsCmd_Flags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"query", "restaurants"},
		{"limit", "10"},
	}
	for _, tt := range flags {
		f := restaurantsCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("restaurants missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("restaurants --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// trip-cost command
// ---------------------------------------------------------------------------

func TestTripCostCmd_RequiresTwoArgs(t *testing.T) {
	cmd := tripCostCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with only 1 arg")
	}
}

func TestTripCostCmd_RequiresDepartReturn(t *testing.T) {
	cmd := tripCostCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"HEL", "BCN"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --depart/--return missing")
	}
}

func TestTripCostCmd_Flags(t *testing.T) {
	cmd := tripCostCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"depart", ""},
		{"return", ""},
		{"guests", "1"},
		{"currency", "EUR"},
		{"format", "table"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("trip-cost missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("trip-cost --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// weekend command
// ---------------------------------------------------------------------------

func TestWeekendCmd_RequiresOneArg(t *testing.T) {
	cmd := weekendCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}
}

func TestWeekendCmd_Flags(t *testing.T) {
	cmd := weekendCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"month", ""},
		{"budget", "0"},
		{"nights", "2"},
		{"format", "table"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("weekend missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("weekend --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// mcp command
// ---------------------------------------------------------------------------

func TestMcpCmd_Flags(t *testing.T) {
	cmd := mcpCmd()
	flags := []struct {
		name     string
		defValue string
	}{
		{"http", "false"},
		{"port", "8080"},
	}
	for _, tt := range flags {
		f := cmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("mcp missing --%s flag", tt.name)
			continue
		}
		if f.DefValue != tt.defValue {
			t.Errorf("mcp --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		mins int
		want string
	}{
		{0, "-"},
		{45, "45m"},
		{60, "1h 0m"},
		{90, "1h 30m"},
		{150, "2h 30m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.mins)
		if got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.mins, got, tt.want)
		}
	}
}

func TestFormatStops(t *testing.T) {
	tests := []struct {
		stops int
		want  string
	}{
		{0, "Direct"},
		{1, "1 stop"},
		{2, "2 stops"},
		{3, "3 stops"},
	}
	for _, tt := range tests {
		got := formatStops(tt.stops)
		if got != tt.want {
			t.Errorf("formatStops(%d) = %q, want %q", tt.stops, got, tt.want)
		}
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		amount   float64
		currency string
		want     string
	}{
		{0, "EUR", "-"},
		{199, "EUR", "EUR 199"},
		{1234, "USD", "USD 1234"},
	}
	for _, tt := range tests {
		got := formatPrice(tt.amount, tt.currency)
		if got != tt.want {
			t.Errorf("formatPrice(%v, %q) = %q, want %q", tt.amount, tt.currency, got, tt.want)
		}
	}
}

func TestFormatGroundTime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-07-01T14:30:00", "14:30"},
		{"2026-07-01T08:00:00+02:00", "08:00"},
		{"short", "short"},
	}
	for _, tt := range tests {
		got := formatGroundTime(tt.input)
		if got != tt.want {
			t.Errorf("formatGroundTime(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"ab", 2, "ab"},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"one", 1},
		{"one\ntwo\nthree", 3},
		{"", 0},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitLines(%q) returned %d lines, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestFlightRoute(t *testing.T) {
	tests := []struct {
		name string
		f    models.FlightResult
		want string
	}{
		{"empty", models.FlightResult{}, ""},
		{"direct", models.FlightResult{
			Legs: []models.FlightLeg{
				{DepartureAirport: models.AirportInfo{Code: "HEL"}, ArrivalAirport: models.AirportInfo{Code: "NRT"}},
			},
		}, "HEL -> NRT"},
		{"one stop", models.FlightResult{
			Legs: []models.FlightLeg{
				{DepartureAirport: models.AirportInfo{Code: "HEL"}, ArrivalAirport: models.AirportInfo{Code: "FRA"}},
				{DepartureAirport: models.AirportInfo{Code: "FRA"}, ArrivalAirport: models.AirportInfo{Code: "NRT"}},
			},
		}, "HEL -> FRA -> NRT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flightRoute(tt.f)
			if got != tt.want {
				t.Errorf("flightRoute() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStarRating(t *testing.T) {
	tests := []struct {
		rating float64
		full   int
		total  int // total star runes
	}{
		{5.0, 5, 5},
		{4.5, 4, 5},
		{3.0, 3, 5},
		{0.0, 0, 5},
	}
	for _, tt := range tests {
		got := starRating(tt.rating)
		// Count filled stars (U+2605 = 3 bytes).
		filled := strings.Count(got, "\u2605")
		if filled != tt.full {
			t.Errorf("starRating(%.1f) has %d filled stars, want %d", tt.rating, filled, tt.full)
		}
		totalRunes := strings.Count(got, "\u2605") + strings.Count(got, "\u2606")
		if totalRunes != tt.total {
			t.Errorf("starRating(%.1f) has %d total stars, want %d", tt.rating, totalRunes, tt.total)
		}
	}
}

func TestFormatDealAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Minute), "30m ago"},
		{now.Add(-5 * time.Hour), "5h ago"},
		{now.Add(-48 * time.Hour), "2d ago"},
	}
	for _, tt := range tests {
		got := formatDealAge(tt.t)
		if got != tt.want {
			t.Errorf("formatDealAge(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestAirportCompletion(t *testing.T) {
	// With 2+ args, should return no suggestions.
	suggestions, directive := airportCompletion(nil, []string{"HEL", "NRT"}, "")
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions with 2 args, got %d", len(suggestions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("unexpected directive: %v", directive)
	}
}

func TestAirportCompletion_Partial(t *testing.T) {
	suggestions, directive := airportCompletion(nil, nil, "HEL")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("unexpected directive: %v", directive)
	}
	// HEL should appear in suggestions (Helsinki-Vantaa).
	found := false
	for _, s := range suggestions {
		if strings.HasPrefix(s, "HEL\t") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected HEL in completion suggestions")
	}
}

func TestVersionCmd_Exists(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %q, want %q", versionCmd.Use, "version")
	}
}
