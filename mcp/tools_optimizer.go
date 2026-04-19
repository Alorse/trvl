package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/optimizer"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

func optimizeBookingTool() ToolDef {
	return ToolDef{
		Name:  "optimize_booking",
		Title: "Optimize Booking",
		Description: "Find the cheapest way to book a trip by searching alternative origins, " +
			"destinations, rail+fly stations, and applying all-in cost (baggage + FF status). " +
			"Returns ranked booking strategies with savings vs naive direct booking.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":          {Type: "string", Description: "Origin IATA airport code (e.g. HEL)"},
				"destination":     {Type: "string", Description: "Destination IATA airport code or city (e.g. BCN)"},
				"departure_date":  {Type: "string", Description: "Departure date (YYYY-MM-DD)"},
				"return_date":     {Type: "string", Description: "Return date (YYYY-MM-DD); omit for one-way"},
				"flex_days":       {Type: "integer", Description: "Date flexibility +/-N days (default 3)"},
				"guests":          {Type: "integer", Description: "Number of passengers (default 1)"},
				"currency":        {Type: "string", Description: "Display currency (default: EUR)"},
				"max_results":     {Type: "integer", Description: "Top N results to return (default 5)"},
				"max_api_calls":   {Type: "integer", Description: "API call budget (default 15)"},
				"need_checked_bag": {Type: "boolean", Description: "Whether a checked bag is needed"},
				"carry_on_only":   {Type: "boolean", Description: "Carry-on only trip"},
			},
			Required: []string{"origin", "destination", "departure_date"},
		},
		OutputSchema: optimizeBookingOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Optimize Booking",
			ReadOnlyHint:   true,
			OpenWorldHint:  true,
			IdempotentHint: true,
		},
	}
}

func optimizeBookingOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": schemaBool(),
			"error":   schemaString(),
			"options": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"rank":                schemaInt(),
					"strategy":            schemaString(),
					"base_cost":           schemaNum(),
					"bag_cost":            schemaNum(),
					"ff_savings":          schemaNum(),
					"transfer_cost":       schemaNum(),
					"all_in_cost":         schemaNum(),
					"currency":            schemaString(),
					"savings_vs_baseline": schemaNum(),
					"hacks_applied":       schemaStringArray(),
					"legs": schemaArray(map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"type":         schemaString(),
							"from":         schemaString(),
							"to":           schemaString(),
							"date":         schemaString(),
							"price":        schemaNum(),
							"currency":     schemaString(),
							"airline":      schemaString(),
							"duration_min": schemaInt(),
							"notes":        schemaString(),
						},
					}),
				},
			}),
			"baseline": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"all_in_cost": schemaNum(),
					"currency":    schemaString(),
				},
			},
		},
		"required": []string{"success"},
	}
}

func handleOptimizeBooking(ctx context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	input := optimizer.OptimizeInput{
		Origin:         strings.ToUpper(argString(args, "origin")),
		Destination:    strings.ToUpper(argString(args, "destination")),
		DepartDate:     argString(args, "departure_date"),
		ReturnDate:     argString(args, "return_date"),
		FlexDays:       argInt(args, "flex_days", 3),
		Guests:         argInt(args, "guests", 1),
		Currency:       argString(args, "currency"),
		MaxResults:     argInt(args, "max_results", 5),
		MaxAPICalls:    argInt(args, "max_api_calls", 15),
		NeedCheckedBag: argBool(args, "need_checked_bag", false),
		CarryOnOnly:    argBool(args, "carry_on_only", false),
	}

	// Load user preferences for FF statuses and home airports.
	if prefs, err := preferences.Load(); err == nil && prefs != nil {
		for _, ff := range prefs.FrequentFlyerPrograms {
			input.FFStatuses = append(input.FFStatuses, optimizer.FFStatus{
				Alliance: ff.Alliance,
				Tier:     ff.Tier,
			})
		}
		input.HomeAirports = prefs.HomeAirports
		if !input.CarryOnOnly && prefs.CarryOnOnly {
			input.CarryOnOnly = true
		}
	}

	if progress != nil {
		progress(0, 1, "Optimizing booking strategies...")
	}

	result, err := optimizer.Optimize(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("optimize_booking: %w", err)
	}

	// Build summary.
	var summary string
	if result.Success && len(result.Options) > 0 {
		best := result.Options[0]
		summary = fmt.Sprintf("Best: %s — %.0f %s all-in", best.Strategy, best.AllInCost, best.Currency)
		if best.SavingsVsBaseline > 0 {
			summary += fmt.Sprintf(" (saves %.0f %s vs direct)", best.SavingsVsBaseline, best.Currency)
		}
		summary += fmt.Sprintf(" | %d options found", len(result.Options))
	} else {
		summary = "No booking optimizations found"
		if result.Error != "" {
			summary += ": " + result.Error
		}
	}

	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}
	return content, result, nil
}
