package mcp

import (
	"encoding/json"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// registerTools adds all trvl tool definitions and handlers to the server.
func registerTools(s *Server) {
	s.tools = []ToolDef{
		searchFlightsTool(),
		searchDatesTool(),
		searchHotelsTool(),
		hotelPricesTool(),
	}
	s.handlers["search_flights"] = handleSearchFlights
	s.handlers["search_dates"] = handleSearchDates
	s.handlers["search_hotels"] = handleSearchHotels
	s.handlers["hotel_prices"] = handleHotelPrices
}

// --- Tool definitions ---

func searchFlightsTool() ToolDef {
	return ToolDef{
		Name:        "search_flights",
		Description: "Search flights via Google Flights. Returns real-time pricing, durations, stops, and leg details for a given route and date.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":         {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK, NRT)"},
				"destination":    {Type: "string", Description: "Arrival airport IATA code (e.g., NRT, LAX, CDG)"},
				"departure_date": {Type: "string", Description: "Departure date in YYYY-MM-DD format"},
				"return_date":    {Type: "string", Description: "Return date in YYYY-MM-DD format for round-trip (omit for one-way)"},
				"cabin_class":    {Type: "string", Description: "Cabin class: economy, premium_economy, business, or first (default: economy)"},
				"max_stops":      {Type: "string", Description: "Maximum stops: any, nonstop, one_stop, or two_plus (default: any)"},
				"sort_by":        {Type: "string", Description: "Sort order: cheapest, duration, departure, or arrival (default: cheapest)"},
			},
			Required: []string{"origin", "destination", "departure_date"},
		},
	}
}

func searchDatesTool() ToolDef {
	return ToolDef{
		Name:        "search_dates",
		Description: "Find the cheapest flight prices across a date range. Returns one price per departure date, useful for finding the best travel dates.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":        {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK, NRT)"},
				"destination":   {Type: "string", Description: "Arrival airport IATA code (e.g., NRT, LAX, CDG)"},
				"start_date":    {Type: "string", Description: "Start of date range in YYYY-MM-DD format"},
				"end_date":      {Type: "string", Description: "End of date range in YYYY-MM-DD format"},
				"trip_duration": {Type: "integer", Description: "Trip duration in days for round-trip (omit for one-way)"},
				"is_round_trip": {Type: "boolean", Description: "Whether to search round-trip fares (default: false)"},
			},
			Required: []string{"origin", "destination", "start_date", "end_date"},
		},
	}
}

func searchHotelsTool() ToolDef {
	return ToolDef{
		Name:        "search_hotels",
		Description: "Search hotels via Google Hotels. Returns real-time pricing, ratings, star levels, and amenities for a given location and dates.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"location":  {Type: "string", Description: "Location name or address (e.g., Helsinki, Tokyo, Manhattan New York)"},
				"check_in":  {Type: "string", Description: "Check-in date in YYYY-MM-DD format"},
				"check_out": {Type: "string", Description: "Check-out date in YYYY-MM-DD format"},
				"guests":    {Type: "integer", Description: "Number of guests (default: 2)"},
				"stars":     {Type: "integer", Description: "Minimum star rating 1-5 (default: no filter)"},
				"sort":      {Type: "string", Description: "Sort order: relevance, price, rating, or distance (default: relevance)"},
			},
			Required: []string{"location", "check_in", "check_out"},
		},
	}
}

func hotelPricesTool() ToolDef {
	return ToolDef{
		Name:        "hotel_prices",
		Description: "Get prices from multiple booking providers for a specific hotel. Compares prices across providers like Booking.com, Hotels.com, Expedia, etc.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"hotel_id":  {Type: "string", Description: "Google Hotels property ID (from search_hotels results)"},
				"check_in":  {Type: "string", Description: "Check-in date in YYYY-MM-DD format"},
				"check_out": {Type: "string", Description: "Check-out date in YYYY-MM-DD format"},
			},
			Required: []string{"hotel_id", "check_in", "check_out"},
		},
	}
}

// --- Tool handlers ---
//
// All handlers currently return stub responses. The actual search packages
// (internal/flights and internal/hotels) are being built in parallel by other
// agents. Once they are ready, these handlers will call into them.

func handleSearchFlights(args map[string]any) (string, error) {
	result := models.FlightSearchResult{
		Success: false,
		Error:   "not yet implemented",
	}
	return marshalResult(result)
}

func handleSearchDates(args map[string]any) (string, error) {
	result := models.DateSearchResult{
		Success: false,
		Error:   "not yet implemented",
	}
	return marshalResult(result)
}

func handleSearchHotels(args map[string]any) (string, error) {
	result := models.HotelSearchResult{
		Success: false,
		Error:   "not yet implemented",
	}
	return marshalResult(result)
}

func handleHotelPrices(args map[string]any) (string, error) {
	result := models.HotelPriceResult{
		Success: false,
		Error:   "not yet implemented",
	}
	return marshalResult(result)
}

// marshalResult encodes a result value as indented JSON.
func marshalResult(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
