package providers

import (
	"encoding/json"
	"fmt"
)

// discoverArrayPaths walks a JSON value tree up to 4 levels deep, looking for
// arrays with 1+ elements that look like result sets (contain objects). Returns
// a suggestions map like {"results_path": "data.search.results (25 items)"}.
// excludePath is the path the caller already tried (omitted from suggestions).
func discoverArrayPaths(v any, excludePath string) map[string]string {
	suggestions := make(map[string]string)
	var walk func(val any, path string, depth int)
	walk = func(val any, path string, depth int) {
		if depth > 4 {
			return
		}
		switch t := val.(type) {
		case []any:
			if len(t) > 0 && path != excludePath {
				// Check if elements are objects (likely result items).
				if _, isObj := t[0].(map[string]any); isObj {
					suggestions["results_path"] = fmt.Sprintf("%s (%d items)", path, len(t))
				}
			}
		case map[string]any:
			for k, child := range t {
				childPath := k
				if path != "" {
					childPath = path + "." + k
				}
				walk(child, childPath, depth+1)
			}
		}
	}
	walk(v, "", 0)
	return suggestions
}

// discoverFieldMappings inspects a result object and suggests field_mapping
// entries by matching common hotel-like field names. Returns suggestions like
// {"field:name": "displayName.text", "field:price": "priceInfo.amount"}.
func discoverFieldMappings(obj map[string]any, prefix string) map[string]string {
	suggestions := make(map[string]string)

	// Common name patterns → target field.
	namePatterns := map[string]string{
		"name": "name", "hotel_name": "name", "hotelName": "name",
		"displayName": "name", "title": "name", "property_name": "name",
		"propertyName": "name",
	}
	pricePatterns := map[string]string{
		"price": "price", "amount": "price", "total": "price",
		"amountPerStay": "price", "displayPrice": "price",
		"pricePerNight": "price", "rate": "price",
	}
	idPatterns := map[string]string{
		"id": "hotel_id", "hotelId": "hotel_id", "hotel_id": "hotel_id",
		"propertyId": "hotel_id", "listingId": "hotel_id",
	}
	ratingPatterns := map[string]string{
		"rating": "rating", "score": "rating", "reviewScore": "rating",
		"starRating": "rating", "overallRating": "rating",
	}
	latPatterns := map[string]string{
		"latitude": "lat", "lat": "lat",
	}
	lonPatterns := map[string]string{
		"longitude": "lon", "lon": "lon", "lng": "lon",
	}

	allPatterns := []struct {
		field    string
		patterns map[string]string
	}{
		{"name", namePatterns},
		{"price", pricePatterns},
		{"hotel_id", idPatterns},
		{"rating", ratingPatterns},
		{"lat", latPatterns},
		{"lon", lonPatterns},
	}

	var scan func(val any, path string, depth int)
	scan = func(val any, path string, depth int) {
		if depth > 3 {
			return
		}
		m, ok := val.(map[string]any)
		if !ok {
			return
		}
		for k, child := range m {
			childPath := k
			if path != "" {
				childPath = path + "." + k
			}
			for _, pat := range allPatterns {
				if _, matched := pat.patterns[k]; matched {
					key := "field:" + pat.field
					// Prefer shallower paths.
					if _, exists := suggestions[key]; !exists {
						switch child.(type) {
						case string, float64, json.Number:
							suggestions[key] = childPath
						case map[string]any:
							// Nested — keep scanning for the leaf value.
							scan(child, childPath, depth+1)
						}
					}
				}
			}
			if childMap, isMap := child.(map[string]any); isMap {
				scan(childMap, childPath, depth+1)
			}
		}
	}
	scan(obj, prefix, 0)
	return suggestions
}
