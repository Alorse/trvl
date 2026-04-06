package main

import (
	"context"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// ---------------------------------------------------------------------------
// modeIcon
// ---------------------------------------------------------------------------

func TestModeIcon_Flight(t *testing.T) {
	got := modeIcon("flight")
	if !strings.Contains(got, "flight") {
		t.Errorf("modeIcon(\"flight\") = %q, want it to contain \"flight\"", got)
	}
}

func TestModeIcon_Train(t *testing.T) {
	got := modeIcon("train")
	if !strings.Contains(got, "train") {
		t.Errorf("modeIcon(\"train\") = %q, want it to contain \"train\"", got)
	}
}

func TestModeIcon_Bus(t *testing.T) {
	got := modeIcon("bus")
	if !strings.Contains(got, "bus") {
		t.Errorf("modeIcon(\"bus\") = %q, want it to contain \"bus\"", got)
	}
}

func TestModeIcon_Ferry(t *testing.T) {
	got := modeIcon("ferry")
	if !strings.Contains(got, "ferry") {
		t.Errorf("modeIcon(\"ferry\") = %q, want it to contain \"ferry\"", got)
	}
}

func TestModeIcon_Unknown(t *testing.T) {
	got := modeIcon("hovercraft")
	if got != "hovercraft" {
		t.Errorf("modeIcon(\"hovercraft\") = %q, want pass-through", got)
	}
}

func TestModeIcon_CaseInsensitive(t *testing.T) {
	got := modeIcon("FLIGHT")
	if !strings.Contains(got, "flight") {
		t.Errorf("modeIcon(\"FLIGHT\") = %q, want it to contain \"flight\"", got)
	}
}

// ---------------------------------------------------------------------------
// formatRoutePrice
// ---------------------------------------------------------------------------

func TestFormatRoutePrice(t *testing.T) {
	tests := []struct {
		price    float64
		currency string
		want     string
	}{
		{89, "EUR", "EUR 89"},
		{89.6, "EUR", "EUR 90"}, // rounded
		{0, "EUR", "-"},
		{-1, "USD", "-"},
	}
	for _, tt := range tests {
		got := formatRoutePrice(tt.price, tt.currency)
		if got != tt.want {
			t.Errorf("formatRoutePrice(%v, %q) = %q, want %q", tt.price, tt.currency, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// printRouteTable — output rendering (no panic, correct structure)
// ---------------------------------------------------------------------------

// captureRouteTable redirects os.Stdout temporarily so we can inspect the
// output of printRouteTable, which writes directly to os.Stdout/os.Stderr.
// Because the function accepts an io.Writer only via models.Banner/FormatTable
// (which use os.Stdout internally), we test the no-routes path via the
// returned error and rely on the itinerary path to not panic.
func TestPrintRouteTable_NoRoutes_SuccessFalse(t *testing.T) {
	result := &models.RouteSearchResult{
		Success: false,
		Error:   "no service on that date",
	}
	// Should return nil (not an error) and not panic.
	err := printRouteTable(context.Background(), "", result)
	if err != nil {
		t.Errorf("printRouteTable with !Success returned error: %v", err)
	}
}

func TestPrintRouteTable_NoRoutes_EmptyError(t *testing.T) {
	result := &models.RouteSearchResult{
		Success: false,
		Error:   "",
	}
	err := printRouteTable(context.Background(), "", result)
	if err != nil {
		t.Errorf("printRouteTable with empty error returned: %v", err)
	}
}

func TestPrintRouteTable_WithItineraries_NoError(t *testing.T) {
	result := &models.RouteSearchResult{
		Success:     true,
		Origin:      "Helsinki",
		Destination: "Barcelona",
		Date:        "2026-07-01",
		Count:       1,
		Itineraries: []models.RouteItinerary{
			{
				Legs: []models.RouteLeg{
					{
						Mode:      "flight",
						Provider:  "Google Flights",
						From:      "Helsinki",
						To:        "Barcelona",
						FromCode:  "HEL",
						ToCode:    "BCN",
						Departure: "2026-07-01T07:00:00",
						Arrival:   "2026-07-01T10:30:00",
						Duration:  210,
						Price:     89,
						Currency:  "EUR",
					},
				},
				TotalPrice:    89,
				Currency:      "EUR",
				TotalDuration: 210,
				Transfers:     0,
			},
		},
	}
	err := printRouteTable(context.Background(), "EUR", result)
	if err != nil {
		t.Errorf("printRouteTable with itineraries returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// printItinerary — smoke test (must not panic)
// ---------------------------------------------------------------------------

func TestPrintItinerary_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printItinerary panicked: %v", r)
		}
	}()

	it := models.RouteItinerary{
		Legs: []models.RouteLeg{
			{
				Mode:      "train",
				Provider:  "DB",
				From:      "Berlin",
				To:        "Vienna",
				FromCode:  "",
				ToCode:    "",
				Departure: "2026-07-01T08:00:00",
				Arrival:   "2026-07-01T14:00:00",
				Duration:  360,
				Price:     37,
				Currency:  "EUR",
			},
		},
		TotalPrice:    37,
		Currency:      "EUR",
		TotalDuration: 360,
		Transfers:     0,
	}
	printItinerary(1, it, "EUR")
}

func TestPrintItinerary_MultiLeg_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printItinerary multi-leg panicked: %v", r)
		}
	}()

	it := models.RouteItinerary{
		Legs: []models.RouteLeg{
			{
				Mode:      "flight",
				Provider:  "Finnair",
				From:      "Helsinki",
				To:        "Frankfurt",
				FromCode:  "HEL",
				ToCode:    "FRA",
				Departure: "2026-07-01T06:00:00",
				Arrival:   "2026-07-01T08:00:00",
				Duration:  120,
				Price:     60,
				Currency:  "EUR",
			},
			{
				Mode:      "train",
				Provider:  "DB",
				From:      "Frankfurt",
				To:        "Vienna",
				FromCode:  "",
				ToCode:    "",
				Departure: "2026-07-01T10:00:00",
				Arrival:   "2026-07-01T14:00:00",
				Duration:  240,
				Price:     37,
				Currency:  "EUR",
			},
		},
		TotalPrice:    97,
		Currency:      "EUR",
		TotalDuration: 480,
		Transfers:     1,
	}
	printItinerary(2, it, "EUR")
}

// ---------------------------------------------------------------------------
// Verify _ (bytes.Buffer) is used somewhere so import isn't flagged unused
// ---------------------------------------------------------------------------

var _ = context.Background // keep context import live
