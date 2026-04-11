package main

import (
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/watch"
)

func TestFormatWatchDates_SpecificDate(t *testing.T) {
	w := watch.Watch{DepartDate: "2026-07-01", ReturnDate: "2026-07-08"}
	got := formatWatchDates(w)
	if got != "2026-07-01 / 2026-07-08" {
		t.Errorf("got %q, want %q", got, "2026-07-01 / 2026-07-08")
	}
}

func TestFormatWatchDates_SpecificDateOneWay(t *testing.T) {
	w := watch.Watch{DepartDate: "2026-07-01"}
	got := formatWatchDates(w)
	if got != "2026-07-01" {
		t.Errorf("got %q, want %q", got, "2026-07-01")
	}
}

func TestFormatWatchDates_DateRange(t *testing.T) {
	w := watch.Watch{DepartFrom: "2026-07-01", DepartTo: "2026-08-31"}
	got := formatWatchDates(w)
	if got != "2026-07-01 .. 2026-08-31" {
		t.Errorf("got %q, want %q", got, "2026-07-01 .. 2026-08-31")
	}
}

func TestFormatWatchDates_DateRangeWithCheapest(t *testing.T) {
	w := watch.Watch{DepartFrom: "2026-07-01", DepartTo: "2026-08-31", CheapestDate: "2026-07-15"}
	got := formatWatchDates(w)
	want := "2026-07-01 .. 2026-08-31 (best: 2026-07-15)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatWatchDates_RouteWatch(t *testing.T) {
	w := watch.Watch{} // no dates at all
	got := formatWatchDates(w)
	if got != "any (next 60d)" {
		t.Errorf("got %q, want %q", got, "any (next 60d)")
	}
}

func TestFormatWatchDates_RouteWatchWithCheapest(t *testing.T) {
	w := watch.Watch{CheapestDate: "2026-05-10"}
	got := formatWatchDates(w)
	if got != "any (best: 2026-05-10)" {
		t.Errorf("got %q, want %q", got, "any (best: 2026-05-10)")
	}
}

func TestFormatLastCheck_Never(t *testing.T) {
	if got := formatLastCheck(time.Time{}); got != "never" {
		t.Errorf("zero time: got %q, want %q", got, "never")
	}
}

func TestFormatLastCheck_JustNow(t *testing.T) {
	if got := formatLastCheck(time.Now().Add(-10 * time.Second)); got != "just now" {
		t.Errorf("10s ago: got %q, want %q", got, "just now")
	}
}

func TestFormatLastCheck_Minutes(t *testing.T) {
	got := formatLastCheck(time.Now().Add(-15 * time.Minute))
	if got != "15m ago" {
		t.Errorf("15m ago: got %q, want %q", got, "15m ago")
	}
}

func TestFormatLastCheck_Hours(t *testing.T) {
	got := formatLastCheck(time.Now().Add(-3 * time.Hour))
	if got != "3h ago" {
		t.Errorf("3h ago: got %q, want %q", got, "3h ago")
	}
}

func TestFormatLastCheck_Days(t *testing.T) {
	got := formatLastCheck(time.Now().Add(-48 * time.Hour))
	if got != "2d ago" {
		t.Errorf("48h ago: got %q, want %q", got, "2d ago")
	}
}
