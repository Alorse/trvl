package trip

import "testing"

// ============================================================
// PlanTrip validation paths
// ============================================================

func TestPlanTrip_MissingOrigin(t *testing.T) {
	_, err := PlanTrip(t.Context(), PlanInput{
		Destination: "BCN",
		DepartDate:  "2026-07-01",
		ReturnDate:  "2026-07-08",
		Guests:      2,
	})
	if err == nil {
		t.Error("expected error for missing origin")
	}
}

func TestPlanTrip_MissingDestination(t *testing.T) {
	_, err := PlanTrip(t.Context(), PlanInput{
		Origin:     "HEL",
		DepartDate: "2026-07-01",
		ReturnDate: "2026-07-08",
		Guests:     2,
	})
	if err == nil {
		t.Error("expected error for missing destination")
	}
}

func TestPlanTrip_MissingDates(t *testing.T) {
	_, err := PlanTrip(t.Context(), PlanInput{
		Origin:      "HEL",
		Destination: "BCN",
		Guests:      2,
	})
	if err == nil {
		t.Error("expected error for missing dates")
	}
}

func TestPlanTrip_InvalidDepartDate(t *testing.T) {
	_, err := PlanTrip(t.Context(), PlanInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "not-a-date",
		ReturnDate:  "2026-07-08",
		Guests:      2,
	})
	if err == nil {
		t.Error("expected error for invalid depart date")
	}
}

func TestPlanTrip_InvalidReturnDate(t *testing.T) {
	_, err := PlanTrip(t.Context(), PlanInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-01",
		ReturnDate:  "not-a-date",
		Guests:      2,
	})
	if err == nil {
		t.Error("expected error for invalid return date")
	}
}

func TestPlanTrip_ReturnBeforeDepart(t *testing.T) {
	_, err := PlanTrip(t.Context(), PlanInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-08",
		ReturnDate:  "2026-07-01",
		Guests:      2,
	})
	if err == nil {
		t.Error("expected error when return is before depart")
	}
}

func TestPlanTrip_SameDay(t *testing.T) {
	_, err := PlanTrip(t.Context(), PlanInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-01",
		ReturnDate:  "2026-07-01",
		Guests:      2,
	})
	if err == nil {
		t.Error("expected error for same-day trip")
	}
}
