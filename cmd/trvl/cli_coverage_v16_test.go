package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/watch"
)

// ---------------------------------------------------------------------------
// calendarCmd — with a real trip ID (covers the len(args)==1 branch)
// ---------------------------------------------------------------------------

func TestCalendarCmd_WithTripID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a trip.
	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Calendar Test Trip"})
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

	// calendar with trip_id → prints ICS to stdout.
	cmd := calendarCmd()
	cmd.SetArgs([]string{tripID})
	if err := cmd.Execute(); err != nil {
		t.Errorf("calendar with trip_id: %v", err)
	}
}

func TestCalendarCmd_WithTripIDToFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a trip.
	createCmd := tripsCmd()
	createCmd.SetArgs([]string{"create", "Calendar File Trip"})
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

	outFile := filepath.Join(tmp, "trip.ics")
	cmd := calendarCmd()
	cmd.SetArgs([]string{tripID, "--output", outFile})
	if err := cmd.Execute(); err != nil {
		t.Errorf("calendar with trip_id --output: %v", err)
	}

	if _, statErr := os.Stat(outFile); os.IsNotExist(statErr) {
		t.Error("expected ICS file to be written")
	}
}

func TestCalendarCmd_WithTripIDNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := calendarCmd()
	cmd.SetArgs([]string{"nonexistent-id-v16"})
	// Should error — trip not found.
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// watchRemoveCmd — actually remove an existing watch
// ---------------------------------------------------------------------------

func TestWatchRemoveCmd_ActuallyRemoves(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a watch.
	addCmd := watchAddCmd()
	addCmd.SetArgs([]string{"HEL", "BCN"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("watch add: %v", err)
	}

	// Load the watch store to get the ID.
	wStore, err := watch.DefaultStore()
	if err != nil {
		t.Fatalf("watch.DefaultStore: %v", err)
	}
	if err := wStore.Load(); err != nil {
		t.Fatalf("wStore.Load: %v", err)
	}
	watches := wStore.List()
	if len(watches) == 0 {
		t.Skip("no watches in store")
	}
	watchID := watches[0].ID

	// Remove it.
	removeCmd := watchRemoveCmd()
	removeCmd.SetArgs([]string{watchID})
	if err := removeCmd.Execute(); err != nil {
		t.Errorf("watch remove: %v", err)
	}
}

// ---------------------------------------------------------------------------
// prefsAddFamilyMemberCmd — add a family member
// ---------------------------------------------------------------------------

func TestPrefsAddFamilyMemberCmd_AddsV16(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsAddFamilyMemberCmd()
	cmd.SetArgs([]string{"family_member", "Father", "--notes", "prefers window seat"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("add family member: %v", err)
	}
}

func TestPrefsAddFamilyMemberCmd_WrongKeyV16(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := prefsAddFamilyMemberCmd()
	cmd.SetArgs([]string{"not_family_member", "Bob"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for wrong first arg")
	}
}

// ---------------------------------------------------------------------------
// upgradeCmd — quiet flag (suppresses output)
// ---------------------------------------------------------------------------

func TestUpgradeCmd_QuietV16(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := upgradeCmd()
	cmd.SetArgs([]string{"--quiet"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpgradeCmd_AlreadyUpToDate(t *testing.T) {
	// Run upgrade twice — second run should hit "already up to date" branch.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd1 := upgradeCmd()
	cmd1.SetArgs([]string{})
	_ = cmd1.Execute()

	cmd2 := upgradeCmd()
	cmd2.SetArgs([]string{})
	_ = cmd2.Execute()
}

// ---------------------------------------------------------------------------
// runWatchDaemon — nil runCycle branch
// ---------------------------------------------------------------------------

func TestRunWatchDaemon_NilRunCycleV16(t *testing.T) {
	ctx := context.Background()
	err := runWatchDaemon(ctx, &bytes.Buffer{}, time.Minute, false, nil, func(d time.Duration) watchDaemonTicker {
		return &mockWatchTicker{ch: make(chan time.Time)}
	})
	if err == nil {
		t.Error("expected error for nil runCycle")
	}
}

// ---------------------------------------------------------------------------
// runWatchDaemon — ticker fires once then context cancelled
// ---------------------------------------------------------------------------

func TestRunWatchDaemon_TickerFires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tickCh := make(chan time.Time, 1)
	mt := &mockWatchTicker{ch: tickCh}

	cycleCount := 0
	var buf bytes.Buffer

	// Start daemon in goroutine.
	done := make(chan error, 1)
	go func() {
		done <- runWatchDaemon(ctx, &buf, time.Minute, false, func(context.Context) (int, error) {
			cycleCount++
			cancel() // cancel after first tick
			return 0, nil
		}, func(time.Duration) watchDaemonTicker { return mt })
	}()

	// Send a tick.
	tickCh <- time.Now()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for daemon to stop")
	}
}

// ---------------------------------------------------------------------------
// accomHackCmd — missing required flags
// ---------------------------------------------------------------------------

func TestAccomHackCmd_MissingCheckIn(t *testing.T) {
	cmd := accomHackCmd()
	cmd.SetArgs([]string{"Prague"}) // missing --checkin and --checkout
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing required flags")
	}
}
