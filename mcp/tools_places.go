package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// --- nearby_places tool ---

func nearbyPlacesTool() ToolDef {
	return ToolDef{
		Name:        "nearby_places",
		Title:       "Nearby Places",
		Description: "Find nearby points of interest (restaurants, cafes, attractions, pharmacies, etc.) from OpenStreetMap. Uses Google Maps ratings when no Foursquare key is set. Optionally enriched with Foursquare ratings, Geoapify walking data, and OpenTripMap attractions when API keys are set.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"lat":      {Type: "number", Description: "Latitude of the center point"},
				"lon":      {Type: "number", Description: "Longitude of the center point"},
				"radius_m": {Type: "integer", Description: "Search radius in meters (default: 500, max: 5000)"},
				"category": {Type: "string", Description: "POI category: restaurant, cafe, bar, pharmacy, atm, bank, supermarket, hospital, museum, attraction, all (default: all)"},
			},
			Required: []string{"lat", "lon"},
		},
		OutputSchema: nearbyPlacesOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Nearby Places",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func nearbyPlacesOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pois": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":          schemaString(),
					"type":          schemaString(),
					"lat":           schemaNum(),
					"lon":           schemaNum(),
					"distance_m":    schemaInt(),
					"cuisine":       schemaString(),
					"opening_hours": schemaString(),
					"phone":         schemaString(),
					"website":       schemaString(),
				},
			}),
			"rated_places": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":        schemaString(),
					"rating":      schemaNum(),
					"category":    schemaString(),
					"price_level": schemaInt(),
					"distance_m":  schemaInt(),
					"address":     schemaString(),
					"tip":         schemaString(),
				},
			}),
			"attractions": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":          schemaString(),
					"kind":          schemaString(),
					"distance_m":    schemaInt(),
					"wikipedia_url": schemaString(),
				},
			}),
		},
	}
}

func handleNearbyPlaces(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	lat := argFloat(args, "lat", 0)
	lon := argFloat(args, "lon", 0)
	if lat == 0 && lon == 0 {
		return nil, nil, fmt.Errorf("lat and lon are required")
	}

	radius := argInt(args, "radius_m", 500)
	category := argString(args, "category")
	if category == "" {
		category = "all"
	}

	result, err := destinations.GetNearbyPlaces(ctx, lat, lon, radius, category)
	if err != nil {
		return nil, nil, err
	}

	summary := nearbyPlacesSummary(result, category)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func nearbyPlacesSummary(result *destinations.NearbyResult, category string) string {
	parts := []string{fmt.Sprintf("Found %d nearby places", len(result.POIs))}

	if category != "all" && category != "" {
		parts[0] += fmt.Sprintf(" (%s)", category)
	}

	if len(result.POIs) > 0 {
		closest := result.POIs[0]
		parts = append(parts, fmt.Sprintf("Closest: %s (%s, %dm away)", closest.Name, closest.Type, closest.Distance))
	}

	if len(result.RatedPlaces) > 0 {
		top := result.RatedPlaces[0]
		parts = append(parts, fmt.Sprintf("Top rated: %s (%.1f/10)", top.Name, top.Rating))
	}

	if len(result.Attractions) > 0 {
		parts = append(parts, fmt.Sprintf("%d tourist attractions nearby", len(result.Attractions)))
	}

	return strings.Join(parts, ". ") + "."
}

// --- travel_guide tool ---

func travelGuideTool() ToolDef {
	return ToolDef{
		Name:        "travel_guide",
		Title:       "Travel Guide",
		Description: "Get a Wikivoyage travel guide for any destination with structured sections (Get in, See, Do, Eat, Drink, Sleep, Stay safe). Free, no API key required.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"location": {Type: "string", Description: "City or destination name (e.g., Barcelona, Tokyo, New York)"},
			},
			Required: []string{"location"},
		},
		OutputSchema: travelGuideOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Travel Guide",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func travelGuideOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": schemaString(),
			"summary":  schemaString(),
			"sections": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": schemaString(),
			},
			"url": schemaString(),
		},
		"required": []string{"location"},
	}
}

func handleTravelGuide(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	location := argString(args, "location")
	if location == "" {
		return nil, nil, fmt.Errorf("location is required")
	}

	guide, err := destinations.GetWikivoyageGuide(ctx, location)
	if err != nil {
		return nil, nil, err
	}

	summary := travelGuideSummary(guide)
	content, err := buildAnnotatedContentBlocks(summary, guide)
	if err != nil {
		return nil, nil, err
	}

	return content, guide, nil
}

func travelGuideSummary(guide *models.WikivoyageGuide) string {
	parts := []string{fmt.Sprintf("Travel guide: %s", guide.Location)}

	if guide.Summary != "" {
		// Truncate summary to first 200 chars for the text block.
		s := guide.Summary
		if len(s) > 200 {
			s = s[:200] + "..."
		}
		parts = append(parts, s)
	}

	sectionNames := make([]string, 0, len(guide.Sections))
	for name := range guide.Sections {
		sectionNames = append(sectionNames, name)
	}
	if len(sectionNames) > 0 {
		parts = append(parts, fmt.Sprintf("Sections: %s", strings.Join(sectionNames, ", ")))
	}

	parts = append(parts, fmt.Sprintf("Full guide: %s", guide.URL))

	return strings.Join(parts, ". ") + "."
}

// --- local_events tool ---

func localEventsTool() ToolDef {
	return ToolDef{
		Name:        "local_events",
		Title:       "Local Events",
		Description: "Find local events at a destination during your travel dates. Requires TICKETMASTER_API_KEY environment variable (free at developer.ticketmaster.com).",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"location":   {Type: "string", Description: "City name (e.g., Barcelona, Tokyo, New York)"},
				"start_date": {Type: "string", Description: "Start date in YYYY-MM-DD format"},
				"end_date":   {Type: "string", Description: "End date in YYYY-MM-DD format"},
			},
			Required: []string{"location", "start_date", "end_date"},
		},
		OutputSchema: localEventsOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Local Events",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func localEventsOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"events": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":        schemaString(),
					"date":        schemaString(),
					"time":        schemaString(),
					"venue":       schemaString(),
					"type":        schemaString(),
					"url":         schemaString(),
					"price_range": schemaString(),
				},
			}),
			"message": schemaString(),
		},
	}
}

func handleLocalEvents(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	location := argString(args, "location")
	startDate := argString(args, "start_date")
	endDate := argString(args, "end_date")

	if location == "" {
		return nil, nil, fmt.Errorf("location is required")
	}
	if startDate == "" || endDate == "" {
		return nil, nil, fmt.Errorf("start_date and end_date are required")
	}

	// Check if Ticketmaster key is set.
	if os.Getenv("TICKETMASTER_API_KEY") == "" {
		result := map[string]interface{}{
			"events":  []models.Event{},
			"message": "Set TICKETMASTER_API_KEY for event listings (free at developer.ticketmaster.com)",
		}
		summary := "Event search unavailable. Set TICKETMASTER_API_KEY for event listings (free at developer.ticketmaster.com)."
		content, err := buildAnnotatedContentBlocks(summary, result)
		if err != nil {
			return nil, nil, err
		}
		return content, result, nil
	}

	events, err := destinations.GetEvents(ctx, location, startDate, endDate)
	if err != nil {
		return nil, nil, err
	}

	result := map[string]interface{}{
		"events": events,
	}

	summary := localEventsSummary(events, location, startDate, endDate)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func localEventsSummary(events []models.Event, location, startDate, endDate string) string {
	if len(events) == 0 {
		return fmt.Sprintf("No events found in %s from %s to %s.", location, startDate, endDate)
	}

	parts := []string{
		fmt.Sprintf("Found %d events in %s from %s to %s", len(events), location, startDate, endDate),
	}

	// Show first event.
	e := events[0]
	detail := fmt.Sprintf("Next: %s at %s on %s", e.Name, e.Venue, e.Date)
	if e.PriceRange != "" {
		detail += fmt.Sprintf(" (%s)", e.PriceRange)
	}
	parts = append(parts, detail)

	return strings.Join(parts, ". ") + "."
}
