package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/watch"
	"github.com/spf13/cobra"
)

func watchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track flight and hotel prices",
		Long: `Monitor flight and hotel prices over time and get alerts when prices drop.

Examples:
  trvl watch add HEL BCN --depart 2026-07-01 --return 2026-07-08 --below 200
  trvl watch list
  trvl watch check
  trvl watch history <id>
  trvl watch remove <id>`,
	}

	cmd.AddCommand(
		watchAddCmd(),
		watchListCmd(),
		watchRemoveCmd(),
		watchCheckCmd(),
		watchHistoryCmd(),
	)
	return cmd
}

func watchAddCmd() *cobra.Command {
	var (
		departDate string
		returnDate string
		departFrom string
		departTo   string
		belowPrice float64
		currency   string
		watchType  string
	)

	cmd := &cobra.Command{
		Use:   "add ORIGIN DESTINATION",
		Short: "Add a price watch",
		Long: `Add a new price watch for a flight or hotel route.

Three modes:
  Specific date:  --depart 2026-07-01        Check one date
  Date range:     --from 2026-07-01 --to 2026-07-31   Cheapest in range
  Route watch:    (no dates)                  Monitor next 60 days for deals

Examples:
  trvl watch add HEL BCN --depart 2026-07-01 --return 2026-07-08 --below 200
  trvl watch add HEL PRG --from 2026-07-01 --to 2026-08-31 --below 100
  trvl watch add HEL NRT --below 500
  trvl watch add --type hotel Barcelona --depart 2026-07-01 --return 2026-07-08 --below 80`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := watch.DefaultStore()
			if err != nil {
				return err
			}
			if err := store.Load(); err != nil {
				return err
			}

			w := watch.Watch{
				Type:        watchType,
				Origin:      args[0],
				Destination: args[1],
				DepartDate:  departDate,
				ReturnDate:  returnDate,
				DepartFrom:  departFrom,
				DepartTo:    departTo,
				BelowPrice:  belowPrice,
				Currency:    currency,
			}

			id, err := store.Add(w)
			if err != nil {
				return fmt.Errorf("add watch: %w", err)
			}

			mode := ""
			switch {
			case w.IsRouteWatch():
				mode = fmt.Sprintf("route watch (next 60 days)")
			case w.IsDateRange():
				mode = fmt.Sprintf("date range %s to %s", w.DepartFrom, w.DepartTo)
			default:
				mode = fmt.Sprintf("on %s", w.DepartDate)
			}

			fmt.Printf("Added %s watch %s: %s -> %s %s",
				w.Type, id, w.Origin, w.Destination, mode)
			if w.ReturnDate != "" {
				fmt.Printf(" (return %s)", w.ReturnDate)
			}
			if w.BelowPrice > 0 {
				fmt.Printf(" [alert below %.0f %s]", w.BelowPrice, w.Currency)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&departDate, "depart", "", "Specific departure date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&returnDate, "return", "", "Return/check-out date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&departFrom, "from", "", "Date range start (YYYY-MM-DD)")
	cmd.Flags().StringVar(&departTo, "to", "", "Date range end (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&belowPrice, "below", 0, "Alert when price drops below this amount")
	cmd.Flags().StringVar(&currency, "currency", "EUR", "Currency code")
	cmd.Flags().StringVar(&watchType, "type", "flight", "Watch type: flight or hotel")
	// --depart is optional: route watches monitor next 60 days without specific dates

	return cmd
}

func watchListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show all active watches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := watch.DefaultStore()
			if err != nil {
				return err
			}
			if err := store.Load(); err != nil {
				return err
			}

			watches := store.List()
			if len(watches) == 0 {
				fmt.Println("No active watches.")
				return nil
			}

			if format == "json" {
				return models.FormatJSON(os.Stdout, watches)
			}

			headers := []string{"ID", "Type", "Route", "Depart", "Return", "Goal", "Last", "Lowest"}
			rows := make([][]string, 0, len(watches))
			for _, w := range watches {
				route := w.Origin + " -> " + w.Destination
				goal := ""
				if w.BelowPrice > 0 {
					goal = fmt.Sprintf("%.0f %s", w.BelowPrice, w.Currency)
				}
				last := ""
				if w.LastPrice > 0 {
					last = fmt.Sprintf("%.0f %s", w.LastPrice, w.Currency)
				}
				lowest := ""
				if w.LowestPrice > 0 {
					lowest = fmt.Sprintf("%.0f %s", w.LowestPrice, w.Currency)
				}
				rows = append(rows, []string{
					w.ID, w.Type, route, w.DepartDate, w.ReturnDate,
					goal, last, lowest,
				})
			}

			models.FormatTable(os.Stdout, headers, rows)
			return nil
		},
	}
}

func watchRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := watch.DefaultStore()
			if err != nil {
				return err
			}
			if err := store.Load(); err != nil {
				return err
			}

			found, err := store.Remove(args[0])
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("watch %s not found", args[0])
			}

			fmt.Printf("Removed watch %s\n", args[0])
			return nil
		},
	}
}

func watchCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check all watches for price changes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := watch.DefaultStore()
			if err != nil {
				return err
			}
			if err := store.Load(); err != nil {
				return err
			}

			watches := store.List()
			if len(watches) == 0 {
				fmt.Println("No active watches to check.")
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			checker := &liveChecker{}
			results := watch.CheckAll(ctx, store, checker)

			notifier := &watch.Notifier{
				Out:      os.Stdout,
				UseColor: models.UseColor,
				Desktop:  true,
			}
			notifier.NotifyAll(results)
			return nil
		},
	}
}

func watchHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history ID",
		Short: "Show price history for a watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := watch.DefaultStore()
			if err != nil {
				return err
			}
			if err := store.Load(); err != nil {
				return err
			}

			w, ok := store.Get(args[0])
			if !ok {
				return fmt.Errorf("watch %s not found", args[0])
			}

			history := store.History(args[0])
			if len(history) == 0 {
				fmt.Printf("No price history for watch %s (%s -> %s).\n",
					w.ID, w.Origin, w.Destination)
				return nil
			}

			if format == "json" {
				return models.FormatJSON(os.Stdout, history)
			}

			fmt.Printf("Price history for %s -> %s (watch %s):\n\n",
				w.Origin, w.Destination, w.ID)

			headers := []string{"Date", "Price", "Currency"}
			rows := make([][]string, 0, len(history))
			for _, p := range history {
				rows = append(rows, []string{
					p.Timestamp.Format("2006-01-02 15:04"),
					fmt.Sprintf("%.0f", p.Price),
					p.Currency,
				})
			}

			models.FormatTable(os.Stdout, headers, rows)
			return nil
		},
	}
}

// liveChecker implements watch.PriceChecker using the real flight/hotel search APIs.
type liveChecker struct{}

func (c *liveChecker) CheckPrice(ctx context.Context, w watch.Watch) (float64, string, string, error) {
	switch w.Type {
	case "flight":
		return c.checkFlight(ctx, w)
	case "hotel":
		return c.checkHotel(ctx, w)
	default:
		return 0, "", "", fmt.Errorf("unknown watch type: %s", w.Type)
	}
}

func (c *liveChecker) checkFlight(ctx context.Context, w watch.Watch) (float64, string, string, error) {
	// Route watch or date range: use calendar/dates search.
	if w.IsRouteWatch() || w.IsDateRange() {
		return c.checkFlightRange(ctx, w)
	}

	// Specific date search.
	opts := flights.SearchOptions{ReturnDate: w.ReturnDate}
	result, err := flights.SearchFlights(ctx, w.Origin, w.Destination, w.DepartDate, opts)
	if err != nil {
		return 0, "", "", err
	}
	if !result.Success || len(result.Flights) == 0 {
		return 0, "", "", nil
	}

	cheapest := result.Flights[0]
	for _, f := range result.Flights[1:] {
		if f.Price > 0 && (cheapest.Price == 0 || f.Price < cheapest.Price) {
			cheapest = f
		}
	}
	return cheapest.Price, cheapest.Currency, w.DepartDate, nil
}

func (c *liveChecker) checkFlightRange(ctx context.Context, w watch.Watch) (float64, string, string, error) {
	from := w.DepartFrom
	to := w.DepartTo
	if w.IsRouteWatch() {
		// No dates specified — scan next 60 days.
		from = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		to = time.Now().AddDate(0, 0, 60).Format("2006-01-02")
	}

	result, err := flights.SearchCalendar(ctx, w.Origin, w.Destination, flights.CalendarOptions{
		FromDate: from,
		ToDate:   to,
	})
	if err != nil {
		return 0, "", "", err
	}
	if !result.Success || len(result.Dates) == 0 {
		return 0, "", "", nil
	}

	cheapest := result.Dates[0]
	for _, d := range result.Dates[1:] {
		if d.Price > 0 && (cheapest.Price == 0 || d.Price < cheapest.Price) {
			cheapest = d
		}
	}
	return cheapest.Price, cheapest.Currency, cheapest.Date, nil
}

func (c *liveChecker) checkHotel(ctx context.Context, w watch.Watch) (float64, string, string, error) {
	checkIn := w.DepartDate
	checkOut := w.ReturnDate
	if w.IsRouteWatch() {
		// Default to next weekend.
		now := time.Now()
		fri := now.AddDate(0, 0, int((5-now.Weekday()+7)%7))
		checkIn = fri.Format("2006-01-02")
		checkOut = fri.AddDate(0, 0, 2).Format("2006-01-02")
	}

	opts := hotels.HotelSearchOptions{
		CheckIn:  checkIn,
		CheckOut: checkOut,
		Currency: w.Currency,
	}
	result, err := hotels.SearchHotels(ctx, w.Destination, opts)
	if err != nil {
		return 0, "", "", err
	}
	if len(result.Hotels) == 0 {
		return 0, "", "", nil
	}

	cheapest := result.Hotels[0]
	for _, h := range result.Hotels[1:] {
		if h.Price > 0 && (cheapest.Price == 0 || h.Price < cheapest.Price) {
			cheapest = h
		}
	}
	return cheapest.Price, cheapest.Currency, checkIn, nil
}
