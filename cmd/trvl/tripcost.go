package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
	"github.com/spf13/cobra"
)

func tripCostCmd() *cobra.Command {
	var (
		departDate string
		returnDate string
		guests     int
		currency   string
		format     string
	)

	cmd := &cobra.Command{
		Use:   "trip-cost ORIGIN DESTINATION",
		Short: "Estimate total trip cost (flights + hotel)",
		Long: `Calculate the total estimated cost for a trip including outbound flight,
return flight, and hotel accommodation at the destination.

ORIGIN and DESTINATION are IATA airport codes (e.g. HEL, BCN, JFK).
Flights are priced per person; hotels are per room.

Examples:
  trvl trip-cost HEL BCN --depart 2026-07-01 --return 2026-07-08
  trvl trip-cost HEL BCN --depart 2026-07-01 --return 2026-07-08 --guests 2
  trvl trip-cost JFK LHR --depart 2026-08-01 --return 2026-08-10 --currency USD --format json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			origin := strings.ToUpper(args[0])
			dest := strings.ToUpper(args[1])

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			input := trip.TripCostInput{
				Origin:      origin,
				Destination: dest,
				DepartDate:  departDate,
				ReturnDate:  returnDate,
				Guests:      guests,
				Currency:    currency,
			}

			result, err := trip.CalculateTripCost(ctx, input)
			if err != nil {
				return err
			}

			if format == "json" {
				return models.FormatJSON(os.Stdout, result)
			}

			return printTripCostTable(result, origin, dest, guests)
		},
	}

	cmd.Flags().StringVar(&departDate, "depart", "", "Departure date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&returnDate, "return", "", "Return date (YYYY-MM-DD, required)")
	cmd.Flags().IntVar(&guests, "guests", 1, "Number of guests")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "Currency for totals (e.g. EUR, USD). Passed to search APIs")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json")

	_ = cmd.MarkFlagRequired("depart")
	_ = cmd.MarkFlagRequired("return")

	return cmd
}

func printTripCostTable(result *trip.TripCostResult, origin, dest string, guests int) error {
	if !result.Success {
		fmt.Fprintf(os.Stderr, "Trip cost estimation failed: %s\n", result.Error)
		return nil
	}

	cur := result.Currency
	fmt.Printf("Trip: %s -> %s (%d nights, %d guest(s))\n\n", origin, dest, result.Nights, guests)

	headers := []string{"Component", "Amount", "Details"}
	var rows [][]string

	rows = append(rows, []string{
		"Outbound flight",
		formatPrice(result.Flights.Outbound, result.Flights.Currency),
		fmt.Sprintf("%s -> %s, %s", origin, dest, formatStops(result.Flights.OutboundStops)),
	})
	rows = append(rows, []string{
		"Return flight",
		formatPrice(result.Flights.Return, result.Flights.Currency),
		fmt.Sprintf("%s -> %s, %s", dest, origin, formatStops(result.Flights.ReturnStops)),
	})

	hotelDetail := ""
	if result.Hotels.Name != "" {
		hotelDetail = result.Hotels.Name + ", "
	}
	hotelDetail += fmt.Sprintf("%s %.0f/night x %d nights", result.Hotels.Currency, result.Hotels.PerNight, result.Nights)
	rows = append(rows, []string{
		"Hotel",
		formatPrice(result.Hotels.Total, result.Hotels.Currency),
		hotelDetail,
	})

	rows = append(rows, []string{"", "", ""})
	rows = append(rows, []string{"Total", fmt.Sprintf("%s %.0f", cur, result.Total), ""})
	rows = append(rows, []string{"Per person", fmt.Sprintf("%s %.0f", cur, result.PerPerson), ""})
	rows = append(rows, []string{"Per day", fmt.Sprintf("%s %.0f", cur, result.PerDay), ""})

	models.FormatTable(os.Stdout, headers, rows)
	return nil
}
