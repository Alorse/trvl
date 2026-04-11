package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/lounges"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// --- Output schema ---

func loungeSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{"type": "boolean"},
			"airport": map[string]interface{}{"type": "string"},
			"count":   map[string]interface{}{"type": "integer"},
			"lounges": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":     map[string]interface{}{"type": "string"},
						"airport":  map[string]interface{}{"type": "string"},
						"terminal": map[string]interface{}{"type": "string"},
						"cards": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
						"amenities": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
						"open_hours": map[string]interface{}{"type": "string"},
						"accessible_with": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "Subset of the user's own lounge cards that grant free entry",
						},
					},
					"required": []string{"name", "airport"},
				},
			},
			"source": map[string]interface{}{"type": "string"},
			"error":  map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

// --- Tool definition ---

func searchLoungesTool() ToolDef {
	return ToolDef{
		Name:  "search_lounges",
		Title: "Search Airport Lounges",
		Description: "Find airport lounges at a given airport. Returns name, terminal, " +
			"accepted access cards (Priority Pass, Diners Club, LoungeKey, etc.), amenities, " +
			"and opening hours. Results are annotated with the user's own lounge cards " +
			"(from preferences) so you can tell the user exactly which lounges they can enter for free.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"airport": {
					Type:        "string",
					Description: "Airport IATA code (e.g. HEL, LHR, JFK)",
				},
			},
			Required: []string{"airport"},
		},
		OutputSchema: loungeSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Search Airport Lounges",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

// --- Handler ---

func handleSearchLounges(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	airport := strings.ToUpper(strings.TrimSpace(argString(args, "airport")))

	if airport == "" {
		return nil, nil, fmt.Errorf("airport is required")
	}
	if err := models.ValidateIATA(airport); err != nil {
		return nil, nil, fmt.Errorf("invalid airport: %w", err)
	}

	result, err := lounges.SearchLounges(ctx, airport)
	if err != nil {
		return nil, nil, fmt.Errorf("lounge search: %w", err)
	}

	// Annotate with user's lounge cards from preferences.
	prefs, _ := preferences.Load()
	if prefs != nil && len(prefs.LoungeCards) > 0 {
		lounges.AnnotateAccess(result, prefs.LoungeCards)
	}

	summary := loungeSummary(result, prefs)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

// loungeSummary builds a human-readable summary of the lounge search result.
func loungeSummary(result *lounges.SearchResult, prefs *preferences.Preferences) string {
	if !result.Success {
		if result.Error != "" {
			return fmt.Sprintf("Lounge search for %s failed: %s", result.Airport, result.Error)
		}
		return fmt.Sprintf("No lounge information available for %s.", result.Airport)
	}

	if result.Count == 0 {
		return fmt.Sprintf("No lounges found at %s in our database.", result.Airport)
	}

	summary := fmt.Sprintf("Found %d lounge(s) at %s.", result.Count, result.Airport)

	// If user has lounge cards, count which lounges they can access.
	if prefs != nil && len(prefs.LoungeCards) > 0 {
		accessible := 0
		for _, l := range result.Lounges {
			if len(l.AccessibleWith) > 0 {
				accessible++
			}
		}
		if accessible > 0 {
			cardNames := strings.Join(prefs.LoungeCards, ", ")
			summary += fmt.Sprintf(" You have free access to %d lounge(s) with your card(s): %s.", accessible, cardNames)
		} else {
			summary += " None of these lounges accept your current lounge cards."
		}
	}

	return summary
}
