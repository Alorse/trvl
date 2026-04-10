package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// getPreferencesTool returns the MCP tool definition for get_preferences.
func getPreferencesTool() ToolDef {
	return ToolDef{
		Name:        "get_preferences",
		Title:       "Get User Preferences",
		Description: "Returns the user's personal travel preferences including home airports, accommodation requirements, currency, and loyalty programmes. Use this to personalise search results before calling search_hotels or search_flights.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
			Required:   []string{},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"home_airports":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"home_cities":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"carry_on_only":       map[string]interface{}{"type": "boolean"},
				"prefer_direct":       map[string]interface{}{"type": "boolean"},
				"no_dormitories":      map[string]interface{}{"type": "boolean"},
				"ensuite_only":        map[string]interface{}{"type": "boolean"},
				"fast_wifi_needed":    map[string]interface{}{"type": "boolean"},
				"min_hotel_stars":     map[string]interface{}{"type": "integer"},
				"min_hotel_rating":    map[string]interface{}{"type": "number"},
				"display_currency":    map[string]interface{}{"type": "string"},
				"locale":              map[string]interface{}{"type": "string"},
				"loyalty_airlines":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"loyalty_hotels":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"preferred_districts": map[string]interface{}{"type": "object"},
				"default_companions":     map[string]interface{}{"type": "integer"},
				"trip_types":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"seat_preference":        map[string]interface{}{"type": "string"},
				"budget_per_night_min":   map[string]interface{}{"type": "number"},
				"budget_per_night_max":   map[string]interface{}{"type": "number"},
				"budget_flight_max":      map[string]interface{}{"type": "number"},
				"deal_tolerance":         map[string]interface{}{"type": "string"},
				"flight_time_earliest":   map[string]interface{}{"type": "string"},
				"flight_time_latest":     map[string]interface{}{"type": "string"},
				"red_eye_ok":             map[string]interface{}{"type": "boolean"},
				"nationality":            map[string]interface{}{"type": "string"},
				"languages":              map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"previous_trips":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"bucket_list":            map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"activity_preferences":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"dietary_needs":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"notes":                  map[string]interface{}{"type": "string"},
				"family_members": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":         map[string]interface{}{"type": "string"},
							"relationship": map[string]interface{}{"type": "string"},
							"notes":        map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
		Annotations: &ToolAnnotations{
			Title:          "Get User Preferences",
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}
}

// handleGetPreferences returns the user's preferences as structured data.
func handleGetPreferences(args map[string]any, _ ElicitFunc, _ SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	p, err := preferences.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load preferences: %w", err)
	}

	var summary string
	if len(p.HomeAirports) > 0 {
		summary = fmt.Sprintf("Home airports: %v. Display currency: %s.", p.HomeAirports, p.DisplayCurrency)
	} else {
		summary = fmt.Sprintf("No home airports set. Display currency: %s.", p.DisplayCurrency)
	}

	var filters []string
	if p.MinHotelRating > 0 {
		filters = append(filters, fmt.Sprintf("min rating %.1f", p.MinHotelRating))
	}
	if p.MinHotelStars > 0 {
		filters = append(filters, fmt.Sprintf("min %d stars", p.MinHotelStars))
	}
	if p.NoDormitories {
		filters = append(filters, "no dormitories")
	}
	if p.EnSuiteOnly {
		filters = append(filters, "en-suite only")
	}
	if len(filters) > 0 {
		summary += " Hotel filters: " + joinStrings(filters, ", ") + "."
	}

	// Budget summary.
	if p.BudgetPerNightMax > 0 {
		if p.BudgetPerNightMin > 0 {
			summary += fmt.Sprintf(" Budget: %.0f-%.0f/night.", p.BudgetPerNightMin, p.BudgetPerNightMax)
		} else {
			summary += fmt.Sprintf(" Budget: up to %.0f/night.", p.BudgetPerNightMax)
		}
	}
	if p.BudgetFlightMax > 0 {
		summary += fmt.Sprintf(" Max flight: %.0f.", p.BudgetFlightMax)
	}

	// Identity summary.
	if p.Nationality != "" {
		summary += fmt.Sprintf(" Nationality: %s.", p.Nationality)
	}
	if len(p.Languages) > 0 {
		summary += fmt.Sprintf(" Languages: %v.", p.Languages)
	}

	// Travel style summary.
	if len(p.TripTypes) > 0 {
		summary += fmt.Sprintf(" Trip types: %v.", p.TripTypes)
	}
	if p.SeatPreference != "" && p.SeatPreference != "no_preference" {
		summary += fmt.Sprintf(" Seat: %s.", p.SeatPreference)
	}
	if p.DealTolerance != "" {
		summary += fmt.Sprintf(" Deal tolerance: %s.", p.DealTolerance)
	}
	if p.Notes != "" {
		summary += fmt.Sprintf(" Notes: %s", p.Notes)
	}

	content, err := buildAnnotatedContentBlocks(summary, p)
	if err != nil {
		return nil, nil, err
	}
	return content, p, nil
}

// updatePreferencesTool returns the MCP tool definition for update_preferences.
func updatePreferencesTool() ToolDef {
	return ToolDef{
		Name:  "update_preferences",
		Title: "Update User Preferences",
		Description: `Updates the user's travel preferences by merging provided fields into the existing profile. Only the fields you include are changed — all other preferences are preserved.

When to use this tool:
- After the initial preference interview to save what the user told you.
- When the user explicitly mentions a preference change (e.g. "I got Star Alliance Gold", "I moved to Amsterdam").
- When you observe a strong pattern (e.g. user searched 4-star hotels 5 times) — but ALWAYS confirm with the user before saving.

You MUST confirm with the user before calling this tool. Never update silently.`,
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"home_airports":       {Type: "string", Description: "JSON array of IATA codes, e.g. [\"HEL\",\"AMS\"]. Replaces existing list."},
				"home_cities":         {Type: "string", Description: "JSON array of city names, e.g. [\"Helsinki\",\"Amsterdam\"]. Replaces existing list."},
				"carry_on_only":       {Type: "boolean", Description: "True if user only travels with carry-on luggage."},
				"prefer_direct":       {Type: "boolean", Description: "True if user prefers direct flights over connections."},
				"no_dormitories":      {Type: "boolean", Description: "True to exclude hostels and shared-room accommodation."},
				"ensuite_only":        {Type: "boolean", Description: "True to require private bathroom in all accommodation."},
				"fast_wifi_needed":    {Type: "boolean", Description: "True if user needs co-working capable fast wifi."},
				"min_hotel_stars":     {Type: "integer", Description: "Minimum hotel star rating (0-5). 0 means no minimum."},
				"min_hotel_rating":    {Type: "number", Description: "Minimum hotel review score (e.g. 4.0). 0 means no minimum."},
				"display_currency":    {Type: "string", Description: "ISO 4217 currency code for display, e.g. \"EUR\"."},
				"locale":             {Type: "string", Description: "BCP 47 locale tag, e.g. \"en-FI\"."},
				"loyalty_airlines":    {Type: "string", Description: "JSON array of airline IATA codes, e.g. [\"KL\",\"AY\"]. Replaces existing list."},
				"loyalty_hotels":      {Type: "string", Description: "JSON array of hotel programme names, e.g. [\"Marriott Bonvoy\"]. Replaces existing list."},
				"preferred_districts": {Type: "string", Description: "JSON object mapping city names to district arrays, e.g. {\"Prague\":[\"Prague 1\",\"Prague 2\"]}. Merged with existing districts (new cities added, existing cities replaced)."},
				"family_members":      {Type: "string", Description: "JSON array of family member objects with name, relationship, and notes fields. Replaces entire family list."},
				// Travel style (extended)
				"default_companions":    {Type: "integer", Description: "Default number of companions. 0 = solo, 1 = couple, 2+ = family/group."},
				"trip_types":            {Type: "string", Description: "JSON array of trip types, e.g. [\"city_break\",\"beach\",\"adventure\"]. Replaces existing list."},
				"seat_preference":       {Type: "string", Description: "Preferred seat: \"window\", \"aisle\", or \"no_preference\"."},
				// Budget
				"budget_per_night_min":  {Type: "number", Description: "Minimum acceptable hotel price per night (filters too-cheap-to-trust)."},
				"budget_per_night_max":  {Type: "number", Description: "Maximum hotel price per night."},
				"budget_flight_max":     {Type: "number", Description: "Maximum one-way flight price."},
				"deal_tolerance":        {Type: "string", Description: "Price sensitivity: \"price\" (6am flight to save money), \"comfort\" (pay for convenience), or \"balanced\"."},
				// Flight preferences
				"flight_time_earliest":  {Type: "string", Description: "Earliest acceptable flight departure time, e.g. \"06:00\"."},
				"flight_time_latest":    {Type: "string", Description: "Latest acceptable flight departure time, e.g. \"23:00\"."},
				"red_eye_ok":            {Type: "boolean", Description: "True if overnight (red-eye) flights are acceptable."},
				// Identity
				"nationality":           {Type: "string", Description: "ISO 3166-1 alpha-2 country code, e.g. \"FI\". Used for visa warnings."},
				"languages":             {Type: "string", Description: "JSON array of spoken language codes, e.g. [\"en\",\"fi\",\"sv\"]. Replaces existing list."},
				// Context (personalization)
				"previous_trips":        {Type: "string", Description: "JSON array of cities/countries visited, e.g. [\"Japan\",\"Barcelona\"]. Replaces existing list."},
				"bucket_list":           {Type: "string", Description: "JSON array of dream destinations, e.g. [\"New Zealand\",\"Iceland\"]. Replaces existing list."},
				"activity_preferences":  {Type: "string", Description: "JSON array of preferred activities, e.g. [\"museums\",\"food\",\"nature\"]. Replaces existing list."},
				"dietary_needs":         {Type: "string", Description: "JSON array of dietary requirements, e.g. [\"vegetarian\",\"gluten_free\"]. Replaces existing list."},
				"notes":                 {Type: "string", Description: "Free-text notes for anything that doesn't fit another field."},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"home_airports":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"home_cities":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"carry_on_only":       map[string]interface{}{"type": "boolean"},
				"prefer_direct":       map[string]interface{}{"type": "boolean"},
				"no_dormitories":      map[string]interface{}{"type": "boolean"},
				"ensuite_only":        map[string]interface{}{"type": "boolean"},
				"fast_wifi_needed":    map[string]interface{}{"type": "boolean"},
				"min_hotel_stars":     map[string]interface{}{"type": "integer"},
				"min_hotel_rating":    map[string]interface{}{"type": "number"},
				"display_currency":    map[string]interface{}{"type": "string"},
				"locale":              map[string]interface{}{"type": "string"},
				"loyalty_airlines":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"loyalty_hotels":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"preferred_districts": map[string]interface{}{"type": "object"},
				"default_companions":     map[string]interface{}{"type": "integer"},
				"trip_types":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"seat_preference":        map[string]interface{}{"type": "string"},
				"budget_per_night_min":   map[string]interface{}{"type": "number"},
				"budget_per_night_max":   map[string]interface{}{"type": "number"},
				"budget_flight_max":      map[string]interface{}{"type": "number"},
				"deal_tolerance":         map[string]interface{}{"type": "string"},
				"flight_time_earliest":   map[string]interface{}{"type": "string"},
				"flight_time_latest":     map[string]interface{}{"type": "string"},
				"red_eye_ok":             map[string]interface{}{"type": "boolean"},
				"nationality":            map[string]interface{}{"type": "string"},
				"languages":              map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"previous_trips":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"bucket_list":            map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"activity_preferences":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"dietary_needs":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"notes":                  map[string]interface{}{"type": "string"},
				"family_members": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":         map[string]interface{}{"type": "string"},
							"relationship": map[string]interface{}{"type": "string"},
							"notes":        map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
		Annotations: &ToolAnnotations{
			Title:           "Update User Preferences",
			ReadOnlyHint:    false,
			DestructiveHint: false,
			IdempotentHint:  true,
		},
	}
}

// handleUpdatePreferences merges provided fields into the existing preferences
// and saves back to disk. Only fields present in the request are updated.
func handleUpdatePreferences(args map[string]any, _ ElicitFunc, _ SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	return handleUpdatePreferencesWithPath(args, "", progress)
}

// handleUpdatePreferencesWithPath is the testable core: when path is "" it uses
// the default ~/.trvl/preferences.json; otherwise it uses the given path.
func handleUpdatePreferencesWithPath(args map[string]any, path string, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	// Load current preferences.
	var (
		p   *preferences.Preferences
		err error
	)
	if path != "" {
		p, err = preferences.LoadFrom(path)
	} else {
		p, err = preferences.Load()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load preferences: %w", err)
	}

	// Merge provided fields.
	updated := mergePreferenceArgs(p, args)

	// Save.
	if path != "" {
		err = preferences.SaveTo(path, updated)
	} else {
		err = preferences.Save(updated)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("save preferences: %w", err)
	}

	summary := "Preferences updated."
	content, err := buildAnnotatedContentBlocks(summary, updated)
	if err != nil {
		return nil, nil, err
	}
	return content, updated, nil
}

// mergePreferenceArgs applies only the fields present in args to the
// preferences struct. Unrecognised keys are silently ignored.
func mergePreferenceArgs(p *preferences.Preferences, args map[string]any) *preferences.Preferences {
	if args == nil {
		return p
	}

	// String slices: passed as JSON array string or []any from MCP.
	if v := argStringSliceOrJSON(args, "home_airports"); v != nil {
		p.HomeAirports = v
	}
	if v := argStringSliceOrJSON(args, "home_cities"); v != nil {
		p.HomeCities = v
	}
	if v := argStringSliceOrJSON(args, "loyalty_airlines"); v != nil {
		p.LoyaltyAirlines = v
	}
	if v := argStringSliceOrJSON(args, "loyalty_hotels"); v != nil {
		p.LoyaltyHotels = v
	}

	// Booleans: only update if the key is present.
	if _, ok := args["carry_on_only"]; ok {
		p.CarryOnOnly = argBool(args, "carry_on_only", p.CarryOnOnly)
	}
	if _, ok := args["prefer_direct"]; ok {
		p.PreferDirect = argBool(args, "prefer_direct", p.PreferDirect)
	}
	if _, ok := args["no_dormitories"]; ok {
		p.NoDormitories = argBool(args, "no_dormitories", p.NoDormitories)
	}
	if _, ok := args["ensuite_only"]; ok {
		p.EnSuiteOnly = argBool(args, "ensuite_only", p.EnSuiteOnly)
	}
	if _, ok := args["fast_wifi_needed"]; ok {
		p.FastWifiNeeded = argBool(args, "fast_wifi_needed", p.FastWifiNeeded)
	}

	// Numeric fields.
	if _, ok := args["min_hotel_stars"]; ok {
		p.MinHotelStars = argInt(args, "min_hotel_stars", p.MinHotelStars)
	}
	if _, ok := args["min_hotel_rating"]; ok {
		p.MinHotelRating = argFloat(args, "min_hotel_rating", p.MinHotelRating)
	}

	// Simple strings.
	if v, ok := args["display_currency"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.DisplayCurrency = s
		}
	}
	if v, ok := args["locale"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.Locale = s
		}
	}

	// Preferred districts: merge (add new cities, replace existing).
	if v, ok := args["preferred_districts"]; ok {
		mergeDistricts(p, v)
	}

	// --- New fields: travel style (extended) ---
	if _, ok := args["default_companions"]; ok {
		p.DefaultCompanions = argInt(args, "default_companions", p.DefaultCompanions)
	}
	if v := argStringSliceOrJSON(args, "trip_types"); v != nil {
		p.TripTypes = v
	}
	if v, ok := args["seat_preference"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.SeatPreference = s
		}
	}

	// --- New fields: budget ---
	if _, ok := args["budget_per_night_min"]; ok {
		p.BudgetPerNightMin = argFloat(args, "budget_per_night_min", p.BudgetPerNightMin)
	}
	if _, ok := args["budget_per_night_max"]; ok {
		p.BudgetPerNightMax = argFloat(args, "budget_per_night_max", p.BudgetPerNightMax)
	}
	if _, ok := args["budget_flight_max"]; ok {
		p.BudgetFlightMax = argFloat(args, "budget_flight_max", p.BudgetFlightMax)
	}
	if v, ok := args["deal_tolerance"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.DealTolerance = s
		}
	}

	// --- New fields: flight preferences ---
	if v, ok := args["flight_time_earliest"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.FlightTimeEarliest = s
		}
	}
	if v, ok := args["flight_time_latest"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.FlightTimeLatest = s
		}
	}
	if _, ok := args["red_eye_ok"]; ok {
		p.RedEyeOK = argBool(args, "red_eye_ok", p.RedEyeOK)
	}

	// --- New fields: identity ---
	if v, ok := args["nationality"]; ok {
		if s, ok := v.(string); ok && s != "" {
			p.Nationality = s
		}
	}
	if v := argStringSliceOrJSON(args, "languages"); v != nil {
		p.Languages = v
	}

	// --- New fields: context / personalization ---
	if v := argStringSliceOrJSON(args, "previous_trips"); v != nil {
		p.PreviousTrips = v
	}
	if v := argStringSliceOrJSON(args, "bucket_list"); v != nil {
		p.BucketList = v
	}
	if v := argStringSliceOrJSON(args, "activity_preferences"); v != nil {
		p.ActivityPreferences = v
	}
	if v := argStringSliceOrJSON(args, "dietary_needs"); v != nil {
		p.DietaryNeeds = v
	}
	if v, ok := args["notes"]; ok {
		if s, ok := v.(string); ok {
			p.Notes = s
		}
	}

	// Family members: full replacement.
	if v, ok := args["family_members"]; ok {
		mergeFamilyMembers(p, v)
	}

	return p
}

// argStringSliceOrJSON extracts a string slice from args. Handles:
// - []any (native JSON array from MCP)
// - string containing a JSON array (e.g. "[\"HEL\",\"AMS\"]")
// - comma-separated string (fallback)
// Returns nil if the key is absent (meaning: don't update).
func argStringSliceOrJSON(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}

	// Native []any from JSON.
	if arr, ok := v.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, elem := range arr {
			if s, ok := elem.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	// String: try JSON parse first, then comma-separated.
	if s, ok := v.(string); ok && s != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			return parsed
		}
		// Fallback to argStringSlice which handles comma-separated.
		return argStringSlice(args, key)
	}

	return nil
}

// mergeDistricts merges preferred_districts from the args value into the
// preferences. Accepts map[string]any (native) or a JSON string.
func mergeDistricts(p *preferences.Preferences, v any) {
	var districts map[string][]string

	switch val := v.(type) {
	case map[string]any:
		districts = make(map[string][]string, len(val))
		for city, arr := range val {
			if list, ok := arr.([]any); ok {
				var ds []string
				for _, d := range list {
					if s, ok := d.(string); ok {
						ds = append(ds, s)
					}
				}
				districts[city] = ds
			}
		}
	case string:
		if val == "" {
			return
		}
		if err := json.Unmarshal([]byte(val), &districts); err != nil {
			return
		}
	default:
		return
	}

	if p.PreferredDistricts == nil {
		p.PreferredDistricts = make(map[string][]string)
	}
	for city, ds := range districts {
		p.PreferredDistricts[city] = ds
	}
}

// mergeFamilyMembers replaces the family members list from the args value.
// Accepts []any (native) or a JSON string.
func mergeFamilyMembers(p *preferences.Preferences, v any) {
	var members []preferences.FamilyMember

	switch val := v.(type) {
	case []any:
		members = parseFamilyMemberSlice(val)
	case string:
		if val == "" {
			return
		}
		var raw []any
		if err := json.Unmarshal([]byte(val), &raw); err != nil {
			return
		}
		members = parseFamilyMemberSlice(raw)
	default:
		return
	}

	p.FamilyMembers = members
}

// parseFamilyMemberSlice converts []any to []FamilyMember.
func parseFamilyMemberSlice(raw []any) []preferences.FamilyMember {
	var members []preferences.FamilyMember
	for _, elem := range raw {
		m, ok := elem.(map[string]any)
		if !ok {
			continue
		}
		fm := preferences.FamilyMember{}
		if name, ok := m["name"].(string); ok {
			fm.Name = name
		}
		if rel, ok := m["relationship"].(string); ok {
			fm.Relationship = rel
		}
		if notes, ok := m["notes"].(string); ok {
			fm.Notes = notes
		}
		if fm.Name != "" { // skip entries without a name
			members = append(members, fm)
		}
	}
	return members
}

// joinStrings joins a slice with sep (avoids importing strings in this file).
func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
