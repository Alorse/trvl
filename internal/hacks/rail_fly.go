package hacks

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// railFlyStation describes a railway station that an airline sells as a flight origin.
type railFlyStation struct {
	IATA          string // Rail station IATA code (e.g. "ZWE" for Antwerp)
	City          string // City name (e.g. "Antwerp")
	HubIATA       string // Airline hub airport (e.g. "AMS")
	Airline       string // IATA carrier code (e.g. "KL")
	AirlineName   string // Display name (e.g. "KLM")
	TrainProvider string // Train operator (e.g. "Eurostar")
	TrainMinutes  int    // Approximate train journey time
	FareZone      string // Why it's cheaper: "Belgian market", "German regional", etc.
}

var railFlyStations = []railFlyStation{
	// KLM Air&Rail — trains to Amsterdam Schiphol
	{IATA: "ZWE", City: "Antwerp", HubIATA: "AMS", Airline: "KL", AirlineName: "KLM", TrainProvider: "Eurostar", TrainMinutes: 60, FareZone: "Belgian market"},
	{IATA: "ZYR", City: "Brussels-Midi", HubIATA: "AMS", Airline: "KL", AirlineName: "KLM", TrainProvider: "Eurostar", TrainMinutes: 105, FareZone: "Belgian market"},

	// Lufthansa AIRail — ICE trains to Frankfurt Airport
	{IATA: "QKL", City: "Cologne", HubIATA: "FRA", Airline: "LH", AirlineName: "Lufthansa", TrainProvider: "DB ICE", TrainMinutes: 62, FareZone: "Rhineland regional"},
	{IATA: "ZWS", City: "Stuttgart", HubIATA: "FRA", Airline: "LH", AirlineName: "Lufthansa", TrainProvider: "DB ICE", TrainMinutes: 78, FareZone: "Baden-Württemberg regional"},
	{IATA: "QDU", City: "Düsseldorf Hbf", HubIATA: "FRA", Airline: "LH", AirlineName: "Lufthansa", TrainProvider: "DB ICE", TrainMinutes: 82, FareZone: "NRW regional"},
	{IATA: "QMZ", City: "Mannheim", HubIATA: "FRA", Airline: "LH", AirlineName: "Lufthansa", TrainProvider: "DB ICE", TrainMinutes: 38, FareZone: "Rhein-Neckar regional"},
	{IATA: "QBO", City: "Bonn", HubIATA: "FRA", Airline: "LH", AirlineName: "Lufthansa", TrainProvider: "DB ICE", TrainMinutes: 55, FareZone: "Rhineland regional"},
	{IATA: "ZAQ", City: "Nuremberg", HubIATA: "FRA", Airline: "LH", AirlineName: "Lufthansa", TrainProvider: "DB ICE", TrainMinutes: 127, FareZone: "Bavaria regional"},
	{IATA: "QPP", City: "Kassel-Wilhelmshöhe", HubIATA: "FRA", Airline: "LH", AirlineName: "Lufthansa", TrainProvider: "DB ICE", TrainMinutes: 90, FareZone: "Hesse regional"},

	// Air France TGV Air — TGV trains to Paris CDG
	{IATA: "ZYR", City: "Brussels-Midi", HubIATA: "CDG", Airline: "AF", AirlineName: "Air France", TrainProvider: "Thalys/TGV", TrainMinutes: 80, FareZone: "Belgian market"},

	// Swiss Rail+Air — trains to Zurich Airport
	{IATA: "ZDH", City: "Basel", HubIATA: "ZRH", Airline: "LX", AirlineName: "Swiss", TrainProvider: "SBB", TrainMinutes: 80, FareZone: "Basel border zone"},
}

// DetectRailFlyArbitrage checks if booking via a rail-connected origin
// (e.g., Antwerp instead of Amsterdam for KLM) triggers a cheaper fare zone.
// The train segment is free — included in the airline ticket.
func DetectRailFlyArbitrage(ctx context.Context, origin, destination, departDate, returnDate string) []Hack {
	if origin == "" || destination == "" || departDate == "" {
		return nil
	}
	origin = strings.ToUpper(origin)
	destination = strings.ToUpper(destination)

	// Find rail stations that connect to this origin airport as a hub
	stations := railFlyStationsForHub(origin)
	if len(stations) == 0 {
		return nil
	}

	// Search baseline price from the actual airport
	client := batchexec.NewClient()

	baseOpts := flights.SearchOptions{
		SortBy: models.SortCheapest,
	}
	if returnDate != "" {
		baseOpts.ReturnDate = returnDate
	}

	baseResult, baseErr := flights.SearchFlightsWithClient(ctx, client, origin, destination, departDate, baseOpts)
	basePrice, baseCurrency, _ := cheapestFlightInfo(baseResult, baseErr)
	if basePrice <= 0 {
		return nil
	}

	// Search from each rail-connected origin in parallel
	type railResult struct {
		station  railFlyStation
		price    float64
		currency string
	}

	results := make(chan railResult, len(stations))
	var wg sync.WaitGroup

	for _, st := range stations {
		st := st
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := flights.SearchOptions{
				SortBy: models.SortCheapest,
			}
			if returnDate != "" {
				opts.ReturnDate = returnDate
			}
			res, err := flights.SearchFlightsWithClient(ctx, client, st.IATA, destination, departDate, opts)
			p, c, _ := cheapestFlightInfo(res, err)
			results <- railResult{station: st, price: p, currency: c}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Find the cheapest rail alternative
	var bestStation *railFlyStation
	bestPrice := basePrice
	bestCurrency := baseCurrency

	for r := range results {
		if r.price > 0 && r.price < bestPrice {
			bestPrice = r.price
			bestCurrency = r.currency
			st := r.station
			bestStation = &st
		}
	}

	if bestStation == nil {
		return nil
	}

	savings := basePrice - bestPrice
	// Only report if savings exceed 5% AND at least 15 absolute
	if savings < 15 || savings/basePrice < 0.05 {
		return nil
	}

	return []Hack{buildRailFlyHack(origin, destination, basePrice, baseCurrency, bestPrice, bestCurrency, savings, *bestStation, returnDate)}
}

// detectRailFlyArbitrage adapts DetectRailFlyArbitrage to the detectFn signature
// used by DetectAll.
func detectRailFlyArbitrage(ctx context.Context, in DetectorInput) []Hack {
	return DetectRailFlyArbitrage(ctx, in.Origin, in.Destination, in.Date, in.ReturnDate)
}

func railFlyStationsForHub(hubIATA string) []railFlyStation {
	var result []railFlyStation
	for _, st := range railFlyStations {
		if st.HubIATA == hubIATA {
			result = append(result, st)
		}
	}
	return result
}

func buildRailFlyHack(origin, destination string, basePrice float64, baseCurrency string, railPrice float64, railCurrency string, savings float64, station railFlyStation, returnDate string) Hack {
	tripType := "one-way"
	if returnDate != "" {
		tripType = "round-trip"
	}

	return Hack{
		Type:  "rail_fly_arbitrage",
		Title: fmt.Sprintf("Book via %s — train to %s is free, saves %.0f %s", station.City, station.HubIATA, savings, baseCurrency),
		Description: fmt.Sprintf(
			"Book %s %s→%s instead of %s→%s. The %s train from %s to %s airport (%d min) is included free in the ticket. Different fare zone (%s) triggers a cheaper price.",
			station.AirlineName, station.IATA, destination, origin, destination,
			station.TrainProvider, station.City, origin, station.TrainMinutes, station.FareZone),
		Savings:  roundSavings(savings),
		Currency: baseCurrency,
		Steps: []string{
			fmt.Sprintf("Direct from %s: %.0f %s (%s)", origin, basePrice, baseCurrency, tripType),
			fmt.Sprintf("Via %s (%s): %.0f %s (%s) — %.0f %s cheaper", station.City, station.IATA, railPrice, railCurrency, tripType, savings, baseCurrency),
			fmt.Sprintf("Take %s from %s to %s (%d min, included in ticket)", station.TrainProvider, station.City, origin, station.TrainMinutes),
			"Train ticket appears as a flight segment in the booking — board with your airline booking reference",
		},
		Risks: []string{
			"You MUST board the train — skipping the first segment cancels the entire booking",
			fmt.Sprintf("Allow %d+ minutes for the train journey plus airport transfer", station.TrainMinutes+30),
			"Train is flexible within the travel day (any departure, not fixed to one schedule)",
		},
		Citations: []string{
			fmt.Sprintf("https://www.google.com/travel/flights?q=%s%%20to%%20%s", station.IATA, destination),
			fmt.Sprintf("https://www.google.com/travel/flights?q=%s%%20to%%20%s", origin, destination),
		},
	}
}
