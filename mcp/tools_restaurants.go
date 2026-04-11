package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// --- search_restaurants tool ---

func searchRestaurantsTool() ToolDef {
	return ToolDef{
		Name:        "search_restaurants",
		Title:       "Restaurant Search",
		Description: "Search for restaurants near a location using Google Maps. Returns name, rating, category, address, and distance. Free, no API key required.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"location": {Type: "string", Description: "City or place name (e.g., Helsinki, Barcelona, Shibuya Tokyo)"},
				"query":    {Type: "string", Description: "Optional cuisine or restaurant type filter (e.g., italian, sushi, pizza)"},
				"limit":    {Type: "integer", Description: "Maximum number of results (default: 10, max: 20)"},
			},
			Required: []string{"location"},
		},
		OutputSchema: restaurantSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Restaurant Search",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func restaurantSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":  map[string]interface{}{"type": "boolean"},
			"location": map[string]interface{}{"type": "string"},
			"count":    map[string]interface{}{"type": "integer"},
			"restaurants": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":        map[string]interface{}{"type": "string"},
						"rating":      map[string]interface{}{"type": "number"},
						"category":    map[string]interface{}{"type": "string"},
						"price_level": map[string]interface{}{"type": "integer"},
						"distance_m":  map[string]interface{}{"type": "integer"},
						"address":     map[string]interface{}{"type": "string"},
					},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

func handleSearchRestaurants(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	location := argString(args, "location")
	if location == "" {
		return nil, nil, fmt.Errorf("location is required")
	}

	query := argString(args, "query")
	if query == "" {
		query = "restaurants"
	} else {
		query = query + " restaurants"
	}

	limit := argInt(args, "limit", 10)
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	// Geocode the location name to coordinates.
	geo, err := destinations.Geocode(ctx, location)
	if err != nil {
		return nil, nil, fmt.Errorf("could not find location %q: %w", location, err)
	}

	// Search Google Maps for restaurants near those coordinates.
	places, err := destinations.SearchGoogleMapsPlaces(ctx, geo.Lat, geo.Lon, query, limit)
	if err != nil {
		result := map[string]interface{}{
			"success":     false,
			"location":    location,
			"count":       0,
			"restaurants": []interface{}{},
			"error":       err.Error(),
		}
		summary := fmt.Sprintf("Restaurant search failed for %s: %v", location, err)
		content, buildErr := buildAnnotatedContentBlocks(summary, result)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		return content, result, nil
	}

	result := map[string]interface{}{
		"success":     true,
		"location":    location,
		"count":       len(places),
		"restaurants": places,
	}

	summary := restaurantSummary(places, location)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func restaurantSummary(places []models.RatedPlace, location string) string {
	if len(places) == 0 {
		return fmt.Sprintf("No restaurants found in %s.", location)
	}

	parts := []string{
		fmt.Sprintf("Found %d restaurants in %s", len(places), location),
	}

	top := places[0]
	detail := fmt.Sprintf("Top: %s (%.1f rating)", top.Name, top.Rating)
	if top.Category != "" {
		detail += fmt.Sprintf(", %s", top.Category)
	}
	if top.Address != "" {
		detail += fmt.Sprintf(" - %s", top.Address)
	}
	parts = append(parts, detail)

	return strings.Join(parts, ". ") + "."
}
