package trip

import "testing"

func TestPlanTrip_RejectsNonPositiveGuests(t *testing.T) {
	for _, guests := range []int{0, -2} {
		t.Run("guests", func(t *testing.T) {
			_, err := PlanTrip(t.Context(), PlanInput{
				Origin:      "HEL",
				Destination: "BCN",
				DepartDate:  "2026-07-01",
				ReturnDate:  "2026-07-08",
				Guests:      guests,
			})
			if err == nil {
				t.Fatal("expected error for nonpositive guests")
			}
			if got := err.Error(); got != "guests must be at least 1" {
				t.Fatalf("error = %q, want %q", got, "guests must be at least 1")
			}
		})
	}
}
