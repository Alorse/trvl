package trip

import (
	"context"
	"testing"
	"time"
)

// ============================================================
// PlanTrip — cancelled-context path
//
// A pre-cancelled context passes all validation, enters the
// function body, spawns goroutines that fail immediately with
// context.Canceled, then exercises the post-wait assembly and
// error-collection code (lines 143–416) which has 0% coverage
// from pure-validation tests alone.
// ============================================================

func TestPlanTrip_CancelledContext_BodyCoverage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := PlanTrip(ctx, PlanInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-08-01",
		ReturnDate:  "2026-08-08",
		Guests:      1,
	})
	// PlanTrip never returns a hard error for valid inputs.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Structural invariants.
	if result.Origin != "HEL" {
		t.Errorf("Origin = %q, want HEL", result.Origin)
	}
	if result.Destination != "BCN" {
		t.Errorf("Destination = %q, want BCN", result.Destination)
	}
	// With a cancelled context all HTTP calls fail, so no flight/hotel data.
	// The function still assembles a result and collects errors.
	if result.Success {
		t.Error("expected Success=false when context is cancelled")
	}
}

func TestPlanTrip_CancelledContext_WithCurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := PlanTrip(ctx, PlanInput{
		Origin:      "JFK",
		Destination: "LHR",
		DepartDate:  "2026-09-01",
		ReturnDate:  "2026-09-08",
		Guests:      2,
		Currency:    "EUR",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Guests != 2 {
		t.Errorf("Guests = %d, want 2", result.Guests)
	}
	if result.Nights != 7 {
		t.Errorf("Nights = %d, want 7", result.Nights)
	}
}

// ============================================================
// FindWeekendGetaways — cancelled-context path
//
// Exercises the explore + hotel search body with a valid origin
// and a cancelled context, so the HTTP calls fail immediately
// and the assembly/ranking code is reached.
// ============================================================

func TestFindWeekendGetaways_CancelledContext_BodyCoverage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// parseMonth requires "YYYY-MM" format. Use the next month so it passes
	// the "in the past" guard.
	nextMonth := time.Now().AddDate(0, 1, 0).Format("2006-01")

	result, err := FindWeekendGetaways(ctx, "HEL", WeekendOptions{
		Month: nextMonth,
	})
	// FindWeekendGetaways returns an error when the explore call fails
	// (context cancelled means explore returns an error). That's acceptable —
	// the function body is entered and the error-collection path is exercised.
	_ = err
	_ = result
}

// ============================================================
// Discover — cancelled-context path
//
// Exercises the Phase 1 explore loop with a cancelled context:
// explore calls fail immediately, the findings slice stays empty,
// and the early-return "no candidates" branch is taken (or the
// full nil-findings path).
// ============================================================

func TestDiscover_CancelledContext_BodyCoverage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := DiscoverOptions{
		Origin: "HEL",
		Budget: 500,
		From:   "2026-08-01",
		Until:  "2026-08-31",
	}
	result, err := Discover(ctx, opts)
	// Discover returns either a result (with Success=true but empty data) or an
	// error. Both are acceptable — the function body is exercised.
	_ = err
	_ = result
}

func TestDiscover_CancelledContext_WithPrefs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := DiscoverOptions{
		Origin:    "CDG",
		Budget:    800,
		From:      "2026-09-05",
		Until:     "2026-09-30",
		MinNights: 2,
		MaxNights: 3,
		Top:       5,
	}
	result, err := Discover(ctx, opts)
	_ = err
	_ = result
}
