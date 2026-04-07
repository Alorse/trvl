package hacks

import (
	"context"
	"fmt"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
)

// cheapDays lists weekdays that are statistically cheaper for flying.
var cheapDays = []time.Weekday{time.Tuesday, time.Wednesday, time.Saturday}

// expensiveDays lists weekdays that are statistically more expensive.
var expensiveDays = map[time.Weekday]bool{
	time.Friday: true,
	time.Sunday: true,
}

// tuesdayBookingWindow is the number of days either side to search for cheaper
// weekday alternatives. Larger than dateFlexWindow (3) to cover the nearest
// cheap-day candidates.
const tuesdayBookingWindow = 5

// tuesdayBookingMinSaving is the minimum saving (EUR) to surface this hack.
// Lower than dateFlexMinSaving because the weekday signal is directional.
const tuesdayBookingMinSaving = 10.0

// detectTuesdayBooking checks if the requested departure falls on a typically
// expensive weekday (Friday/Sunday) and searches for cheaper Tuesday/Wednesday/
// Saturday alternatives within ±5 days.
func detectTuesdayBooking(ctx context.Context, in DetectorInput) []Hack {
	if in.Date == "" || in.Origin == "" || in.Destination == "" {
		return nil
	}

	t, err := parseDate(in.Date)
	if err != nil {
		return nil
	}

	// Only trigger when the requested date is on an expensive day.
	if !expensiveDays[t.Weekday()] {
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

	// Build list of cheap-day candidates within the window.
	type dateCandidate struct {
		date    string
		weekday time.Weekday
	}
	var candidates []dateCandidate
	for delta := -tuesdayBookingWindow; delta <= tuesdayBookingWindow; delta++ {
		if delta == 0 {
			continue
		}
		altDate := addDays(in.Date, delta)
		if altDate == "" {
			continue
		}
		altT, err := parseDate(altDate)
		if err != nil {
			continue
		}
		if isCheapDay(altT.Weekday()) {
			candidates = append(candidates, dateCandidate{date: altDate, weekday: altT.Weekday()})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	type searchResult struct {
		date    string
		weekday time.Weekday
		price   float64
	}
	ch := make(chan searchResult, len(candidates))

	for _, c := range candidates {
		c := c
		go func() {
			r, err := flights.SearchFlights(ctx, in.Origin, in.Destination, c.date, flights.SearchOptions{
				ReturnDate: adjustReturnDate(in.ReturnDate, dateDelta(in.Date, c.date)),
			})
			if err != nil || !r.Success || len(r.Flights) == 0 {
				ch <- searchResult{}
				return
			}
			ch <- searchResult{date: c.date, weekday: c.weekday, price: minFlightPrice(r)}
		}()
	}

	bestDate := ""
	bestWeekday := time.Monday
	bestSaving := 0.0

	for range candidates {
		res := <-ch
		if res.date == "" || res.price <= 0 {
			continue
		}
		saving := basePrice - res.price
		if saving > bestSaving {
			bestSaving = saving
			bestDate = res.date
			bestWeekday = res.weekday
		}
	}

	if bestSaving < tuesdayBookingMinSaving || bestDate == "" {
		return nil
	}

	return []Hack{{
		Type:     "tuesday_booking",
		Title:    fmt.Sprintf("Fly %s instead of %s — saves %s %.0f", bestWeekday, t.Weekday(), currency, bestSaving),
		Currency: currency,
		Savings:  roundSavings(bestSaving),
		Description: fmt.Sprintf(
			"%s flights are typically 10-15%% cheaper than %ss. Flying on %s (%s) instead of %s (%s) saves %s %.0f.",
			bestWeekday, t.Weekday(),
			bestWeekday, bestDate,
			t.Weekday(), in.Date,
			currency, bestSaving,
		),
		Risks: []string{
			"Verify your flexibility with any hotel/accommodation bookings",
			"Prices may change between searching and booking",
			"Cheaper date may have fewer flight options or longer travel time",
		},
		Steps: []string{
			fmt.Sprintf("Search %s→%s on %s (%s)", in.Origin, in.Destination, bestDate, bestWeekday),
			"Book the cheaper weekday option",
			"Update hotel or connecting transport if switching dates",
		},
		Citations: []string{
			fmt.Sprintf("https://www.google.com/travel/flights?q=Flights+to+%s+from+%s+on+%s", in.Destination, in.Origin, bestDate),
		},
	}}
}

// isCheapDay returns true when the weekday is in the cheapDays list.
func isCheapDay(w time.Weekday) bool {
	for _, d := range cheapDays {
		if d == w {
			return true
		}
	}
	return false
}

// dateDelta returns the integer day difference between two YYYY-MM-DD strings.
// Returns 0 on error.
func dateDelta(base, alt string) int {
	b, err1 := parseDate(base)
	a, err2 := parseDate(alt)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(a.Sub(b).Hours() / 24)
}
