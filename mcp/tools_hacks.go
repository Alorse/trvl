package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/hacks"
)

// detectTravelHacksTool returns the MCP tool definition for hack detection.
func detectTravelHacksTool() ToolDef {
	return ToolDef{
		Name:        "detect_travel_hacks",
		Title:       "Detect Travel Optimization Hacks",
		Description: "Automatically detect money-saving travel hacks for a route: throwaway ticketing, hidden city, positioning flights, split ticketing, overnight transport (saved hotel night), airline stopover programs, and date flexibility.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":       {Type: "string", Description: "Origin IATA airport code (e.g. HEL)"},
				"destination":  {Type: "string", Description: "Destination IATA airport code (e.g. PRG)"},
				"date":         {Type: "string", Description: "Departure date (YYYY-MM-DD)"},
				"return_date":  {Type: "string", Description: "Return date for round-trip analysis (YYYY-MM-DD); enables split and throwaway checks"},
				"currency":     {Type: "string", Description: "Display currency (default: EUR)"},
				"carry_on":     {Type: "boolean", Description: "Carry-on only trip — enables hidden city suggestions"},
				"naive_price":  {Type: "number", Description: "Known baseline one-way price for comparison (optional)"},
			},
			Required: []string{"origin", "destination", "date"},
		},
		OutputSchema: hacksOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Detect Travel Optimization Hacks",
			ReadOnlyHint:   true,
			OpenWorldHint:  true,
			IdempotentHint: true,
		},
	}
}

func hacksOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"origin":      map[string]interface{}{"type": "string"},
			"destination": map[string]interface{}{"type": "string"},
			"date":        map[string]interface{}{"type": "string"},
			"count":       map[string]interface{}{"type": "integer"},
			"hacks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type":        map[string]interface{}{"type": "string"},
						"title":       map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"savings":     map[string]interface{}{"type": "number"},
						"currency":    map[string]interface{}{"type": "string"},
						"risks":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"steps":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"citations":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					},
				},
			},
		},
		"required": []string{"origin", "destination", "date", "count", "hacks"},
	}
}

func handleDetectTravelHacks(args map[string]any, _ ElicitFunc, _ SamplingFunc) ([]ContentBlock, interface{}, error) {
	origin := strings.ToUpper(argString(args, "origin"))
	destination := strings.ToUpper(argString(args, "destination"))
	date := argString(args, "date")
	returnDate := argString(args, "return_date")
	currency := argString(args, "currency")
	if currency == "" {
		currency = "EUR"
	}
	carryOn := argBool(args, "carry_on", false)
	naivePrice := argFloat(args, "naive_price", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	input := hacks.DetectorInput{
		Origin:      origin,
		Destination: destination,
		Date:        date,
		ReturnDate:  returnDate,
		Currency:    currency,
		CarryOnOnly: carryOn,
		NaivePrice:  naivePrice,
	}

	detected := hacks.DetectAll(ctx, input)

	type response struct {
		Origin      string       `json:"origin"`
		Destination string       `json:"destination"`
		Date        string       `json:"date"`
		Count       int          `json:"count"`
		Hacks       []hacks.Hack `json:"hacks"`
	}

	resp := response{
		Origin:      origin,
		Destination: destination,
		Date:        date,
		Count:       len(detected),
		Hacks:       detected,
	}
	if resp.Hacks == nil {
		resp.Hacks = []hacks.Hack{}
	}

	summary := buildHacksSummary(origin, destination, date, detected)
	content := []ContentBlock{
		{Type: "text", Text: summary, Annotations: &ContentAnnotation{Audience: []string{"user"}, Priority: 1.0}},
		{Type: "text", Text: "Structured hack data attached.", Annotations: &ContentAnnotation{Audience: []string{"assistant"}, Priority: 0.5}},
	}
	return content, resp, nil
}

func buildHacksSummary(origin, destination, date string, detected []hacks.Hack) string {
	if len(detected) == 0 {
		return "No travel hacks detected for " + origin + "→" + destination + " on " + date + "."
	}
	var sb strings.Builder
	sb.WriteString("Travel hacks for " + origin + "→" + destination + " on " + date + ":\n\n")
	for i, h := range detected {
		sb.WriteString(fmt.Sprintf("%d. %s", i+1, h.Title))
		if h.Savings > 0 {
			sb.WriteString(fmt.Sprintf(" — saves %s %.0f", h.Currency, h.Savings))
		}
		sb.WriteString("\n")
		sb.WriteString("   " + h.Description + "\n\n")
	}
	return sb.String()
}

// detectAccommodationHacksTool returns the MCP tool definition for accommodation
// split detection.
func detectAccommodationHacksTool() ToolDef {
	return ToolDef{
		Name:        "detect_accommodation_hacks",
		Title:       "Detect Accommodation Split Opportunities",
		Description: "Find cheaper hotel stays by splitting a long booking across 2-3 properties. Accounts for moving costs (EUR 15/move) and only reports splits saving at least EUR 50 and 15% vs a single booking.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"city":       {Type: "string", Description: "City name (e.g. Prague, Amsterdam)"},
				"checkin":    {Type: "string", Description: "Check-in date (YYYY-MM-DD)"},
				"checkout":   {Type: "string", Description: "Check-out date (YYYY-MM-DD)"},
				"currency":   {Type: "string", Description: "Display currency (default: EUR)"},
				"max_splits": {Type: "integer", Description: "Maximum properties to split across, 2 or 3 (default: 3)"},
				"guests":     {Type: "integer", Description: "Number of guests (default: 2)"},
			},
			Required: []string{"city", "checkin", "checkout"},
		},
		OutputSchema: accommodationHacksOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Detect Accommodation Split Opportunities",
			ReadOnlyHint:   true,
			OpenWorldHint:  true,
			IdempotentHint: true,
		},
	}
}

func accommodationHacksOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city":     map[string]interface{}{"type": "string"},
			"checkin":  map[string]interface{}{"type": "string"},
			"checkout": map[string]interface{}{"type": "string"},
			"count":    map[string]interface{}{"type": "integer"},
			"hacks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type":        map[string]interface{}{"type": "string"},
						"title":       map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
						"savings":     map[string]interface{}{"type": "number"},
						"currency":    map[string]interface{}{"type": "string"},
						"risks":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"steps":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"citations":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					},
				},
			},
		},
		"required": []string{"city", "checkin", "checkout", "count", "hacks"},
	}
}

func handleDetectAccommodationHacks(args map[string]any, _ ElicitFunc, _ SamplingFunc) ([]ContentBlock, interface{}, error) {
	city := argString(args, "city")
	checkin := argString(args, "checkin")
	checkout := argString(args, "checkout")
	currency := argString(args, "currency")
	if currency == "" {
		currency = "EUR"
	}
	maxSplits := argInt(args, "max_splits", 3)
	guests := argInt(args, "guests", 2)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	in := hacks.AccommodationSplitInput{
		City:      city,
		CheckIn:   checkin,
		CheckOut:  checkout,
		Currency:  currency,
		MaxSplits: maxSplits,
		Guests:    guests,
	}

	detected := hacks.DetectAccommodationSplit(ctx, in)

	type response struct {
		City     string       `json:"city"`
		CheckIn  string       `json:"checkin"`
		CheckOut string       `json:"checkout"`
		Count    int          `json:"count"`
		Hacks    []hacks.Hack `json:"hacks"`
	}

	resp := response{
		City:     city,
		CheckIn:  checkin,
		CheckOut: checkout,
		Count:    len(detected),
		Hacks:    detected,
	}
	if resp.Hacks == nil {
		resp.Hacks = []hacks.Hack{}
	}

	summary := buildAccomHacksSummary(city, checkin, checkout, detected)
	content := []ContentBlock{
		{Type: "text", Text: summary, Annotations: &ContentAnnotation{Audience: []string{"user"}, Priority: 1.0}},
		{Type: "text", Text: "Structured accommodation hack data attached.", Annotations: &ContentAnnotation{Audience: []string{"assistant"}, Priority: 0.5}},
	}
	return content, resp, nil
}

func buildAccomHacksSummary(city, checkin, checkout string, detected []hacks.Hack) string {
	if len(detected) == 0 {
		return fmt.Sprintf("No accommodation split opportunities found for %s (%s to %s).", city, checkin, checkout)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Accommodation hacks for %s (%s to %s):\n\n", city, checkin, checkout))
	for i, h := range detected {
		sb.WriteString(fmt.Sprintf("%d. %s", i+1, h.Title))
		if h.Savings > 0 {
			sb.WriteString(fmt.Sprintf(" — saves %s %.0f", h.Currency, h.Savings))
		}
		sb.WriteString("\n")
		sb.WriteString("   " + h.Description + "\n\n")
	}
	return sb.String()
}
