package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/MikkoParkkola/trvl/internal/trip"
	"github.com/spf13/cobra"
)

func discoverCmd() *cobra.Command {
	var (
		origin    string
		from      string
		until     string
		budget    float64
		minNights int
		maxNights int
		top       int
		formatOut string
	)

	cmd := &cobra.Command{
		Use:   "discover --from DATE --until DATE --budget NNN",
		Short: "Inverted search: best trips that fit your budget and calendar",
		Long: `Discover finds the best-value trips within a budget and a flexible date
window, applying your preferences automatically.

You tell it:
  - how much you want to spend (total, including hotel)
  - when you are free (date window + nights range)

It tells you the highest-quality trips it could find, ranked by value score
(hotel rating × remaining budget slack). No need to pick a destination first.

Examples:
  trvl discover --from 2026-07-01 --until 2026-07-31 --budget 500
  trvl discover --from 2026-08-01 --until 2026-09-30 --budget 800 --min-nights 3 --max-nights 5
  trvl discover --from 2026-07-01 --until 2026-07-31 --budget 600 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if origin == "" {
				if p, err := preferences.Load(); err == nil && p != nil && len(p.HomeAirports) > 0 {
					origin = p.HomeAirports[0]
				}
			}
			if origin == "" {
				return fmt.Errorf("--origin is required (or set home_airports in ~/.trvl/preferences.json)")
			}
			if err := models.ValidateIATA(origin); err != nil {
				return fmt.Errorf("invalid origin: %w", err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 240*time.Second)
			defer cancel()

			opts := trip.DiscoverOptions{
				Origin:    origin,
				From:      from,
				Until:     until,
				Budget:    budget,
				MinNights: minNights,
				MaxNights: maxNights,
				Top:       top,
			}

			out, err := trip.Discover(ctx, opts)
			if err != nil {
				return err
			}

			if formatOut == "json" {
				return models.FormatJSON(os.Stdout, out)
			}

			return printDiscoverTable(out)
		},
	}

	cmd.Flags().StringVar(&origin, "origin", "", "Origin IATA code (default: first home_airport)")
	cmd.Flags().StringVar(&from, "from", "", "Earliest departure date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&until, "until", "", "Latest return date YYYY-MM-DD (required)")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Maximum total cost in EUR (required)")
	cmd.Flags().IntVar(&minNights, "min-nights", 2, "Minimum trip length in nights")
	cmd.Flags().IntVar(&maxNights, "max-nights", 4, "Maximum trip length in nights")
	cmd.Flags().IntVar(&top, "top", 5, "Number of results to show")
	cmd.Flags().StringVar(&formatOut, "format", "table", "Output format: table, json")

	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("until")
	_ = cmd.MarkFlagRequired("budget")

	return cmd
}

func printDiscoverTable(out *trip.DiscoverOutput) error {
	if out.Count == 0 {
		fmt.Printf("No trips found from %s within budget %s %.0f between %s and %s.\n",
			out.Origin, "EUR", out.Budget, out.From, out.Until)
		return nil
	}

	fmt.Printf("Top %d value trips from %s within %.0f %s (%s to %s)\n\n",
		out.Count, out.Origin, out.Budget, "EUR", out.From, out.Until)

	headers := []string{"#", "Destination", "Dates", "Nights", "Flight", "Hotel", "Total", "Slack", "Notes"}
	var rows [][]string

	for i, t := range out.Trips {
		hotelCol := fmt.Sprintf("%s %.0f", t.Currency, t.HotelPrice)
		if t.HotelName != "" {
			hotelCol += " (" + truncateName(t.HotelName, 25) + ")"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("%s (%s)", t.Destination, t.AirportCode),
			fmt.Sprintf("%s -> %s", t.DepartDate, t.ReturnDate),
			fmt.Sprintf("%d", t.Nights),
			fmt.Sprintf("%s %.0f", t.Currency, t.FlightPrice),
			hotelCol,
			fmt.Sprintf("%s %.0f", t.Currency, t.Total),
			fmt.Sprintf("%s %.0f", t.Currency, t.BudgetSlack),
			t.Reasoning,
		})
	}

	models.FormatTable(os.Stdout, headers, rows)
	return nil
}
