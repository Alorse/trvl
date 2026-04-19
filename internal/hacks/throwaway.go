package hacks

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/flights"
)

// detectThrowaway finds cases where a round-trip ticket is cheaper than a
// one-way, allowing the traveller to book round-trip and skip the return leg.
//
// Rule: if round-trip price < one-way price × 1.6, flag it.
func detectThrowaway(ctx context.Context, in DetectorInput) []Hack {
	if !in.valid() || in.Date == "" {
		return nil
	}

	// We need a return date to search round-trip. Pick +7 days if none supplied.
	returnDate := in.ReturnDate
	if returnDate == "" {
		returnDate = addDays(in.Date, 7)
		if returnDate == "" {
			return nil
		}
	}

	// Search one-way outbound.
	owResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{})
	if err != nil || !owResult.Success || len(owResult.Flights) == 0 {
		return nil
	}
	owPrice := minFlightPrice(owResult)
	if owPrice <= 0 {
		return nil
	}

	// Search round-trip (same outbound date, with return).
	rtResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{
		ReturnDate: returnDate,
	})
	if err != nil || !rtResult.Success || len(rtResult.Flights) == 0 {
		return nil
	}
	rtPrice := minFlightPrice(rtResult)
	if rtPrice <= 0 {
		return nil
	}

	// Only flag when round-trip is materially cheaper (threshold 1.6×).
	if rtPrice >= owPrice*1.6 {
		return nil
	}

	savings := owPrice - rtPrice
	currency := flightCurrency(owResult, in.currency())
	airlineCode := primaryAirlineCode(owResult)

	risks := []string{
		"Violates most airline contracts of carriage — airline may cancel return leg without refund",
		"Frequent-flyer account may be penalised or closed",
		"If you miss the outbound leg, the return is void automatically",
		"Do not check bags — luggage is tagged to the final destination",
	}
	if note := baggage.BaggageNote(airlineCode); note != "" {
		risks = append(risks, note)
	}

	return []Hack{{
		Type:     "throwaway",
		Title:    "Throwaway ticketing",
		Currency: currency,
		Savings:  roundSavings(savings),
		Description: fmt.Sprintf(
			"Round-trip %s→%s costs %s %.0f (vs %.0f one-way). Book round-trip and skip the return leg.",
			in.Origin, in.Destination, currency, rtPrice, owPrice,
		),
		Risks: risks,
		Steps: []string{
			fmt.Sprintf("Search round-trip %s→%s (depart %s, return %s)", in.Origin, in.Destination, in.Date, returnDate),
			"Book the cheapest round-trip option",
			"Only board the outbound leg — discard or ignore the return",
		},
		Citations: []string{
			googleFlightsURL(in.Destination, in.Origin, in.Date),
		},
	}}
}
