package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

func pricesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prices <hotel_id>",
		Short: "Look up booking prices for a specific hotel",
		Long: `Get prices from multiple booking providers for a specific hotel.

The hotel_id is a Google place ID (e.g. "/g/11b6d4_v_4") returned by the
hotels search command.

Examples:
  trvl prices "/g/11b6d4_v_4" --checkin 2026-06-15 --checkout 2026-06-18
  trvl prices "ChIJy7MSZP0LkkYRZw2dDekQP78" --checkin 2026-06-15 --checkout 2026-06-18 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: runPrices,
	}

	cmd.Flags().String("checkin", "", "Check-in date (YYYY-MM-DD, required)")
	cmd.Flags().String("checkout", "", "Check-out date (YYYY-MM-DD, required)")
	cmd.Flags().String("currency", "USD", "Currency code (e.g. USD, EUR)")

	_ = cmd.MarkFlagRequired("checkin")
	_ = cmd.MarkFlagRequired("checkout")

	return cmd
}

func runPrices(cmd *cobra.Command, args []string) error {
	hotelID := args[0]

	checkin, _ := cmd.Flags().GetString("checkin")
	checkout, _ := cmd.Flags().GetString("checkout")
	currency, _ := cmd.Flags().GetString("currency")
	format, _ := cmd.Flags().GetString("format")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := hotels.GetHotelPrices(ctx, hotelID, checkin, checkout, currency)
	if err != nil {
		return fmt.Errorf("hotel prices: %w", err)
	}

	if format == "json" {
		return models.FormatJSON(os.Stdout, result)
	}

	return formatPricesTable(result)
}

func formatPricesTable(result *models.HotelPriceResult) error {
	if len(result.Providers) == 0 {
		fmt.Println("No prices found.")
		return nil
	}

	fmt.Printf("Prices for hotel %s (%s to %s):\n\n", result.HotelID, result.CheckIn, result.CheckOut)

	headers := []string{"Provider", "Price", "Currency"}
	rows := make([][]string, 0, len(result.Providers))
	for _, p := range result.Providers {
		rows = append(rows, []string{
			p.Provider,
			fmt.Sprintf("%.2f", p.Price),
			p.Currency,
		})
	}

	models.FormatTable(os.Stdout, headers, rows)
	return nil
}
