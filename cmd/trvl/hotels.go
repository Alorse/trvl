package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
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
	cmd.Flags().String("currency", "", "Target currency (e.g. EUR, USD). Empty = API default. Passed to Google if supported, otherwise converted")
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

	return formatHotelsTable(cmd.Context(), currency, result)
}

func formatHotelsTable(ctx context.Context, targetCurrency string, result *models.HotelSearchResult) error {
	if len(result.Hotels) == 0 {
		fmt.Println("No hotels found.")
		return nil
	}

	models.Banner(os.Stdout, "🏨", "Hotels",
		fmt.Sprintf("Found %d hotels", result.Count))
	fmt.Println()

	// Convert prices if --currency specified and differs from API result.
	if targetCurrency != "" && len(result.Hotels) > 0 && result.Hotels[0].Currency != targetCurrency {
		for i := range result.Hotels {
			if result.Hotels[i].Price > 0 && result.Hotels[i].Currency != targetCurrency {
				converted, cur := destinations.ConvertCurrency(ctx, result.Hotels[i].Price, result.Hotels[i].Currency, targetCurrency)
				result.Hotels[i].Price = math.Round(converted)
				result.Hotels[i].Currency = cur
			}
		}
	}

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

	// Summary
	if len(result.Hotels) > 0 {
		cheapest := result.Hotels[0]
		bestRated := result.Hotels[0]
		for _, h := range result.Hotels[1:] {
			if h.Price > 0 && (cheapest.Price == 0 || h.Price < cheapest.Price) {
				cheapest = h
			}
			if h.Rating > bestRated.Rating {
				bestRated = h
			}
		}
		parts := []string{}
		if cheapest.Price > 0 {
			parts = append(parts, fmt.Sprintf("Cheapest: %.0f %s (%s)", cheapest.Price, cheapest.Currency, cheapest.Name))
		}
		if bestRated.Rating > 0 {
			parts = append(parts, fmt.Sprintf("Top rated: %.1f (%s)", bestRated.Rating, bestRated.Name))
		}
		if len(parts) > 0 {
			models.Summary(os.Stdout, strings.Join(parts, " · "))
		}
	}
	return nil
}
