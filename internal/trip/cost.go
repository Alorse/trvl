// Package trip implements trip cost estimation by combining flight and hotel searches.
package trip

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// TripCostInput configures a trip cost calculation.
type TripCostInput struct {
	Origin      string
	Destination string
	DepartDate  string
	ReturnDate  string
	Guests      int
	Currency    string // target currency for total
}

// FlightCost holds flight price details.
type FlightCost struct {
	Outbound      float64 `json:"outbound"`
	Return        float64 `json:"return"`
	Currency      string  `json:"currency"`
	OutboundStops int     `json:"outbound_stops"`
	ReturnStops   int     `json:"return_stops"`
}

// HotelCost holds hotel price details.
type HotelCost struct {
	PerNight float64 `json:"per_night"`
	Total    float64 `json:"total"`
	Currency string  `json:"currency"`
	Name     string  `json:"name,omitempty"`
}

// TripCostResult is the top-level response for a trip cost calculation.
type TripCostResult struct {
	Success   bool       `json:"success"`
	Flights   FlightCost `json:"flights"`
	Hotels    HotelCost  `json:"hotels"`
	Total     float64    `json:"total"`
	Currency  string     `json:"currency"`
	PerPerson float64    `json:"per_person"`
	PerDay    float64    `json:"per_day"`
	Nights    int        `json:"nights"`
	Error     string     `json:"error,omitempty"`
}

// defaults fills in zero-value fields with sensible defaults.
func (in *TripCostInput) defaults() {
	if in.Guests <= 0 {
		in.Guests = 1
	}
	if in.Currency == "" {
		in.Currency = "EUR"
	}
}

// CalculateTripCost estimates the total cost of a trip by searching for the
// cheapest outbound flight, return flight, and hotel at the destination.
//
// Flights are priced per person; hotels are per room (not multiplied by guests).
// Total = (outbound + return) * guests + hotel_per_night * nights.
func CalculateTripCost(ctx context.Context, input TripCostInput) (*TripCostResult, error) {
	input.defaults()

	if input.Origin == "" || input.Destination == "" {
		return nil, fmt.Errorf("origin and destination are required")
	}
	if input.DepartDate == "" || input.ReturnDate == "" {
		return nil, fmt.Errorf("depart_date and return_date are required")
	}

	departDate, err := time.Parse("2006-01-02", input.DepartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid depart_date %q: %w", input.DepartDate, err)
	}
	returnDate, err := time.Parse("2006-01-02", input.ReturnDate)
	if err != nil {
		return nil, fmt.Errorf("invalid return_date %q: %w", input.ReturnDate, err)
	}
	if !returnDate.After(departDate) {
		return nil, fmt.Errorf("return_date must be after depart_date")
	}

	nights := int(math.Round(returnDate.Sub(departDate).Hours() / 24))
	if nights <= 0 {
		return nil, fmt.Errorf("trip must be at least 1 night")
	}

	result := &TripCostResult{
		Currency: input.Currency,
		Nights:   nights,
	}

	// Search outbound flights.
	outResult, outErr := flights.SearchFlights(ctx, input.Origin, input.Destination, input.DepartDate, flights.SearchOptions{
		SortBy: models.SortCheapest,
		Adults: 1,
	})

	// Search return flights.
	retResult, retErr := flights.SearchFlights(ctx, input.Destination, input.Origin, input.ReturnDate, flights.SearchOptions{
		SortBy: models.SortCheapest,
		Adults: 1,
	})

	// Search hotels.
	hotelResult, hotelErr := hotels.SearchHotels(ctx, input.Destination, hotels.HotelSearchOptions{
		CheckIn:  input.DepartDate,
		CheckOut: input.ReturnDate,
		Guests:   input.Guests,
		Sort:     "cheapest",
		Currency: input.Currency,
	})

	assembleTripCost(result, outResult, outErr, retResult, retErr, hotelResult, hotelErr, nights, input.Guests)

	return result, nil
}

// assembleTripCost populates the TripCostResult from search results.
// It extracts cheapest flights/hotels, aggregates costs, and computes per-person/per-day.
func assembleTripCost(result *TripCostResult, outResult *models.FlightSearchResult, outErr error, retResult *models.FlightSearchResult, retErr error, hotelResult *models.HotelSearchResult, hotelErr error, nights, guests int) {
	// Extract cheapest outbound flight.
	if outErr == nil && outResult != nil && outResult.Success && len(outResult.Flights) > 0 {
		cheapest := cheapestFlight(outResult.Flights)
		result.Flights.Outbound = cheapest.Price
		result.Flights.Currency = cheapest.Currency
		result.Flights.OutboundStops = cheapest.Stops
	}

	// Extract cheapest return flight.
	if retErr == nil && retResult != nil && retResult.Success && len(retResult.Flights) > 0 {
		cheapest := cheapestFlight(retResult.Flights)
		result.Flights.Return = cheapest.Price
		if result.Flights.Currency == "" {
			result.Flights.Currency = cheapest.Currency
		}
		result.Flights.ReturnStops = cheapest.Stops
	}

	// Extract cheapest hotel.
	if hotelErr == nil && hotelResult != nil && hotelResult.Success && len(hotelResult.Hotels) > 0 {
		cheapest := cheapestHotel(hotelResult.Hotels)
		result.Hotels.PerNight = cheapest.Price
		result.Hotels.Total = cheapest.Price * float64(nights)
		result.Hotels.Currency = cheapest.Currency
		result.Hotels.Name = cheapest.Name
	}

	// Collect errors.
	var errors []string
	if outErr != nil {
		errors = append(errors, fmt.Sprintf("outbound flights: %v", outErr))
	}
	if retErr != nil {
		errors = append(errors, fmt.Sprintf("return flights: %v", retErr))
	}
	if hotelErr != nil {
		errors = append(errors, fmt.Sprintf("hotels: %v", hotelErr))
	}

	// Calculate total.
	// Flights are per person, hotels are per room.
	flightPerPerson := result.Flights.Outbound + result.Flights.Return
	flightTotal := flightPerPerson * float64(guests)
	result.Total = flightTotal + result.Hotels.Total

	if result.Total > 0 {
		result.Success = true
		result.PerPerson = result.Total / float64(guests)
		if nights > 0 {
			result.PerDay = result.Total / float64(nights)
		}
	}

	if !result.Success && len(errors) > 0 {
		result.Error = fmt.Sprintf("partial failure: %s", errors[0])
	}
}

// cheapestFlight returns the flight with the lowest positive price.
func cheapestFlight(flts []models.FlightResult) models.FlightResult {
	best := flts[0]
	for _, f := range flts[1:] {
		if f.Price > 0 && (best.Price <= 0 || f.Price < best.Price) {
			best = f
		}
	}
	return best
}

// cheapestHotel returns the hotel with the lowest positive price.
func cheapestHotel(htls []models.HotelResult) models.HotelResult {
	best := htls[0]
	for _, h := range htls[1:] {
		if h.Price > 0 && (best.Price <= 0 || h.Price < best.Price) {
			best = h
		}
	}
	return best
}
