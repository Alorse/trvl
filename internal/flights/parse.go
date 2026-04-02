// Package flights implements Google Flights search via the internal batchexecute API.
package flights

import (
	"fmt"
	"math"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// parseFlights extracts FlightResult structs from the raw flight entries
// returned by batchexec.ExtractFlightData.
//
// Each flight entry is a deeply nested JSON array with positional semantics:
//
//	entry[0][0]  — legs array (each leg has airline, airports, times)
//	entry[0][9]  — total duration in minutes
//	entry[1]     — price array: [null, amount, null, currency_code]
//
// Leg structure (entry[0][0][i]):
//
//	leg[1] — departure airport code
//	leg[2] — departure airport name
//	leg[3] — arrival airport code
//	leg[4] — arrival airport name
//	leg[5] — departure time array [year, month, day, hour, minute]
//	leg[6] — arrival time array [year, month, day, hour, minute]
//	leg[7] — duration in minutes
//	leg[8] — airline name
//	leg[9] — airline code (IATA 2-letter)
//	leg[10] — flight number (e.g. "NH 6521")
func parseFlights(rawFlights []any) []models.FlightResult {
	var results []models.FlightResult

	for _, raw := range rawFlights {
		entry, ok := raw.([]any)
		if !ok || len(entry) < 2 {
			continue
		}

		fr, err := parseOneFlight(entry)
		if err != nil {
			continue // skip unparseable entries
		}

		results = append(results, fr)
	}

	return results
}

// parseOneFlight parses a single flight entry into a FlightResult.
func parseOneFlight(entry []any) (models.FlightResult, error) {
	var fr models.FlightResult

	// entry[0] is the flight info array
	flightInfo, ok := entry[0].([]any)
	if !ok {
		return fr, fmt.Errorf("entry[0] not array")
	}

	// Parse legs from flightInfo[0]. Google places the legs array at the first
	// position of the flight info structure. If [0] isn't an array of legs,
	// try [2] as a fallback (some response variants may differ).
	if len(flightInfo) > 0 {
		fr.Legs = parseLegs(flightInfo[0])
		if len(fr.Legs) == 0 && len(flightInfo) > 2 {
			fr.Legs = parseLegs(flightInfo[2])
		}
	}
	fr.Stops = max(len(fr.Legs)-1, 0)

	// Parse total duration from flightInfo[9]
	if len(flightInfo) > 9 {
		fr.Duration = toInt(flightInfo[9])
	}

	// Parse price from entry[1]
	if len(entry) > 1 {
		price, currency := parsePrice(entry[1])
		fr.Price = price
		fr.Currency = currency
	}

	return fr, nil
}

// parseLegs extracts flight legs from the legs array.
func parseLegs(raw any) []models.FlightLeg {
	legsArr, ok := raw.([]any)
	if !ok {
		return nil
	}

	var legs []models.FlightLeg
	for _, rawLeg := range legsArr {
		leg, ok := rawLeg.([]any)
		if !ok {
			continue
		}

		fl := parseOneLeg(leg)
		legs = append(legs, fl)
	}

	return legs
}

// parseOneLeg parses a single leg from the nested array.
func parseOneLeg(leg []any) models.FlightLeg {
	var fl models.FlightLeg

	if len(leg) > 1 {
		fl.DepartureAirport.Code = toString(leg[1])
	}
	if len(leg) > 2 {
		fl.DepartureAirport.Name = toString(leg[2])
	}
	if len(leg) > 3 {
		fl.ArrivalAirport.Code = toString(leg[3])
	}
	if len(leg) > 4 {
		fl.ArrivalAirport.Name = toString(leg[4])
	}
	if len(leg) > 5 {
		fl.DepartureTime = formatTime(leg[5])
	}
	if len(leg) > 6 {
		fl.ArrivalTime = formatTime(leg[6])
	}
	if len(leg) > 7 {
		fl.Duration = toInt(leg[7])
	}
	if len(leg) > 8 {
		fl.Airline = toString(leg[8])
	}
	if len(leg) > 9 {
		fl.AirlineCode = toString(leg[9])
	}
	if len(leg) > 10 {
		fl.FlightNumber = toString(leg[10])
	}

	return fl
}

// parsePrice extracts price and currency from the price array.
// Google encodes price as: [null, amount_units, null, currency_code] or similar
// variations. We try multiple known positions.
func parsePrice(raw any) (float64, string) {
	arr, ok := raw.([]any)
	if !ok {
		return 0, ""
	}

	// Common format: arr is [currency_info, amount, ...]
	// or arr is [null, amount, null, currency_string]
	// Try index patterns observed from real responses:

	var amount float64
	var currency string

	// Try to find a numeric price value
	for i, v := range arr {
		if f, ok := toFloat(v); ok && f > 0 && amount == 0 {
			amount = f
			_ = i
		}
		if s := toString(v); len(s) == 3 && s >= "A" && amount > 0 && currency == "" {
			// Looks like a 3-letter currency code after we found an amount
			currency = s
		}
	}

	// If we found amount but not currency, look in nested arrays
	if amount > 0 && currency == "" {
		for _, v := range arr {
			if sub, ok := v.([]any); ok {
				for _, sv := range sub {
					if s := toString(sv); len(s) == 3 && s >= "A" {
						currency = s
						break
					}
				}
				if currency != "" {
					break
				}
			}
		}
	}

	// Default currency if still not found
	if amount > 0 && currency == "" {
		currency = "USD"
	}

	return amount, currency
}

// formatTime converts a time array [year, month, day, hour, minute] to
// an ISO 8601 string "YYYY-MM-DDTHH:MM".
func formatTime(raw any) string {
	arr, ok := raw.([]any)
	if !ok || len(arr) < 5 {
		return ""
	}

	year := toInt(arr[0])
	month := toInt(arr[1])
	day := toInt(arr[2])
	hour := toInt(arr[3])
	minute := toInt(arr[4])

	if year == 0 {
		return ""
	}

	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d", year, month, day, hour, minute)
}

// toString safely converts a JSON value to string.
func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	// json.Number or float64 representation
	if f, ok := v.(float64); ok {
		if f == math.Trunc(f) {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%g", f)
	}
	return fmt.Sprintf("%v", v)
}

// toInt safely converts a JSON value to int.
func toInt(v any) int {
	if v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// toFloat safely converts a JSON value to float64, returning ok=false if not numeric.
func toFloat(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	if f, ok := v.(float64); ok {
		return f, true
	}
	return 0, false
}
