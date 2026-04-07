package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

func baggageCmd() *cobra.Command {
	var (
		all          bool
		carryOnOnly  bool
	)

	cmd := &cobra.Command{
		Use:   "baggage [AIRLINE_CODE]",
		Short: "Show airline baggage allowance rules",
		Long: `Look up baggage rules for airlines — carry-on limits, checked bag fees, LCC restrictions.

Examples:
  trvl baggage KL                 # KLM rules
  trvl baggage FR                 # Ryanair (LCC) rules
  trvl baggage --all              # list all airlines
  trvl baggage --all --format json
  trvl baggage --carry-on-only    # only airlines with generous carry-on`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !all && !carryOnOnly {
				return cmd.Help()
			}

			if all || carryOnOnly {
				airlines := baggage.All()
				if carryOnOnly {
					var filtered []baggage.AirlineBaggage
					for _, ab := range airlines {
						if !ab.OverheadOnly {
							filtered = append(filtered, ab)
						}
					}
					airlines = filtered
				}
				if format == "json" {
					return models.FormatJSON(os.Stdout, airlines)
				}
				return printBaggageList(airlines)
			}

			// Single airline lookup
			code := strings.ToUpper(args[0])
			ab, ok := baggage.Get(code)
			if !ok {
				return fmt.Errorf("unknown airline code %q — use --all to see available airlines", code)
			}
			if format == "json" {
				return models.FormatJSON(os.Stdout, ab)
			}
			return printBaggageDetail(ab)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "List all airlines")
	cmd.Flags().BoolVar(&carryOnOnly, "carry-on-only", false, "Only show airlines with full overhead carry-on included in base fare")
	return cmd
}

func printBaggageDetail(ab baggage.AirlineBaggage) error {
	models.Banner(os.Stdout, "✈️", fmt.Sprintf("Baggage rules · %s (%s)", ab.Name, ab.Code))
	fmt.Println()

	// Carry-on
	carryOnWeight := "No weight limit"
	if ab.CarryOnMaxKg > 0 {
		carryOnWeight = fmt.Sprintf("%.0f kg", ab.CarryOnMaxKg)
	}
	if ab.CarryOnDimensions != "" {
		carryOnWeight += ", " + ab.CarryOnDimensions
	}
	fmt.Printf("  Carry-on:    %s\n", carryOnWeight)

	// Personal item
	personal := "No"
	if ab.PersonalItem {
		personal = "Yes (handbag/laptop bag)"
	}
	fmt.Printf("  Personal:    %s\n", personal)

	// Checked
	checked := "Not included"
	if ab.CheckedIncluded > 0 {
		checked = fmt.Sprintf("%d bag included (23 kg)", ab.CheckedIncluded)
	} else if ab.CheckedFee > 0 {
		checked = fmt.Sprintf("Not included — from EUR %.0f", ab.CheckedFee)
	}
	fmt.Printf("  Checked:     %s\n", checked)

	// LCC warning
	if ab.OverheadOnly {
		fmt.Println()
		fmt.Println("  " + models.Yellow("Base fare: only small under-seat bag free."))
		fmt.Println("  " + models.Yellow("Overhead cabin bag requires priority/add-on purchase."))
	}

	// Notes
	if ab.Notes != "" {
		fmt.Println()
		fmt.Printf("  %s\n", models.Dim(ab.Notes))
	}

	fmt.Println()
	return nil
}

func printBaggageList(airlines []baggage.AirlineBaggage) error {
	models.Banner(os.Stdout, "✈️", "Airline Baggage Rules")
	fmt.Println()

	headers := []string{"Code", "Airline", "Carry-on", "Personal", "Checked", "LCC"}
	var rows [][]string
	for _, ab := range airlines {
		carryOn := "no limit"
		if ab.CarryOnMaxKg > 0 {
			carryOn = fmt.Sprintf("%.0fkg", ab.CarryOnMaxKg)
		}

		personal := "no"
		if ab.PersonalItem {
			personal = "yes"
		}

		checked := "not included"
		if ab.CheckedIncluded > 0 {
			checked = fmt.Sprintf("%dx23kg", ab.CheckedIncluded)
		} else if ab.CheckedFee > 0 {
			checked = fmt.Sprintf("~EUR%.0f", ab.CheckedFee)
		}

		lcc := ""
		if ab.OverheadOnly {
			lcc = models.Yellow("overhead fee")
		}

		rows = append(rows, []string{
			ab.Code,
			ab.Name,
			carryOn,
			personal,
			checked,
			lcc,
		})
	}
	models.FormatTable(os.Stdout, headers, rows)
	return nil
}
