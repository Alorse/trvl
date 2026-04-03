package ground

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

const eurostarGateway = "https://site-api.eurostar.com/gateway"

// eurostarLimiter enforces Eurostar's aggressive rate limit: 3 req/min (conservative).
var eurostarLimiter = rate.NewLimiter(rate.Every(20*time.Second), 1)

// eurostarClient is a dedicated HTTP client for Eurostar API calls.
var eurostarClient = &http.Client{
	Timeout: 30 * time.Second,
}

// EurostarStation holds metadata for a Eurostar station.
type EurostarStation struct {
	UIC     string
	Name    string
	City    string
	Country string
}

// eurostarStations maps lowercase city name to station info.
var eurostarStations = map[string]EurostarStation{
	"london": {
		UIC: "7015400", Name: "London St Pancras", City: "London", Country: "GB",
	},
	"paris": {
		UIC: "8727100", Name: "Paris Gare du Nord", City: "Paris", Country: "FR",
	},
	"brussels": {
		UIC: "8814001", Name: "Brussels Midi", City: "Brussels", Country: "BE",
	},
	"amsterdam": {
		UIC: "8400058", Name: "Amsterdam Centraal", City: "Amsterdam", Country: "NL",
	},
	"rotterdam": {
		UIC: "8400530", Name: "Rotterdam Centraal", City: "Rotterdam", Country: "NL",
	},
	"cologne": {
		UIC: "8015458", Name: "Cologne Hbf", City: "Cologne", Country: "DE",
	},
	"lille": {
		UIC: "8722326", Name: "Lille Europe", City: "Lille", Country: "FR",
	},
}

// LookupEurostarStation resolves a city name to a Eurostar station (case-insensitive).
func LookupEurostarStation(city string) (EurostarStation, bool) {
	s, ok := eurostarStations[strings.ToLower(strings.TrimSpace(city))]
	return s, ok
}

// HasEurostarRoute returns true if both cities have Eurostar stations.
func HasEurostarRoute(from, to string) bool {
	_, fromOK := LookupEurostarStation(from)
	_, toOK := LookupEurostarStation(to)
	return fromOK && toOK
}

// eurostarGraphQLQuery builds the cheapestFaresSearch GraphQL query.
// If snapOnly is true, adds productFamiliesSearch: "SNAP" to filter for
// Eurostar Snap last-minute deals (released ~1 week before travel).
func eurostarGraphQLQuery(originUIC, destUIC, startDate, endDate, currency string, snapOnly bool) string {
	productFilter := ""
	if snapOnly {
		productFilter = `productFamiliesSearch: "SNAP"`
	}
	return fmt.Sprintf(`{
  cheapestFaresSearch(
    cheapestFaresLists: [{
      origin: %q
      destination: %q
      direction: OUTBOUND
      startDate: %q
      endDate: %q
    }]
    numberOfPassenger: 1
    mainPassengerType: { type: "ADULT" }
    %s
    currency: %s
  ) {
    cheapestFares { date price }
  }
}`, originUIC, destUIC, startDate, endDate, productFilter, strings.ToUpper(currency))
}

// eurostarGQLResponse is the expected GraphQL response structure.
type eurostarGQLResponse struct {
	Data struct {
		CheapestFaresSearch []struct {
			CheapestFares []struct {
				Date  string  `json:"date"`
				Price float64 `json:"price"`
			} `json:"cheapestFares"`
		} `json:"cheapestFaresSearch"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// SearchEurostar searches Eurostar for cheapest fares between two cities.
// from/to are city names (e.g. "London", "Paris"). startDate and endDate are YYYY-MM-DD.
// If snapOnly is true, filters for Eurostar Snap last-minute deals only.
func SearchEurostar(ctx context.Context, from, to, startDate, endDate, currency string, snapOnly bool) ([]models.GroundRoute, error) {
	fromStation, ok := LookupEurostarStation(from)
	if !ok {
		return nil, fmt.Errorf("no Eurostar station for %q", from)
	}
	toStation, ok := LookupEurostarStation(to)
	if !ok {
		return nil, fmt.Errorf("no Eurostar station for %q", to)
	}

	if currency == "" {
		currency = "GBP"
	}

	query := eurostarGraphQLQuery(fromStation.UIC, toStation.UIC, startDate, endDate, currency, snapOnly)
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, fmt.Errorf("eurostar marshal query: %w", err)
	}

	// Wait for rate limiter.
	if err := eurostarLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("eurostar rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, eurostarGateway, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.eurostar.com")
	req.Header.Set("Referer", "https://www.eurostar.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	slog.Debug("eurostar search", "from", fromStation.City, "to", toStation.City,
		"start", startDate, "end", endDate)

	resp, err := eurostarClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eurostar search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("eurostar search: HTTP %d: %s", resp.StatusCode, respBody)
	}

	var gqlResp eurostarGQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("eurostar decode: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("eurostar graphql: %s", gqlResp.Errors[0].Message)
	}

	var routes []models.GroundRoute
	for _, search := range gqlResp.Data.CheapestFaresSearch {
		for _, fare := range search.CheapestFares {
			if fare.Price <= 0 {
				continue
			}
			provider := "eurostar"
			if snapOnly {
				provider = "eurostar_snap"
			}
			route := models.GroundRoute{
				Provider: provider,
				Type:     "train",
				Price:    fare.Price,
				Currency: strings.ToUpper(currency),
				Departure: models.GroundStop{
					City:    fromStation.City,
					Station: fromStation.Name,
					Time:    fare.Date,
				},
				Arrival: models.GroundStop{
					City:    toStation.City,
					Station: toStation.Name,
					Time:    fare.Date,
				},
				BookingURL: buildEurostarBookingURL(fromStation.UIC, toStation.UIC, fare.Date),
			}
			routes = append(routes, route)
		}
	}

	return routes, nil
}

func buildEurostarBookingURL(originUIC, destUIC, date string) string {
	return fmt.Sprintf("https://www.eurostar.com/en/train-tickets?origin=%s&destination=%s&outbound=%s",
		url.QueryEscape(originUIC), url.QueryEscape(destUIC), url.QueryEscape(date))
}
