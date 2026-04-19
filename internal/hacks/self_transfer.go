package hacks

import (
	"context"
	"fmt"
	"strings"
)

// selfTransferHub describes an airport suitable for self-connecting
// between two separate LCC tickets.
type selfTransferHub struct {
	IATA             string
	City             string
	MinConnectionMin int      // minimum safe layover in minutes
	Terminal         string   // terminal info
	Airlines         []string // LCC IATA codes that serve this airport
}

// selfTransferHubs is a curated list of airports where self-connecting
// between two separate LCC tickets is practical.
var selfTransferHubs = []selfTransferHub{
	{IATA: "BGY", City: "Bergamo/Milan", MinConnectionMin: 180, Terminal: "Single terminal", Airlines: []string{"FR", "W6"}},
	{IATA: "STN", City: "London Stansted", MinConnectionMin: 180, Terminal: "Single terminal", Airlines: []string{"FR", "W6"}},
	{IATA: "BVA", City: "Paris Beauvais", MinConnectionMin: 180, Terminal: "Single terminal", Airlines: []string{"FR", "W6"}},
	{IATA: "CRL", City: "Brussels Charleroi", MinConnectionMin: 180, Terminal: "Single terminal", Airlines: []string{"FR", "W6"}},
	{IATA: "CIA", City: "Rome Ciampino", MinConnectionMin: 150, Terminal: "Single terminal", Airlines: []string{"FR", "W6"}},
	{IATA: "BCN", City: "Barcelona", MinConnectionMin: 120, Terminal: "Terminals 1 and 2 (LCCs use T2)", Airlines: []string{"FR", "VY", "W6"}},
	{IATA: "BUD", City: "Budapest", MinConnectionMin: 150, Terminal: "Terminal 2", Airlines: []string{"FR", "W6"}},
	{IATA: "DUB", City: "Dublin", MinConnectionMin: 150, Terminal: "Terminals 1 and 2", Airlines: []string{"FR"}},
	{IATA: "LTN", City: "London Luton", MinConnectionMin: 180, Terminal: "Single terminal", Airlines: []string{"W6", "U2"}},
	{IATA: "AMS", City: "Amsterdam", MinConnectionMin: 120, Terminal: "Single terminal (Schiphol)", Airlines: []string{"U2", "VY"}},
}

// selfTransferAirlineNames maps IATA codes to LCC names for display.
var selfTransferAirlineNames = map[string]string{
	"FR": "Ryanair",
	"W6": "Wizz Air",
	"U2": "easyJet",
	"VY": "Vueling",
}

// lccDirectRoutes lists airport pairs with known dense LCC service.
// If the origin-destination pair is already directly served by LCCs,
// a self-transfer hack is unlikely to save money.
var lccDirectRoutes = map[string]map[string]bool{
	"STN": {"BCN": true, "BGY": true, "CIA": true, "DUB": true, "BUD": true, "CRL": true},
	"BGY": {"STN": true, "CRL": true, "BVA": true, "CIA": true, "BCN": true, "BUD": true},
	"BVA": {"BGY": true, "CIA": true, "BCN": true},
	"CRL": {"BGY": true, "STN": true, "BCN": true, "BUD": true, "CIA": true},
	"CIA": {"STN": true, "BGY": true, "BVA": true, "CRL": true, "BCN": true, "BUD": true},
	"BCN": {"STN": true, "BGY": true, "BVA": true, "CRL": true, "CIA": true, "BUD": true, "DUB": true, "LTN": true},
	"BUD": {"STN": true, "BGY": true, "CRL": true, "CIA": true, "BCN": true, "LTN": true, "DUB": true},
	"DUB": {"STN": true, "BCN": true, "BUD": true},
	"LTN": {"BCN": true, "BUD": true},
}

// detectSelfTransfer suggests checking self-transfer (virtual interlining)
// via known LCC hub airports. This is purely advisory — zero API calls.
// It fires when the direct route is not densely served by LCCs, meaning
// the user might save by booking two separate one-way LCC tickets through
// a hub airport.
func detectSelfTransfer(_ context.Context, in DetectorInput) []Hack {
	if !in.valid() {
		return nil
	}

	origin := strings.ToUpper(in.Origin)
	dest := strings.ToUpper(in.Destination)

	// If origin == destination, nothing to suggest.
	if origin == dest {
		return nil
	}

	// If the route is already densely served by LCCs, skip — self-transfer
	// is unlikely to beat a direct LCC flight.
	if isDirectLCCRoute(origin, dest) {
		return nil
	}

	var hacks []Hack

	for _, hub := range selfTransferHubs {
		// Skip if the hub is the origin or destination (no self-transfer needed).
		if hub.IATA == origin || hub.IATA == dest {
			continue
		}

		airlineNames := hubAirlineNames(hub.Airlines)

		hacks = append(hacks, Hack{
			Type:     "self_transfer",
			Title:    fmt.Sprintf("Self-transfer via %s (%s)", hub.City, hub.IATA),
			Currency: in.currency(),
			Savings:  0, // advisory — no price lookup
			Description: fmt.Sprintf(
				"Book two separate one-way tickets via %s (%s): %s→%s on one LCC, "+
					"then %s→%s on another. Airlines serving %s: %s. "+
					"Allow at least %d minutes between flights for immigration, "+
					"baggage reclaim, and re-check-in. %s.",
				hub.City, hub.IATA, origin, hub.IATA,
				hub.IATA, dest, hub.IATA, airlineNames,
				hub.MinConnectionMin, hub.Terminal,
			),
			Risks: []string{
				"Two separate tickets — if the first flight is delayed, the second is not protected",
				fmt.Sprintf("Allow minimum %d minutes connection at %s", hub.MinConnectionMin, hub.IATA),
				"Checked bags must be reclaimed and re-checked (carry-on only is safest)",
				"No passenger rights protection across separate tickets if you miss the connection",
				"Travel insurance may not cover missed connections on separate bookings",
			},
			Steps: []string{
				fmt.Sprintf("Search %s→%s on %s", origin, hub.IATA, airlineNames),
				fmt.Sprintf("Search %s→%s on %s", hub.IATA, dest, airlineNames),
				fmt.Sprintf("Ensure at least %d minutes between arrival and departure at %s", hub.MinConnectionMin, hub.IATA),
				"Book carry-on only to avoid baggage reclaim delays",
				"Compare total of both tickets against the direct route price",
			},
		})
	}

	return hacks
}

// isDirectLCCRoute returns true if the origin-destination pair is known
// to have dense LCC service, making self-transfer unlikely to save money.
func isDirectLCCRoute(origin, dest string) bool {
	if dests, ok := lccDirectRoutes[origin]; ok {
		if dests[dest] {
			return true
		}
	}
	// Check reverse direction too.
	if dests, ok := lccDirectRoutes[dest]; ok {
		if dests[origin] {
			return true
		}
	}
	return false
}

// hubAirlineNames returns a comma-separated string of airline names for
// the given IATA codes.
func hubAirlineNames(codes []string) string {
	names := make([]string, 0, len(codes))
	for _, code := range codes {
		if name, ok := selfTransferAirlineNames[code]; ok {
			names = append(names, name)
		} else {
			names = append(names, code)
		}
	}
	return strings.Join(names, ", ")
}
