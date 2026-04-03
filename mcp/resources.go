package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/flights"
)

// registerResources adds all static resource definitions to the server.
func registerResources(s *Server) {
	s.resources = []ResourceDef{
		{
			URI:         "trvl://airports/popular",
			Name:        "Popular Airports",
			Description: "List of 50 popular airport codes with city names",
			MimeType:    "text/plain",
		},
		{
			URI:         "trvl://help/flights",
			Name:        "Flight Search Guide",
			Description: "Flight search usage guide with examples",
			MimeType:    "text/markdown",
		},
		{
			URI:         "trvl://help/hotels",
			Name:        "Hotel Search Guide",
			Description: "Hotel search usage guide with examples",
			MimeType:    "text/markdown",
		},
		{
			URI:         "trvl://trip/summary",
			Name:        "Trip Planning Summary",
			Description: "Accumulated summary of all searches in this session",
			MimeType:    "text/plain",
		},
	}
}

// listResources returns the static resources plus any dynamic watch resources
// created from flight searches in the session.
func (s *Server) listResources() []ResourceDef {
	// Start with static resources.
	resources := make([]ResourceDef, len(s.resources))
	copy(resources, s.resources)

	// Add dynamic watch resources from searches.
	s.tripState.mu.Lock()
	seen := make(map[string]bool)
	for _, sr := range s.tripState.Searches {
		if sr.Type == "flight" {
			// Extract origin-dest-date from query like "HEL->BCN 2026-07-01".
			uri := watchURIFromQuery(sr.Query)
			if uri != "" && !seen[uri] {
				seen[uri] = true
				resources = append(resources, ResourceDef{
					URI:         uri,
					Name:        fmt.Sprintf("Price watch: %s", sr.Query),
					Description: "Re-fetch to check for price changes",
					MimeType:    "text/plain",
				})
			}
		}
	}
	s.tripState.mu.Unlock()

	return resources
}

// watchURIFromQuery converts a query like "HEL->BCN 2026-07-01" to
// "trvl://watch/HEL-BCN-2026-07-01".
func watchURIFromQuery(query string) string {
	// Expected format: "HEL->BCN 2026-07-01" or "HEL->BCN 2026-07-01 (round-trip ...)"
	parts := strings.Fields(query)
	if len(parts) < 2 {
		return ""
	}
	route := parts[0] // "HEL->BCN"
	date := parts[1]  // "2026-07-01"

	routeParts := strings.SplitN(route, "->", 2)
	if len(routeParts) != 2 {
		return ""
	}
	origin := routeParts[0]
	dest := routeParts[1]

	return fmt.Sprintf("trvl://watch/%s-%s-%s", origin, dest, date)
}

// readResource returns the content for a resource URI, including dynamic resources.
func (s *Server) readResource(uri string) (*ResourcesReadResult, error) {
	switch {
	case uri == "trvl://airports/popular":
		return &ResourcesReadResult{
			Contents: []ResourceContent{{
				URI:      uri,
				MimeType: "text/plain",
				Text:     popularAirports,
			}},
		}, nil
	case uri == "trvl://help/flights":
		return &ResourcesReadResult{
			Contents: []ResourceContent{{
				URI:      uri,
				MimeType: "text/markdown",
				Text:     flightSearchGuide,
			}},
		}, nil
	case uri == "trvl://help/hotels":
		return &ResourcesReadResult{
			Contents: []ResourceContent{{
				URI:      uri,
				MimeType: "text/markdown",
				Text:     hotelSearchGuide,
			}},
		}, nil
	case uri == "trvl://trip/summary":
		return s.readTripSummary()
	case strings.HasPrefix(uri, "trvl://watch/"):
		return s.readWatchResource(uri)
	default:
		return nil, fmt.Errorf("resource not found: %s", uri)
	}
}

// readTripSummary returns a formatted summary of all searches in the session.
func (s *Server) readTripSummary() (*ResourcesReadResult, error) {
	s.tripState.mu.Lock()
	searches := make([]SearchRecord, len(s.tripState.Searches))
	copy(searches, s.tripState.Searches)
	s.tripState.mu.Unlock()

	if len(searches) == 0 {
		return &ResourcesReadResult{
			Contents: []ResourceContent{{
				URI:      "trvl://trip/summary",
				MimeType: "text/plain",
				Text:     "Trip Planning Session Summary\n\nNo searches yet. Use search_flights, search_hotels, or destination_info to start planning.",
			}},
		}, nil
	}

	// Count by type.
	counts := make(map[string]int)
	for _, sr := range searches {
		counts[sr.Type]++
	}

	var b strings.Builder
	b.WriteString("Trip Planning Session Summary\n")
	b.WriteString(strings.Repeat("=", 40))
	b.WriteString("\n")

	// Searched line.
	var parts []string
	if n := counts["flight"]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d flight(s)", n))
	}
	if n := counts["hotel"]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d hotel(s)", n))
	}
	if n := counts["destination"]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d destination(s)", n))
	}
	b.WriteString(fmt.Sprintf("Searched: %s\n\n", strings.Join(parts, ", ")))

	// Individual searches.
	var totalCost float64
	var currency string
	for _, sr := range searches {
		icon := "  "
		switch sr.Type {
		case "flight":
			icon = ">> "
		case "hotel":
			icon = "## "
		case "destination":
			icon = "** "
		}
		if sr.BestPrice > 0 {
			b.WriteString(fmt.Sprintf("%s%s: cheapest %s %.0f\n", icon, sr.Query, sr.Currency, sr.BestPrice))
			totalCost += sr.BestPrice
			if currency == "" {
				currency = sr.Currency
			}
		} else {
			b.WriteString(fmt.Sprintf("%s%s\n", icon, sr.Query))
		}
	}

	if totalCost > 0 {
		b.WriteString(fmt.Sprintf("\nEstimated total: %s %.0f", currency, totalCost))
	}

	return &ResourcesReadResult{
		Contents: []ResourceContent{{
			URI:      "trvl://trip/summary",
			MimeType: "text/plain",
			Text:     b.String(),
		}},
	}, nil
}

// readWatchResource handles trvl://watch/{origin}-{dest}-{date} URIs.
// It runs a fresh cheapest-flight search and returns the price with delta.
func (s *Server) readWatchResource(uri string) (*ResourcesReadResult, error) {
	// Parse: "trvl://watch/HEL-BCN-2026-07-01"
	path := strings.TrimPrefix(uri, "trvl://watch/")
	parts := strings.SplitN(path, "-", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, fmt.Errorf("invalid watch URI: %s (expected trvl://watch/ORIGIN-DEST-YYYY-MM-DD)", uri)
	}
	origin := strings.ToUpper(parts[0])
	dest := strings.ToUpper(parts[1])
	date := parts[2]

	// Run a quick search for the cheapest flight.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := flights.SearchOptions{}
	result, err := flights.SearchFlights(ctx, origin, dest, date, opts)
	if err != nil {
		return nil, fmt.Errorf("watch search failed: %w", err)
	}

	cacheKey := fmt.Sprintf("%s-%s-%s", origin, dest, date)

	if !result.Success || result.Count == 0 {
		return &ResourcesReadResult{
			Contents: []ResourceContent{{
				URI:      uri,
				MimeType: "text/plain",
				Text:     fmt.Sprintf("No flights found for %s -> %s on %s.", origin, dest, date),
			}},
		}, nil
	}

	// Find cheapest.
	cheapest := result.Flights[0]
	for _, f := range result.Flights[1:] {
		if f.Price > 0 && f.Price < cheapest.Price {
			cheapest = f
		}
	}

	// Check delta from cached price.
	var text string
	if prev, ok := s.priceCache.get(cacheKey); ok {
		delta := cheapest.Price - prev
		direction := "unchanged"
		if delta > 0 {
			direction = fmt.Sprintf("up %.0f", delta)
		} else if delta < 0 {
			direction = fmt.Sprintf("down %.0f", -delta)
		}
		text = fmt.Sprintf("%s -> %s on %s: %s%.0f (%s from previous %s%.0f)",
			origin, dest, date, cheapest.Currency, cheapest.Price,
			direction, cheapest.Currency, prev)
	} else {
		text = fmt.Sprintf("%s -> %s on %s: %s%.0f (first check)",
			origin, dest, date, cheapest.Currency, cheapest.Price)
	}

	// Update cache.
	s.priceCache.set(cacheKey, cheapest.Price)

	return &ResourcesReadResult{
		Contents: []ResourceContent{{
			URI:      uri,
			MimeType: "text/plain",
			Text:     text,
		}},
	}, nil
}

const popularAirports = `HEL - Helsinki, Finland
JFK - New York (John F. Kennedy), USA
LHR - London Heathrow, UK
NRT - Tokyo Narita, Japan
CDG - Paris Charles de Gaulle, France
LAX - Los Angeles, USA
SIN - Singapore Changi, Singapore
DXB - Dubai, UAE
FRA - Frankfurt, Germany
AMS - Amsterdam Schiphol, Netherlands
HND - Tokyo Haneda, Japan
ICN - Seoul Incheon, South Korea
SFO - San Francisco, USA
ORD - Chicago O'Hare, USA
BKK - Bangkok Suvarnabhumi, Thailand
IST - Istanbul, Turkey
MUC - Munich, Germany
FCO - Rome Fiumicino, Italy
MAD - Madrid Barajas, Spain
BCN - Barcelona El Prat, Spain
ZRH - Zurich, Switzerland
HKG - Hong Kong, China
SYD - Sydney Kingsford Smith, Australia
MIA - Miami, USA
EWR - Newark Liberty, USA
ARN - Stockholm Arlanda, Sweden
OSL - Oslo Gardermoen, Norway
CPH - Copenhagen, Denmark
LIS - Lisbon, Portugal
VIE - Vienna, Austria
ATL - Atlanta Hartsfield-Jackson, USA
DEN - Denver, USA
SEA - Seattle-Tacoma, USA
BOS - Boston Logan, USA
DOH - Doha Hamad, Qatar
DEL - Delhi Indira Gandhi, India
BOM - Mumbai, India
PEK - Beijing Capital, China
PVG - Shanghai Pudong, China
KUL - Kuala Lumpur, Malaysia
MEX - Mexico City, Mexico
GRU - Sao Paulo Guarulhos, Brazil
JNB - Johannesburg, South Africa
CAI - Cairo, Egypt
DUS - Dusseldorf, Germany
HAM - Hamburg, Germany
MXP - Milan Malpensa, Italy
PMI - Palma de Mallorca, Spain
TLV - Tel Aviv Ben Gurion, Israel
AKL - Auckland, New Zealand`

const flightSearchGuide = `# Flight Search Guide

## search_flights

Search for flights on a specific date. Returns real-time pricing from Google Flights.

### Required Parameters
- **origin**: IATA airport code (e.g., HEL, JFK, NRT)
- **destination**: IATA airport code
- **departure_date**: Date in YYYY-MM-DD format

### Optional Parameters
- **return_date**: For round-trip searches (YYYY-MM-DD)
- **cabin_class**: economy (default), premium_economy, business, first
- **max_stops**: any (default), nonstop, one_stop, two_plus
- **sort_by**: cheapest (default), duration, departure, arrival

### Examples

**One-way flight:**
` + "```json" + `
{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15"}
` + "```" + `

**Round-trip flight:**
` + "```json" + `
{"origin": "HEL", "destination": "NRT", "departure_date": "2026-06-15", "return_date": "2026-06-22"}
` + "```" + `

**Nonstop business class:**
` + "```json" + `
{"origin": "JFK", "destination": "LHR", "departure_date": "2026-06-15", "cabin_class": "business", "max_stops": "nonstop"}
` + "```" + `

## search_dates

Find the cheapest flight prices across a date range.

### Required Parameters
- **origin**: IATA airport code
- **destination**: IATA airport code
- **start_date**: Start of range (YYYY-MM-DD)
- **end_date**: End of range (YYYY-MM-DD)

### Optional Parameters
- **trip_duration**: Days for round-trip (e.g., 7)
- **is_round_trip**: true/false (default: false)

### Examples

**Cheapest one-way dates in June:**
` + "```json" + `
{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30"}
` + "```" + `

**Cheapest round-trip week in June:**
` + "```json" + `
{"origin": "HEL", "destination": "NRT", "start_date": "2026-06-01", "end_date": "2026-06-30", "is_round_trip": true, "trip_duration": 7}
` + "```" + `

## Tips

- Use **search_dates** first to find the cheapest day, then **search_flights** for full details on that day
- Airport codes are case-insensitive (hel = HEL)
- Prices are real-time from Google Flights and may change
- Round-trip searches often show lower per-leg prices than one-way
`

const hotelSearchGuide = `# Hotel Search Guide

## search_hotels

Search for hotels in a location. Returns real-time pricing from Google Hotels.

### Required Parameters
- **location**: City name, neighborhood, or address (e.g., "Helsinki", "Shibuya Tokyo", "Manhattan New York")
- **check_in**: Check-in date (YYYY-MM-DD)
- **check_out**: Check-out date (YYYY-MM-DD)

### Optional Parameters
- **guests**: Number of guests (default: 2)
- **stars**: Minimum star rating 1-5 (default: no filter)
- **sort**: relevance (default), price, rating, distance

### Examples

**Basic hotel search:**
` + "```json" + `
{"location": "Helsinki", "check_in": "2026-06-15", "check_out": "2026-06-18"}
` + "```" + `

**4+ star hotels sorted by price:**
` + "```json" + `
{"location": "Tokyo Shinjuku", "check_in": "2026-06-15", "check_out": "2026-06-22", "stars": 4, "sort": "price"}
` + "```" + `

## hotel_prices

Compare prices across booking providers for a specific hotel.

### Required Parameters
- **hotel_id**: Google Hotels property ID (from search_hotels results)
- **check_in**: Check-in date (YYYY-MM-DD)
- **check_out**: Check-out date (YYYY-MM-DD)

### Example

` + "```json" + `
{"hotel_id": "/g/11b6d4_v_4", "check_in": "2026-06-15", "check_out": "2026-06-18"}
` + "```" + `

## Tips

- Use **search_hotels** to find options, then **hotel_prices** to compare booking providers for the best deal
- Location can be specific ("Shibuya Tokyo") or general ("Tokyo")
- Prices shown are per night
- The hotel_id from search results is needed for price comparison
- Star ratings and guest ratings are different: stars = hotel class, rating = guest reviews out of 5
`
