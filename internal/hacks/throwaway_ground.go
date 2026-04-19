package hacks

import "context"

// detectThrowawayGround suggests booking ground transport beyond the
// destination when a longer route may be cheaper.  Unlike flights, bus and
// train operators do not cancel tickets if the passenger exits early.
func detectThrowawayGround(_ context.Context, in DetectorInput) []Hack {
	if in.Destination == "" || in.Origin == "" {
		return nil
	}

	return []Hack{{
		Type:  "throwaway_ground",
		Title: "Check if booking past your stop is cheaper",
		Description: "Unlike flights, bus and train operators don't cancel your ticket if you exit early. " +
			"Booking a longer route (e.g., Munich→Milan via Zurich) can be cheaper than the shorter direct ticket.",
		Steps: []string{
			"Search ground transport to a city BEYOND " + in.Destination,
			"If the longer route costs less, book it and exit at " + in.Destination,
			"Works on FlixBus, RegioJet, DB, SNCF, Trenitalia, and most European operators",
		},
		Risks: []string{
			"Your luggage must be with you (not in a cargo hold you can't access mid-route)",
			"Verify the bus/train actually stops at your destination (express services may skip it)",
		},
	}}
}
