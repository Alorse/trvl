package hacks

import (
	"context"
	"fmt"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
)

// hubStopoverAllowance lists airlines known to allow free multi-day stopovers
// at their hubs. Key = hub IATA code, value = max free stopover nights.
// This is a subset focused on European routing hubs.
var hubStopoverAllowance = map[string]struct {
	Airline  string
	MaxNight int
}{
	"AMS": {Airline: "KLM", MaxNight: 2},
	"HEL": {Airline: "Finnair", MaxNight: 5},
	"FRA": {Airline: "Lufthansa", MaxNight: 1},
	"MUC": {Airline: "Lufthansa", MaxNight: 1},
	"CDG": {Airline: "Air France", MaxNight: 1},
	"ZRH": {Airline: "Swiss", MaxNight: 1},
	"VIE": {Airline: "Austrian", MaxNight: 1},
	"IST": {Airline: "Turkish Airlines", MaxNight: 2},
	"DOH": {Airline: "Qatar Airways", MaxNight: 4},
	"DXB": {Airline: "Emirates", MaxNight: 4},
}

// multistopRoutings lists known pairs (origin, final destination) where
// routing through a hub gives a significant layover opportunity.
// Key: destination; values: possible hubs on the way from common origins.
var multistopHubs = map[string][]string{
	"PRG": {"AMS", "FRA", "MUC", "CDG"},
	"VIE": {"AMS", "FRA", "MUC"},
	"BUD": {"AMS", "FRA", "MUC", "VIE"},
	"WAW": {"AMS", "FRA", "CDG"},
	"KRK": {"AMS", "FRA"},
	"ARN": {"AMS", "FRA", "CDG"},
	"CPH": {"AMS", "FRA"},
	"OSL": {"AMS", "FRA", "CDG"},
	"ATH": {"AMS", "FRA", "CDG", "IST"},
	"IST": {"AMS", "FRA", "CDG"},
	"DBV": {"AMS", "FRA", "MUC"},
	"SPU": {"AMS", "FRA", "MUC"},
	"BCN": {"AMS", "CDG"},
	"MAD": {"AMS", "CDG"},
	"FCO": {"AMS", "FRA", "CDG", "MUC"},
	"LIS": {"AMS", "CDG"},
	"HEL": {"AMS", "FRA"},
}

// minLayoverMinutesForStopover is the minimum layover duration (in minutes)
// for the stopover to be worth flagging as a city visit opportunity.
const minLayoverMinutesForStopover = 240 // 4 hours

// detectMultiStop identifies round-trips that route through an airline hub
// with a long enough layover to make a meaningful city visit.
func detectMultiStop(ctx context.Context, in DetectorInput) []Hack {
	if !in.valid() || in.Date == "" {
		return nil
	}

	hubs, ok := multistopHubs[in.Destination]
	if !ok {
		return nil
	}

	// Baseline: direct/cheapest round-trip or one-way.
	opts := flights.SearchOptions{}
	if in.ReturnDate != "" {
		opts.ReturnDate = in.ReturnDate
	}
	baseResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, opts)
	if err != nil || !baseResult.Success || len(baseResult.Flights) == 0 {
		return nil
	}
	currency := flightCurrency(baseResult, in.currency())

	var hacks []Hack
	seenHubs := map[string]bool{}

	for _, f := range baseResult.Flights {
		for i, leg := range f.Legs {
			hub := leg.ArrivalAirport.Code
			if seenHubs[hub] {
				continue
			}
			// Must be an intermediate stop (not the final destination).
			if i == len(f.Legs)-1 {
				continue
			}
			if hub == in.Origin || hub == in.Destination {
				continue
			}
			// Must be a known multi-stop hub.
			if !sliceContains(hubs, hub) {
				continue
			}

			// Check layover duration.
			nextLeg := f.Legs[i+1]
			layover := layoverMinutes(leg.ArrivalTime, nextLeg.DepartureTime)
			if layover < minLayoverMinutesForStopover {
				continue
			}

			seenHubs[hub] = true

			info, hasInfo := hubStopoverAllowance[hub]
			airlineName := leg.Airline
			if hasInfo {
				airlineName = info.Airline
			}
			maxNights := 0
			if hasInfo {
				maxNights = info.MaxNight
			}

			var layoverDesc string
			if layover >= 1440 {
				layoverDesc = fmt.Sprintf("~%d-day layover", layover/1440)
			} else {
				layoverDesc = fmt.Sprintf("~%dh layover", layover/60)
			}

			steps := []string{
				fmt.Sprintf("Book %s→%s routing via %s with %s", in.Origin, in.Destination, hub, airlineName),
				fmt.Sprintf("On outbound: exit at %s (%s) — explore the city", hub, layoverDesc),
				"Ensure you have any required transit visa for " + hub,
			}
			if maxNights > 0 {
				steps = append(steps, fmt.Sprintf("Ask %s for a free stopover extension (up to %d nights)", airlineName, maxNights))
			}

			hacks = append(hacks, Hack{
				Type:     "multi_stop",
				Title:    fmt.Sprintf("Two-city trip: visit %s on the way to %s", hubCityName(hub), in.Destination),
				Currency: currency,
				Savings:  0, // This is a value-add, not a price saving
				Description: fmt.Sprintf(
					"Your %s→%s routing via %s has a %s at %s (%s). "+
						"Extend it into a free city stop — two destinations, one ticket price.",
					in.Origin, in.Destination, hub, layoverDesc, hub, airlineName,
				),
				Risks: []string{
					"Stopover extension must be requested at booking — not all fare classes allow it",
					"Visa required for some hub countries depending on your nationality",
					"Airline schedule change may shorten your layover without notice",
				},
				Steps: steps,
				Citations: []string{
					googleFlightsURL(in.Destination, in.Origin, in.Date),
				},
			})
		}
	}

	return hacks
}

// sliceContains returns true if s contains v.
func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// layoverMinutes computes the layover in minutes between an arrival ISO time
// and the next departure ISO time. Returns 0 on parse errors.
func layoverMinutes(arrivalISO, departureISO string) int {
	arr, err1 := parseDatetime(arrivalISO)
	dep, err2 := parseDatetime(departureISO)
	if err1 != nil || err2 != nil {
		return 0
	}
	diff := dep.Sub(arr).Minutes()
	if diff < 0 {
		return 0
	}
	return int(diff)
}

// parseDatetime parses an ISO 8601 datetime string. Tries multiple formats.
func parseDatetime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse datetime: %s", s)
}
