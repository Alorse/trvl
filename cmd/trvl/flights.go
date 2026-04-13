package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/deals"
	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/spf13/cobra"
)

func flightsCmd() *cobra.Command {
	var (
		returnDate     string
		cabin          string
		maxStops       string
		sortBy         string
		airlines       []string
		adults         int
		format         string
		targetCurrency string
		compareCabins  bool
	)

	cmd := &cobra.Command{
		Use:   "flights ORIGIN DESTINATION DATE",
		Short: "Search flights between airports (supports multi-airport)",
		Long: `Search flights between airports on a specific date.

ORIGIN and DESTINATION are IATA codes, comma-separated for multi-airport.
DATE is the departure date in YYYY-MM-DD format.

Examples:
  trvl flights HEL NRT 2026-06-15
  trvl flights AMS,EIN,ANR HEL,TKU,TLL 2026-06-15
  trvl flights HEL NRT 2026-06-15 --return 2026-06-22
  trvl flights HEL NRT 2026-06-15 --cabin business --stops nonstop`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			originArg := args[0]

			// If the user passes "home" as origin, resolve from preferences.
			if strings.EqualFold(strings.TrimSpace(originArg), "home") {
				if prefs, err := preferences.Load(); err == nil && prefs.HomeAirport() != "" {
					originArg = prefs.HomeAirport()
				}
			}

			origins := flights.ParseAirports(originArg)
			destinations := flights.ParseAirports(args[1])
			date := args[2]

			cabinClass, err := models.ParseCabinClass(cabin)
			if err != nil {
				return fmt.Errorf("invalid cabin class: %w", err)
			}

			stops, err := models.ParseMaxStops(maxStops)
			if err != nil {
				return fmt.Errorf("invalid max stops: %w", err)
			}

			sort, err := models.ParseSortBy(sortBy)
			if err != nil {
				return fmt.Errorf("invalid sort order: %w", err)
			}

			opts := flights.SearchOptions{
				ReturnDate: returnDate,
				CabinClass: cabinClass,
				MaxStops:   stops,
				SortBy:     sort,
				Airlines:   airlines,
				Adults:     adults,
			}

			// --compare-cabins: search all cabin classes in parallel.
			if compareCabins {
				return runCabinComparison(cmd.Context(), origins, destinations, date, opts, format)
			}

			var result *models.FlightSearchResult
			if len(origins) > 1 || len(destinations) > 1 {
				result, err = flights.SearchMultiAirport(cmd.Context(), origins, destinations, date, opts)
			} else {
				result, err = flights.SearchFlights(cmd.Context(), origins[0], destinations[0], date, opts)
			}
			if err != nil {
				return err
			}

			// Cache best result for `trvl share --last`.
			if result != nil && result.Success && len(result.Flights) > 0 {
				f := result.Flights[0]
				airline := ""
				if len(f.Legs) > 0 {
					airline = f.Legs[0].Airline
				}
				if airline == "" {
					airline = flightProviderLabel(f)
				}
				saveLastSearch(&LastSearch{
					Command:        "flights",
					Origin:         strings.Join(origins, ","),
					Destination:    strings.Join(destinations, ","),
					DepartDate:     date,
					FlightPrice:    f.Price,
					FlightCurrency: f.Currency,
					FlightAirline:  airline,
					FlightStops:    f.Stops,
				})
			}

			if format == "json" {
				return models.FormatJSON(os.Stdout, result)
			}

			if err := printFlightsTable(cmd.Context(), strings.Join(origins, ","), strings.Join(destinations, ","), targetCurrency, result); err != nil {
				return err
			}

			if openFlag && result.Success && len(result.Flights) > 0 && result.Flights[0].BookingURL != "" {
				_ = openBrowser(result.Flights[0].BookingURL)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&returnDate, "return", "", "Return date for round-trip (YYYY-MM-DD)")
	cmd.Flags().StringVar(&cabin, "cabin", "economy", "Cabin class: economy, premium_economy, business, first")
	cmd.Flags().StringVar(&maxStops, "stops", "any", "Max stops: any, nonstop, one_stop, two_plus")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort by: cheapest, duration, departure, arrival")
	cmd.Flags().StringSliceVar(&airlines, "airline", nil, "Filter by airline IATA code (repeatable)")
	cmd.Flags().IntVar(&adults, "adults", 1, "Number of adult passengers")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&targetCurrency, "currency", "", "Convert prices to this currency (e.g. EUR, USD). Empty = show API default")
	cmd.Flags().BoolVar(&compareCabins, "compare-cabins", false, "Compare prices across all cabin classes (economy, premium, business, first)")

	cmd.ValidArgsFunction = airportCompletion

	return cmd
}

// printFlightsTable renders flight results as an ASCII table.
// If targetCurrency is set and differs from API currency, converts prices.
func printFlightsTable(ctx context.Context, origin, destination, targetCurrency string, result *models.FlightSearchResult) error {
	if !result.Success {
		fmt.Fprintf(os.Stderr, "Search failed: %s\n", result.Error)
		return nil
	}

	if result.Count == 0 {
		fmt.Println("No flights found.")
		return nil
	}

	// Check for matching deals from RSS feeds (cached, non-blocking).
	bannerLines := []string{fmt.Sprintf("Found %d flights", result.Count)}
	matchedDeals := deals.MatchDeals(ctx, origin, destination)
	for _, d := range matchedDeals {
		dealLine := fmt.Sprintf("🔥 %s: %s", deals.SourceNames[d.Source], d.Title)
		if len(dealLine) > 70 {
			dealLine = dealLine[:67] + "..."
		}
		bannerLines = append(bannerLines, dealLine)
	}

	models.Banner(os.Stdout, "✈️", fmt.Sprintf("Flights · %s", result.TripType), bannerLines...)
	fmt.Println()

	// Convert prices if --currency specified and differs from API currency.
	if targetCurrency != "" && len(result.Flights) > 0 && result.Flights[0].Currency != targetCurrency {
		for i := range result.Flights {
			if result.Flights[i].Price > 0 && result.Flights[i].Currency != targetCurrency {
				converted, cur := destinations.ConvertCurrency(ctx, result.Flights[i].Price, result.Flights[i].Currency, targetCurrency)
				result.Flights[i].Price = math.Round(converted)
				result.Flights[i].Currency = cur
			}
		}
	}

	showProvider := false
	showNotes := false
	for _, f := range result.Flights {
		if f.Provider != "" && !strings.EqualFold(f.Provider, "google_flights") {
			showProvider = true
		}
		if flightWarnings(f) != "" {
			showNotes = true
		}
	}

	headers := []string{"Price", "Duration", "Stops", "Route"}
	if showProvider {
		headers = append(headers, "Provider")
	}
	headers = append(headers, "Airline", "Flight", "Departs", "Arrives")
	if showNotes {
		headers = append(headers, "Notes")
	}
	var rows [][]string
	var prices priceScale

	for _, f := range result.Flights {
		prices = prices.With(f.Price)
	}

	for _, f := range result.Flights {
		route := flightRoute(f)
		airline := ""
		flightNum := ""
		departs := ""
		arrives := ""

		if len(f.Legs) > 0 {
			airline = f.Legs[0].Airline
			flightNum = f.Legs[0].FlightNumber
			departs = f.Legs[0].DepartureTime
			arrives = f.Legs[len(f.Legs)-1].ArrivalTime
		}

		row := []string{
			prices.Apply(f.Price, formatPrice(f.Price, f.Currency)),
			formatDuration(f.Duration),
			colorizeStops(f.Stops),
			route,
		}
		if showProvider {
			row = append(row, flightProviderLabel(f))
		}
		row = append(row, airline, flightNum, departs, arrives)
		if showNotes {
			row = append(row, flightWarnings(f))
		}
		rows = append(rows, row)
	}

	models.FormatTable(os.Stdout, headers, rows)

	// Summary: cheapest flight
	if len(result.Flights) > 0 {
		cheapest := result.Flights[0]
		for _, f := range result.Flights[1:] {
			if f.Price > 0 && f.Price < cheapest.Price {
				cheapest = f
			}
		}
		airline := ""
		if len(cheapest.Legs) > 0 {
			airline = cheapest.Legs[0].Airline
		}
		descriptorParts := []string{}
		if provider := flightProviderLabel(cheapest); provider != "" && (!strings.EqualFold(cheapest.Provider, "google_flights") || airline == "") {
			descriptorParts = append(descriptorParts, provider)
		}
		if airline != "" {
			descriptorParts = append(descriptorParts, airline)
		}
		if cheapest.SelfConnect {
			descriptorParts = append(descriptorParts, "self-connect")
		}
		descriptor := strings.Join(descriptorParts, ", ")
		if descriptor == "" {
			descriptor = "-"
		}
		models.Summary(os.Stdout, fmt.Sprintf("Cheapest: %s %.0f (%s, %s)",
			cheapest.Currency, cheapest.Price, descriptor, formatStops(cheapest.Stops)))
		models.BookingHint(os.Stdout)
	}
	return nil
}

func flightProviderLabel(f models.FlightResult) string {
	switch strings.ToLower(strings.TrimSpace(f.Provider)) {
	case "":
		return ""
	case "google_flights":
		return "Google"
	case "kiwi":
		return "Kiwi"
	default:
		return f.Provider
	}
}

func flightWarnings(f models.FlightResult) string {
	if len(f.Warnings) > 0 {
		return strings.Join(f.Warnings, "; ")
	}
	if f.SelfConnect {
		return "Self-connect: protect your own connection"
	}
	return ""
}

// flightRoute builds a route string like "HEL -> FRA -> NRT".
func flightRoute(f models.FlightResult) string {
	if len(f.Legs) == 0 {
		return ""
	}

	parts := []string{f.Legs[0].DepartureAirport.Code}
	for _, leg := range f.Legs {
		parts = append(parts, leg.ArrivalAirport.Code)
	}
	return strings.Join(parts, " -> ")
}

// formatPrice formats a price with currency.
func formatPrice(amount float64, currency string) string {
	if amount == 0 {
		return "-"
	}
	return fmt.Sprintf("%s %.0f", currency, amount)
}

// formatDuration converts minutes to a human-readable duration string.
func formatDuration(minutes int) string {
	if minutes == 0 {
		return "-"
	}
	h := minutes / 60
	m := minutes % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// formatStops returns a human-readable stops string.
func formatStops(stops int) string {
	switch stops {
	case 0:
		return "Direct"
	case 1:
		return "1 stop"
	default:
		return fmt.Sprintf("%d stops", stops)
	}
}
