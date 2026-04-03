package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/ground"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

func groundCmd() *cobra.Command {
	var (
		currency  string
		providers string
		maxPrice  float64
		typeFilter string
	)

	cmd := &cobra.Command{
		Use:     "ground FROM TO DATE",
		Aliases: []string{"bus", "train"},
		Short:   "Search bus and train connections (FlixBus, RegioJet)",
		Long: `Search ground transport (buses and trains) between two cities.

Searches FlixBus and RegioJet in parallel. No API key required.

FROM and TO are city names (e.g. "Prague", "Vienna", "Helsinki").
DATE is the departure date in YYYY-MM-DD format.

Examples:
  trvl ground Prague Vienna 2026-07-01
  trvl bus "Prague" "Krakow" 2026-07-01
  trvl train Prague Vienna 2026-07-01 --type train
  trvl ground Helsinki Tampere 2026-07-01 --provider flixbus
  trvl ground Prague Budapest 2026-07-01 --max-price 30`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			from := args[0]
			to := args[1]
			date := args[2]

			opts := ground.SearchOptions{
				Currency: currency,
				MaxPrice: maxPrice,
				Type:     typeFilter,
				NoCache:  noCache,
			}
			if providers != "" {
				opts.Providers = strings.Split(providers, ",")
			}

			result, err := ground.SearchByName(cmd.Context(), from, to, date, opts)
			if err != nil {
				return err
			}

			if format == "json" {
				return models.FormatJSON(os.Stdout, result)
			}

			return printGroundTable(cmd.Context(), currency, result)
		},
	}

	cmd.Flags().StringVar(&currency, "currency", "", "Convert prices to this currency (e.g. EUR). Empty = provider default")
	cmd.Flags().StringVar(&providers, "provider", "", "Restrict to providers (flixbus,regiojet)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price filter")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by type (bus, train)")

	return cmd
}

func printGroundTable(ctx context.Context, targetCurrency string, result *models.GroundSearchResult) error {
	if !result.Success {
		if result.Error != "" {
			fmt.Fprintf(os.Stderr, "No routes found: %s\n", result.Error)
		} else {
			fmt.Fprintln(os.Stderr, "No routes found.")
		}
		return nil
	}

	// Convert prices if --currency specified.
	if targetCurrency != "" {
		for i := range result.Routes {
			r := &result.Routes[i]
			if r.Currency != targetCurrency && r.Price > 0 {
				converted, cur := destinations.ConvertCurrency(ctx, r.Price, r.Currency, targetCurrency)
				r.Price = math.Round(converted*100) / 100
				if r.PriceMax > 0 {
					convertedMax, _ := destinations.ConvertCurrency(ctx, r.PriceMax, r.Currency, targetCurrency)
					r.PriceMax = math.Round(convertedMax*100) / 100
				}
				r.Currency = cur
			}
		}
	}

	// Count unique providers
	providers := map[string]bool{}
	for _, r := range result.Routes {
		providers[r.Provider] = true
	}
	provList := make([]string, 0, len(providers))
	for p := range providers {
		provList = append(provList, p)
	}
	models.Banner(os.Stdout, "🚂", fmt.Sprintf("Ground Transport · %d providers", len(providers)),
		fmt.Sprintf("Found %d routes (%s)", result.Count, strings.Join(provList, ", ")))
	fmt.Println()

	headers := []string{"Price", "Duration", "Type", "Provider", "Transfers", "Departs", "Arrives", "Seats"}
	var rows [][]string

	for _, r := range result.Routes {
		price := "-"
		if r.Price > 0 {
			price = fmt.Sprintf("%s %.2f", r.Currency, r.Price)
			if r.PriceMax > 0 && r.PriceMax != r.Price {
				price = fmt.Sprintf("%s %.2f-%.2f", r.Currency, r.Price, r.PriceMax)
			}
		}

		dur := formatDuration(r.Duration)
		transfers := "Direct"
		if r.Transfers > 0 {
			transfers = fmt.Sprintf("%d", r.Transfers)
		}

		depTime := formatGroundTime(r.Departure.Time)
		arrTime := formatGroundTime(r.Arrival.Time)

		seats := "-"
		if r.SeatsLeft != nil {
			seats = fmt.Sprintf("%d", *r.SeatsLeft)
			if *r.SeatsLeft <= 5 {
				seats = models.Red(seats + "!")
			}
		}

		rows = append(rows, []string{
			models.Green(price),
			dur,
			r.Type,
			r.Provider,
			transfers,
			depTime,
			arrTime,
			seats,
		})
	}

	models.FormatTable(os.Stdout, headers, rows)
	return nil
}

func formatGroundTime(isoTime string) string {
	// Extract just the time portion from ISO 8601
	if len(isoTime) >= 16 {
		return isoTime[11:16] // HH:MM
	}
	return isoTime
}
