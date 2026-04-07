package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
)

// --- Tool definition ---

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
							"date":             map[string]interface{}{"type": "string"},
							"temp_high_c":      map[string]interface{}{"type": "number"},
							"temp_low_c":       map[string]interface{}{"type": "number"},
							"precipitation_mm": map[string]interface{}{"type": "number"},
							"description":      map[string]interface{}{"type": "string"},
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

// --- Tool handler ---

func handleDestinationInfo(args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
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

// --- Weekend Getaway tool ---

// weekendGetawayOutputSchema returns the JSON Schema for WeekendResult.
func weekendGetawayOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{"type": "boolean"},
			"origin":  map[string]interface{}{"type": "string"},
			"month":   map[string]interface{}{"type": "string"},
			"nights":  map[string]interface{}{"type": "integer"},
			"count":   map[string]interface{}{"type": "integer"},
			"destinations": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"destination":    map[string]interface{}{"type": "string"},
						"airport_code":   map[string]interface{}{"type": "string"},
						"flight_price":   map[string]interface{}{"type": "number"},
						"hotel_estimate": map[string]interface{}{"type": "number"},
						"total_estimate": map[string]interface{}{"type": "number"},
						"currency":       map[string]interface{}{"type": "string"},
						"stops":          map[string]interface{}{"type": "integer"},
						"airline_name":   map[string]interface{}{"type": "string"},
					},
					"required": []string{"destination", "airport_code", "total_estimate", "currency"},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

func weekendGetawayTool() ToolDef {
	return ToolDef{
		Name:        "weekend_getaway",
		Title:       "Weekend Getaway Finder",
		Description: "Find cheap weekend getaway destinations from an airport. Returns top 10 cheapest destinations ranked by total estimated cost (round-trip flight + estimated hotel).",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":     {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK)"},
				"month":      {Type: "string", Description: "Month to search (e.g., july-2026, 2026-07)"},
				"max_budget": {Type: "number", Description: "Maximum total budget in EUR (0 = no limit)"},
				"nights":     {Type: "integer", Description: "Number of nights (default: 2)"},
			},
			Required: []string{"origin", "month"},
		},
		OutputSchema: weekendGetawayOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Weekend Getaway Finder",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func handleWeekendGetaway(args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	origin := strings.ToUpper(argString(args, "origin"))
	month := argString(args, "month")

	if origin == "" {
		return nil, nil, fmt.Errorf("origin is required")
	}
	if month == "" {
		return nil, nil, fmt.Errorf("month is required")
	}

	if err := models.ValidateIATA(origin); err != nil {
		return nil, nil, fmt.Errorf("invalid origin: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := trip.WeekendOptions{
		Month:     month,
		MaxBudget: argFloat(args, "max_budget", 0),
		Nights:    argInt(args, "nights", 2),
	}

	result, err := trip.FindWeekendGetaways(ctx, origin, opts)
	if err != nil {
		return nil, nil, err
	}

	summary := weekendSummary(result)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func weekendSummary(result *trip.WeekendResult) string {
	if !result.Success || result.Count == 0 {
		if result.Error != "" {
			return fmt.Sprintf("Weekend getaway search failed: %s", result.Error)
		}
		return "No weekend getaway destinations found."
	}

	parts := []string{
		fmt.Sprintf("Found %d weekend getaway destinations from %s in %s (%d nights)",
			result.Count, result.Origin, result.Month, result.Nights),
	}

	if len(result.Destinations) > 0 {
		d := result.Destinations[0]
		parts = append(parts, fmt.Sprintf("Cheapest: %s (%s) - EUR %.0f total (flight %.0f + hotel est. %.0f)",
			d.Destination, d.AirportCode, d.TotalEstimate, d.FlightPrice, d.HotelEstimate))
	}

	return strings.Join(parts, ". ") + "."
}

// --- Trip Cost tool ---

func tripCostTool() ToolDef {
	return ToolDef{
		Name:        "calculate_trip_cost",
		Title:       "Calculate Trip Cost",
		Description: "Estimate total trip cost including outbound flight, return flight, and hotel accommodation. Flights are priced per person; hotels are per room.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":      {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK)"},
				"destination": {Type: "string", Description: "Destination airport IATA code (e.g., BCN, LHR)"},
				"depart_date": {Type: "string", Description: "Departure date in YYYY-MM-DD format"},
				"return_date": {Type: "string", Description: "Return date in YYYY-MM-DD format"},
				"guests":      {Type: "integer", Description: "Number of guests (default: 1, must be >= 1). Flights multiply by guests; hotel is per room."},
				"currency":    {Type: "string", Description: "Currency code for totals (default: EUR)"},
			},
			Required: []string{"origin", "destination", "depart_date", "return_date"},
		},
		OutputSchema: tripCostOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Calculate Trip Cost",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func tripCostOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":    map[string]interface{}{"type": "boolean"},
			"flights":    map[string]interface{}{"type": "object"},
			"hotels":     map[string]interface{}{"type": "object"},
			"total":      map[string]interface{}{"type": "number"},
			"currency":   map[string]interface{}{"type": "string"},
			"per_person": map[string]interface{}{"type": "number"},
			"per_day":    map[string]interface{}{"type": "number"},
			"nights":     map[string]interface{}{"type": "integer"},
			"error":      map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "total", "currency"},
	}
}

func handleTripCost(args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	origin := strings.ToUpper(argString(args, "origin"))
	dest := strings.ToUpper(argString(args, "destination"))
	departDate := argString(args, "depart_date")
	returnDate := argString(args, "return_date")
	guests := argInt(args, "guests", 1)
	currency := argString(args, "currency")

	if origin == "" || dest == "" {
		return nil, nil, fmt.Errorf("origin and destination are required")
	}
	if departDate == "" || returnDate == "" {
		return nil, nil, fmt.Errorf("depart_date and return_date are required")
	}

	if err := models.ValidateIATA(origin); err != nil {
		return nil, nil, fmt.Errorf("invalid origin: %w", err)
	}
	if err := models.ValidateIATA(dest); err != nil {
		return nil, nil, fmt.Errorf("invalid destination: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := trip.CalculateTripCost(ctx, trip.TripCostInput{
		Origin:      origin,
		Destination: dest,
		DepartDate:  departDate,
		ReturnDate:  returnDate,
		Guests:      guests,
		Currency:    currency,
	})
	if err != nil {
		return nil, nil, err
	}

	summary := tripCostSummary(result, origin, dest, guests)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func tripCostSummary(result *trip.TripCostResult, origin, dest string, guests int) string {
	if !result.Success {
		if result.Error != "" {
			return fmt.Sprintf("Trip cost estimation %s to %s failed: %s", origin, dest, result.Error)
		}
		return fmt.Sprintf("Could not estimate trip cost from %s to %s.", origin, dest)
	}

	parts := []string{
		fmt.Sprintf("Trip %s -> %s: %d nights, %d guest(s)", origin, dest, result.Nights, guests),
	}

	if result.Flights.Outbound > 0 || result.Flights.Return > 0 {
		parts = append(parts, fmt.Sprintf("Flights: outbound %s, return %s",
			tripCostSummaryAmount(result.Flights.Outbound, result.Flights.Currency),
			tripCostSummaryAmount(result.Flights.Return, result.Flights.Currency)))
	}
	if result.Hotels.PerNight > 0 {
		parts = append(parts, fmt.Sprintf("Hotel: %s %.0f/night (%s)",
			result.Hotels.Currency, result.Hotels.PerNight, result.Hotels.Name))
	} else {
		parts = append(parts, "Hotel: unavailable")
	}

	parts = append(parts, fmt.Sprintf("Total: %s %.0f (%.0f/person, %.0f/day)",
		result.Currency, result.Total, result.PerPerson, result.PerDay))
	if result.Error != "" {
		parts = append(parts, "Warning: "+result.Error)
	}

	return strings.Join(parts, ". ") + "."
}

func tripCostSummaryAmount(amount float64, currency string) string {
	if amount <= 0 {
		return "unavailable"
	}
	return fmt.Sprintf("%s %.0f", currency, amount)
}

// --- Plan Trip tool ---

func planTripOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":     map[string]interface{}{"type": "boolean"},
			"origin":      map[string]interface{}{"type": "string"},
			"destination": map[string]interface{}{"type": "string"},
			"depart_date": map[string]interface{}{"type": "string"},
			"return_date": map[string]interface{}{"type": "string"},
			"nights":      map[string]interface{}{"type": "integer"},
			"guests":      map[string]interface{}{"type": "integer"},
			"outbound_flights": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"price":         map[string]interface{}{"type": "number"},
						"currency":      map[string]interface{}{"type": "string"},
						"airline":       map[string]interface{}{"type": "string"},
						"flight_number": map[string]interface{}{"type": "string"},
						"stops":         map[string]interface{}{"type": "integer"},
						"duration_min":  map[string]interface{}{"type": "integer"},
						"departure":     map[string]interface{}{"type": "string"},
						"arrival":       map[string]interface{}{"type": "string"},
						"route":         map[string]interface{}{"type": "string"},
					},
				},
			},
			"return_flights": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"price":         map[string]interface{}{"type": "number"},
						"currency":      map[string]interface{}{"type": "string"},
						"airline":       map[string]interface{}{"type": "string"},
						"flight_number": map[string]interface{}{"type": "string"},
						"stops":         map[string]interface{}{"type": "integer"},
						"duration_min":  map[string]interface{}{"type": "integer"},
						"departure":     map[string]interface{}{"type": "string"},
						"arrival":       map[string]interface{}{"type": "string"},
						"route":         map[string]interface{}{"type": "string"},
					},
				},
			},
			"hotels": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":      map[string]interface{}{"type": "string"},
						"rating":    map[string]interface{}{"type": "number"},
						"reviews":   map[string]interface{}{"type": "integer"},
						"per_night": map[string]interface{}{"type": "number"},
						"total":     map[string]interface{}{"type": "number"},
						"currency":  map[string]interface{}{"type": "string"},
						"amenities": map[string]interface{}{"type": "string"},
					},
				},
			},
			"summary": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"flights_total": map[string]interface{}{"type": "number"},
					"hotel_total":   map[string]interface{}{"type": "number"},
					"grand_total":   map[string]interface{}{"type": "number"},
					"per_person":    map[string]interface{}{"type": "number"},
					"per_day":       map[string]interface{}{"type": "number"},
					"currency":      map[string]interface{}{"type": "string"},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "origin", "destination"},
	}
}

func planTripTool() ToolDef {
	return ToolDef{
		Name:        "plan_trip",
		Title:       "Plan Complete Trip",
		Description: "Plan a complete trip with outbound flights, return flights, and hotel options in one search. Returns top 5 options for each plus a total cost summary.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":      {Type: "string", Description: "Origin IATA airport code (e.g. AMS, HEL)"},
				"destination": {Type: "string", Description: "Destination IATA airport code (e.g. PRG, BCN)"},
				"depart_date": {Type: "string", Description: "Departure date (YYYY-MM-DD)"},
				"return_date": {Type: "string", Description: "Return date (YYYY-MM-DD)"},
				"guests":      {Type: "integer", Description: "Number of guests (default: 1, must be >= 1)"},
				"currency":    {Type: "string", Description: "Target currency (e.g. EUR, USD)"},
			},
			Required: []string{"origin", "destination", "depart_date", "return_date"},
		},
		OutputSchema: planTripOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Plan Complete Trip",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func handlePlanTrip(args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	origin := strings.ToUpper(argString(args, "origin"))
	dest := strings.ToUpper(argString(args, "destination"))
	departDate := argString(args, "depart_date")
	returnDate := argString(args, "return_date")

	if origin == "" || dest == "" || departDate == "" || returnDate == "" {
		return nil, nil, fmt.Errorf("origin, destination, depart_date, and return_date are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	input := trip.PlanInput{
		Origin:      origin,
		Destination: dest,
		DepartDate:  departDate,
		ReturnDate:  returnDate,
		Guests:      argInt(args, "guests", 1),
		Currency:    argString(args, "currency"),
	}

	result, err := trip.PlanTrip(ctx, input)
	if err != nil {
		return []ContentBlock{{Type: "text", Text: fmt.Sprintf("Trip planning failed: %v", err)}}, nil, nil
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	if !result.Success && (len(result.OutboundFlights) > 0 || len(result.ReturnFlights) > 0 || len(result.Hotels) > 0) {
		return []ContentBlock{
			{Type: "text", Text: fmt.Sprintf("Partial trip plan: %s", result.Error)},
			{Type: "text", Text: string(data)},
		}, result, nil
	}
	return []ContentBlock{{Type: "text", Text: string(data)}}, result, nil
}
