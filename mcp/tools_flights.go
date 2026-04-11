package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/MikkoParkkola/trvl/internal/trip"
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
		Description: "Search flights via Google Flights. Returns real-time pricing, durations, stops, and leg details for a given route and date. IMPORTANT: call get_preferences before your first search in a conversation to load the user's home airport and flight preferences. If the profile is empty, interview the user first — get_preferences returns instructions. Use home_airports as default origin when the user doesn't specify where from.",
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

// --- Tool handlers ---

func handleSearchFlights(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
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

	result, err := flights.SearchFlights(ctx, origin, dest, date, opts)
	if err != nil {
		return nil, nil, err
	}

	// Apply preference-based post-filters (budget, departure time window).
	prefs, _ := preferences.Load()
	if prefs != nil && result != nil && result.Success {
		result.Flights = flights.FilterFlightsByBudget(result.Flights, prefs.BudgetFlightMax)
		result.Flights = flights.FilterFlightsByTimePreference(result.Flights, prefs.FlightTimeEarliest, prefs.FlightTimeLatest)
		result.Count = len(result.Flights)
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

func handleSearchDates(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
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

// --- Suggest Dates tool ---

// suggestDatesOutputSchema returns the JSON Schema for SmartDateResult.
func suggestDatesOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":     map[string]interface{}{"type": "boolean"},
			"origin":      map[string]interface{}{"type": "string"},
			"destination": map[string]interface{}{"type": "string"},
			"cheapest_dates": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date":        map[string]interface{}{"type": "string"},
						"day_of_week": map[string]interface{}{"type": "string"},
						"price":       map[string]interface{}{"type": "number"},
						"currency":    map[string]interface{}{"type": "string"},
						"return_date": map[string]interface{}{"type": "string"},
					},
					"required": []string{"date", "price", "currency"},
				},
			},
			"average_price": map[string]interface{}{"type": "number"},
			"currency":      map[string]interface{}{"type": "string"},
			"insights": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type":        map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"date":        map[string]interface{}{"type": "string"},
						"price":       map[string]interface{}{"type": "number"},
						"savings":     map[string]interface{}{"type": "number"},
					},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success"},
	}
}

func suggestDatesTool() ToolDef {
	return ToolDef{
		Name:        "suggest_dates",
		Title:       "Smart Date Suggestions",
		Description: "Analyze flight prices around a target date and suggest the cheapest travel dates. Returns the 3 cheapest dates, weekday vs weekend analysis, and actionable savings insights.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":      {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK)"},
				"destination": {Type: "string", Description: "Arrival airport IATA code (e.g., BCN, LHR)"},
				"target_date": {Type: "string", Description: "Center date to search around (YYYY-MM-DD)"},
				"flex_days":   {Type: "integer", Description: "Days of flexibility around target date (default: 7)"},
				"round_trip":  {Type: "boolean", Description: "Whether to search round-trip prices (default: false)"},
				"duration":    {Type: "integer", Description: "Trip duration in days for round-trip (default: 7)"},
			},
			Required: []string{"origin", "destination", "target_date"},
		},
		OutputSchema: suggestDatesOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Smart Date Suggestions",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func handleSuggestDates(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	origin := strings.ToUpper(argString(args, "origin"))
	dest := strings.ToUpper(argString(args, "destination"))
	targetDate := argString(args, "target_date")

	if origin == "" || dest == "" {
		return nil, nil, fmt.Errorf("origin and destination are required")
	}
	if targetDate == "" {
		return nil, nil, fmt.Errorf("target_date is required")
	}

	if err := models.ValidateIATA(origin); err != nil {
		return nil, nil, fmt.Errorf("invalid origin: %w", err)
	}
	if err := models.ValidateIATA(dest); err != nil {
		return nil, nil, fmt.Errorf("invalid destination: %w", err)
	}

	opts := trip.SmartDateOptions{
		TargetDate: targetDate,
		FlexDays:   argInt(args, "flex_days", 7),
		RoundTrip:  argBool(args, "round_trip", false),
		Duration:   argInt(args, "duration", 7),
	}

	result, err := trip.SuggestDates(ctx, origin, dest, opts)
	if err != nil {
		return nil, nil, err
	}

	summary := suggestDatesSummary(result, origin, dest)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func suggestDatesSummary(result *trip.SmartDateResult, origin, dest string) string {
	if !result.Success {
		if result.Error != "" {
			return fmt.Sprintf("Date suggestion %s to %s failed: %s", origin, dest, result.Error)
		}
		return fmt.Sprintf("Could not find date suggestions from %s to %s.", origin, dest)
	}

	parts := []string{
		fmt.Sprintf("Date analysis %s -> %s (avg %s %.0f)", origin, dest, result.Currency, result.AveragePrice),
	}

	for _, ins := range result.Insights {
		parts = append(parts, ins.Description)
	}

	return strings.Join(parts, ". ") + "."
}

// --- Optimize Multi-City tool ---

// multiCityOutputSchema returns the JSON Schema for MultiCityResult.
func multiCityOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":      map[string]interface{}{"type": "boolean"},
			"home_airport": map[string]interface{}{"type": "string"},
			"optimal_order": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
			"segments": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"from":     map[string]interface{}{"type": "string"},
						"to":       map[string]interface{}{"type": "string"},
						"price":    map[string]interface{}{"type": "number"},
						"currency": map[string]interface{}{"type": "string"},
					},
					"required": []string{"from", "to", "price", "currency"},
				},
			},
			"total_cost":          map[string]interface{}{"type": "number"},
			"currency":            map[string]interface{}{"type": "string"},
			"worst_cost":          map[string]interface{}{"type": "number"},
			"savings":             map[string]interface{}{"type": "number"},
			"permutations_checked": map[string]interface{}{"type": "integer"},
			"error":               map[string]interface{}{"type": "string"},
		},
		"required": []string{"success"},
	}
}

func optimizeMultiCityTool() ToolDef {
	return ToolDef{
		Name:        "optimize_multi_city",
		Title:       "Multi-City Trip Optimizer",
		Description: "Find the cheapest routing order for visiting multiple cities. Tries all permutations (up to 6 cities) and returns the optimal visit order with per-segment prices.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"home_airport": {Type: "string", Description: "Home airport IATA code (e.g., HEL, JFK)"},
				"cities":       {Type: "string", Description: "Comma-separated list of city IATA codes to visit (e.g., BCN,ROM,PAR)"},
				"depart_date":  {Type: "string", Description: "Departure date (YYYY-MM-DD)"},
				"return_date":  {Type: "string", Description: "Return date (YYYY-MM-DD, optional)"},
			},
			Required: []string{"home_airport", "cities", "depart_date"},
		},
		OutputSchema: multiCityOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Multi-City Trip Optimizer",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func handleOptimizeMultiCity(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	home := strings.ToUpper(argString(args, "home_airport"))
	citiesStr := argString(args, "cities")
	departDate := argString(args, "depart_date")
	returnDate := argString(args, "return_date")

	if home == "" {
		return nil, nil, fmt.Errorf("home_airport is required")
	}
	if citiesStr == "" {
		return nil, nil, fmt.Errorf("cities is required")
	}
	if departDate == "" {
		return nil, nil, fmt.Errorf("depart_date is required")
	}

	if err := models.ValidateIATA(home); err != nil {
		return nil, nil, fmt.Errorf("invalid home_airport: %w", err)
	}

	cities := argStringSlice(args, "cities")
	if len(cities) == 0 {
		return nil, nil, fmt.Errorf("at least one city is required")
	}
	for i, c := range cities {
		cities[i] = strings.ToUpper(strings.TrimSpace(c))
		if err := models.ValidateIATA(cities[i]); err != nil {
			return nil, nil, fmt.Errorf("invalid city %q: %w", c, err)
		}
	}

	opts := trip.MultiCityOptions{
		DepartDate: departDate,
		ReturnDate: returnDate,
	}

	result, err := trip.OptimizeMultiCity(ctx, home, cities, opts)
	if err != nil {
		return nil, nil, err
	}

	summary := multiCitySummary(result)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func multiCitySummary(result *trip.MultiCityResult) string {
	if !result.Success {
		if result.Error != "" {
			return fmt.Sprintf("Multi-city optimization failed: %s", result.Error)
		}
		return "Could not optimize multi-city routing."
	}

	route := append([]string{result.HomeAirport}, result.OptimalOrder...)
	route = append(route, result.HomeAirport)
	routeStr := strings.Join(route, " -> ")

	summary := fmt.Sprintf("Optimal route: %s. Total: %s %.0f.", routeStr, result.Currency, result.TotalCost)
	if result.Savings > 0 {
		summary += fmt.Sprintf(" Saves %s %.0f vs worst order.", result.Currency, result.Savings)
	}

	return summary
}
