package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/ground"
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
			"routes": map[string]interface{}{
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
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

func handleSearchGround(args map[string]any, elicit ElicitFunc, sampling SamplingFunc) ([]ContentBlock, interface{}, error) {
	from := argString(args, "from")
	to := argString(args, "to")
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

	// Build summary for user
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d bus/train routes from %s to %s on %s:\n\n", result.Count, from, to, date)

	limit := result.Count
	if limit > 10 {
		limit = 10
	}
	for i, r := range result.Routes[:limit] {
		price := fmt.Sprintf("%s %.2f", r.Currency, r.Price)
		if r.PriceMax > 0 && r.PriceMax != r.Price {
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
	if result.Count > 10 {
		fmt.Fprintf(&sb, "\n... and %d more routes", result.Count-10)
	}

	content := []ContentBlock{
		{Type: "text", Text: sb.String(), Annotations: &ContentAnnotation{Audience: []string{"user"}, Priority: 1.0}},
		{Type: "text", Text: "Structured data attached.", Annotations: &ContentAnnotation{Audience: []string{"assistant"}, Priority: 0.5}},
	}

	return content, result, nil
}

// safeTimeSlice extracts HH:MM from an ISO 8601 timestamp, or returns the raw string.
func safeTimeSlice(t string) string {
	if len(t) >= 16 {
		return t[11:16]
	}
	return t
}
