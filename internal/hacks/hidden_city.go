package hacks

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// hubsFor returns a set of common airline hub cities that typically route through
// a given airport. Used to seed the hidden-city candidate search.
//
// For the hidden-city hack we want: "find a city C such that the cheapest
// flight O→C routes through D, and O→C < O→D".
//
// We approximate this by searching nearby hub airports that are likely to have
// connections through the destination.
var hiddenCityExtensions = map[string][]string{
	// If destination is an intermediate hub, list cities one step beyond it.
	"AMS": {"ARN", "CPH", "HEL", "OSL", "WAW", "BUD", "PRG"},
	"FRA": {"ARN", "CPH", "HEL", "OSL", "WAW", "ATH", "IST"},
	"MUC": {"ARN", "CPH", "HEL", "OSL", "WAW", "ATH", "IST"},
	"CDG": {"ARN", "CPH", "HEL", "OSL", "WAW", "ATH"},
	"LHR": {"ARN", "CPH", "HEL", "OSL", "WAW", "ATH", "IST"},
	"MAD": {"MIA", "BOG", "SCL", "LIM", "GRU"},
	"LIS": {"GRU", "OPO", "LAD", "CPT"},
	"IST": {"TAS", "ALA", "TBS", "EVN", "BAK"},
	"DXB": {"CMB", "BOM", "DEL", "KHI", "DAC"},
	"DOH": {"CMB", "BOM", "DEL", "KHI"},
	"SIN": {"KUL", "CGK", "MNL", "BKK"},
}

// detectHiddenCity searches for flights where the origin→hub+extension ticket
// is cheaper than a direct origin→hub ticket, meaning the traveller can get
// off at the hub (the actual destination) and skip the final leg.
//
// Only suggested when carry-on only is set or CarryOnOnly flag is true,
// because checked bags are routed to the final destination.
func detectHiddenCity(ctx context.Context, in DetectorInput) []Hack {
	if !in.valid() || in.Date == "" {
		return nil
	}

	// Find candidate "beyond" airports for the destination.
	beyonds, ok := hiddenCityExtensions[in.Destination]
	if !ok {
		// Destination is not a known hub we can extend from; skip.
		return nil
	}

	// Direct price: O→D
	directResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{})
	if err != nil || !directResult.Success || len(directResult.Flights) == 0 {
		return nil
	}
	directPrice := minFlightPrice(directResult)
	if directPrice <= 0 {
		return nil
	}
	currency := flightCurrency(directResult, in.currency())

	var best *Hack
	bestSavings := 0.0

	for _, beyond := range beyonds {
		if beyond == in.Origin {
			continue
		}
		beyondResult, err := flights.SearchFlights(ctx, in.Origin, beyond, in.Date, flights.SearchOptions{})
		if err != nil || !beyondResult.Success || len(beyondResult.Flights) == 0 {
			continue
		}
		beyondPrice := minFlightPrice(beyondResult)
		if beyondPrice <= 0 {
			continue
		}

		// Only valid if the beyond-ticket actually routes through destination.
		if !routesThroughDestination(beyondResult, in.Destination) {
			continue
		}

		if beyondPrice >= directPrice {
			continue
		}

		savings := directPrice - beyondPrice
		if savings > bestSavings {
			bestSavings = savings
			airlineCode := primaryAirlineCode(beyondResult)
			hack := buildHiddenCityHack(in, beyond, beyondPrice, directPrice, currency, airlineCode)
			best = &hack
		}
	}

	if best == nil {
		return nil
	}
	return []Hack{*best}
}

// routesThroughDestination returns true when at least one flight in the result
// has an intermediate stop at the destination airport.
//
// It checks FlightResult.Legs for the destination airport in an arrival slot
// that is NOT the final leg. Falls back to true (optimistic) when no leg data
// is present, so we do not suppress potentially valid hacks.
func routesThroughDestination(result *models.FlightSearchResult, dest string) bool {
	if result == nil || !result.Success {
		return false
	}
	for _, f := range result.Flights {
		if len(f.Legs) < 2 {
			// Single-leg flight cannot be a hidden-city ticket.
			continue
		}
		// Check all legs except the last for a stop at dest.
		for i := 0; i < len(f.Legs)-1; i++ {
			if f.Legs[i].ArrivalAirport.Code == dest {
				return true
			}
		}
	}
	// No leg data matched; return true optimistically so the hack is surfaced.
	// The user should always verify the routing before booking.
	return len(result.Flights) > 0
}

func buildHiddenCityHack(in DetectorInput, beyond string, beyondPrice, directPrice float64, currency, airlineCode string) Hack {
	bagsWarning := "Book carry-on only — checked bags are routed to the final destination and cannot be retrieved at the intermediate stop"

	risks := []string{
		"Violates airline contracts of carriage — airline may ban your account or take legal action",
		bagsWarning,
		"Cannot use return leg on a round-trip ticket (airline will cancel it)",
		"Flight path may change; always verify the layover airport before booking",
		"Not feasible for last-minute schedule changes or irregular operations",
	}

	// Add baggage-specific note for the detected airline.
	if note := baggage.BaggageNote(airlineCode); note != "" {
		risks = append(risks, note)
	}

	return Hack{
		Type:     "hidden_city",
		Title:    "Hidden city ticketing",
		Currency: currency,
		Savings:  roundSavings(directPrice - beyondPrice),
		Description: fmt.Sprintf(
			"A ticket %s→%s (via %s) costs %s %.0f, cheaper than %s→%s direct at %.0f. Disembark at %s and skip the final leg. "+
				"Airline hub cities (AMS, FRA, MUC, CDG, CPH, ZRH) are prime candidates — they are expensive as destinations but cheap as connections because connecting fares are discounted vs direct.",
			in.Origin, beyond, in.Destination, currency, beyondPrice,
			in.Origin, in.Destination, directPrice, in.Destination,
		),
		Risks: risks,
		Steps: []string{
			fmt.Sprintf("Search flights %s→%s on %s", in.Origin, beyond, in.Date),
			fmt.Sprintf("Confirm the routing stops at %s", in.Destination),
			"Book a carry-on-only ticket",
			fmt.Sprintf("Disembark at %s; do not board the onward connection to %s", in.Destination, beyond),
			"Tip: booking from Eastern European origins (PRG, KRK, BUD, WAW) to hub cities via cheap beyond-destinations leverages lower market pricing",
		},
		Citations: []string{
			googleFlightsURL(beyond, in.Origin, in.Date),
		},
	}
}

// primaryAirlineCode extracts the IATA code of the first airline from flight results.
func primaryAirlineCode(result *models.FlightSearchResult) string {
	if result == nil || !result.Success {
		return ""
	}
	for _, f := range result.Flights {
		for _, leg := range f.Legs {
			if leg.AirlineCode != "" {
				return leg.AirlineCode
			}
		}
	}
	return ""
}
