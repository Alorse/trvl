package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trips"
	"github.com/MikkoParkkola/trvl/internal/weather"
	"github.com/spf13/cobra"
)

func tripsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trips",
		Short: "Plan, save, and track your trips",
		Long: `Manage planned and booked trips with legs, bookings, and status tracking.

Examples:
  trvl trips create "April Prague trip"
  trvl trips list
  trvl trips show trip_abc123
  trvl trips add-leg trip_abc123 flight --from HEL --to AMS --provider KLM --start "2026-04-11T18:25" --end "2026-04-11T20:00" --price 269 --currency EUR
  trvl trips book trip_abc123 --provider KLM --ref XYZ789
  trvl trips status
  trvl trips alerts`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTripsList(false)
		},
	}

	cmd.AddCommand(tripsListCmd())
	cmd.AddCommand(tripsShowCmd())
	cmd.AddCommand(tripsCreateCmd())
	cmd.AddCommand(tripsAddLegCmd())
	cmd.AddCommand(tripsBookCmd())
	cmd.AddCommand(tripsDeleteCmd())
	cmd.AddCommand(tripsStatusCmd())
	cmd.AddCommand(tripsAlertsCmd())
	return cmd
}

// tripsListCmd — trvl trips list [--all]
func tripsListCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List trips (active only by default)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTripsList(all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Show all trips including completed and cancelled")
	return cmd
}

func runTripsList(all bool) error {
	store, err := loadTripStore()
	if err != nil {
		return err
	}

	var list []trips.Trip
	if all {
		list = store.List()
	} else {
		list = store.Active()
	}

	if len(list) == 0 {
		if all {
			fmt.Println("No trips found. Create one with: trvl trips create \"My Trip\"")
		} else {
			fmt.Println("No active trips. Use --all to see completed trips.")
		}
		return nil
	}

	if format == "json" {
		return printJSON(list)
	}

	headers := []string{"ID", "Name", "Status", "Legs", "Next Leg", "Tags"}
	var rows [][]string
	for _, t := range list {
		nextLeg := nextLegSummary(t)
		tags := strings.Join(t.Tags, ", ")
		rows = append(rows, []string{
			t.ID,
			t.Name,
			colorizeStatus(t.Status),
			fmt.Sprintf("%d", len(t.Legs)),
			nextLeg,
			tags,
		})
	}
	models.FormatTable(os.Stdout, headers, rows)
	return nil
}

// tripsShowCmd — trvl trips show <id>
func tripsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show detailed view of a trip",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadTripStore()
			if err != nil {
				return err
			}
			trip, err := store.Get(args[0])
			if err != nil {
				return err
			}

			if format == "json" {
				return printJSON(trip)
			}

			printTripDetail(trip)

			// Auto-fetch weather for each unique destination+date combination.
			// Runs best-effort: errors are silently swallowed (offline is fine).
			printTripWeather(cmd.Context(), trip)

			return nil
		},
	}
}

// tripsCreateCmd — trvl trips create <name>
func tripsCreateCmd() *cobra.Command {
	var (
		tags   []string
		notes  string
		status string
	)
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new trip",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadTripStore()
			if err != nil {
				return err
			}
			t := trips.Trip{
				Name:   args[0],
				Tags:   tags,
				Notes:  notes,
				Status: status,
			}
			id, err := store.Add(t)
			if err != nil {
				return err
			}
			fmt.Printf("Created trip: %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Comma-separated tags (e.g. work,family)")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form notes")
	cmd.Flags().StringVar(&status, "status", "planning", "Initial status (planning, booked)")
	return cmd
}

// tripsAddLegCmd — trvl trips add-leg <id> <type>
func tripsAddLegCmd() *cobra.Command {
	var (
		from       string
		to         string
		provider   string
		startTime  string
		endTime    string
		price      float64
		currency   string
		bookingURL string
		confirmed  bool
		reference  string
	)

	cmd := &cobra.Command{
		Use:   "add-leg <id> <type>",
		Short: "Add a leg to a trip",
		Long: `Add a travel segment (flight, train, hotel, etc.) to an existing trip.

Types: flight, train, bus, ferry, hotel, activity

Examples:
  trvl trips add-leg trip_abc flight --from HEL --to AMS --provider KLM --price 269 --currency EUR --start "2026-04-11T18:25" --end "2026-04-11T20:00"
  trvl trips add-leg trip_abc hotel --from Amsterdam --to Amsterdam --provider "Hotel V" --start "2026-04-11" --end "2026-04-13"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			legType := args[1]

			store, err := loadTripStore()
			if err != nil {
				return err
			}

			leg := trips.TripLeg{
				Type:       legType,
				From:       from,
				To:         to,
				Provider:   provider,
				StartTime:  startTime,
				EndTime:    endTime,
				Price:      price,
				Currency:   currency,
				BookingURL: bookingURL,
				Confirmed:  confirmed,
				Reference:  reference,
			}

			err = store.Update(id, func(t *trips.Trip) error {
				t.Legs = append(t.Legs, leg)
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Printf("Leg added to trip %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Origin city or location (required)")
	cmd.Flags().StringVar(&to, "to", "", "Destination city or location (required)")
	cmd.Flags().StringVar(&provider, "provider", "", "Carrier or hotel name")
	cmd.Flags().StringVar(&startTime, "start", "", "Departure/check-in datetime (ISO format, e.g. 2026-04-11T18:25)")
	cmd.Flags().StringVar(&endTime, "end", "", "Arrival/check-out datetime (ISO format)")
	cmd.Flags().Float64Var(&price, "price", 0, "Price amount")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency code (e.g. EUR, GBP)")
	cmd.Flags().StringVar(&bookingURL, "url", "", "Booking URL")
	cmd.Flags().BoolVar(&confirmed, "confirmed", false, "Mark leg as confirmed/booked")
	cmd.Flags().StringVar(&reference, "ref", "", "Booking reference / PNR")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")

	return cmd
}

// tripsBookCmd — trvl trips book <id>
func tripsBookCmd() *cobra.Command {
	var (
		provider  string
		reference string
		bookType  string
		url       string
		notes     string
	)

	cmd := &cobra.Command{
		Use:   "book <id>",
		Short: "Add a booking reference to a trip",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			store, err := loadTripStore()
			if err != nil {
				return err
			}

			booking := trips.Booking{
				Type:      bookType,
				Provider:  provider,
				Reference: reference,
				URL:       url,
				Notes:     notes,
			}

			return store.Update(id, func(t *trips.Trip) error {
				t.Bookings = append(t.Bookings, booking)
				if t.Status == "planning" {
					t.Status = "booked"
				}
				fmt.Printf("Booking %s/%s added to trip %s (status: %s)\n", provider, reference, id, t.Status)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Carrier or hotel name (required)")
	cmd.Flags().StringVar(&reference, "ref", "", "Booking reference / PNR (required)")
	cmd.Flags().StringVar(&bookType, "type", "flight", "Booking type (flight, hotel, other)")
	cmd.Flags().StringVar(&url, "url", "", "Confirmation URL")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes")
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("ref")
	return cmd
}

// tripsDeleteCmd — trvl trips delete <id>
func tripsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a trip",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadTripStore()
			if err != nil {
				return err
			}
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("Trip %s deleted\n", args[0])
			return nil
		},
	}
}

// tripsStatusCmd — trvl trips status
func tripsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show upcoming trips and next departure",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadTripStore()
			if err != nil {
				return err
			}

			upcoming := store.Upcoming(30 * 24 * time.Hour) // next 30 days
			if len(upcoming) == 0 {
				fmt.Println("No upcoming trips in the next 30 days.")
				fmt.Println("Create one with: trvl trips create \"My Trip\"")
				return nil
			}

			if format == "json" {
				return printJSON(upcoming)
			}

			now := time.Now()
			for _, t := range upcoming {
				first := trips.FirstLegStart(t)
				var countdown string
				if !first.IsZero() {
					d := first.Sub(now)
					countdown = formatCountdown(d)
				}

				fmt.Printf("Trip: %s", models.Bold(t.Name))
				if countdown != "" {
					fmt.Printf(" — departs %s", countdown)
				}
				fmt.Println()

				for _, leg := range t.Legs {
					printLegLine(leg)
				}
				fmt.Println()
			}
			return nil
		},
	}
}

// tripsAlertsCmd — trvl trips alerts
func tripsAlertsCmd() *cobra.Command {
	var markRead bool
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Show trip alerts and reminders",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadTripStore()
			if err != nil {
				return err
			}

			if markRead {
				return store.MarkAlertsRead()
			}

			alerts := store.Alerts(false)
			if len(alerts) == 0 {
				fmt.Println("No alerts.")
				return nil
			}

			if format == "json" {
				return printJSON(alerts)
			}

			for _, a := range alerts {
				readMark := " "
				if !a.Read {
					readMark = "*"
				}
				fmt.Printf("[%s] %s  %s\n  %s\n",
					readMark,
					a.CreatedAt.Format("Jan 02 15:04"),
					models.Bold(a.TripName),
					a.Message,
				)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&markRead, "mark-read", false, "Mark all alerts as read")
	return cmd
}

// --- Helpers ---

func loadTripStore() (*trips.Store, error) {
	store, err := trips.DefaultStore()
	if err != nil {
		return nil, err
	}
	if err := store.Load(); err != nil {
		return nil, err
	}
	return store, nil
}

func printTripDetail(t *trips.Trip) {
	models.Banner(os.Stdout, "", t.Name,
		fmt.Sprintf("Status: %s", t.Status),
		fmt.Sprintf("ID: %s", t.ID),
		fmt.Sprintf("Created: %s", t.CreatedAt.Format("2006-01-02")),
	)
	fmt.Println()

	if len(t.Tags) > 0 {
		fmt.Printf("  Tags: %s\n\n", strings.Join(t.Tags, ", "))
	}
	if t.Notes != "" {
		fmt.Printf("  Notes: %s\n\n", t.Notes)
	}

	if len(t.Legs) > 0 {
		fmt.Println(models.Bold("  Legs:"))
		for _, leg := range t.Legs {
			printLegLine(leg)
		}
		fmt.Println()
	}

	if len(t.Bookings) > 0 {
		fmt.Println(models.Bold("  Bookings:"))
		for _, b := range t.Bookings {
			fmt.Printf("    %s  %s  ref: %s", b.Type, b.Provider, b.Reference)
			if b.URL != "" {
				fmt.Printf("  %s", b.URL)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}

func printLegLine(leg trips.TripLeg) {
	status := "planned"
	if leg.Confirmed {
		status = models.Green("confirmed")
	}

	parts := []string{
		fmt.Sprintf("  %s", leg.Type),
		fmt.Sprintf("%s->%s", leg.From, leg.To),
	}
	if leg.Provider != "" {
		parts = append(parts, leg.Provider)
	}
	if leg.StartTime != "" {
		parts = append(parts, leg.StartTime)
	}
	if leg.Price > 0 && leg.Currency != "" {
		parts = append(parts, fmt.Sprintf("%s %.0f", leg.Currency, leg.Price))
	}
	parts = append(parts, fmt.Sprintf("(%s)", status))
	if leg.Reference != "" {
		parts = append(parts, fmt.Sprintf("ref:%s", leg.Reference))
	}
	fmt.Println(strings.Join(parts, "  "))
}

func nextLegSummary(t trips.Trip) string {
	now := time.Now()
	for _, leg := range t.Legs {
		if leg.StartTime == "" {
			continue
		}
		ts, err := time.Parse("2006-01-02T15:04", leg.StartTime)
		if err != nil {
			ts, err = time.Parse("2006-01-02", leg.StartTime)
			if err != nil {
				continue
			}
		}
		if ts.After(now) {
			label := fmt.Sprintf("%s %s->%s", leg.Type, leg.From, leg.To)
			d := ts.Sub(now)
			return fmt.Sprintf("%s (%s)", label, formatCountdown(d))
		}
	}
	return ""
}

func formatCountdown(d time.Duration) string {
	if d < 0 {
		return "departed"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	switch {
	case days >= 2:
		return fmt.Sprintf("in %d days", days)
	case days == 1:
		return fmt.Sprintf("in 1 day %dh", hours)
	case d.Hours() >= 2:
		return fmt.Sprintf("in %.0fh", d.Hours())
	default:
		return fmt.Sprintf("in %.0fm", d.Minutes())
	}
}

func colorizeStatus(s string) string {
	switch s {
	case "planning":
		return models.Yellow(s)
	case "booked", "in_progress":
		return models.Green(s)
	case "completed":
		return s
	case "cancelled":
		return models.Red(s)
	default:
		return s
	}
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printTripWeather fetches and displays weather forecasts for each leg destination
// with a known date. Best-effort: silently skips on any error.
func printTripWeather(ctx context.Context, t *trips.Trip) {
	type destDate struct {
		city     string
		fromDate string
		toDate   string
	}

	// Collect unique destinations with date ranges from trip legs.
	seen := make(map[string]bool)
	var targets []destDate

	for i, leg := range t.Legs {
		dest := leg.To
		if dest == "" || leg.StartTime == "" {
			continue
		}

		// Parse date from StartTime (accepts "2006-01-02" or "2006-01-02T15:04").
		fromDate := leg.StartTime
		if len(fromDate) > 10 {
			fromDate = fromDate[:10]
		}

		// End date: use leg.EndTime or next leg's start, or fromDate+3 as fallback.
		toDate := fromDate
		if leg.EndTime != "" {
			td := leg.EndTime
			if len(td) > 10 {
				td = td[:10]
			}
			toDate = td
		} else if i+1 < len(t.Legs) && t.Legs[i+1].StartTime != "" {
			td := t.Legs[i+1].StartTime
			if len(td) > 10 {
				td = td[:10]
			}
			toDate = td
		}

		// Limit to a reasonable window (7 days max per destination).
		if from, err := time.Parse("2006-01-02", fromDate); err == nil {
			if to, err2 := time.Parse("2006-01-02", toDate); err2 == nil {
				if to.Sub(from) > 7*24*time.Hour {
					toDate = from.AddDate(0, 0, 7).Format("2006-01-02")
				}
			}
		}

		key := dest + "|" + fromDate
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, destDate{city: dest, fromDate: fromDate, toDate: toDate})
	}

	if len(targets) == 0 {
		return
	}

	fmt.Println(models.Bold("  Weather forecast:"))

	wCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	for _, td := range targets {
		result, err := weather.GetForecast(wCtx, td.city, td.fromDate, td.toDate)
		if err != nil || result == nil || !result.Success || len(result.Forecasts) == 0 {
			continue
		}

		from := weather.FormatDateShort(td.fromDate)
		to := weather.FormatDateShort(td.toDate)
		fmt.Printf("\n  🌤️  %s · %s-%s\n", result.City, from, to)
		for _, f := range result.Forecasts {
			emoji := weather.WeatherEmoji(f.Description)
			rain := ""
			if f.Precipitation > 0 {
				rain = fmt.Sprintf("  %.0fmm", f.Precipitation)
			}
			fmt.Printf("    %s %s  %s  %d°/%d°C%s  %s\n",
				weather.FormatDateShort(f.Date),
				weather.DayOfWeek(f.Date),
				emoji,
				int(f.TempMin), int(f.TempMax),
				rain,
				f.Description,
			)
		}
	}
	fmt.Println()
}
