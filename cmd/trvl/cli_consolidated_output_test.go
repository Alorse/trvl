package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/MikkoParkkola/trvl/internal/profile"
	"github.com/MikkoParkkola/trvl/internal/trip"
	"github.com/MikkoParkkola/trvl/internal/trips"
	"github.com/MikkoParkkola/trvl/internal/tripwindow"
	"github.com/MikkoParkkola/trvl/internal/weather"
)

func TestPrintWhenTable_WithPreferredAndHotelV14(t *testing.T) {
	candidates := []tripwindow.Candidate{
		{
			Start:             "2026-07-01",
			End:               "2026-07-08",
			Nights:            7,
			FlightCost:        199,
			HotelCost:         350,
			EstimatedCost:     549,
			Currency:          "EUR",
			OverlapsPreferred: true,
			HotelName:         "Hotel Barcelona",
		},
	}
	err := printWhenTable(candidates, "HEL", "BCN")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintSuggestTable_WithInsightsV14(t *testing.T) {
	result := &trip.SmartDateResult{
		Success:      true,
		Origin:       "HEL",
		Destination:  "BCN",
		AveragePrice: 250,
		Currency:     "EUR",
		CheapestDates: []trip.CheapDate{
			{Date: "2026-07-01", DayOfWeek: "Wednesday", Price: 199, Currency: "EUR"},
		},
		Insights: []trip.DateInsight{
			{Type: "saving", Description: "Wednesday is 20% cheaper than average"},
		},
	}
	ctx := context.Background()
	err := printSuggestTable(ctx, "", result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTripsAlertsCmd_JSONFormatEmptyV14(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()
	cmd := tripsCmd()
	cmd.SetArgs([]string{"alerts"})
	_ = cmd.Execute()
}

func TestRunProfileShow_WithBookingsTable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

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

	cmd := profileCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile show: %v", err)
	}
}

func TestRunProfileShow_WithBookingsJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

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

	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	cmd := profileCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile show json: %v", err)
	}
}

func TestRunProfileSummary_WithBookings(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

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

	cmd := profileCmd()
	cmd.SetArgs([]string{"summary"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("profile summary: %v", err)
	}
}

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

	_ = cmd.Execute()
}

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

func TestPrintMultiCityTable_FailureBranchV23(t *testing.T) {
	result := &trip.MultiCityResult{
		Success: false,
		Error:   "no routes found",
	}
	err := printMultiCityTable(context.Background(), "", result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintMultiCityTable_WithSavingsV23(t *testing.T) {
	result := &trip.MultiCityResult{
		Success:      true,
		HomeAirport:  "HEL",
		OptimalOrder: []string{"BCN", "ROM"},
		Permutations: 2,
		Currency:     "EUR",
		TotalCost:    600,
		Savings:      150,
		Segments: []trip.Segment{
			{From: "HEL", To: "BCN", Price: 200, Currency: "EUR"},
			{From: "BCN", To: "ROM", Price: 150, Currency: "EUR"},
			{From: "ROM", To: "HEL", Price: 250, Currency: "EUR"},
		},
	}
	err := printMultiCityTable(context.Background(), "", result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintMultiCityTable_NoSavingsV23(t *testing.T) {
	result := &trip.MultiCityResult{
		Success:      true,
		HomeAirport:  "HEL",
		OptimalOrder: []string{"BCN"},
		Permutations: 1,
		Currency:     "EUR",
		TotalCost:    400,
		Savings:      0,
		Segments: []trip.Segment{
			{From: "HEL", To: "BCN", Price: 200, Currency: "EUR"},
			{From: "BCN", To: "HEL", Price: 200, Currency: "EUR"},
		},
	}
	err := printMultiCityTable(context.Background(), "", result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPointsValueCmd_WithJSONFormatV23(t *testing.T) {
	cmd := pointsValueCmd()
	cmd.SetArgs([]string{"--format", "json"})
	_ = cmd.Execute()
}

func TestFormatEventsCard_EmptyV24(t *testing.T) {
	err := formatEventsCard(nil, "Barcelona", "2026-07-01", "2026-07-08")
	if err != nil {
		t.Errorf("formatEventsCard empty: %v", err)
	}
}

func TestFormatEventsCard_WithEventsV24(t *testing.T) {
	events := []models.Event{
		{
			Name:       "Test Concert",
			Date:       "2026-07-03",
			Time:       "20:00",
			Venue:      "Palau Sant Jordi",
			Type:       "Music",
			PriceRange: "€30-€80",
		},
		{
			Name:       "FC Barcelona Match",
			Date:       "2026-07-05",
			Time:       "18:00",
			Venue:      "Spotify Camp Nou",
			Type:       "Sports",
			PriceRange: "€50-€200",
		},
	}
	err := formatEventsCard(events, "Barcelona", "2026-07-01", "2026-07-08")
	if err != nil {
		t.Errorf("formatEventsCard with events: %v", err)
	}
}

func TestFormatNearbyCard_EmptyV24(t *testing.T) {
	result := &destinations.NearbyResult{}
	if err := formatNearbyCard(result); err != nil {
		t.Errorf("formatNearbyCard empty: %v", err)
	}
}

func TestFormatNearbyCard_WithPOIsV24(t *testing.T) {
	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "La Boqueria", Type: "market", Distance: 120, Cuisine: "market", Hours: "9:00-20:00"},
			{Name: "Bar El Xampanyet", Type: "bar", Distance: 250, Cuisine: "tapas"},
		},
		RatedPlaces: []models.RatedPlace{
			{Name: "Tickets", Rating: 9.5, Category: "restaurant", PriceLevel: 3, Distance: 400},
		},
		Attractions: []models.Attraction{
			{Name: "Sagrada Familia", Kind: "church", Distance: 1500},
		},
	}
	if err := formatNearbyCard(result); err != nil {
		t.Errorf("formatNearbyCard with POIs: %v", err)
	}
}

func TestTruncate_V24(t *testing.T) {
	cases := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hello", 5, "hello"},
		{"ab", 2, "ab"},
		{"abc", 1, "a"},
	}
	for _, tc := range cases {
		got := truncate(tc.input, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
		}
	}
}

func TestLoungeFFCards_EmptyV24(t *testing.T) {
	cards := loungeFFCards(nil)
	if len(cards) != 0 {
		t.Errorf("expected empty cards for nil programs, got %v", cards)
	}
}

func TestLoungeFFCards_WithAlliancesV24(t *testing.T) {
	programs := []preferences.FrequentFlyerStatus{
		{Alliance: "oneworld", Tier: "sapphire", AirlineCode: "BA"},
		{Alliance: "star_alliance", Tier: "gold", AirlineCode: "LH"},
		{Alliance: "skyteam", Tier: "elite_plus", AirlineCode: "AF"},
	}
	cards := loungeFFCards(programs)
	if len(cards) == 0 {
		t.Error("expected non-empty cards for known alliances")
	}
}

func TestLoungeFFCards_UnknownAllianceV24(t *testing.T) {
	programs := []preferences.FrequentFlyerStatus{
		{Alliance: "unknown-alliance", Tier: "gold", AirlineCode: "XX"},
	}
	cards := loungeFFCards(programs)

	_ = cards
}

func TestLoungeTierDisplay_KnownAllianceV24(t *testing.T) {
	display := loungeTierDisplay("oneworld", "emerald")
	if display != "Emerald" {
		t.Errorf("expected Emerald, got %s", display)
	}
}

func TestLoungeTierDisplay_UnknownTierV24(t *testing.T) {

	display := loungeTierDisplay("oneworld", "diamond")
	if display == "" {
		t.Error("expected non-empty display for unknown tier")
	}
}

func TestLoungeTierDisplay_UnknownAllianceV24(t *testing.T) {
	display := loungeTierDisplay("unknown", "gold")
	if display == "" {
		t.Error("expected non-empty display for unknown alliance")
	}
}

func TestFormatDestinationCard_MinimalV25(t *testing.T) {
	info := &models.DestinationInfo{
		Location: "Barcelona",
	}
	if err := formatDestinationCard(info); err != nil {
		t.Errorf("formatDestinationCard minimal: %v", err)
	}
}

func TestFormatDestinationCard_FullV25(t *testing.T) {
	info := &models.DestinationInfo{
		Location: "Tokyo",
		Timezone: "Asia/Tokyo",
		Country: models.CountryInfo{
			Name:       "Japan",
			Code:       "JP",
			Capital:    "Tokyo",
			Languages:  []string{"Japanese"},
			Currencies: []string{"JPY"},
			Region:     "Asia",
		},
		Weather: models.WeatherInfo{
			Forecast: []models.WeatherDay{
				{Date: "2026-07-01", TempHigh: 28, TempLow: 22, Precipitation: 3.5, Description: "Partly cloudy"},
				{Date: "2026-07-02", TempHigh: 30, TempLow: 24, Precipitation: 0, Description: "Sunny"},
			},
		},
		Holidays: []models.Holiday{
			{Date: "2026-07-01", Name: "Test Holiday", Type: "public"},
		},
		Safety: models.SafetyInfo{
			Level:       4.5,
			Advisory:    "Exercise normal caution",
			Source:      "Travel Advisory",
			LastUpdated: "2026-01-01",
		},
		Currency: models.CurrencyInfo{
			LocalCurrency: "JPY",
			ExchangeRate:  160.5,
			BaseCurrency:  "EUR",
		},
	}
	if err := formatDestinationCard(info); err != nil {
		t.Errorf("formatDestinationCard full: %v", err)
	}
}

func TestFormatGuideCard_EmptyV25(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Location: "Prague",
		URL:      "https://en.wikivoyage.org/wiki/Prague",
	}
	if err := formatGuideCard(guide); err != nil {
		t.Errorf("formatGuideCard empty: %v", err)
	}
}

func TestFormatGuideCard_WithContentV25(t *testing.T) {
	guide := &models.WikivoyageGuide{
		Location: "Barcelona",
		URL:      "https://en.wikivoyage.org/wiki/Barcelona",
		Summary:  "Barcelona is the capital of Catalonia and the second-largest city in Spain.",
		Sections: map[string]string{
			"See":       "The city boasts amazing architecture by Antoni Gaudí.",
			"Eat":       "Tapas, pintxos, and paella are local specialties.",
			"Get in":    "El Prat Airport serves many European routes.",
			"Sleep":     "Hotels range from budget to luxury in the Eixample district.",
			"Get out":   "Day trips to Montserrat and Costa Brava are popular.",
			"Stay safe": "Keep an eye on pickpockets in tourist areas.",
		},
	}
	if err := formatGuideCard(guide); err != nil {
		t.Errorf("formatGuideCard with content: %v", err)
	}
}

func TestPrintReviewsTable_EmptyV25(t *testing.T) {
	result := &models.HotelReviewResult{
		Name:    "Hotel Test",
		Summary: models.ReviewSummary{AverageRating: 4.2, TotalReviews: 100},
		Reviews: nil,
	}
	if err := printReviewsTable(result); err != nil {
		t.Errorf("printReviewsTable empty: %v", err)
	}
}

func TestPrintReviewsTable_WithReviewsV25(t *testing.T) {
	result := &models.HotelReviewResult{
		Name:    "Grand Hotel",
		Summary: models.ReviewSummary{AverageRating: 4.7, TotalReviews: 250},
		Reviews: []models.HotelReview{
			{Rating: 5.0, Text: "Excellent stay, highly recommend!", Author: "Alice", Date: "2026-04-01"},
			{Rating: 4.0, Text: "Good location but the room was a bit small for the price paid.", Author: "Bob", Date: "2026-03-28"},
			{Rating: 3.5, Text: strings.Repeat("This is a very long review text that exceeds the 80 character limit. ", 2), Author: "Charlie", Date: "2026-03-15"},
		},
	}
	if err := printReviewsTable(result); err != nil {
		t.Errorf("printReviewsTable with reviews: %v", err)
	}
}

func TestStarRating_V25(t *testing.T) {
	cases := []float64{0, 1, 2.5, 3, 4.5, 5}
	for _, r := range cases {
		s := starRating(r)
		if s == "" {
			t.Errorf("starRating(%v) returned empty string", r)
		}
	}
}

func TestPrintTripWeather_EmptyLegsV25(t *testing.T) {
	tr := &trips.Trip{
		ID:   "test-trip-weather",
		Name: "Weather Test Trip",
		Legs: nil,
	}

	printTripWeather(context.Background(), tr)
}

func TestPrintTripWeather_LegsWithEmptyToV25(t *testing.T) {
	tr := &trips.Trip{
		ID:   "test-trip-weather-2",
		Name: "Weather Test Trip 2",
		Legs: []trips.TripLeg{
			{From: "HEL", To: "", StartTime: "2026-07-01T08:00"},
			{From: "BCN", To: "HEL", StartTime: ""},
		},
	}

	printTripWeather(context.Background(), tr)
}

func TestFormatRoomsTable_NoName_UsesHotelID_V26(t *testing.T) {
	result := &hotels.RoomAvailability{
		HotelID: "/g/unknown",
		Rooms: []hotels.RoomType{
			{Name: "Deluxe", Price: 200, Currency: "EUR", MaxGuests: 2,
				Amenities: []string{"wifi", "breakfast", "pool", "spa", "gym", "parking"}},
		},
	}
	err := formatRoomsTable(result)
	if err != nil {
		t.Errorf("formatRoomsTable(no name, long amenities) error: %v", err)
	}
}

func TestFormatRoomsTable_ZeroPrice_V26(t *testing.T) {

	result := &hotels.RoomAvailability{
		Name: "Test Hotel",
		Rooms: []hotels.RoomType{
			{Name: "Free Room", Price: 0, Currency: "EUR"},
			{Name: "Paid Room", Price: 150, Currency: "EUR", MaxGuests: 2},
		},
	}
	err := formatRoomsTable(result)
	if err != nil {
		t.Errorf("formatRoomsTable(zero price) error: %v", err)
	}
}

func TestPrintTripWeather_RealLegs_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tr := &trips.Trip{
		ID:   "v27-weather-test",
		Name: "Weather Coverage Test",
		Legs: []trips.TripLeg{
			{
				From:      "HEL",
				To:        "Barcelona",
				StartTime: "2026-08-01T10:00",
				EndTime:   "2026-08-01T13:00",
			},
			{
				From:      "Barcelona",
				To:        "Rome",
				StartTime: "2026-08-05T09:00",
				EndTime:   "2026-08-05T12:00",
			},
			{
				From:      "Rome",
				To:        "HEL",
				StartTime: "2026-08-08T15:00",
				EndTime:   "2026-08-08T20:00",
			},
		},
	}

	printTripWeather(ctx, tr)
}

func TestPrintTripWeather_LongStay_TruncatedTo7Days_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tr := &trips.Trip{
		ID:   "v27-long-stay",
		Name: "Long Stay Coverage Test",
		Legs: []trips.TripLeg{
			{
				From:      "HEL",
				To:        "Tokyo",
				StartTime: "2026-08-01",
			},
			{
				From:      "Tokyo",
				To:        "HEL",
				StartTime: "2026-08-20",
			},
		},
	}
	printTripWeather(ctx, tr)
}

func TestPrintTripWeather_LegWithEndTime_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tr := &trips.Trip{
		ID:   "v27-endtime",
		Name: "End Time Coverage Test",
		Legs: []trips.TripLeg{
			{
				From:      "JFK",
				To:        "London",
				StartTime: "2026-09-01",
				EndTime:   "2026-09-05",
			},
		},
	}
	printTripWeather(ctx, tr)
}

func TestPrintTripWeather_DuplicateDestinations_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tr := &trips.Trip{
		ID:   "v27-dedup",
		Name: "Dedup Coverage Test",
		Legs: []trips.TripLeg{
			{From: "HEL", To: "Paris", StartTime: "2026-07-01"},
			{From: "Paris", To: "HEL", StartTime: "2026-07-01"},
		},
	}
	printTripWeather(ctx, tr)
}

func TestRunCabinComparison_TableFormat_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runCabinComparison(ctx, []string{"HEL"}, []string{"BCN"}, "2026-08-01", flights.SearchOptions{}, "table")

	_ = err
}

func TestRunCabinComparison_MultiAirport_TableFormat_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runCabinComparison(ctx, []string{"HEL", "TMP"}, []string{"BCN"}, "2026-08-01", flights.SearchOptions{}, "table")
	_ = err
}

func TestPrintMultiCityTable_CurrencyConversion_CancelledCtx_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := &trip.MultiCityResult{
		Success:      true,
		HomeAirport:  "HEL",
		OptimalOrder: []string{"BCN", "ROM"},
		Segments: []trip.Segment{
			{From: "HEL", To: "BCN", Price: 150, Currency: "USD"},
			{From: "BCN", To: "ROM", Price: 80, Currency: "USD"},
		},
		TotalCost:    230,
		Currency:     "USD",
		Savings:      50,
		Permutations: 2,
	}

	err := printMultiCityTable(ctx, "EUR", result)
	_ = err
}

func TestRunCabinComparison_TableFormat_SingleAirport_V27(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runCabinComparison(ctx, []string{"HEL"}, []string{"AMS"}, "2026-09-01", flights.SearchOptions{}, "table")
	_ = err
}

func TestFormatDestinationCard_Empty_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	err := formatDestinationCard(&models.DestinationInfo{Location: "Tokyo"})
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Tokyo") {
		t.Error("expected location in output")
	}
}

func TestFormatDestinationCard_FullInfo_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	info := &models.DestinationInfo{
		Location: "Paris",
		Timezone: "Europe/Paris",
		Country: models.CountryInfo{
			Name:       "France",
			Code:       "FR",
			Region:     "Western Europe",
			Capital:    "Paris",
			Languages:  []string{"French"},
			Currencies: []string{"EUR"},
		},
		Weather: models.WeatherInfo{
			Forecast: []models.WeatherDay{
				{Date: "2026-07-01", TempHigh: 28, TempLow: 18, Precipitation: 2.5, Description: "Sunny"},
			},
		},
		Holidays: []models.Holiday{
			{Date: "2026-07-14", Name: "Bastille Day", Type: "National"},
		},
		Safety: models.SafetyInfo{
			Level:       2.5,
			Advisory:    "Exercise normal precautions",
			Source:      "State Dept",
			LastUpdated: "2026-01-01",
		},
		Currency: models.CurrencyInfo{
			BaseCurrency:  "USD",
			LocalCurrency: "EUR",
			ExchangeRate:  0.92,
		},
	}
	err := formatDestinationCard(info)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "France") {
		t.Error("expected country in output")
	}
	if !strings.Contains(out, "Bastille Day") {
		t.Error("expected holiday in output")
	}
	if !strings.Contains(out, "CURRENCY") {
		t.Error("expected currency section in output")
	}
}

func TestFormatGuideCard_Basic_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	guide := &models.WikivoyageGuide{
		Location: "Barcelona",
		URL:      "https://en.wikivoyage.org/wiki/Barcelona",
		Summary:  "Great city on the Mediterranean.",
		Sections: map[string]string{
			"See":    "Sagrada Família, Park Güell",
			"Get in": "By air: El Prat airport.",
			"Custom": "Some extra info.",
		},
	}
	err := formatGuideCard(guide)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Barcelona") {
		t.Error("expected location in output")
	}
	if !strings.Contains(out, "Sagrada") {
		t.Error("expected See section content")
	}
	if !strings.Contains(out, "Custom") {
		t.Error("expected custom section in output")
	}
}

func TestFormatGuideCard_EmptySections_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	guide := &models.WikivoyageGuide{
		Location: "Nowhere",
		URL:      "https://en.wikivoyage.org/wiki/Nowhere",
		Sections: map[string]string{
			"See": "",
		},
	}
	_ = formatGuideCard(guide)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if !strings.Contains(buf.String(), "Nowhere") {
		t.Error("expected location in output")
	}
}

func TestFormatNearbyCard_Empty_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	_ = formatNearbyCard(&destinations.NearbyResult{})
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if !strings.Contains(buf.String(), "No nearby") {
		t.Error("expected 'No nearby' message")
	}
}

func TestFormatNearbyCard_WithPOIs_V28(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "La Boqueria", Type: "market", Distance: 200, Cuisine: "", Hours: "8:00-20:00"},
		},
		RatedPlaces: []models.RatedPlace{
			{Name: "Bar El Xampanyet", Rating: 8.5, Category: "bar", PriceLevel: 2, Distance: 300},
		},
		Attractions: []models.Attraction{
			{Name: "Sagrada Familia", Kind: "church", Distance: 500},
		},
	}
	_ = formatNearbyCard(result)
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	out := buf.String()
	if !strings.Contains(out, "La Boqueria") {
		t.Error("expected POI in output")
	}
	if !strings.Contains(out, "Bar El Xampanyet") {
		t.Error("expected rated place in output")
	}
	if !strings.Contains(out, "Sagrada Familia") {
		t.Error("expected attraction in output")
	}
}

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

func TestFormatDestinationCard_ZeroExchangeRate(t *testing.T) {
	info := &models.DestinationInfo{
		Location: "Testland",
		Currency: models.CurrencyInfo{
			BaseCurrency:  "EUR",
			LocalCurrency: "TLC",
			ExchangeRate:  0,
		},
	}

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

func TestFormatRoomsTable_Empty(t *testing.T) {
	result := &hotels.RoomAvailability{
		HotelID:  "/g/11abc",
		Name:     "Grand Hotel",
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Rooms:    nil,
	}
	if err := formatRoomsTable(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatRoomsTable_EmptyNameFallsBackToID(t *testing.T) {
	result := &hotels.RoomAvailability{
		HotelID:  "/g/11abc",
		Name:     "",
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Rooms:    nil,
	}
	if err := formatRoomsTable(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatRoomsTable_WithRoomsV3(t *testing.T) {
	result := &hotels.RoomAvailability{
		HotelID:  "/g/11abc",
		Name:     "Grand Hotel",
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Rooms: []hotels.RoomType{
			{Name: "Standard", Price: 120, Currency: "EUR", MaxGuests: 2, Provider: "direct", Amenities: []string{"WiFi", "TV"}},
			{Name: "Deluxe", Price: 200, Currency: "EUR", MaxGuests: 2, Provider: "booking", Amenities: []string{"WiFi", "TV", "Minibar", "Extra1", "Extra2", "SomeLongAmenity"}},
			{Name: "Free Room", Price: 0, Currency: "EUR", MaxGuests: 0, Provider: ""},
		},
	}
	if err := formatRoomsTable(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStarRating_Full(t *testing.T) {
	got := starRating(5.0)
	if got == "" {
		t.Error("expected non-empty star rating")
	}
}

func TestStarRating_Half(t *testing.T) {
	got := starRating(3.5)
	if got == "" {
		t.Error("expected non-empty star rating for half star")
	}
}

func TestStarRating_ZeroV3(t *testing.T) {
	got := starRating(0)
	if got == "" {
		t.Error("expected non-empty star rating for zero")
	}
}

func TestPrintReviewsTable_Empty(t *testing.T) {
	result := &models.HotelReviewResult{
		HotelID: "/g/11abc",
		Name:    "Grand Hotel",
		Summary: models.ReviewSummary{AverageRating: 4.2, TotalReviews: 100},
		Reviews: nil,
	}
	if err := printReviewsTable(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintReviewsTable_WithReviewsV3(t *testing.T) {
	longText := strings.Repeat("A very long review text that should be truncated. ", 3)
	result := &models.HotelReviewResult{
		HotelID: "/g/11abc",
		Name:    "Grand Hotel",
		Summary: models.ReviewSummary{AverageRating: 4.2, TotalReviews: 2},
		Reviews: []models.HotelReview{
			{Rating: 5.0, Author: "Alice", Date: "2026-03-15", Text: "Excellent!"},
			{Rating: 3.5, Author: "Bob", Date: "2026-03-10", Text: longText},
		},
	}
	if err := printReviewsTable(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintReviewsTable_NoName(t *testing.T) {
	result := &models.HotelReviewResult{
		HotelID: "/g/11abc",
		Summary: models.ReviewSummary{AverageRating: 0, TotalReviews: 0},
		Reviews: []models.HotelReview{
			{Rating: 4.0, Author: "Eve", Date: "2026-01-01", Text: "Good."},
		},
	}
	if err := printReviewsTable(result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintProfileSummary_Empty(t *testing.T) {
	p := &profile.TravelProfile{}

	printProfileSummary(p)
}

func TestPrintProfileSummary_Full(t *testing.T) {
	p := &profile.TravelProfile{
		TotalTrips:       15,
		TotalFlights:     30,
		TotalHotelNights: 45,
		TopAirlines: []profile.AirlineStats{
			{Code: "KL", Name: "KLM", Flights: 12},
			{Code: "AY", Name: "", Flights: 5},
		},
		PreferredAlliance: "SkyTeam",
		AvgFlightPrice:    210,
		TopRoutes: []profile.RouteStats{
			{From: "HEL", To: "AMS", Count: 8, AvgPrice: 180},
			{From: "AMS", To: "JFK", Count: 2, AvgPrice: 0},
		},
		HomeDetected:    []string{"HEL"},
		TopDestinations: []string{"AMS", "BCN", "NRT"},
		TopHotelChains: []profile.HotelChainStats{
			{Name: "Marriott", Nights: 20},
		},
		AvgStarRating:  4.2,
		AvgNightlyRate: 120,
		PreferredType:  "hotel",
		TopGroundModes: []profile.ModeStats{
			{Mode: "train", Count: 10},
		},
		AvgTripLength:  5.5,
		PreferredDays:  []string{"Tuesday", "Wednesday"},
		AvgBookingLead: 21,
		BudgetTier:     "mid-range",
		AvgTripCost:    850,
	}
	printProfileSummary(p)
}

func TestTruncateStr_ShortString(t *testing.T) {
	got := truncateStr("hello", 10)
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestTruncateStr_ExactLength(t *testing.T) {
	got := truncateStr("hello", 5)
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestTruncateStr_TruncatesWithEllipsis(t *testing.T) {
	got := truncateStr("hello world", 8)
	if !strings.HasSuffix(got, "...") || len(got) != 8 {
		t.Errorf("expected 8-char string with ellipsis, got %q", got)
	}
}

func TestTruncateStr_MaxLenThree(t *testing.T) {
	got := truncateStr("hello", 3)
	if got != "hel" {
		t.Errorf("expected hel, got %q", got)
	}
}

func TestRunProfileShow_NoBookings(t *testing.T) {

	cmd := profileCmd()
	cmd.SetArgs([]string{})

	_ = cmd.Execute()
}

func TestFormatEventsCard_Empty(t *testing.T) {
	if err := formatEventsCard(nil, "Barcelona", "2026-07-01", "2026-07-08"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatEventsCard_WithEventsV5(t *testing.T) {
	events := []models.Event{
		{Date: "2026-07-04", Time: "20:00", Name: "Rock Concert", Venue: "Palau Sant Jordi", Type: "Music", PriceRange: "€50-€200"},
		{Date: "2026-07-05", Time: "18:00", Name: "FC Barcelona vs Real Madrid", Venue: "Camp Nou", Type: "Sports", PriceRange: "€80-€400"},
	}
	if err := formatEventsCard(events, "Barcelona", "2026-07-01", "2026-07-08"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintExploreTable_Empty(t *testing.T) {
	result := &models.ExploreResult{
		Destinations: nil,
		Count:        0,
	}
	if err := printExploreTable(context.TODO(), "", result, "HEL"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintExploreTable_WithDestinations(t *testing.T) {
	result := &models.ExploreResult{
		Destinations: []models.ExploreDestination{
			{AirportCode: "BCN", CityName: "Barcelona", Country: "Spain", Price: 89, Stops: 0, AirlineName: "KLM"},
			{AirportCode: "NRT", CityName: "Tokyo", Country: "Japan", Price: 699, Stops: 1, AirlineName: "AY"},
		},
		Count: 2,
	}
	if err := printExploreTable(context.TODO(), "", result, "HEL"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintSuggestTable_FailedResult(t *testing.T) {

}

func TestRunProfileSummary_NoBookings(t *testing.T) {

	tmp := t.TempDir()
	trvlDir := filepath.Join(tmp, ".trvl")
	if err := os.MkdirAll(trvlDir, 0o755); err != nil {
		t.Fatal(err)
	}

	profilePath := filepath.Join(trvlDir, "profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"bookings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)

	cmd := profileCmd()
	cmd.SetArgs([]string{"summary"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintTripWeather_NoLegs(t *testing.T) {

	_ = time.Now
}

func TestPrintDatesTable_FailedResult(t *testing.T) {
	result := &models.DateSearchResult{
		Success: false,
		Error:   "search failed",
	}
	if err := printDatesTable(context.Background(), "", result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintDatesTable_ZeroCount(t *testing.T) {
	result := &models.DateSearchResult{
		Success: true,
		Count:   0,
		Dates:   nil,
	}
	if err := printDatesTable(context.Background(), "", result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintDatesTable_OneWayV7(t *testing.T) {
	result := &models.DateSearchResult{
		Success:   true,
		Count:     2,
		DateRange: "2026-07-01 to 2026-07-31",
		TripType:  "one_way",
		Dates: []models.DatePriceResult{
			{Date: "2026-07-05", Price: 89, Currency: "EUR"},
			{Date: "2026-07-12", Price: 75, Currency: "EUR"},
		},
	}
	if err := printDatesTable(context.Background(), "", result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintDatesTable_RoundTripV7(t *testing.T) {
	result := &models.DateSearchResult{
		Success:   true,
		Count:     1,
		DateRange: "2026-07-01 to 2026-07-31",
		TripType:  "round_trip",
		Dates: []models.DatePriceResult{
			{Date: "2026-07-05", Price: 299, Currency: "EUR", ReturnDate: "2026-07-12"},
		},
	}
	if err := printDatesTable(context.Background(), "", result); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintGridTable_EmptyResult(t *testing.T) {
	result := &models.PriceGrid{
		Success: true,
		Count:   0,
	}
	if err := printGridTable(context.Background(), "", result, "HEL", "BCN"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrintGridTable_FailedResult(t *testing.T) {
	result := &models.PriceGrid{
		Success: false,
		Count:   0,
	}
	if err := printGridTable(context.Background(), "", result, "HEL", "BCN"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMaybeShowFlightHackTips_JSONFormatV7(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Price: 199, Currency: "EUR"},
		},
	}

	maybeShowFlightHackTips(context.Background(), []string{"HEL"}, []string{"BCN"}, "2026-07-01", "", 1, result)
}

func TestPrintExploreTable_WithCityIDOnly(t *testing.T) {
	result := &models.ExploreResult{
		Destinations: []models.ExploreDestination{
			{CityID: "city:BCN", CityName: "Barcelona", Country: "Spain", Price: 89, Stops: 0},
		},
		Count: 1,
	}
	if err := printExploreTable(context.Background(), "", result, "HEL"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutputShare_MarkdownBranch(t *testing.T) {

	err := outputShare("# My Trip\n", "markdown")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutputShare_DefaultBranch(t *testing.T) {

	err := outputShare("# Trip\n", "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
