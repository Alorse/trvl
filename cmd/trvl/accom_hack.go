package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MikkoParkkola/trvl/internal/hacks"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

func accomHackCmd() *cobra.Command {
	var (
		checkIn   string
		checkOut  string
		currency  string
		maxSplits int
		guests    int
	)

	cmd := &cobra.Command{
		Use:   "hacks-accom CITY",
		Short: "Find cheaper hotel stays by splitting across multiple properties",
		Long: `Detect accommodation split opportunities: staying in 2-3 different hotels
can sometimes be significantly cheaper than one continuous booking.

This works because some hotels have cheaper rates for shorter stays, others
have minimum-stay requirements that inflate per-night cost, and weekend vs
weekday pricing varies across properties.

Moving cost (EUR 15 per hotel change) is deducted from reported savings.
Only splits saving at least EUR 50 and 15% vs baseline are reported.

Examples:
  trvl hacks-accom Prague --checkin 2026-04-12 --checkout 2026-04-19
  trvl hacks-accom Amsterdam --checkin 2026-06-01 --checkout 2026-06-08 --currency EUR
  trvl hacks-accom "New York" --checkin 2026-07-10 --checkout 2026-07-17 --max-splits 2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			city := args[0]

			if checkIn == "" || checkOut == "" {
				return fmt.Errorf("--checkin and --checkout are required")
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()

			in := hacks.AccommodationSplitInput{
				City:      city,
				CheckIn:   checkIn,
				CheckOut:  checkOut,
				Currency:  currency,
				MaxSplits: maxSplits,
				Guests:    guests,
			}

			detected := hacks.DetectAccommodationSplit(ctx, in)

			if format == "json" {
				return models.FormatJSON(os.Stdout, map[string]interface{}{
					"city":     city,
					"checkin":  checkIn,
					"checkout": checkOut,
					"count":    len(detected),
					"hacks":    detected,
				})
			}

			return printAccomHacks(city, checkIn, checkOut, detected)
		},
	}

	cmd.Flags().StringVar(&checkIn, "checkin", "", "Check-in date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&checkOut, "checkout", "", "Check-out date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "Display currency")
	cmd.Flags().IntVar(&maxSplits, "max-splits", 3, "Maximum number of properties to split across (2 or 3)")
	cmd.Flags().IntVar(&guests, "guests", 2, "Number of guests")

	_ = cmd.MarkFlagRequired("checkin")
	_ = cmd.MarkFlagRequired("checkout")

	return cmd
}

func printAccomHacks(city, checkIn, checkOut string, detected []hacks.Hack) error {
	header := fmt.Sprintf("Accommodation Hacks · %s · %s to %s", city, checkIn, checkOut)
	models.Banner(os.Stdout, "🏨", header,
		fmt.Sprintf("Found %d split opportunity/ies", len(detected)),
	)
	fmt.Println()

	if len(detected) == 0 {
		fmt.Println("No accommodation split detected for this stay.")
		fmt.Println("Try a longer stay (≥4 nights) or a different city.")
		return nil
	}

	for i, h := range detected {
		printHack(i+1, h)
	}
	return nil
}
