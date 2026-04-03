package watch

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
)

// Notifier formats and delivers price check results.
type Notifier struct {
	Out      io.Writer
	UseColor bool
	Desktop  bool // attempt macOS desktop notifications
}

// Notify prints a check result to the writer with color coding.
// Green = price dropped, Red = price increased, bold alert if below threshold.
func (n *Notifier) Notify(r CheckResult) {
	if r.Error != nil {
		fmt.Fprintf(n.Out, "%s  %s -> %s  %s\n",
			n.red("ERR"),
			r.Watch.Origin, r.Watch.Destination,
			r.Error,
		)
		return
	}

	if r.NewPrice == 0 {
		fmt.Fprintf(n.Out, "%s  %s -> %s  no price data\n",
			n.yellow("---"),
			r.Watch.Origin, r.Watch.Destination,
		)
		return
	}

	route := fmt.Sprintf("%s -> %s", r.Watch.Origin, r.Watch.Destination)
	priceStr := fmt.Sprintf("%.0f %s", r.NewPrice, r.Currency)

	// Below-threshold alert.
	if r.BelowGoal {
		line := fmt.Sprintf("DEAL  %s  %s (below %.0f %s goal!)",
			route, priceStr, r.Watch.BelowPrice, r.Currency)
		fmt.Fprintln(n.Out, n.green(line))

		if r.Watch.DepartDate != "" {
			url := buildBookingURL(r.Watch)
			fmt.Fprintf(n.Out, "      Book: %s\n", url)
		}

		if n.Desktop {
			n.desktopNotify(
				"trvl: Price Alert!",
				fmt.Sprintf("%s %s — below your %.0f %s goal",
					route, priceStr, r.Watch.BelowPrice, r.Currency),
			)
		}
		return
	}

	// Regular price report with change indicator.
	var changeStr string
	if r.PrevPrice > 0 {
		diff := r.PriceDrop
		if diff < 0 {
			changeStr = n.green(fmt.Sprintf(" (%.0f)", diff))
		} else if diff > 0 {
			changeStr = n.red(fmt.Sprintf(" (+%.0f)", diff))
		} else {
			changeStr = " (unchanged)"
		}
	}

	lowest := ""
	if r.Watch.LowestPrice > 0 && r.Watch.LowestPrice < r.NewPrice {
		lowest = fmt.Sprintf("  lowest: %.0f", r.Watch.LowestPrice)
	}

	// Actionable advice based on price movement.
	advice := ""
	if r.PrevPrice > 0 {
		if r.PriceDrop < -r.PrevPrice*0.3 {
			// 30%+ drop — likely error fare or flash sale.
			advice = n.green("  ⚡ big drop — error fare or flash sale? Book fast!")
		} else if r.PriceDrop < 0 {
			// Normal drop — campaign, competition, demand shift.
			advice = n.green("  ↓ price dropped — good time to book")
		} else if r.PriceDrop > 0 && r.Watch.Type == "flight" {
			// Flight prices trending up — normal closer to departure.
			advice = n.red("  ↑ trending up — consider booking soon")
		}
	}

	fmt.Fprintf(n.Out, " %s  %s  %s%s%s%s\n",
		strings.ToUpper(r.Watch.Type[:1])+r.Watch.Type[1:],
		route, priceStr, changeStr, lowest, advice,
	)
}

// NotifyAll prints results for all checks.
func (n *Notifier) NotifyAll(results []CheckResult) {
	for _, r := range results {
		n.Notify(r)
	}
}

func buildBookingURL(w Watch) string {
	switch w.Type {
	case "flight":
		return fmt.Sprintf("https://www.google.com/travel/flights?q=Flights+to+%s+from+%s+on+%s",
			w.Destination, w.Origin, w.DepartDate)
	case "hotel":
		dates := w.DepartDate
		if w.ReturnDate != "" {
			dates += "," + w.ReturnDate
		}
		return fmt.Sprintf("https://www.google.com/travel/hotels/%s?dates=%s",
			w.Destination, dates)
	default:
		return ""
	}
}

func (n *Notifier) green(s string) string {
	if !n.UseColor {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func (n *Notifier) red(s string) string {
	if !n.UseColor {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func (n *Notifier) yellow(s string) string {
	if !n.UseColor {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

// desktopNotify sends a macOS notification via osascript. Best-effort; errors are ignored.
func (n *Notifier) desktopNotify(title, message string) {
	if runtime.GOOS != "darwin" {
		return
	}
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	_ = exec.Command("osascript", "-e", script).Run()
}
