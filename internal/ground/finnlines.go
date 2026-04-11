package ground

import (
	"bytes"
	"context"
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

// finnlinesGraphQL is the AWS AppSync GraphQL endpoint for Finnlines booking.
const finnlinesGraphQL = "https://dm3xyy44wbeivgqmeymvmw22be.appsync-api.eu-central-1.amazonaws.com/graphql"

// finnlinesAPIKey is the public API key embedded in the booking SPA JS bundle.
const finnlinesAPIKey = "da2-zvuktusyubbstlw7khps4vyeie"

// finnlinesLimiter: 10 req/min to be respectful.
var finnlinesLimiter = rate.NewLimiter(rate.Every(6*time.Second), 1)

// finnlinesClient is a shared HTTP client for Finnlines API calls.
var finnlinesClient = &http.Client{
	Timeout: 30 * time.Second,
}

// finnlinesPort holds metadata for a Finnlines ferry port.
type finnlinesPort struct {
	Code string // Finnlines port code (FIHEL, DETRV, FINLI, SEKPS, etc.)
	Name string // Full port/terminal name
	City string // Display city name
}

// finnlinesPorts maps lowercase city name / alias to Finnlines port metadata.
var finnlinesPorts = map[string]finnlinesPort{
	// Helsinki
	"helsinki": {Code: "FIHEL", Name: "Helsinki Vuosaari Harbour", City: "Helsinki"},
	"hel":     {Code: "FIHEL", Name: "Helsinki Vuosaari Harbour", City: "Helsinki"},

	// Naantali
	"naantali": {Code: "FINLI", Name: "Naantali Harbour", City: "Naantali"},
	"nli":      {Code: "FINLI", Name: "Naantali Harbour", City: "Naantali"},

	// Travemünde (Germany)
	"travemünde": {Code: "DETRV", Name: "Travemünde Ferry Terminal", City: "Travemünde"},
	"travemunde": {Code: "DETRV", Name: "Travemünde Ferry Terminal", City: "Travemünde"},
	"trv":        {Code: "DETRV", Name: "Travemünde Ferry Terminal", City: "Travemünde"},

	// Rostock (Germany) — same terminal area as Travemünde for some services
	"rostock": {Code: "DETRV", Name: "Travemünde Ferry Terminal", City: "Travemünde"},

	// Kapellskär (Sweden)
	"kapellskär": {Code: "SEKPS", Name: "Kapellskär Ferry Terminal", City: "Kapellskär"},
	"kapellskar": {Code: "SEKPS", Name: "Kapellskär Ferry Terminal", City: "Kapellskär"},
	"kps":        {Code: "SEKPS", Name: "Kapellskär Ferry Terminal", City: "Kapellskär"},

	// Malmö (Sweden)
	"malmö": {Code: "SEMMA", Name: "Malmö Ferry Terminal", City: "Malmö"},
	"malmo": {Code: "SEMMA", Name: "Malmö Ferry Terminal", City: "Malmö"},
	"mma":   {Code: "SEMMA", Name: "Malmö Ferry Terminal", City: "Malmö"},

	// Świnoujście (Poland)
	"świnoujście": {Code: "PLSWI", Name: "Świnoujście Ferry Terminal", City: "Świnoujście"},
	"swinoujscie": {Code: "PLSWI", Name: "Świnoujście Ferry Terminal", City: "Świnoujście"},
	"swi":         {Code: "PLSWI", Name: "Świnoujście Ferry Terminal", City: "Świnoujście"},

	// Långnäs (Åland)
	"långnäs": {Code: "FILAN", Name: "Långnäs Ferry Terminal", City: "Långnäs"},
	"langnäs": {Code: "FILAN", Name: "Långnäs Ferry Terminal", City: "Långnäs"},
	"langnas": {Code: "FILAN", Name: "Långnäs Ferry Terminal", City: "Långnäs"},
}

// LookupFinnlinesPort resolves a city name or alias to a Finnlines port.
func LookupFinnlinesPort(city string) (finnlinesPort, bool) {
	p, ok := finnlinesPorts[strings.ToLower(strings.TrimSpace(city))]
	return p, ok
}

// HasFinnlinesPort returns true if the city has a known Finnlines port.
func HasFinnlinesPort(city string) bool {
	_, ok := LookupFinnlinesPort(city)
	return ok
}

// HasFinnlinesRoute returns true if both cities have Finnlines ports.
func HasFinnlinesRoute(from, to string) bool {
	return HasFinnlinesPort(from) && HasFinnlinesPort(to)
}

// finnlinesTimetableResponse is a single timetable entry from the GraphQL response.
type finnlinesTimetableEntry struct {
	SailingCode   string `json:"sailingCode"`
	DepartureDate string `json:"departureDate"` // "2026-05-01"
	DepartureTime string `json:"departureTime"` // "10:00"
	ArrivalDate   string `json:"arrivalDate"`
	ArrivalTime   string `json:"arrivalTime"`
	DeparturePort string `json:"departurePort"`
	ArrivalPort   string `json:"arrivalPort"`
	IsAvailable   bool   `json:"isAvailable"`
	ShipName      string `json:"shipName"`
	CrossingTime  string `json:"crossingTime"` // "7:45"
	ChargeTotal   *int   `json:"chargeTotal"`  // cents, nullable
}

// finnlinesGraphQLResponse wraps the GraphQL response envelope.
type finnlinesGraphQLResponse struct {
	Data struct {
		ListTimeTableAvailability []finnlinesTimetableEntry `json:"listTimeTableAvailability"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// fetchFinnlinesTimetables queries the Finnlines GraphQL API for timetables with prices.
func fetchFinnlinesTimetables(ctx context.Context, fromCode, toCode, date string) ([]finnlinesTimetableEntry, error) {
	query := `query ListTimeTableAvailability($query:TimetableQuery!){listTimeTableAvailability(query:$query){...on Timetable{sailingCode departureDate departureTime arrivalDate arrivalTime departurePort arrivalPort isAvailable shipName crossingTime chargeTotal}}}`

	variables := map[string]any{
		"query": map[string]any{
			"currency": "EUR",
			"language": "EN",
			"tariff": []map[string]any{
				{"legCode": 1, "type": "SPECIAL"},
			},
			"sailings": []map[string]any{
				{
					"legCode":       1,
					"departurePort": fromCode,
					"arrivalPort":   toCode,
					"startDate":     date,
					"numberOfDays":  1,
				},
			},
			"passengers": []map[string]any{
				{"legCode": 1, "id": 1, "type": "ADULT"},
			},
			"accommodations": []any{},
		},
	}

	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("finnlines: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, finnlinesGraphQL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", finnlinesAPIKey)

	resp, err := finnlinesClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("finnlines: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("finnlines: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnlines: HTTP %d: %s", resp.StatusCode, respBody)
	}

	var gqlResp finnlinesGraphQLResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("finnlines: decode: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("finnlines: GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data.ListTimeTableAvailability, nil
}

// buildFinnlinesBookingURL constructs a Finnlines booking URL.
func buildFinnlinesBookingURL(fromCode, toCode, date string) string {
	return fmt.Sprintf(
		"https://booking.finnlines.com/search?departurePort=%s&arrivalPort=%s&departureDate=%s&adults=1",
		fromCode, toCode, date,
	)
}

// parseFinnlinesCrossingMinutes parses a crossing time like "7:45" to minutes (465).
func parseFinnlinesCrossingMinutes(s string) int {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	var h, m int
	fmt.Sscanf(parts[0], "%d", &h)
	fmt.Sscanf(parts[1], "%d", &m)
	return h*60 + m
}

// SearchFinnlines searches Finnlines for ferry crossings between two cities.
func SearchFinnlines(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromPort, ok := LookupFinnlinesPort(from)
	if !ok {
		return nil, fmt.Errorf("finnlines: no port for %q", from)
	}
	toPort, ok := LookupFinnlinesPort(to)
	if !ok {
		return nil, fmt.Errorf("finnlines: no port for %q", to)
	}

	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, fmt.Errorf("finnlines: invalid date %q: %w", date, err)
	}

	if err := finnlinesLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("finnlines: rate limiter: %w", err)
	}

	slog.Debug("finnlines search", "from", fromPort.City, "to", toPort.City, "date", date)

	entries, err := fetchFinnlinesTimetables(ctx, fromPort.Code, toPort.Code, date)
	if err != nil {
		return nil, fmt.Errorf("finnlines: %w", err)
	}

	bookingURL := buildFinnlinesBookingURL(fromPort.Code, toPort.Code, date)

	var routes []models.GroundRoute
	for _, e := range entries {
		// Skip if not on the requested date.
		if e.DepartureDate != date {
			continue
		}

		depTime := e.DepartureDate + "T" + e.DepartureTime + ":00"
		arrTime := e.ArrivalDate + "T" + e.ArrivalTime + ":00"

		duration := parseFinnlinesCrossingMinutes(e.CrossingTime)
		if duration == 0 {
			if computed := computeDurationMinutes(depTime, arrTime); computed > 0 {
				duration = computed
			}
		}

		// Price is in cents; convert to EUR.
		var price float64
		if e.ChargeTotal != nil {
			price = float64(*e.ChargeTotal) / 100.0
		}

		var amenities []string
		if !e.IsAvailable {
			amenities = append(amenities, "Sold out")
		}

		routes = append(routes, models.GroundRoute{
			Provider: "finnlines",
			Type:     "ferry",
			Price:    price,
			Currency: "EUR",
			Duration: duration,
			Departure: models.GroundStop{
				City:    fromPort.City,
				Station: fromPort.Name + finnlinesShipSuffix(e.ShipName),
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

	slog.Debug("finnlines results", "routes", len(routes))
	return routes, nil
}

// finnlinesShipSuffix returns a ship name suffix for display.
func finnlinesShipSuffix(shipName string) string {
	if shipName == "" {
		return ""
	}
	// Capitalize nicely: "FINNCANOPUS" → "Finncanopus"
	name := strings.ToUpper(shipName[:1]) + strings.ToLower(shipName[1:])
	return " (" + name + ")"
}
