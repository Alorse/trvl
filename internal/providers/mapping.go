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
			h.HotelID = fmt.Sprintf("%v", val)
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
