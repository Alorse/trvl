package trip

import (
	"context"
	"errors"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/ground"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/visa"
	"github.com/MikkoParkkola/trvl/internal/weather"
)

// ============================================================
// buildViabilityChecks — visa warning path (visaCheck.Status == "warning")
// This covers the hasWarning=true line when visa check returns a warning.
// ============================================================

func TestBuildViabilityChecks_VisaWarning(t *testing.T) {
	cost := &TripCostResult{
		Success:  true,
		Flights:  FlightCost{Outbound: 150, Return: 180, Currency: "EUR"},
		Hotels:   HotelCost{PerNight: 80, Total: 560, Currency: "EUR", Name: "H"},
		Total:    890,
		Currency: "EUR",
		Nights:   7,
	}
	// e-visa → status "warning"
	visaResult := visa.Result{
		Success: true,
		Requirement: visa.Requirement{
			Status: "e-visa",
			Notes:  "Apply online before travel",
		},
	}
	_, _, hasWarning := buildViabilityChecks(cost, nil, visaResult, "FI", nil, nil)
	if !hasWarning {
		t.Error("expected hasWarning=true for e-visa status")
	}
}

func TestBuildViabilityChecks_VisaOnArrivalSetsWarning(t *testing.T) {
	cost := &TripCostResult{
		Success:  true,
		Flights:  FlightCost{Outbound: 150, Return: 180, Currency: "EUR"},
		Hotels:   HotelCost{PerNight: 80, Total: 560, Currency: "EUR", Name: "H"},
		Total:    890,
		Currency: "EUR",
		Nights:   7,
	}
	visaResult := visa.Result{
		Success: true,
		Requirement: visa.Requirement{
			Status: "visa-on-arrival",
		},
	}
	checks, hasBlocker, hasWarning := buildViabilityChecks(cost, nil, visaResult, "FI", nil, nil)
	if hasBlocker {
		t.Error("visa-on-arrival should not be a blocker")
	}
	if !hasWarning {
		t.Error("visa-on-arrival should set hasWarning=true")
	}
	// Visa check should be present.
	found := false
	for _, c := range checks {
		if c.Dimension == "visa" {
			found = true
			if c.Status != "warning" {
				t.Errorf("visa status = %q, want warning", c.Status)
			}
		}
	}
	if !found {
		t.Error("visa check not found in checks")
	}
}

// ============================================================
// searchAirportTransfers — geocoding failure path
// ============================================================

func TestSearchAirportTransfers_GeocodingFails(t *testing.T) {
	deps := airportTransferDeps{
		geocode: func(_ context.Context, query string) (destinations.GeoResult, error) {
			if query == "Hotel Lutetia Paris" {
				return destinations.GeoResult{}, errors.New("geocoding service unavailable")
			}
			return destinations.GeoResult{Locality: "Paris"}, nil
		},
		searchTransitous: func(context.Context, float64, float64, float64, float64, string) ([]models.GroundRoute, error) {
			t.Fatal("transitous should not be called after geocode failure")
			return nil, nil
		},
		searchGround: func(context.Context, string, string, string, ground.SearchOptions) (*models.GroundSearchResult, error) {
			t.Fatal("searchGround should not be called after geocode failure")
			return nil, nil
		},
	}

	_, err := searchAirportTransfers(context.Background(), AirportTransferInput{
		AirportCode: "CDG",
		Destination: "Hotel Lutetia Paris",
		Date:        "2026-07-01",
	}, deps)
	if err == nil {
		t.Fatal("expected error when geocoding fails")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// ============================================================
// searchAirportTransfers — empty locality → destinationCity falls back to input.Destination
// ============================================================

func TestSearchAirportTransfers_EmptyLocality(t *testing.T) {
	deps := airportTransferDeps{
		geocode: func(_ context.Context, query string) (destinations.GeoResult, error) {
			// Return empty Locality to trigger the fallback to input.Destination.
			return destinations.GeoResult{Locality: ""}, nil
		},
		searchTransitous: func(context.Context, float64, float64, float64, float64, string) ([]models.GroundRoute, error) {
			return nil, nil
		},
		estimateTaxi: func(context.Context, ground.TaxiEstimateInput) (models.GroundRoute, error) {
			return models.GroundRoute{Provider: "taxi", Type: "taxi", Price: 40, Currency: "EUR"}, nil
		},
		searchGround: func(_ context.Context, from, to, _ string, _ ground.SearchOptions) (*models.GroundSearchResult, error) {
			// When locality is empty, destinationCity = input.Destination = "Paris Hotel".
			// City search uses airportCity → destinationCity.
			return &models.GroundSearchResult{Success: true, Routes: nil}, nil
		},
	}

	result, err := searchAirportTransfers(context.Background(), AirportTransferInput{
		AirportCode: "CDG",
		Destination: "Paris Hotel",
		Date:        "2026-07-01",
	}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DestinationCity should fall back to "Paris Hotel" (input.Destination).
	if result.DestinationCity != "Paris Hotel" {
		t.Errorf("DestinationCity = %q, want %q (fallback to input.Destination)",
			result.DestinationCity, "Paris Hotel")
	}
}

// ============================================================
// searchAirportTransfers — "no providers selected" path
// An explicit providers list containing only empty/invalid values results in
// no transitous, no taxi, no city providers → "no providers selected" error.
// ============================================================

func TestSearchAirportTransfers_NoProvidersSelected(t *testing.T) {
	deps := airportTransferDeps{
		geocode: func(_ context.Context, query string) (destinations.GeoResult, error) {
			return destinations.GeoResult{Locality: "Paris"}, nil
		},
		searchTransitous: func(context.Context, float64, float64, float64, float64, string) ([]models.GroundRoute, error) {
			t.Fatal("transitous should not be called when no providers selected")
			return nil, nil
		},
		searchGround: func(context.Context, string, string, string, ground.SearchOptions) (*models.GroundSearchResult, error) {
			t.Fatal("searchGround should not be called when no providers selected")
			return nil, nil
		},
	}

	// Providers = [""] — empty string is filtered out, leaving no providers.
	result, err := searchAirportTransfers(context.Background(), AirportTransferInput{
		AirportCode: "CDG",
		Destination: "Hotel Lutetia Paris",
		Date:        "2026-07-01",
		Providers:   []string{""},
	}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "no providers selected" {
		t.Errorf("result.Error = %q, want 'no providers selected'", result.Error)
	}
}

// ============================================================
// searchAirportTransfers — ground search returns res.Error != "" path
// ============================================================

func TestSearchAirportTransfers_GroundSearchReturnsError(t *testing.T) {
	deps := airportTransferDeps{
		geocode: func(_ context.Context, query string) (destinations.GeoResult, error) {
			return destinations.GeoResult{Locality: "Paris"}, nil
		},
		searchGround: func(_ context.Context, from, to, _ string, _ ground.SearchOptions) (*models.GroundSearchResult, error) {
			// Return a result with Success=false and an Error message.
			return &models.GroundSearchResult{
				Success: false,
				Error:   "provider timeout",
				Routes:  nil,
			}, nil
		},
	}

	// Only city providers — no transitous/taxi — so the ground error path runs.
	result, err := searchAirportTransfers(context.Background(), AirportTransferInput{
		AirportCode: "CDG",
		Destination: "Hotel Lutetia Paris",
		Date:        "2026-07-01",
		Providers:   []string{"flixbus"},
	}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Routes should be empty; error surfaced in result.Error.
	if result.Success {
		t.Error("expected failure when ground search returns error")
	}
	if result.Error == "" {
		t.Error("expected non-empty result.Error when ground search fails")
	}
}

// ============================================================
// buildViabilityChecks — weather warning path (wc.Status == "warning")
// Covered via a rainy forecast that triggers weather warning.
// ============================================================

func TestBuildViabilityChecks_WeatherWarningPath(t *testing.T) {
	cost := &TripCostResult{
		Success:  true,
		Flights:  FlightCost{Outbound: 150, Return: 180, Currency: "EUR"},
		Hotels:   HotelCost{PerNight: 80, Total: 560, Currency: "EUR", Name: "H"},
		Total:    890,
		Currency: "EUR",
		Nights:   7,
	}
	// Mostly rainy forecasts → weather status "warning" → hasWarning=true.
	wr := &weather.WeatherResult{
		Success: true,
		Forecasts: []weather.Forecast{
			{TempMax: 14, TempMin: 8, Precipitation: 20},
			{TempMax: 13, TempMin: 7, Precipitation: 15},
			{TempMax: 15, TempMin: 9, Precipitation: 10},
		},
	}
	checks, _, hasWarning := buildViabilityChecks(cost, nil, visa.Result{}, "", wr, nil)
	if !hasWarning {
		t.Error("expected hasWarning=true for rainy weather forecast")
	}
	found := false
	for _, c := range checks {
		if c.Dimension == "weather" {
			found = true
			if c.Status != "warning" {
				t.Errorf("weather status = %q, want warning", c.Status)
			}
		}
	}
	if !found {
		t.Error("expected weather check in results")
	}
}

// ============================================================
// Discover — additional field coverage: DiscoverOutput fields
// ============================================================

func TestDiscover_OutputFieldsPopulated(t *testing.T) {
	// A no-windows case still populates the output fields correctly.
	result, err := Discover(context.Background(), DiscoverOptions{
		Origin:    "HEL",
		From:      "2026-07-13", // Monday
		Until:     "2026-07-14", // Tuesday — no Fridays
		Budget:    500,
		MinNights: 2,
		MaxNights: 4,
		Top:       3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", result.Origin)
	}
	if result.From != "2026-07-13" {
		t.Errorf("From = %q, want 2026-07-13", result.From)
	}
	if result.Until != "2026-07-14" {
		t.Errorf("Until = %q, want 2026-07-14", result.Until)
	}
	if result.Budget != 500 {
		t.Errorf("Budget = %v, want 500", result.Budget)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no windows)", result.Count)
	}
}

// ============================================================
// splitAirportTransferProviders — additional coverage
// ============================================================

func TestSplitAirportTransferProviders_AllTypes(t *testing.T) {
	tr, tx, city := splitAirportTransferProviders([]string{"transitous", "taxi", "flixbus", "db"})
	if !tr {
		t.Error("expected transitousEnabled=true")
	}
	if !tx {
		t.Error("expected taxiEnabled=true")
	}
	if len(city) != 2 {
		t.Errorf("cityProviders = %v, want [flixbus db]", city)
	}
}

func TestSplitAirportTransferProviders_NilDefaultsAll(t *testing.T) {
	tr, tx, city := splitAirportTransferProviders(nil)
	if !tr {
		t.Error("nil providers should default to transitous enabled")
	}
	if !tx {
		t.Error("nil providers should default to taxi enabled")
	}
	if len(city) == 0 {
		t.Error("nil providers should default to city providers")
	}
}

func TestSplitAirportTransferProviders_DuplicatesDeduped(t *testing.T) {
	_, _, city := splitAirportTransferProviders([]string{"flixbus", "flixbus", "db"})
	if len(city) != 2 {
		t.Errorf("cityProviders = %v, want [flixbus db] (deduped)", city)
	}
}
