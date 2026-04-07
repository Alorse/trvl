package hacks

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

// openJawAlternates lists nearby airports that are reasonable alternate return
// points for an open-jaw itinerary. Keyed by the destination airport: if
// you fly OUT→DEST, you could return from one of these instead.
var openJawAlternates = map[string][]string{
	"PRG": {"BRU", "AMS", "FRA", "VIE", "MUC"},
	"AMS": {"BRU", "FRA", "CDG", "DUS"},
	"CDG": {"BRU", "AMS", "FRA"},
	"BCN": {"MAD", "GRO", "REU"},
	"MAD": {"BCN", "LIS", "VLC"},
	"LIS": {"OPO", "MAD"},
	"FCO": {"MXP", "CIA", "NAP"},
	"MXP": {"FCO", "BGY", "TRN"},
	"MUC": {"NUE", "FMM", "FRA"},
	"FRA": {"MUC", "DUS", "CGN"},
	"BER": {"HAM", "FRA"},
	"VIE": {"PRG", "BUD", "BRN"},
	"BUD": {"VIE", "PRG", "KRK"},
	"WAW": {"KRK", "WRO", "GDN"},
	"CPH": {"GOT", "ARN", "MMX"},
	"ARN": {"CPH", "GOT"},
	"OSL": {"BGO", "SVG"},
	"ATH": {"SKG", "HER"},
	"IST": {"SAW", "ESB"},
	"DUB": {"ORK", "SNN"},
}

// detectOpenJaw detects open-jaw opportunities: fly into city A, return from
// city B. When one end is the user's home airport this is particularly powerful
// because the return ticket from home is not needed.
func detectOpenJaw(ctx context.Context, in DetectorInput) []Hack {
	if in.Date == "" || in.ReturnDate == "" || in.Origin == "" || in.Destination == "" {
		return nil
	}

	prefs, _ := preferences.Load()

	// Check if origin is a home airport — the strongest open-jaw signal.
	isHome := isHomeAirport(in.Origin, prefs)

	// Baseline: round-trip from origin to destination.
	rtResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{
		ReturnDate: in.ReturnDate,
	})
	if err != nil || !rtResult.Success || len(rtResult.Flights) == 0 {
		return nil
	}
	rtPrice := minFlightPrice(rtResult)
	if rtPrice <= 0 {
		return nil
	}
	currency := flightCurrency(rtResult, in.currency())

	// One-way outbound (origin → destination).
	owOutResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{})
	if err != nil || !owOutResult.Success || len(owOutResult.Flights) == 0 {
		return nil
	}
	owOutPrice := minFlightPrice(owOutResult)
	if owOutPrice <= 0 {
		return nil
	}

	// Try each alternate return city.
	alts, ok := openJawAlternates[in.Destination]
	if !ok {
		return nil
	}

	type ch struct {
		alt   string
		price float64
	}
	results := make(chan ch, len(alts))

	for _, alt := range alts {
		if alt == in.Origin {
			continue
		}
		alt := alt
		go func() {
			r, err := flights.SearchFlights(ctx, alt, in.Origin, in.ReturnDate, flights.SearchOptions{})
			if err != nil || !r.Success || len(r.Flights) == 0 {
				results <- ch{alt: alt, price: 0}
				return
			}
			results <- ch{alt: alt, price: minFlightPrice(r)}
		}()
	}

	var hacks []Hack
	for range alts {
		res := <-results
		if res.price <= 0 {
			continue
		}

		// Estimated ground cost to reach alternate return city (conservative).
		groundCost := groundCostBetween(in.Destination, res.alt)
		totalOpenJaw := owOutPrice + res.price + groundCost
		savings := rtPrice - totalOpenJaw

		// Require at least EUR 20 saving, or strong home-airport signal.
		minSaving := 20.0
		if isHome {
			minSaving = 10.0
		}
		if savings < minSaving {
			continue
		}

		h := Hack{
			Type:     "open_jaw",
			Title:    fmt.Sprintf("Open-jaw: fly into %s, return from %s", in.Destination, res.alt),
			Currency: currency,
			Savings:  roundSavings(savings),
			Description: fmt.Sprintf(
				"Fly %s→%s one-way (%.0f) + travel to %s + fly %s→%s (%.0f) = %.0f total, vs round-trip %.0f. Saves %s %.0f and lets you visit two areas.",
				in.Origin, in.Destination, owOutPrice,
				res.alt, res.alt, in.Origin, res.price,
				totalOpenJaw, rtPrice, currency, savings,
			),
			Risks: []string{
				"You must make your own way from " + in.Destination + " to " + res.alt + " (train/bus)",
				"One-way tickets may have different fare conditions than round-trips",
				"Prices are for separate bookings — lock in both at the same time",
			},
			Steps: []string{
				fmt.Sprintf("Book one-way %s→%s on %s (%s %.0f)", in.Origin, in.Destination, in.Date, currency, owOutPrice),
				fmt.Sprintf("Travel from %s to %s by ground transport", in.Destination, res.alt),
				fmt.Sprintf("Book one-way %s→%s on %s (%s %.0f)", res.alt, in.Origin, in.ReturnDate, currency, res.price),
			},
			Citations: []string{
				fmt.Sprintf("https://www.google.com/travel/flights?q=Flights+to+%s+from+%s+on+%s", in.Destination, in.Origin, in.Date),
				fmt.Sprintf("https://www.google.com/travel/flights?q=Flights+to+%s+from+%s+on+%s", in.Origin, res.alt, in.ReturnDate),
			},
		}
		hacks = append(hacks, h)
	}

	return hacks
}

// isHomeAirport returns true when the given airport code is in the user's home airports.
func isHomeAirport(code string, prefs *preferences.Preferences) bool {
	if prefs == nil {
		return false
	}
	for _, ha := range prefs.HomeAirports {
		if ha == code {
			return true
		}
	}
	return false
}

// groundCostBetween returns a conservative estimate (EUR) of ground transport
// cost between two nearby airports/cities.
func groundCostBetween(from, to string) float64 {
	// Known short ground connections.
	pairs := map[[2]string]float64{
		{"PRG", "BRN"}: 10,
		{"PRG", "VIE"}: 15,
		{"VIE", "BUD"}: 15,
		{"AMS", "BRU"}: 20,
		{"AMS", "FRA"}: 25,
		{"AMS", "DUS"}: 20,
		{"BCN", "MAD"}: 30,
		{"FCO", "MXP"}: 30,
		{"CPH", "ARN"}: 30,
		{"CPH", "GOT"}: 20,
	}
	if v, ok := pairs[[2]string{from, to}]; ok {
		return v
	}
	if v, ok := pairs[[2]string{to, from}]; ok {
		return v
	}
	// Default: assume EUR 25 for any nearby pair.
	return 25
}
