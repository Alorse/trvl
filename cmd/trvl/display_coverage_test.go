package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/deals"
	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/hacks"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// captureStdout redirects os.Stdout to a buffer for the duration of fn,
// returning everything written. Safe for single-goroutine test helpers that
// write to os.Stdout directly.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// ---------------------------------------------------------------------------
// printFlightsTable
// ---------------------------------------------------------------------------

func TestPrintFlightsTable_Success(t *testing.T) {
	models.UseColor = false
	defer func() { models.UseColor = false }()

	result := &models.FlightSearchResult{
		Success:  true,
		Count:    2,
		TripType: "one_way",
		Flights: []models.FlightResult{
			{
				Price:    199,
				Currency: "EUR",
				Duration: 150,
				Stops:    0,
				Legs: []models.FlightLeg{
					{
						DepartureAirport: models.AirportInfo{Code: "HEL", Name: "Helsinki"},
						ArrivalAirport:   models.AirportInfo{Code: "AMS", Name: "Amsterdam"},
						DepartureTime:    "08:00",
						ArrivalTime:      "10:30",
						Airline:          "Finnair",
						FlightNumber:     "AY1571",
					},
				},
			},
			{
				Price:    299,
				Currency: "EUR",
				Duration: 195,
				Stops:    1,
				Legs: []models.FlightLeg{
					{
						DepartureAirport: models.AirportInfo{Code: "HEL", Name: "Helsinki"},
						ArrivalAirport:   models.AirportInfo{Code: "FRA", Name: "Frankfurt"},
						DepartureTime:    "09:00",
						ArrivalTime:      "11:00",
						Airline:          "Lufthansa",
						FlightNumber:     "LH1155",
					},
					{
						DepartureAirport: models.AirportInfo{Code: "FRA", Name: "Frankfurt"},
						ArrivalAirport:   models.AirportInfo{Code: "AMS", Name: "Amsterdam"},
						DepartureTime:    "12:00",
						ArrivalTime:      "13:15",
						Airline:          "Lufthansa",
						FlightNumber:     "LH992",
					},
				},
			},
		},
	}

	out := captureStdout(t, func() {
		err := printFlightsTable(context.Background(), "HEL", "AMS", "", result)
		if err != nil {
			t.Errorf("printFlightsTable returned error: %v", err)
		}
	})

	// Should contain banner, airline names, and prices.
	for _, want := range []string{"Flights", "Finnair", "EUR 199", "Lufthansa", "EUR 299", "HEL", "AMS"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintFlightsTable_NoFlights(t *testing.T) {
	models.UseColor = false

	result := &models.FlightSearchResult{
		Success:  true,
		Count:    0,
		TripType: "one_way",
	}

	out := captureStdout(t, func() {
		err := printFlightsTable(context.Background(), "HEL", "NRT", "", result)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "No flights found") {
		t.Errorf("expected 'No flights found' in output, got: %s", out)
	}
}

func TestPrintFlightsTable_Failed(t *testing.T) {
	models.UseColor = false

	result := &models.FlightSearchResult{
		Success: false,
		Error:   "timeout",
	}

	out := captureStdout(t, func() {
		// Also writes to stderr, but we just check no panic and it runs.
		_ = printFlightsTable(context.Background(), "HEL", "NRT", "", result)
	})
	// The error goes to stderr, stdout may be empty. Just verify no panic.
	_ = out
}

func TestPrintFlightsTable_WithSelfConnect(t *testing.T) {
	models.UseColor = false

	result := &models.FlightSearchResult{
		Success:  true,
		Count:    1,
		TripType: "one_way",
		Flights: []models.FlightResult{
			{
				Price:       249,
				Currency:    "EUR",
				Duration:    300,
				Stops:       1,
				SelfConnect: true,
				Legs: []models.FlightLeg{
					{
						DepartureAirport: models.AirportInfo{Code: "HEL"},
						ArrivalAirport:   models.AirportInfo{Code: "FRA"},
						DepartureTime:    "06:00",
						ArrivalTime:      "08:00",
						Airline:          "Finnair",
						FlightNumber:     "AY811",
					},
					{
						DepartureAirport: models.AirportInfo{Code: "FRA"},
						ArrivalAirport:   models.AirportInfo{Code: "BCN"},
						DepartureTime:    "10:00",
						ArrivalTime:      "12:30",
						Airline:          "Ryanair",
						FlightNumber:     "FR123",
					},
				},
			},
		},
	}

	out := captureStdout(t, func() {
		_ = printFlightsTable(context.Background(), "HEL", "BCN", "", result)
	})

	if !strings.Contains(out, "self-connect") {
		t.Errorf("expected 'self-connect' mention in output, got: %s", out)
	}
}

func TestPrintFlightsTable_WithProvider(t *testing.T) {
	models.UseColor = false

	result := &models.FlightSearchResult{
		Success:  true,
		Count:    1,
		TripType: "one_way",
		Flights: []models.FlightResult{
			{
				Price:    150,
				Currency: "EUR",
				Duration: 120,
				Stops:    0,
				Provider: "kiwi",
				Legs: []models.FlightLeg{
					{
						DepartureAirport: models.AirportInfo{Code: "HEL"},
						ArrivalAirport:   models.AirportInfo{Code: "AMS"},
						Airline:          "KLM",
						FlightNumber:     "KL1234",
					},
				},
			},
		},
	}

	out := captureStdout(t, func() {
		_ = printFlightsTable(context.Background(), "HEL", "AMS", "", result)
	})

	if !strings.Contains(out, "Kiwi") {
		t.Errorf("expected 'Kiwi' provider label in output")
	}
}

// ---------------------------------------------------------------------------
// formatHotelsTable
// ---------------------------------------------------------------------------

func TestFormatHotelsTable_Success(t *testing.T) {
	models.UseColor = false

	result := &models.HotelSearchResult{
		Success: true,
		Count:   2,
		Hotels: []models.HotelResult{
			{
				Name:        "Hotel Alpha",
				Stars:       4,
				Rating:      8.5,
				ReviewCount: 200,
				Price:       120,
				Currency:    "EUR",
				Amenities:   []string{"wifi", "pool"},
			},
			{
				Name:        "Hotel Beta",
				Stars:       3,
				Rating:      7.2,
				ReviewCount: 50,
				Price:       80,
				Currency:    "EUR",
				Amenities:   []string{"wifi"},
			},
		},
	}

	out := captureStdout(t, func() {
		err := formatHotelsTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("formatHotelsTable returned error: %v", err)
		}
	})

	for _, want := range []string{"Hotels", "Hotel Alpha", "Hotel Beta", "120", "80", "wifi"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatHotelsTable_Empty(t *testing.T) {
	models.UseColor = false

	result := &models.HotelSearchResult{
		Success: true,
		Count:   0,
		Hotels:  nil,
	}

	out := captureStdout(t, func() {
		err := formatHotelsTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "No hotels found") {
		t.Errorf("expected 'No hotels found', got: %s", out)
	}
}

func TestFormatHotelsTable_WithSources(t *testing.T) {
	models.UseColor = false

	result := &models.HotelSearchResult{
		Success: true,
		Count:   1,
		Hotels: []models.HotelResult{
			{
				Name:     "Multi-Source Hotel",
				Price:    100,
				Currency: "EUR",
				Sources: []models.PriceSource{
					{Provider: "google_hotels", Price: 100, Currency: "EUR"},
					{Provider: "booking", Price: 105, Currency: "EUR"},
				},
			},
		},
	}

	out := captureStdout(t, func() {
		_ = formatHotelsTable(context.Background(), "", result)
	})

	// Should show Sources column since booking != Google.
	if !strings.Contains(out, "Booking") {
		t.Errorf("expected 'Booking' source label in output")
	}
}

func TestFormatHotelsTable_TotalAvailable(t *testing.T) {
	models.UseColor = false

	result := &models.HotelSearchResult{
		Success:        true,
		Count:          1,
		TotalAvailable: 50,
		Hotels: []models.HotelResult{
			{
				Name:     "Only Hotel",
				Price:    90,
				Currency: "EUR",
			},
		},
	}

	out := captureStdout(t, func() {
		_ = formatHotelsTable(context.Background(), "", result)
	})

	if !strings.Contains(out, "Showing 1 of 50") {
		t.Errorf("expected 'Showing 1 of 50' in output, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// printGroundTable
// ---------------------------------------------------------------------------

func TestPrintGroundTable_Success(t *testing.T) {
	models.UseColor = false

	seats := 3
	result := &models.GroundSearchResult{
		Success: true,
		Count:   2,
		Routes: []models.GroundRoute{
			{
				Provider:  "FlixBus",
				Type:      "bus",
				Price:     15.50,
				Currency:  "EUR",
				Duration:  240,
				Departure: models.GroundStop{City: "Prague", Time: "2026-07-01T08:00:00"},
				Arrival:   models.GroundStop{City: "Vienna", Time: "2026-07-01T12:00:00"},
				Transfers: 0,
				SeatsLeft: &seats,
			},
			{
				Provider:  "RegioJet",
				Type:      "bus",
				Price:     12.00,
				PriceMax:  18.00,
				Currency:  "EUR",
				Duration:  270,
				Departure: models.GroundStop{City: "Prague", Time: "2026-07-01T09:30:00"},
				Arrival:   models.GroundStop{City: "Vienna", Time: "2026-07-01T14:00:00"},
				Transfers: 1,
			},
		},
	}

	out := captureStdout(t, func() {
		err := printGroundTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("printGroundTable returned error: %v", err)
		}
	})

	for _, want := range []string{"Ground Transport", "FlixBus", "RegioJet", "bus", "08:00", "12:00"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintGroundTable_NotSuccess(t *testing.T) {
	models.UseColor = false

	result := &models.GroundSearchResult{
		Success: false,
		Error:   "no routes available",
	}

	// Error goes to stderr; just verify no panic.
	captureStdout(t, func() {
		_ = printGroundTable(context.Background(), "", result)
	})
}

func TestPrintGroundTable_NotSuccessNoError(t *testing.T) {
	models.UseColor = false

	result := &models.GroundSearchResult{
		Success: false,
	}

	captureStdout(t, func() {
		_ = printGroundTable(context.Background(), "", result)
	})
}

// ---------------------------------------------------------------------------
// printDealsTable
// ---------------------------------------------------------------------------

func TestPrintDealsTable_Success(t *testing.T) {
	models.UseColor = false

	result := &deals.DealsResult{
		Success: true,
		Count:   2,
		Deals: []deals.Deal{
			{
				Title:       "Helsinki to Tokyo for EUR 299",
				Price:       299,
				Currency:    "EUR",
				Origin:      "HEL",
				Destination: "NRT",
				Type:        "deal",
				Source:      "secretflying",
				Published:   time.Now().Add(-2 * time.Hour),
			},
			{
				Title:       "Error Fare: London to NYC for GBP 150",
				Price:       150,
				Currency:    "GBP",
				Origin:      "LHR",
				Destination: "JFK",
				Type:        "error_fare",
				Source:      "fly4free",
				Published:   time.Now().Add(-30 * time.Minute),
			},
		},
	}

	out := captureStdout(t, func() {
		err := printDealsTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("printDealsTable returned error: %v", err)
		}
	})

	for _, want := range []string{"Travel Deals", "EUR 299", "HEL -> NRT", "error_fare"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintDealsTable_Empty(t *testing.T) {
	models.UseColor = false

	result := &deals.DealsResult{
		Success: false,
		Error:   "feed unavailable",
	}

	captureStdout(t, func() {
		_ = printDealsTable(context.Background(), "", result)
	})
}

func TestPrintDealsTable_NoError(t *testing.T) {
	models.UseColor = false

	result := &deals.DealsResult{
		Success: false,
	}

	captureStdout(t, func() {
		_ = printDealsTable(context.Background(), "", result)
	})
}

func TestPrintDealsTable_DealWithNoRoute(t *testing.T) {
	models.UseColor = false

	result := &deals.DealsResult{
		Success: true,
		Count:   1,
		Deals: []deals.Deal{
			{
				Title:     "Great deal somewhere",
				Price:     0,
				Type:      "deal",
				Source:    "thepointsguy",
				Published: time.Now().Add(-48 * time.Hour),
			},
		},
	}

	out := captureStdout(t, func() {
		_ = printDealsTable(context.Background(), "", result)
	})

	// No route info — should show "-" for route and price.
	if !strings.Contains(out, "-") {
		t.Errorf("expected '-' for missing route/price")
	}
}

func TestPrintDealsTable_DealPartialRoute(t *testing.T) {
	models.UseColor = false

	result := &deals.DealsResult{
		Success: true,
		Count:   2,
		Deals: []deals.Deal{
			{
				Title:       "To Bali",
				Destination: "DPS",
				Type:        "deal",
				Source:      "secretflying",
				Published:   time.Now(),
			},
			{
				Title:  "From Helsinki",
				Origin: "HEL",
				Type:   "deal",
				Source: "secretflying",
				Published: time.Now(),
			},
		},
	}

	out := captureStdout(t, func() {
		_ = printDealsTable(context.Background(), "", result)
	})

	if !strings.Contains(out, "-> DPS") {
		t.Errorf("expected '-> DPS' for destination-only deal")
	}
	if !strings.Contains(out, "HEL -> ?") {
		t.Errorf("expected 'HEL -> ?' for origin-only deal")
	}
}

// ---------------------------------------------------------------------------
// printExploreTable
// ---------------------------------------------------------------------------

func TestPrintExploreTable_Success(t *testing.T) {
	models.UseColor = false

	result := &models.ExploreResult{
		Success: true,
		Count:   2,
		Destinations: []models.ExploreDestination{
			{
				CityName:    "Tallinn",
				Country:     "Estonia",
				AirportCode: "TLL",
				Price:       49,
				AirlineName: "Finnair",
				Stops:       0,
			},
			{
				CityName:    "Stockholm",
				Country:     "Sweden",
				AirportCode: "ARN",
				Price:       89,
				AirlineName: "SAS",
				Stops:       0,
			},
		},
	}

	out := captureStdout(t, func() {
		// Pass empty targetCurrency to skip conversion. The function calls
		// flights.DetectSourceCurrency which would need network; however for
		// the test we just let it fall back to EUR.
		err := printExploreTable(context.Background(), "", result, "HEL")
		if err != nil {
			t.Errorf("printExploreTable returned error: %v", err)
		}
	})

	for _, want := range []string{"Tallinn", "Estonia", "TLL", "Finnair", "Direct", "Stockholm"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintExploreTable_NoDestinations(t *testing.T) {
	models.UseColor = false

	result := &models.ExploreResult{
		Success: true,
		Count:   0,
	}

	out := captureStdout(t, func() {
		_ = printExploreTable(context.Background(), "", result, "HEL")
	})

	if !strings.Contains(out, "No destinations found") {
		t.Errorf("expected 'No destinations found'")
	}
}

func TestPrintExploreTable_Failed(t *testing.T) {
	models.UseColor = false

	result := &models.ExploreResult{
		Success: false,
		Error:   "some error",
	}

	captureStdout(t, func() {
		_ = printExploreTable(context.Background(), "", result, "HEL")
	})
}

// ---------------------------------------------------------------------------
// formatDestinationCard
// ---------------------------------------------------------------------------

func TestFormatDestinationCard_Full(t *testing.T) {
	models.UseColor = false

	info := &models.DestinationInfo{
		Location: "Tokyo, Japan",
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
				{Date: "2026-06-15", TempHigh: 28, TempLow: 20, Precipitation: 5.2, Description: "Partly cloudy"},
				{Date: "2026-06-16", TempHigh: 30, TempLow: 22, Precipitation: 0, Description: "Sunny"},
			},
		},
		Holidays: []models.Holiday{
			{Date: "2026-06-15", Name: "Mountain Day", Type: "public"},
		},
		Safety: models.SafetyInfo{
			Level:       1.5,
			Advisory:    "exercise normal caution",
			Source:      "Travel Advisory",
			LastUpdated: "2026-01-01",
		},
		Currency: models.CurrencyInfo{
			LocalCurrency: "JPY",
			ExchangeRate:  160.5,
			BaseCurrency:  "EUR",
		},
	}

	out := captureStdout(t, func() {
		err := formatDestinationCard(info)
		if err != nil {
			t.Errorf("formatDestinationCard returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Tokyo, Japan",
		"Asia/Tokyo",
		"COUNTRY",
		"Japan (JP)",
		"Capital: Tokyo",
		"Languages: Japanese",
		"Currencies: JPY",
		"WEATHER",
		"28 C",
		"HOLIDAYS",
		"Mountain Day",
		"SAFETY",
		"1.5/5.0",
		"CURRENCY",
		"JPY",
		"160.50",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatDestinationCard_Minimal(t *testing.T) {
	models.UseColor = false

	info := &models.DestinationInfo{
		Location: "Unknown City",
	}

	out := captureStdout(t, func() {
		err := formatDestinationCard(info)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Unknown City") {
		t.Errorf("expected location name in output")
	}
}

// ---------------------------------------------------------------------------
// printDatesTable
// ---------------------------------------------------------------------------

func TestPrintDatesTable_Success(t *testing.T) {
	models.UseColor = false

	result := &models.DateSearchResult{
		Success:   true,
		Count:     3,
		TripType:  "one_way",
		DateRange: "2026-06-01 to 2026-06-03",
		Dates: []models.DatePriceResult{
			{Date: "2026-06-01", Price: 199, Currency: "EUR"},
			{Date: "2026-06-02", Price: 179, Currency: "EUR"},
			{Date: "2026-06-03", Price: 220, Currency: "EUR"},
		},
	}

	out := captureStdout(t, func() {
		err := printDatesTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("printDatesTable returned error: %v", err)
		}
	})

	for _, want := range []string{"2026-06-01", "EUR 199", "EUR 179", "EUR 220", "3 dates"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintDatesTable_RoundTrip(t *testing.T) {
	models.UseColor = false

	result := &models.DateSearchResult{
		Success:   true,
		Count:     2,
		TripType:  "round_trip",
		DateRange: "2026-06-01 to 2026-06-02",
		Dates: []models.DatePriceResult{
			{Date: "2026-06-01", Price: 399, Currency: "EUR", ReturnDate: "2026-06-08"},
			{Date: "2026-06-02", Price: 410, Currency: "EUR", ReturnDate: "2026-06-09"},
		},
	}

	out := captureStdout(t, func() {
		_ = printDatesTable(context.Background(), "", result)
	})

	// Round-trip should show Return column.
	if !strings.Contains(out, "Return") {
		t.Errorf("expected 'Return' column header for round-trip")
	}
	if !strings.Contains(out, "2026-06-08") {
		t.Errorf("expected return date in output")
	}
}

func TestPrintDatesTable_Empty(t *testing.T) {
	models.UseColor = false

	result := &models.DateSearchResult{
		Success: true,
		Count:   0,
	}

	out := captureStdout(t, func() {
		_ = printDatesTable(context.Background(), "", result)
	})

	if !strings.Contains(out, "No prices found") {
		t.Errorf("expected 'No prices found'")
	}
}

func TestPrintDatesTable_Failed(t *testing.T) {
	models.UseColor = false

	result := &models.DateSearchResult{
		Success: false,
		Error:   "bad request",
	}

	captureStdout(t, func() {
		_ = printDatesTable(context.Background(), "", result)
	})
}

// ---------------------------------------------------------------------------
// printHacksTable / printHack
// ---------------------------------------------------------------------------

func TestPrintHacksTable_WithHacks(t *testing.T) {
	models.UseColor = false

	detected := []hacks.Hack{
		{
			Type:        "hidden_city",
			Title:       "Hidden City Ticketing",
			Description: "Book a flight through AMS and deplane there.",
			Savings:     50,
			Currency:    "EUR",
			Steps:       []string{"Book HEL-AMS-CDG", "Exit at AMS"},
			Risks:       []string{"Airline may penalize"},
			Citations:   []string{"https://example.com"},
		},
		{
			Type:        "date_flex",
			Title:       "Date Flexibility",
			Description: "Fly 2 days earlier for cheaper fare.",
			Savings:     30,
			Currency:    "EUR",
			Steps:       []string{"Change departure to Tuesday"},
		},
	}

	out := captureStdout(t, func() {
		err := printHacksTable("HEL", "AMS", "2026-04-15", 200, "EUR", detected)
		if err != nil {
			t.Errorf("printHacksTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Travel Hacks",
		"HEL",
		"AMS",
		"Baseline: EUR 200",
		"Hidden City Ticketing",
		"saves EUR 50",
		"Date Flexibility",
		"Book HEL-AMS-CDG",
		"Airline may penalize",
		"example.com",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintHacksTable_NoHacks(t *testing.T) {
	models.UseColor = false

	out := captureStdout(t, func() {
		_ = printHacksTable("HEL", "AMS", "2026-04-15", 0, "EUR", nil)
	})

	if !strings.Contains(out, "No hacks detected") {
		t.Errorf("expected 'No hacks detected'")
	}
}

func TestPrintHacksTable_NoBaseline(t *testing.T) {
	models.UseColor = false

	detected := []hacks.Hack{
		{
			Type:        "split",
			Title:       "Split Ticketing",
			Description: "Book two separate tickets.",
			Savings:     0,
		},
	}

	out := captureStdout(t, func() {
		_ = printHacksTable("HEL", "AMS", "2026-04-15", 0, "EUR", detected)
	})

	// Without baseline, should not show "Baseline:" line.
	if strings.Contains(out, "Baseline:") {
		t.Errorf("should not show Baseline when naivePrice is 0")
	}
}

func TestPrintHack_NoCurrency(t *testing.T) {
	models.UseColor = false

	h := hacks.Hack{
		Type:        "throwaway",
		Title:       "Throwaway Ticketing",
		Description: "Buy a round-trip and skip the return.",
		Savings:     100,
		Currency:    "", // should default to EUR
	}

	out := captureStdout(t, func() {
		printHack(1, h)
	})

	if !strings.Contains(out, "EUR 100") {
		t.Errorf("expected default EUR currency, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// formatEventsCard
// ---------------------------------------------------------------------------

func TestFormatEventsCard_WithEvents(t *testing.T) {
	models.UseColor = false

	events := []models.Event{
		{
			Name:       "Rock Concert",
			Date:       "2026-07-01",
			Time:       "20:00",
			Venue:      "Palau Sant Jordi",
			Type:       "Music",
			PriceRange: "EUR 50-150",
		},
		{
			Name:  "Art Exhibition",
			Date:  "2026-07-02",
			Venue: "MACBA",
			Type:  "Arts",
		},
	}

	out := captureStdout(t, func() {
		err := formatEventsCard(events, "Barcelona", "2026-07-01", "2026-07-08")
		if err != nil {
			t.Errorf("formatEventsCard returned error: %v", err)
		}
	})

	for _, want := range []string{"EVENTS IN Barcelona", "2 events", "Rock Concert", "Palau Sant Jordi", "Music", "Art Exhibition"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatEventsCard_NoEvents(t *testing.T) {
	models.UseColor = false

	out := captureStdout(t, func() {
		_ = formatEventsCard(nil, "Helsinki", "2026-07-01", "2026-07-08")
	})

	if !strings.Contains(out, "No events found") {
		t.Errorf("expected 'No events found'")
	}
}

// ---------------------------------------------------------------------------
// printGridTable
// ---------------------------------------------------------------------------

func TestPrintGridTable_Success(t *testing.T) {
	models.UseColor = false

	result := &models.PriceGrid{
		Success:        true,
		Count:          4,
		DepartureDates: []string{"2026-07-01", "2026-07-02"},
		ReturnDates:    []string{"2026-07-08", "2026-07-09"},
		Cells: []models.GridCell{
			{DepartureDate: "2026-07-01", ReturnDate: "2026-07-08", Price: 450, Currency: "EUR"},
			{DepartureDate: "2026-07-01", ReturnDate: "2026-07-09", Price: 480, Currency: "EUR"},
			{DepartureDate: "2026-07-02", ReturnDate: "2026-07-08", Price: 420, Currency: "EUR"},
			{DepartureDate: "2026-07-02", ReturnDate: "2026-07-09", Price: 460, Currency: "EUR"},
		},
	}

	out := captureStdout(t, func() {
		err := printGridTable(context.Background(), "", result, "HEL", "NRT")
		if err != nil {
			t.Errorf("printGridTable returned error: %v", err)
		}
	})

	for _, want := range []string{"Price grid", "HEL", "NRT", "4 combinations", "EUR 450", "EUR 420"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintGridTable_Empty(t *testing.T) {
	models.UseColor = false

	result := &models.PriceGrid{
		Success: true,
		Count:   0,
	}

	out := captureStdout(t, func() {
		_ = printGridTable(context.Background(), "", result, "HEL", "NRT")
	})

	if !strings.Contains(out, "No price data found") {
		t.Errorf("expected 'No price data found'")
	}
}

func TestPrintGridTable_Failed(t *testing.T) {
	models.UseColor = false

	result := &models.PriceGrid{
		Success: false,
		Error:   "failed",
	}

	captureStdout(t, func() {
		_ = printGridTable(context.Background(), "", result, "HEL", "NRT")
	})
}

func TestPrintGridTable_MissingCell(t *testing.T) {
	models.UseColor = false

	result := &models.PriceGrid{
		Success:        true,
		Count:          1,
		DepartureDates: []string{"2026-07-01"},
		ReturnDates:    []string{"2026-07-08", "2026-07-09"},
		Cells: []models.GridCell{
			{DepartureDate: "2026-07-01", ReturnDate: "2026-07-08", Price: 350, Currency: "EUR"},
			// 2026-07-09 intentionally missing
		},
	}

	out := captureStdout(t, func() {
		_ = printGridTable(context.Background(), "", result, "HEL", "BCN")
	})

	// Missing cell should render as "-"
	if !strings.Contains(out, "-") {
		t.Errorf("expected '-' for missing grid cell")
	}
}

// ---------------------------------------------------------------------------
// formatGuideCard
// ---------------------------------------------------------------------------

func TestFormatGuideCard_Full(t *testing.T) {
	models.UseColor = false

	guide := &models.WikivoyageGuide{
		Location: "Barcelona",
		Summary:  "Barcelona is a vibrant Mediterranean city.",
		URL:      "https://en.wikivoyage.org/wiki/Barcelona",
		Sections: map[string]string{
			"See":        "Visit La Sagrada Familia and Park Guell.",
			"Eat":        "Try tapas at La Boqueria.",
			"Get in":     "Fly to El Prat airport.",
			"Stay safe":  "Watch for pickpockets on Las Ramblas.",
			"Extra Info": "Some custom section.",
		},
	}

	out := captureStdout(t, func() {
		err := formatGuideCard(guide)
		if err != nil {
			t.Errorf("formatGuideCard returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Barcelona",
		"wikivoyage.org",
		"OVERVIEW",
		"vibrant Mediterranean",
		"See",
		"La Sagrada Familia",
		"Eat",
		"tapas",
		"Get in",
		"El Prat",
		"Stay safe",
		"pickpockets",
		"Extra Info",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatGuideCard_Minimal(t *testing.T) {
	models.UseColor = false

	guide := &models.WikivoyageGuide{
		Location: "Nowhere",
		URL:      "https://en.wikivoyage.org/wiki/Nowhere",
		Sections: map[string]string{},
	}

	out := captureStdout(t, func() {
		_ = formatGuideCard(guide)
	})

	if !strings.Contains(out, "Nowhere") {
		t.Errorf("expected location name in output")
	}
}

// ---------------------------------------------------------------------------
// formatNearbyCard
// ---------------------------------------------------------------------------

func TestFormatNearbyCard_WithPOIs(t *testing.T) {
	models.UseColor = false

	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "Cafe Helsinki", Type: "cafe", Distance: 150, Cuisine: "Finnish", Hours: "08:00-20:00"},
			{Name: "Sushi Bar", Type: "restaurant", Distance: 300, Cuisine: "Japanese"},
		},
	}

	out := captureStdout(t, func() {
		err := formatNearbyCard(result)
		if err != nil {
			t.Errorf("formatNearbyCard returned error: %v", err)
		}
	})

	for _, want := range []string{"NEARBY PLACES", "2 found", "Cafe Helsinki", "cafe", "150m", "Finnish", "Sushi Bar"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatNearbyCard_WithRatedPlaces(t *testing.T) {
	models.UseColor = false

	result := &destinations.NearbyResult{
		POIs: []models.NearbyPOI{
			{Name: "Some Cafe", Type: "cafe", Distance: 100},
		},
		RatedPlaces: []models.RatedPlace{
			{Name: "Top Restaurant", Rating: 9.2, Category: "Italian", PriceLevel: 3, Distance: 200},
		},
	}

	out := captureStdout(t, func() {
		_ = formatNearbyCard(result)
	})

	for _, want := range []string{"TOP RATED", "Top Restaurant", "9.2/10", "Italian", "$$$"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatNearbyCard_WithAttractions(t *testing.T) {
	models.UseColor = false

	result := &destinations.NearbyResult{
		Attractions: []models.Attraction{
			{Name: "Helsinki Cathedral", Kind: "church", Distance: 500},
		},
	}

	out := captureStdout(t, func() {
		_ = formatNearbyCard(result)
	})

	for _, want := range []string{"ATTRACTIONS", "Helsinki Cathedral", "church", "500m"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatNearbyCard_Empty(t *testing.T) {
	models.UseColor = false

	result := &destinations.NearbyResult{}

	out := captureStdout(t, func() {
		_ = formatNearbyCard(result)
	})

	if !strings.Contains(out, "No nearby places found") {
		t.Errorf("expected 'No nearby places found'")
	}
}
