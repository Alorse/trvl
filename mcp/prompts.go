package mcp

import "fmt"

// registerPrompts adds all prompt definitions to the server.
func registerPrompts(s *Server) {
	s.prompts = []PromptDef{
		{
			Name:        "plan-trip",
			Description: "Plan a complete trip with flights and hotels",
			Arguments: []PromptArgument{
				{Name: "origin", Description: "Departure airport IATA code (e.g., HEL)", Required: true},
				{Name: "destination", Description: "Destination city or airport code (e.g., Tokyo or NRT)", Required: true},
				{Name: "departure_date", Description: "Departure date in YYYY-MM-DD format", Required: true},
				{Name: "return_date", Description: "Return date in YYYY-MM-DD format", Required: true},
				{Name: "budget", Description: "Total budget in USD (e.g., 3000)", Required: false},
			},
		},
		{
			Name:        "find-cheapest-dates",
			Description: "Find the cheapest dates to fly between two cities",
			Arguments: []PromptArgument{
				{Name: "origin", Description: "Departure airport IATA code (e.g., HEL)", Required: true},
				{Name: "destination", Description: "Destination airport IATA code (e.g., NRT)", Required: true},
				{Name: "month", Description: "Month to search (e.g., june-2026 or 2026-06)", Required: true},
			},
		},
		{
			Name:        "compare-hotels",
			Description: "Compare hotels in a destination",
			Arguments: []PromptArgument{
				{Name: "location", Description: "Destination city or area (e.g., Shibuya Tokyo)", Required: true},
				{Name: "check_in", Description: "Check-in date in YYYY-MM-DD format", Required: true},
				{Name: "check_out", Description: "Check-out date in YYYY-MM-DD format", Required: true},
				{Name: "priorities", Description: "Comma-separated priorities: price, rating, location (e.g., price,rating)", Required: false},
			},
		},
		{
			Name:        "where-should-i-go",
			Description: "Discover the best travel destinations from your city within a budget",
			Arguments: []PromptArgument{
				{Name: "origin", Description: "Departure airport IATA code (e.g., HEL, JFK)", Required: true},
				{Name: "month", Description: "Travel month (e.g., july-2026 or 2026-07)", Required: false},
				{Name: "budget", Description: "Maximum flight budget in local currency (e.g., 500)", Required: false},
			},
		},
	}
}

// getPrompt generates a prompt by name and arguments.
func getPrompt(name string, args map[string]any) (*PromptsGetResult, error) {
	switch name {
	case "plan-trip":
		return promptPlanTrip(args)
	case "find-cheapest-dates":
		return promptFindCheapestDates(args)
	case "compare-hotels":
		return promptCompareHotels(args)
	case "where-should-i-go":
		return promptWhereShouldIGo(args)
	default:
		return nil, fmt.Errorf("unknown prompt: %s", name)
	}
}

func argOr(args map[string]any, key, fallback string) string {
	if args == nil {
		return fallback
	}
	v, ok := args[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return fallback
	}
	return s
}

func promptPlanTrip(args map[string]any) (*PromptsGetResult, error) {
	origin := argOr(args, "origin", "")
	dest := argOr(args, "destination", "")
	depart := argOr(args, "departure_date", "")
	ret := argOr(args, "return_date", "")
	budget := argOr(args, "budget", "")

	if origin == "" || dest == "" || depart == "" || ret == "" {
		return nil, fmt.Errorf("origin, destination, departure_date, and return_date are required")
	}

	budgetLine := ""
	if budget != "" {
		budgetLine = fmt.Sprintf("\n\nThe total budget is $%s USD. Prioritize options that fit within this budget and flag any that exceed it.", budget)
	}

	prompt := fmt.Sprintf(`Plan a complete trip from %s to %s, departing %s and returning %s.%s

Follow these steps:

1. **Search flights**: Use search_flights to find outbound flights from %s to %s on %s with return_date %s. Look at the top 5 options by price and note airlines, stops, duration, and price.

2. **Search hotels**: Use search_hotels to find hotels in %s from %s to %s. Note the name, price per night, rating, and star level for the top options.

3. **Compare options**: Create a comparison table with:
   - Flight options (price, airline, duration, stops)
   - Hotel options (price/night, total cost, rating, stars)
   - Total trip cost for each combination

4. **Recommend**: Suggest the best value combination considering price, convenience, and quality. If there are nonstop flight options, highlight them even if slightly more expensive.

5. **Suggest alternatives**: Mention if flexible dates could save money (use search_dates if the price seems high) and if upgrading cabin class or hotel tier is worth considering.`,
		origin, dest, depart, ret, budgetLine,
		origin, dest, depart, ret,
		dest, depart, ret)

	return &PromptsGetResult{
		Description: fmt.Sprintf("Trip plan: %s to %s, %s - %s", origin, dest, depart, ret),
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentBlock{Type: "text", Text: prompt},
			},
		},
	}, nil
}

func promptFindCheapestDates(args map[string]any) (*PromptsGetResult, error) {
	origin := argOr(args, "origin", "")
	dest := argOr(args, "destination", "")
	month := argOr(args, "month", "")

	if origin == "" || dest == "" || month == "" {
		return nil, fmt.Errorf("origin, destination, and month are required")
	}

	prompt := fmt.Sprintf(`Find the cheapest dates to fly from %s to %s in %s.

Follow these steps:

1. **Search the full month**: Use search_dates with origin=%s, destination=%s, and the appropriate start_date and end_date for %s. Set is_round_trip=true with trip_duration=7 for a typical week-long trip.

2. **Analyze results**: Identify:
   - The single cheapest departure date and its price
   - The cheapest week (7-day window)
   - Any patterns (e.g., midweek departures being cheaper, avoid holidays)
   - Price range across the month (cheapest vs most expensive)

3. **Present findings**: Create a summary with:
   - Top 3 cheapest dates with prices
   - A brief price calendar showing relative prices across the month
   - Recommendation for the best dates to book

4. **Follow up**: For the cheapest date, use search_flights to show the actual flight options available that day.`,
		origin, dest, month,
		origin, dest, month)

	return &PromptsGetResult{
		Description: fmt.Sprintf("Cheapest dates: %s to %s in %s", origin, dest, month),
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentBlock{Type: "text", Text: prompt},
			},
		},
	}, nil
}

func promptCompareHotels(args map[string]any) (*PromptsGetResult, error) {
	location := argOr(args, "location", "")
	checkIn := argOr(args, "check_in", "")
	checkOut := argOr(args, "check_out", "")
	priorities := argOr(args, "priorities", "price,rating")

	if location == "" || checkIn == "" || checkOut == "" {
		return nil, fmt.Errorf("location, check_in, and check_out are required")
	}

	prompt := fmt.Sprintf(`Compare hotels in %s from %s to %s, prioritizing: %s.

Follow these steps:

1. **Search hotels**: Use search_hotels to find hotels in %s from %s to %s.

2. **Rank by priorities**: Re-rank the results according to the priorities: %s. Create a weighted ranking if multiple priorities are given.

3. **Get detailed pricing**: For the top 3 hotels, use hotel_prices with their hotel_id to compare booking provider prices. Note which provider offers the best deal for each.

4. **Create comparison table**:
   | Hotel | Stars | Rating | Price/Night | Best Provider | Total Cost |
   Include amenities and location details where available.

5. **Recommend**: Based on the priorities (%s), recommend the best hotel with reasoning. Mention any trade-offs (e.g., "Hotel X is $20/night more but rated 4.8 vs 4.2").

6. **Budget alternatives**: If the top picks are expensive, suggest filtering by a lower star rating or searching a nearby area.`,
		location, checkIn, checkOut, priorities,
		location, checkIn, checkOut,
		priorities,
		priorities)

	return &PromptsGetResult{
		Description: fmt.Sprintf("Hotel comparison: %s, %s to %s", location, checkIn, checkOut),
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentBlock{Type: "text", Text: prompt},
			},
		},
	}, nil
}

func promptWhereShouldIGo(args map[string]any) (*PromptsGetResult, error) {
	origin := argOr(args, "origin", "")
	month := argOr(args, "month", "")
	budget := argOr(args, "budget", "")

	if origin == "" {
		return nil, fmt.Errorf("origin is required")
	}

	budgetLine := ""
	if budget != "" {
		budgetLine = fmt.Sprintf("\n\nMy flight budget is %s (local currency). Filter out destinations that exceed this budget.", budget)
	}

	monthLine := ""
	if month != "" {
		monthLine = fmt.Sprintf(" in %s", month)
	}

	prompt := fmt.Sprintf(`I want to travel from %s%s but I'm not sure where to go.%s

Follow these steps:

1. **Explore destinations**: Use explore_destinations with origin=%s to discover available destinations and their prices. Note the cheapest 10-15 options.

2. **Filter and rank**: From the results:
   - Filter by budget if specified
   - Group by region (Europe, Asia, Americas, etc.)
   - Highlight the top 3 cheapest destinations
   - Highlight any surprisingly affordable long-haul options

3. **Get destination details**: For the top 3 cheapest destinations, search for actual flights using search_flights to confirm availability and show specific options.

4. **Present recommendations**: Create a summary with:
   - A ranked list of top destinations with prices, airlines, and stop counts
   - A "Best value" pick (cheapest with good connectivity)
   - A "Hidden gem" pick (surprisingly affordable or interesting destination)
   - A "Premium pick" (best destination if budget allows)

5. **Next steps**: Suggest searching hotels at the top pick, or checking flexible dates with search_dates for the recommended destination.`,
		origin, monthLine, budgetLine, origin)

	desc := fmt.Sprintf("Destination discovery from %s", origin)
	if month != "" {
		desc += " in " + month
	}

	return &PromptsGetResult{
		Description: desc,
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: ContentBlock{Type: "text", Text: prompt},
			},
		},
	}, nil
}
