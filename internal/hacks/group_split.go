package hacks

import (
	"context"
	"fmt"
)

// detectGroupSplit advises splitting group bookings into individual searches.
// Airlines price all passengers at the most expensive fare bucket needed to
// fill the request. When searching for 4 passengers and only 2 seats remain
// at the cheapest fare, all 4 are priced at the next bucket up. Booking in
// batches of 1-2 can access cheaper buckets.
func detectGroupSplit(_ context.Context, in DetectorInput) []Hack {
	if in.Passengers < 3 || in.NaivePrice <= 0 {
		return nil
	}

	passengers := in.Passengers
	totalPrice := in.NaivePrice
	currency := in.currency()

	perPerson := totalPrice / float64(passengers)
	savings := totalPrice * estimatedSavingsRate(passengers)

	return []Hack{{
		Type:     "group_split",
		Title:    "Search individually — groups pay more",
		Currency: currency,
		Savings:  roundSavings(savings),
		Description: fmt.Sprintf(
			"Airlines price all %d passengers at the most expensive fare bucket needed. "+
				"Searching as 1 passenger may find cheaper seats. Book in batches of 1-2.",
			passengers),
		Steps: []string{
			fmt.Sprintf("Current: %d passengers x %.0f %s = %.0f %s total",
				passengers, perPerson, currency, totalPrice, currency),
			"Re-search with 1 passenger to check if cheaper fares are available",
			"If cheaper: book 1-2 seats at a time (different bookings, same flights)",
			"Tip: book all within minutes — fare buckets can sell out quickly",
		},
		Risks: []string{
			"Separate bookings mean separate check-ins and no shared booking reference",
			"If the flight is changed/cancelled, each booking is handled independently",
			"Seat selection may need to be purchased to sit together",
		},
	}}
}

// estimatedSavingsRate returns the estimated percentage savings for splitting
// a group booking, based on the number of passengers.
func estimatedSavingsRate(passengers int) float64 {
	switch {
	case passengers >= 6:
		return 0.20 // 20% estimated savings for large groups
	case passengers >= 4:
		return 0.15 // 15% for medium groups
	default:
		return 0.10 // 10% for groups of 3
	}
}
