package explore

import (
	"encoding/json"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// ParseExploreResponse parses the raw HTTP response from GetExploreDestinations
// into a slice of ExploreDestination.
//
// The response uses the standard FlightsFrontendUi format: anti-XSSI prefix,
// then length-prefixed JSON chunks. Each chunk contains an inner JSON string
// with destination data.
//
// Observed response structure (2026-04):
//
//	Entry 0 inner: [[session], null, [coords], [[dest1,dest2,...],...]]
//	  -> destinations at [3][0]
//	Entry 1 inner: [[session], null, null, null, [[dest1,dest2,...],...]]
//	  -> destinations at [4][0]
//
// Each destination: [cityID, [[null,price],"token"], null,null,null,null,
//
//	[airlineCode, airlineName, stops, duration, null, airportCode, ...],
//	null,null, visible, ...]
func ParseExploreResponse(body []byte) ([]models.ExploreDestination, error) {
	entries, err := batchexec.DecodeBatchResponse(body)
	if err != nil {
		inner, err2 := batchexec.DecodeFlightResponse(body)
		if err2 != nil {
			return nil, fmt.Errorf("decode failed: batch=%w, flight=%v", err, err2)
		}
		return parseExploreFromInner(inner)
	}

	var all []models.ExploreDestination

	for _, entry := range entries {
		inner, err := extractInnerJSON(entry)
		if err != nil {
			continue
		}
		dests, err := parseExploreFromInner(inner)
		if err != nil {
			continue
		}
		all = append(all, dests...)
	}

	return all, nil
}

// extractInnerJSON extracts the inner JSON string from a batch response entry.
// Entry format: ["wrb.fr", null, "<inner-json-string>", ...]
func extractInnerJSON(entry any) (any, error) {
	// Entry could be a flat array: ["wrb.fr", null, "json-string"]
	arr, ok := entry.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response entry format")
	}

	// Look for string element that contains JSON
	for _, elem := range arr {
		s, ok := elem.(string)
		if !ok || len(s) < 10 {
			continue
		}
		var inner any
		if err := json.Unmarshal([]byte(s), &inner); err != nil {
			continue
		}
		return inner, nil
	}

	return nil, fmt.Errorf("no inner JSON found")
}

// parseExploreFromInner parses destinations from the decoded inner JSON.
//
// Scans indices 3 and 4 for destination arrays, as the position varies
// between the different response chunks.
func parseExploreFromInner(data any) ([]models.ExploreDestination, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected explore data format")
	}

	// Try indices 3, 4, and 5 for the destination array
	for _, idx := range []int{3, 4, 5} {
		if idx >= len(arr) {
			continue
		}

		mainArr, ok := arr[idx].([]any)
		if !ok || len(mainArr) == 0 {
			continue
		}

		// Check if mainArr[0] is an array of destinations
		destsArr, ok := mainArr[0].([]any)
		if !ok || len(destsArr) == 0 {
			continue
		}

		// Verify first element looks like a destination (has string city ID at [0])
		firstDest, ok := destsArr[0].([]any)
		if !ok || len(firstDest) < 2 {
			continue
		}
		if _, ok := firstDest[0].(string); !ok {
			continue
		}

		dests := parseDestinationArray(destsArr)
		if len(dests) > 0 {
			return dests, nil
		}
	}

	return nil, fmt.Errorf("no destination array found in inner data (%d elements)", len(arr))
}

// parseDestinationArray parses individual destinations from the array.
//
// Destination format (observed 2026-04):
//
//	[0] = city ID (e.g. "/m/04llb")
//	[1] = price info: [[null, price], "booking_token"]
//	[2-5] = null/metadata
//	[6] = airline info: [code, name, stops, duration, null, airportCode, ...]
//
// Alternative format in entry[0] (with images):
//
//	[0] = city ID
//	[1] = [lat, lng]
//	[2] = city name
//	[3] = image URL
//	[4] = country name
//	... more metadata ...
//	[14] = airport code (e.g. "LIS")
func parseDestinationArray(destsArr []any) []models.ExploreDestination {
	var dests []models.ExploreDestination

	for _, item := range destsArr {
		dest, ok := item.([]any)
		if !ok || len(dest) < 7 {
			continue
		}

		d := models.ExploreDestination{}
		d.CityID, _ = dest[0].(string)

		// Try format 1: price at [1][[null,price]], airline at [6]
		if d1 := tryParseFormat1(dest); d1 != nil {
			dests = append(dests, *d1)
			continue
		}

		// Try format 2: image-rich format with price in different positions
		if d2 := tryParseFormat2(dest); d2 != nil {
			dests = append(dests, *d2)
			continue
		}
	}

	return dests
}

// tryParseFormat1 parses the compact destination format (from entry[1+]):
// [cityID, [[null,price],"token"], null,null,null,null, [airline_info], ...]
func tryParseFormat1(dest []any) *models.ExploreDestination {
	if len(dest) < 7 {
		return nil
	}

	d := models.ExploreDestination{}
	d.CityID, _ = dest[0].(string)

	// Price at [1][0][1]
	if priceArr, ok := dest[1].([]any); ok && len(priceArr) > 0 {
		if priceData, ok := priceArr[0].([]any); ok && len(priceData) > 1 {
			if p, ok := priceData[1].(float64); ok {
				d.Price = p
			}
		}
	}

	// Airline/airport info at [6]
	if info, ok := dest[6].([]any); ok && len(info) > 5 {
		d.AirlineCode, _ = info[0].(string)
		d.AirlineName, _ = info[1].(string)
		if stops, ok := info[2].(float64); ok {
			d.Stops = int(stops)
		}
		d.AirportCode, _ = info[5].(string)
	}

	if d.AirportCode != "" && d.Price > 0 {
		// Enrich city name from airport lookup if not already set.
		if d.CityName == "" {
			d.CityName = models.LookupAirportName(d.AirportCode)
			// Only use if it's different from the code itself.
			if d.CityName == d.AirportCode {
				d.CityName = ""
			}
		}
		return &d
	}
	return nil
}

// tryParseFormat2 parses the image-rich destination format (from entry[0]):
// [cityID, [lat,lng], "CityName", "imgURL", "Country", ...]
// Airport code is typically at index 14.
func tryParseFormat2(dest []any) *models.ExploreDestination {
	if len(dest) < 15 {
		return nil
	}

	d := models.ExploreDestination{}
	d.CityID, _ = dest[0].(string)

	// City name at [2]
	cityName, _ := dest[2].(string)
	if cityName != "" {
		d.CityID = d.CityID + " (" + cityName + ")"
	}

	// Airport code at [14]
	d.AirportCode, _ = dest[14].(string)

	// No price in this format -- it's just the destination listing
	// Return nil to fall through to format1 in other entries
	if d.AirportCode == "" {
		return nil
	}

	return nil // Price required -- this format doesn't have it
}
