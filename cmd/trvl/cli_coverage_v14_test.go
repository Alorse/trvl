package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/trip"
	"github.com/MikkoParkkola/trvl/internal/tripwindow"
)

// ---------------------------------------------------------------------------
// shouldShowNudge — branches not covered elsewhere
// ---------------------------------------------------------------------------

func TestShouldShowNudge_NotSearchCmdV14(t *testing.T) {
	got := shouldShowNudge("setup", "", os.Getenv, os.Stderr.Fd(), func(int) bool { return true })
	if got {
		t.Error("expected false for non-search command")
	}
}

func TestShouldShowNudge_TrvlNoNudgeEnvV14(t *testing.T) {
	got := shouldShowNudge("flights", "", func(key string) string {
		if key == "TRVL_NO_NUDGE" {
			return "1"
		}
		return ""
	}, os.Stderr.Fd(), func(int) bool { return true })
	if got {
		t.Error("expected false when TRVL_NO_NUDGE=1")
	}
}

// ---------------------------------------------------------------------------
// nudge state — filesystem operations not covered by nudge_test.go
// ---------------------------------------------------------------------------

func TestLoadNudgeState_InvalidJSONV14(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "nudge.json")
	_ = os.WriteFile(p, []byte("not-json"), 0o600)
	s := loadNudgeState(p)
	if s.SearchCount != 0 || s.Shown {
		t.Errorf("expected zero nudgeState for invalid JSON, got %+v", s)
	}
}

func TestSaveAndLoadNudgeStateV14(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "nudge.json")
	original := nudgeState{SearchCount: 2, Shown: false}
	saveNudgeState(p, original)
	loaded := loadNudgeState(p)
	if loaded.SearchCount != 2 {
		t.Errorf("expected SearchCount=2, got %d", loaded.SearchCount)
	}
}

func TestSaveNudgeState_ShownTrueV14(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "nudge.json")
	s := nudgeState{SearchCount: 5, Shown: true, ShownAt: time.Now()}
	saveNudgeState(p, s)
	loaded := loadNudgeState(p)
	if !loaded.Shown {
		t.Error("expected Shown=true after save")
	}
}

// ---------------------------------------------------------------------------
// printWhenTable — additional branch: with preferred and hotel name note
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// printSuggestTable — branches not yet covered
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// multiCityCmd — missing --dates path
// ---------------------------------------------------------------------------

func TestMultiCityCmd_MissingDatesV14(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SetArgs([]string{"HEL", "--visit", "BCN,ROM"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --dates is missing")
	}
}

func TestMultiCityCmd_InvalidCityIATAV14(t *testing.T) {
	cmd := multiCityCmd()
	cmd.SetArgs([]string{"HEL", "--visit", "12,ROM", "--dates", "2026-07-01,2026-07-21"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid city IATA in --visit")
	}
}

func TestMultiCityCmd_FlagsExistV14(t *testing.T) {
	cmd := multiCityCmd()
	for _, name := range []string{"visit", "dates", "format"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on multiCityCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// weekendCmd — invalid IATA path
// ---------------------------------------------------------------------------

func TestWeekendCmd_InvalidIATAV14(t *testing.T) {
	cmd := weekendCmd()
	cmd.SetArgs([]string{"12"}) // invalid IATA
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid origin IATA")
	}
}

// ---------------------------------------------------------------------------
// weatherCmd — flag registration (no network)
// ---------------------------------------------------------------------------

func TestWeatherCmd_FlagsExistV14(t *testing.T) {
	cmd := weatherCmd()
	for _, name := range []string{"from", "to"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on weatherCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// groundCmd — flag registration
// ---------------------------------------------------------------------------

func TestGroundCmd_FlagsExistV14(t *testing.T) {
	cmd := groundCmd()
	for _, name := range []string{"max-price", "type"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on groundCmd", name)
		}
	}
}

// ---------------------------------------------------------------------------
// loungesCmd — invalid IATA
// ---------------------------------------------------------------------------

func TestLoungesCmd_InvalidIATAV14(t *testing.T) {
	cmd := loungesCmd()
	cmd.SetArgs([]string{"12"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid airport IATA")
	}
}

// ---------------------------------------------------------------------------
// runWatchDaemon — direct call with mock ticker (no network)
// ---------------------------------------------------------------------------

type mockWatchTicker struct {
	ch chan time.Time
}

func (m *mockWatchTicker) Chan() <-chan time.Time { return m.ch }
func (m *mockWatchTicker) Stop()                 {}

func TestRunWatchDaemon_InvalidIntervalV14(t *testing.T) {
	ctx := context.Background()
	err := runWatchDaemon(ctx, &bytes.Buffer{}, 0, false, func(context.Context) (int, error) {
		return 0, nil
	}, func(d time.Duration) watchDaemonTicker {
		return &mockWatchTicker{ch: make(chan time.Time)}
	})
	if err == nil {
		t.Error("expected error for zero interval")
	}
}

func TestRunWatchDaemon_CancelledContextV14(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled — daemon loop exits immediately

	mt := &mockWatchTicker{ch: make(chan time.Time)}
	err := runWatchDaemon(ctx, &bytes.Buffer{}, time.Minute, false, func(context.Context) (int, error) {
		return 0, nil
	}, func(time.Duration) watchDaemonTicker { return mt })
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunWatchDaemon_RunNow_EmptyStoreV14(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel so the loop exits right after runNow

	mt := &mockWatchTicker{ch: make(chan time.Time)}
	var buf bytes.Buffer
	err := runWatchDaemon(ctx, &buf, time.Minute, true, func(context.Context) (int, error) {
		return 0, nil
	}, func(time.Duration) watchDaemonTicker { return mt })
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// tripsAlertsCmd — JSON format branch via global format var
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// upgradeCmd — flags exist
// ---------------------------------------------------------------------------

func TestUpgradeCmd_FlagsExistV14(t *testing.T) {
	cmd := upgradeCmd()
	for _, name := range []string{"dry-run", "quiet"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on upgradeCmd", name)
		}
	}
}
