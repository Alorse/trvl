package flights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
)

const (
	duffelDefaultEndpoint = "https://api.duffel.com/air/offer_requests?return_offers=true&supplier_timeout=15000"
	duffelVersion         = "v2"
)

// duffelEndpoint is overridable in tests via duffelSetEndpointForTest.
var duffelEndpoint = duffelDefaultEndpoint

func duffelSetEndpointForTest(url string) func() {
	prev := duffelEndpoint
	duffelEndpoint = url
	return func() { duffelEndpoint = prev }
}

// DuffelEnabled reports whether at least one Duffel key is configured.
func DuffelEnabled() bool { return len(duffelKeys()) > 0 }

// DuffelSlice is one leg of a Duffel offer request. One slice = one-way; two =
// round-trip; N = multi-city, all priced as a single combined offer.
type DuffelSlice struct {
	Origin        string
	Destination   string
	DepartureDate string
}

// --- request types ---

type duffelReqSlice struct {
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
}

type duffelReqPassenger struct {
	Type string `json:"type"`
}

type duffelRequest struct {
	Data struct {
		Slices     []duffelReqSlice     `json:"slices"`
		Passengers []duffelReqPassenger `json:"passengers"`
		CabinClass string               `json:"cabin_class,omitempty"`
	} `json:"data"`
}

// --- response types ---

type duffelPlace struct {
	IATACode string `json:"iata_code"`
	Name     string `json:"name"`
}

type duffelBaggage struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
}

type duffelSegPassenger struct {
	Baggages []duffelBaggage `json:"baggages"`
}

type duffelSegment struct {
	Origin                       duffelPlace          `json:"origin"`
	Destination                  duffelPlace          `json:"destination"`
	DepartingAt                  string               `json:"departing_at"`
	ArrivingAt                   string               `json:"arriving_at"`
	Duration                     string               `json:"duration"`
	MarketingCarrier             duffelPlace          `json:"marketing_carrier"`
	MarketingCarrierFlightNumber string               `json:"marketing_carrier_flight_number"`
	Aircraft                     struct{ Name string } `json:"aircraft"`
	Passengers                   []duffelSegPassenger `json:"passengers"`
}

type duffelOfferSlice struct {
	Origin      duffelPlace     `json:"origin"`
	Destination duffelPlace     `json:"destination"`
	Duration    string          `json:"duration"`
	Segments    []duffelSegment `json:"segments"`
}

type duffelOffer struct {
	TotalAmount      string             `json:"total_amount"`
	TotalCurrency    string             `json:"total_currency"`
	TotalEmissionsKg string             `json:"total_emissions_kg"`
	Owner            duffelPlace        `json:"owner"`
	Slices           []duffelOfferSlice `json:"slices"`
}

type duffelResponse struct {
	Data struct {
		Offers []duffelOffer `json:"offers"`
	} `json:"data"`
}

// SearchDuffel queries the Duffel offer-request API for the given slices and
// maps each offer into a models.FlightResult. Keys are rotated round-robin and
// the search fails over to the next key on error. Requires at least one key
// (DUFFEL_API_KEYS or DUFFEL_API_KEY).
func SearchDuffel(ctx context.Context, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error) {
	opts.defaults()
	keys := duffelKeys()
	if len(keys) == 0 {
		return nil, fmt.Errorf("duffel: no API keys configured (set DUFFEL_API_KEYS or DUFFEL_API_KEY)")
	}
	if len(slices) == 0 {
		return nil, fmt.Errorf("duffel: at least one slice required")
	}

	var lastErr error
	for _, key := range duffelKeyOrder(keys) {
		results, err := searchDuffelOnce(ctx, key, slices, opts)
		if err != nil {
			lastErr = err
			slog.Warn("duffel key failed, failing over", "error", err)
			continue
		}
		return results, nil
	}
	return nil, fmt.Errorf("duffel: all keys exhausted: %w", lastErr)
}

// searchDuffelOnce performs a single Duffel offer-request with one API key.
func searchDuffelOnce(ctx context.Context, key string, slices []DuffelSlice, opts SearchOptions) ([]models.FlightResult, error) {
	var req duffelRequest
	for _, s := range slices {
		req.Data.Slices = append(req.Data.Slices, duffelReqSlice{
			Origin: s.Origin, Destination: s.Destination, DepartureDate: s.DepartureDate,
		})
	}
	for i := 0; i < opts.Adults; i++ {
		req.Data.Passengers = append(req.Data.Passengers, duffelReqPassenger{Type: "adult"})
	}
	req.Data.CabinClass = opts.CabinClass.String() // "economy", "business", ...

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("duffel: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, duffelEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("duffel: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("Duffel-Version", duffelVersion)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("duffel: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("duffel: unexpected status %d", resp.StatusCode)
	}

	var decoded duffelResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("duffel: decode response: %w", err)
	}

	results := make([]models.FlightResult, 0, len(decoded.Data.Offers))
	for _, o := range decoded.Data.Offers {
		results = append(results, mapDuffelOffer(o))
	}
	return results, nil
}

// mapDuffelOffer converts a single Duffel offer into a FlightResult. All
// segments across all slices are flattened into Legs; Stops is the total number
// of in-leg connections (segments minus one per slice). BookingURL is empty:
// Duffel exposes no public booking deep-link.
func mapDuffelOffer(o duffelOffer) models.FlightResult {
	price, _ := strconv.ParseFloat(o.TotalAmount, 64)

	var legs []models.FlightLeg
	totalDuration, stops := 0, 0
	var carryOn *bool
	var checked *int

	for _, sl := range o.Slices {
		totalDuration += parseISO8601Duration(sl.Duration)
		if len(sl.Segments) > 0 {
			stops += len(sl.Segments) - 1
		}
		for si, seg := range sl.Segments {
			layover := 0
			if si > 0 {
				prev := sl.Segments[si-1]
				layover = minutesBetween(prev.ArrivingAt, seg.DepartingAt)
			}
			legs = append(legs, models.FlightLeg{
				DepartureAirport: models.AirportInfo{Code: seg.Origin.IATACode, Name: seg.Origin.Name},
				ArrivalAirport:   models.AirportInfo{Code: seg.Destination.IATACode, Name: seg.Destination.Name},
				DepartureTime:    seg.DepartingAt,
				ArrivalTime:      seg.ArrivingAt,
				Duration:         parseISO8601Duration(seg.Duration),
				Airline:          seg.MarketingCarrier.Name,
				AirlineCode:      seg.MarketingCarrier.IATACode,
				FlightNumber:     seg.MarketingCarrierFlightNumber,
				Aircraft:         seg.Aircraft.Name,
				LayoverMinutes:   layover,
			})
			// Baggage from the first segment's first passenger (offer-wide).
			if carryOn == nil && len(seg.Passengers) > 0 {
				co, ck := duffelBaggageCounts(seg.Passengers[0].Baggages)
				carryOn, checked = co, ck
			}
		}
	}

	emissionsKg, _ := strconv.Atoi(o.TotalEmissionsKg)

	return models.FlightResult{
		Price:               price,
		Currency:            o.TotalCurrency,
		Duration:            totalDuration,
		Stops:               stops,
		Provider:            "duffel",
		Legs:                legs,
		BookingURL:          "",
		CarryOnIncluded:     carryOn,
		CheckedBagsIncluded: checked,
		Emissions:           emissionsKg * 1000,
	}
}

func duffelBaggageCounts(bags []duffelBaggage) (*bool, *int) {
	co := false
	ck := 0
	for _, b := range bags {
		switch b.Type {
		case "carry_on":
			if b.Quantity > 0 {
				co = true
			}
		case "checked":
			ck += b.Quantity
		}
	}
	return &co, &ck
}

// minutesBetween parses two RFC3339-ish timestamps (Duffel omits zone:
// "2006-01-02T15:04:05") and returns the gap in minutes, or 0 on parse failure.
func minutesBetween(a, b string) int {
	const layout = "2006-01-02T15:04:05"
	ta, err1 := time.Parse(layout, a)
	tb, err2 := time.Parse(layout, b)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := int(tb.Sub(ta).Minutes())
	if d < 0 {
		return 0
	}
	return d
}
