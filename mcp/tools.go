package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// registerTools adds all trvl tool definitions and handlers to the server.
// Handlers are wrapped in closures to give them access to the server for
// recording searches and adding resource_link content blocks.
func registerTools(s *Server) {
	s.tools = []ToolDef{
		searchFlightsTool(),
		searchDatesTool(),
		searchHotelsTool(),
		hotelPricesTool(),
		hotelReviewsTool(),
		destinationInfoTool(),
		tripCostTool(),
		weekendGetawayTool(),
		suggestDatesTool(),
		optimizeMultiCityTool(),
		nearbyPlacesTool(),
		travelGuideTool(),
		localEventsTool(),
		searchGroundTool(),
		searchAirportTransfersTool(),
		searchRestaurantsTool(),
		searchDealsTool(),
		planTripTool(),
	}
	s.handlers["search_flights"] = s.wrapHandler(handleSearchFlights)
	s.handlers["search_dates"] = s.wrapHandler(handleSearchDates)
	s.handlers["search_hotels"] = s.wrapHandler(handleSearchHotels)
	s.handlers["hotel_prices"] = s.wrapHandler(handleHotelPrices)
	s.handlers["hotel_reviews"] = s.wrapHandler(handleHotelReviews)
	s.handlers["destination_info"] = s.wrapHandler(handleDestinationInfo)
	s.handlers["calculate_trip_cost"] = s.wrapHandler(handleTripCost)
	s.handlers["weekend_getaway"] = s.wrapHandler(handleWeekendGetaway)
	s.handlers["suggest_dates"] = s.wrapHandler(handleSuggestDates)
	s.handlers["optimize_multi_city"] = s.wrapHandler(handleOptimizeMultiCity)
	s.handlers["nearby_places"] = s.wrapHandler(handleNearbyPlaces)
	s.handlers["travel_guide"] = s.wrapHandler(handleTravelGuide)
	s.handlers["local_events"] = s.wrapHandler(handleLocalEvents)
	s.handlers["search_ground"] = s.wrapHandler(handleSearchGround)
	s.handlers["search_airport_transfers"] = s.wrapHandler(handleSearchAirportTransfers)
	s.handlers["search_restaurants"] = s.wrapHandler(handleSearchRestaurants)
	s.handlers["search_deals"] = s.wrapHandler(handleSearchDeals)
	s.handlers["plan_trip"] = s.wrapHandler(handlePlanTrip)
}

// wrapHandler returns a ToolHandler that delegates to the inner handler and
// then post-processes the result to add resource_link blocks and record the
// search in trip state.
func (s *Server) wrapHandler(inner ToolHandler) ToolHandler {
	return func(args map[string]any, elicit ElicitFunc, sampling SamplingFunc) ([]ContentBlock, interface{}, error) {
		content, structured, err := inner(args, elicit, sampling)
		if err != nil {
			return content, structured, err
		}

		// Post-process: add resource links and record searches based on args.
		content = s.addResourceLinks(content, args)
		s.recordSearchFromArgs(args, structured)

		return content, structured, nil
	}
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

// --- Resource link and search recording ---

// addResourceLinks inspects the tool arguments and appends a resource_link
// content block so the user can re-fetch updated prices later.
func (s *Server) addResourceLinks(content []ContentBlock, args map[string]any) []ContentBlock {
	origin := strings.ToUpper(argString(args, "origin"))
	dest := strings.ToUpper(argString(args, "destination"))
	date := argString(args, "departure_date")

	// Flight search: resource link for price watch.
	if origin != "" && dest != "" && date != "" {
		content = append(content, ContentBlock{
			Type:        "resource_link",
			URI:         fmt.Sprintf("trvl://watch/%s-%s-%s", origin, dest, date),
			Name:        fmt.Sprintf("%s->%s flight prices", origin, dest),
			Description: "Re-fetch to check for price changes",
		})
	}

	// Hotel search: resource link referencing the location.
	location := argString(args, "location")
	checkIn := argString(args, "check_in")
	checkOut := argString(args, "check_out")
	if location != "" && checkIn != "" && checkOut != "" {
		// Sanitize location for URI (replace spaces with underscores).
		safeLocation := strings.ReplaceAll(strings.TrimSpace(location), " ", "_")
		content = append(content, ContentBlock{
			Type:        "resource_link",
			URI:         fmt.Sprintf("trvl://search/hotels/%s-%s-%s", safeLocation, checkIn, checkOut),
			Name:        fmt.Sprintf("%s hotel prices", location),
			Description: "Re-fetch to check for price changes",
		})
	}

	return content
}

// recordSearchFromArgs inspects the structured result and args to record the
// search in the trip state for the session summary resource.
func (s *Server) recordSearchFromArgs(args map[string]any, structured interface{}) {
	if structured == nil || args == nil {
		return
	}

	// Try to extract common fields via JSON round-trip.
	data, err := json.Marshal(structured)
	if err != nil {
		return
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}

	// Determine search type and extract best price.
	origin := strings.ToUpper(argString(args, "origin"))
	dest := strings.ToUpper(argString(args, "destination"))
	date := argString(args, "departure_date")
	location := argString(args, "location")

	switch {
	case origin != "" && dest != "" && date != "":
		// Flight search.
		bestPrice, currency := extractBestFlightPrice(m)
		retDate := argString(args, "return_date")
		query := fmt.Sprintf("%s->%s %s", origin, dest, date)
		if retDate != "" {
			query += fmt.Sprintf(" (round-trip return %s)", retDate)
		}
		s.recordSearch("flight", query, bestPrice, currency)

		// Cache the price for watch resources.
		if bestPrice > 0 {
			cacheKey := fmt.Sprintf("%s-%s-%s", origin, dest, date)
			s.priceCache.set(cacheKey, bestPrice)
		}

	case location != "":
		// Hotel or destination search.
		checkIn := argString(args, "check_in")
		checkOut := argString(args, "check_out")
		if checkIn != "" && checkOut != "" {
			// Hotel search.
			bestPrice, currency := extractBestHotelPrice(m)
			query := fmt.Sprintf("%s %s to %s", location, checkIn, checkOut)
			s.recordSearch("hotel", query, bestPrice, currency)
		} else {
			// Destination info.
			query := location
			s.recordSearch("destination", query, 0, "")
		}
	}
}

// extractBestFlightPrice extracts the cheapest flight price from a structured result.
func extractBestFlightPrice(m map[string]interface{}) (float64, string) {
	flightsRaw, ok := m["flights"]
	if !ok {
		return 0, ""
	}
	flightsList, ok := flightsRaw.([]interface{})
	if !ok || len(flightsList) == 0 {
		return 0, ""
	}
	var best float64
	var currency string
	for _, f := range flightsList {
		fm, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		price, _ := fm["price"].(float64)
		if price > 0 && (best == 0 || price < best) {
			best = price
			if c, ok := fm["currency"].(string); ok {
				currency = c
			}
		}
	}
	return best, currency
}

// extractBestHotelPrice extracts the cheapest hotel price from a structured result.
func extractBestHotelPrice(m map[string]interface{}) (float64, string) {
	hotelsRaw, ok := m["hotels"]
	if !ok {
		return 0, ""
	}
	hotelsList, ok := hotelsRaw.([]interface{})
	if !ok || len(hotelsList) == 0 {
		return 0, ""
	}
	var best float64
	var currency string
	for _, h := range hotelsList {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		price, _ := hm["price"].(float64)
		if price > 0 && (best == 0 || price < best) {
			best = price
			if c, ok := hm["currency"].(string); ok {
				currency = c
			}
		}
	}
	return best, currency
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
