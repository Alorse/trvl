package mcp

import (
	"encoding/json"
	"fmt"
)

// registerTools adds all trvl tool definitions and handlers to the server.
func registerTools(s *Server) {
	s.tools = []ToolDef{
		searchFlightsTool(),
		searchDatesTool(),
		searchHotelsTool(),
		hotelPricesTool(),
		destinationInfoTool(),
	}
	s.handlers["search_flights"] = handleSearchFlights
	s.handlers["search_dates"] = handleSearchDates
	s.handlers["search_hotels"] = handleSearchHotels
	s.handlers["hotel_prices"] = handleHotelPrices
	s.handlers["destination_info"] = handleDestinationInfo
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
