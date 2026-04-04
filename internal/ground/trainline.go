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

const trainlineSearchURL = "https://www.trainline.eu/api/v5_1/search"

// trainlineLimiter: 5 req/min to be respectful
var trainlineLimiter = rate.NewLimiter(rate.Every(12*time.Second), 1)

// trainlineClient is a shared HTTP client for Trainline.
var trainlineClient = &http.Client{Timeout: 30 * time.Second}

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
	// Major European hubs
	"london":      "4916", // London St Pancras
	"paris":       "4718", // Paris (all stations)
	"amsterdam":   "5765", // Amsterdam Centraal
	"brussels":    "4717", // Brussels Midi/Zuid
	"berlin":      "4809", // Berlin Hbf
	"munich":      "4877", // München Hbf
	"frankfurt":   "4837", // Frankfurt Hbf
	"hamburg":     "4844", // Hamburg Hbf
	"cologne":     "4862", // Köln Hbf
	"vienna":      "4958", // Wien Hbf
	"zurich":      "378",  // Zürich HB
	"milan":       "5388", // Milano Centrale
	"rome":        "5409", // Roma Termini
	"barcelona":   "6263", // Barcelona Sants
	"madrid":      "6327", // Madrid Puerta de Atocha
	"prague":      "5011", // Praha hl.n.
	"warsaw":      "5156", // Warszawa Centralna
	"budapest":    "5037", // Budapest Keleti
	"copenhagen":  "5505", // København H
	"stockholm":   "5576", // Stockholm Central
	"rotterdam":   "5784", // Rotterdam Centraal
	"lille":       "4656", // Lille Europe
	"lyon":        "4670", // Lyon Part-Dieu
	"marseille":   "4683", // Marseille Saint-Charles
	"nice":        "4694", // Nice Ville
	"strasbourg":  "4736", // Strasbourg
	"toulouse":    "4745", // Toulouse Matabiau
	"venice":      "5465", // Venezia Santa Lucia
	"florence":    "5386", // Firenze S.M.N.
	"salzburg":    "4953", // Salzburg Hbf
	"innsbruck":   "4944", // Innsbruck Hbf
	"geneva":      "351",  // Genève
	"basel":       "312",  // Basel SBB
	"antwerp":     "4727", // Antwerpen Centraal
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

	reqBody := trainlineSearchRequest{
		Search: trainlineSearch{
			DepartureDate:      departureISO,
			DepartureStationID: fromID,
			ArrivalStationID:   toID,
			Passengers: []trainlinePassenger{
				{ID: "p1", Age: 30, Cards: []string{}, Label: "adult"},
			},
			Systems: []string{"sncf", "db", "trenitalia", "renfe", "busbud", "ntv", "westbahn", "ouigo", "thalys", "eurostar", "flixbus"},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("trainline marshal: %w", err)
	}

	if err := trainlineLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("trainline rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trainlineSearchURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CaptainTrain/1574360965(web) (Ember 3.5.1)")
	req.Header.Set("Host", "www.trainline.eu")

	slog.Debug("trainline search", "from", from, "to", to, "date", date)

	resp, err := trainlineClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("trainline search: %w", err)
	}
	defer resp.Body.Close()

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
