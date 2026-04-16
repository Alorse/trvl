package providers

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// substituteVars replaces all ${var} placeholders in s with values from vars.
func substituteVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

// substituteEnvVars replaces ${env.VAR_NAME} placeholders with values from
// the process environment. This allows provider configs to reference API keys
// stored in environment variables without hardcoding them.
func substituteEnvVars(s string) string {
	if !strings.Contains(s, "${env.") {
		return s
	}
	// Find all ${env.XXX} patterns and replace.
	for {
		start := strings.Index(s, "${env.")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			break
		}
		varName := s[start+6 : start+end] // skip "${env." prefix
		envVal := os.Getenv(varName)
		s = s[:start] + envVal + s[start+end+1:]
	}
	return s
}

// jsonPath walks a parsed JSON value using dot-notation.
// Supports nested objects and traversal through arrays by iterating
// elements until it finds a non-empty match.
//
// When a path segment is applied to an array (e.g. Airbnb's
// explore_tabs.sections.listings where sections is an array of
// objects), the function iterates the array and returns the first
// element whose value for that segment is non-empty. Empty arrays
// and nil values are skipped so that metadata/ad sections (e.g.
// Airbnb "inserts" sections with listings:[]) don't shadow the real
// results in the next section.
func jsonPath(data any, path string) any {
	if path == "" {
		return data
	}
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			// Wildcard segment: "prefix*" matches the first map key whose
			// name starts with `prefix`. Enables paths like
			// `searchQueries.search*.results` against Apollo normalized
			// caches where the real key is `search({"input":...})`.
			if strings.HasSuffix(part, "*") {
				prefix := part[:len(part)-1]
				var matched any
				found := false
				for k, val := range v {
					if strings.HasPrefix(k, prefix) {
						matched = val
						found = true
						break
					}
				}
				if !found {
					return nil
				}
				current = matched
				continue
			}
			current = v[part]
		case []any:
			// Iterate the array and prefer the first element with a
			// non-empty value for this path segment. Falls back to
			// the first element with any value (including empty).
			var firstAny any
			foundAny := false
			var chosen any
			chose := false
			for _, elem := range v {
				m, ok := elem.(map[string]any)
				if !ok {
					continue
				}
				val, exists := m[part]
				if !exists {
					continue
				}
				if !foundAny {
					firstAny = val
					foundAny = true
				}
				if !isEmptyValue(val) {
					chosen = val
					chose = true
					break
				}
			}
			if chose {
				current = chosen
			} else if foundAny {
				current = firstAny
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return current
}

// denormalizeApollo recursively resolves __ref pointers in an Apollo
// normalized-cache JSON object. Every object of the form {"__ref": "Key:123"}
// is replaced by the actual entity stored at cache["Key:123"], with nested
// refs resolved recursively. This enables jsonPath to traverse SSR-hydrated
// Apollo data as if it were a plain denormalized JSON tree.
//
// The seen map guards against circular references (unlikely in Apollo, but
// defensive). Returns the resolved value — may be the original if no refs
// are present, so the operation is safe on non-Apollo JSON.
func denormalizeApollo(v any, cache map[string]any, seen map[string]bool) any {
	if seen == nil {
		seen = make(map[string]bool)
	}
	switch obj := v.(type) {
	case map[string]any:
		// Check for __ref pointer.
		if ref, ok := obj["__ref"].(string); ok && len(obj) <= 2 {
			if seen[ref] {
				return obj // circular — stop
			}
			seen[ref] = true
			if entity, exists := cache[ref]; exists {
				return denormalizeApollo(entity, cache, seen)
			}
			return obj // dangling ref
		}
		// Recursively resolve children.
		resolved := make(map[string]any, len(obj))
		for k, child := range obj {
			if k == "__typename" {
				resolved[k] = child
				continue
			}
			resolved[k] = denormalizeApollo(child, cache, seen)
		}
		return resolved
	case []any:
		resolved := make([]any, len(obj))
		for i, elem := range obj {
			resolved[i] = denormalizeApollo(elem, cache, seen)
		}
		return resolved
	default:
		return v
	}
}

// isEmptyValue reports whether v is nil, an empty slice, map, or string.
// Used by jsonPath to skip metadata/placeholder entries when traversing
// arrays of heterogeneous objects.
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	case string:
		return x == ""
	}
	return false
}

// resolveCityID finds the best-matching provider-specific city ID for a
// location string. Matches are case-insensitive and support partial matching
// (e.g. "Prague" → "praha"). Returns the empty string when no mapping exists
// or when the location is blank.
func resolveCityID(lookup map[string]string, location string) string {
	if len(lookup) == 0 {
		return ""
	}
	loc := strings.ToLower(strings.TrimSpace(location))
	if loc == "" {
		return ""
	}
	if id, ok := lookup[loc]; ok {
		return id
	}
	// Partial match: location contains a key, or key contains location.
	for city, id := range lookup {
		c := strings.ToLower(city)
		if c == "" {
			continue
		}
		if strings.Contains(loc, c) || strings.Contains(c, loc) {
			return id
		}
	}
	return ""
}

// resolvePropertyType maps a normalized property type name (e.g. "apartment",
// "hostel", "hotel") to a provider-specific identifier using the lookup table.
// Like resolveCityID, matching is case-insensitive. Returns the empty string
// when no mapping exists or when the property type is blank.
func resolvePropertyType(lookup map[string]string, propertyType string) string {
	if len(lookup) == 0 {
		return ""
	}
	pt := strings.ToLower(strings.TrimSpace(propertyType))
	if pt == "" {
		return ""
	}
	if id, ok := lookup[pt]; ok {
		return id
	}
	// Case-insensitive scan over keys.
	for key, id := range lookup {
		if strings.ToLower(key) == pt {
			return id
		}
	}
	return ""
}

// mapHotelResult maps a raw JSON object to a HotelResult using field mappings.
func mapHotelResult(raw any, fields map[string]string) models.HotelResult {
	var h models.HotelResult
	for modelField, jsonField := range fields {
		val := jsonPath(raw, jsonField)
		if val == nil {
			continue
		}
		switch modelField {
		case "name":
			h.Name, _ = val.(string)
		case "hotel_id":
			// Format IDs without scientific notation. JSON numbers deserialize
			// as float64 in Go; a hotel_id of 1042748 becomes 1.042748e+06
			// which is useless as an identifier. Detect whole-number floats
			// and format as integers.
			switch id := val.(type) {
			case float64:
				if id == float64(int64(id)) {
					h.HotelID = strconv.FormatInt(int64(id), 10)
				} else {
					h.HotelID = strconv.FormatFloat(id, 'f', -1, 64)
				}
			default:
				h.HotelID = fmt.Sprintf("%v", val)
			}
		case "rating":
			h.Rating = toFloat64(val)
		case "review_count":
			h.ReviewCount = toInt(val)
		case "stars":
			h.Stars = toInt(val)
		case "price":
			h.Price = toFloat64(val)
		case "currency":
			h.Currency, _ = val.(string)
		case "address":
			h.Address, _ = val.(string)
		case "lat":
			h.Lat = toFloat64(val)
		case "lon":
			h.Lon = toFloat64(val)
		case "booking_url":
			h.BookingURL, _ = val.(string)
		case "eco_certified":
			h.EcoCertified, _ = val.(bool)
		}
	}
	return h
}

// extractBlocksPriceSpread scans the "blocks" array on a raw hotel result
// (Booking.com SSR structure) and returns the maximum block price and the
// number of distinct room types. This gives the LLM a price spread signal
// ("cheapest room €120, most expensive €280, 4 room types") without
// requiring a separate hotel_rooms drill-down call.
func extractBlocksPriceSpread(raw any) (maxPrice float64, roomCount int) {
	blocksRaw := jsonPath(raw, "blocks")
	blocks, ok := blocksRaw.([]any)
	if !ok || len(blocks) == 0 {
		return 0, 0
	}
	seen := make(map[string]bool)
	for _, b := range blocks {
		price := toFloat64(jsonPath(b, "finalPrice.amount"))
		if price > maxPrice {
			maxPrice = price
		}
		// Count distinct room types by blockId.roomId.
		if roomID := fmt.Sprintf("%v", jsonPath(b, "blockId.roomId")); roomID != "<nil>" {
			seen[roomID] = true
		}
	}
	return maxPrice, len(seen)
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return f
		}
		// Strip currency symbols and whitespace (e.g. "€ 61" -> "61").
		cleaned := stripNonNumeric(n)
		if cleaned != "" {
			f, _ = strconv.ParseFloat(cleaned, 64)
			return f
		}
		return 0
	default:
		return 0
	}
}

// stripNonNumeric removes everything except digits, '.', and '-' from s.
// Used to extract a numeric value from currency-formatted strings like "€ 61".
func stripNonNumeric(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
