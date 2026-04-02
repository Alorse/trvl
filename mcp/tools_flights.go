package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

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
