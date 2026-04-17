package models

// PriceSource tracks which provider found a result at what price.
// When multiple providers return the same hotel, all sources are preserved
// and the lowest price becomes the primary HotelResult.Price.
type PriceSource struct {
	Provider   string  `json:"provider"` // "google_hotels", "trivago", "airbnb", "booking"
	Price      float64 `json:"price"`
	MaxPrice   float64 `json:"max_price,omitempty"`   // highest room/block price (when available)
	Currency   string  `json:"currency"`
	RoomCount  int     `json:"room_count,omitempty"`   // distinct room types (when available)
	BookingURL string  `json:"booking_url,omitempty"`
}

// Room represents a single bookable room type at a property.
type Room struct {
	Name     string  `json:"name"`               // e.g. "Standard Double Room", "Superior Suite"
	Price    float64 `json:"price,omitempty"`     // price for this room type
	Currency string  `json:"currency,omitempty"`
}

// HotelResult represents a single hotel from a search.
type HotelResult struct {
	Name         string        `json:"name"`
	HotelID      string        `json:"hotel_id"`
	Rating       float64       `json:"rating"`
	ReviewCount  int           `json:"review_count"`
	Stars        int           `json:"stars"`
	Price        float64       `json:"price"` // Lowest price across all sources
	Currency     string        `json:"currency"`
	Address      string        `json:"address"`
	Description  string        `json:"description,omitempty"`  // property tagline or summary
	ImageURL     string        `json:"image_url,omitempty"`    // main property image
	RoomTypes    []Room        `json:"room_types,omitempty"`   // available rooms with names and prices
	Lat          float64       `json:"lat"`
	Lon          float64       `json:"lon"`
	Neighborhood string        `json:"neighborhood,omitempty"`    // e.g. "Montmartre", "Le Marais"
	DistanceKm   float64       `json:"distance_km,omitempty"`     // km from city center
	Amenities    []string      `json:"amenities,omitempty"`
	BookingURL   string        `json:"booking_url,omitempty"`
	EcoCertified bool          `json:"eco_certified,omitempty"`
	Sources      []PriceSource `json:"sources,omitempty"` // All providers that found this hotel
}

// ProviderStatus reports the outcome of a single external provider query.
// Included in search responses so the orchestrating LLM can autonomously
// diagnose and fix broken providers.
type ProviderStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`           // "ok", "error", "disabled"
	Results int    `json:"results,omitempty"` // number of results returned
	Error   string `json:"error,omitempty"`   // error message if status != "ok"
	FixHint string `json:"fix_hint,omitempty"` // actionable hint for the LLM
}

// HotelSearchResult is the top-level response for a hotel search.
type HotelSearchResult struct {
	Success          bool             `json:"success"`
	Count            int              `json:"count"`
	TotalAvailable   int              `json:"total_available,omitempty"`
	Hotels           []HotelResult    `json:"hotels"`
	ProviderStatuses []ProviderStatus `json:"provider_statuses,omitempty"`
	Error            string           `json:"error,omitempty"`
}

// ProviderPrice represents a single booking provider's price for a hotel.
type ProviderPrice struct {
	Provider string  `json:"provider"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

// HotelPriceResult is the top-level response for a hotel price lookup.
type HotelPriceResult struct {
	Success   bool            `json:"success"`
	HotelID   string          `json:"hotel_id"`
	Name      string          `json:"name"`
	CheckIn   string          `json:"check_in"`
	CheckOut  string          `json:"check_out"`
	Providers []ProviderPrice `json:"providers"`
	Error     string          `json:"error,omitempty"`
}
