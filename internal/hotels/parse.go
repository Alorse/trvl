package hotels

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// extractBatchPayload extracts the inner JSON payload from a batchexecute
// response entry. The batchexecute response format nests the actual data as
// a JSON string inside the response array.
//
// The response structure is:
//
//	[["wrb.fr","<rpcid>","<JSON-stringified payload>", ...], ...]
//
// This function finds the first entry matching the given rpcid and returns
// the parsed inner payload.
func extractBatchPayload(entries []any, rpcid string) (any, error) {
	for _, entry := range entries {
		arr, ok := entry.([]any)
		if !ok {
			continue
		}

		// Each entry in a batchexecute response is an array of response items.
		for _, item := range arr {
			itemArr, ok := item.([]any)
			if !ok {
				continue
			}
			if len(itemArr) < 3 {
				continue
			}

			// Check if this item matches our rpcid.
			// Format: ["wrb.fr", "<rpcid>", "<json-string>", ...]
			id, ok := itemArr[1].(string)
			if !ok || id != rpcid {
				continue
			}

			payloadStr, ok := itemArr[2].(string)
			if !ok {
				continue
			}

			var payload any
			if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
				return nil, fmt.Errorf("parse %s payload: %w", rpcid, err)
			}
			return payload, nil
		}
	}

	// Fallback: try treating entries directly as the batch array.
	// Sometimes DecodeBatchResponse returns the inner array directly.
	for _, entry := range entries {
		arr, ok := entry.([]any)
		if !ok || len(arr) < 3 {
			continue
		}
		id, ok := arr[1].(string)
		if !ok || id != rpcid {
			continue
		}
		payloadStr, ok := arr[2].(string)
		if !ok {
			continue
		}
		var payload any
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			return nil, fmt.Errorf("parse %s payload: %w", rpcid, err)
		}
		return payload, nil
	}

	return nil, fmt.Errorf("no response found for rpcid %s", rpcid)
}

// ParseHotelSearchResponse parses hotel search results from a decoded
// batchexecute response. It tries multiple known locations within the deeply
// nested response structure to find hotel data.
//
// The response from AtySUc contains nested arrays. Hotels are typically found
// within the result array at various depths depending on the query type.
func ParseHotelSearchResponse(entries []any, currency string) ([]models.HotelResult, error) {
	// Try to extract the AtySUc payload first.
	payload, err := extractBatchPayload(entries, "AtySUc")
	if err != nil {
		// If we can't find it by rpcid, try parsing the raw entries directly.
		// Sometimes the response is already the inner payload.
		return parseHotelsFromRaw(entries, currency)
	}

	return parseHotelsFromPayload(payload, currency)
}

// parseHotelsFromPayload extracts hotels from the AtySUc response payload.
// The payload is a deeply nested array structure. We search through it
// recursively looking for arrays that match the hotel entry signature.
func parseHotelsFromPayload(payload any, currency string) ([]models.HotelResult, error) {
	var hotels []models.HotelResult

	// The hotel data is typically at payload[1] or payload[0][1] as a list
	// of hotel entries. Each hotel entry is an array with specific structure.
	// Since the exact nesting can vary, we use a heuristic search approach.
	found := findHotelArrays(payload, 0)
	for _, h := range found {
		hotel := parseOneHotel(h, currency)
		if hotel.Name != "" {
			hotels = append(hotels, hotel)
		}
	}

	if len(hotels) == 0 {
		return nil, fmt.Errorf("no hotels found in response payload")
	}

	return hotels, nil
}

// findHotelArrays recursively searches the response structure for arrays
// that look like hotel entries. A hotel entry is an array where:
//   - It contains a string (hotel name) at a low index
//   - It contains what looks like a numeric rating
//   - It contains what looks like a price
//
// maxDepth limits recursion to avoid infinite loops on self-referencing data.
func findHotelArrays(v any, depth int) [][]any {
	if depth > 8 {
		return nil
	}

	arr, ok := v.([]any)
	if !ok {
		return nil
	}

	var results [][]any

	// Check if this array itself looks like a list of hotel entries.
	if looksLikeHotelList(arr) {
		for _, item := range arr {
			if hotelArr, ok := item.([]any); ok && looksLikeHotelEntry(hotelArr) {
				results = append(results, hotelArr)
			}
		}
		if len(results) > 0 {
			return results
		}
	}

	// Otherwise, recurse into sub-arrays.
	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			found := findHotelArrays(subArr, depth+1)
			if len(found) > 0 {
				return found
			}
		}
	}

	return nil
}

// looksLikeHotelList checks if an array looks like it contains hotel entries.
// A hotel list has multiple elements, most of which are arrays.
func looksLikeHotelList(arr []any) bool {
	if len(arr) < 2 {
		return false
	}
	arrayCount := 0
	hotelCount := 0
	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			arrayCount++
			if looksLikeHotelEntry(subArr) {
				hotelCount++
			}
		}
	}
	// At least 2 hotel-like entries makes it a likely hotel list.
	return hotelCount >= 2
}

// looksLikeHotelEntry checks if an array has the signature of a hotel entry.
// Hotel entries in Google's response typically have:
//   - A string name early in the array
//   - Nested arrays containing ratings, prices, coordinates
//   - Length >= 10 (hotel entries are deeply structured)
func looksLikeHotelEntry(arr []any) bool {
	if len(arr) < 5 {
		return false
	}

	// Look for a string that could be a hotel name in the first few elements.
	hasName := false
	for i := 0; i < minInt(5, len(arr)); i++ {
		if s, ok := arr[i].(string); ok && len(s) > 2 && !strings.HasPrefix(s, "/") {
			hasName = true
			break
		}
	}

	return hasName
}

// parseOneHotel extracts hotel fields from a single hotel entry array.
// This uses defensive indexing since the exact positions may vary.
func parseOneHotel(arr []any, currency string) models.HotelResult {
	h := models.HotelResult{Currency: currency}

	// Walk through the array looking for recognizable data types and patterns.
	for i, v := range arr {
		switch val := v.(type) {
		case string:
			if h.Name == "" && len(val) > 2 && !strings.HasPrefix(val, "/") && !strings.HasPrefix(val, "http") {
				h.Name = val
			} else if h.HotelID == "" && (strings.HasPrefix(val, "/g/") || strings.HasPrefix(val, "ChIJ") || strings.HasPrefix(val, "/m/")) {
				h.HotelID = val
			} else if h.Address == "" && len(val) > 10 && strings.Contains(val, ",") && i > 2 {
				h.Address = val
			}
		case float64:
			if val >= 1.0 && val <= 5.0 && h.Rating == 0 {
				h.Rating = val
			} else if val >= 1 && val <= 5 && val == float64(int(val)) && h.Stars == 0 && i > 3 {
				h.Stars = int(val)
			}
		case []any:
			// Nested array: check for coordinates [lat, lon], prices, amenities, etc.
			extractNestedHotelData(val, &h, currency)
		}
	}

	// If no explicit hotel ID was found, try deeper in nested arrays.
	if h.HotelID == "" {
		h.HotelID = findHotelID(arr)
	}

	return h
}

// extractNestedHotelData looks inside nested arrays for hotel data like
// coordinates, prices, and amenities.
func extractNestedHotelData(arr []any, h *models.HotelResult, currency string) {
	if len(arr) == 0 {
		return
	}

	// Check for coordinate pair: [float, float] where both are in lat/lon range.
	if len(arr) == 2 {
		lat, latOK := toFloat64(arr[0])
		lon, lonOK := toFloat64(arr[1])
		if latOK && lonOK && lat > -90 && lat < 90 && lon > -180 && lon < 180 {
			if h.Lat == 0 && h.Lon == 0 {
				h.Lat = lat
				h.Lon = lon
			}
		}
	}

	// Look for price-like values in sub-arrays.
	for _, item := range arr {
		switch val := item.(type) {
		case float64:
			if val > 10 && val < 100000 && h.Price == 0 {
				h.Price = val
			}
		case string:
			if h.HotelID == "" && (strings.HasPrefix(val, "/g/") || strings.HasPrefix(val, "ChIJ") || strings.HasPrefix(val, "/m/")) {
				h.HotelID = val
			}
			// Currency codes are 3 uppercase letters.
			if len(val) == 3 && val == strings.ToUpper(val) && h.Currency == "" {
				h.Currency = val
			}
		case []any:
			// Check for amenity strings list.
			if allStrings(val) && len(val) > 2 {
				amenities := toStringSlice(val)
				if len(amenities) > 0 && h.Amenities == nil {
					h.Amenities = amenities
				}
			}
			// Recurse one more level for prices and IDs.
			if h.Price == 0 || h.HotelID == "" {
				extractNestedHotelData(val, h, currency)
			}
		}
	}
}

// findHotelID searches deeply in an array for a Google place/entity ID.
func findHotelID(arr []any) string {
	for _, v := range arr {
		switch val := v.(type) {
		case string:
			if strings.HasPrefix(val, "/g/") || strings.HasPrefix(val, "ChIJ") || strings.HasPrefix(val, "/m/") {
				return val
			}
		case []any:
			if id := findHotelID(val); id != "" {
				return id
			}
		}
	}
	return ""
}

// ParseHotelPriceResponse parses hotel price lookup results from a decoded
// batchexecute response for the yY52ce rpcid.
func ParseHotelPriceResponse(entries []any) ([]models.ProviderPrice, error) {
	payload, err := extractBatchPayload(entries, "yY52ce")
	if err != nil {
		// Try parsing entries directly if rpcid extraction fails.
		return parsePricesFromRaw(entries)
	}

	return parsePricesFromPayload(payload)
}

// parsePricesFromPayload extracts provider prices from the yY52ce response.
// The response contains an array of booking providers with their prices.
func parsePricesFromPayload(payload any) ([]models.ProviderPrice, error) {
	var prices []models.ProviderPrice

	found := findPriceArrays(payload, 0)
	for _, p := range found {
		price := parseOneProvider(p)
		if price.Provider != "" && price.Price > 0 {
			prices = append(prices, price)
		}
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no provider prices found in response")
	}

	return prices, nil
}

// findPriceArrays searches the response for arrays that look like provider
// price entries.
func findPriceArrays(v any, depth int) [][]any {
	if depth > 8 {
		return nil
	}

	arr, ok := v.([]any)
	if !ok {
		return nil
	}

	var results [][]any

	// Check if this array looks like a list of provider entries.
	if looksLikePriceList(arr) {
		for _, item := range arr {
			if provArr, ok := item.([]any); ok && looksLikeProviderEntry(provArr) {
				results = append(results, provArr)
			}
		}
		if len(results) > 0 {
			return results
		}
	}

	// Recurse.
	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			found := findPriceArrays(subArr, depth+1)
			if len(found) > 0 {
				return found
			}
		}
	}

	return nil
}

// looksLikePriceList checks if an array looks like it contains provider entries.
func looksLikePriceList(arr []any) bool {
	if len(arr) < 1 {
		return false
	}
	provCount := 0
	for _, item := range arr {
		if subArr, ok := item.([]any); ok && looksLikeProviderEntry(subArr) {
			provCount++
		}
	}
	return provCount >= 1
}

// looksLikeProviderEntry checks if an array matches the provider price signature.
// Provider entries typically have a name string and a price number.
func looksLikeProviderEntry(arr []any) bool {
	if len(arr) < 2 {
		return false
	}
	hasName := false
	hasPrice := false
	for _, v := range arr {
		switch val := v.(type) {
		case string:
			// Provider names like "Booking.com", "Hotels.com", "Expedia", etc.
			if len(val) > 2 && !strings.HasPrefix(val, "http") && !strings.HasPrefix(val, "/") {
				hasName = true
			}
		case float64:
			if val > 10 && val < 100000 {
				hasPrice = true
			}
		}
	}
	return hasName && hasPrice
}

// parseOneProvider extracts provider name and price from a provider entry.
func parseOneProvider(arr []any) models.ProviderPrice {
	p := models.ProviderPrice{}
	for _, v := range arr {
		switch val := v.(type) {
		case string:
			if p.Provider == "" && len(val) > 2 && !strings.HasPrefix(val, "http") && !strings.HasPrefix(val, "/") {
				p.Provider = val
			}
			if len(val) == 3 && val == strings.ToUpper(val) {
				p.Currency = val
			}
		case float64:
			if val > 10 && val < 100000 && p.Price == 0 {
				p.Price = val
			}
		case []any:
			// Recurse one level to find price/currency in sub-arrays.
			for _, sub := range val {
				switch sv := sub.(type) {
				case float64:
					if sv > 10 && sv < 100000 && p.Price == 0 {
						p.Price = sv
					}
				case string:
					if len(sv) == 3 && sv == strings.ToUpper(sv) && p.Currency == "" {
						p.Currency = sv
					}
				}
			}
		}
	}
	return p
}

// parseHotelsFromRaw tries to extract hotels from raw decoded entries when
// the rpcid-based extraction fails.
func parseHotelsFromRaw(entries []any, currency string) ([]models.HotelResult, error) {
	var hotels []models.HotelResult
	for _, entry := range entries {
		found := findHotelArrays(entry, 0)
		for _, h := range found {
			hotel := parseOneHotel(h, currency)
			if hotel.Name != "" {
				hotels = append(hotels, hotel)
			}
		}
	}
	if len(hotels) == 0 {
		return nil, fmt.Errorf("no hotels found in raw response")
	}
	return hotels, nil
}

// parsePricesFromRaw tries to extract prices from raw decoded entries.
func parsePricesFromRaw(entries []any) ([]models.ProviderPrice, error) {
	var prices []models.ProviderPrice
	for _, entry := range entries {
		found := findPriceArrays(entry, 0)
		for _, p := range found {
			price := parseOneProvider(p)
			if price.Provider != "" && price.Price > 0 {
				prices = append(prices, price)
			}
		}
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("no provider prices found in raw response")
	}
	return prices, nil
}

// Helper functions.

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

func allStrings(arr []any) bool {
	for _, v := range arr {
		if _, ok := v.(string); !ok {
			return false
		}
	}
	return true
}

func toStringSlice(arr []any) []string {
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok && len(s) > 0 {
			result = append(result, s)
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
