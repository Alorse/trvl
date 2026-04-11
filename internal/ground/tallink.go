package ground

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

// tallinkBookingBase is the Tallink booking SPA base URL.
// The timetables API lives under this domain and requires a JSESSIONID cookie
// obtained by first loading the booking page.
const tallinkBookingBase = "https://booking.tallink.com"

// tallinkDealThreshold is the price (EUR) below which a sailing is flagged as a deal.
// HEL-TAL typically costs EUR 20–40; anything below EUR 20 is promotional.
const tallinkDealThreshold = 20.0

// tallinkLimiter: 10 req/min — allows multiple detectors in a single hacks run
// without hitting the context deadline (previously 5 req/min / 12s caused
// "rate limiter: Wait(n=1) would exceed context deadline" during hacks searches).
var tallinkLimiter = rate.NewLimiter(rate.Every(6*time.Second), 1)

// tallinkClient is a shared HTTP client for Tallink API calls.
var tallinkClient = &http.Client{
	Timeout: 30 * time.Second,
}

// tallinkPort holds metadata for a Tallink ferry port.
type tallinkPort struct {
	Code string // Tallink port code (HEL, TAL, STO, RIG, TUR, ALA, PAL, KAP, VIS)
	Name string // Full port/terminal name
	City string // Display city name
}

// tallinkPorts maps lowercase city name / alias to Tallink port metadata.
var tallinkPorts = map[string]tallinkPort{
	// Helsinki
	"helsinki": {Code: "HEL", Name: "Helsinki West Terminal", City: "Helsinki"},
	"hel":      {Code: "HEL", Name: "Helsinki West Terminal", City: "Helsinki"},

	// Tallinn — new API uses TAL, not TLL
	"tallinn": {Code: "TAL", Name: "Tallinn D-Terminal", City: "Tallinn"},
	"tal":     {Code: "TAL", Name: "Tallinn D-Terminal", City: "Tallinn"},
	"tll":     {Code: "TAL", Name: "Tallinn D-Terminal", City: "Tallinn"}, // legacy alias
	"tln":     {Code: "TAL", Name: "Tallinn D-Terminal", City: "Tallinn"}, // legacy alias

	// Stockholm
	"stockholm": {Code: "STO", Name: "Stockholm Värtahamnen", City: "Stockholm"},
	"sto":       {Code: "STO", Name: "Stockholm Värtahamnen", City: "Stockholm"},

	// Riga
	"riga": {Code: "RIG", Name: "Riga Passenger Terminal", City: "Riga"},
	"rig":  {Code: "RIG", Name: "Riga Passenger Terminal", City: "Riga"},

	// Turku
	"turku": {Code: "TUR", Name: "Turku Ferry Terminal", City: "Turku"},
	"tur":   {Code: "TUR", Name: "Turku Ferry Terminal", City: "Turku"},
	"åbo":   {Code: "TUR", Name: "Turku Ferry Terminal", City: "Turku"},

	// Åland / Mariehamn (ALA replaces MAR and LNG in new API)
	"mariehamn": {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},
	"mar":       {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},
	"åland":     {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},
	"aland":     {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},
	"ala":       {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},
	// Långnäs is no longer a separate port; map to ALA.
	"långnäs": {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},
	"langnäs": {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},
	"lng":     {Code: "ALA", Name: "Mariehamn Ferry Terminal", City: "Mariehamn"},

	// Paldiski
	"paldiski": {Code: "PAL", Name: "Paldiski South Harbour", City: "Paldiski"},
	"pal":      {Code: "PAL", Name: "Paldiski South Harbour", City: "Paldiski"},

	// Kapellskär
	"kapellskär": {Code: "KAP", Name: "Kapellskär Ferry Terminal", City: "Kapellskär"},
	"kapellskar": {Code: "KAP", Name: "Kapellskär Ferry Terminal", City: "Kapellskär"},
	"kap":        {Code: "KAP", Name: "Kapellskär Ferry Terminal", City: "Kapellskär"},

	// Visby
	"visby": {Code: "VIS", Name: "Visby Ferry Terminal", City: "Visby"},
	"vis":   {Code: "VIS", Name: "Visby Ferry Terminal", City: "Visby"},
}

// tallinkRouteDurations stores approximate journey durations in minutes for each route pair.
// Key format: "FROM-TO" using uppercase port codes.
var tallinkRouteDurations = map[string]int{
	"HEL-TAL": 120,
	"TAL-HEL": 120,
	"STO-TAL": 960,
	"TAL-STO": 960,
	"STO-HEL": 960,
	"HEL-STO": 960,
	"STO-RIG": 1020,
	"RIG-STO": 1020,
	"TUR-STO": 660,
	"STO-TUR": 660,
	"HEL-ALA": 360,
	"ALA-HEL": 360,
	"PAL-KAP": 540,
	"KAP-PAL": 540,
	"HEL-VIS": 780,
	"VIS-HEL": 780,
}

// LookupTallinkPort resolves a city name or alias to a Tallink port (case-insensitive).
func LookupTallinkPort(city string) (tallinkPort, bool) {
	p, ok := tallinkPorts[strings.ToLower(strings.TrimSpace(city))]
	return p, ok
}

// HasTallinkPort returns true if the city has a known Tallink port.
func HasTallinkPort(city string) bool {
	_, ok := LookupTallinkPort(city)
	return ok
}

// HasTallinkRoute returns true if both cities have Tallink ports.
func HasTallinkRoute(from, to string) bool {
	return HasTallinkPort(from) && HasTallinkPort(to)
}

// tallinkRouteDuration returns the approximate journey duration in minutes for a port pair.
// Falls back to 120 minutes if the route is unknown.
func tallinkRouteDuration(fromCode, toCode string) int {
	key := fromCode + "-" + toCode
	if d, ok := tallinkRouteDurations[key]; ok {
		return d
	}
	return 120
}

// tallinkSail is a single sailing from the booking timetables API response.
type tallinkSail struct {
	SailID               int64   `json:"sailId"`
	ShipCode             string  `json:"shipCode"`
	DepartureIsoDate     string  `json:"departureIsoDate"`     // "2026-05-01T07:30"
	ArrivalIsoDate       string  `json:"arrivalIsoDate"`       // "2026-05-01T09:30"
	PersonPrice          string  `json:"personPrice"`          // "38.90"
	VehiclePrice         *string `json:"vehiclePrice"`         // null or "45.00"
	Duration             float64 `json:"duration"`             // hours, e.g. 2.0
	SailPackageCode      string  `json:"sailPackageCode"`      // "HEL-TAL"
	SailPackageName      string  `json:"sailPackageName"`      // "Helsinki-Tallinn"
	CityFrom             string  `json:"cityFrom"`             // "HEL"
	CityTo               string  `json:"cityTo"`               // "TAL"
	PierFrom             string  `json:"pierFrom"`
	PierTo               string  `json:"pierTo"`
	HasRoom              bool    `json:"hasRoom"`
	IsOvernight          bool    `json:"isOvernight"`
	IsDisabled           bool    `json:"isDisabled"`
	PromotionApplied     bool    `json:"promotionApplied"`
	MarketingMessage     *string `json:"marketingMessage"`
	IsVoucherApplicable  bool    `json:"isVoucherApplicable"`
}

// tallinkDayTrips holds outward and return sails for a single day.
type tallinkDayTrips struct {
	Outwards []tallinkSail `json:"outwards"`
	Returns  []tallinkSail `json:"returns"`
}

// tallinkTimetableResponse is the top-level response from the booking timetables API.
type tallinkTimetableResponse struct {
	DefaultSelections struct {
		OutwardSail int64 `json:"outwardSail"`
		ReturnSail  int64 `json:"returnSail"`
	} `json:"defaultSelections"`
	Trips map[string]tallinkDayTrips `json:"trips"` // key: "2026-05-01"
}

// buildTallinkBookingURL constructs a Tallink booking URL for the user.
func buildTallinkBookingURL(fromCode, toCode, date string) string {
	return fmt.Sprintf(
		"https://booking.tallink.com/?from=%s&to=%s&date=%s&locale=en&country=FI&voyageType=TRANSPORT",
		strings.ToLower(fromCode), strings.ToLower(toCode), date,
	)
}

// tallinkGetSession loads the booking page to obtain a JSESSIONID cookie,
// which is required for subsequent API calls. Returns cookies to attach.
func tallinkGetSession(ctx context.Context, fromCode, toCode, date string) ([]*http.Cookie, error) {
	pageURL := buildTallinkBookingURL(fromCode, toCode, date)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	// Use a client that does NOT follow redirects so we can capture cookies.
	noRedirectClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tallink session: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return nil, fmt.Errorf("tallink session: no cookies returned")
	}
	return cookies, nil
}

// fetchTallinkTimetables calls the booking.tallink.com timetables API
// which supports arbitrary future dates (unlike the old voyage-avails endpoint).
func fetchTallinkTimetables(ctx context.Context, fromCode, toCode, date string) (*tallinkTimetableResponse, error) {
	// Step 1: obtain session cookie
	cookies, err := tallinkGetSession(ctx, fromCode, toCode, date)
	if err != nil {
		return nil, err
	}

	// Step 2: call timetables API with the session cookie
	// dateFrom/dateTo: 3-day window like the SPA does
	dateTo := date // single day is fine; API returns what's in range
	parsedDate, err := time.Parse("2006-01-02", date)
	if err == nil {
		dateTo = parsedDate.Add(2 * 24 * time.Hour).Format("2006-01-02")
	}

	apiURL := fmt.Sprintf(
		"%s/api/timetables?locale=en&country=FI&from=%s&to=%s&oneWay=false&dateFrom=%s&dateTo=%s&voyageType=SHUTTLE&includeOvernight=false&searchFutureSails=false",
		tallinkBookingBase,
		strings.ToLower(fromCode), strings.ToLower(toCode),
		date, dateTo,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := tallinkClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tallink timetables: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("tallink timetables: HTTP %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("tallink timetables read: %w", err)
	}

	var result tallinkTimetableResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("tallink timetables decode: %w", err)
	}
	return &result, nil
}

// tallinkNormalizeDateTime normalizes the timetable API datetime format.
// Input: "2026-05-01T07:30" → "2026-05-01T07:30:00"
func tallinkNormalizeDateTime(s string) string {
	if s == "" {
		return ""
	}
	// The timetables API returns "2026-05-01T07:30" (no seconds).
	// Normalize to full ISO 8601 for consistency.
	if len(s) == 16 { // "2006-01-02T15:04"
		return s + ":00"
	}
	return s
}

// SearchTallink searches Tallink/Silja Line for ferry crossings between two cities.
// Uses the booking.tallink.com timetables API which supports any future date.
func SearchTallink(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromPort, ok := LookupTallinkPort(from)
	if !ok {
		return nil, fmt.Errorf("tallink: no port for %q", from)
	}
	toPort, ok := LookupTallinkPort(to)
	if !ok {
		return nil, fmt.Errorf("tallink: no port for %q", to)
	}

	if currency == "" {
		currency = "EUR"
	}

	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, fmt.Errorf("tallink: invalid date %q: %w", date, err)
	}

	if err := tallinkLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("tallink: rate limiter: %w", err)
	}

	slog.Debug("tallink search", "from", fromPort.City, "to", toPort.City, "date", date)

	timetable, err := fetchTallinkTimetables(ctx, fromPort.Code, toPort.Code, date)
	if err != nil {
		return nil, fmt.Errorf("tallink: %w", err)
	}

	// Collect outward sails for the requested date from the timetable.
	dayTrips, ok := timetable.Trips[date]
	if !ok {
		slog.Debug("tallink: no trips for date", "date", date, "available_dates", len(timetable.Trips))
		return nil, nil
	}

	sails := dayTrips.Outwards
	slog.Debug("tallink sails", "total", len(sails))

	if len(sails) == 0 {
		return nil, nil
	}

	bookingURL := buildTallinkBookingURL(fromPort.Code, toPort.Code, date)
	defaultDuration := tallinkRouteDuration(fromPort.Code, toPort.Code)

	var routes []models.GroundRoute
	for _, s := range sails {
		if s.IsDisabled {
			continue
		}

		depTime := tallinkNormalizeDateTime(s.DepartureIsoDate)
		arrTime := tallinkNormalizeDateTime(s.ArrivalIsoDate)

		duration := defaultDuration
		if computed := computeDurationMinutes(depTime, arrTime); computed > 0 {
			duration = computed
		}

		// Parse price from string ("38.90").
		var price float64
		if s.PersonPrice != "" {
			fmt.Sscanf(s.PersonPrice, "%f", &price)
		}

		var amenities []string
		if price > 0 && price < tallinkDealThreshold {
			amenities = append(amenities, "Deal")
		}
		if s.PromotionApplied {
			amenities = append(amenities, "Promotion")
		}

		routes = append(routes, models.GroundRoute{
			Provider: "tallink",
			Type:     "ferry",
			Price:    price,
			Currency: "EUR",
			Duration: duration,
			Departure: models.GroundStop{
				City:    fromPort.City,
				Station: fromPort.Name + tallinkShipSuffix(s.ShipCode),
				Time:    depTime,
			},
			Arrival: models.GroundStop{
				City:    toPort.City,
				Station: toPort.Name,
				Time:    arrTime,
			},
			Transfers:  0,
			BookingURL: bookingURL,
			Amenities:  amenities,
		})
	}

	slog.Debug("tallink results", "routes", len(routes))
	return routes, nil
}

// tallinkShipSuffix returns a ship name suffix for the station display, or empty string.
func tallinkShipSuffix(shipName string) string {
	if shipName == "" {
		return ""
	}
	return " (" + shipName + ")"
}

// newUUID is retained for potential future use (session tracking etc).
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
