package trip

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestTripCostInput_Defaults(t *testing.T) {
	in := TripCostInput{}
	in.defaults()

	if in.Guests != 1 {
		t.Errorf("Guests = %d, want 1", in.Guests)
	}
	if in.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", in.Currency)
	}
}

func TestTripCostInput_DefaultsPreserve(t *testing.T) {
	in := TripCostInput{Guests: 3, Currency: "USD"}
	in.defaults()

	if in.Guests != 3 {
		t.Errorf("Guests = %d, want 3", in.Guests)
	}
	if in.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", in.Currency)
	}
}

func TestCalculateTripCost_MissingOrigin(t *testing.T) {
	_, err := CalculateTripCost(t.Context(), TripCostInput{
		Destination: "BCN",
		DepartDate:  "2026-07-01",
		ReturnDate:  "2026-07-08",
	})
	if err == nil {
		t.Error("expected error for missing origin")
	}
}

func TestCalculateTripCost_MissingDates(t *testing.T) {
	_, err := CalculateTripCost(t.Context(), TripCostInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if err == nil {
		t.Error("expected error for missing dates")
	}
}

func TestCalculateTripCost_InvalidDates(t *testing.T) {
	_, err := CalculateTripCost(t.Context(), TripCostInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "not-a-date",
		ReturnDate:  "2026-07-08",
	})
	if err == nil {
		t.Error("expected error for invalid depart date")
	}
}

func TestCalculateTripCost_ReturnBeforeDepart(t *testing.T) {
	_, err := CalculateTripCost(t.Context(), TripCostInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-08",
		ReturnDate:  "2026-07-01",
	})
	if err == nil {
		t.Error("expected error for return before depart")
	}
}

func TestCalculateTripCost_SameDay(t *testing.T) {
	_, err := CalculateTripCost(t.Context(), TripCostInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-01",
		ReturnDate:  "2026-07-01",
	})
	if err == nil {
		t.Error("expected error for same-day trip")
	}
}

func TestCheapestFlight(t *testing.T) {
	flts := []models.FlightResult{
		{Price: 300, Currency: "EUR", Stops: 1},
		{Price: 150, Currency: "EUR", Stops: 0},
		{Price: 450, Currency: "EUR", Stops: 2},
	}
	best := cheapestFlight(flts)
	if best.Price != 150 {
		t.Errorf("cheapestFlight = %v, want 150", best.Price)
	}
}

func TestCheapestFlight_SkipsZero(t *testing.T) {
	flts := []models.FlightResult{
		{Price: 0, Currency: "EUR"},
		{Price: 200, Currency: "EUR"},
	}
	best := cheapestFlight(flts)
	if best.Price != 200 {
		t.Errorf("cheapestFlight = %v, want 200", best.Price)
	}
}

func TestCheapestHotel(t *testing.T) {
	htls := []models.HotelResult{
		{Price: 120, Currency: "EUR", Name: "Hotel A"},
		{Price: 80, Currency: "EUR", Name: "Hotel B"},
		{Price: 200, Currency: "EUR", Name: "Hotel C"},
	}
	best := cheapestHotel(htls)
	if best.Price != 80 {
		t.Errorf("cheapestHotel = %v, want 80", best.Price)
	}
	if best.Name != "Hotel B" {
		t.Errorf("cheapestHotel name = %q, want Hotel B", best.Name)
	}
}

func TestCheapestHotel_SkipsZero(t *testing.T) {
	htls := []models.HotelResult{
		{Price: 0, Currency: "EUR"},
		{Price: 100, Currency: "EUR", Name: "Hotel A"},
	}
	best := cheapestHotel(htls)
	if best.Price != 100 {
		t.Errorf("cheapestHotel = %v, want 100", best.Price)
	}
}
