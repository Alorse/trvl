package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/MikkoParkkola/trvl/internal/destinations"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

func nearbyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nearby <lat> <lon>",
		Short: "Find nearby points of interest",
		Long: `Find restaurants, cafes, attractions, pharmacies, and more near a location
using OpenStreetMap. Optionally enriched with Foursquare ratings when
FOURSQUARE_API_KEY is set.

Examples:
  trvl nearby 41.38 2.17 --category restaurant
  trvl nearby 41.38 2.17 --category all --radius 1000
  trvl nearby 35.68 139.76 --format json`,
		Args: cobra.ExactArgs(2),
		RunE: runNearby,
	}

	cmd.Flags().String("category", "all", "POI category (restaurant, cafe, bar, pharmacy, atm, museum, attraction, all)")
	cmd.Flags().Int("radius", 500, "Search radius in meters (max 5000)")

	return cmd
}

func runNearby(cmd *cobra.Command, args []string) error {
	lat, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return fmt.Errorf("invalid latitude: %w", err)
	}
	lon, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return fmt.Errorf("invalid longitude: %w", err)
	}

	category, _ := cmd.Flags().GetString("category")
	radius, _ := cmd.Flags().GetInt("radius")
	format, _ := cmd.Flags().GetString("format")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := destinations.GetNearbyPlaces(ctx, lat, lon, radius, category)
	if err != nil {
		return fmt.Errorf("nearby places: %w", err)
	}

	if format == "json" {
		return models.FormatJSON(os.Stdout, result)
	}

	return formatNearbyCard(result)
}

func formatNearbyCard(result *destinations.NearbyResult) error {
	if len(result.POIs) == 0 && len(result.Attractions) == 0 {
		fmt.Println("\n  No nearby places found.")
		return nil
	}

	if len(result.POIs) > 0 {
		fmt.Printf("\n  NEARBY PLACES (%d found)\n", len(result.POIs))
		headers := []string{"Name", "Type", "Distance", "Cuisine", "Hours"}
		rows := make([][]string, 0, len(result.POIs))
		for _, p := range result.POIs {
			rows = append(rows, []string{
				truncate(p.Name, 30),
				p.Type,
				fmt.Sprintf("%dm", p.Distance),
				p.Cuisine,
				truncate(p.Hours, 20),
			})
		}
		fmt.Print("  ")
		models.FormatTable(os.Stdout, headers, rows)
		fmt.Println()
	}

	if len(result.RatedPlaces) > 0 {
		fmt.Printf("  TOP RATED (%d found)\n", len(result.RatedPlaces))
		headers := []string{"Name", "Rating", "Category", "Price", "Distance"}
		rows := make([][]string, 0, len(result.RatedPlaces))
		for _, p := range result.RatedPlaces {
			priceStr := ""
			for i := 0; i < p.PriceLevel; i++ {
				priceStr += "$"
			}
			rows = append(rows, []string{
				truncate(p.Name, 30),
				fmt.Sprintf("%.1f/10", p.Rating),
				p.Category,
				priceStr,
				fmt.Sprintf("%dm", p.Distance),
			})
		}
		fmt.Print("  ")
		models.FormatTable(os.Stdout, headers, rows)
		fmt.Println()
	}

	if len(result.Attractions) > 0 {
		fmt.Printf("  ATTRACTIONS (%d found)\n", len(result.Attractions))
		headers := []string{"Name", "Type", "Distance"}
		rows := make([][]string, 0, len(result.Attractions))
		for _, a := range result.Attractions {
			rows = append(rows, []string{
				truncate(a.Name, 40),
				a.Kind,
				fmt.Sprintf("%dm", a.Distance),
			})
		}
		fmt.Print("  ")
		models.FormatTable(os.Stdout, headers, rows)
		fmt.Println()
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
