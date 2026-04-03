package main

import (
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

var format string
var noCache bool

var rootCmd = &cobra.Command{
	Use:   "trvl",
	Short: "Google Flights + Hotels from your terminal. Free, no API keys.",
	Long: `trvl — real-time flight and hotel search powered by Google's internal APIs.

No API keys. No monthly fees. No scraping. Just fast, free travel data.

  trvl flights JFK LHR 2026-07-01 --cabin business --stops nonstop
  trvl hotels "Tokyo" --checkin 2026-06-15 --checkout 2026-06-18 --stars 4
  trvl dates HEL BCN --from 2026-07-01 --to 2026-08-31 --round-trip
  trvl explore HEL --format json
  trvl mcp                  # MCP server for AI agents`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&format, "format", "table", "output format (table, json)")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "bypass response cache")

	rootCmd.AddCommand(flightsCmd())
	rootCmd.AddCommand(datesCmd())
	rootCmd.AddCommand(hotelsCmd())
	rootCmd.AddCommand(pricesCmd())
	rootCmd.AddCommand(reviewsCmd)
	rootCmd.AddCommand(exploreCmd())
	rootCmd.AddCommand(gridCmd())
	rootCmd.AddCommand(destinationCmd())
	rootCmd.AddCommand(tripCostCmd())
	rootCmd.AddCommand(weekendCmd())
	rootCmd.AddCommand(suggestCmd())
	rootCmd.AddCommand(multiCityCmd())
	rootCmd.AddCommand(guideCmd())
	rootCmd.AddCommand(nearbyCmd())
	rootCmd.AddCommand(eventsCmd())
	rootCmd.AddCommand(restaurantsCmd)
	rootCmd.AddCommand(groundCmd())
	rootCmd.AddCommand(watchCmd())
	rootCmd.AddCommand(mcpCmd())
}

// airportCompletion provides IATA code completion for cobra commands.
func airportCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	toComplete = strings.ToUpper(toComplete)
	var suggestions []string
	for code, name := range models.AirportNames {
		if strings.HasPrefix(code, toComplete) || strings.Contains(strings.ToUpper(name), toComplete) {
			suggestions = append(suggestions, code+"\t"+name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}
