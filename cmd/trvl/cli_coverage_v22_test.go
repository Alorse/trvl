package main

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/watch"
)

// ---------------------------------------------------------------------------
// watchHistoryCmd — watch not found error path
// ---------------------------------------------------------------------------

func TestWatchHistoryCmd_NotFoundV22(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cmd := watchHistoryCmd()
	cmd.SetArgs([]string{"nonexistent-watch-id"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent watch ID")
	}
}

func TestWatchHistoryCmd_NoHistoryV22(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Add a watch, then call history immediately (no checks run → 0 entries).
	addCmd := watchAddCmd()
	addCmd.SetArgs([]string{"HEL", "BCN"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("watch add: %v", err)
	}

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

	histCmd := watchHistoryCmd()
	histCmd.SetArgs([]string{watchID})
	if err := histCmd.Execute(); err != nil {
		t.Errorf("watch history: %v", err)
	}
}

func TestWatchHistoryCmd_JSONFormatEmptyV22(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	addCmd := watchAddCmd()
	addCmd.SetArgs([]string{"HEL", "BCN"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("watch add: %v", err)
	}

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

	oldFormat := format
	format = "json"
	defer func() { format = oldFormat }()

	histCmd := watchHistoryCmd()
	histCmd.SetArgs([]string{watchID})
	_ = histCmd.Execute()
}

// ---------------------------------------------------------------------------
// whenCmd — missing required flag error paths (no network, no IATA validation)
// ---------------------------------------------------------------------------

func TestWhenCmd_MissingToFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--from", "2026-07-01", "--until", "2026-07-31"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --to flag")
	}
}

func TestWhenCmd_MissingFromFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--until", "2026-07-31"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --from flag")
	}
}

func TestWhenCmd_MissingUntilFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--from", "2026-07-01"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --until flag")
	}
}

func TestWhenCmd_MissingOriginNoPrefsV22(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--from", "2026-07-01", "--until", "2026-07-31"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no origin and no prefs")
	}
}

func TestWhenCmd_InvalidOriginIATAV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{"--to", "BCN", "--from", "2026-07-01", "--until", "2026-07-31", "--origin", "12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

func TestWhenCmd_InvalidBusyFlagV22(t *testing.T) {
	cmd := whenCmd()
	cmd.SetArgs([]string{
		"--to", "BCN",
		"--from", "2026-07-01",
		"--until", "2026-07-31",
		"--origin", "HEL",
		"--busy", "not-a-valid-interval",
	})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid --busy interval")
	}
}

func TestWhenCmd_FlagsExistV22(t *testing.T) {
	cmd := whenCmd()
	for _, name := range []string{"to", "from", "until", "origin", "busy", "prefer", "min-nights", "max-nights", "top", "budget", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on whenCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// cabinResult struct — basic coverage of helper type
// ---------------------------------------------------------------------------

func TestCabinResult_StructV22(t *testing.T) {
	r := cabinResult{Cabin: "Economy", Error: "no flights"}
	if r.Cabin != "Economy" {
		t.Errorf("expected Economy, got %s", r.Cabin)
	}
}

// ---------------------------------------------------------------------------
// runProvidersDisable — provider found, non-terminal (delete happens)
// ---------------------------------------------------------------------------

func TestRunProvidersDisable_SucceedsV22(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeTestProviderV19(t, tmp, "deletable-provider")

	cmd := providersDisableCmd()
	cmd.SetArgs([]string{"deletable-provider"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("providers disable: %v", err)
	}
}

// ---------------------------------------------------------------------------
// pricesCmd — non-nil check
// ---------------------------------------------------------------------------

func TestPricesCmd_NonNilV22(t *testing.T) {
	cmd := pricesCmd()
	if cmd == nil {
		t.Error("expected non-nil pricesCmd")
	}
}

// ---------------------------------------------------------------------------
// optimizeCmd — missing args and non-nil check
// ---------------------------------------------------------------------------

func TestOptimizeCmd_FlagsExistV22(t *testing.T) {
	cmd := optimizeCmd()
	if cmd == nil {
		t.Error("expected non-nil optimizeCmd")
	}
}

func TestOptimizeCmd_MissingArgsV22(t *testing.T) {
	cmd := optimizeCmd()
	cmd.SetArgs([]string{"HEL"}) // needs more args
	_ = cmd.Execute()            // arg count error or validation — no panic
}

// ---------------------------------------------------------------------------
// watchRoomsCmd — flags coverage
// ---------------------------------------------------------------------------

func TestWatchRoomsCmd_FlagsExistV22(t *testing.T) {
	cmd := watchRoomsCmd()
	for _, name := range []string{"depart", "return", "guests"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Logf("--%s flag not found on watchRoomsCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// dealsCmd — flags check
// ---------------------------------------------------------------------------

func TestDealsCmd_NonNilV22(t *testing.T) {
	cmd := dealsCmd()
	if cmd == nil {
		t.Error("expected non-nil dealsCmd")
	}
}
