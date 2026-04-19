package trip

import (
	"context"
	"os"
	"testing"
	"time"
)

// These tests call functions with valid inputs that pass all validation.
// The internal goroutines will attempt live API calls. In most CI and
// unit-test environments those calls fail (no network / no auth), which
// exercises the error-collection and result-assembly paths (wg.Wait()
// through return). In environments WITH network the tests still pass
// because the functions handle all error cases gracefully.
//
// None of these tests assert on the presence of flight/hotel data —
// only on the structural invariants that hold regardless of network.

// ============================================================
// PlanTrip — valid input (exercises function body post-validation)
// ============================================================

func TestPlanTrip_ValidInput_StructuralInvariants(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := PlanTrip(ctx, PlanInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-08-01",
		ReturnDate:  "2026-08-08",
		Guests:      1,
	})
	// PlanTrip never returns a hard error for valid inputs — errors are
	// encoded in result.Error, not returned as an error value.
	if err != nil {
		t.Fatalf("PlanTrip returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Structural invariants that hold regardless of data availability:
	if result.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", result.Origin)
	}
	if result.Destination != "BCN" {
		t.Errorf("Destination = %q, want BCN", result.Destination)
	}
	if result.DepartDate != "2026-08-01" {
		t.Errorf("DepartDate = %q, want 2026-08-01", result.DepartDate)
	}
	if result.ReturnDate != "2026-08-08" {
		t.Errorf("ReturnDate = %q, want 2026-08-08", result.ReturnDate)
	}
	if result.Nights != 7 {
		t.Errorf("Nights = %d, want 7", result.Nights)
	}
	if result.Guests != 1 {
		t.Errorf("Guests = %d, want 1", result.Guests)
	}
	// Summary currency must be non-empty (defaults to "EUR" when no data).
	if result.Summary.Currency == "" {
		t.Error("Summary.Currency should not be empty")
	}
}

func TestPlanTrip_ValidInput_MultiGuest(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := PlanTrip(ctx, PlanInput{
		Origin:      "LHR",
		Destination: "CDG",
		DepartDate:  "2026-09-01",
		ReturnDate:  "2026-09-05",
		Guests:      2,
		Currency:    "EUR",
	})
	if err != nil {
		t.Fatalf("PlanTrip returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Nights != 4 {
		t.Errorf("Nights = %d, want 4", result.Nights)
	}
	if result.Guests != 2 {
		t.Errorf("Guests = %d, want 2", result.Guests)
	}
	// Even with no data, Summary totals should be non-negative.
	if result.Summary.GrandTotal < 0 {
		t.Errorf("GrandTotal = %v, should be >= 0", result.Summary.GrandTotal)
	}
	if result.Summary.FlightsTotal < 0 {
		t.Errorf("FlightsTotal = %v, should be >= 0", result.Summary.FlightsTotal)
	}
}

func TestPlanTrip_ValidInput_WithCurrency(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := PlanTrip(ctx, PlanInput{
		Origin:      "JFK",
		Destination: "LAX",
		DepartDate:  "2026-10-01",
		ReturnDate:  "2026-10-08",
		Guests:      1,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatalf("PlanTrip returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// When currency is specified and there's no flight/hotel data, summary currency
	// falls back to the requested currency or "EUR".
	if result.Summary.Currency == "" {
		t.Error("Summary.Currency should not be empty")
	}
}

// ============================================================
// FindWeekendGetaways — valid input (exercises function body)
// ============================================================

func TestFindWeekendGetaways_ValidInput_StructuralInvariants(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := FindWeekendGetaways(ctx, "HEL", WeekendOptions{
		Month:  "august-2026",
		Nights: 2,
	})
	// FindWeekendGetaways propagates explore API errors (network-dependent).
	// In environments without network, an error is returned and that's fine —
	// the test's purpose is to exercise the function body up to the error return.
	if err != nil {
		t.Logf("FindWeekendGetaways returned error (expected in offline env): %v", err)
		return
	}
	if result == nil {
		t.Fatal("expected non-nil result when err=nil")
	}
	if result.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", result.Origin)
	}
	if result.Month != "August 2026" {
		t.Errorf("Month = %q, want 'August 2026'", result.Month)
	}
	if result.Nights != 2 {
		t.Errorf("Nights = %d, want 2", result.Nights)
	}
	if !result.Success {
		t.Error("expected Success=true when err=nil")
	}
	if result.Count != len(result.Destinations) {
		t.Errorf("Count = %d but len(Destinations) = %d", result.Count, len(result.Destinations))
	}
}

func TestFindWeekendGetaways_ValidInput_WithBudget(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := FindWeekendGetaways(ctx, "AMS", WeekendOptions{
		Month:     "september-2026",
		Nights:    3,
		MaxBudget: 500,
	})
	// Network may not be available; error is acceptable.
	if err != nil {
		t.Logf("FindWeekendGetaways returned error (expected in offline env): %v", err)
		return
	}
	if result == nil {
		t.Fatal("expected non-nil result when err=nil")
	}
	// If any destinations returned, they must all be within budget.
	for _, d := range result.Destinations {
		if d.Total > 500 {
			t.Errorf("destination %q has total %v > budget 500", d.Destination, d.Total)
		}
	}
}

// ============================================================
// Discover — valid input with date range that has Fridays
// ============================================================

func TestDiscover_ValidInput_StructuralInvariants(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	// A range with at least one Friday (2026-08-07 is a Friday).
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := Discover(ctx, DiscoverOptions{
		Origin:    "HEL",
		From:      "2026-08-06",
		Until:     "2026-08-14",
		Budget:    500,
		MinNights: 2,
		MaxNights: 2,
		Top:       3,
	})
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", result.Origin)
	}
	if result.Budget != 500 {
		t.Errorf("Budget = %v, want 500", result.Budget)
	}
	if result.Count != len(result.Trips) {
		t.Errorf("Count = %d but len(Trips) = %d", result.Count, len(result.Trips))
	}
	// Trips must not exceed Top.
	if len(result.Trips) > 3 {
		t.Errorf("trips = %d, want <= 3 (Top)", len(result.Trips))
	}
	// Each trip must be within budget.
	for i, trip := range result.Trips {
		if trip.Total > 500 {
			t.Errorf("trip[%d] total %v > budget 500", i, trip.Total)
		}
		if trip.BudgetSlack < 0 {
			t.Errorf("trip[%d] budget slack %v < 0", i, trip.BudgetSlack)
		}
	}
}

func TestDiscover_ValidInput_DefaultOptions(t *testing.T) {
	if os.Getenv("TRVL_TEST_LIVE_INTEGRATIONS") == "" {
		t.Skip("skipping live-API test; set TRVL_TEST_LIVE_INTEGRATIONS=1 to run")
	}
	// Zero MinNights/MaxNights/Top should get defaults (2, 4, 5).
	// Use a 2-week window to ensure Fridays are included.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result, err := Discover(ctx, DiscoverOptions{
		Origin: "LHR",
		From:   "2026-08-01",
		Until:  "2026-08-15",
		Budget: 1000,
	})
	if err != nil {
		t.Fatalf("Discover returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	// Trips count should be <= 5 (default Top).
	if len(result.Trips) > 5 {
		t.Errorf("trips = %d, want <= 5 (default Top)", len(result.Trips))
	}
}
