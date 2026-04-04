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

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/cookies"
	"github.com/MikkoParkkola/trvl/internal/models"
	"golang.org/x/time/rate"
)

const trainlineSearchURL = "https://www.trainline.eu/api/v5_1/search"

// trainlineLimiter: 5 req/min to be respectful
var trainlineLimiter = rate.NewLimiter(rate.Every(12*time.Second), 1)

// trainlineClient is a shared HTTP client for Trainline.
// Uses Chrome TLS fingerprint via utls to bypass Datadome bot detection.
var trainlineClient = batchexec.ChromeHTTPClient()

// trainlineSearchRequest is the JSON request body for Trainline search.
type trainlineSearchRequest struct {
	Search trainlineSearch `json:"search"`
}

type trainlineSearch struct {
	DepartureDate      string               `json:"departure_date"`
	ReturnDate         string               `json:"return_date,omitempty"`
	Passengers         []trainlinePassenger `json:"passengers"`
	Systems            []string             `json:"systems"`
	ExchangeablePart   interface{}          `json:"exchangeable_part"`
	Via                interface{}          `json:"via"`
	DepartureStationID string               `json:"departure_station_id"`
	ArrivalStationID   string               `json:"arrival_station_id"`
}

type trainlinePassenger struct {
	ID    string   `json:"id"`
	Age   int      `json:"age"`
	Cards []string `json:"cards"`
	Label string   `json:"label"`
}

// trainlineSearchResponse is the top-level API response.
type trainlineSearchResponse struct {
	Trips    []trainlineTrip              `json:"trips"`
	Segments []trainlineSegment           `json:"segments"`
	Stations map[string]trainlineStation  `json:"stations"`
	Folders  []trainlineFolder            `json:"folders"`
}

type trainlineTrip struct {
	ID            string   `json:"id"`
	SegmentIDs    []string `json:"segment_ids"`
	DepartureDate string   `json:"departure_date"`
	ArrivalDate   string   `json:"arrival_date"`
	Duration      int      `json:"duration"` // seconds
	FolderIDs     []string `json:"folder_ids"`
}

type trainlineSegment struct {
	ID                 string `json:"id"`
	DepartureDate      string `json:"departure_date"`
	ArrivalDate        string `json:"arrival_date"`
	DepartureStationID string `json:"departure_station_id"`
	ArrivalStationID   string `json:"arrival_station_id"`
	TransportationMean string `json:"transportation_mean"` // "train", "coach", "bus"
	Carrier            string `json:"carrier"`
	TrainNumber        string `json:"train_number"`
	TrainName          string `json:"train_name"`
}

type trainlineStation struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	City    string  `json:"city"`
	Country string  `json:"country"`
	Lat     float64 `json:"latitude"`
	Lon     float64 `json:"longitude"`
}

type trainlineFolder struct {
	ID               string   `json:"id"`
	TripIDs          []string `json:"trip_ids"`
	CentsProposition float64  `json:"cents_proposition"`
	Currency         string   `json:"currency"`
}

// trainlineStations maps city names to Trainline station IDs.
// Station IDs from: https://github.com/trainline-eu/stations
var trainlineStations = map[string]string{
	// IDs from trainline.eu/api/v5/stations (verified 2026-04-04)
	"london":     "8267",
	"paris":      "4916",
	"amsterdam":  "8657",
	"brussels":   "5893",
	"berlin":     "7527",
	"munich":     "7480",
	"frankfurt":  "7604",
	"hamburg":    "7626",
	"cologne":    "21178",
	"vienna":     "22644",
	"zurich":     "6401",
	"milan":      "8490",
	"rome":       "8544",
	"barcelona":  "6617",
	"madrid":     "6663",
	"prague":     "17587",
	"warsaw":     "10491",
	"budapest":   "18819",
	"copenhagen": "17515",
	"stockholm":  "38711",
	"rotterdam":  "23616",
	"lille":      "4652",
	"lyon":       "4718",
	"marseille":  "4790",
	"nice":       "4836",
	"strasbourg": "153",
	"toulouse":   "5306",
	"venice":     "8574",
	"florence":   "8434",
	"salzburg":   "6994",
	"innsbruck":  "10461",
	"geneva":     "5335",
	"basel":      "5877",
	"antwerp":    "5929",
}

// LookupTrainlineStation resolves a city name to a Trainline station ID.
func LookupTrainlineStation(city string) (string, bool) {
	id, ok := trainlineStations[strings.ToLower(strings.TrimSpace(city))]
	return id, ok
}

// HasTrainlineStation returns true if the city has a known Trainline station.
func HasTrainlineStation(city string) bool {
	_, ok := LookupTrainlineStation(city)
	return ok
}

// SearchTrainline searches Trainline for train connections between two cities.
func SearchTrainline(ctx context.Context, from, to, date, currency string) ([]models.GroundRoute, error) {
	fromID, ok := LookupTrainlineStation(from)
	if !ok {
		return nil, fmt.Errorf("no Trainline station for %q", from)
	}
	toID, ok := LookupTrainlineStation(to)
	if !ok {
		return nil, fmt.Errorf("no Trainline station for %q", to)
	}

	// Parse date to RFC3339 format for Trainline
	dateTime, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", date, err)
	}
	departureISO := dateTime.Add(6 * time.Hour).Format("2006-01-02T15:04:05+00:00")

	// Build raw request to ensure null fields are properly serialized.
	reqMap := map[string]any{
		"search": map[string]any{
			"departure_date":       departureISO,
			"departure_station_id": fromID,
			"arrival_station_id":   toID,
			"return_date":          nil,
			"exchangeable_part":    nil,
			"via":                  nil,
			"passengers": []map[string]any{
				{"id": "0", "age": 30, "cards": []any{}, "label": "adult"},
			},
		},
	}

	body, err := json.Marshal(reqMap)
	if err != nil {
		return nil, fmt.Errorf("trainline marshal: %w", err)
	}

	if err := trainlineLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("trainline rate limiter: %w", err)
	}

	// newTrainlineRequest builds a POST request with the standard Trainline headers.
	// cookieHeader is optional; pass "" to omit.
	newTrainlineRequest := func(cookieHeader string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, trainlineSearchURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		// Note: don't set Accept-Encoding manually — Go handles it automatically
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		req.Header.Set("Sec-Ch-Ua", `"Chromium";v="131", "Not_A Brand";v="24"`)
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Origin", "https://www.trainline.eu")
		req.Header.Set("Referer", "https://www.trainline.eu/")
		if cookieHeader != "" {
			req.Header.Set("Cookie", cookieHeader)
		}
		return req, nil
	}

	slog.Debug("trainline search", "from", from, "to", to, "date", date, "body", string(body))

	req, err := newTrainlineRequest("")
	if err != nil {
		return nil, err
	}

	resp, err := trainlineClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("trainline search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		firstBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()

		// Attempt retry with browser cookies.
		cookieHeader := cookies.BrowserCookies("trainline.eu")
		if cookieHeader != "" {
			slog.Debug("retrying trainline with browser cookies")
			req2, err2 := newTrainlineRequest(cookieHeader)
			if err2 != nil {
				return nil, fmt.Errorf("trainline retry build: %w", err2)
			}
			resp2, err2 := trainlineClient.Do(req2)
			if err2 != nil {
				return nil, fmt.Errorf("trainline retry: %w", err2)
			}
			defer resp2.Body.Close()
			if resp2.StatusCode == http.StatusOK {
				respBody, err3 := io.ReadAll(io.LimitReader(resp2.Body, 5*1024*1024))
				if err3 != nil {
					return nil, fmt.Errorf("trainline read: %w", err3)
				}
				var tlResp trainlineSearchResponse
				if err3 = json.Unmarshal(respBody, &tlResp); err3 != nil {
					return nil, fmt.Errorf("trainline decode: %w", err3)
				}
				return parseTrainlineResults(tlResp, from, to, currency)
			}
		}

		isCaptcha, captchaURL := cookies.IsCaptchaResponse(http.StatusForbidden, firstBody)
		if isCaptcha {
			slog.Warn("trainline requires browser verification", "captcha_url", captchaURL)
		}
		return nil, fmt.Errorf("trainline: HTTP 403: %s", firstBody)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("trainline: HTTP %d: %s", resp.StatusCode, respBody)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("trainline read: %w", err)
	}

	var tlResp trainlineSearchResponse
	if err := json.Unmarshal(respBody, &tlResp); err != nil {
		return nil, fmt.Errorf("trainline decode: %w", err)
	}

	return parseTrainlineResults(tlResp, from, to, currency)
}

func parseTrainlineResults(resp trainlineSearchResponse, from, to, currency string) ([]models.GroundRoute, error) {
	// Build segment lookup
	segmentMap := make(map[string]trainlineSegment)
	for _, s := range resp.Segments {
		segmentMap[s.ID] = s
	}

	// Build price lookup from folders
	tripPrices := make(map[string]float64)
	tripCurrencies := make(map[string]string)
	for _, f := range resp.Folders {
		for _, tripID := range f.TripIDs {
			price := f.CentsProposition / 100.0
			cur := strings.ToUpper(f.Currency)
			// Keep cheapest price per trip
			if existing, ok := tripPrices[tripID]; !ok || price < existing {
				tripPrices[tripID] = price
				tripCurrencies[tripID] = cur
			}
		}
	}

	var routes []models.GroundRoute
	for _, trip := range resp.Trips {
		price := tripPrices[trip.ID]
		cur := tripCurrencies[trip.ID]
		if cur == "" {
			cur = "EUR"
		}

		// Determine type from segments
		routeType := "train"
		carrier := ""
		trainNum := ""
		for _, segID := range trip.SegmentIDs {
			if seg, ok := segmentMap[segID]; ok {
				if seg.TransportationMean == "coach" || seg.TransportationMean == "bus" {
					routeType = "bus"
				}
				if carrier == "" {
					carrier = seg.Carrier
					trainNum = seg.TrainName
					if trainNum == "" {
						trainNum = seg.TrainNumber
					}
				}
			}
		}
		_ = carrier
		_ = trainNum

		// Parse times
		depTime := trip.DepartureDate
		arrTime := trip.ArrivalDate
		duration := trip.Duration / 60 // seconds to minutes

		transfers := len(trip.SegmentIDs) - 1
		if transfers < 0 {
			transfers = 0
		}

		route := models.GroundRoute{
			Provider:  "trainline",
			Type:      routeType,
			Price:     price,
			Currency:  cur,
			Duration:  duration,
			Transfers: transfers,
			Departure: models.GroundStop{
				City: from,
				Time: depTime,
			},
			Arrival: models.GroundStop{
				City: to,
				Time: arrTime,
			},
			BookingURL: fmt.Sprintf("https://www.trainline.eu/search/%s/%s/%s",
				strings.ReplaceAll(strings.ToLower(from), " ", "-"),
				strings.ReplaceAll(strings.ToLower(to), " ", "-"),
				depTime[:10]),
		}
		routes = append(routes, route)
	}

	slog.Debug("trainline results", "count", len(routes))
	return routes, nil
}
