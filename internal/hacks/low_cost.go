package hacks

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// lowCostCarriers is the set of IATA airline codes for low-cost carriers.
var lowCostCarriers = map[string]string{
	"FR": "Ryanair",
	"W6": "Wizz Air",
	"U2": "easyJet",
	"PC": "Pegasus",
	"DY": "Norwegian",
	"LS": "Jet2",
	"VY": "Vueling",
	"F9": "Frontier",
	"B6": "JetBlue",
	"G3": "Gol",
	"4U": "Eurowings",
	"DE": "Condor",
	"HV": "Transavia",
	"TO": "Transavia France",
	"V7": "Volotea",
}

// lowCostMinSavingPct is the minimum price difference (%) to flag a LCC option.
const lowCostMinSavingPct = 20.0

// detectLowCostCarrier looks for low-cost carrier options on the same route
// that are at least 20% cheaper than the cheapest result returned by the
// baseline search (which may include legacy carriers).
func detectLowCostCarrier(ctx context.Context, in DetectorInput) []Hack {
	if !in.valid() || in.Date == "" {
		return nil
	}

	// Load preferences to check loyalty airline conflicts.
	prefs, _ := preferences.Load()

	// Baseline: search all carriers.
	baseResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{
		ReturnDate: in.ReturnDate,
	})
	if err != nil || !baseResult.Success || len(baseResult.Flights) == 0 {
		return nil
	}
	basePrice := minFlightPrice(baseResult)
	if basePrice <= 0 {
		return nil
	}
	currency := flightCurrency(baseResult, in.currency())

	// Find the cheapest LCC result in the baseline response.
	bestLCCPrice := 0.0
	bestLCCName := ""
	for _, f := range baseResult.Flights {
		if !isLowCostFlight(f) {
			continue
		}
		if f.Price > 0 && (bestLCCPrice == 0 || f.Price < bestLCCPrice) {
			bestLCCPrice = f.Price
			bestLCCName = lccName(f)
		}
	}

	if bestLCCPrice <= 0 {
		return nil
	}

	savings := basePrice - bestLCCPrice
	savingPct := savings / basePrice * 100
	if savingPct < lowCostMinSavingPct {
		return nil
	}

	// Check if user has loyalty programmes with legacy carriers on this route.
	loyaltyWarning := loyaltyConflictNote(baseResult, prefs)

	risks := []string{
		"Checked baggage costs extra — carry-on only is cheapest",
		"Seat selection fees may apply (some carriers charge EUR 5-20 per seat)",
		"Hand luggage size limits are strictly enforced (measure your bag)",
		"No rebooking or refund without a paid flex fare",
		"Smaller / secondary airports — check transfer time to city centre",
	}
	if loyaltyWarning != "" {
		risks = append(risks, loyaltyWarning)
	}

	return []Hack{{
		Type:     "low_cost_carrier",
		Title:    fmt.Sprintf("%s from same airport — %s %.0f cheaper", bestLCCName, currency, savings),
		Currency: currency,
		Savings:  roundSavings(savings),
		Description: fmt.Sprintf(
			"%s offers %s→%s at %s %.0f vs %.0f on legacy carriers (%.0f%% cheaper). "+
				"Carry-on only to avoid baggage fees.",
			bestLCCName, in.Origin, in.Destination,
			currency, bestLCCPrice, basePrice, savingPct,
		),
		Risks: risks,
		Steps: []string{
			fmt.Sprintf("Search %s directly on %s website or via an aggregator", bestLCCName, bestLCCName),
			"Select carry-on-only fare (no checked bag)",
			"Measure bag before travel — LCCs enforce size limits",
			"Arrive early: LCC check-in cuts off 30-45 min before departure",
		},
		Citations: []string{
			googleFlightsURL(in.Destination, in.Origin, in.Date),
		},
	}}
}

// isLowCostFlight returns true when any leg of the flight is operated by a LCC.
func isLowCostFlight(f models.FlightResult) bool {
	for _, leg := range f.Legs {
		if _, ok := lowCostCarriers[leg.AirlineCode]; ok {
			return true
		}
	}
	return false
}

// lccName returns the first LCC airline name found in the flight's legs.
func lccName(f models.FlightResult) string {
	for _, leg := range f.Legs {
		if name, ok := lowCostCarriers[leg.AirlineCode]; ok {
			return name
		}
	}
	return "LCC"
}

// loyaltyConflictNote returns a warning string if the cheapest legacy-carrier
// result is with an airline the user has a loyalty programme with.
func loyaltyConflictNote(result *models.FlightSearchResult, prefs *preferences.Preferences) string {
	if prefs == nil || len(prefs.LoyaltyAirlines) == 0 {
		return ""
	}
	loyaltySet := map[string]bool{}
	for _, code := range prefs.LoyaltyAirlines {
		loyaltySet[code] = true
	}
	for _, f := range result.Flights {
		for _, leg := range f.Legs {
			if loyaltySet[leg.AirlineCode] {
				return fmt.Sprintf("You have a loyalty programme with %s (%s) — switching to LCC means losing miles on this route", leg.Airline, leg.AirlineCode)
			}
		}
	}
	return ""
}
