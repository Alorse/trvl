package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// registerTools adds all trvl tool definitions and handlers to the server.
func registerTools(s *Server) {
	s.tools = []ToolDef{
		searchFlightsTool(),
		searchDatesTool(),
		searchHotelsTool(),
		hotelPricesTool(),
		destinationInfoTool(),
	}
	s.handlers["search_flights"] = handleSearchFlights
	s.handlers["search_dates"] = handleSearchDates
	s.handlers["search_hotels"] = handleSearchHotels
	s.handlers["hotel_prices"] = handleHotelPrices
	s.handlers["destination_info"] = handleDestinationInfo
}

// --- Output schema builders ---

// flightSearchOutputSchema returns the JSON Schema for FlightSearchResult.
func flightSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":   map[string]interface{}{"type": "boolean"},
			"count":     map[string]interface{}{"type": "integer"},
			"trip_type": map[string]interface{}{"type": "string"},
			"flights": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"price":    map[string]interface{}{"type": "number"},
						"currency": map[string]interface{}{"type": "string"},
						"duration": map[string]interface{}{"type": "integer"},
						"stops":    map[string]interface{}{"type": "integer"},
						"legs": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"departure_airport": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"code": map[string]interface{}{"type": "string"},
											"name": map[string]interface{}{"type": "string"},
										},
									},
									"arrival_airport": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"code": map[string]interface{}{"type": "string"},
											"name": map[string]interface{}{"type": "string"},
										},
									},
									"departure_time": map[string]interface{}{"type": "string"},
									"arrival_time":   map[string]interface{}{"type": "string"},
									"duration":       map[string]interface{}{"type": "integer"},
									"airline":        map[string]interface{}{"type": "string"},
									"airline_code":   map[string]interface{}{"type": "string"},
									"flight_number":  map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
			"suggestions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":      map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"params":      map[string]interface{}{"type": "object"},
					},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

// dateSearchOutputSchema returns the JSON Schema for DateSearchResult.
func dateSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":    map[string]interface{}{"type": "boolean"},
			"count":      map[string]interface{}{"type": "integer"},
			"trip_type":  map[string]interface{}{"type": "string"},
			"date_range": map[string]interface{}{"type": "string"},
			"dates": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date":        map[string]interface{}{"type": "string"},
						"price":       map[string]interface{}{"type": "number"},
						"currency":    map[string]interface{}{"type": "string"},
						"return_date": map[string]interface{}{"type": "string"},
					},
					"required": []string{"date", "price", "currency"},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

// hotelSearchOutputSchema returns the JSON Schema for HotelSearchResult.
func hotelSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{"type": "boolean"},
			"count":   map[string]interface{}{"type": "integer"},
			"hotels": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":         map[string]interface{}{"type": "string"},
						"hotel_id":     map[string]interface{}{"type": "string"},
						"rating":       map[string]interface{}{"type": "number"},
						"review_count": map[string]interface{}{"type": "integer"},
						"stars":        map[string]interface{}{"type": "integer"},
						"price":        map[string]interface{}{"type": "number"},
						"currency":     map[string]interface{}{"type": "string"},
						"address":      map[string]interface{}{"type": "string"},
						"lat":          map[string]interface{}{"type": "number"},
						"lon":          map[string]interface{}{"type": "number"},
						"amenities": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
			"suggestions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":      map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"params":      map[string]interface{}{"type": "object"},
					},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

// hotelPricesOutputSchema returns the JSON Schema for HotelPriceResult.
func hotelPricesOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":   map[string]interface{}{"type": "boolean"},
			"hotel_id":  map[string]interface{}{"type": "string"},
			"name":      map[string]interface{}{"type": "string"},
			"check_in":  map[string]interface{}{"type": "string"},
			"check_out": map[string]interface{}{"type": "string"},
			"providers": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"provider": map[string]interface{}{"type": "string"},
						"price":    map[string]interface{}{"type": "number"},
						"currency": map[string]interface{}{"type": "string"},
					},
					"required": []string{"provider", "price", "currency"},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success"},
	}
}

// --- Tool definitions ---

func searchFlightsTool() ToolDef {
	return ToolDef{
		Name:        "search_flights",
		Title:       "Search Flights",
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
		OutputSchema: flightSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Search Flights",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func searchDatesTool() ToolDef {
	return ToolDef{
		Name:        "search_dates",
		Title:       "Search Flight Dates",
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
		OutputSchema: dateSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Search Flight Dates",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func searchHotelsTool() ToolDef {
	return ToolDef{
		Name:        "search_hotels",
		Title:       "Search Hotels",
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
		OutputSchema: hotelSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Search Hotels",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func hotelPricesTool() ToolDef {
	return ToolDef{
		Name:        "hotel_prices",
		Title:       "Hotel Prices Comparison",
		Description: "Get prices from multiple booking providers for a specific hotel. Compares prices across providers like Booking.com, Hotels.com, Expedia, etc.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"hotel_id":  {Type: "string", Description: "Google Hotels property ID (from search_hotels results)"},
				"check_in":  {Type: "string", Description: "Check-in date in YYYY-MM-DD format"},
				"check_out": {Type: "string", Description: "Check-out date in YYYY-MM-DD format"},
				"currency":  {Type: "string", Description: "Currency code (e.g. USD, EUR). Default: USD"},
			},
			Required: []string{"hotel_id", "check_in", "check_out"},
		},
		OutputSchema: hotelPricesOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Hotel Prices Comparison",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

// --- Suggestion types ---

// Suggestion represents a follow-up action the user might take.
type Suggestion struct {
	Action      string         `json:"action"`
	Description string         `json:"description"`
	Params      map[string]any `json:"params,omitempty"`
}

// --- Helper: extract string from args ---

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func argInt(args map[string]any, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return def
		}
		return int(i)
	default:
		return def
	}
}

func argBool(args map[string]any, key string, def bool) bool {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

// --- Elicitation schemas ---

// flightElicitationSchema returns the schema for eliciting missing flight search params.
func flightElicitationSchema(origin, dest string) map[string]interface{} {
	msg := "When would you like to fly"
	if origin != "" && dest != "" {
		msg += fmt.Sprintf(" from %s to %s", origin, dest)
	}
	msg += "?"
	_ = msg // used by caller

	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"departure_date": map[string]interface{}{
				"type":        "string",
				"format":      "date",
				"title":       "Departure Date",
				"description": "When do you want to depart?",
			},
			"return_date": map[string]interface{}{
				"type":        "string",
				"format":      "date",
				"title":       "Return Date (optional)",
				"description": "Leave empty for one-way",
			},
			"cabin_class": map[string]interface{}{
				"type":    "string",
				"title":   "Cabin Class",
				"enum":    []string{"economy", "premium_economy", "business", "first"},
				"default": "economy",
			},
			"max_stops": map[string]interface{}{
				"type":    "string",
				"title":   "Maximum Stops",
				"enum":    []string{"any", "nonstop", "one_stop"},
				"default": "any",
			},
		},
		"required": []string{"departure_date"},
	}
}

// hotelElicitationSchema returns the schema for refining hotel search results.
func hotelElicitationSchema(count int, location string) map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"min_stars": map[string]interface{}{
				"type":    "integer",
				"title":   "Minimum Stars",
				"minimum": 1,
				"maximum": 5,
				"default": 3,
			},
			"max_price": map[string]interface{}{
				"type":        "number",
				"title":       "Maximum Price Per Night",
				"description": "In local currency",
			},
			"sort_by": map[string]interface{}{
				"type":    "string",
				"title":   "Sort By",
				"enum":    []string{"price", "rating", "distance"},
				"default": "price",
			},
		},
	}
}

// --- Tool handlers ---

func handleSearchFlights(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error) {
	origin := strings.ToUpper(argString(args, "origin"))
	dest := strings.ToUpper(argString(args, "destination"))
	date := argString(args, "departure_date")

	if origin == "" || dest == "" {
		return nil, nil, fmt.Errorf("origin and destination are required")
	}

	// Validate IATA codes.
	if err := models.ValidateIATA(origin); err != nil {
		return nil, nil, fmt.Errorf("invalid origin: %w", err)
	}
	if err := models.ValidateIATA(dest); err != nil {
		return nil, nil, fmt.Errorf("invalid destination: %w", err)
	}

	// Elicit departure date if missing and client supports it.
	if date == "" && elicit != nil {
		schema := flightElicitationSchema(origin, dest)
		msg := fmt.Sprintf("When would you like to fly from %s to %s?", origin, dest)
		result, err := elicit(msg, schema)
		if err != nil {
			return nil, nil, fmt.Errorf("elicitation failed: %w", err)
		}
		if result != nil {
			if d, ok := result["departure_date"].(string); ok && d != "" {
				date = d
			}
			if r, ok := result["return_date"].(string); ok && r != "" {
				args["return_date"] = r
			}
			if c, ok := result["cabin_class"].(string); ok && c != "" {
				args["cabin_class"] = c
			}
			if m, ok := result["max_stops"].(string); ok && m != "" {
				args["max_stops"] = m
			}
		}
	}

	if date == "" {
		return nil, nil, fmt.Errorf("departure_date is required")
	}

	// Validate date.
	if err := models.ValidateDate(date); err != nil {
		return nil, nil, err
	}

	// Validate return date if provided.
	if ret := argString(args, "return_date"); ret != "" {
		if err := models.ValidateDate(ret); err != nil {
			return nil, nil, fmt.Errorf("invalid return_date: %w", err)
		}
	}

	opts := flights.SearchOptions{
		ReturnDate: argString(args, "return_date"),
	}

	if cc := argString(args, "cabin_class"); cc != "" {
		parsed, err := models.ParseCabinClass(cc)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid cabin_class: %w", err)
		}
		opts.CabinClass = parsed
	}

	if ms := argString(args, "max_stops"); ms != "" {
		parsed, err := models.ParseMaxStops(ms)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid max_stops: %w", err)
		}
		opts.MaxStops = parsed
	}

	if sb := argString(args, "sort_by"); sb != "" {
		parsed, err := models.ParseSortBy(sb)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid sort_by: %w", err)
		}
		opts.SortBy = parsed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := flights.SearchFlights(ctx, origin, dest, date, opts)
	if err != nil {
		return nil, nil, err
	}

	// Build suggestions for progressive disclosure.
	suggestions := flightSuggestions(result, origin, dest, date, opts)

	// Build structured response.
	type flightResponse struct {
		*models.FlightSearchResult
		Suggestions []Suggestion `json:"suggestions,omitempty"`
	}
	resp := flightResponse{
		FlightSearchResult: result,
		Suggestions:        suggestions,
	}

	content, err := buildAnnotatedContentBlocks(flightSummary(result, origin, dest), resp)
	if err != nil {
		return nil, nil, err
	}

	return content, resp, nil
}

func handleSearchDates(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error) {
	origin := strings.ToUpper(argString(args, "origin"))
	dest := strings.ToUpper(argString(args, "destination"))
	startDate := argString(args, "start_date")
	endDate := argString(args, "end_date")

	if origin == "" || dest == "" || startDate == "" || endDate == "" {
		return nil, nil, fmt.Errorf("origin, destination, start_date, and end_date are required")
	}

	// Validate IATA codes.
	if err := models.ValidateIATA(origin); err != nil {
		return nil, nil, fmt.Errorf("invalid origin: %w", err)
	}
	if err := models.ValidateIATA(dest); err != nil {
		return nil, nil, fmt.Errorf("invalid destination: %w", err)
	}

	// Validate date range.
	if err := models.ValidateDateRange(startDate, endDate); err != nil {
		return nil, nil, err
	}

	opts := flights.DateSearchOptions{
		FromDate:  startDate,
		ToDate:    endDate,
		Duration:  argInt(args, "trip_duration", 0),
		RoundTrip: argBool(args, "is_round_trip", false),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := flights.SearchDates(ctx, origin, dest, opts)
	if err != nil {
		return nil, nil, err
	}

	summary := fmt.Sprintf("Found prices for %d dates from %s to %s (%s to %s).",
		result.Count, origin, dest, startDate, endDate)
	if result.Count > 0 {
		cheapest := result.Dates[0]
		for _, d := range result.Dates[1:] {
			if d.Price > 0 && d.Price < cheapest.Price {
				cheapest = d
			}
		}
		summary += fmt.Sprintf(" Cheapest: %s %.0f on %s.", cheapest.Currency, cheapest.Price, cheapest.Date)
	}

	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func handleSearchHotels(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error) {
	location := argString(args, "location")
	checkIn := argString(args, "check_in")
	checkOut := argString(args, "check_out")

	if location == "" || checkIn == "" || checkOut == "" {
		return nil, nil, fmt.Errorf("location, check_in, and check_out are required")
	}

	// Validate dates.
	if err := models.ValidateDateRange(checkIn, checkOut); err != nil {
		return nil, nil, err
	}

	opts := hotels.HotelSearchOptions{
		CheckIn:  checkIn,
		CheckOut: checkOut,
		Guests:   argInt(args, "guests", 2),
		Stars:    argInt(args, "stars", 0),
		Sort:     argString(args, "sort"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := hotels.SearchHotels(ctx, location, opts)
	if err != nil {
		return nil, nil, err
	}

	// If many results and client supports elicitation, offer to refine.
	if result.Success && result.Count > 10 && elicit != nil && opts.Stars == 0 {
		schema := hotelElicitationSchema(result.Count, location)
		msg := fmt.Sprintf("Found %d hotels in %s. Would you like to refine?", result.Count, location)
		refinement, elicitErr := elicit(msg, schema)
		if elicitErr == nil && refinement != nil {
			// Re-search with refined parameters.
			if stars, ok := refinement["min_stars"].(float64); ok && stars > 0 {
				opts.Stars = int(stars)
			}
			if sort, ok := refinement["sort_by"].(string); ok && sort != "" {
				opts.Sort = sort
			}
			// Re-run the search with refined options.
			result, err = hotels.SearchHotels(ctx, location, opts)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// Build suggestions for progressive disclosure.
	suggestions := hotelSuggestions(result, opts)

	type hotelResponse struct {
		*models.HotelSearchResult
		Suggestions []Suggestion `json:"suggestions,omitempty"`
	}
	resp := hotelResponse{
		HotelSearchResult: result,
		Suggestions:       suggestions,
	}

	content, err := buildAnnotatedContentBlocks(hotelSummary(result, location), resp)
	if err != nil {
		return nil, nil, err
	}

	return content, resp, nil
}

func handleHotelPrices(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error) {
	hotelID := argString(args, "hotel_id")
	checkIn := argString(args, "check_in")
	checkOut := argString(args, "check_out")
	currency := argString(args, "currency")
	if currency == "" {
		currency = "USD"
	}

	if hotelID == "" || checkIn == "" || checkOut == "" {
		return nil, nil, fmt.Errorf("hotel_id, check_in, and check_out are required")
	}

	// Validate dates.
	if err := models.ValidateDateRange(checkIn, checkOut); err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := hotels.GetHotelPrices(ctx, hotelID, checkIn, checkOut, currency)
	if err != nil {
		return nil, nil, err
	}

	summary := fmt.Sprintf("Found %d booking providers for hotel %s (%s to %s).",
		len(result.Providers), hotelID, checkIn, checkOut)
	if len(result.Providers) > 0 {
		cheapest := result.Providers[0]
		for _, p := range result.Providers[1:] {
			if p.Price > 0 && p.Price < cheapest.Price {
				cheapest = p
			}
		}
		summary += fmt.Sprintf(" Cheapest: %s %.0f via %s.", cheapest.Currency, cheapest.Price, cheapest.Provider)
	}

	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

// --- Summary builders ---

func flightSummary(result *models.FlightSearchResult, origin, dest string) string {
	if !result.Success || result.Count == 0 {
		if result.Error != "" {
			return fmt.Sprintf("Flight search from %s to %s failed: %s", origin, dest, result.Error)
		}
		return fmt.Sprintf("No flights found from %s to %s.", origin, dest)
	}

	summary := fmt.Sprintf("Found %d flights from %s to %s.", result.Count, origin, dest)

	// Find cheapest.
	cheapest := result.Flights[0]
	for _, f := range result.Flights[1:] {
		if f.Price > 0 && f.Price < cheapest.Price {
			cheapest = f
		}
	}
	if cheapest.Price > 0 {
		stopStr := "nonstop"
		if cheapest.Stops == 1 {
			stopStr = "1 stop"
		} else if cheapest.Stops > 1 {
			stopStr = fmt.Sprintf("%d stops", cheapest.Stops)
		}
		airline := ""
		if len(cheapest.Legs) > 0 {
			airline = cheapest.Legs[0].Airline
		}
		summary += fmt.Sprintf(" Cheapest: %s%.0f (%s, %s).",
			cheapest.Currency, cheapest.Price, airline, stopStr)
	}

	// Check for nonstop options.
	nonstopCount := 0
	var cheapestNonstop *models.FlightResult
	for i := range result.Flights {
		if result.Flights[i].Stops == 0 {
			nonstopCount++
			if cheapestNonstop == nil || result.Flights[i].Price < cheapestNonstop.Price {
				cheapestNonstop = &result.Flights[i]
			}
		}
	}
	if nonstopCount > 0 && cheapestNonstop != nil {
		summary += fmt.Sprintf(" Nonstop options from %s%.0f.", cheapestNonstop.Currency, cheapestNonstop.Price)
	}

	return summary
}

func hotelSummary(result *models.HotelSearchResult, location string) string {
	if !result.Success || result.Count == 0 {
		if result.Error != "" {
			return fmt.Sprintf("Hotel search in %s failed: %s", location, result.Error)
		}
		return fmt.Sprintf("No hotels found in %s.", location)
	}

	summary := fmt.Sprintf("Found %d hotels in %s.", result.Count, location)

	// Find cheapest.
	var cheapest *models.HotelResult
	for i := range result.Hotels {
		if result.Hotels[i].Price > 0 {
			if cheapest == nil || result.Hotels[i].Price < cheapest.Price {
				cheapest = &result.Hotels[i]
			}
		}
	}
	if cheapest != nil {
		summary += fmt.Sprintf(" Cheapest: %s%.0f/night (%s).",
			cheapest.Currency, cheapest.Price, cheapest.Name)
	}

	// Find highest rated.
	var bestRated *models.HotelResult
	for i := range result.Hotels {
		if result.Hotels[i].Rating > 0 {
			if bestRated == nil || result.Hotels[i].Rating > bestRated.Rating {
				bestRated = &result.Hotels[i]
			}
		}
	}
	if bestRated != nil && (cheapest == nil || bestRated.Name != cheapest.Name) {
		summary += fmt.Sprintf(" Highest rated: %s (%.1f/5).", bestRated.Name, bestRated.Rating)
	}

	return summary
}

// --- Suggestion builders ---

func flightSuggestions(result *models.FlightSearchResult, origin, dest, date string, opts flights.SearchOptions) []Suggestion {
	var suggestions []Suggestion

	if !result.Success || result.Count == 0 {
		return nil
	}

	// If searching one-way, suggest round-trip.
	if opts.ReturnDate == "" {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_flights",
			Description: "Search round-trip for potentially lower fares",
			Params:      map[string]any{"origin": origin, "destination": dest, "departure_date": date, "return_date": "YYYY-MM-DD"},
		})
	}

	// If there are many stops, suggest nonstop filter.
	hasMultiStop := false
	for _, f := range result.Flights {
		if f.Stops >= 2 {
			hasMultiStop = true
			break
		}
	}
	if hasMultiStop && opts.MaxStops == 0 {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_flights",
			Description: "Filter to nonstop flights only",
			Params:      map[string]any{"origin": origin, "destination": dest, "departure_date": date, "max_stops": "nonstop"},
		})
	}

	// If prices vary widely, suggest flexible dates.
	if result.Count >= 3 {
		minPrice := result.Flights[0].Price
		maxPrice := result.Flights[0].Price
		for _, f := range result.Flights[1:] {
			if f.Price > 0 && f.Price < minPrice {
				minPrice = f.Price
			}
			if f.Price > maxPrice {
				maxPrice = f.Price
			}
		}
		if maxPrice > 0 && minPrice > 0 && maxPrice > minPrice*2 {
			suggestions = append(suggestions, Suggestion{
				Action:      "search_dates",
				Description: "Find the cheapest departure date this month",
				Params:      map[string]any{"origin": origin, "destination": dest},
			})
		}
	}

	// If economy, suggest checking business class.
	if opts.CabinClass == 0 || opts.CabinClass == models.Economy {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_flights",
			Description: "Check business class availability",
			Params:      map[string]any{"origin": origin, "destination": dest, "departure_date": date, "cabin_class": "business"},
		})
	}

	return suggestions
}

func hotelSuggestions(result *models.HotelSearchResult, opts hotels.HotelSearchOptions) []Suggestion {
	var suggestions []Suggestion

	if !result.Success || result.Count == 0 {
		return nil
	}

	// If no star filter, suggest filtering.
	if opts.Stars == 0 {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_hotels",
			Description: "Filter to 4+ star hotels only",
			Params:      map[string]any{"stars": 4},
		})
	}

	// If a great-rated hotel is found, suggest getting detailed pricing.
	for _, h := range result.Hotels {
		if h.Rating >= 4.5 && h.HotelID != "" {
			suggestions = append(suggestions, Suggestion{
				Action:      "hotel_prices",
				Description: fmt.Sprintf("Get detailed pricing for %s (%.1f rating)", h.Name, h.Rating),
				Params:      map[string]any{"hotel_id": h.HotelID, "check_in": opts.CheckIn, "check_out": opts.CheckOut},
			})
			break // Only suggest the top one.
		}
	}

	// If results are expensive, suggest expanding search.
	expensiveCount := 0
	for _, h := range result.Hotels {
		if h.Price > 300 {
			expensiveCount++
		}
	}
	if expensiveCount > result.Count/2 {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_hotels",
			Description: "Try expanding the area or checking different dates for more affordable options",
		})
	}

	return suggestions
}

// --- Destination info tool ---

func destinationInfoTool() ToolDef {
	return ToolDef{
		Name:        "destination_info",
		Title:       "Destination Info",
		Description: "Get travel intelligence for any city: weather forecast, country info, public holidays, safety advisory, and currency exchange rates. All from free APIs, no keys required.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"location":     {Type: "string", Description: "City or location name (e.g., Tokyo, Barcelona, New York)"},
				"travel_dates": {Type: "string", Description: "Optional travel date range as YYYY-MM-DD,YYYY-MM-DD (e.g., 2026-06-15,2026-06-18)"},
			},
			Required: []string{"location"},
		},
		OutputSchema: destinationInfoOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Destination Info",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func destinationInfoOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{"type": "string"},
			"country": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":       map[string]interface{}{"type": "string"},
					"code":       map[string]interface{}{"type": "string"},
					"capital":    map[string]interface{}{"type": "string"},
					"languages":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"currencies": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"region":     map[string]interface{}{"type": "string"},
				},
			},
			"weather": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"current": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"date":               map[string]interface{}{"type": "string"},
							"temp_high_c":        map[string]interface{}{"type": "number"},
							"temp_low_c":         map[string]interface{}{"type": "number"},
							"precipitation_mm":   map[string]interface{}{"type": "number"},
							"description":        map[string]interface{}{"type": "string"},
						},
					},
					"forecast": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"date":             map[string]interface{}{"type": "string"},
								"temp_high_c":      map[string]interface{}{"type": "number"},
								"temp_low_c":       map[string]interface{}{"type": "number"},
								"precipitation_mm": map[string]interface{}{"type": "number"},
								"description":      map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			"holidays": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date": map[string]interface{}{"type": "string"},
						"name": map[string]interface{}{"type": "string"},
						"type": map[string]interface{}{"type": "string"},
					},
				},
			},
			"safety": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"level":        map[string]interface{}{"type": "number"},
					"advisory":     map[string]interface{}{"type": "string"},
					"source":       map[string]interface{}{"type": "string"},
					"last_updated": map[string]interface{}{"type": "string"},
				},
			},
			"currency": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"local_currency": map[string]interface{}{"type": "string"},
					"exchange_rate":  map[string]interface{}{"type": "number"},
					"base_currency":  map[string]interface{}{"type": "string"},
				},
			},
			"timezone": map[string]interface{}{"type": "string"},
		},
		"required": []string{"location"},
	}
}

func handleDestinationInfo(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error) {
	location := argString(args, "location")
	if location == "" {
		return nil, nil, fmt.Errorf("location is required")
	}

	travelDates := argString(args, "travel_dates")
	var dates models.DateRange
	if travelDates != "" {
		parts := strings.SplitN(travelDates, ",", 2)
		if len(parts) == 2 {
			dates.CheckIn = strings.TrimSpace(parts[0])
			dates.CheckOut = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 {
			dates.CheckIn = strings.TrimSpace(parts[0])
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := destinations.GetDestinationInfo(ctx, location, dates)
	if err != nil {
		return nil, nil, err
	}

	summary := destinationSummary(info)
	content, err := buildAnnotatedContentBlocks(summary, info)
	if err != nil {
		return nil, nil, err
	}

	return content, info, nil
}

func destinationSummary(info *models.DestinationInfo) string {
	parts := []string{fmt.Sprintf("Destination: %s", info.Location)}

	if info.Country.Name != "" {
		parts = append(parts, fmt.Sprintf("Country: %s (%s)", info.Country.Name, info.Country.Region))
	}

	if info.Weather.Current.Date != "" {
		parts = append(parts, fmt.Sprintf("Today: %s, %.0f-%.0f C",
			info.Weather.Current.Description, info.Weather.Current.TempLow, info.Weather.Current.TempHigh))
	}

	if info.Safety.Source != "" {
		parts = append(parts, fmt.Sprintf("Safety: %.1f/5 - %s", info.Safety.Level, info.Safety.Advisory))
	}

	if info.Currency.LocalCurrency != "" && info.Currency.ExchangeRate > 0 {
		parts = append(parts, fmt.Sprintf("Currency: 1 EUR = %.2f %s", info.Currency.ExchangeRate, info.Currency.LocalCurrency))
	}

	if len(info.Holidays) > 0 {
		parts = append(parts, fmt.Sprintf("%d public holidays during travel dates", len(info.Holidays)))
	}

	return strings.Join(parts, ". ") + "."
}

// --- Content block builder ---

// buildAnnotatedContentBlocks creates a text summary block (for user) and a
// structured JSON block (for assistant), with content annotations per the
// 2025-11-25 spec.
func buildAnnotatedContentBlocks(summary string, data any) ([]ContentBlock, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	return []ContentBlock{
		{
			Type: "text",
			Text: summary,
			Annotations: &ContentAnnotation{
				Audience: []string{"user"},
				Priority: 1.0,
			},
		},
		{
			Type: "text",
			Text: string(jsonData),
			Annotations: &ContentAnnotation{
				Audience: []string{"assistant"},
				Priority: 0.5,
			},
		},
	}, nil
}
