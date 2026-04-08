package hacks

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/flights"
)

// detectDateFlex searches ±3 days around the requested departure date and
// flags cheaper alternatives when the saving exceeds a threshold.
const dateFlexWindow = 3 // days each direction
const dateFlexMinSaving = 20.0

// detectDateFlex finds cheaper dates within ±3 days of the requested date.
func detectDateFlex(ctx context.Context, in DetectorInput) []Hack {
	if in.Date == "" || in.Origin == "" || in.Destination == "" {
		return nil
	}

	// Baseline price for the requested date.
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

	// Channel to collect results from parallel searches.
	type searchResult struct {
		date  string
		price float64
	}

	ch := make(chan searchResult, dateFlexWindow*2)
	pending := 0

	for delta := -dateFlexWindow; delta <= dateFlexWindow; delta++ {
		if delta == 0 {
			continue
		}
		altDate := addDays(in.Date, delta)
		if altDate == "" {
			continue
		}
		pending++

		returnDate := adjustReturnDate(in.ReturnDate, delta)

		go func(d, ret string) {
			altResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, d, flights.SearchOptions{
				ReturnDate: ret,
			})
			if err != nil || !altResult.Success || len(altResult.Flights) == 0 {
				ch <- searchResult{}
				return
			}
			ch <- searchResult{date: d, price: minFlightPrice(altResult)}
		}(altDate, returnDate)
	}

	bestDate := ""
	bestSaving := 0.0

	for i := 0; i < pending; i++ {
		r := <-ch
		if r.date == "" || r.price <= 0 {
			continue
		}
		saving := basePrice - r.price
		if saving > bestSaving {
			bestSaving = saving
			bestDate = r.date
		}
	}

	if bestSaving < dateFlexMinSaving || bestDate == "" {
		return nil
	}

	return []Hack{{
		Type:     "date_flex",
		Title:    "Date flexibility saves money",
		Currency: currency,
		Savings:  roundSavings(bestSaving),
		Description: fmt.Sprintf(
			"Flying on %s instead of %s saves %s %.0f (%.0f vs %.0f).",
			bestDate, in.Date, currency, bestSaving, basePrice-bestSaving, basePrice,
		),
		Risks: []string{
			"Verify your flexibility with any hotel/accommodation bookings",
			"Prices may change between searching and booking",
		},
		Steps: []string{
			fmt.Sprintf("Search flights %s→%s on %s", in.Origin, in.Destination, bestDate),
			"Book the cheaper date",
			"Update any hotel or connecting-transport bookings accordingly",
		},
		Citations: []string{
			fmt.Sprintf("https://www.google.com/travel/flights?q=Flights+to+%s+from+%s+on+%s", in.Destination, in.Origin, bestDate),
		},
	}}
}

// adjustReturnDate shifts the return date by the same delta as the outbound
// date, so round-trip windows stay consistent.
func adjustReturnDate(returnDate string, delta int) string {
	if returnDate == "" {
		return ""
	}
	return addDays(returnDate, delta)
}
