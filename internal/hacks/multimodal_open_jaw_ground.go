package hacks

import (
	"context"
	"fmt"
	"sync"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/ground"
)

// nearbyHub describes a major airport near a regional destination. Flying into
// the hub and taking ground transport to the final destination is often cheaper
// than flying directly to the regional airport.
type nearbyHub struct {
	// HubCode is the IATA code of the larger/cheaper hub airport.
	HubCode string
	// HubCity is the city name used for display and ground search.
	HubCity string
	// DestCity is the city name of the final destination for ground search.
	DestCity string
	// StaticGroundEUR is a conservative estimate of ground cost (EUR).
	StaticGroundEUR float64
	// Overnight is true when the ground leg is typically overnight (adds hotel saving).
	Overnight bool
	// Notes describes the ground connection.
	Notes string
}

// nearbyHubs maps a destination IATA code to hub airports from which ground
// transport can complete the journey more cheaply.
var nearbyHubs = map[string][]nearbyHub{
	// Croatia
	"DBV": {
		{
			HubCode: "ZAG", HubCity: "Zagreb", DestCity: "Dubrovnik",
			StaticGroundEUR: 15, Overnight: true,
			Notes: "FlixBus ZAG→DBV (~8h overnight; departs ~22:00 arrives ~06:00)",
		},
		{
			HubCode: "SPU", HubCity: "Split", DestCity: "Dubrovnik",
			StaticGroundEUR: 10, Overnight: false,
			Notes: "FlixBus SPU→DBV (~4.5h)",
		},
	},
	"SPU": {
		{
			HubCode: "ZAG", HubCity: "Zagreb", DestCity: "Split",
			StaticGroundEUR: 12, Overnight: false,
			Notes: "FlixBus ZAG→SPU (~5.5h)",
		},
	},
	// Czech Republic / Slovakia
	"OSR": {
		{
			HubCode: "BTS", HubCity: "Bratislava", DestCity: "Ostrava",
			StaticGroundEUR: 10, Overnight: false,
			Notes: "Bus BTS→OSR (~3h)",
		},
		{
			HubCode: "KRK", HubCity: "Krakow", DestCity: "Ostrava",
			StaticGroundEUR: 8, Overnight: false,
			Notes: "Bus KRK→OSR (~2h)",
		},
	},
	// Portugal
	"FAO": {
		{
			HubCode: "LIS", HubCity: "Lisbon", DestCity: "Faro",
			StaticGroundEUR: 20, Overnight: false,
			Notes: "Bus/train LIS→FAO (~3.5h)",
		},
	},
	// Greece
	"JMK": {
		{
			HubCode: "ATH", HubCity: "Athens", DestCity: "Mykonos",
			StaticGroundEUR: 30, Overnight: false,
			Notes: "Ferry ATH (Piraeus)→JMK (~5h)",
		},
	},
	"JSI": {
		{
			HubCode: "ATH", HubCity: "Athens", DestCity: "Skiathos",
			StaticGroundEUR: 35, Overnight: false,
			Notes: "Ferry ATH (Piraeus)→JSI (~4h)",
		},
	},
	// Italy
	"BRI": {
		{
			HubCode: "NAP", HubCity: "Naples", DestCity: "Bari",
			StaticGroundEUR: 20, Overnight: false,
			Notes: "FlixBus NAP→BRI (~4h)",
		},
	},
	// Germany
	"NUE": {
		{
			HubCode: "MUC", HubCity: "Munich", DestCity: "Nuremberg",
			StaticGroundEUR: 15, Overnight: false,
			Notes: "Train MUC→NUE (~1h10)",
		},
	},
}

// detectMultiModalOpenJawGround checks whether flying to a nearby hub airport
// and completing the journey by ground transport is cheaper than flying directly
// to the destination.
func detectMultiModalOpenJawGround(ctx context.Context, in DetectorInput) []Hack {
	if !in.valid() || in.Date == "" {
		return nil
	}

	hubs, ok := nearbyHubs[in.Destination]
	if !ok {
		return nil
	}

	// Baseline: direct flight to destination.
	directResult, err := flights.SearchFlights(ctx, in.Origin, in.Destination, in.Date, flights.SearchOptions{})
	if err != nil || !directResult.Success || len(directResult.Flights) == 0 {
		return nil
	}
	directPrice := minFlightPrice(directResult)
	if directPrice <= 0 {
		return nil
	}
	currency := flightCurrency(directResult, in.currency())

	type candidate struct {
		hub       nearbyHub
		groundEUR float64
		flightEUR float64
	}

	ch := make(chan candidate, len(hubs))
	var wg sync.WaitGroup

	for _, h := range hubs {
		h := h
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Live ground price (fallback to static estimate).
			groundEUR := h.StaticGroundEUR
			gr, gerr := ground.SearchByName(ctx, h.HubCity, h.DestCity, in.Date, ground.SearchOptions{
				Currency: "EUR",
			})
			if gerr == nil && gr.Success {
				for _, r := range gr.Routes {
					if r.Price > 0 && r.Price < groundEUR {
						groundEUR = r.Price
					}
				}
			}

			// Flight from origin to hub.
			fr, ferr := flights.SearchFlights(ctx, in.Origin, h.HubCode, in.Date, flights.SearchOptions{})
			if ferr != nil || !fr.Success || len(fr.Flights) == 0 {
				ch <- candidate{}
				return
			}
			flightPrice := minFlightPrice(fr)
			if flightPrice <= 0 {
				ch <- candidate{}
				return
			}
			ch <- candidate{hub: h, groundEUR: groundEUR, flightEUR: flightPrice}
		}()
	}

	wg.Wait()
	close(ch)

	var hacks []Hack
	for c := range ch {
		if c.flightEUR == 0 {
			continue
		}

		total := c.flightEUR + c.groundEUR
		hotelBonus := 0.0
		if c.hub.Overnight {
			hotelBonus = averageHotelCost
		}
		savings := directPrice - total + hotelBonus
		if savings < 50 {
			continue
		}

		overnightNote := ""
		if c.hub.Overnight {
			overnightNote = fmt.Sprintf(" + saves ~%.0f hotel night", averageHotelCost)
		}

		hacks = append(hacks, Hack{
			Type:     "multimodal_open_jaw_ground",
			Title:    fmt.Sprintf("Fly to %s, complete journey to %s by ground", c.hub.HubCity, cityFromCode(in.Destination)),
			Currency: currency,
			Savings:  roundSavings(savings),
			Description: fmt.Sprintf(
				"Flight %s→%s (%.0f %s) + ground %s→%s (%.0f %s) = %.0f total%s, vs direct %s→%s %.0f %s. Saves %s %.0f.",
				in.Origin, c.hub.HubCode, c.flightEUR, currency,
				c.hub.HubCity, c.hub.DestCity, c.groundEUR, currency,
				total, overnightNote,
				in.Origin, in.Destination, directPrice, currency,
				currency, savings,
			),
			Risks: []string{
				"Two separate bookings — if the flight is delayed you may miss your ground connection",
				"Ground leg adds travel time; plan accordingly",
				"Checked luggage complicates ground connections — prefer carry-on",
			},
			Steps: []string{
				fmt.Sprintf("Book flight %s→%s on %s (%s %.0f)", in.Origin, c.hub.HubCode, in.Date, currency, c.flightEUR),
				fmt.Sprintf("Ground transfer: %s", c.hub.Notes),
				fmt.Sprintf("Arrive at %s (~%.0f EUR ground cost)", c.hub.DestCity, c.groundEUR),
			},
			Citations: []string{
				googleFlightsURL(c.hub.HubCode, in.Origin, in.Date),
			},
		})
	}

	return hacks
}
