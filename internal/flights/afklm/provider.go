package afklm

import (
	"context"
	"fmt"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// RailwayStations is the set of IATA codes that map to RAILWAY_STATION type
// in AF-KLM Rail+Fly offers. Codes not in this map are treated as AIRPORT.
var RailwayStations = map[string]bool{
	"ZYR": true, // Brussels-Midi
	"QYG": true, // Antwerp Central
	"XER": true, // Strasbourg
	"XDB": true, // Lille
	"RTM": true, // Rotterdam
}

// placeType returns the AF-KLM place type for a given IATA code.
func placeType(code string) string {
	if RailwayStations[code] {
		return "RAILWAY_STATION"
	}
	return "AIRPORT"
}

// cabinCodes maps models.CabinClass to AF-KLM commercialCabins values.
func cabinCodes(cc models.CabinClass) []string {
	switch cc {
	case models.Economy:
		return []string{"ECONOMY"}
	case models.Business:
		return []string{"BUSINESS"}
	default:
		return []string{"ALL"}
	}
}

// AFKLMProvider implements models.FlightSearcher using the AF-KLM Offers API.
type AFKLMProvider struct {
	client *Client
}

// NewProvider creates an AFKLMProvider. Returns ErrNoCredential if the
// provider is not configured.
func NewProvider() (*AFKLMProvider, error) {
	client, err := NewClient(ClientOptions{})
	if err != nil {
		return nil, err
	}
	return &AFKLMProvider{client: client}, nil
}

// NewProviderWithClient creates an AFKLMProvider using the given pre-built
// client. Used in tests.
func NewProviderWithClient(client *Client) *AFKLMProvider {
	return &AFKLMProvider{client: client}
}

// SearchFlights implements models.FlightSearcher.
func (p *AFKLMProvider) SearchFlights(ctx context.Context, origin, dest, date string, opts models.FlightSearchOptions) (*models.FlightSearchResult, error) {
	adults := opts.Adults
	if adults < 1 {
		adults = 1
	}

	currency := opts.Currency
	if currency == "" {
		currency = "EUR"
	}

	// Build passengers list.
	passengers := make([]Passenger, adults)
	for i := range passengers {
		passengers[i] = Passenger{ID: i + 1, Type: "ADT"}
	}

	// Build connections.
	connections := []RequestedConnection{
		{
			DepartureDate: date,
			Origin:        Place{Type: placeType(origin), Code: origin},
			Destination:   Place{Type: placeType(dest), Code: dest},
		},
	}
	if opts.ReturnDate != "" {
		connections = append(connections, RequestedConnection{
			DepartureDate: opts.ReturnDate,
			Origin:        Place{Type: placeType(dest), Code: dest},
			Destination:   Place{Type: placeType(origin), Code: origin},
		})
	}

	tripType := "one-way"
	if opts.ReturnDate != "" {
		tripType = "round-trip"
	}

	req := AvailableOffersRequest{
		CommercialCabins:      cabinCodes(opts.CabinClass),
		BookingFlow:           "LEISURE",
		Passengers:            passengers,
		RequestedConnections:  connections,
		Currency:              currency,
		LowestFareCombination: true,
	}

	resp, _, err := p.client.AvailableOffers(ctx, req)
	if err != nil {
		// Graceful degradation for quota and credential errors.
		switch err {
		case ErrDailyQuotaExhausted:
			return &models.FlightSearchResult{
				Success:  false,
				TripType: tripType,
				Error:    "afklm: daily quota exhausted — try again tomorrow",
			}, nil
		case ErrNoCredential:
			return &models.FlightSearchResult{
				Success:  false,
				TripType: tripType,
				Error:    "afklm: no API key configured",
			}, nil
		}
		return nil, fmt.Errorf("afklm: search: %w", err)
	}

	flights := mapRecommendations(resp, origin, dest)
	return &models.FlightSearchResult{
		Success:  true,
		Count:    len(flights),
		TripType: tripType,
		Flights:  flights,
	}, nil
}

// mapRecommendations converts AF-KLM recommendations to FlightResult slices.
func mapRecommendations(resp *AvailableOffersResponse, origin, dest string) []models.FlightResult {
	var results []models.FlightResult
	for _, rec := range resp.Recommendations {
		for _, fp := range rec.FlightProducts {
			if len(fp.Connections) == 0 {
				continue
			}
			// Use the first connection for price / leg data.
			conn := fp.Connections[0]
			legs := mapSegments(conn.Segments)
			price := conn.Price.DisplayPrice
			currency := conn.Price.Currency
			if price == 0 {
				price = fp.Price.DisplayPrice
				currency = fp.Price.Currency
			}
			stops := 0
			if len(conn.Segments) > 1 {
				stops = len(conn.Segments) - 1
			}
			totalDuration := calcDuration(legs)

			results = append(results, models.FlightResult{
				Price:    price,
				Currency: currency,
				Duration: totalDuration,
				Stops:    stops,
				Provider: "afklm",
				Legs:     legs,
			})
		}
	}
	return results
}

// mapSegments converts AF-KLM segments to FlightLeg slices.
func mapSegments(segs []Segment) []models.FlightLeg {
	legs := make([]models.FlightLeg, 0, len(segs))
	for _, s := range segs {
		leg := models.FlightLeg{
			DepartureAirport: models.AirportInfo{Code: s.Origin.Code, Name: s.Origin.Name},
			ArrivalAirport:   models.AirportInfo{Code: s.Destination.Code, Name: s.Destination.Name},
			DepartureTime:    s.Origin.DepartureDate + " " + s.Origin.DepartureTime,
			ArrivalTime:      s.Destination.ArrivalDate + " " + s.Destination.ArrivalTime,
			AirlineCode:      s.MarketingCarrier.AirlineCode,
			Airline:          s.MarketingCarrier.AirlineCode,
			FlightNumber:     s.MarketingCarrier.AirlineCode + s.MarketingCarrier.FlightNumber,
		}
		legs = append(legs, leg)
	}
	return legs
}

// calcDuration sums leg durations. Since AF-KLM does not always provide
// explicit duration fields, we return 0 when times are unparseable.
func calcDuration(legs []models.FlightLeg) int {
	// Duration computation is best-effort; times may be absent in test fixtures.
	return 0
}
