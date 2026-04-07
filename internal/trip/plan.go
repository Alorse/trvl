// Package trip implements trip planning by combining flight and hotel searches.
package trip

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// PlanInput configures a trip plan search.
type PlanInput struct {
	Origin      string
	Destination string
	DepartDate  string
	ReturnDate  string
	Guests      int
	Currency    string
}

// PlanFlight is a flight option in the trip plan.
type PlanFlight struct {
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Airline   string  `json:"airline"`
	Flight    string  `json:"flight_number"`
	Stops     int     `json:"stops"`
	Duration  int     `json:"duration_min"`
	Departure string  `json:"departure"`
	Arrival   string  `json:"arrival"`
	Route     string  `json:"route"`
}

// PlanHotel is a hotel option in the trip plan.
type PlanHotel struct {
	Name      string  `json:"name"`
	Rating    float64 `json:"rating"`
	Reviews   int     `json:"reviews"`
	PerNight  float64 `json:"per_night"`
	Total     float64 `json:"total"`
	Currency  string  `json:"currency"`
	Amenities string  `json:"amenities,omitempty"`
}

// PlanSummary shows the cheapest combination.
type PlanSummary struct {
	FlightsTotal float64 `json:"flights_total"`
	HotelTotal   float64 `json:"hotel_total"`
	GrandTotal   float64 `json:"grand_total"`
	PerPerson    float64 `json:"per_person"`
	PerDay       float64 `json:"per_day"`
	Currency     string  `json:"currency"`
}

// PlanResult is the full trip plan response.
type PlanResult struct {
	Success         bool         `json:"success"`
	Origin          string       `json:"origin"`
	Destination     string       `json:"destination"`
	DepartDate      string       `json:"depart_date"`
	ReturnDate      string       `json:"return_date"`
	Nights          int          `json:"nights"`
	Guests          int          `json:"guests"`
	OutboundFlights []PlanFlight `json:"outbound_flights"`
	ReturnFlights   []PlanFlight `json:"return_flights"`
	Hotels          []PlanHotel  `json:"hotels"`
	Summary         PlanSummary  `json:"summary"`
	Error           string       `json:"error,omitempty"`
}

// PlanTrip searches flights and hotels in parallel and returns the top options
// along with a cheapest-combination summary.
func PlanTrip(ctx context.Context, input PlanInput) (*PlanResult, error) {
	if input.Origin == "" || input.Destination == "" {
		return nil, fmt.Errorf("origin and destination are required")
	}
	if input.DepartDate == "" || input.ReturnDate == "" {
		return nil, fmt.Errorf("depart and return dates are required")
	}
	if input.Guests <= 0 {
		return nil, fmt.Errorf("guests must be at least 1")
	}

	departDate, err := time.Parse("2006-01-02", input.DepartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid depart date %q: %w", input.DepartDate, err)
	}
	returnDate, err := time.Parse("2006-01-02", input.ReturnDate)
	if err != nil {
		return nil, fmt.Errorf("invalid return date %q: %w", input.ReturnDate, err)
	}
	if !returnDate.After(departDate) {
		return nil, fmt.Errorf("return date must be after depart date")
	}

	nights := int(math.Round(returnDate.Sub(departDate).Hours() / 24))

	result := &PlanResult{
		Origin:      input.Origin,
		Destination: input.Destination,
		DepartDate:  input.DepartDate,
		ReturnDate:  input.ReturnDate,
		Nights:      nights,
		Guests:      input.Guests,
	}

	// Search all three in parallel.
	var (
		outResult   *models.FlightSearchResult
		retResult   *models.FlightSearchResult
		hotelResult *models.HotelSearchResult
		outErr      error
		retErr      error
		hotelErr    error
		wg          sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		outResult, outErr = flights.SearchFlights(ctx, input.Origin, input.Destination, input.DepartDate, flights.SearchOptions{
			SortBy: models.SortCheapest,
			Adults: input.Guests,
		})
	}()
	go func() {
		defer wg.Done()
		retResult, retErr = flights.SearchFlights(ctx, input.Destination, input.Origin, input.ReturnDate, flights.SearchOptions{
			SortBy: models.SortCheapest,
			Adults: input.Guests,
		})
	}()
	go func() {
		defer wg.Done()
		// Hotels need city name, not IATA code (Google needs "Prague" not "PRG").
		hotelLocation := models.ResolveLocationName(input.Destination)
		hotelResult, hotelErr = hotels.SearchHotels(ctx, hotelLocation, hotels.HotelSearchOptions{
			CheckIn:  input.DepartDate,
			CheckOut: input.ReturnDate,
			Guests:   input.Guests,
			Sort:     "cheapest",
			Currency: input.Currency,
		})
	}()
	wg.Wait()

	// Extract top outbound flights (up to 5).
	if outErr == nil && outResult != nil && outResult.Success {
		result.OutboundFlights = extractTopFlights(outResult.Flights, 5)
	}

	// Extract top return flights (up to 5).
	if retErr == nil && retResult != nil && retResult.Success {
		result.ReturnFlights = extractTopFlights(retResult.Flights, 5)
	}

	// Extract top hotels (up to 5).
	if hotelErr == nil && hotelResult != nil && hotelResult.Success {
		result.Hotels = extractTopHotels(hotelResult.Hotels, nights, 5)
	}

	if input.Currency != "" {
		convertPlanFlights(ctx, result.OutboundFlights, input.Currency)
		convertPlanFlights(ctx, result.ReturnFlights, input.Currency)
		convertPlanHotels(ctx, result.Hotels, input.Currency)
	}

	// Build summary from cheapest options.
	var cheapOut, cheapRet float64
	var cheapHotel float64
	cur := choosePlanSummaryCurrency(input.Currency, result)

	if len(result.OutboundFlights) > 0 {
		cheapOut = convertedPlanAmount(ctx, result.OutboundFlights[0].Price, result.OutboundFlights[0].Currency, cur)
	}
	if len(result.ReturnFlights) > 0 {
		cheapRet = convertedPlanAmount(ctx, result.ReturnFlights[0].Price, result.ReturnFlights[0].Currency, cur)
	}
	if len(result.Hotels) > 0 {
		cheapHotel = convertedPlanAmount(ctx, result.Hotels[0].Total, result.Hotels[0].Currency, cur)
	}

	flightsTotal := (cheapOut + cheapRet) * float64(input.Guests)
	grandTotal := flightsTotal + cheapHotel

	result.Summary = PlanSummary{
		FlightsTotal: flightsTotal,
		HotelTotal:   cheapHotel,
		GrandTotal:   grandTotal,
		Currency:     cur,
	}
	if input.Guests > 0 {
		result.Summary.PerPerson = grandTotal / float64(input.Guests)
	}
	if nights > 0 {
		result.Summary.PerDay = grandTotal / float64(nights)
	}

	result.Success = len(result.OutboundFlights) > 0 && len(result.ReturnFlights) > 0 && len(result.Hotels) > 0

	// Collect errors.
	var errs []string
	var missing []string
	if len(result.OutboundFlights) == 0 {
		missing = append(missing, "outbound flights")
	}
	if len(result.ReturnFlights) == 0 {
		missing = append(missing, "return flights")
	}
	if len(result.Hotels) == 0 {
		missing = append(missing, "hotels")
	}
	if len(missing) > 0 {
		errs = append(errs, "missing "+strings.Join(missing, ", "))
	}
	if outErr != nil {
		errs = append(errs, fmt.Sprintf("outbound: %v", outErr))
	}
	if retErr != nil {
		errs = append(errs, fmt.Sprintf("return: %v", retErr))
	}
	if hotelErr != nil {
		errs = append(errs, fmt.Sprintf("hotels: %v", hotelErr))
	}
	if !result.Success && len(errs) > 0 {
		result.Error = strings.Join(errs, "; ")
	}

	return result, nil
}

func extractTopFlights(flts []models.FlightResult, n int) []PlanFlight {
	// Sort by price.
	sorted := make([]models.FlightResult, len(flts))
	copy(sorted, flts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Price < sorted[j].Price
	})

	if len(sorted) > n {
		sorted = sorted[:n]
	}

	var result []PlanFlight
	for _, f := range sorted {
		if f.Price <= 0 {
			continue
		}
		pf := PlanFlight{
			Price:    f.Price,
			Currency: f.Currency,
			Stops:    f.Stops,
			Duration: f.Duration,
		}
		if len(f.Legs) > 0 {
			pf.Airline = f.Legs[0].Airline
			pf.Flight = f.Legs[0].FlightNumber
			pf.Departure = f.Legs[0].DepartureTime
			pf.Arrival = f.Legs[len(f.Legs)-1].ArrivalTime

			parts := []string{f.Legs[0].DepartureAirport.Code}
			for _, leg := range f.Legs {
				parts = append(parts, leg.ArrivalAirport.Code)
			}
			pf.Route = joinRoute(parts)
		}
		result = append(result, pf)
	}
	return result
}

func joinRoute(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " -> "
		}
		out += p
	}
	return out
}

func extractTopHotels(htls []models.HotelResult, nights, n int) []PlanHotel {
	// Sort by price.
	sorted := make([]models.HotelResult, len(htls))
	copy(sorted, htls)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Price < sorted[j].Price
	})

	if len(sorted) > n {
		sorted = sorted[:n]
	}

	var result []PlanHotel
	for _, h := range sorted {
		if h.Price <= 0 {
			continue
		}
		ph := PlanHotel{
			Name:     h.Name,
			Rating:   h.Rating,
			Reviews:  h.ReviewCount,
			PerNight: h.Price,
			Total:    h.Price * float64(nights),
			Currency: h.Currency,
		}
		if len(h.Amenities) > 0 {
			if len(h.Amenities) > 3 {
				ph.Amenities = fmt.Sprintf("%s +%d more", joinAmenities(h.Amenities[:3]), len(h.Amenities)-3)
			} else {
				ph.Amenities = joinAmenities(h.Amenities)
			}
		}
		result = append(result, ph)
	}
	return result
}

func joinAmenities(amenities []string) string {
	out := ""
	for i, a := range amenities {
		if i > 0 {
			out += ", "
		}
		out += a
	}
	return out
}

func choosePlanSummaryCurrency(requested string, result *PlanResult) string {
	if requested != "" {
		return requested
	}
	if len(result.OutboundFlights) > 0 && result.OutboundFlights[0].Currency != "" {
		return result.OutboundFlights[0].Currency
	}
	if len(result.ReturnFlights) > 0 && result.ReturnFlights[0].Currency != "" {
		return result.ReturnFlights[0].Currency
	}
	if len(result.Hotels) > 0 && result.Hotels[0].Currency != "" {
		return result.Hotels[0].Currency
	}
	return "EUR"
}

func convertedPlanAmount(ctx context.Context, amount float64, from, to string) float64 {
	converted, _ := destinations.ConvertCurrency(ctx, amount, from, to)
	return math.Round(converted*100) / 100
}

func convertPlanFlights(ctx context.Context, flights []PlanFlight, currency string) {
	for i := range flights {
		if flights[i].Price <= 0 || flights[i].Currency == "" || flights[i].Currency == currency {
			continue
		}
		flights[i].Price = convertedPlanAmount(ctx, flights[i].Price, flights[i].Currency, currency)
		flights[i].Currency = currency
	}
}

func convertPlanHotels(ctx context.Context, hotels []PlanHotel, currency string) {
	for i := range hotels {
		if hotels[i].Currency == "" || hotels[i].Currency == currency {
			continue
		}
		if hotels[i].PerNight > 0 {
			hotels[i].PerNight = convertedPlanAmount(ctx, hotels[i].PerNight, hotels[i].Currency, currency)
		}
		if hotels[i].Total > 0 {
			hotels[i].Total = convertedPlanAmount(ctx, hotels[i].Total, hotels[i].Currency, currency)
		}
		hotels[i].Currency = currency
	}
}
