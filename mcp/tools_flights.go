package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hacks"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/points"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/MikkoParkkola/trvl/internal/profile"
	"github.com/MikkoParkkola/trvl/internal/trip"
)

// --- Output schema builders ---

// flightSearchOutputSchema returns the JSON Schema for FlightSearchResult.
func flightSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":   schemaBool(),
			"count":     schemaInt(),
			"trip_type": schemaString(),
			"flights": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"price":          schemaNum(),
					"currency":       schemaString(),
					"duration":       schemaInt(),
					"stops":          schemaInt(),
					"provider":       schemaString(),
					"booking_url":    schemaString(),
					"all_in_cost":    schemaNumDesc("Total cost including baggage fees adjusted for FF status"),
					"bag_breakdown":  schemaStringDesc("Baggage cost explanation, e.g. '+€35 checked bag' or 'bags included'"),
					"self_connect":   schemaBool(),
					"miles_earned": schemaArrayDesc("Estimated miles/points earned per FF programme", map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"program":      schemaString(),
							"miles_earned": schemaInt(),
							"method":       schemaStringDesc("'revenue' or 'distance'"),
						},
					}),
					"miles_value": schemaNumDesc("Cents-per-mile value if this flight were redeemed with points"),
					"warnings":   schemaStringArray(),
					"legs": schemaArray(map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"departure_airport": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"code": schemaString(),
									"name": schemaString(),
								},
							},
							"arrival_airport": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"code": schemaString(),
									"name": schemaString(),
								},
							},
							"departure_time": schemaString(),
							"arrival_time":   schemaString(),
							"duration":       schemaInt(),
							"airline":        schemaString(),
							"airline_code":   schemaString(),
							"flight_number":  schemaString(),
						},
					}),
				},
			}),
			"suggestions": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action":      schemaString(),
					"description": schemaString(),
					"params":      schemaObject(),
				},
			}),
			"hacks": schemaArrayDesc("Auto-detected travel optimization tips for this route (zero-API-call detectors only)", map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type":        schemaString(),
					"title":       schemaString(),
					"description": schemaString(),
					"savings":     schemaNum(),
					"currency":    schemaString(),
					"steps":       schemaStringArray(),
				},
			}),
			"error": schemaString(),
		},
		"required": []string{"success", "count"},
	}
}

// dateSearchOutputSchema returns the JSON Schema for DateSearchResult.
func dateSearchOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":    schemaBool(),
			"count":      schemaInt(),
			"trip_type":  schemaString(),
			"date_range": schemaString(),
			"dates": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"date":        schemaString(),
					"price":       schemaNum(),
					"currency":    schemaString(),
					"return_date": schemaString(),
				},
				"required": []string{"date", "price", "currency"},
			}),
			"error": schemaString(),
		},
		"required": []string{"success", "count"},
	}
}

// --- Tool definitions ---

func searchFlightsTool() ToolDef {
	return ToolDef{
		Name:        "search_flights",
		Title:       "Search Flights",
		Description: "Search flights via Google Flights, and on compatible one-way searches also include Kiwi virtual-interlining results with explicit self-connect warnings. Returns real-time pricing, durations, stops, and leg details for a given route and date. IMPORTANT: call get_preferences before your first search in a conversation to load the user's home airport and flight preferences. If the profile is empty, interview the user first — get_preferences returns instructions. Use home_airports as default origin when the user doesn't specify where from.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":              {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK, NRT)"},
				"destination":         {Type: "string", Description: "Arrival airport IATA code (e.g., NRT, LAX, CDG)"},
				"departure_date":      {Type: "string", Description: "Departure date in YYYY-MM-DD format"},
				"return_date":         {Type: "string", Description: "Return date in YYYY-MM-DD format for round-trip (omit for one-way)"},
				"cabin_class":         {Type: "string", Description: "Cabin class: economy, premium_economy, business, or first (default: economy)"},
				"max_stops":           {Type: "string", Description: "Maximum stops: any, nonstop, one_stop, or two_plus (default: any)"},
				"sort_by":             {Type: "string", Description: "Sort order: cheapest, duration, departure, or arrival (default: cheapest)"},
				"alliances":           {Type: "string", Description: "Filter by airline alliance (comma-separated): STAR_ALLIANCE, ONEWORLD, SKYTEAM (default: no filter)"},
				"depart_after":        {Type: "string", Description: "Earliest departure time HH:MM, e.g. 06:00 (default: no filter)"},
				"depart_before":       {Type: "string", Description: "Latest departure time HH:MM, e.g. 22:00 (default: no filter)"},
				"max_price":           {Type: "integer", Description: "Maximum price in whole currency units (0 = no limit). Server-side filter."},
				"max_duration":        {Type: "integer", Description: "Maximum total flight duration in minutes (0 = no limit). Server-side filter."},
				"exclude_basic":       {Type: "boolean", Description: "Exclude basic economy fares (default: false). Server-side filter."},
				"less_emissions":      {Type: "boolean", Description: "Only show flights with lower CO2 emissions (default: false)"},
				"carry_on_bags":       {Type: "integer", Description: "Require N carry-on bags included in price (0 = no filter, 1 = require carry-on). Server-side price recalculation."},
				"checked_bags":        {Type: "integer", Description: "Checked bags pricing hint (0 = default, 1+ = recalculate prices including N checked bags). Changes price display, does not remove flights. Use require_checked_bag for actual filtering."},
				"require_checked_bag": {Type: "boolean", Description: "Only show flights with ≥1 free checked bag included (default: false). Client-side post-filter on response data."},
			},
			Required: []string{"origin", "destination", "departure_date"},
		},
		OutputSchema: flightSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Search Flights",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func searchDatesTool() ToolDef {
	return ToolDef{
		Name:        "search_dates",
		Title:       "Search Flight Dates",
		Description: "Find the cheapest flight prices across a date range. Returns one price per departure date, useful for finding the best travel dates.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":        {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK, NRT)"},
				"destination":   {Type: "string", Description: "Arrival airport IATA code (e.g., NRT, LAX, CDG)"},
				"start_date":    {Type: "string", Description: "Start of date range in YYYY-MM-DD format"},
				"end_date":      {Type: "string", Description: "End of date range in YYYY-MM-DD format"},
				"trip_duration": {Type: "integer", Description: "Trip duration in days for round-trip (omit for one-way)"},
				"is_round_trip": {Type: "boolean", Description: "Whether to search round-trip fares (default: false)"},
			},
			Required: []string{"origin", "destination", "start_date", "end_date"},
		},
		OutputSchema: dateSearchOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Search Flight Dates",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

// --- Tool handlers ---

func handleSearchFlights(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	origin, dest, err := validateOriginDest(args)
	if err != nil {
		return nil, nil, err
	}

	date, err := validateDate(args, "departure_date")
	if err != nil {
		return nil, nil, err
	}

	// Validate return date if provided.
	if ret := argString(args, "return_date"); ret != "" {
		if err := models.ValidateDate(ret); err != nil {
			return nil, nil, fmt.Errorf("invalid return_date: %w", err)
		}
	}

	opts := flights.SearchOptions{
		ReturnDate:        argString(args, "return_date"),
		MaxPrice:          argInt(args, "max_price", 0),
		MaxDuration:       argInt(args, "max_duration", 0),
		ExcludeBasic:      argBool(args, "exclude_basic", false),
		Alliances:         argStringSlice(args, "alliances"),
		DepartAfter:       argString(args, "depart_after"),
		DepartBefore:      argString(args, "depart_before"),
		LessEmissions:     argBool(args, "less_emissions", false),
		CarryOnBags:       argInt(args, "carry_on_bags", 0),
		CheckedBags:       argInt(args, "checked_bags", 0),
		RequireCheckedBag: argBool(args, "require_checked_bag", false),
	}

	if cc := argString(args, "cabin_class"); cc != "" {
		parsed, err := models.ParseCabinClass(cc)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid cabin_class: %w", err)
		}
		opts.CabinClass = parsed
	}

	if ms := argString(args, "max_stops"); ms != "" {
		parsed, err := models.ParseMaxStops(ms)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid max_stops: %w", err)
		}
		opts.MaxStops = parsed
	}

	if sb := argString(args, "sort_by"); sb != "" {
		parsed, err := models.ParseSortBy(sb)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid sort_by: %w", err)
		}
		opts.SortBy = parsed
	}

	// Apply profile hints as defaults — only when the caller has not set the
	// corresponding parameter explicitly.
	prof, _ := profile.Load()
	hints := profile.FlightHints(prof, origin, dest)
	if _, explicit := args["cabin_class"]; !explicit && hints.CabinClass > 0 && opts.CabinClass == 0 {
		opts.CabinClass = models.CabinClass(hints.CabinClass)
	}
	if _, explicit := args["alliances"]; !explicit && hints.PreferredAlliance != "" && len(opts.Alliances) == 0 {
		opts.Alliances = []string{hints.PreferredAlliance}
	}
	if _, explicit := args["max_price"]; !explicit && hints.MaxPrice > 0 && opts.MaxPrice == 0 {
		opts.MaxPrice = hints.MaxPrice
	}

	result, err := flights.SearchFlights(ctx, origin, dest, date, opts)
	if err != nil {
		return nil, nil, err
	}

	// Apply preference-based post-filters (budget, departure time window,
	// and frequent flyer bag allowance adjustments).
	prefs, _ := preferences.Load()
	if prefs != nil && result != nil && result.Success {
		result.Flights = flights.FilterFlightsByBudget(result.Flights, prefs.BudgetFlightMax)
		result.Flights = flights.FilterFlightsByTimePreference(result.Flights, prefs.FlightTimeEarliest, prefs.FlightTimeLatest)
		result.Flights = flights.AdjustBagAllowance(result.Flights, prefs.FrequentFlyerPrograms)
		result.Count = len(result.Flights)
	}

	// Enrich flights with all-in cost (base fare + baggage - FF benefits).
	// Miles earning info per FF programme.
	type milesEarningInfo struct {
		Program     string `json:"program"`
		MilesEarned int    `json:"miles_earned"`
		Method      string `json:"method"` // "revenue" or "distance"
	}
	type enrichedFlight struct {
		models.FlightResult
		AllInCost    float64            `json:"all_in_cost,omitempty"`
		BagBreakdown string             `json:"bag_breakdown,omitempty"`
		MilesEarned  []milesEarningInfo `json:"miles_earned,omitempty"`
		MilesValue   float64            `json:"miles_value,omitempty"` // cents-per-mile if redeemed at this price
	}
	enrichedFlights := make([]enrichedFlight, len(result.Flights))
	if prefs != nil && result.Success {
		needCheckedBag := !prefs.CarryOnOnly
		needCarryOn := true
		var ffStatuses []baggage.FFStatus
		for _, fp := range prefs.FrequentFlyerPrograms {
			ffStatuses = append(ffStatuses, baggage.FFStatus{
				Alliance: fp.Alliance,
				Tier:     fp.Tier,
			})
		}

		// Determine cabin class for earning estimation.
		cabinClass := "economy"
		if cc := argString(args, "cabin_class"); cc != "" {
			cabinClass = cc
		}

		for i, f := range result.Flights {
			enrichedFlights[i].FlightResult = f
			airlineCode := ""
			if len(f.Legs) > 0 {
				airlineCode = f.Legs[0].AirlineCode
			}
			if airlineCode != "" {
				allIn, breakdown := baggage.AllInCost(f.Price, airlineCode, needCheckedBag, needCarryOn, ffStatuses)
				if breakdown != "" {
					enrichedFlights[i].AllInCost = allIn
					enrichedFlights[i].BagBreakdown = breakdown
				}
			}

			// Miles earning estimate per FF programme.
			if airlineCode != "" {
				for _, ff := range prefs.FrequentFlyerPrograms {
					est := points.EstimateMilesEarned(origin, dest, cabinClass, airlineCode, ff.Alliance, f.Price)
					if est.Miles > 0 {
						programLabel := ff.ProgramName
						if programLabel == "" {
							programLabel = est.Program
						}
						enrichedFlights[i].MilesEarned = append(enrichedFlights[i].MilesEarned, milesEarningInfo{
							Program:     programLabel,
							MilesEarned: est.Miles,
							Method:      est.Method,
						})
					}
				}
			}
		}
	} else {
		for i, f := range result.Flights {
			enrichedFlights[i].FlightResult = f
		}
	}

	// Build suggestions for progressive disclosure.
	suggestions := flightSuggestions(result, origin, dest, date, opts)

	// Run zero-API-call hack detectors for auto-tips.
	var flightHacks []hacks.Hack
	if result.Success && len(result.Flights) > 0 {
		cheapest := result.Flights[0]
		for _, f := range result.Flights[1:] {
			if f.Price > 0 && f.Price < cheapest.Price {
				cheapest = f
			}
		}
		hackCurrency := cheapest.Currency
		if hackCurrency == "" {
			hackCurrency = "EUR"
		}

		hackInput := hacks.DetectorInput{
			Origin:      origin,
			Destination: dest,
			Date:        date,
			ReturnDate:  opts.ReturnDate,
			Currency:    hackCurrency,
			NaivePrice:  cheapest.Price,
			Passengers:  1,
		}
		flightHacks = hacks.DetectFlightTips(ctx, hackInput)

		// Fuel surcharge — collect airline codes from results.
		airlineCodeSet := make(map[string]bool)
		for _, f := range result.Flights {
			for _, leg := range f.Legs {
				if leg.AirlineCode != "" {
					airlineCodeSet[leg.AirlineCode] = true
				}
			}
		}
		if len(airlineCodeSet) > 0 {
			var codes []string
			for code := range airlineCodeSet {
				codes = append(codes, code)
			}
			flightHacks = append(flightHacks, hacks.DetectFuelSurcharge(origin, dest, codes)...)
		}

		// Sort by savings descending, then type for deterministic ordering.
		sort.Slice(flightHacks, func(i, j int) bool {
			if flightHacks[i].Savings != flightHacks[j].Savings {
				return flightHacks[i].Savings > flightHacks[j].Savings
			}
			return flightHacks[i].Type < flightHacks[j].Type
		})

		// Cap at 3.
		if len(flightHacks) > 3 {
			flightHacks = flightHacks[:3]
		}
	}

	// Build structured response.
	type enrichedFlightSearchResult struct {
		Success     bool             `json:"success"`
		Count       int              `json:"count"`
		TripType    string           `json:"trip_type"`
		Flights     []enrichedFlight `json:"flights"`
		Error       string           `json:"error,omitempty"`
		Suggestions []Suggestion     `json:"suggestions,omitempty"`
		Hacks       []hacks.Hack     `json:"hacks,omitempty"`
	}
	resp := enrichedFlightSearchResult{
		Success:     result.Success,
		Count:       result.Count,
		TripType:    result.TripType,
		Flights:     enrichedFlights,
		Error:       result.Error,
		Suggestions: suggestions,
		Hacks:       flightHacks,
	}

	content, err := buildAnnotatedContentBlocks(flightSummary(result, origin, dest), resp)
	if err != nil {
		return nil, nil, err
	}

	return content, resp, nil
}

func handleSearchDates(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	origin, dest, err := validateOriginDest(args)
	if err != nil {
		return nil, nil, err
	}

	startDate := argString(args, "start_date")
	endDate := argString(args, "end_date")
	if startDate == "" || endDate == "" {
		return nil, nil, fmt.Errorf("start_date and end_date are required")
	}

	// Validate date range.
	if err := models.ValidateDateRange(startDate, endDate); err != nil {
		return nil, nil, err
	}

	opts := flights.CalendarOptions{
		FromDate:   startDate,
		ToDate:     endDate,
		TripLength: argInt(args, "trip_duration", 0),
		RoundTrip:  argBool(args, "is_round_trip", false),
	}

	// Use SearchCalendar (1 API call via GetCalendarGraph) instead of the
	// legacy SearchDates (N calls, one per date). Falls back to N-call
	// automatically if CalendarGraph fails.
	result, err := flights.SearchCalendar(ctx, origin, dest, opts)
	if err != nil {
		return nil, nil, err
	}

	summary := fmt.Sprintf("Found prices for %d dates from %s to %s (%s to %s).",
		result.Count, origin, dest, startDate, endDate)
	if result.Count > 0 {
		cheapest := result.Dates[0]
		for _, d := range result.Dates[1:] {
			if d.Price > 0 && d.Price < cheapest.Price {
				cheapest = d
			}
		}
		summary += fmt.Sprintf(" Cheapest: %s %.0f on %s.", cheapest.Currency, cheapest.Price, cheapest.Date)
	}

	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

// --- Summary builders ---

func flightSummary(result *models.FlightSearchResult, origin, dest string) string {
	if !result.Success || result.Count == 0 {
		if result.Error != "" {
			return fmt.Sprintf("Flight search from %s to %s failed: %s", origin, dest, result.Error)
		}
		return fmt.Sprintf("No flights found from %s to %s.", origin, dest)
	}

	summary := fmt.Sprintf("Found %d flights from %s to %s.", result.Count, origin, dest)

	// Find cheapest.
	cheapest := result.Flights[0]
	for _, f := range result.Flights[1:] {
		if f.Price > 0 && f.Price < cheapest.Price {
			cheapest = f
		}
	}
	if cheapest.Price > 0 {
		stopStr := "nonstop"
		if cheapest.Stops == 1 {
			stopStr = "1 stop"
		} else if cheapest.Stops > 1 {
			stopStr = fmt.Sprintf("%d stops", cheapest.Stops)
		}
		airline := ""
		if len(cheapest.Legs) > 0 {
			airline = cheapest.Legs[0].Airline
		}
		if airline == "" && cheapest.Provider != "" {
			airline = flightProviderSummaryLabel(cheapest.Provider)
		}
		summary += fmt.Sprintf(" Cheapest: %s%.0f (%s, %s).",
			cheapest.Currency, cheapest.Price, airline, stopStr)
	}

	// Check for nonstop options.
	nonstopCount := 0
	var cheapestNonstop *models.FlightResult
	for i := range result.Flights {
		if result.Flights[i].Stops == 0 {
			nonstopCount++
			if cheapestNonstop == nil || result.Flights[i].Price < cheapestNonstop.Price {
				cheapestNonstop = &result.Flights[i]
			}
		}
	}
	if nonstopCount > 0 && cheapestNonstop != nil {
		summary += fmt.Sprintf(" Nonstop options from %s%.0f.", cheapestNonstop.Currency, cheapestNonstop.Price)
	}

	selfConnectCount := 0
	for _, flight := range result.Flights {
		if flight.SelfConnect {
			selfConnectCount++
		}
	}
	if selfConnectCount > 0 {
		summary += fmt.Sprintf(" Includes %d Kiwi self-connect option%s with connection-risk warnings.",
			selfConnectCount, pluralSuffix(selfConnectCount))
	}

	return summary
}

func flightProviderSummaryLabel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "google_flights":
		return "Google Flights"
	case "kiwi":
		return "Kiwi"
	default:
		return provider
	}
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// --- Suggestion builders ---

func flightSuggestions(result *models.FlightSearchResult, origin, dest, date string, opts flights.SearchOptions) []Suggestion {
	var suggestions []Suggestion

	if !result.Success || result.Count == 0 {
		return nil
	}

	// If searching one-way, suggest round-trip.
	if opts.ReturnDate == "" {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_flights",
			Description: "Search round-trip for potentially lower fares",
			Params:      map[string]any{"origin": origin, "destination": dest, "departure_date": date, "return_date": "YYYY-MM-DD"},
		})
	}

	// If there are many stops, suggest nonstop filter.
	hasMultiStop := false
	for _, f := range result.Flights {
		if f.Stops >= 2 {
			hasMultiStop = true
			break
		}
	}
	if hasMultiStop && opts.MaxStops == 0 {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_flights",
			Description: "Filter to nonstop flights only",
			Params:      map[string]any{"origin": origin, "destination": dest, "departure_date": date, "max_stops": "nonstop"},
		})
	}

	// If prices vary widely, suggest flexible dates.
	if result.Count >= 3 {
		minPrice := result.Flights[0].Price
		maxPrice := result.Flights[0].Price
		for _, f := range result.Flights[1:] {
			if f.Price > 0 && f.Price < minPrice {
				minPrice = f.Price
			}
			if f.Price > maxPrice {
				maxPrice = f.Price
			}
		}
		if maxPrice > 0 && minPrice > 0 && maxPrice > minPrice*2 {
			suggestions = append(suggestions, Suggestion{
				Action:      "search_dates",
				Description: "Find the cheapest departure date this month",
				Params:      map[string]any{"origin": origin, "destination": dest},
			})
		}
	}

	// If economy, suggest checking business class.
	if opts.CabinClass == 0 || opts.CabinClass == models.Economy {
		suggestions = append(suggestions, Suggestion{
			Action:      "search_flights",
			Description: "Check business class availability",
			Params:      map[string]any{"origin": origin, "destination": dest, "departure_date": date, "cabin_class": "business"},
		})
	}

	return suggestions
}

// --- Suggest Dates tool ---

// suggestDatesOutputSchema returns the JSON Schema for SmartDateResult.
func suggestDatesOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":     schemaBool(),
			"origin":      schemaString(),
			"destination": schemaString(),
			"cheapest_dates": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"date":        schemaString(),
					"day_of_week": schemaString(),
					"price":       schemaNum(),
					"currency":    schemaString(),
					"return_date": schemaString(),
				},
				"required": []string{"date", "price", "currency"},
			}),
			"average_price": schemaNum(),
			"currency":      schemaString(),
			"insights": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type":        schemaString(),
					"description": schemaString(),
					"date":        schemaString(),
					"price":       schemaNum(),
					"savings":     schemaNum(),
				},
			}),
			"error": schemaString(),
		},
		"required": []string{"success"},
	}
}

func suggestDatesTool() ToolDef {
	return ToolDef{
		Name:        "suggest_dates",
		Title:       "Smart Date Suggestions",
		Description: "Analyze flight prices around a target date and suggest the cheapest travel dates. Returns the 3 cheapest dates, weekday vs weekend analysis, and actionable savings insights.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":      {Type: "string", Description: "Departure airport IATA code (e.g., HEL, JFK)"},
				"destination": {Type: "string", Description: "Arrival airport IATA code (e.g., BCN, LHR)"},
				"target_date": {Type: "string", Description: "Center date to search around (YYYY-MM-DD)"},
				"flex_days":   {Type: "integer", Description: "Days of flexibility around target date (default: 7)"},
				"round_trip":  {Type: "boolean", Description: "Whether to search round-trip prices (default: false)"},
				"duration":    {Type: "integer", Description: "Trip duration in days for round-trip (default: 7)"},
			},
			Required: []string{"origin", "destination", "target_date"},
		},
		OutputSchema: suggestDatesOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Smart Date Suggestions",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func handleSuggestDates(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	origin, dest, err := validateOriginDest(args)
	if err != nil {
		return nil, nil, err
	}

	targetDate := argString(args, "target_date")
	if targetDate == "" {
		return nil, nil, fmt.Errorf("target_date is required")
	}

	opts := trip.SmartDateOptions{
		TargetDate: targetDate,
		FlexDays:   argInt(args, "flex_days", 7),
		RoundTrip:  argBool(args, "round_trip", false),
		Duration:   argInt(args, "duration", 7),
	}

	result, err := trip.SuggestDates(ctx, origin, dest, opts)
	if err != nil {
		return nil, nil, err
	}

	summary := suggestDatesSummary(result, origin, dest)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func suggestDatesSummary(result *trip.SmartDateResult, origin, dest string) string {
	if !result.Success {
		if result.Error != "" {
			return fmt.Sprintf("Date suggestion %s to %s failed: %s", origin, dest, result.Error)
		}
		return fmt.Sprintf("Could not find date suggestions from %s to %s.", origin, dest)
	}

	parts := []string{
		fmt.Sprintf("Date analysis %s -> %s (avg %s %.0f)", origin, dest, result.Currency, result.AveragePrice),
	}

	for _, ins := range result.Insights {
		parts = append(parts, ins.Description)
	}

	return strings.Join(parts, ". ") + "."
}

// --- Optimize Multi-City tool ---

// multiCityOutputSchema returns the JSON Schema for MultiCityResult.
func multiCityOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success":      schemaBool(),
			"home_airport": schemaString(),
			"optimal_order": schemaStringArray(),
			"segments": schemaArray(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"from":     schemaString(),
					"to":       schemaString(),
					"price":    schemaNum(),
					"currency": schemaString(),
				},
				"required": []string{"from", "to", "price", "currency"},
			}),
			"total_cost":           schemaNum(),
			"currency":             schemaString(),
			"worst_cost":           schemaNum(),
			"savings":              schemaNum(),
			"permutations_checked": schemaInt(),
			"error":                schemaString(),
		},
		"required": []string{"success"},
	}
}

func optimizeMultiCityTool() ToolDef {
	return ToolDef{
		Name:        "optimize_multi_city",
		Title:       "Multi-City Trip Optimizer",
		Description: "Find the cheapest routing order for visiting multiple cities. Tries all permutations (up to 6 cities) and returns the optimal visit order with per-segment prices.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"home_airport": {Type: "string", Description: "Home airport IATA code (e.g., HEL, JFK)"},
				"cities":       {Type: "string", Description: "Comma-separated list of city IATA codes to visit (e.g., BCN,ROM,PAR)"},
				"depart_date":  {Type: "string", Description: "Departure date (YYYY-MM-DD)"},
				"return_date":  {Type: "string", Description: "Return date (YYYY-MM-DD, optional)"},
			},
			Required: []string{"home_airport", "cities", "depart_date"},
		},
		OutputSchema: multiCityOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Multi-City Trip Optimizer",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  true,
		},
	}
}

func handleOptimizeMultiCity(ctx context.Context, args map[string]any, elicit ElicitFunc, sampling SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	home := strings.ToUpper(argString(args, "home_airport"))
	citiesStr := argString(args, "cities")
	departDate := argString(args, "depart_date")
	returnDate := argString(args, "return_date")

	if home == "" {
		return nil, nil, fmt.Errorf("home_airport is required")
	}
	if citiesStr == "" {
		return nil, nil, fmt.Errorf("cities is required")
	}
	if departDate == "" {
		return nil, nil, fmt.Errorf("depart_date is required")
	}

	if err := models.ValidateIATA(home); err != nil {
		return nil, nil, fmt.Errorf("invalid home_airport: %w", err)
	}

	cities := argStringSlice(args, "cities")
	if len(cities) == 0 {
		return nil, nil, fmt.Errorf("at least one city is required")
	}
	for i, c := range cities {
		cities[i] = strings.ToUpper(strings.TrimSpace(c))
		if err := models.ValidateIATA(cities[i]); err != nil {
			return nil, nil, fmt.Errorf("invalid city %q: %w", c, err)
		}
	}

	opts := trip.MultiCityOptions{
		DepartDate: departDate,
		ReturnDate: returnDate,
	}

	result, err := trip.OptimizeMultiCity(ctx, home, cities, opts)
	if err != nil {
		return nil, nil, err
	}

	summary := multiCitySummary(result)
	content, err := buildAnnotatedContentBlocks(summary, result)
	if err != nil {
		return nil, nil, err
	}

	return content, result, nil
}

func multiCitySummary(result *trip.MultiCityResult) string {
	if !result.Success {
		if result.Error != "" {
			return fmt.Sprintf("Multi-city optimization failed: %s", result.Error)
		}
		return "Could not optimize multi-city routing."
	}

	route := append([]string{result.HomeAirport}, result.OptimalOrder...)
	route = append(route, result.HomeAirport)
	routeStr := strings.Join(route, " -> ")

	summary := fmt.Sprintf("Optimal route: %s. Total: %s %.0f.", routeStr, result.Currency, result.TotalCost)
	if result.Savings > 0 {
		summary += fmt.Sprintf(" Saves %s %.0f vs worst order.", result.Currency, result.Savings)
	}

	return summary
}
