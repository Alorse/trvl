package trips

import (
	"context"
	"testing"
	"time"
)

// ============================================================
// within
// ============================================================

func TestWithin(t *testing.T) {
	tests := []struct {
		name      string
		d         time.Duration
		target    time.Duration
		tolerance time.Duration
		want      bool
	}{
		{
			"exact match",
			24 * time.Hour,
			24 * time.Hour,
			30 * time.Minute,
			true,
		},
		{
			"within tolerance low",
			23*time.Hour + 31*time.Minute,
			24 * time.Hour,
			30 * time.Minute,
			true,
		},
		{
			"within tolerance high",
			24*time.Hour + 29*time.Minute,
			24 * time.Hour,
			30 * time.Minute,
			true,
		},
		{
			"below tolerance",
			23 * time.Hour,
			24 * time.Hour,
			30 * time.Minute,
			false,
		},
		{
			"above tolerance",
			25 * time.Hour,
			24 * time.Hour,
			30 * time.Minute,
			false,
		},
		{
			"at exact lower boundary",
			23*time.Hour + 30*time.Minute,
			24 * time.Hour,
			30 * time.Minute,
			true,
		},
		{
			"at exact upper boundary",
			24*time.Hour + 30*time.Minute,
			24 * time.Hour,
			30 * time.Minute,
			true,
		},
		{
			"zero tolerance exact match",
			6 * time.Hour,
			6 * time.Hour,
			0,
			true,
		},
		{
			"zero tolerance miss",
			6*time.Hour + 1*time.Second,
			6 * time.Hour,
			0,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := within(tt.d, tt.target, tt.tolerance); got != tt.want {
				t.Errorf("within(%v, %v, %v) = %v, want %v", tt.d, tt.target, tt.tolerance, got, tt.want)
			}
		})
	}
}

// ============================================================
// checkReminders
// ============================================================

func TestCheckReminders_24h(t *testing.T) {
	now := time.Now()
	start := now.Add(24 * time.Hour)

	trip := Trip{
		ID:     "trip_test1",
		Name:   "24h test",
		Status: "booked",
		Legs: []TripLeg{{
			Type:      "flight",
			From:      "HEL",
			To:        "AMS",
			Provider:  "KLM",
			StartTime: start.Format("2006-01-02T15:04:05"),
		}},
	}

	alerts := checkReminders(trip, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Type != "reminder" {
		t.Errorf("type = %q, want reminder", alerts[0].Type)
	}
	if alerts[0].TripID != "trip_test1" {
		t.Errorf("TripID = %q", alerts[0].TripID)
	}
}

func TestCheckReminders_6h(t *testing.T) {
	now := time.Now()
	start := now.Add(6 * time.Hour)

	trip := Trip{
		ID:     "trip_test2",
		Name:   "6h test",
		Status: "booked",
		Legs: []TripLeg{{
			Type:      "train",
			From:      "Prague",
			To:        "Vienna",
			Provider:  "CD",
			StartTime: start.Format("2006-01-02T15:04:05"),
		}},
	}

	alerts := checkReminders(trip, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].TripName != "6h test" {
		t.Errorf("TripName = %q", alerts[0].TripName)
	}
}

func TestCheckReminders_2h(t *testing.T) {
	now := time.Now()
	start := now.Add(2 * time.Hour)

	trip := Trip{
		ID:     "trip_test3",
		Name:   "2h test",
		Status: "booked",
		Legs: []TripLeg{{
			Type:      "flight",
			From:      "AMS",
			To:        "PRG",
			Provider:  "KLM",
			StartTime: start.Format("2006-01-02T15:04:05"),
		}},
	}

	alerts := checkReminders(trip, now)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestCheckReminders_NoAlert(t *testing.T) {
	now := time.Now()
	// 48 hours out - no alert should fire.
	start := now.Add(48 * time.Hour)

	trip := Trip{
		ID:     "trip_test4",
		Name:   "far out",
		Status: "booked",
		Legs: []TripLeg{{
			Type:      "flight",
			From:      "HEL",
			To:        "BCN",
			Provider:  "Finnair",
			StartTime: start.Format("2006-01-02T15:04:05"),
		}},
	}

	alerts := checkReminders(trip, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for far-out leg, got %d", len(alerts))
	}
}

func TestCheckReminders_PastLeg(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)

	trip := Trip{
		ID:     "trip_past",
		Name:   "past",
		Status: "booked",
		Legs: []TripLeg{{
			Type:      "flight",
			From:      "A",
			To:        "B",
			StartTime: past.Format("2006-01-02T15:04:05"),
		}},
	}

	alerts := checkReminders(trip, now)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for past leg, got %d", len(alerts))
	}
}

func TestCheckReminders_EmptyStartTime(t *testing.T) {
	trip := Trip{
		ID:   "trip_empty",
		Name: "empty",
		Legs: []TripLeg{{Type: "flight", StartTime: ""}},
	}
	alerts := checkReminders(trip, time.Now())
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for empty start time, got %d", len(alerts))
	}
}

func TestCheckReminders_InvalidStartTime(t *testing.T) {
	trip := Trip{
		ID:   "trip_bad",
		Name: "bad",
		Legs: []TripLeg{{Type: "flight", StartTime: "not-a-date"}},
	}
	alerts := checkReminders(trip, time.Now())
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for invalid start time, got %d", len(alerts))
	}
}

func TestCheckReminders_MultipleLegs(t *testing.T) {
	now := time.Now()
	// One leg at 24h, one at 6h, one at 48h.
	trip := Trip{
		ID:     "trip_multi",
		Name:   "multi",
		Status: "booked",
		Legs: []TripLeg{
			{Type: "flight", From: "A", To: "B", Provider: "X", StartTime: now.Add(24 * time.Hour).Format("2006-01-02T15:04:05")},
			{Type: "train", From: "B", To: "C", Provider: "Y", StartTime: now.Add(6 * time.Hour).Format("2006-01-02T15:04:05")},
			{Type: "bus", From: "C", To: "D", Provider: "Z", StartTime: now.Add(48 * time.Hour).Format("2006-01-02T15:04:05")},
		},
	}

	alerts := checkReminders(trip, now)
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts (24h + 6h), got %d", len(alerts))
	}
}

// ============================================================
// Monitor
// ============================================================

func TestMonitor_FiltersInactiveTrips(t *testing.T) {
	now := time.Now()
	start := now.Add(24 * time.Hour)

	trips := []Trip{
		{
			ID:     "active",
			Name:   "Active Trip",
			Status: "booked",
			Legs: []TripLeg{{
				Type:      "flight",
				From:      "A",
				To:        "B",
				Provider:  "X",
				StartTime: start.Format("2006-01-02T15:04:05"),
			}},
		},
		{
			ID:     "inactive",
			Name:   "Completed Trip",
			Status: "completed",
			Legs: []TripLeg{{
				Type:      "flight",
				From:      "C",
				To:        "D",
				Provider:  "Y",
				StartTime: start.Format("2006-01-02T15:04:05"),
			}},
		},
		{
			ID:     "cancelled",
			Name:   "Cancelled Trip",
			Status: "cancelled",
			Legs: []TripLeg{{
				Type:      "flight",
				From:      "E",
				To:        "F",
				Provider:  "Z",
				StartTime: start.Format("2006-01-02T15:04:05"),
			}},
		},
	}

	alerts := Monitor(context.Background(), trips)
	// Only the active trip should produce an alert.
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert from active trip, got %d", len(alerts))
	}
	if alerts[0].TripID != "active" {
		t.Errorf("alert TripID = %q, want active", alerts[0].TripID)
	}
}

func TestMonitor_EmptyTrips(t *testing.T) {
	alerts := Monitor(context.Background(), nil)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for nil trips, got %d", len(alerts))
	}
}

func TestMonitor_PlanningTrips(t *testing.T) {
	now := time.Now()
	start := now.Add(24 * time.Hour)

	trips := []Trip{
		{
			ID:     "planning",
			Name:   "Planning Trip",
			Status: "planning",
			Legs: []TripLeg{{
				Type:      "flight",
				From:      "A",
				To:        "B",
				Provider:  "X",
				StartTime: start.Format("2006-01-02T15:04:05"),
			}},
		},
	}

	alerts := Monitor(context.Background(), trips)
	// Planning is an active status.
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert from planning trip, got %d", len(alerts))
	}
}

// ============================================================
// ValidStatuses
// ============================================================

func TestValidStatuses(t *testing.T) {
	expected := []string{"planning", "booked", "in_progress", "completed", "cancelled"}
	for _, s := range expected {
		if !ValidStatuses[s] {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if ValidStatuses["unknown"] {
		t.Error("unknown should not be valid")
	}
}

// ============================================================
// activeStatuses
// ============================================================

func TestActiveStatuses(t *testing.T) {
	active := []string{"planning", "booked", "in_progress"}
	for _, s := range active {
		if !activeStatuses[s] {
			t.Errorf("expected %q to be active", s)
		}
	}
	inactive := []string{"completed", "cancelled"}
	for _, s := range inactive {
		if activeStatuses[s] {
			t.Errorf("expected %q to be inactive", s)
		}
	}
}
