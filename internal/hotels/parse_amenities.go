package hotels

import "strings"

// amenityCodeMap maps Google Hotels amenity codes to human-readable names.
var amenityCodeMap = map[int]string{
	1:  "air_conditioning",
	2:  "free_wifi",
	4:  "pool",
	7:  "fitness_center",
	8:  "spa",
	11: "accessible",
	14: "pet_friendly",
	15: "airport_shuttle",
	17: "room_service",
	18: "ev_charger",
	22: "restaurant",
	23: "free_parking",
	24: "paid_parking",
	26: "bar",
	27: "breakfast",
	29: "kitchen",
	31: "laundry",
	54: "accessible",
}

// extractAmenityCodes parses structured amenity data from entry[10].
//
// The structure is a nested array where the last non-nil sub-array contains
// pairs like [1, 54], [1, 29], etc. The first element indicates availability
// (1 = available), and the second is the amenity code.
func extractAmenityCodes(raw any) []string {
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return nil
	}

	// Find the last non-nil sub-array which contains the amenity pairs.
	var pairs []any
	for i := len(arr) - 1; i >= 0; i-- {
		if sub, ok := arr[i].([]any); ok && len(sub) > 0 {
			// Check if the first element looks like an amenity pair [int, int].
			if first, ok := sub[0].([]any); ok && len(first) == 2 {
				pairs = sub
				break
			}
		}
	}

	if pairs == nil {
		return nil
	}

	seen := make(map[string]bool)
	var amenities []string

	for _, p := range pairs {
		pair, ok := p.([]any)
		if !ok || len(pair) != 2 {
			continue
		}

		available, ok := pair[0].(float64)
		if !ok || available != 1 {
			continue
		}

		code, ok := pair[1].(float64)
		if !ok {
			continue
		}

		name, known := amenityCodeMap[int(code)]
		if !known {
			continue
		}

		if !seen[name] {
			seen[name] = true
			amenities = append(amenities, name)
		}
	}

	return amenities
}

// descriptionAmenityKeywords maps text keywords to amenity names for
// fallback extraction from hotel descriptions.
var descriptionAmenityKeywords = map[string]string{
	"pool":       "pool",
	"spa":        "spa",
	"gym":        "fitness_center",
	"fitness":    "fitness_center",
	"wifi":       "free_wifi",
	"wi-fi":      "free_wifi",
	"parking":    "free_parking",
	"breakfast":  "breakfast",
	"restaurant": "restaurant",
	"bar":        "bar",
	"kitchen":    "kitchen",
	"laundry":    "laundry",
}

// enrichAmenitiesFromDescription scans a description string for amenity
// keywords and adds any that are not already present.
func enrichAmenitiesFromDescription(existing []string, description string) []string {
	if description == "" {
		return existing
	}

	seen := make(map[string]bool, len(existing))
	for _, a := range existing {
		seen[a] = true
	}

	lower := strings.ToLower(description)
	result := append([]string(nil), existing...)

	for keyword, amenity := range descriptionAmenityKeywords {
		if seen[amenity] {
			continue
		}
		if strings.Contains(lower, keyword) {
			seen[amenity] = true
			result = append(result, amenity)
		}
	}

	return result
}
