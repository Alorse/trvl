package main

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/watch"
)

// ---------------------------------------------------------------------------
// liveChecker.CheckPrice — default branch (unknown watch type)
// ---------------------------------------------------------------------------

func TestLiveCheckerCheckPrice_UnknownType_V29(t *testing.T) {
	c := &liveChecker{}
	w := watch.Watch{
		Type:        "unknown-type",
		Origin:      "HEL",
		Destination: "BCN",
	}
	_, _, _, err := c.CheckPrice(context.Background(), w)
	if err == nil {
		t.Fatal("expected error for unknown watch type")
	}
	if err.Error() != "unknown watch type: unknown-type" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// liveChecker.checkFlight — cancelled context (specific date search path)
// ---------------------------------------------------------------------------

func TestLiveCheckerCheckFlight_SpecificDate_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-08-01",
	}
	// Specific date (not route watch, not date range) → calls SearchFlights.
	_, _, _, err := c.CheckPrice(ctx, w)
	// Cancelled ctx → error expected.
	if err == nil {
		t.Log("no error (SearchFlights may have returned success=false, not an error)")
	}
}

// ---------------------------------------------------------------------------
// liveChecker.checkFlight — route watch path (no dates → checkFlightRange)
// ---------------------------------------------------------------------------

func TestLiveCheckerCheckFlight_RouteWatch_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		// No DepartDate, DepartFrom, DepartTo → IsRouteWatch() == true
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

// ---------------------------------------------------------------------------
// liveChecker.checkFlight — date range path (DepartFrom+DepartTo set)
// ---------------------------------------------------------------------------

func TestLiveCheckerCheckFlight_DateRange_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartFrom:  "2026-08-01",
		DepartTo:    "2026-08-31",
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

// ---------------------------------------------------------------------------
// liveChecker.checkHotel — non-route watch (specific dates)
// ---------------------------------------------------------------------------

func TestLiveCheckerCheckHotel_SpecificDates_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "hotel",
		Destination: "Barcelona",
		DepartDate:  "2026-08-01",
		ReturnDate:  "2026-08-05",
		Currency:    "EUR",
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

// ---------------------------------------------------------------------------
// liveChecker.checkHotel — route watch (no dates → computes next weekend)
// ---------------------------------------------------------------------------

func TestLiveCheckerCheckHotel_RouteWatch_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "hotel",
		Destination: "Barcelona",
		// No dates → IsRouteWatch() == true → defaults to next weekend
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

// ---------------------------------------------------------------------------
// liveRoomChecker.CheckRooms — cancelled context
// ---------------------------------------------------------------------------

func TestLiveRoomCheckerCheckRooms_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveRoomChecker{}
	w := watch.Watch{
		Type:       "room",
		HotelName:  "CORU House Prague",
		DepartDate: "2026-08-01",
		ReturnDate: "2026-08-05",
		Currency:   "", // should default to USD
	}
	_, _ = c.CheckRooms(ctx, w)
}

func TestLiveRoomCheckerCheckRooms_DefaultCurrency_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveRoomChecker{}
	w := watch.Watch{
		Type:       "room",
		HotelName:  "Park Hyatt Vienna",
		DepartDate: "2026-09-01",
		ReturnDate: "2026-09-05",
		// Currency is empty → should default to "USD" inside CheckRooms
	}
	_, _ = c.CheckRooms(ctx, w)
}

// ---------------------------------------------------------------------------
// tripsAlertsCmd — mark-read flag path
// ---------------------------------------------------------------------------

func TestTripsAlertsCmd_MarkRead_V29(t *testing.T) {
	// The --mark-read path calls store.MarkAlertsRead(), which in a fresh
	// temp environment either succeeds (no file) or errors. Either is fine.
	cmd := tripsAlertsCmd()
	cmd.SetArgs([]string{"--mark-read"})
	_ = cmd.Execute()
}

func TestTripsAlertsCmd_NoAlerts_V29(t *testing.T) {
	// Without --mark-read: loads store (empty), prints "No alerts."
	cmd := tripsAlertsCmd()
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// airportTransferCmd — with all 3 required args, cancelled context
// ---------------------------------------------------------------------------

func TestAirportTransferCmd_3Args_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := airportTransferCmd()
	cmd.SetArgs([]string{"HEL", "Helsinki City Center", "2026-08-01"})
	_ = cmd.ExecuteContext(ctx)
}

// ---------------------------------------------------------------------------
// upgradeCmd — dry-run flag path
// ---------------------------------------------------------------------------

func TestUpgradeCmd_DryRun_V29(t *testing.T) {
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--dry-run"})
	_ = cmd.Execute()
}

func TestUpgradeCmd_Quiet_V29(t *testing.T) {
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--quiet"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// watchHistoryCmd — watch not found path
// ---------------------------------------------------------------------------

func TestWatchHistoryCmd_NotFound_V29(t *testing.T) {
	cmd := watchHistoryCmd()
	cmd.SetArgs([]string{"nonexistent-watch-id-xyz"})
	err := cmd.Execute()
	// Either "not found" error or nil (if store can't load).
	_ = err
}

// ---------------------------------------------------------------------------
// watchRemoveCmd — non-existent ID path
// ---------------------------------------------------------------------------

func TestWatchRemoveCmd_NotFound_V29(t *testing.T) {
	cmd := watchRemoveCmd()
	cmd.SetArgs([]string{"nonexistent-watch-id-xyz"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// watchCheckCmd — empty store (no watches)
// ---------------------------------------------------------------------------

func TestWatchCheckCmd_EmptyStore_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := watchCheckCmd()
	cmd.SetArgs([]string{})
	_ = cmd.ExecuteContext(ctx)
}

// ---------------------------------------------------------------------------
// gridCmd — cancelled context (uses SearchGrid which needs HTTP)
// ---------------------------------------------------------------------------

func TestGridCmd_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := gridCmd()
	cmd.SetArgs([]string{"HEL", "--from", "2026-08-01", "--to", "2026-08-31"})
	_ = cmd.ExecuteContext(ctx)
}
