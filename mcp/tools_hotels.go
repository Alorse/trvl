package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// --- Output schema builders ---

// hotelSearchOutputSchema returns the JSON Schema for HotelSearchResult.
func hotelSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{"type": "boolean"},
			"count":   map[string]interface{}{"type": "integer"},
			"hotels": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":         map[string]interface{}{"type": "string"},
						"hotel_id":     map[string]interface{}{"type": "string"},
						"rating":       map[string]interface{}{"type": "number"},
						"review_count": map[string]interface{}{"type": "integer"},
						"stars":        map[string]interface{}{"type": "integer"},
						"price":        map[string]interface{}{"type": "number"},
						"currency":     map[string]interface{}{"type": "string"},
						"address":      map[string]interface{}{"type": "string"},
						"lat":          map[string]interface{}{"type": "number"},
						"lon":          map[string]interface{}{"type": "number"},
						"amenities": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
			"suggestions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":      map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"params":      map[string]interface{}{"type": "object"},
					},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "count"},
	}
}

// hotelPricesOutputSchema returns the JSON Schema for HotelPriceResult.
func hotelPricesOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":   map[string]interface{}{"type": "boolean"},
			"hotel_id":  map[string]interface{}{"type": "string"},
			"name":      map[string]interface{}{"type": "string"},
			"check_in":  map[string]interface{}{"type": "string"},
			"check_out": map[string]interface{}{"type": "string"},
			"providers": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"provider": map[string]interface{}{"type": "string"},
						"price":    map[string]interface{}{"type": "number"},
						"currency": map[string]interface{}{"type": "string"},
					},
					"required": []string{"provider", "price", "currency"},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success"},
	}
}

// --- Tool definitions ---

func searchHotelsTool() ToolDef {
	return ToolDef{
		Name:        "search_hotels",
		Title:       "Search Hotels",
		Description: "Search hotels via Google Hotels. Returns real-time pricing, ratings, star levels, and amenities for a given location and dates.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"location":  {Type: "string", Description: "Location name or address (e.g., Helsinki, Tokyo, Manhattan New York)"},
				"check_in":  {Type: "string", Description: "Check-in date in YYYY-MM-DD format"},
				"check_out": {Type: "string", Description: "Check-out date in YYYY-MM-DD format"},
				"guests":    {Type: "integer", Description: "Number of guests (default: 2)"},
				"stars":     {Type: "integer", Description: "Minimum star rating 1-5 (default: no filter)"},
				"sort":      {Type: "string", Description: "Sort order: relevance, price, rating, or distance (default: relevance)"},
			},
			Required: []string{"location", "check_in", "check_out"},
		},
		OutputSchema: hotelSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Search Hotels",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func hotelPricesTool() ToolDef {
	return ToolDef{
		Name:        "hotel_prices",
		Title:       "Hotel Prices Comparison",
		Description: "Get prices from multiple booking providers for a specific hotel. Compares prices across providers like Booking.com, Hotels.com, Expedia, etc.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"hotel_id":  {Type: "string", Description: "Google Hotels property ID (from search_hotels results)"},
				"check_in":  {Type: "string", Description: "Check-in date in YYYY-MM-DD format"},
				"check_out": {Type: "string", Description: "Check-out date in YYYY-MM-DD format"},
				"currency":  {Type: "string", Description: "Currency code (e.g. USD, EUR). Default: USD"},
			},
			Required: []string{"hotel_id", "check_in", "check_out"},
		},
		OutputSchema: hotelPricesOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Hotel Prices Comparison",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

// --- Elicitation schemas ---

// hotelElicitationSchema returns the schema for refining hotel search results.
func hotelElicitationSchema(count int, location string) map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"min_stars": map[string]interface{}{
				"type":    "integer",
				"title":   "Minimum Stars",
				"minimum": 1,
				"maximum": 5,
				"default": 3,
			},
			"max_price": map[string]interface{}{
				"type":        "number",
				"title":       "Maximum Price Per Night",
				"description": "In local currency",
			},
			"sort_by": map[string]interface{}{
				"type":    "string",
				"title":   "Sort By",
				"enum":    []string{"price", "rating", "distance"},
				"default": "price",
			},
		},
	}
}

// --- Tool handlers ---

func handleSearchHotels(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error) {
	location := argString(args, "location")
	checkIn := argString(args, "check_in")
	checkOut := argString(args, "check_out")

	if location == "" || checkIn == "" || checkOut == "" {
		return nil, nil, fmt.Errorf("location, check_in, and check_out are required")
	}

	// Validate dates.
	if err := models.ValidateDateRange(checkIn, checkOut); err != nil {
		return nil, nil, err
	}

	opts := hotels.HotelSearchOptions{
		CheckIn:  checkIn,
		CheckOut: checkOut,
		Guests:   argInt(args, "guests", 2),
		Stars:    argInt(args, "stars", 0),
		Sort:     argString(args, "sort"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := hotels.SearchHotels(ctx, location, opts)
	if err != nil {
		return nil, nil, err
	}

	// If many results and client supports elicitation, offer to refine.
	if result.Success && result.Count > 10 && elicit != nil && opts.Stars == 0 {
		schema := hotelElicitationSchema(result.Count, location)
		msg := fmt.Sprintf("Found %d hotels in %s. Would you like to refine?", result.Count, location)
		refinement, elicitErr := elicit(msg, schema)
		if elicitErr == nil && refinement != nil {
			// Re-search with refined parameters.
			if stars, ok := refinement["min_stars"].(float64); ok && stars > 0 {
				opts.Stars = int(stars)
			}
			if sort, ok := refinement["sort_by"].(string); ok && sort != "" {
				opts.Sort = sort
			}
			// Re-run the search with refined options.
			result, err = hotels.SearchHotels(ctx, location, opts)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// Build suggestions for progressive disclosure.
	suggestions := hotelSuggestions(result, opts)

	type hotelResponse struct {
		*models.HotelSearchResult
		Suggestions []Suggestion `json:"suggestions,omitempty"`
	}
	resp := hotelResponse{
		HotelSearchResult: result,
		Suggestions:       suggestions,
	}

	content, err := buildAnnotatedContentBlocks(hotelSummary(result, location), resp)
	if err != nil {
		return nil, nil, err
	}

	return content, resp, nil
}

func handleHotelPrices(args map[string]any, elicit ElicitFunc) ([]ContentBlock, interface{}, error) {
	hotelID := argString(args, "hotel_id")
	checkIn := argString(args, "check_in")
	checkOut := argString(args, "check_out")
	currency := argString(args, "currency")
	if currency == "" {
		currency = "USD"
	}

	if hotelID == "" || checkIn == "" || checkOut == "" {
		return nil, nil, fmt.Errorf("hotel_id, check_in, and check_out are required")
	}

	// Validate dates.
	if err := models.ValidateDateRange(checkIn, checkOut); err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := hotels.GetHotelPrices(ctx, hotelID, checkIn, checkOut, currency)
	if err != nil {
		return nil, nil, err
	}

	summary := fmt.Sprintf("Found %d booking providers for hotel %s (%s to %s).",
		len(result.Providers), hotelID, checkIn, checkOut)
	if len(result.Providers) > 0 {
		cheapest := result.Providers[0]
		for _, p := range result.Providers[1:] {
			if p.Price > 0 && p.Price < cheapest.Price {
				cheapest = p
			}
		}
		summary += fmt.Sprintf(" Cheapest: %s %.0f via %s.", cheapest.Currency, cheapest.Price, cheapest.Provider)
	}

	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

// --- Summary builders ---

func hotelSummary(result *models.HotelSearchResult, location string) string {
	if !result.Success || result.Count == 0 {
		if result.Error != "" {
			return fmt.Sprintf("Hotel search in %s failed: %s", location, result.Error)
		}
		return fmt.Sprintf("No hotels found in %s.", location)
	}

	summary := fmt.Sprintf("Found %d hotels in %s.", result.Count, location)

	// Find cheapest.
	var cheapest *models.HotelResult
	for i := range result.Hotels {
		if result.Hotels[i].Price > 0 {
			if cheapest == nil || result.Hotels[i].Price < cheapest.Price {
				cheapest = &result.Hotels[i]
			}
		}
	}
	if cheapest != nil {
		summary += fmt.Sprintf(" Cheapest: %s%.0f/night (%s).",
			cheapest.Currency, cheapest.Price, cheapest.Name)
	}

	// Find highest rated.
	var bestRated *models.HotelResult
	for i := range result.Hotels {
		if result.Hotels[i].Rating > 0 {
			if bestRated == nil || result.Hotels[i].Rating > bestRated.Rating {
				bestRated = &result.Hotels[i]
			}
		}
	}
	if bestRated != nil && (cheapest == nil || bestRated.Name != cheapest.Name) {
		summary += fmt.Sprintf(" Highest rated: %s (%.1f/5).", bestRated.Name, bestRated.Rating)
	}

	return summary
}

// --- Suggestion builders ---

func hotelSuggestions(result *models.HotelSearchResult, opts hotels.HotelSearchOptions) []Suggestion {
	var suggestions []Suggestion

	if !result.Success || result.Count == 0 {
		return nil
	}

	// If no star filter, suggest filtering.
	if opts.Stars == 0 {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_hotels",
			Description: "Filter to 4+ star hotels only",
			Params:      map[string]any{"stars": 4},
		})
	}

	// If a great-rated hotel is found, suggest getting detailed pricing.
	for _, h := range result.Hotels {
		if h.Rating >= 4.5 && h.HotelID != "" {
			suggestions = append(suggestions, Suggestion{
				Action:      "hotel_prices",
				Description: fmt.Sprintf("Get detailed pricing for %s (%.1f rating)", h.Name, h.Rating),
				Params:      map[string]any{"hotel_id": h.HotelID, "check_in": opts.CheckIn, "check_out": opts.CheckOut},
			})
			break // Only suggest the top one.
		}
	}

	// If results are expensive, suggest expanding search.
	expensiveCount := 0
	for _, h := range result.Hotels {
		if h.Price > 300 {
			expensiveCount++
		}
	}
	if expensiveCount > result.Count/2 {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_hotels",
			Description: "Try expanding the area or checking different dates for more affordable options",
		})
	}

	return suggestions
}
