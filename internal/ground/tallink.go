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

// tallinkAPIBase is the Tallink voyage availability API base URL.
const tallinkAPIBase = "https://book.tallink.com/api"

// tallinkDealThreshold is the price (EUR) below which a sailing is flagged as a deal.
// HEL-TAL typically costs EUR 20–40; anything below EUR 20 is promotional.
const tallinkDealThreshold = 20.0

// tallinkLimiter: conservative 5 req/min.
var tallinkLimiter = rate.NewLimiter(rate.Every(12*time.Second), 1)

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

// tallinkSailLeg is one leg of a sailing from the voyage-avails response.
type tallinkSailLeg struct {
	From struct {
		Port          string `json:"port"`
		Pier          string `json:"pier"`
		LocalDateTime string `json:"localDateTime"` // "2026-04-10T07:30:00"
	} `json:"from"`
	To struct {
		Port          string `json:"port"`
		Pier          string `json:"pier"`
		LocalDateTime string `json:"localDateTime"`
	} `json:"to"`
}

// tallinkTravelClass holds price information embedded in a voyage.
type tallinkTravelClass struct {
	MinPrice float64 `json:"minPrice"`
}

// tallinkVoyageAvail is the inner voyage payload within a voyage-avails array entry.
type tallinkVoyageAvail struct {
	PackageID     int64              `json:"packageId"`
	ShipCode      string             `json:"shipCode"`
	SailType      string             `json:"sailType"`
	TravelClass   tallinkTravelClass `json:"travelClass"`
	SailLegs      []tallinkSailLeg   `json:"sailLegs"`
	IsCheckInOpen bool               `json:"isCheckInOpen"`
	IsOvernight   bool               `json:"isOvernight"`
	PackageMode   string             `json:"packageMode"`
}

// tallinkVoyageAvailEntry is a top-level entry in the voyage-avails JSON array.
// The API wraps each voyage in an {"initialSail": {...}} object.
type tallinkVoyageAvailEntry struct {
	InitialSail tallinkVoyageAvail `json:"initialSail"`
}

// buildTallinkBookingURL constructs a Tallink booking URL using the new site.
func buildTallinkBookingURL(fromCode, toCode, date string) string {
	return fmt.Sprintf(
		"https://book.tallink.com/?departure=%s-%s&date=%s&adults=1",
		fromCode, toCode, date,
	)
}

// fetchTallinkVoyages calls the Tallink voyage-avails API and returns available sailings.
func fetchTallinkVoyages(ctx context.Context, fromCode, toCode, country string) ([]tallinkVoyageAvail, error) {
	if country == "" {
		country = "FI"
	}

	reqURL := fmt.Sprintf(
		"%s/voyage-avails?sailType=TRANSPORT&routes=%s-%s&routeSeqN=1&country=%s",
		tallinkAPIBase, fromCode, toCode, country,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp, err := tallinkClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tallink voyage-avails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("tallink voyage-avails: HTTP %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("tallink voyage-avails read: %w", err)
	}

	// Response is a JSON array where each element wraps a voyage in {"initialSail": {...}}.
	var entries []tallinkVoyageAvailEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("tallink voyage-avails decode: %w", err)
	}
	voyages := make([]tallinkVoyageAvail, 0, len(entries))
	for _, e := range entries {
		voyages = append(voyages, e.InitialSail)
	}
	return voyages, nil
}

// tallinkParseLocalDateTime parses the ISO 8601 local date-time from the API.
// Input: "2026-04-10T07:30:00" — returned as-is after validation.
func tallinkParseLocalDateTime(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02T15:04:05")
}

// tallinkFilterByDate returns only voyages whose first leg departure is on the given date ("2026-04-10").
func tallinkFilterByDate(voyages []tallinkVoyageAvail, date string) []tallinkVoyageAvail {
	var out []tallinkVoyageAvail
	for _, v := range voyages {
		if len(v.SailLegs) == 0 {
			continue
		}
		dep := v.SailLegs[0].From.LocalDateTime
		if strings.HasPrefix(dep, date) {
			out = append(out, v)
		}
	}
	return out
}

// SearchTallink searches Tallink/Silja Line for ferry crossings between two cities.
// It calls the public voyage-avails API (single call, includes prices).
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

	// Single API call returns schedules and prices together.
	voyages, err := fetchTallinkVoyages(ctx, fromPort.Code, toPort.Code, "FI")
	if err != nil {
		return nil, fmt.Errorf("tallink: %w", err)
	}

	// Filter to the requested departure date.
	voyages = tallinkFilterByDate(voyages, date)
	slog.Debug("tallink voyages", "total", len(voyages))

	if len(voyages) == 0 {
		return nil, nil
	}

	bookingURL := buildTallinkBookingURL(fromPort.Code, toPort.Code, date)
	defaultDuration := tallinkRouteDuration(fromPort.Code, toPort.Code)

	var routes []models.GroundRoute
	for _, v := range voyages {
		if len(v.SailLegs) == 0 {
			continue
		}

		firstLeg := v.SailLegs[0]
		lastLeg := v.SailLegs[len(v.SailLegs)-1]

		depTime := tallinkParseLocalDateTime(firstLeg.From.LocalDateTime)
		arrTime := tallinkParseLocalDateTime(lastLeg.To.LocalDateTime)

		duration := defaultDuration
		if computed := computeDurationMinutes(depTime, arrTime); computed > 0 {
			duration = computed
		}

		price := v.TravelClass.MinPrice

		// Flag promotional pricing.
		var amenities []string
		if price > 0 && price < tallinkDealThreshold {
			amenities = append(amenities, "Deal")
		}

		routes = append(routes, models.GroundRoute{
			Provider: "tallink",
			Type:     "ferry",
			Price:    price,
			Currency: "EUR",
			Duration: duration,
			Departure: models.GroundStop{
				City:    fromPort.City,
				Station: fromPort.Name + tallinkShipSuffix(v.ShipCode),
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

// newUUID generates a random UUID v4 string using crypto/rand.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: use time-based pseudo-random (extremely unlikely path).
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
