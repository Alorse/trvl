package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/nlsearch"
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
				"intent":        map[string]interface{}{"type": "string"},
				"result":        map[string]interface{}{"type": "object"},
				"query":         map[string]interface{}{"type": "string"},
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

// naturalSearchParams is an alias for the shared nlsearch.Params type so the
// MCP package and the trvl CLI parse queries with identical semantics.
// All field documentation lives on nlsearch.Params.
type naturalSearchParams = nlsearch.Params

// handleSearchNatural handles the search_natural tool.
func handleSearchNatural(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	query := strings.TrimSpace(argString(args, "query"))
	if query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

	sendProgress(progress, 0, 100, "Parsing travel query...")

	today := time.Now().Format("2006-01-02")

	// Heuristic parse: extract intent, origin, destination, and dates from
	// keywords. Shared with the `trvl search` CLI command via the
	// internal/nlsearch package so both surfaces parse identically.
	params := heuristicParse(query, today)

	sendProgress(progress, 30, 100, fmt.Sprintf("Dispatching %s search...", params.Intent))

	// Dispatch to the appropriate handler.
	return dispatchNatural(ctx, params, query, elicit, sampling, progress)
}

// dispatchNatural routes parsed params to the right tool handler.
func dispatchNatural(ctx context.Context, p naturalSearchParams, originalQuery string, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
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
		return handleSearchHotels(ctx, hotelArgs, elicit, sampling, progress)

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
		return handleSearchFlights(ctx, flightArgs, elicit, sampling, progress)

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
		return handleSearchRoute(ctx, routeArgs, elicit, sampling, progress)

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

// heuristicParse delegates to nlsearch.Heuristic. It is kept as a thin
// in-package wrapper so the existing in-package tests
// (mcp/tools_natural_test.go) continue to compile against the same name.
func heuristicParse(query, today string) naturalSearchParams {
	return nlsearch.Heuristic(query, today)
}
