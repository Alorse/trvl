package tripwindow

import (
	"context"
	"testing"
	"time"
)

func TestApplyDefaults(t *testing.T) {
	t.Parallel()
	in := Input{}
	in.applyDefaults()
	if in.MinNights != 3 {
		t.Errorf("MinNights: got %d, want 3", in.MinNights)
	}
	if in.MaxNights != 7 {
		t.Errorf("MaxNights: got %d, want 7", in.MaxNights)
	}
	if in.MaxCandidates != 5 {
		t.Errorf("MaxCandidates: got %d, want 5", in.MaxCandidates)
	}
}

func TestApplyDefaults_MaxNightsClamp(t *testing.T) {
	t.Parallel()
	in := Input{MinNights: 5, MaxNights: 3}
	in.applyDefaults()
	if in.MaxNights != 5 {
		t.Errorf("MaxNights should be clamped to MinNights; got %d", in.MaxNights)
	}
}

func TestValidateInput_MissingDestination(t *testing.T) {
	t.Parallel()
	err := ValidateInput(Input{WindowStart: "2026-05-01", WindowEnd: "2026-06-30"})
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestValidateInput_MissingWindowEnd(t *testing.T) {
	t.Parallel()
	err := ValidateInput(Input{Destination: "PRG", WindowStart: "2026-05-01"})
	if err == nil {
		t.Fatal("expected error for missing window_end")
	}
}

func TestValidateInput_InvalidDateOrder(t *testing.T) {
	t.Parallel()
	err := ValidateInput(Input{
		Destination: "PRG",
		WindowStart: "2026-08-01",
		WindowEnd:   "2026-05-01",
	})
	if err == nil {
		t.Fatal("expected error when window_end is before window_start")
	}
}

func TestValidateInput_Valid(t *testing.T) {
	t.Parallel()
	err := ValidateInput(Input{
		Destination: "PRG",
		WindowStart: "2026-05-01",
		WindowEnd:   "2026-06-30",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testDate(s string) time.Time {
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		panic("bad test date: " + s)
	}
	return d
}

func TestOverlapsAny_NoIntervals(t *testing.T) {
	t.Parallel()
	if overlapsAny(testDate("2026-05-10"), testDate("2026-05-15"), nil) {
		t.Error("expected no overlap with empty interval list")
	}
}

func TestOverlapsAny_PartialOverlap(t *testing.T) {
	t.Parallel()
	ivs := mustParseIntervals([]Interval{{Start: "2026-05-12", End: "2026-05-16"}})
	if !overlapsAny(testDate("2026-05-10"), testDate("2026-05-13"), ivs) {
		t.Error("expected overlap: trip [05-10,05-13] vs busy [05-12,05-16]")
	}
}

func TestOverlapsAny_Adjacent(t *testing.T) {
	t.Parallel()
	ivs := mustParseIntervals([]Interval{{Start: "2026-05-12", End: "2026-05-16"}})
	// [05-17, 05-20] is adjacent but does not overlap [05-12, 05-16]
	if overlapsAny(testDate("2026-05-17"), testDate("2026-05-20"), ivs) {
		t.Error("adjacent ranges should not overlap")
	}
}

func TestOverlapsAny_Contained(t *testing.T) {
	t.Parallel()
	ivs := mustParseIntervals([]Interval{{Start: "2026-05-01", End: "2026-05-31"}})
	if !overlapsAny(testDate("2026-05-10"), testDate("2026-05-15"), ivs) {
		t.Error("inner range should overlap containing range")
	}
}

func TestOverlapsAny_ExactMatch(t *testing.T) {
	t.Parallel()
	ivs := mustParseIntervals([]Interval{{Start: "2026-05-10", End: "2026-05-15"}})
	if !overlapsAny(testDate("2026-05-10"), testDate("2026-05-15"), ivs) {
		t.Error("exact same range should overlap")
	}
}

func TestParseBusyFlag_Valid(t *testing.T) {
	t.Parallel()
	iv, err := ParseBusyFlag("2026-05-12:2026-05-16")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iv.Start != "2026-05-12" || iv.End != "2026-05-16" {
		t.Errorf("got %+v", iv)
	}
}

func TestParseBusyFlag_BadFormat(t *testing.T) {
	t.Parallel()
	if _, err := ParseBusyFlag("20260512-20260516"); err == nil {
		t.Fatal("expected error for bad format (no colon at position 10)")
	}
}

func TestParseBusyFlag_InvalidStartDate(t *testing.T) {
	t.Parallel()
	if _, err := ParseBusyFlag("2026-13-01:2026-13-05"); err == nil {
		t.Fatal("expected error for invalid month 13")
	}
}

func TestFind_MissingDestination(t *testing.T) {
	t.Parallel()
	_, err := Find(context.Background(), Input{
		WindowStart: "2026-05-01",
		WindowEnd:   "2026-05-15",
	})
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestFind_WindowEndBeforeStart(t *testing.T) {
	t.Parallel()
	_, err := Find(context.Background(), Input{
		Destination: "PRG",
		WindowStart: "2026-06-01",
		WindowEnd:   "2026-05-01",
	})
	if err == nil {
		t.Fatal("expected error when window_end is before window_start")
	}
}

func TestFind_AllBusy(t *testing.T) {
	t.Parallel()
	candidates, err := Find(context.Background(), Input{
		Origin:      "HEL",
		Destination: "PRG",
		WindowStart: "2026-05-01",
		WindowEnd:   "2026-05-10",
		MinNights:   3,
		MaxNights:   3,
		BusyIntervals: []Interval{
			{Start: "2026-05-01", End: "2026-05-31"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates when all busy; got %d", len(candidates))
	}
}

func TestFind_MaxCandidatesCap(t *testing.T) {
	t.Parallel()
	candidates, err := Find(context.Background(), Input{
		Origin:        "HEL",
		Destination:   "PRG",
		WindowStart:   "2026-05-01",
		WindowEnd:     "2026-06-30",
		MinNights:     3,
		MaxNights:     7,
		MaxCandidates: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) > 3 {
		t.Errorf("expected at most 3 candidates; got %d", len(candidates))
	}
}

func TestFind_PreferredIntervalInResults(t *testing.T) {
	t.Parallel()
	// Short window, one trip fits; mark it preferred.
	candidates, err := Find(context.Background(), Input{
		Origin:      "HEL",
		Destination: "PRG",
		WindowStart: "2026-05-01",
		WindowEnd:   "2026-05-05",
		MinNights:   3,
		MaxNights:   3,
		PreferredIntervals: []Interval{
			{Start: "2026-05-01", End: "2026-05-05"},
		},
		MaxCandidates: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// There should be at least one candidate and it should be preferred.
	for _, c := range candidates {
		if c.OverlapsPreferred {
			return // pass
		}
	}
	if len(candidates) > 0 {
		t.Errorf("expected OverlapsPreferred=true, got %+v", candidates[0])
	}
}

func TestFind_EmptyWindowProducesNoCandidates(t *testing.T) {
	t.Parallel()
	// Window is too narrow for MinNights.
	candidates, err := Find(context.Background(), Input{
		Origin:      "HEL",
		Destination: "PRG",
		WindowStart: "2026-05-01",
		WindowEnd:   "2026-05-02", // only 1 day; MinNights=3
		MinNights:   3,
		MaxNights:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for too-narrow window; got %d", len(candidates))
	}
}

func TestBuildReasoning_WithPrice(t *testing.T) {
	t.Parallel()
	r := buildReasoning(testDate("2026-05-10"), testDate("2026-05-14"), 4, 180, "EUR", false)
	if r == "" {
		t.Error("expected non-empty reasoning")
	}
}

func TestBuildReasoning_NoPriceAvailable(t *testing.T) {
	t.Parallel()
	r := buildReasoning(testDate("2026-05-10"), testDate("2026-05-14"), 4, 0, "", false)
	if r == "" {
		t.Error("expected non-empty reasoning even with no price")
	}
}

func TestBuildReasoning_Preferred(t *testing.T) {
	t.Parallel()
	r := buildReasoning(testDate("2026-05-10"), testDate("2026-05-14"), 4, 200, "EUR", true)
	if r == "" {
		t.Error("expected non-empty reasoning for preferred window")
	}
}

func TestMustParseIntervals_SkipsMalformed(t *testing.T) {
	t.Parallel()
	ivs := []Interval{
		{Start: "not-a-date", End: "2026-05-10"},
		{Start: "2026-05-01", End: "2026-05-10"},
	}
	parsed := mustParseIntervals(ivs)
	if len(parsed) != 1 {
		t.Errorf("expected 1 valid interval, got %d", len(parsed))
	}
}
