package ground

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// browserScraperTimeout is the maximum time allowed for a single browser scrape.
const browserScraperTimeout = 45 * time.Second

// scraperRoute is the JSON shape emitted by scraper.py per route.
type scraperRoute struct {
	Price      float64 `json:"price"`
	Currency   string  `json:"currency"`
	Departure  string  `json:"departure"`
	Arrival    string  `json:"arrival"`
	Duration   int     `json:"duration"`
	Type       string  `json:"type"`
	Provider   string  `json:"provider"`
	Transfers  int     `json:"transfers"`
	BookingURL string  `json:"booking_url"`
}

// scraperOutput is the top-level JSON shape emitted by scraper.py.
type scraperOutput struct {
	Routes []scraperRoute `json:"routes"`
	Error  string         `json:"error,omitempty"`
}

// scraperScriptPath returns the absolute path to scraper.py, resolved relative
// to this source file so it works regardless of the working directory.
func scraperScriptPath() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		// Fallback: assume cwd contains the script.
		return "scraper.py"
	}
	return filepath.Join(filepath.Dir(thisFile), "scraper.py")
}

// BrowserScrapeRoutes launches the Playwright scraper to fetch live train
// prices from provider websites via a real headless browser.
//
// Falls back gracefully if Python 3 or Playwright is not installed — callers
// should treat a non-nil error as "unavailable" and continue without results.
func BrowserScrapeRoutes(ctx context.Context, provider, from, to, date, currency string) ([]models.GroundRoute, error) {
	input := map[string]string{
		"provider": provider,
		"from":     from,
		"to":       to,
		"date":     date,
		"currency": currency,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("browser scraper marshal: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, browserScraperTimeout)
	defer cancel()

	scriptPath := scraperScriptPath()
	cmd := exec.CommandContext(timeoutCtx, "python3", scriptPath)
	cmd.Stdin = bytes.NewReader(inputJSON)

	slog.Debug("browser scraper launching", "provider", provider, "from", from, "to", to, "date", date)

	output, err := cmd.Output()
	if err != nil {
		// Distinguish "python3 not found" from a script error.
		if isExecNotFound(err) {
			return nil, fmt.Errorf("browser scraper: python3 not found in PATH")
		}
		// cmd.Output() wraps stderr in *exec.ExitError.Detail; surface it.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("browser scraper exited %d: %s", exitErr.ExitCode(), exitErr.Stderr)
		}
		return nil, fmt.Errorf("browser scraper: %w", err)
	}

	var result scraperOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("browser scraper decode: %w (raw: %.200s)", err, output)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("browser scraper: %s", result.Error)
	}

	routes := make([]models.GroundRoute, 0, len(result.Routes))
	for _, sr := range result.Routes {
		if sr.Price <= 0 {
			continue
		}
		r := models.GroundRoute{
			Provider:  sr.Provider,
			Type:      sr.Type,
			Price:     sr.Price,
			Currency:  sr.Currency,
			Duration:  sr.Duration,
			Transfers: sr.Transfers,
			Departure: models.GroundStop{
				City: from,
				Time: sr.Departure,
			},
			Arrival: models.GroundStop{
				City: to,
				Time: sr.Arrival,
			},
			BookingURL: sr.BookingURL,
		}
		if r.Type == "" {
			r.Type = "train"
		}
		if r.Provider == "" {
			r.Provider = provider
		}
		routes = append(routes, r)
	}

	slog.Debug("browser scraper results", "provider", provider, "count", len(routes))
	return routes, nil
}

// isExecNotFound returns true when the error indicates the executable was not
// found on PATH (i.e. python3 is not installed).
func isExecNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "exec: \"python3\": executable file not found in $PATH"
}
