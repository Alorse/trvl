package trip

import (
	"fmt"
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

func TestCheapestFlight_SingleElement(t *testing.T) {
	flts := []models.FlightResult{
		{Price: 250, Currency: "EUR", Stops: 1},
	}
	best := cheapestFlight(flts)
	if best.Price != 250 {
		t.Errorf("cheapestFlight = %v, want 250", best.Price)
	}
}

func TestCheapestFlight_AllZero(t *testing.T) {
	flts := []models.FlightResult{
		{Price: 0, Currency: "EUR"},
		{Price: 0, Currency: "EUR"},
	}
	best := cheapestFlight(flts)
	// When all prices are zero, returns the first element (no positive price found).
	if best.Price != 0 {
		t.Errorf("cheapestFlight = %v, want 0", best.Price)
	}
}

func TestCheapestFlight_NegativePrice(t *testing.T) {
	flts := []models.FlightResult{
		{Price: -10, Currency: "EUR"},
		{Price: 200, Currency: "EUR"},
	}
	best := cheapestFlight(flts)
	if best.Price != 200 {
		t.Errorf("cheapestFlight = %v, want 200 (should skip negative)", best.Price)
	}
}

func TestCheapestFlight_FirstZeroThenMultiple(t *testing.T) {
	flts := []models.FlightResult{
		{Price: 0, Currency: "EUR"},
		{Price: 300, Currency: "EUR"},
		{Price: 100, Currency: "EUR"},
	}
	best := cheapestFlight(flts)
	if best.Price != 100 {
		t.Errorf("cheapestFlight = %v, want 100", best.Price)
	}
}

func TestCheapestHotel_SingleElement(t *testing.T) {
	htls := []models.HotelResult{
		{Price: 80, Currency: "EUR", Name: "Solo Hotel"},
	}
	best := cheapestHotel(htls)
	if best.Price != 80 {
		t.Errorf("cheapestHotel = %v, want 80", best.Price)
	}
	if best.Name != "Solo Hotel" {
		t.Errorf("cheapestHotel name = %q, want Solo Hotel", best.Name)
	}
}

func TestCheapestHotel_AllZero(t *testing.T) {
	htls := []models.HotelResult{
		{Price: 0, Currency: "EUR", Name: "Free A"},
		{Price: 0, Currency: "EUR", Name: "Free B"},
	}
	best := cheapestHotel(htls)
	if best.Price != 0 {
		t.Errorf("cheapestHotel = %v, want 0", best.Price)
	}
}

func TestCalculateTripCost_InvalidReturnDate(t *testing.T) {
	_, err := CalculateTripCost(t.Context(), TripCostInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-01",
		ReturnDate:  "bad-date",
	})
	if err == nil {
		t.Error("expected error for invalid return date")
	}
}

func TestCalculateTripCost_MissingDestination(t *testing.T) {
	_, err := CalculateTripCost(t.Context(), TripCostInput{
		Origin:     "HEL",
		DepartDate: "2026-07-01",
		ReturnDate: "2026-07-08",
	})
	if err == nil {
		t.Error("expected error for missing destination")
	}
}

func TestTripCostInput_DefaultsNegativeGuests(t *testing.T) {
	in := TripCostInput{Guests: -5}
	in.defaults()
	if in.Guests != 1 {
		t.Errorf("Guests = %d, want 1 for negative input", in.Guests)
	}
}

func TestAssembleTripCost_AllSuccess(t *testing.T) {
	result := &TripCostResult{Currency: "EUR", Nights: 3}
	outResult := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Price: 150, Currency: "EUR", Stops: 0},
			{Price: 250, Currency: "EUR", Stops: 1},
		},
	}
	retResult := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Price: 180, Currency: "EUR", Stops: 1},
		},
	}
	hotelResult := &models.HotelSearchResult{
		Success: true,
		Hotels: []models.HotelResult{
			{Price: 80, Currency: "EUR", Name: "Hotel Test"},
		},
	}

	assembleTripCost(result, outResult, nil, retResult, nil, hotelResult, nil, 3, 2)

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Flights.Outbound != 150 {
		t.Errorf("outbound = %v, want 150", result.Flights.Outbound)
	}
	if result.Flights.Return != 180 {
		t.Errorf("return = %v, want 180", result.Flights.Return)
	}
	if result.Flights.OutboundStops != 0 {
		t.Errorf("outbound stops = %v, want 0", result.Flights.OutboundStops)
	}
	if result.Flights.ReturnStops != 1 {
		t.Errorf("return stops = %v, want 1", result.Flights.ReturnStops)
	}
	if result.Hotels.PerNight != 80 {
		t.Errorf("hotel per night = %v, want 80", result.Hotels.PerNight)
	}
	if result.Hotels.Total != 240 {
		t.Errorf("hotel total = %v, want 240", result.Hotels.Total)
	}
	if result.Hotels.Name != "Hotel Test" {
		t.Errorf("hotel name = %q, want Hotel Test", result.Hotels.Name)
	}
	// Total = (150 + 180) * 2 + 80 * 3 = 660 + 240 = 900
	if result.Total != 900 {
		t.Errorf("total = %v, want 900", result.Total)
	}
	// PerPerson = 900 / 2 = 450
	if result.PerPerson != 450 {
		t.Errorf("per person = %v, want 450", result.PerPerson)
	}
	// PerDay = 900 / 3 = 300
	if result.PerDay != 300 {
		t.Errorf("per day = %v, want 300", result.PerDay)
	}
}

func TestAssembleTripCost_FlightsOnlyNoHotel(t *testing.T) {
	result := &TripCostResult{Currency: "EUR", Nights: 2}
	outResult := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{{Price: 100, Currency: "EUR"}},
	}
	retResult := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{{Price: 120, Currency: "EUR"}},
	}

	assembleTripCost(result, outResult, nil, retResult, nil, nil, fmt.Errorf("hotel error"), 2, 1)

	if !result.Success {
		t.Fatal("should succeed with flights only")
	}
	// Total = (100 + 120) * 1 + 0 = 220
	if result.Total != 220 {
		t.Errorf("total = %v, want 220", result.Total)
	}
}

func TestAssembleTripCost_AllErrors(t *testing.T) {
	result := &TripCostResult{Currency: "EUR", Nights: 2}
	outErr := fmt.Errorf("outbound fail")
	retErr := fmt.Errorf("return fail")
	hotelErr := fmt.Errorf("hotel fail")

	assembleTripCost(result, nil, outErr, nil, retErr, nil, hotelErr, 2, 1)

	if result.Success {
		t.Error("expected failure when all searches fail")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestAssembleTripCost_NilResults(t *testing.T) {
	result := &TripCostResult{Currency: "EUR", Nights: 1}
	assembleTripCost(result, nil, nil, nil, nil, nil, nil, 1, 1)

	if result.Success {
		t.Error("expected failure with nil results and no errors")
	}
}

func TestAssembleTripCost_EmptyFlightResults(t *testing.T) {
	result := &TripCostResult{Currency: "EUR", Nights: 2}
	outResult := &models.FlightSearchResult{Success: true, Flights: nil}
	retResult := &models.FlightSearchResult{Success: true, Flights: []models.FlightResult{}}

	assembleTripCost(result, outResult, nil, retResult, nil, nil, nil, 2, 1)

	if result.Success {
		t.Error("expected failure when no flights found")
	}
}

func TestAssembleTripCost_ReturnSetsCurrencyWhenOutboundEmpty(t *testing.T) {
	result := &TripCostResult{Currency: "EUR", Nights: 1}
	// Outbound fails, return succeeds: currency from return should be used for flights.
	retResult := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{{Price: 200, Currency: "USD"}},
	}

	assembleTripCost(result, nil, fmt.Errorf("out fail"), retResult, nil, nil, nil, 1, 1)

	if result.Flights.Currency != "USD" {
		t.Errorf("flights currency = %q, want USD", result.Flights.Currency)
	}
}

func TestAssembleTripCost_SuccessFalse(t *testing.T) {
	result := &TripCostResult{Currency: "EUR", Nights: 1}
	// Unsuccessful search result should be ignored.
	outResult := &models.FlightSearchResult{
		Success: false,
		Flights: []models.FlightResult{{Price: 100, Currency: "EUR"}},
	}

	assembleTripCost(result, outResult, nil, nil, nil, nil, nil, 1, 1)

	if result.Flights.Outbound != 0 {
		t.Errorf("outbound should be 0 when success=false, got %v", result.Flights.Outbound)
	}
}
