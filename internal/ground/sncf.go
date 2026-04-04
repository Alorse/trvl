package ground

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/cookies"
	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

// SNCF calendar prices endpoint (public, no auth).
// Based on https://github.com/juliuste/sncf — the journey search endpoint
// (oui.sncf/proposition/rest/search-travels/outward) now requires Cloudflare
// JS challenge, but the calendar API may still work for price lookups.
const sncfCalendarEndpoint = "https://www.sncf-connect.com/calendar/cdp/api/public/calendar/v4/outward"

// sncfLimiter enforces a conservative rate limit: 10 req/min.
var sncfLimiter = rate.NewLimiter(rate.Every(6*time.Second), 1)

// sncfClient is a dedicated HTTP client for SNCF API calls.
// Uses Chrome TLS fingerprint via utls to bypass Cloudflare bot detection.
var sncfClient = batchexec.ChromeHTTPClient()

// SNCFStation holds metadata for an SNCF station.
type SNCFStation struct {
	Code    string // 5-letter SNCF code (e.g. FRPLY)
	Name    string
	City    string
	Country string
}

// sncfStations maps lowercase city name to station info.
// Station codes are 5-letter codes used by the SNCF internal API.
var sncfStations = map[string]SNCFStation{
	// Paris stations
	"paris":          {Code: "FRPAR", Name: "Paris (toutes gares)", City: "Paris", Country: "FR"},
	"paris gare de lyon": {Code: "FRPLY", Name: "Paris Gare de Lyon", City: "Paris", Country: "FR"},
	"paris nord":     {Code: "FRPNO", Name: "Paris Gare du Nord", City: "Paris", Country: "FR"},
	"paris montparnasse": {Code: "FRPMO", Name: "Paris Montparnasse", City: "Paris", Country: "FR"},
	"paris est":      {Code: "FRPST", Name: "Paris Gare de l'Est", City: "Paris", Country: "FR"},
	// Major French cities
	"lyon":           {Code: "FRLYS", Name: "Lyon Part-Dieu", City: "Lyon", Country: "FR"},
	"marseille":      {Code: "FRMRS", Name: "Marseille Saint-Charles", City: "Marseille", Country: "FR"},
	"bordeaux":       {Code: "FRBOJ", Name: "Bordeaux Saint-Jean", City: "Bordeaux", Country: "FR"},
	"toulouse":       {Code: "FRTLS", Name: "Toulouse Matabiau", City: "Toulouse", Country: "FR"},
	"nice":           {Code: "FRNIC", Name: "Nice Ville", City: "Nice", Country: "FR"},
	"strasbourg":     {Code: "FRSBG", Name: "Strasbourg", City: "Strasbourg", Country: "FR"},
	"lille":          {Code: "FRLIL", Name: "Lille Flandres", City: "Lille", Country: "FR"},
	"nantes":         {Code: "FRNTE", Name: "Nantes", City: "Nantes", Country: "FR"},
	"montpellier":    {Code: "FRMPL", Name: "Montpellier Saint-Roch", City: "Montpellier", Country: "FR"},
	"rennes":         {Code: "FRRNS", Name: "Rennes", City: "Rennes", Country: "FR"},
	"avignon":        {Code: "FRAVT", Name: "Avignon TGV", City: "Avignon", Country: "FR"},
	"dijon":          {Code: "FRDIJ", Name: "Dijon Ville", City: "Dijon", Country: "FR"},
	// Connecting international cities served by SNCF/TGV
	"brussels":       {Code: "BEBMI", Name: "Bruxelles-Midi", City: "Brussels", Country: "BE"},
	"geneva":         {Code: "CHGVA", Name: "Genève", City: "Geneva", Country: "CH"},
	"zurich":         {Code: "CHZRH", Name: "Zürich HB", City: "Zurich", Country: "CH"},
	"barcelona":      {Code: "ESBCN", Name: "Barcelona Sants", City: "Barcelona", Country: "ES"},
	"turin":          {Code: "ITTOI", Name: "Torino Porta Susa", City: "Turin", Country: "IT"},
	"milan":          {Code: "ITMIL", Name: "Milano Centrale", City: "Milan", Country: "IT"},
	"frankfurt":      {Code: "DEFRA", Name: "Frankfurt (Main) Hbf", City: "Frankfurt", Country: "DE"},
	"london":         {Code: "GBSPX", Name: "London St Pancras", City: "London", Country: "GB"},
}

// LookupSNCFStation resolves a city name to an SNCF station (case-insensitive).
func LookupSNCFStation(city string) (SNCFStation, bool) {
	s, ok := sncfStations[strings.ToLower(strings.TrimSpace(city))]
	return s, ok
}

// HasSNCFRoute returns true if both cities have SNCF stations AND at least one is French.
func HasSNCFRoute(from, to string) bool {
	fromStation, fromOK := LookupSNCFStation(from)
	toStation, toOK := LookupSNCFStation(to)
	if !fromOK || !toOK {
		return false
	}
	return fromStation.Country == "FR" || toStation.Country == "FR"
}

// sncfCalendarResponse is a single day's price from the calendar API.
type sncfCalendarResponse struct {
	Date  string `json:"date"`  // YYYY-MM-DD
	Price *int   `json:"price"` // price in cents, nil if unavailable
}

// SearchSNCF searches SNCF for cheapest fares between two stations.
// from/to are city names (e.g. "Paris", "Lyon"). date is YYYY-MM-DD.
//
// The old sncf-connect.com calendar API (sncfCalendarEndpoint) now returns
// 403/404 on all requests; the SPA uses an internal BFF that requires a live
// browser session and Datadome cookies. We therefore use the Playwright browser
// scraper as the primary method, with the calendar API as a best-effort fallback
// for environments where Playwright is not available.
func SearchSNCF(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromStation, ok := LookupSNCFStation(from)
	if !ok {
		return nil, fmt.Errorf("no SNCF station for %q", from)
	}
	toStation, ok := LookupSNCFStation(to)
	if !ok {
		return nil, fmt.Errorf("no SNCF station for %q", to)
	}

	if currency == "" {
		currency = "EUR"
	}

	slog.Debug("sncf search", "from", fromStation.City, "to", toStation.City, "date", date)

	// Primary: browser scraper (page-context API approach, same as ÖBB).
	// This navigates to sncf-connect.com, obtains the Datadome session, then
	// calls the internal BFF endpoints from JavaScript context.
	if bRoutes, bErr := BrowserScrapeRoutes(ctx, "sncf", from, to, date, currency); bErr == nil && len(bRoutes) > 0 {
		return bRoutes, nil
	} else if bErr != nil {
		slog.Debug("sncf browser scraper failed, trying calendar API fallback", "err", bErr)
	}

	// Fallback: legacy calendar API (may return 403 in most environments).
	if err := sncfLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("sncf rate limiter: %w", err)
	}

	apiURL := fmt.Sprintf("%s/%s/%s/%s/26-NO_CARD/2/en?onlyDirectTrains=false&currency=%s",
		sncfCalendarEndpoint,
		fromStation.Code,
		toStation.Code,
		date,
		url.QueryEscape(strings.ToUpper(currency)),
	)

	newSNCFRequest := func(cookieHeader string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "trvl/1.0 (travel agent; github.com/MikkoParkkola/trvl)")
		req.Header.Set("Origin", "https://www.sncf-connect.com")
		req.Header.Set("Referer", "https://www.sncf-connect.com/")
		if cookieHeader != "" {
			req.Header.Set("Cookie", cookieHeader)
		}
		return req, nil
	}

	req, err := newSNCFRequest("")
	if err != nil {
		return nil, err
	}

	resp, err := sncfClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sncf calendar api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		firstBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()

		// Try once more with browser cookies before giving up.
		cookieHeader := cookies.BrowserCookies("sncf-connect.com")
		if cookieHeader != "" {
			slog.Debug("retrying sncf calendar api with browser cookies")
			req2, err2 := newSNCFRequest(cookieHeader)
			if err2 == nil {
				if resp2, err2 := sncfClient.Do(req2); err2 == nil {
					defer resp2.Body.Close()
					if resp2.StatusCode == http.StatusOK {
						return parseSNCFResponse(resp2.Body, fromStation, toStation, date, currency)
					}
				}
			}
		}

		isCaptcha, captchaURL := cookies.IsCaptchaResponse(http.StatusForbidden, firstBody)
		if isCaptcha {
			slog.Warn("sncf requires browser verification", "captcha_url", captchaURL)
		}

		return nil, fmt.Errorf("sncf calendar api: HTTP %d (browser scraper also returned no results)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("sncf calendar api: HTTP %d: %s", resp.StatusCode, respBody)
	}

	return parseSNCFResponse(resp.Body, fromStation, toStation, date, currency)
}

// parseSNCFResponse decodes the calendar JSON body and returns GroundRoute values
// for the requested date.
func parseSNCFResponse(body io.Reader, fromStation, toStation SNCFStation, date, currency string) ([]models.GroundRoute, error) {
	var calEntries []sncfCalendarResponse
	if err := json.NewDecoder(body).Decode(&calEntries); err != nil {
		return nil, fmt.Errorf("sncf decode: %w", err)
	}

	var routes []models.GroundRoute
	for _, entry := range calEntries {
		if entry.Price == nil || *entry.Price <= 0 {
			continue
		}
		// Only include the requested date (or all dates if doing a range search).
		if entry.Date != date {
			continue
		}
		route := models.GroundRoute{
			Provider: "sncf",
			Type:     "train",
			Price:    float64(*entry.Price) / 100.0, // cents to euros
			Currency: strings.ToUpper(currency),
			Departure: models.GroundStop{
				City:    fromStation.City,
				Station: fromStation.Name,
				Time:    entry.Date,
			},
			Arrival: models.GroundStop{
				City:    toStation.City,
				Station: toStation.Name,
				Time:    entry.Date,
			},
			BookingURL: buildSNCFBookingURL(fromStation.Code, toStation.Code, entry.Date),
		}
		routes = append(routes, route)
	}

	return routes, nil
}

func buildSNCFBookingURL(originCode, destCode, date string) string {
	return fmt.Sprintf("https://www.sncf-connect.com/en-en/result/train/%s/%s/%s",
		url.PathEscape(originCode), url.PathEscape(destCode), url.PathEscape(date))
}
