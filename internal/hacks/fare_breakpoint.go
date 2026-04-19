package hacks

import (
	"context"
	"fmt"
	"strings"
)

// breakpointHub describes a connection hub that sits at an IATA fare zone
// boundary. Routing via such hubs can split a ticket into cheaper zone
// components.
type breakpointHub struct {
	IATA          string  // hub airport IATA code
	City          string  // human-readable city name
	Airline       string  // primary carrier IATA code
	Zone          string  // explanation of the fare zone boundary
	MinDistanceKm float64 // only consider routes longer than this
}

// fareBreakpointHubs is the static database of known fare breakpoint hubs.
var fareBreakpointHubs = []breakpointHub{
	// TC2/TC3 boundary (Europe↔Asia)
	{IATA: "IST", City: "Istanbul", Airline: "TK", Zone: "Europe-Asia boundary", MinDistanceKm: 3000},
	{IATA: "DOH", City: "Doha", Airline: "QR", Zone: "Europe-Asia via Gulf", MinDistanceKm: 4000},
	{IATA: "DXB", City: "Dubai", Airline: "EK", Zone: "Europe-Asia via Gulf", MinDistanceKm: 4000},
	{IATA: "AUH", City: "Abu Dhabi", Airline: "EY", Zone: "Europe-Asia via Gulf", MinDistanceKm: 4000},
	// TC2/TC1 boundary (Europe↔Americas)
	{IATA: "MAD", City: "Madrid", Airline: "IB", Zone: "Europe-LatAm via Iberia", MinDistanceKm: 5000},
	{IATA: "LIS", City: "Lisbon", Airline: "TP", Zone: "Europe-LatAm/Africa", MinDistanceKm: 3000},
	// TC1 (Americas connections)
	{IATA: "BOG", City: "Bogotá", Airline: "AV", Zone: "LatAm hub", MinDistanceKm: 7000},
	// Africa/Middle East
	{IATA: "ADD", City: "Addis Ababa", Airline: "ET", Zone: "Europe-Africa-Asia", MinDistanceKm: 4000},
	{IATA: "CMN", City: "Casablanca", Airline: "AT", Zone: "Europe-Africa", MinDistanceKm: 2000},
}

// airlineNames maps carrier IATA codes to display names for fare breakpoint suggestions.
var airlineNames = map[string]string{
	"TK": "Turkish Airlines",
	"QR": "Qatar Airways",
	"EK": "Emirates",
	"EY": "Etihad Airways",
	"IB": "Iberia",
	"TP": "TAP Portugal",
	"AV": "Avianca",
	"ET": "Ethiopian Airlines",
	"AT": "Royal Air Maroc",
}

// detectFareBreakpoint checks whether routing via a fare zone boundary hub
// could yield cheaper tickets by splitting the journey into separate zone
// components. This is purely advisory — no API calls are made.
func detectFareBreakpoint(_ context.Context, in DetectorInput) []Hack {
	if !in.valid() {
		return nil
	}

	origin := strings.ToUpper(in.Origin)
	destination := strings.ToUpper(in.Destination)

	directDist := airportDistanceKm(origin, destination)
	if directDist == 0 {
		// Unknown airports — cannot compute distances.
		return nil
	}

	var hacks []Hack

	for _, hub := range fareBreakpointHubs {
		// Skip if the route is too short for this hub to matter.
		if directDist < hub.MinDistanceKm {
			continue
		}

		// Skip if origin or destination IS the hub.
		if origin == hub.IATA || destination == hub.IATA {
			continue
		}

		// Check geographic sanity: via-hub distance should be < 1.5× direct.
		legA := airportDistanceKm(origin, hub.IATA)
		legB := airportDistanceKm(hub.IATA, destination)
		if legA == 0 || legB == 0 {
			continue // hub coordinates unknown
		}

		viaDistance := legA + legB
		if viaDistance > directDist*1.5 {
			continue // hub is too far out of the way
		}

		airlineName := airlineNames[hub.Airline]
		if airlineName == "" {
			airlineName = hub.Airline
		}

		hacks = append(hacks, Hack{
			Type:  "fare_breakpoint",
			Title: fmt.Sprintf("Consider routing via %s on %s", hub.City, airlineName),
			Description: fmt.Sprintf(
				"%s sits at the %s fare zone boundary. Routing %s→%s→%s on %s may split the ticket into cheaper zone components. "+
					"Direct distance: %.0f km, via %s: %.0f km (%.0f%% of direct).",
				hub.City, hub.Zone,
				origin, hub.IATA, destination, airlineName,
				directDist, hub.City, viaDistance,
				(viaDistance/directDist)*100,
			),
			Savings:  0, // advisory — no concrete savings estimate
			Currency: in.currency(),
			Steps: []string{
				fmt.Sprintf("Search %s→%s→%s on %s (%s)", origin, hub.IATA, destination, airlineName, hub.Airline),
				fmt.Sprintf("Compare with direct %s→%s pricing", origin, destination),
				"Look for multi-city or stopover fares through the airline's website",
				fmt.Sprintf("Check if %s offers a free stopover program in %s", airlineName, hub.City),
			},
			Risks: []string{
				"Connection adds travel time and layover risk",
				"Fare savings depend on specific dates and booking classes",
				"Two-segment itinerary increases missed-connection risk",
			},
			Citations: []string{
				fmt.Sprintf("https://www.google.com/travel/flights?q=%s%%20to%%20%s%%20via%%20%s", origin, destination, hub.IATA),
			},
		})
	}

	return hacks
}
