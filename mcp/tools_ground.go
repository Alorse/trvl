package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/ground"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
)

func searchGroundTool() ToolDef {
	return ToolDef{
		Name:        "search_ground",
		Title:       "Ground Transport Search",
		Description: "Search bus and train connections between cities. Queries FlixBus and RegioJet in parallel. Free, no API key. Covers most European routes.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"from":      {Type: "string", Description: "Departure city name (e.g. Prague, Helsinki, Vienna)"},
				"to":        {Type: "string", Description: "Arrival city name"},
				"date":      {Type: "string", Description: "Departure date (YYYY-MM-DD)"},
				"currency":  {Type: "string", Description: "Price currency (default: EUR)"},
				"type":      {Type: "string", Description: "Filter: bus, train, or empty for all"},
				"max_price": {Type: "number", Description: "Maximum price filter (0 = no limit)"},
				"provider":  {Type: "string", Description: "Restrict to provider: flixbus, regiojet, or empty for all"},
			},
			Required: []string{"from", "to", "date"},
		},
		OutputSchema: groundSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Ground Transport Search",
			ReadOnlyHint:   true,
			OpenWorldHint:  true,
			IdempotentHint: true,
		},
	}
}

func groundSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{"type": "boolean"},
			"count":   map[string]interface{}{"type": "integer"},
			"routes":  groundRoutesOutputSchema(),
			"error":   map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

func groundRoutesOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider":         map[string]interface{}{"type": "string"},
				"type":             map[string]interface{}{"type": "string"},
				"price":            map[string]interface{}{"type": "number"},
				"price_max":        map[string]interface{}{"type": "number"},
				"currency":         map[string]interface{}{"type": "string"},
				"duration_minutes": map[string]interface{}{"type": "integer"},
				"transfers":        map[string]interface{}{"type": "integer"},
				"amenities":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"seats_left":       map[string]interface{}{"type": "integer"},
				"booking_url":      map[string]interface{}{"type": "string"},
			},
		},
	}
}

func searchAirportTransfersTool() ToolDef {
	return ToolDef{
		Name:        "search_airport_transfers",
		Title:       "Airport Transfer Search",
		Description: "Search airport-to-hotel or airport-to-city ground transport. Lists exact airport routing first, adds taxi fare estimates, then broader city-level providers.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"airport_code": {Type: "string", Description: "Arrival airport IATA code (e.g. CDG, LHR, FCO)"},
				"destination":  {Type: "string", Description: "Hotel, address, district, or city destination"},
				"date":         {Type: "string", Description: "Travel date (YYYY-MM-DD)"},
				"arrival_time": {Type: "string", Description: "Only include routes departing at or after this local time (HH:MM)"},
				"currency":     {Type: "string", Description: "Price currency (default: EUR)"},
				"type":         {Type: "string", Description: "Filter: bus, train, taxi, tram, metro, mixed, or empty for all"},
				"max_price":    {Type: "number", Description: "Maximum price filter (0 = no limit)"},
				"provider":     {Type: "string", Description: "Restrict to provider: transitous, taxi, flixbus, regiojet, eurostar, db, sncf, trainline"},
			},
			Required: []string{"airport_code", "destination", "date"},
		},
		OutputSchema: airportTransferOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Airport Transfer Search",
			ReadOnlyHint:   true,
			OpenWorldHint:  true,
			IdempotentHint: true,
		},
	}
}

func airportTransferOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":          map[string]interface{}{"type": "boolean"},
			"airport_code":     map[string]interface{}{"type": "string"},
			"airport":          map[string]interface{}{"type": "string"},
			"airport_city":     map[string]interface{}{"type": "string"},
			"destination":      map[string]interface{}{"type": "string"},
			"destination_city": map[string]interface{}{"type": "string"},
			"date":             map[string]interface{}{"type": "string"},
			"arrival_time":     map[string]interface{}{"type": "string"},
			"count":            map[string]interface{}{"type": "integer"},
			"exact_matches":    map[string]interface{}{"type": "integer"},
			"city_matches":     map[string]interface{}{"type": "integer"},
			"routes":           groundRoutesOutputSchema(),
			"error":            map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "airport_code", "airport", "airport_city", "destination", "date", "count", "exact_matches", "city_matches"},
	}
}

func handleSearchGround(args map[string]any, elicit ElicitFunc, sampling SamplingFunc) ([]ContentBlock, interface{}, error) {
	from := models.ResolveLocationName(argString(args, "from"))
	to := models.ResolveLocationName(argString(args, "to"))
	date := argString(args, "date")

	if from == "" || to == "" || date == "" {
		return nil, nil, fmt.Errorf("from, to, and date are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := ground.SearchOptions{
		Currency: argString(args, "currency"),
		MaxPrice: argFloat(args, "max_price", 0),
		Type:     argString(args, "type"),
	}
	if p := argString(args, "provider"); p != "" {
		opts.Providers = strings.Split(p, ",")
	}

	result, err := ground.SearchByName(ctx, from, to, date, opts)
	if err != nil {
		return []ContentBlock{{Type: "text", Text: fmt.Sprintf("Ground transport search failed: %v", err)}}, nil, nil
	}

	if !result.Success {
		msg := fmt.Sprintf("No bus/train routes found from %s to %s on %s", from, to, date)
		if result.Error != "" {
			msg += ": " + result.Error
		}
		return []ContentBlock{{Type: "text", Text: msg}}, result, nil
	}

	summary := buildGroundRouteSummary(
		fmt.Sprintf("Found %d bus/train routes from %s to %s on %s", result.Count, from, to, date),
		result.Routes,
	)

	content := []ContentBlock{
		{Type: "text", Text: summary, Annotations: &ContentAnnotation{Audience: []string{"user"}, Priority: 1.0}},
		{Type: "text", Text: "Structured data attached.", Annotations: &ContentAnnotation{Audience: []string{"assistant"}, Priority: 0.5}},
	}

	return content, result, nil
}

func handleSearchAirportTransfers(args map[string]any, elicit ElicitFunc, sampling SamplingFunc) ([]ContentBlock, interface{}, error) {
	input := trip.AirportTransferInput{
		AirportCode: argString(args, "airport_code"),
		Destination: argString(args, "destination"),
		Date:        argString(args, "date"),
		ArrivalTime: argString(args, "arrival_time"),
		Currency:    argString(args, "currency"),
		MaxPrice:    argFloat(args, "max_price", 0),
		Type:        argString(args, "type"),
	}
	if p := argString(args, "provider"); p != "" {
		input.Providers = strings.Split(p, ",")
	}
	if input.AirportCode == "" || input.Destination == "" || input.Date == "" {
		return nil, nil, fmt.Errorf("airport_code, destination, and date are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := trip.SearchAirportTransfers(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	if !result.Success {
		msg := fmt.Sprintf("No airport transfer routes found from %s to %s on %s", result.Airport, result.Destination, result.Date)
		if result.Error != "" {
			msg += ": " + result.Error
		}
		return []ContentBlock{{Type: "text", Text: msg}}, result, nil
	}

	summary := buildGroundRouteSummary(
		fmt.Sprintf("Found %d airport transfer routes from %s to %s on %s", result.Count, result.Airport, result.Destination, result.Date),
		result.Routes,
	)
	if result.ArrivalTime != "" {
		summary += fmt.Sprintf("\n\nFiltered to departures at or after %s.", result.ArrivalTime)
	}
	if result.CityMatches > 0 {
		summary += fmt.Sprintf("\n\nExact airport matches are listed first (%d exact, %d broader city).", result.ExactMatches, result.CityMatches)
	}
	if groundRoutesHaveProvider(result.Routes, "taxi") {
		summary += "\n\nTaxi fares are estimates based on route distance and typical local tariffs."
	}

	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}
	return content, result, nil
}

func groundRoutesHaveProvider(routes []models.GroundRoute, provider string) bool {
	for _, route := range routes {
		if strings.EqualFold(route.Provider, provider) {
			return true
		}
	}
	return false
}

func buildGroundRouteSummary(header string, routes []models.GroundRoute) string {
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString(":\n\n")

	limit := len(routes)
	if limit > 10 {
		limit = 10
	}
	for i, r := range routes[:limit] {
		price := fmt.Sprintf("%s %.2f", r.Currency, r.Price)
		if r.Price <= 0 {
			price = "price unavailable"
		} else if r.PriceMax > 0 && r.PriceMax != r.Price {
			price = fmt.Sprintf("%s %.2f-%.2f", r.Currency, r.Price, r.PriceMax)
		}
		dur := fmt.Sprintf("%dh%02dm", r.Duration/60, r.Duration%60)
		transfers := "direct"
		if r.Transfers > 0 {
			transfers = fmt.Sprintf("%d transfers", r.Transfers)
		}

		depTime := safeTimeSlice(r.Departure.Time)
		arrTime := safeTimeSlice(r.Arrival.Time)

		fmt.Fprintf(&sb, "%d. **%s** %s | %s | %s | %s %s→%s",
			i+1, price, r.Type, dur, transfers, r.Provider, depTime, arrTime)

		if r.SeatsLeft != nil && *r.SeatsLeft <= 10 {
			fmt.Fprintf(&sb, " | %d seats left", *r.SeatsLeft)
		}
		if len(r.Amenities) > 0 {
			fmt.Fprintf(&sb, " | %s", strings.Join(r.Amenities, ", "))
		}
		sb.WriteString("\n")
	}
	if len(routes) > 10 {
		fmt.Fprintf(&sb, "\n... and %d more routes", len(routes)-10)
	}
	return sb.String()
}

// safeTimeSlice extracts HH:MM from an ISO 8601 timestamp, or returns the raw string.
func safeTimeSlice(t string) string {
	if len(t) >= 16 {
		return t[11:16]
	}
	return t
}
