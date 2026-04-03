package hotels

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/jsonutil"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// parseHotelsFromPage extracts hotel data from a Google Travel Hotels HTML page.
//
// The page contains AF_initDataCallback blocks with JSON data. Hotel data
// is in the "ds:0" callback, nested deeply within map-keyed arrays.
//
// Two types of hotel entries exist:
//
// 1. Organic hotels at data[0][0][0][1][N][1]["397419284"][0]:
//   - [1] = hotel name
//   - [2][0] = [lat, lon]
//   - [3] = ["X-star hotel", X]
//   - [6] = price block
//   - [7][0] = [rating, review_count]
//   - [9] = Google Place ID
//   - [11] = description array
//
// 2. Sponsored hotels at data[0][0][0][1][1][1]["300000000"][2]:
//   - [0] = hotel name
//   - [2] = price string (e.g. "PLN 420")
//   - [4] = review count
//   - [5] = rating (float)
//   - [6] = provider name
func parseHotelsFromPage(page string, currency string) ([]models.HotelResult, error) {
	// Extract AF_initDataCallback data blocks from the HTML.
	callbacks := extractCallbacks(page)
	if len(callbacks) == 0 {
		return nil, fmt.Errorf("no AF_initDataCallback blocks found in page")
	}

	// Find the largest callback (typically "ds:0") which contains hotel data.
	var hotelData any
	maxSize := 0
	for _, cb := range callbacks {
		data, _ := json.Marshal(cb)
		if len(data) > maxSize {
			maxSize = len(data)
			hotelData = cb
		}
	}

	if hotelData == nil {
		return nil, fmt.Errorf("no parseable data callback found")
	}

	var hotels []models.HotelResult

	// Extract organic hotel entries.
	organic := extractOrganicHotels(hotelData, currency)
	hotels = append(hotels, organic...)

	// Extract sponsored/ad hotel entries.
	sponsored := extractSponsoredHotels(hotelData, currency)
	hotels = append(hotels, sponsored...)

	if len(hotels) == 0 {
		return nil, fmt.Errorf("no hotels found in response payload")
	}

	// Deduplicate by name (sponsored and organic can overlap).
	hotels = deduplicateHotels(hotels)

	return hotels, nil
}

// extractCallbacks extracts parsed JSON data from AF_initDataCallback blocks
// in an HTML page. Returns a slice of parsed JSON values.
func extractCallbacks(page string) []any {
	var results []any
	remaining := page

	for {
		idx := strings.Index(remaining, "AF_initDataCallback({")
		if idx < 0 {
			break
		}
		remaining = remaining[idx:]

		// Find the "data:" field.
		dataStart := strings.Index(remaining, "data:")
		if dataStart < 0 || dataStart > 500 {
			remaining = remaining[20:]
			continue
		}

		dataStr := strings.TrimSpace(remaining[dataStart+5:])
		if len(dataStr) == 0 || dataStr[0] != '[' {
			remaining = remaining[20:]
			continue
		}

		// Parse the JSON array.
		dec := json.NewDecoder(strings.NewReader(dataStr))
		var parsed any
		if err := dec.Decode(&parsed); err == nil {
			results = append(results, parsed)
		}

		remaining = remaining[20:]
	}

	return results
}

// extractOrganicHotels extracts organic (non-sponsored) hotel entries from
// the parsed hotel data.
//
// Organic hotels live at: data[0][0][0][1][N][1]{numericKey}[0]
// where N iterates over hotel indices and numericKey is typically "397419284".
func extractOrganicHotels(data any, currency string) []models.HotelResult {
	var hotels []models.HotelResult

	// Navigate to data[0][0][0][1]
	hotelList := jsonutil.NavigateArray(data, 0, 0, 0, 1)
	if hotelList == nil {
		return nil
	}

	arr, ok := hotelList.([]any)
	if !ok {
		return nil
	}

	for _, entry := range arr {
		entryArr, ok := entry.([]any)
		if !ok || len(entryArr) < 2 {
			continue
		}

		// entryArr[1] should be a map with a numeric key containing the hotel data.
		mapVal, ok := entryArr[1].(map[string]any)
		if !ok {
			continue
		}

		for key, val := range mapVal {
			// Skip the sponsored hotels key (300000000).
			if key == "300000000" {
				continue
			}

			hotelArr, ok := val.([]any)
			if !ok || len(hotelArr) == 0 {
				continue
			}

			// The hotel data is at hotelArr[0].
			hotelEntry, ok := hotelArr[0].([]any)
			if !ok || len(hotelEntry) < 3 {
				continue
			}

			hotel := parseOrganicHotel(hotelEntry, currency)
			if hotel.Name != "" {
				hotels = append(hotels, hotel)
			}
		}
	}

	return hotels
}

// parseOrganicHotel extracts hotel fields from an organic hotel entry array.
func parseOrganicHotel(entry []any, currency string) models.HotelResult {
	h := models.HotelResult{Currency: currency}

	// [1] = hotel name
	if len(entry) > 1 {
		h.Name = jsonutil.StringValue(entry[1])
	}

	// [2] = location info, [2][0] = [lat, lon]
	if len(entry) > 2 {
		if locArr, ok := entry[2].([]any); ok && len(locArr) > 0 {
			if coords, ok := locArr[0].([]any); ok && len(coords) >= 2 {
				if lat, ok := coords[0].(float64); ok {
					h.Lat = lat
				}
				if lon, ok := coords[1].(float64); ok {
					h.Lon = lon
				}
			}
		}
	}

	// [3] = ["X-star hotel", X] star rating
	if len(entry) > 3 {
		if starArr, ok := entry[3].([]any); ok && len(starArr) >= 2 {
			if stars, ok := starArr[1].(float64); ok {
				h.Stars = int(stars)
			}
		}
	}

	// [6] = price block: [null, [[price, 0], null, null, "currency", ...]]
	if len(entry) > 6 {
		price, cur := extractOrganicPrice(entry[6])
		if price > 0 {
			h.Price = price
		}
		if cur != "" {
			h.Currency = cur
		}
	}

	// [7][0] = [rating, review_count]
	if len(entry) > 7 {
		if ratingArr, ok := entry[7].([]any); ok && len(ratingArr) > 0 {
			if pair, ok := ratingArr[0].([]any); ok && len(pair) >= 2 {
				if rating, ok := pair[0].(float64); ok {
					h.Rating = rating
				}
				if reviews, ok := pair[1].(float64); ok {
					h.ReviewCount = int(reviews)
				}
			}
		}
	}

	// [9] = Google Place ID (hex entity ID)
	if len(entry) > 9 {
		if id := jsonutil.StringValue(entry[9]); id != "" {
			h.HotelID = id
		}
	}

	// [10] = amenity codes (structured data)
	if len(entry) > 10 {
		h.Amenities = extractAmenityCodes(entry[10])
	}

	// [11] = description array
	var description string
	if len(entry) > 11 {
		if descArr, ok := entry[11].([]any); ok && len(descArr) > 0 {
			if desc := jsonutil.StringValue(descArr[0]); desc != "" {
				h.Address = desc // Use description as address fallback
				description = desc
			}
		}
	}

	// Enrich amenities from description text (adds only new ones).
	if description != "" {
		h.Amenities = enrichAmenitiesFromDescription(h.Amenities, description)
	}

	return h
}

// extractOrganicPrice extracts price and currency from an organic hotel's
// price block.
//
// The price block at entry[6] has this structure:
//
//	[6][0] = null
//	[6][1] = search-wide params: [[maxPrice, 0], null, null, "currency", dates, ...]
//	[6][2] = actual per-hotel price: [null, ["€61", null, 60.72, null, 61], ...]
//	[6][3] = identifier string
//
// The per-hotel price is at [6][2][1], which contains:
//
//	[0] = formatted price string (e.g. "€61")
//	[1] = null
//	[2] = exact float price (e.g. 60.720634)
//	[3] = null
//	[4] = rounded integer price (e.g. 61)
//
// We prefer the rounded integer price at [4], falling back to the float at [2].
// Currency is extracted from [6][1][3].
func extractOrganicPrice(raw any) (float64, string) {
	arr, ok := raw.([]any)
	if !ok || len(arr) < 2 {
		return 0, ""
	}

	// Extract currency from [6][1][3] (search-wide params).
	var currency string
	if len(arr) > 1 {
		if searchParams, ok := arr[1].([]any); ok && len(searchParams) > 3 {
			currency = jsonutil.StringValue(searchParams[3])
		}
	}

	// Extract per-hotel price from [6][2][1].
	if len(arr) > 2 && arr[2] != nil {
		if priceOuter, ok := arr[2].([]any); ok && len(priceOuter) > 1 && priceOuter[1] != nil {
			if priceInfo, ok := priceOuter[1].([]any); ok {
				// Try rounded integer at [4] first.
				if len(priceInfo) > 4 {
					if price, ok := priceInfo[4].(float64); ok && price > 0 {
						return price, currency
					}
				}
				// Fall back to exact float at [2].
				if len(priceInfo) > 2 {
					if price, ok := priceInfo[2].(float64); ok && price > 0 {
						return price, currency
					}
				}
			}
		}
	}

	// Legacy fallback: look for [[price, 0], null, null, "currency", ...] in [6][1].
	for _, item := range arr {
		innerArr, ok := item.([]any)
		if !ok || len(innerArr) < 4 {
			continue
		}
		if priceArr, ok := innerArr[0].([]any); ok && len(priceArr) >= 1 {
			if price, ok := priceArr[0].(float64); ok && price > 0 {
				if len(innerArr) > 3 {
					if cur := jsonutil.StringValue(innerArr[3]); cur != "" {
						currency = cur
					}
				}
				return price, currency
			}
		}
	}

	return 0, ""
}

// extractSponsoredHotels extracts sponsored/ad hotel entries.
//
// Sponsored hotels live at: data[0][0][0][1][1][1]["300000000"][2]
// Each entry: [name, link, price_string, [image], review_count, rating, provider, ...]
func extractSponsoredHotels(data any, currency string) []models.HotelResult {
	var hotels []models.HotelResult

	// Navigate to data[0][0][0][1]
	hotelList := jsonutil.NavigateArray(data, 0, 0, 0, 1)
	if hotelList == nil {
		return nil
	}

	arr, ok := hotelList.([]any)
	if !ok {
		return nil
	}

	// Find the entry with the "300000000" key (sponsored).
	for _, entry := range arr {
		entryArr, ok := entry.([]any)
		if !ok || len(entryArr) < 2 {
			continue
		}

		mapVal, ok := entryArr[1].(map[string]any)
		if !ok {
			continue
		}

		sponsoredData, ok := mapVal["300000000"]
		if !ok {
			continue
		}

		sponsoredArr, ok := sponsoredData.([]any)
		if !ok || len(sponsoredArr) < 3 {
			continue
		}

		// The hotel list is at sponsoredArr[2].
		hotelEntries, ok := sponsoredArr[2].([]any)
		if !ok {
			continue
		}

		for _, rawHotel := range hotelEntries {
			hotelArr, ok := rawHotel.([]any)
			if !ok || len(hotelArr) < 6 {
				continue
			}

			hotel := parseSponsoredHotel(hotelArr, currency)
			if hotel.Name != "" {
				hotels = append(hotels, hotel)
			}
		}
	}

	return hotels
}

// parseSponsoredHotel extracts hotel fields from a sponsored hotel entry.
func parseSponsoredHotel(entry []any, currency string) models.HotelResult {
	h := models.HotelResult{Currency: currency}

	// [0] = hotel name
	if len(entry) > 0 {
		h.Name = jsonutil.StringValue(entry[0])
	}

	// [2] = price string (e.g. "PLN 420", "USD 150")
	if len(entry) > 2 {
		if priceStr := jsonutil.StringValue(entry[2]); priceStr != "" {
			price, cur := parsePriceString(priceStr)
			if price > 0 {
				h.Price = price
			}
			if cur != "" {
				h.Currency = cur
			}
		}
	}

	// [4] = review count
	if len(entry) > 4 {
		if reviews, ok := entry[4].(float64); ok {
			h.ReviewCount = int(reviews)
		}
	}

	// [5] = rating
	if len(entry) > 5 {
		if rating, ok := entry[5].(float64); ok {
			h.Rating = rating
		}
	}

	return h
}

// parsePriceString parses a price string like "PLN 420" or "USD 150.50".
func parsePriceString(s string) (float64, string) {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return 0, ""
	}

	// Try currency first, then amount.
	currency := parts[0]
	amountStr := strings.ReplaceAll(parts[1], ",", "")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		// Maybe the format is "420 PLN" (amount first).
		amountStr = strings.ReplaceAll(parts[0], ",", "")
		amount, err = strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return 0, ""
		}
		currency = parts[1]
	}

	// Validate currency looks like a currency code (3 uppercase letters).
	if len(currency) != 3 || currency != strings.ToUpper(currency) {
		currency = ""
	}

	return amount, currency
}

// deduplicateHotels removes duplicate hotels by name, keeping the first
// occurrence (organic hotels are added before sponsored, so they take priority).
func deduplicateHotels(hotels []models.HotelResult) []models.HotelResult {
	seen := make(map[string]bool)
	result := make([]models.HotelResult, 0, len(hotels))
	for _, h := range hotels {
		key := strings.ToLower(h.Name)
		if !seen[key] {
			seen[key] = true
			result = append(result, h)
		}
	}
	return result
}

// ParseHotelSearchResponse parses hotel search results from a decoded
// batchexecute response. This is the legacy path used when batchexecute
// responses are available. The Travel page scraping path (parseHotelsFromPage)
// is preferred.
func ParseHotelSearchResponse(entries []any, currency string) ([]models.HotelResult, error) {
	// Try to extract the AtySUc payload first.
	payload, err := extractBatchPayload(entries, "AtySUc")
	if err != nil {
		return parseHotelsFromRaw(entries, currency)
	}

	return parseHotelsFromPayload(payload, currency)
}

// extractBatchPayload extracts the inner JSON payload from a batchexecute
// response entry.
func extractBatchPayload(entries []any, rpcid string) (any, error) {
	for _, entry := range entries {
		arr, ok := entry.([]any)
		if !ok {
			continue
		}

		for _, item := range arr {
			itemArr, ok := item.([]any)
			if !ok || len(itemArr) < 3 {
				continue
			}

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

// parseHotelsFromPayload extracts hotels from the AtySUc response payload.
// It searches the nested map/array structure for hotel entries.
func parseHotelsFromPayload(payload any, currency string) ([]models.HotelResult, error) {
	var hotels []models.HotelResult

	// Search through the nested structure for hotel entries.
	found := findHotelEntries(payload, 0)
	for _, h := range found {
		hotel := parseHotelFromMapEntry(h, currency)
		if hotel.Name != "" {
			hotels = append(hotels, hotel)
		}
	}

	if len(hotels) == 0 {
		return nil, fmt.Errorf("no hotels found in response payload")
	}

	return hotels, nil
}

// findHotelEntries recursively searches for arrays that look like organic
// hotel entries (27-element arrays with name at [1] and coordinates at [2]).
func findHotelEntries(v any, depth int) [][]any {
	if depth > 10 {
		return nil
	}

	switch val := v.(type) {
	case []any:
		// Check if this looks like a hotel entry (name at [1], coords at [2]).
		if len(val) > 10 && val[0] == nil {
			if name, ok := val[1].(string); ok && len(name) > 2 {
				if locArr, ok := val[2].([]any); ok && len(locArr) > 0 {
					if coords, ok := locArr[0].([]any); ok && len(coords) == 2 {
						if _, ok := coords[0].(float64); ok {
							return [][]any{val}
						}
					}
				}
			}
		}
		// Recurse into sub-arrays.
		var results [][]any
		for _, item := range val {
			found := findHotelEntries(item, depth+1)
			results = append(results, found...)
		}
		return results

	case map[string]any:
		var results [][]any
		for _, mv := range val {
			found := findHotelEntries(mv, depth+1)
			results = append(results, found...)
		}
		return results
	}

	return nil
}

// parseHotelFromMapEntry parses a hotel from the organic hotel array format.
func parseHotelFromMapEntry(entry []any, currency string) models.HotelResult {
	return parseOrganicHotel(entry, currency)
}

// parseHotelsFromRaw tries to extract hotels from raw decoded entries.
func parseHotelsFromRaw(entries []any, currency string) ([]models.HotelResult, error) {
	var hotels []models.HotelResult
	for _, entry := range entries {
		found := findHotelEntries(entry, 0)
		for _, h := range found {
			hotel := parseHotelFromMapEntry(h, currency)
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
