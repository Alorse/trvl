package ground

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
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

// sncfBFFPaths lists the SNCF BFF API paths to try, in order of preference.
// These match the paths discovered by scraper.py via XHR interception.
var sncfBFFPaths = []struct {
	path   string
	bodyFn func(fromCode, toCode, date string) string
}{
	{
		path: "https://www.sncf-connect.com/bff/api/v1/itinerary-search",
		bodyFn: func(fromCode, toCode, date string) string {
			return fmt.Sprintf(
				`{"passengers":[{"type":"ADULT","fareType":"NO_CARD"}],"origin":%q,"destination":%q,"date":%q,"directTrainsOnly":false,"currency":"EUR"}`,
				fromCode, toCode, date+"T06:00:00",
			)
		},
	},
	{
		path: "https://www.sncf-connect.com/bff/api/v1/trainschedules",
		bodyFn: func(fromCode, toCode, date string) string {
			return fmt.Sprintf(
				`{"origin":%q,"destination":%q,"departureDate":%q,"passengers":[{"type":"ADULT","discountCards":[]}],"directOnly":false}`,
				fromCode, toCode, date+"T06:00:00",
			)
		},
	},
	{
		path: "https://www.sncf-connect.com/bff/api/v1/travel-proposals",
		bodyFn: func(fromCode, toCode, date string) string {
			return fmt.Sprintf(
				`{"origin":%q,"destination":%q,"outwardDate":%q,"passengers":[{"type":"ADULT"}]}`,
				fromCode, toCode, date,
			)
		},
	},
}

// sncfViaCurl tries to call SNCF's BFF API by shelling out to the system curl
// binary. macOS curl uses the system's native TLS stack (BoringSSL / Secure
// Transport) which produces a real browser-like ClientHello that Datadome does
// not flag as a bot — unlike Go's crypto/tls or even utls.
//
// No x-bff-key is passed because we have no live browser session to capture it
// from. Some BFF endpoints return data without the key; others return 401. We
// try all three known paths and return the first successful parse.
func sncfViaCurl(ctx context.Context, fromCode, toCode, date, currency string) ([]models.GroundRoute, error) {
	// Build a booking URL using the station codes directly.
	bookingURL := buildSNCFBookingURL(fromCode, toCode, date)

	// Shared Chrome-like headers for all requests.
	commonArgs := []string{
		"-s", "--http2",
		"-H", "Content-Type: application/json",
		"-H", "Accept: application/json",
		"-H", "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"-H", "Origin: https://www.sncf-connect.com",
		"-H", "Referer: https://www.sncf-connect.com/",
		"-H", "Accept-Language: en-US,en;q=0.9",
		"-H", "sec-ch-ua: \"Google Chrome\";v=\"131\", \"Chromium\";v=\"131\", \"Not_A Brand\";v=\"24\"",
		"-H", "sec-ch-ua-mobile: ?0",
		"-H", "sec-ch-ua-platform: \"macOS\"",
		"-H", "sec-fetch-dest: empty",
		"-H", "sec-fetch-mode: cors",
		"-H", "sec-fetch-site: same-origin",
	}

	for _, bffPath := range sncfBFFPaths {
		body := bffPath.bodyFn(fromCode, toCode, date)
		args := append(append([]string{}, commonArgs...), "-X", "POST", "-d", body, bffPath.path)

		cmd := exec.CommandContext(ctx, "curl", args...)
		output, err := cmd.Output()
		if err != nil {
			slog.Debug("sncf curl attempt failed", "path", bffPath.path, "err", err)
			continue
		}
		if len(output) == 0 {
			continue
		}

		// Reject non-JSON responses (HTML error pages, Datadome challenges).
		trimmed := strings.TrimSpace(string(output))
		if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
			slog.Debug("sncf curl non-JSON response", "path", bffPath.path, "preview", trimmed[:min(len(trimmed), 80)])
			continue
		}

		var data map[string]any
		if err := json.Unmarshal(output, &data); err != nil {
			continue
		}

		// Check for API-level error responses (401, 403, etc. wrapped in JSON).
		if errMsg, ok := data["error"].(string); ok && errMsg != "" {
			slog.Debug("sncf curl API error", "path", bffPath.path, "error", errMsg)
			continue
		}
		if status, ok := data["status"].(float64); ok && status >= 400 {
			slog.Debug("sncf curl API status error", "path", bffPath.path, "status", status)
			continue
		}

		routes := parseSNCFBFFResponse(data, bookingURL, date, currency)
		if len(routes) > 0 {
			slog.Debug("sncf curl success", "path", bffPath.path, "routes", len(routes))
			return routes, nil
		}
	}

	return nil, fmt.Errorf("sncf curl: no results from %d BFF endpoints", len(sncfBFFPaths))
}

// parseSNCFBFFResponse extracts GroundRoute values from a parsed SNCF BFF JSON
// response. It tolerates the multiple response shapes returned by the three BFF
// paths (itinerary-search, trainschedules, travel-proposals).
func parseSNCFBFFResponse(data map[string]any, bookingURL, date, currency string) []models.GroundRoute {
	if currency == "" {
		currency = "EUR"
	}

	// Try common top-level keys that contain journey arrays.
	topLevelKeys := []string{"journeys", "proposals", "trainSchedules", "results",
		"trips", "travelProposals", "connections", "outwardJourneys"}

	var items []any
	for _, key := range topLevelKeys {
		val, ok := data[key]
		if !ok {
			continue
		}
		arr, ok := val.([]any)
		if ok && len(arr) > 0 {
			items = arr
			break
		}
	}

	// Some responses nest under a "data" key.
	if len(items) == 0 {
		if nested, ok := data["data"].(map[string]any); ok {
			return parseSNCFBFFResponse(nested, bookingURL, date, currency)
		}
	}

	var routes []models.GroundRoute
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		r := extractSNCFBFFRoute(obj, bookingURL, date, currency)
		if r != nil {
			routes = append(routes, *r)
		}
	}
	return routes
}

// extractSNCFBFFRoute extracts a single GroundRoute from a SNCF journey/proposal
// map. Field names vary across the three BFF endpoints — we try all known variants.
func extractSNCFBFFRoute(item map[string]any, bookingURL, date, currency string) *models.GroundRoute {
	// --- Price ---
	price := 0.0
	cur := currency
	for _, pk := range []string{"price", "minPrice", "cheapestPrice", "amount", "totalPrice", "priceInCents"} {
		val := item[pk]
		if val == nil {
			continue
		}
		switch v := val.(type) {
		case map[string]any:
			raw := firstFloat(v, "amount", "value", "cents")
			if c, ok := v["currency"].(string); ok && c != "" {
				cur = c
			} else if c, ok := v["currencyCode"].(string); ok && c != "" {
				cur = c
			}
			if raw > 0 {
				if strings.Contains(strings.ToLower(pk), "cent") {
					price = raw / 100
				} else {
					price = raw
				}
			}
		case float64:
			if v > 0 {
				if strings.Contains(strings.ToLower(pk), "cent") {
					price = v / 100
				} else {
					price = v
				}
			}
		}
		if price > 0 {
			break
		}
	}
	if price <= 0 {
		return nil
	}

	// --- Departure / arrival times ---
	depTime := firstString(item, "departureDate", "departureTime", "departure", "startTime", "dep", "scheduledDepartureTime")
	arrTime := firstString(item, "arrivalDate", "arrivalTime", "arrival", "endTime", "arr", "scheduledArrivalTime")
	if depTime == "" {
		return nil
	}
	if len(depTime) > 19 {
		depTime = depTime[:19]
	}
	if len(arrTime) > 19 {
		arrTime = arrTime[:19]
	}

	// --- Duration (minutes) ---
	duration := 0
	for _, dk := range []string{"duration", "travelTime", "durationInMinutes", "journeyDuration"} {
		if v, ok := item[dk].(float64); ok && v > 0 {
			switch {
			case v > 86400:
				duration = int(v) / 60000 // milliseconds
			case v > 1440:
				duration = int(v) / 60 // seconds
			default:
				duration = int(v) // minutes
			}
			break
		}
	}

	// --- Transfers ---
	transfers := 0
	for _, tk := range []string{"transfers", "changes", "numberOfChanges", "numChanges"} {
		if v, ok := item[tk].(float64); ok {
			transfers = max(0, int(v))
			break
		}
	}

	return &models.GroundRoute{
		Provider: "sncf",
		Type:     "train",
		Price:    price,
		Currency: strings.ToUpper(cur),
		Duration: duration,
		Transfers: transfers,
		Departure: models.GroundStop{Time: depTime},
		Arrival:   models.GroundStop{Time: arrTime},
		BookingURL: bookingURL,
	}
}

// firstFloat returns the first non-zero float64 found under any of the given
// keys in a map[string]any.
func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok && v != 0 {
			return v
		}
	}
	return 0
}

// firstString returns the first non-empty string found under any of the given
// keys in a map[string]any.
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// min returns the smaller of two ints (Go 1.21+ builtin, added here for clarity).
func sncfMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SearchSNCF searches SNCF for cheapest fares between two stations.
// from/to are city names (e.g. "Paris", "Lyon"). date is YYYY-MM-DD.
//
// Search order:
//  1. curl-based BFF call: macOS curl uses BoringSSL/Secure Transport which
//     produces a real browser TLS fingerprint that bypasses Datadome at the
//     network layer. Tries three known BFF paths.
//  2. Playwright browser scraper: navigates sncf-connect.com headlessly, captures
//     the x-bff-key the SPA injects, and calls the BFF from page context.
//  3. Legacy calendar API: the old public endpoint; returns 403 in most
//     environments but still works from some networks/IPs.
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

	// Primary: curl BFF — macOS BoringSSL TLS fingerprint bypasses Datadome.
	if cRoutes, cErr := sncfViaCurl(ctx, fromStation.Code, toStation.Code, date, currency); cErr == nil && len(cRoutes) > 0 {
		// Populate city/station names that parseSNCFBFFResponse cannot fill in.
		for i := range cRoutes {
			if cRoutes[i].Departure.City == "" {
				cRoutes[i].Departure.City = fromStation.City
				cRoutes[i].Departure.Station = fromStation.Name
			}
			if cRoutes[i].Arrival.City == "" {
				cRoutes[i].Arrival.City = toStation.City
				cRoutes[i].Arrival.Station = toStation.Name
			}
		}
		return cRoutes, nil
	} else {
		slog.Debug("sncf curl failed, trying browser scraper", "err", cErr)
	}

	// Secondary: browser scraper (page-context API approach, same as ÖBB).
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
