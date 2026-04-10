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
