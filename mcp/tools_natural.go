package mcp

import (
	"fmt"
	"strings"
	"time"
)

// searchNaturalTool returns the MCP tool definition for natural-language search.
func searchNaturalTool() ToolDef {
	return ToolDef{
		Name:  "search_natural",
		Title: "Natural Language Travel Search",
		Description: "Accept a free-form travel query in plain language and dispatch to the " +
			"appropriate search tool. Examples: " +
			"\"cheapest way from Helsinki to Dubrovnik next weekend\", " +
			"\"hotels in Prague for 3 nights in July under EUR 120\", " +
			"\"I want to explore a Croatian island, budget EUR 500, long weekend next month\". " +
			"Uses a keyword heuristic parser to extract intent, origin, destination, and dates. " +
			"Works on every MCP client. For complex or ambiguous queries, prefer calling the " +
			"specific tools (search_flights, search_route, search_hotels) directly with " +
			"structured parameters. " +
			"IMPORTANT: call get_preferences before your first travel search in a conversation. " +
			"If the profile is empty, get_preferences returns interview instructions — ask the " +
			"user those questions first, save with update_preferences, then proceed.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "Natural language travel request",
				},
			},
			Required: []string{"query"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"intent":      map[string]interface{}{"type": "string"},
				"result":      map[string]interface{}{"type": "object"},
				"query":       map[string]interface{}{"type": "string"},
				"dispatched_to": map[string]interface{}{"type": "string"},
			},
		},
		Annotations: &ToolAnnotations{
			Title:          "Natural Language Travel Search",
			ReadOnlyHint:   true,
			OpenWorldHint:  true,
			IdempotentHint: true,
		},
	}
}

// naturalSearchParams holds the structured parameters extracted from a free-form query.
type naturalSearchParams struct {
	Intent        string   `json:"intent"`         // "route", "flight", "hotel", "deals"
	Origin        string   `json:"origin"`         // IATA or city; empty if not mentioned
	Destination   string   `json:"destination"`    // IATA or city
	Date          string   `json:"date"`           // YYYY-MM-DD or empty
	ReturnDate    string   `json:"return_date"`    // YYYY-MM-DD or empty
	CheckIn       string   `json:"check_in"`       // YYYY-MM-DD (hotels)
	CheckOut      string   `json:"check_out"`      // YYYY-MM-DD (hotels)
	MaxBudget     float64  `json:"max_budget"`     // 0 = unlimited
	TravelerCount int      `json:"traveler_count"` // 0 = unspecified (default 1 or 2)
	Modes         []string `json:"transport_modes"`// "flight", "train", "bus", "ferry"
	Location      string   `json:"location"`       // hotel location when intent=hotel
}

// handleSearchNatural handles the search_natural tool.
func handleSearchNatural(args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	query := strings.TrimSpace(argString(args, "query"))
	if query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

	sendProgress(progress, 0, 100, "Parsing travel query...")

	today := time.Now().Format("2006-01-02")

	// Heuristic parse: extract intent, origin, destination, and dates from keywords.
	params := heuristicParse(query, today)

	sendProgress(progress, 30, 100, fmt.Sprintf("Dispatching %s search...", params.Intent))

	// Dispatch to the appropriate handler.
	return dispatchNatural(params, query, elicit, sampling, progress)
}

// dispatchNatural routes parsed params to the right tool handler.
func dispatchNatural(p naturalSearchParams, originalQuery string, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	switch p.Intent {
	case "hotel":
		if p.Location == "" && p.Destination != "" {
			p.Location = p.Destination
		}
		if p.Location == "" {
			return []ContentBlock{{Type: "text", Text: "Could not determine hotel location from your query. Please specify a city."}}, nil, nil
		}
		if p.CheckIn == "" || p.CheckOut == "" {
			return []ContentBlock{{Type: "text", Text: "Could not determine check-in or check-out dates from your query. Please specify dates."}}, nil, nil
		}
		hotelArgs := map[string]any{
			"location":  p.Location,
			"check_in":  p.CheckIn,
			"check_out": p.CheckOut,
		}
		if p.TravelerCount > 0 {
			hotelArgs["guests"] = p.TravelerCount
		}
		if p.MaxBudget > 0 {
			hotelArgs["max_price"] = p.MaxBudget
		}
		return handleSearchHotels(hotelArgs, elicit, sampling, progress)

	case "flight":
		if p.Origin == "" || p.Destination == "" || p.Date == "" {
			return []ContentBlock{{Type: "text", Text: "Could not determine origin, destination, or date for the flight search. Please specify them."}}, nil, nil
		}
		flightArgs := map[string]any{
			"origin":         p.Origin,
			"destination":    p.Destination,
			"departure_date": p.Date,
		}
		if p.ReturnDate != "" {
			flightArgs["return_date"] = p.ReturnDate
		}
		return handleSearchFlights(flightArgs, elicit, sampling, progress)

	case "route":
		if p.Origin == "" || p.Destination == "" || p.Date == "" {
			return []ContentBlock{{Type: "text", Text: "Could not determine origin, destination, or date for the route search. Please specify them."}}, nil, nil
		}
		routeArgs := map[string]any{
			"origin":      p.Origin,
			"destination": p.Destination,
			"date":        p.Date,
		}
		if p.MaxBudget > 0 {
			routeArgs["max_price"] = p.MaxBudget
		}
		if len(p.Modes) > 0 {
			// If user only wants trains/buses, avoid flights.
			wantsFlights := false
			for _, m := range p.Modes {
				if m == "flight" {
					wantsFlights = true
				}
			}
			if !wantsFlights {
				routeArgs["avoid"] = "flight"
			}
		}
		return handleSearchRoute(routeArgs, elicit, sampling, progress)

	default:
		// Fallback: return a helpful message describing what we parsed.
		msg := fmt.Sprintf("Interpreted your query as: %q\n\n", originalQuery)
		if p.Origin != "" {
			msg += fmt.Sprintf("From: %s\n", p.Origin)
		}
		if p.Destination != "" {
			msg += fmt.Sprintf("To: %s\n", p.Destination)
		}
		if p.Date != "" {
			msg += fmt.Sprintf("Date: %s\n", p.Date)
		}
		msg += "\nCould not determine the search intent. Try search_flights, search_route, or search_hotels with explicit parameters."
		return []ContentBlock{{Type: "text", Text: msg}}, nil, nil
	}
}

// heuristicParse extracts travel intent and parameters from a free-form query
// using keyword matching and simple date resolution. It is the primary (and
// only active) parse path — sampling is not wired in production.
func heuristicParse(query, today string) naturalSearchParams {
	lower := strings.ToLower(query)
	p := naturalSearchParams{Intent: "route"}

	// Detect intent using explicit keyword checks (not ContainsAny which is char-based).
	switch {
	case strings.Contains(lower, "hotel") || strings.Contains(lower, "hostel") ||
		strings.Contains(lower, "accommodation") || strings.Contains(lower, "stay") ||
		strings.Contains(lower, "sleep") || strings.Contains(lower, "room") ||
		strings.Contains(lower, "check-in") || strings.Contains(lower, "check in"):
		p.Intent = "hotel"
	case strings.Contains(lower, "fly ") || strings.Contains(lower, "flying") ||
		strings.Contains(lower, "flight") || strings.Contains(lower, "airport"):
		p.Intent = "flight"
	case strings.Contains(lower, "deal") || strings.Contains(lower, "inspiration"):
		p.Intent = "deals"
	}

	// Resolve "next weekend" — the simplest relative date.
	if strings.Contains(lower, "next weekend") || strings.Contains(lower, "this weekend") {
		t, _ := time.Parse("2006-01-02", today)
		// Advance to next Saturday.
		daysUntilSat := (6 - int(t.Weekday()) + 7) % 7
		if daysUntilSat == 0 {
			daysUntilSat = 7
		}
		sat := t.AddDate(0, 0, daysUntilSat)
		mon := sat.AddDate(0, 0, 2)
		p.Date = sat.Format("2006-01-02")
		p.CheckIn = p.Date
		p.CheckOut = mon.Format("2006-01-02")
	}

	return p
}
