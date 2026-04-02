package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// registerTools adds all trvl tool definitions and handlers to the server.
func registerTools(s *Server) {
	s.tools = []ToolDef{
		searchFlightsTool(),
		searchDatesTool(),
		searchHotelsTool(),
		hotelPricesTool(),
		destinationInfoTool(),
		tripCostTool(),
		weekendGetawayTool(),
		suggestDatesTool(),
		optimizeMultiCityTool(),
	}
	s.handlers["search_flights"] = handleSearchFlights
	s.handlers["search_dates"] = handleSearchDates
	s.handlers["search_hotels"] = handleSearchHotels
	s.handlers["hotel_prices"] = handleHotelPrices
	s.handlers["destination_info"] = handleDestinationInfo
	s.handlers["calculate_trip_cost"] = handleTripCost
	s.handlers["weekend_getaway"] = handleWeekendGetaway
	s.handlers["suggest_dates"] = handleSuggestDates
	s.handlers["optimize_multi_city"] = handleOptimizeMultiCity
}

// --- Suggestion types ---

// Suggestion represents a follow-up action the user might take.
type Suggestion struct {
	Action      string         `json:"action"`
	Description string         `json:"description"`
	Params      map[string]any `json:"params,omitempty"`
}

// --- Helper: extract values from args ---

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func argInt(args map[string]any, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return def
		}
		return int(i)
	default:
		return def
	}
}

func argFloat(args map[string]any, key string, def float64) float64 {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return def
		}
		return f
	default:
		return def
	}
}

func argStringSlice(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	v, ok := args[key]
	if !ok {
		return nil
	}
	// Try string (comma-separated).
	if s, ok := v.(string); ok && s != "" {
		parts := strings.Split(s, ",")
		var result []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
	// Try []any (JSON array).
	if arr, ok := v.([]any); ok {
		var result []string
		for _, elem := range arr {
			if s, ok := elem.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func argBool(args map[string]any, key string, def bool) bool {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

// --- Content block builder ---

// buildAnnotatedContentBlocks creates a text summary block (for user) and a
// structured JSON block (for assistant), with content annotations per the
// 2025-11-25 spec.
func buildAnnotatedContentBlocks(summary string, data any) ([]ContentBlock, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	return []ContentBlock{
		{
			Type: "text",
			Text: summary,
			Annotations: &ContentAnnotation{
				Audience: []string{"user"},
				Priority: 1.0,
			},
		},
		{
			Type: "text",
			Text: string(jsonData),
			Annotations: &ContentAnnotation{
				Audience: []string{"assistant"},
				Priority: 0.5,
			},
		},
	}, nil
}
