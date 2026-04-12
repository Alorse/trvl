package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/points"
	"github.com/spf13/cobra"
)

func pointsValueCmd() *cobra.Command {
	var (
		cashPrice      float64
		pointsRequired int
		program        string
		listPrograms   bool
		format         string
	)

	cmd := &cobra.Command{
		Use:   "points-value",
		Short: "Calculate whether using points or paying cash is better",
		Long: `Compare the value of redeeming loyalty points against paying cash.

Computes the effective cents-per-point (cpp) for a specific redemption and
compares it against the published floor and ceiling values for the program.

Examples:
  trvl points-value --cash 450 --points 60000 --program finnair-plus
  trvl points-value --cash 1200 --points 50000 --program ana-mileage-club
  trvl points-value --cash 300 --points 30000 --program world-of-hyatt --format json
  trvl points-value --list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listPrograms {
				return printProgramList(format)
			}

			if program == "" {
				return fmt.Errorf("--program is required (use --list to see options)")
			}
			if cashPrice <= 0 {
				return fmt.Errorf("--cash must be greater than 0")
			}
			if pointsRequired <= 0 {
				return fmt.Errorf("--points must be greater than 0")
			}

			rec, err := points.CalculateValue(cashPrice, pointsRequired, program)
			if err != nil {
				return err
			}

			if format == "json" {
				return models.FormatJSON(os.Stdout, rec)
			}

			printRecommendation(rec)
			return nil
		},
	}

	cmd.Flags().Float64Var(&cashPrice, "cash", 0, "Cash price of the flight/hotel (e.g. 450.00)")
	cmd.Flags().IntVar(&pointsRequired, "points", 0, "Points required for the redemption (e.g. 60000)")
	cmd.Flags().StringVar(&program, "program", "", "Loyalty program slug (e.g. finnair-plus)")
	cmd.Flags().BoolVar(&listPrograms, "list", false, "List all supported programs and their valuations")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json")

	return cmd
}

// printRecommendation renders a single recommendation as a pretty table.
func printRecommendation(r *points.Recommendation) {
	verdictColored := colorizeVerdict(r.Verdict)

	models.Banner(os.Stdout, "✦", "Points vs Cash",
		fmt.Sprintf("Program: %s", r.ProgramName),
	)
	fmt.Println()

	headers := []string{"Metric", "Value"}
	rows := [][]string{
		{"Cash price", fmt.Sprintf("$%.2f", r.CashPrice)},
		{"Points required", fmt.Sprintf("%s", formatPoints(r.PointsRequired))},
		{"Effective CPP", fmt.Sprintf("%.2f¢/pt", r.CPP)},
		{"Floor CPP", fmt.Sprintf("%.2f¢/pt", r.FloorCPP)},
		{"Ceiling CPP (sweet spot)", fmt.Sprintf("%.2f¢/pt", r.CeilingCPP)},
		{"Verdict", verdictColored},
	}
	models.FormatTable(os.Stdout, headers, rows)

	fmt.Println()
	models.Summary(os.Stdout, r.Explanation)
}

// printProgramList prints all supported programs grouped by category.
func printProgramList(format string) error {
	if format == "json" {
		return models.FormatJSON(os.Stdout, points.Programs)
	}

	// Group by category.
	grouped := map[string][]points.Program{}
	for _, p := range points.Programs {
		grouped[p.Category] = append(grouped[p.Category], p)
	}

	categories := []string{"airline", "hotel", "transferable"}
	categoryLabels := map[string]string{
		"airline":      "Airline Programs",
		"hotel":        "Hotel Programs",
		"transferable": "Transferable Currencies",
	}

	for _, cat := range categories {
		progs, ok := grouped[cat]
		if !ok || len(progs) == 0 {
			continue
		}
		sort.Slice(progs, func(i, j int) bool { return progs[i].Slug < progs[j].Slug })

		label := categoryLabels[cat]
		models.Banner(os.Stdout, "✦", label)
		fmt.Println()

		headers := []string{"Slug", "Name", "Floor CPP", "Ceiling CPP"}
		var rows [][]string
		for _, p := range progs {
			rows = append(rows, []string{
				p.Slug,
				p.Name,
				fmt.Sprintf("%.2f¢", p.FloorCPP),
				fmt.Sprintf("%.2f¢", p.CeilingCPP),
			})
		}
		models.FormatTable(os.Stdout, headers, rows)
		fmt.Println()
	}

	fmt.Printf("Use --program <slug> to calculate a specific redemption.\n")
	return nil
}

// formatPoints adds thousands separators to a point count.
func formatPoints(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	b.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// colorizeVerdict applies terminal color to the verdict string.
func colorizeVerdict(verdict string) string {
	switch verdict {
	case "use points":
		return models.Green(verdict)
	case "pay cash":
		return models.Red(verdict)
	default:
		return models.Yellow(verdict)
	}
}
