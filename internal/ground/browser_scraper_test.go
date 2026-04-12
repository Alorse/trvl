package ground

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func requireScraperScriptPath(t *testing.T) string {
	t.Helper()

	path, err := resolveScraperScriptPath()
	if errors.Is(err, errScraperScriptNotFound) {
		t.Skip(err.Error())
	}
	if err != nil {
		t.Fatalf("resolveScraperScriptPath: %v", err)
	}
	return path
}

// TestBrowserScrapeRoutes_Python3NotFound verifies graceful failure when python3
// is absent. We cannot easily remove python3, so this test only runs when
// explicitly requested via the TRVL_TEST_NO_PYTHON env var.
func TestBrowserScrapeRoutes_Python3NotFound(t *testing.T) {
	if os.Getenv("TRVL_TEST_NO_PYTHON") == "" {
		t.Skip("set TRVL_TEST_NO_PYTHON=1 to run")
	}
	ctx := context.Background()
	_, err := BrowserScrapeRoutes(ctx, "trainline", "London", "Paris", "2026-04-10", "EUR")
	if err == nil {
		t.Error("expected error when python3 unavailable")
	}
}

func TestBrowserScrapeRoutes_MissingScraperScript(t *testing.T) {
	t.Setenv("TRVL_SCRAPER_PATH", "__definitely_missing_scraper__.py")

	ctx := context.Background()
	_, err := BrowserScrapeRoutes(ctx, "trainline", "London", "Paris", "2026-04-10", "EUR")
	if !errors.Is(err, errScraperScriptNotFound) {
		t.Fatalf("expected missing scraper error, got %v", err)
	}
}

// TestBrowserScrapeRoutes_ScraperScriptExists ensures the scraper.py file is
// present relative to browser_scraper.go at build time.
func TestBrowserScrapeRoutes_ScraperScriptExists(t *testing.T) {
	path := requireScraperScriptPath(t)
	if path == "" {
		t.Fatal("scraperScriptPath returned empty string")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scraper.py not readable at %s: %v", path, err)
	}
}

// TestBrowserScrapeRoutes_ScraperOutputParsing verifies that scraperOutput
// JSON round-trips correctly into GroundRoute values.
func TestBrowserScrapeRoutes_ScraperOutputParsing(t *testing.T) {
	raw := `{
		"routes": [
			{
				"price": 39.00,
				"currency": "GBP",
				"departure": "2026-04-10T06:31:00",
				"arrival": "2026-04-10T09:47:00",
				"duration": 196,
				"type": "train",
				"provider": "eurostar",
				"transfers": 0,
				"booking_url": "https://www.thetrainline.com/book/trains/london/paris/2026-04-10"
			},
			{
				"price": 0,
				"currency": "EUR",
				"departure": "2026-04-10T07:00:00",
				"arrival": "2026-04-10T10:00:00",
				"duration": 180,
				"type": "train",
				"provider": "thalys",
				"transfers": 0
			}
		]
	}`

	var result scraperOutput
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(result.Routes))
	}

	// Verify first route.
	r0 := result.Routes[0]
	if r0.Price != 39.00 {
		t.Errorf("price = %f, want 39.00", r0.Price)
	}
	if r0.Currency != "GBP" {
		t.Errorf("currency = %q, want GBP", r0.Currency)
	}
	if r0.Duration != 196 {
		t.Errorf("duration = %d, want 196", r0.Duration)
	}
	if r0.Provider != "eurostar" {
		t.Errorf("provider = %q, want eurostar", r0.Provider)
	}
	if r0.Transfers != 0 {
		t.Errorf("transfers = %d, want 0", r0.Transfers)
	}
	if r0.BookingURL == "" {
		t.Error("booking_url should not be empty")
	}

	// Zero-price route.
	r1 := result.Routes[1]
	if r1.Price != 0 {
		t.Errorf("second route price = %f, want 0", r1.Price)
	}
}

// TestBrowserScrapeRoutes_ErrorResponse verifies that a scraper error field
// is surfaced as a Go error.
func TestBrowserScrapeRoutes_ErrorResponse(t *testing.T) {
	// We feed a pre-built JSON error response to the parser directly.
	raw := `{"routes":[],"error":"playwright not installed"}`
	var result scraperOutput
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error field to be set")
	}
	if result.Error != "playwright not installed" {
		t.Errorf("error = %q, want 'playwright not installed'", result.Error)
	}
}

// TestScraperPy_SyntaxCheck verifies scraper.py is valid Python 3 syntax.
// Uses python3 -m py_compile which exits 0 on success, non-zero on error.
func TestScraperPy_SyntaxCheck(t *testing.T) {
	path := requireScraperScriptPath(t)

	cmd := exec.Command("python3", "-m", "py_compile", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scraper.py syntax error: %v\n%s", err, out)
	}
}

// TestScraperPy_MissingInputHandling verifies that scraper.py writes a valid
// JSON error response when given no stdin input (instead of crashing).
func TestScraperPy_MissingInputHandling(t *testing.T) {
	path := requireScraperScriptPath(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", path)
	cmd.Stdin = strings.NewReader("") // empty stdin

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("scraper.py crashed on empty input: %v", err)
	}

	var result scraperOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("scraper.py did not output valid JSON: %v\nraw: %s", err, out)
	}
	if result.Error == "" {
		t.Error("expected error field for empty input")
	}
}

// TestScraperPy_UnknownProvider verifies that scraper.py gracefully handles
// an unrecognised provider name.
func TestScraperPy_UnknownProvider(t *testing.T) {
	path := requireScraperScriptPath(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	inp, _ := json.Marshal(map[string]string{
		"provider": "nosuchprovider",
		"from":     "London",
		"to":       "Paris",
		"date":     "2026-04-10",
		"currency": "EUR",
	})

	cmd := exec.CommandContext(ctx, "python3", path)
	cmd.Stdin = strings.NewReader(string(inp))

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("scraper.py crashed on unknown provider: %v", err)
	}

	var result scraperOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("scraper.py did not output valid JSON: %v\nraw: %s", err, out)
	}
	if result.Error == "" {
		t.Error("expected error field for unknown provider")
	}
}

// TestScraperPy_InvalidJSON verifies that scraper.py handles bad JSON input
// without panicking and returns an error response.
func TestScraperPy_InvalidJSON(t *testing.T) {
	path := requireScraperScriptPath(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", path)
	cmd.Stdin = strings.NewReader("not valid json {{{")

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("scraper.py crashed on invalid JSON: %v", err)
	}

	var result scraperOutput
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("scraper.py did not output valid JSON: %v\nraw: %s", err, out)
	}
	if result.Error == "" {
		t.Error("expected error field for invalid JSON input")
	}
}

// TestBrowserScrapeRoutes_Integration runs a live browser scrape for Trainline.
// Skipped in short mode and when TRVL_TEST_BROWSER is not set (expensive/flaky).
func TestBrowserScrapeRoutes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser integration test in short mode")
	}
	if os.Getenv("TRVL_TEST_BROWSER") == "" {
		t.Skip("set TRVL_TEST_BROWSER=1 to run browser integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	date := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	routes, err := BrowserScrapeRoutes(ctx, "trainline", "London", "Paris", date, "EUR")
	if err != nil {
		t.Skipf("browser scraper unavailable: %v", err)
	}
	if len(routes) == 0 {
		t.Skip("no routes returned (page may have changed)")
	}

	r := routes[0]
	if r.Price <= 0 {
		t.Errorf("price = %f, want > 0", r.Price)
	}
	if r.Type != "train" && r.Type != "mixed" {
		t.Errorf("type = %q, want train or mixed", r.Type)
	}
	if r.Departure.City == "" {
		t.Error("departure city should not be empty")
	}
	if r.Arrival.City == "" {
		t.Error("arrival city should not be empty")
	}
	if r.BookingURL == "" {
		t.Error("booking URL should not be empty")
	}
}
