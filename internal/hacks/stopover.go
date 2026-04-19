package hacks

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// detectStopover identifies when a route passes through an airline hub that
// offers a free multi-day stopover program, so the traveller can add a free
// city visit with no extra airfare.
func detectStopover(ctx context.Context, in DetectorInput) []Hack {
	if !in.valid() || in.Date == "" {
		return nil
	}

	result, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{})
	if err != nil || !result.Success {
		return nil
	}

	seen := map[string]bool{}
	var hacks []Hack

	for _, f := range result.Flights {
		for _, leg := range f.Legs {
			// Check each stopover airport.
			hub := leg.ArrivalAirport.Code
			prog, ok := matchStopoverProgram(hub, leg.AirlineCode)
			if !ok || seen[hub] {
				continue
			}
			// Skip if origin or destination IS the hub (not a stopover scenario).
			if hub == in.Origin || hub == in.Destination {
				continue
			}
			seen[hub] = true

			hacks = append(hacks, buildStopoverHack(in, prog, f, hub))
		}
	}

	return hacks
}

// matchStopoverProgram returns the stopover program if the airline/hub pair
// matches any registered program. Checks by hub airport code.
func matchStopoverProgram(hub, airlineCode string) (StopoverProgram, bool) {
	// Check by airline IATA code first.
	if prog, ok := stopoverPrograms[airlineCode]; ok && prog.Hub == hub {
		return prog, true
	}
	// Also match by hub airport directly (some airlines operate under multiple codes).
	for _, prog := range stopoverPrograms {
		if prog.Hub == hub {
			return prog, true
		}
	}
	return StopoverProgram{}, false
}

func buildStopoverHack(in DetectorInput, prog StopoverProgram, f models.FlightResult, hub string) Hack {
	currency := in.currency()
	if f.Currency != "" {
		currency = f.Currency
	}

	hubName := hubCityName(hub)

	return Hack{
		Type:     "stopover",
		Title:    fmt.Sprintf("Free %s stopover (%s)", hubName, prog.Airline),
		Currency: currency,
		Savings:  0, // Stopover is a value-add, not a price saving vs naive booking
		Description: fmt.Sprintf(
			"%s offers a free stopover in %s (up to %d nights) when transiting through %s. "+
				"Add %s to your itinerary at no extra airfare cost.",
			prog.Airline, hubName, prog.MaxNights, hub, hubName,
		),
		Risks: []string{
			fmt.Sprintf("Restrictions: %s", prog.Restrictions),
			"Must book directly with " + prog.Airline + " to activate the stopover program",
			"Stopover programs are subject to availability and may change without notice",
			"Visa requirements for " + hubName + " may apply depending on your nationality",
		},
		Steps: []string{
			fmt.Sprintf("Book your %s→%s ticket with %s via %s", in.Origin, in.Destination, prog.Airline, hub),
			fmt.Sprintf("Request a stopover in %s at time of booking (up to %d nights)", hubName, prog.MaxNights),
			"Check visa requirements for " + hubName,
			"Book accommodation in " + hubName + " (not included)",
		},
		Citations: []string{prog.URL},
	}
}

// hubCityName returns a human-readable city name for a hub airport code.
func hubCityName(code string) string {
	names := map[string]string{
		"HEL": "Helsinki",
		"KEF": "Reykjavik",
		"LIS": "Lisbon",
		"IST": "Istanbul",
		"DOH": "Doha",
		"DXB": "Dubai",
		"SIN": "Singapore",
		"AUH": "Abu Dhabi",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}
