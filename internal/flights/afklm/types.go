package afklm

// AvailableOffersRequest is the request body for POST /opendata/offers/v3/available-offers.
type AvailableOffersRequest struct {
	CommercialCabins      []string               `json:"commercialCabins,omitempty"`
	BookingFlow           string                 `json:"bookingFlow"`
	Passengers            []Passenger            `json:"passengers"`
	RequestedConnections  []RequestedConnection  `json:"requestedConnections"`
	Currency              string                 `json:"currency,omitempty"`
	FareOption            string                 `json:"fareOption,omitempty"`
	WithUpsellCabins      bool                   `json:"withUpsellCabins,omitempty"`
	LowestFareCombination bool                   `json:"lowestFareCombination,omitempty"`
}

// Passenger represents a traveller in the request.
type Passenger struct {
	ID   int    `json:"id"`
	Type string `json:"type"` // ADT = adult
}

// RequestedConnection represents one leg of the journey.
type RequestedConnection struct {
	DepartureDate string `json:"departureDate"` // ISO calendar date "2006-01-02"
	Origin        Place  `json:"origin"`
	Destination   Place  `json:"destination"`
}

// Place identifies an airport, railway station, city, etc.
type Place struct {
	Type string `json:"type"` // AIRPORT, RAILWAY_STATION, CITY, STOPOVER, etc.
	Code string `json:"code"` // IATA code
}

// AvailableOffersResponse is the top-level response from available-offers.
type AvailableOffersResponse struct {
	Recommendations []Recommendation `json:"recommendations"`
}

// Recommendation is a priced bookable bundle.
type Recommendation struct {
	FlightProducts []FlightProduct `json:"flightProducts"`
}

// FlightProduct is a priced itinerary option within a recommendation.
type FlightProduct struct {
	Connections []Connection `json:"connections"`
	Price       Price        `json:"price"`
}

// Connection represents one bound (e.g. outbound or return).
type Connection struct {
	CommercialCabinLabel   string     `json:"commercialCabinLabel"`
	NumberOfSeatsAvailable int        `json:"numberOfSeatsAvailable"`
	FareFamily             FareFamily `json:"fareFamily"`
	Segments               []Segment  `json:"segments"`
	Price                  Price      `json:"price"`
}

// FareFamily identifies the fare product family.
type FareFamily struct {
	Code string `json:"code"`
}

// Segment is a single flight segment within a connection.
type Segment struct {
	Origin      SegmentPlace `json:"origin"`
	Destination SegmentPlace `json:"destination"`
	// MarketingCarrier contains the airline code and flight number.
	MarketingCarrier SegmentCarrier `json:"marketingCarrier"`
	// OperatingCarrier is the operator (may differ from marketing carrier).
	OperatingCarrier SegmentCarrier `json:"operatingCarrier"`
}

// SegmentPlace is a departure or arrival point in a segment.
type SegmentPlace struct {
	Code          string `json:"code"`
	Name          string `json:"name,omitempty"`
	DepartureDate string `json:"departureDate,omitempty"` // ISO date
	DepartureTime string `json:"departureTime,omitempty"` // HH:MM
	ArrivalDate   string `json:"arrivalDate,omitempty"`
	ArrivalTime   string `json:"arrivalTime,omitempty"`
}

// SegmentCarrier holds carrier code and flight number.
type SegmentCarrier struct {
	AirlineCode  string `json:"airlineCode"`
	FlightNumber string `json:"flightNumber"`
}

// Price contains fare amounts in the requested currency.
type Price struct {
	DisplayPrice float64 `json:"displayPrice"`
	TotalPrice   float64 `json:"totalPrice"`
	Currency     string  `json:"currency"`
}

// LowestFaresRequest is the request body for POST /opendata/offers/v3/lowest-fares-by-destination.
type LowestFaresRequest struct {
	BookingFlow        string `json:"bookingFlow"`
	DestinationCities  string `json:"destinationCities"` // comma-separated, embedded quotes per API quirk
	Type               string `json:"type"`              // "DAY"
	FromDate           string `json:"fromDate"`          // RFC3339
	UntilDate          string `json:"untilDate"`         // RFC3339
	Origin             Place  `json:"origin"`
}

// LowestFaresResponse is the response from lowest-fares-by-destination.
type LowestFaresResponse struct {
	DestinationFares []DestinationFare `json:"destinationFares"`
}

// DestinationFare is the cheapest fare found for a single destination.
type DestinationFare struct {
	Destination Place   `json:"destination"`
	Price       Price   `json:"price"`
	DepartDate  string  `json:"departDate,omitempty"`
}
