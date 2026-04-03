package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

func hotelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hotels <location>",
		Short: "Search hotels by location",
		Long: `Search Google Hotels for a given location (city, address, or landmark).

Examples:
  trvl hotels "Helsinki" --checkin 2026-06-15 --checkout 2026-06-18
  trvl hotels "Tokyo" --checkin 2026-06-15 --checkout 2026-06-18 --guests 2 --stars 4
  trvl hotels "Paris" --checkin 2026-06-15 --checkout 2026-06-18 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: runHotels,
	}

	cmd.Flags().String("checkin", "", "Check-in date (YYYY-MM-DD, required)")
	cmd.Flags().String("checkout", "", "Check-out date (YYYY-MM-DD, required)")
	cmd.Flags().Int("guests", 2, "Number of guests")
	cmd.Flags().Int("stars", 0, "Minimum star rating (0=any, 2-5)")
	cmd.Flags().String("sort", "cheapest", "Sort by: cheapest, rating, distance, stars")
	cmd.Flags().String("currency", "USD", "Currency code (e.g. USD, EUR)")
	cmd.Flags().Float64("min-price", 0, "Minimum price per night")
	cmd.Flags().Float64("max-price", 0, "Maximum price per night")
	cmd.Flags().Float64("min-rating", 0, "Minimum guest rating (e.g. 4.0)")
	cmd.Flags().Float64("max-distance", 0, "Maximum distance from city center in km")

	_ = cmd.MarkFlagRequired("checkin")
	_ = cmd.MarkFlagRequired("checkout")

	return cmd
}

func runHotels(cmd *cobra.Command, args []string) error {
	location := args[0]

	checkin, _ := cmd.Flags().GetString("checkin")
	checkout, _ := cmd.Flags().GetString("checkout")
	guests, _ := cmd.Flags().GetInt("guests")
	stars, _ := cmd.Flags().GetInt("stars")
	sortBy, _ := cmd.Flags().GetString("sort")
	currency, _ := cmd.Flags().GetString("currency")
	format, _ := cmd.Flags().GetString("format")
	minPrice, _ := cmd.Flags().GetFloat64("min-price")
	maxPrice, _ := cmd.Flags().GetFloat64("max-price")
	minRating, _ := cmd.Flags().GetFloat64("min-rating")
	maxDistance, _ := cmd.Flags().GetFloat64("max-distance")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := hotels.HotelSearchOptions{
		CheckIn:       checkin,
		CheckOut:      checkout,
		Guests:        guests,
		Stars:         stars,
		Sort:          sortBy,
		Currency:      currency,
		MinPrice:      minPrice,
		MaxPrice:      maxPrice,
		MinRating:     minRating,
		MaxDistanceKm: maxDistance,
	}

	result, err := hotels.SearchHotels(ctx, location, opts)
	if err != nil {
		return fmt.Errorf("hotel search: %w", err)
	}

	if format == "json" {
		return models.FormatJSON(os.Stdout, result)
	}

	return formatHotelsTable(result)
}

func formatHotelsTable(result *models.HotelSearchResult) error {
	if len(result.Hotels) == 0 {
		fmt.Println("No hotels found.")
		return nil
	}

	fmt.Printf("Found %d hotels:\n\n", result.Count)

	headers := []string{"Name", "Stars", "Rating", "Reviews", "Price", "Amenities"}
	rows := make([][]string, 0, len(result.Hotels))
	for _, h := range result.Hotels {
		starsStr := ""
		if h.Stars > 0 {
			starsStr = fmt.Sprintf("%d", h.Stars)
		}
		ratingStr := ""
		if h.Rating > 0 {
			ratingStr = fmt.Sprintf("%.1f", h.Rating)
		}
		reviewsStr := ""
		if h.ReviewCount > 0 {
			reviewsStr = fmt.Sprintf("%d", h.ReviewCount)
		}
		priceStr := ""
		if h.Price > 0 {
			priceStr = fmt.Sprintf("%.0f %s", h.Price, h.Currency)
		}
		amenStr := strings.Join(h.Amenities, ", ")
		if len(amenStr) > 40 {
			amenStr = amenStr[:37] + "..."
		}
		rows = append(rows, []string{h.Name, starsStr, ratingStr, reviewsStr, priceStr, amenStr})
	}

	models.FormatTable(os.Stdout, headers, rows)
	return nil
}
