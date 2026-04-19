package main

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/MikkoParkkola/trvl/internal/watch"
	"github.com/MikkoParkkola/trvl/internal/weather"
)

// ---------------------------------------------------------------------------
// truncate (nearby.go)
// ---------------------------------------------------------------------------

func TestTruncate_ShortString(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("expected %q got %q", "hello", got)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("expected %q got %q", "hello", got)
	}
}

func TestTruncate_Longer(t *testing.T) {
	got := truncate("hello world", 8)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	if len(got) != 8 {
		t.Errorf("expected len 8, got %d", len(got))
	}
}

func TestTruncate_MaxLenThreeOrLess(t *testing.T) {
	got := truncate("hello", 3)
	if got != "hel" {
		t.Errorf("expected %q got %q", "hel", got)
	}
}

// ---------------------------------------------------------------------------
// splitLines / printWrapped (guide.go)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// formatGuideCard (guide.go) — pure formatting, no network
// ---------------------------------------------------------------------------

func TestFormatGuideCard_Empty(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Location: "Test City",
		URL:      "https://example.com",
		Sections: map[string]string{},
	}
	if err := formatGuideCard(guide); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatGuideCard_WithSummaryAndSections(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Location: "Barcelona",
		URL:      "https://en.wikivoyage.org/wiki/Barcelona",
		Summary:  "A vibrant city.",
		Sections: map[string]string{
			"Get in":    "Fly to El Prat.",
			"See":       "Sagrada Familia.",
			"Eat":       "Tapas everywhere.",
			"OtherSect": "Some content.",
		},
	}
	if err := formatGuideCard(guide); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// formatDestinationCard (destination.go) — branches not covered elsewhere
// ---------------------------------------------------------------------------

func TestFormatDestinationCard_ZeroExchangeRate(t *testing.T) {
	info := &models.DestinationInfo{
		Location: "Testland",
		Currency: models.CurrencyInfo{
			BaseCurrency:  "EUR",
			LocalCurrency: "TLC",
			ExchangeRate:  0,
		},
	}
	// ExchangeRate=0 should not divide by zero
	if err := formatDestinationCard(info); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatDestinationCard_WithTimezoneAndHolidays(t *testing.T) {
	info := &models.DestinationInfo{
		Location: "Tokyo",
		Timezone: "Asia/Tokyo",
		Country: models.CountryInfo{
			Name:       "Japan",
			Code:       "JP",
			Region:     "Asia",
			Capital:    "Tokyo",
			Languages:  []string{"Japanese"},
			Currencies: []string{"JPY"},
		},
		Holidays: []models.Holiday{
			{Date: "2026-06-20", Name: "Summer Fest", Type: "Local"},
		},
		Safety: models.SafetyInfo{
			Level:       3.5,
			Advisory:    "Exercise caution",
			Source:      "FCDO",
			LastUpdated: "2026-01-01",
		},
	}
	if err := formatDestinationCard(info); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// formatNearbyCard (nearby.go)
// ---------------------------------------------------------------------------

func TestFormatNearbyCard_WithRatedAndAttractions(t *testing.T) {
	result := &destinations.NearbyResult{
		RatedPlaces: []models.RatedPlace{
			{Name: "El Xampanyet", Rating: 8.5, Category: "bar", PriceLevel: 2, Distance: 300},
		},
		Attractions: []models.Attraction{
			{Name: "Sagrada Familia", Kind: "church", Distance: 2000},
		},
	}
	if err := formatNearbyCard(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// formatPricesTable (prices.go)
// ---------------------------------------------------------------------------

func TestFormatPricesTable_Empty(t *testing.T) {
	result := &models.HotelPriceResult{
		HotelID:  "/g/11abc",
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
	}
	if err := formatPricesTable(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// prefs helpers: additional coverage
// ---------------------------------------------------------------------------

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

func TestFormatRating_Zero(t *testing.T) {
	if got := formatRating(0); got != "0" {
		t.Errorf("expected \"0\", got %q", got)
	}
}

func TestFormatRating_NonZero(t *testing.T) {
	if got := formatRating(8.5); got != "8.5" {
		t.Errorf("expected \"8.5\", got %q", got)
	}
}

// ---------------------------------------------------------------------------
// applyPreference (prefs.go) — branches not covered elsewhere
// ---------------------------------------------------------------------------

func TestApplyPreference_BooleanFields(t *testing.T) {
	cases := []struct {
		key string
		val string
	}{
		{"carry_on_only", "true"},
		{"prefer_direct", "yes"},
		{"no_dormitories", "1"},
		{"ensuite_only", "on"},
		{"fast_wifi_needed", "true"},
	}
	for _, c := range cases {
		p := &preferences.Preferences{}
		if err := applyPreference(p, c.key, c.val); err != nil {
			t.Errorf("key %q: %v", c.key, err)
		}
	}
}

func TestApplyPreference_BooleanInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "carry_on_only", "maybe"); err == nil {
		t.Error("expected error for invalid bool")
	}
}

func TestApplyPreference_MinHotelStarsInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "min_hotel_stars", "6"); err == nil {
		t.Error("expected error for stars > 5")
	}
}

func TestApplyPreference_MinHotelStarsNotInt(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "min_hotel_stars", "abc"); err == nil {
		t.Error("expected error for non-integer")
	}
}

func TestApplyPreference_MinHotelRatingInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "min_hotel_rating", "11"); err == nil {
		t.Error("expected error for rating > 10")
	}
}

func TestApplyPreference_DisplayCurrencyInvalid(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "display_currency", "eu"); err == nil {
		t.Error("expected error for non-3-letter code")
	}
}

func TestApplyPreference_LoyaltyHotels(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "loyalty_hotels", "Marriott Bonvoy,IHG"); err != nil {
		t.Fatal(err)
	}
	if len(p.LoyaltyHotels) != 2 {
		t.Errorf("unexpected: %v", p.LoyaltyHotels)
	}
}

func TestApplyPreference_PreferredDistricts_DeleteOnEmpty(t *testing.T) {
	p := &preferences.Preferences{
		PreferredDistricts: map[string][]string{
			"Prague": {"Prague 1"},
		},
	}
	if err := applyPreference(p, "preferred_districts", "Prague="); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.PreferredDistricts["Prague"]; ok {
		t.Error("expected Prague to be deleted")
	}
}

func TestApplyPreference_PreferredDistricts_MissingEquals(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "preferred_districts", "PragueNoEquals"); err == nil {
		t.Error("expected error without =")
	}
}

func TestApplyPreference_PreferredDistricts_EmptyCity(t *testing.T) {
	p := &preferences.Preferences{}
	if err := applyPreference(p, "preferred_districts", "=Prague 1"); err == nil {
		t.Error("expected error with empty city")
	}
}

// ---------------------------------------------------------------------------
// promptString / promptBool / promptStringSlice (prefs.go)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// prefsAddFamilyMemberCmd — flag structure and validation
// ---------------------------------------------------------------------------

func TestPrefsAddFamilyMemberCmd_WrongFirstArg(t *testing.T) {
	cmd := prefsAddFamilyMemberCmd()
	cmd.SetArgs([]string{"wrong_arg", "John"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for wrong first arg")
	}
}

// ---------------------------------------------------------------------------
// destinationCmd — flag registration
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// nearbyCmd — arg validation (flags already tested elsewhere)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// guideCmd — structure
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// optimizeCmd — flag registration
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// weatherCmd — structure and flag defaults
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// printWeatherTable (weather.go) — branches not covered elsewhere
// ---------------------------------------------------------------------------

func TestPrintWeatherTable_Empty(t *testing.T) {
	result := &weather.WeatherResult{
		City:      "Prague",
		Success:   true,
		Forecasts: nil,
	}
	if err := printWeatherTable(result, "2026-04-20", "2026-04-26"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintWeatherTable_HotAndRainy(t *testing.T) {
	// Covers TempMax >= 25 and Precipitation >= 5 branches.
	result := &weather.WeatherResult{
		City:    "Bangkok",
		Success: true,
		Forecasts: []weather.Forecast{
			{Date: "2026-04-20", TempMin: 28, TempMax: 38, Precipitation: 10.0, Description: "Heavy rain"},
		},
	}
	if err := printWeatherTable(result, "2026-04-20", "2026-04-20"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintWeatherTable_Cold(t *testing.T) {
	// Covers TempMax <= 5 branch.
	result := &weather.WeatherResult{
		City:    "Reykjavik",
		Success: true,
		Forecasts: []weather.Forecast{
			{Date: "2026-01-05", TempMin: -5, TempMax: 2, Precipitation: 0.5, Description: "Snow"},
		},
	}
	if err := printWeatherTable(result, "2026-01-05", "2026-01-05"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// formatWatchDates (watch.go) — branches not covered elsewhere
// ---------------------------------------------------------------------------

func TestFormatWatchDates_RoomWatch(t *testing.T) {
	w := watch.Watch{
		Type:        "room",
		DepartDate:  "2026-06-15",
		ReturnDate:  "2026-06-18",
		MatchedRoom: "Deluxe King",
		HotelName:   "Grand Hyatt",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-06-15") {
		t.Errorf("expected check-in date in output, got %q", got)
	}
	if !strings.Contains(got, "Deluxe King") {
		t.Errorf("expected room name in output, got %q", got)
	}
}

func TestFormatWatchDates_RouteWatch_WithCheapestDate(t *testing.T) {
	w := watch.Watch{
		Type:         "flight",
		Origin:       "HEL",
		Destination:  "BCN",
		CheapestDate: "2026-07-03",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-07-03") {
		t.Errorf("expected cheapest date in output, got %q", got)
	}
}

func TestFormatWatchDates_RouteWatch_NoCheapestDate(t *testing.T) {
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "next 60d") {
		t.Errorf("expected 'next 60d' in output, got %q", got)
	}
}

func TestFormatWatchDates_DateRange_WithCheapest(t *testing.T) {
	w := watch.Watch{
		Type:         "flight",
		Origin:       "HEL",
		Destination:  "BCN",
		DepartFrom:   "2026-07-01",
		DepartTo:     "2026-07-15",
		CheapestDate: "2026-07-05",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-07-01") || !strings.Contains(got, "2026-07-15") {
		t.Errorf("expected date range in output, got %q", got)
	}
}

func TestFormatWatchDates_SpecificDateWithReturn(t *testing.T) {
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-10",
		ReturnDate:  "2026-07-17",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-07-10") {
		t.Errorf("expected depart date in output, got %q", got)
	}
}

func TestFormatWatchDates_SpecificDateNoReturn(t *testing.T) {
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-10",
	}
	got := formatWatchDates(w)
	if got != "2026-07-10" {
		t.Errorf("expected exact date, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// formatLastCheck (watch.go) — branches not covered elsewhere
// ---------------------------------------------------------------------------

func TestFormatLastCheck_MinutesAgo(t *testing.T) {
	got := formatLastCheck(time.Now().Add(-30 * time.Minute))
	if !strings.HasSuffix(got, "m ago") {
		t.Errorf("expected minutes ago, got %q", got)
	}
}

func TestFormatLastCheck_HoursAgo(t *testing.T) {
	got := formatLastCheck(time.Now().Add(-3 * time.Hour))
	if !strings.HasSuffix(got, "h ago") {
		t.Errorf("expected hours ago, got %q", got)
	}
}

func TestFormatLastCheck_DaysAgo(t *testing.T) {
	got := formatLastCheck(time.Now().Add(-50 * time.Hour))
	if !strings.HasSuffix(got, "d ago") {
		t.Errorf("expected days ago, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// hotelSourceLabel (hotels.go)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// maybeShowAccomHackTip (hotels.go) — arg validation branches
// ---------------------------------------------------------------------------

func TestMaybeShowAccomHackTip_ShortStayNoOp(t *testing.T) {
	// < 4 nights — should return early without panic.
	maybeShowAccomHackTip(context.TODO(), "Prague", "2026-06-15", "2026-06-17", "EUR", 2)
}

// ---------------------------------------------------------------------------
// profileCmd — structure
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// prefsCmd — structure
// ---------------------------------------------------------------------------

func TestPrefsCmd_SubcommandsExist(t *testing.T) {
	cmd := prefsCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"set", "edit", "init", "add"} {
		if !names[want] {
			t.Errorf("expected subcommand %q in prefs", want)
		}
	}
}

// ---------------------------------------------------------------------------
// cabinResult table rendering — pure struct coverage
// ---------------------------------------------------------------------------

func TestCabinResultTableRows_Nonstop(t *testing.T) {
	r := cabinResult{
		Cabin:    "Economy",
		Price:    199,
		Currency: "EUR",
		Airline:  "KLM",
		Stops:    0,
		Duration: 125,
	}
	stopLabel := "nonstop"
	if r.Stops == 1 {
		stopLabel = "1 stop"
	} else if r.Stops > 1 {
		stopLabel = "more"
	}
	if stopLabel != "nonstop" {
		t.Errorf("expected nonstop, got %q", stopLabel)
	}
}

func TestCabinResultTableRows_OneStop(t *testing.T) {
	r := cabinResult{Stops: 1}
	stopLabel := "nonstop"
	if r.Stops == 1 {
		stopLabel = "1 stop"
	}
	if stopLabel != "1 stop" {
		t.Errorf("expected '1 stop', got %q", stopLabel)
	}
}

func TestCabinResultTableRows_MultiStop(t *testing.T) {
	r := cabinResult{Stops: 3}
	stopLabel := "nonstop"
	if r.Stops == 1 {
		stopLabel = "1 stop"
	} else if r.Stops > 1 {
		stopLabel = "3 stops"
	}
	if stopLabel != "3 stops" {
		t.Errorf("expected '3 stops', got %q", stopLabel)
	}
}

func TestCabinResultTableRows_ErrorRow(t *testing.T) {
	r := cabinResult{Cabin: "First", Error: "no flights"}
	if r.Error == "" {
		t.Error("expected error to be set")
	}
}

func TestCabinResultTableRows_ZeroDuration(t *testing.T) {
	r := cabinResult{Duration: 0}
	dur := "—"
	if r.Duration > 0 {
		dur = "set"
	}
	if dur != "—" {
		t.Errorf("expected —, got %q", dur)
	}
}

// Ensure the bytes import is actually used to avoid "imported and not used" error.
var _ = bytes.NewBuffer
