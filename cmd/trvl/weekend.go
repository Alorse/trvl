package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
	"github.com/spf13/cobra"
)

func weekendCmd() *cobra.Command {
	var (
		month          string
		budget         float64
		nights         int
		formatOut      string
		targetCurrency string
	)

	cmd := &cobra.Command{
		Use:   "weekend ORIGIN",
		Short: "Find cheap weekend getaway destinations",
		Long: `Search for affordable weekend getaway destinations from an airport.

ORIGIN is an IATA airport code (e.g. HEL, JFK, LHR).
Returns the top 10 cheapest destinations ranked by total estimated cost
(round-trip flight + estimated hotel).

Examples:
  trvl weekend HEL --month july-2026
  trvl weekend HEL --month 2026-07 --budget 500
  trvl weekend JFK --month aug-2026 --nights 3 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			origin := strings.ToUpper(args[0])

			if err := models.ValidateIATA(origin); err != nil {
				return fmt.Errorf("invalid origin: %w", err)
			}

			if month == "" {
				// Default to next month.
				month = time.Now().AddDate(0, 1, 0).Format("2006-01")
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			opts := trip.WeekendOptions{
				Month:     month,
				MaxBudget: budget,
				Nights:    nights,
			}

			result, err := trip.FindWeekendGetaways(ctx, origin, opts)
			if err != nil {
				return err
			}

			if formatOut == "json" {
				return models.FormatJSON(os.Stdout, result)
			}

			return printWeekendTable(cmd.Context(), targetCurrency, result)
		},
	}

	cmd.Flags().StringVar(&month, "month", "", "Month to search (e.g. july-2026, 2026-07); default: next month")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Maximum total budget (0 = no limit)")
	cmd.Flags().IntVar(&nights, "nights", 2, "Number of nights (default: 2)")
	cmd.Flags().StringVar(&formatOut, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&targetCurrency, "currency", "", "Convert prices to this currency (e.g. EUR, USD). Empty = show API default")

	cmd.ValidArgsFunction = airportCompletion

	return cmd
}

func printWeekendTable(ctx context.Context, targetCurrency string, result *trip.WeekendResult) error {
	if !result.Success {
		fmt.Fprintf(os.Stderr, "Weekend search failed: %s\n", result.Error)
		return nil
	}

	if result.Count == 0 {
		fmt.Println("No destinations found within budget.")
		return nil
	}

	// Convert prices if --currency specified and differs from data currency.
	if targetCurrency != "" {
		for i := range result.Destinations {
			d := &result.Destinations[i]
			if d.Currency != targetCurrency {
				if d.FlightPrice > 0 {
					converted, _ := destinations.ConvertCurrency(ctx, d.FlightPrice, d.Currency, targetCurrency)
					d.FlightPrice = math.Round(converted)
				}
				if d.HotelEstimate > 0 {
					converted, _ := destinations.ConvertCurrency(ctx, d.HotelEstimate, d.Currency, targetCurrency)
					d.HotelEstimate = math.Round(converted)
				}
				if d.TotalEstimate > 0 {
					converted, _ := destinations.ConvertCurrency(ctx, d.TotalEstimate, d.Currency, targetCurrency)
					d.TotalEstimate = math.Round(converted)
				}
				d.Currency = targetCurrency
			}
		}
	}

	fmt.Printf("Weekend getaways from %s in %s (%d nights)\n\n", result.Origin, result.Month, result.Nights)

	headers := []string{"Destination", "Airport", "Flight", "Hotel Est.", "Total", "Stops"}
	var rows [][]string

	for _, d := range result.Destinations {
		rows = append(rows, []string{
			d.Destination,
			d.AirportCode,
			fmt.Sprintf("%s %.0f", d.Currency, d.FlightPrice),
			fmt.Sprintf("%s %.0f", d.Currency, d.HotelEstimate),
			fmt.Sprintf("%s %.0f", d.Currency, d.TotalEstimate),
			formatStops(d.Stops),
		})
	}

	models.FormatTable(os.Stdout, headers, rows)
	return nil
}
