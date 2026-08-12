package models

import (
	"fmt"
	"strings"
)

// AirportInfo identifies an airport by IATA code and name.
type AirportInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// FlightLeg represents a single segment of a flight itinerary.
type FlightLeg struct {
	DepartureAirport AirportInfo `json:"departure_airport"`
	ArrivalAirport   AirportInfo `json:"arrival_airport"`
	DepartureTime    string      `json:"departure_time"`
	ArrivalTime      string      `json:"arrival_time"`
	Duration         int         `json:"duration"` // minutes
	Airline          string      `json:"airline"`
	AirlineCode      string      `json:"airline_code"`
	FlightNumber     string      `json:"flight_number"`
	Aircraft         string      `json:"aircraft,omitempty"`        // e.g. "Airbus A350"
	LayoverMinutes   int         `json:"layover_minutes,omitempty"` // time between arrival of previous leg and this departure (0 for first leg)
}

// FlightResult represents a single flight option with price and routing.
type FlightResult struct {
	Price               float64     `json:"price"`
	Currency            string      `json:"currency"`
	Duration            int         `json:"duration"` // total minutes
	Stops               int         `json:"stops"`
	Provider            string      `json:"provider,omitempty"`
	SelfConnect         bool        `json:"self_connect,omitempty"`
	Warnings            []string    `json:"warnings,omitempty"`
	Legs                []FlightLeg `json:"legs"`
	BookingURL          string      `json:"booking_url,omitempty"`
	CarryOnIncluded     *bool       `json:"carry_on_included,omitempty"`     // true if carry-on bag is included in price
	CheckedBagsIncluded *int        `json:"checked_bags_included,omitempty"` // 0=not included, 1=one bag, 2=two bags
	Emissions           int         `json:"emissions,omitempty"`             // estimated CO2 in grams; 0 if unavailable
	// BagEstimate resolves the checked-bag situation from the best evidence
	// available and records which evidence that was. Nil when not computed.
	BagEstimate *BagEstimate `json:"bag_estimate,omitempty"`
	// AllInMin/AllInMax bound the fare plus what a checked bag would add, in
	// Currency. A range rather than a point because published bag fees swing
	// several-fold within one carrier; sort on the floor, budget for the
	// ceiling. Both stay zero when no total can be stated honestly — the
	// airline's terms are unknown, or its fee is quoted in another currency
	// and we hold no rate to convert it. Zero therefore means "no total",
	// never "free".
	AllInMin float64 `json:"all_in_min,omitempty"`
	AllInMax float64 `json:"all_in_max,omitempty"`
}

// FlightSearchResult is the top-level response for a flight search.
type FlightSearchResult struct {
	Success  bool           `json:"success"`
	Count    int            `json:"count"`
	TripType string         `json:"trip_type"`
	Flights  []FlightResult `json:"flights"`
	Error    string         `json:"error,omitempty"`
}

// DatePriceResult represents the cheapest price for a single departure date.
type DatePriceResult struct {
	Date       string  `json:"date"`
	Price      float64 `json:"price"`
	Currency   string  `json:"currency"`
	ReturnDate string  `json:"return_date,omitempty"`
}

// DateSearchResult is the top-level response for a date range price search.
type DateSearchResult struct {
	Success   bool              `json:"success"`
	Count     int               `json:"count"`
	TripType  string            `json:"trip_type"`
	DateRange string            `json:"date_range"`
	Dates     []DatePriceResult `json:"dates"`
	Error     string            `json:"error,omitempty"`
}

// CabinClass represents the cabin/service class for a flight.
type CabinClass int

const (
	Economy        CabinClass = 1
	PremiumEconomy CabinClass = 2
	Business       CabinClass = 3
	First          CabinClass = 4
)

// String returns the human-readable name of the cabin class.
func (c CabinClass) String() string {
	switch c {
	case Economy:
		return "economy"
	case PremiumEconomy:
		return "premium_economy"
	case Business:
		return "business"
	case First:
		return "first"
	default:
		return "economy"
	}
}

// ParseCabinClass converts a string to a CabinClass. Case-insensitive.
func ParseCabinClass(s string) (CabinClass, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "economy", "e", "1":
		return Economy, nil
	case "premium_economy", "premium-economy", "premiumeconomy", "pe", "2":
		return PremiumEconomy, nil
	case "business", "b", "3":
		return Business, nil
	case "first", "f", "4":
		return First, nil
	default:
		return Economy, fmt.Errorf("unknown cabin class: %q", s)
	}
}

// MaxStops constrains the number of stops in a flight search.
type MaxStops int

const (
	AnyStops     MaxStops = 0
	NonStop      MaxStops = 1
	OneStop      MaxStops = 2
	TwoPlusStops MaxStops = 3
)

// String returns the human-readable name of the stop filter.
func (m MaxStops) String() string {
	switch m {
	case AnyStops:
		return "any"
	case NonStop:
		return "nonstop"
	case OneStop:
		return "one_stop"
	case TwoPlusStops:
		return "two_plus"
	default:
		return "any"
	}
}

// ParseMaxStops converts a string to a MaxStops value. Case-insensitive.
func ParseMaxStops(s string) (MaxStops, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "any", "0", "":
		return AnyStops, nil
	case "nonstop", "non_stop", "non-stop", "1":
		return NonStop, nil
	case "one_stop", "one-stop", "onestop", "2":
		return OneStop, nil
	case "two_plus", "two-plus", "twoplus", "3":
		return TwoPlusStops, nil
	default:
		return AnyStops, fmt.Errorf("unknown max stops: %q", s)
	}
}

// SortBy controls the ordering of flight search results.
type SortBy int

const (
	SortCheapest      SortBy = 0
	SortDuration      SortBy = 1
	SortDepartureTime SortBy = 2
	SortArrivalTime   SortBy = 3
)

// String returns the human-readable name of the sort order.
func (s SortBy) String() string {
	switch s {
	case SortCheapest:
		return "cheapest"
	case SortDuration:
		return "duration"
	case SortDepartureTime:
		return "departure"
	case SortArrivalTime:
		return "arrival"
	default:
		return "cheapest"
	}
}

// ParseSortBy converts a string to a SortBy value. Case-insensitive.
func ParseSortBy(s string) (SortBy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "cheapest", "price", "0", "":
		return SortCheapest, nil
	case "duration", "time", "1":
		return SortDuration, nil
	case "departure", "departure_time", "depart", "2":
		return SortDepartureTime, nil
	case "arrival", "arrival_time", "arrive", "3":
		return SortArrivalTime, nil
	default:
		return SortCheapest, fmt.Errorf("unknown sort order: %q", s)
	}
}

// BagSource names where a checked-bag verdict came from, so a consumer can
// decide how far to trust it. This travels in the flight JSON: trvl estimates
// trip cost rather than selling tickets, and an estimate is only useful when
// its provenance is visible.
type BagSource string

const (
	// BagSourceProvider — the flight provider stated the allowance in its own
	// payload (Duffel always; Google/SerpApi on routes where it publishes the
	// figure). Hard data.
	BagSourceProvider BagSource = "provider"
	// BagSourceTableSourced — trvl's airline table, where the figure carries a
	// primary-source URL and a verification date.
	BagSourceTableSourced BagSource = "table_sourced"
	// BagSourceTableUnsourced — trvl's airline table, where the figure has no
	// citation behind it. A hint, not a number to rely on.
	BagSourceTableUnsourced BagSource = "table_unsourced"
	// BagSourceFrequentFlyer — the traveller's alliance status entitles them to
	// a free checked bag regardless of the fare.
	BagSourceFrequentFlyer BagSource = "frequent_flyer"
	// BagSourceUnknown — no source covers this airline. Treated as "no free
	// checked bag" so a bag-required filter cannot pass it, but no fee is
	// invented.
	BagSourceUnknown BagSource = "unknown"
)

// BagEstimate is a checked-bag verdict plus the provenance of that verdict.
//
// AmountMin/AmountMax bound what a first checked bag costs when one is not
// included. They are a published range, not a point: route, booking channel,
// timing and weight tier move real fees several-fold within a single carrier,
// so one number would be wrong nearly always. Both stay zero when no figure is
// available — including for carriers that publish none at all, where Reference
// says so instead.
//
// Source describes the INCLUSION verdict, not the fee. A provider that states
// "no free bag" is hard data even when the fee attached to it is an estimate;
// the fee's own provenance travels in Reference and Verified.
type BagEstimate struct {
	Included  bool      `json:"included"`
	Source    BagSource `json:"source"`
	AmountMin float64   `json:"amount_min,omitempty"`
	AmountMax float64   `json:"amount_max,omitempty"`
	Currency  string    `json:"currency,omitempty"`
	Reference string    `json:"reference,omitempty"` // where the figure came from, or why there is none
	Verified  string    `json:"verified,omitempty"`  // YYYY-MM the reference was checked
}
