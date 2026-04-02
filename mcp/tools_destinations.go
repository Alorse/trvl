package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
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
