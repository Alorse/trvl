package hacks

import (
	"context"
)

// detectDayUse suggests day-use hotel rooms for travellers who may have long
// layovers. Day-use services offer 4-8 hour room blocks at 50-70% of the
// overnight rate. Purely advisory — zero API calls.
func detectDayUse(_ context.Context, in DetectorInput) []Hack {
	if !in.valid() {
		return nil
	}

	return []Hack{{
		Type:  "day_use_hotel",
		Title: "Long layover? Book a day-use hotel room (50-70% off overnight)",
		Description: "Services like DayUse.com and HotelsByDay offer 4-8 hour room blocks " +
			"at major airports. Useful for layovers over 4 hours.",
		Savings:  0, // advisory — savings depend on specific hotel and layover
		Currency: in.currency(),
		Steps: []string{
			"Check DayUse.com or HotelsByDay.com at your connection airport",
			"Typical 6-hour block: EUR 35-60 vs EUR 100-150 overnight",
			"Most airport hotels participate — Novotel, Ibis, Holiday Inn, etc.",
		},
		Risks: []string{
			"Must account for travel time to/from hotel",
			"Availability can be limited on peak days",
		},
		Citations: []string{"https://www.dayuse.com", "https://www.hotelsbyday.com"},
	}}
}
