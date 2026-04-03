package hotels

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

var (
	defaultClient     *batchexec.Client
	defaultClientOnce sync.Once
)

// DefaultClient returns a shared batchexec.Client for the hotels package.
// The client is created once and reused across all requests, enabling
// connection reuse and shared rate limiting.
func DefaultClient() *batchexec.Client {
	defaultClientOnce.Do(func() {
		defaultClient = batchexec.NewClient()
	})
	return defaultClient
}

// HotelSearchOptions configures a hotel search.
type HotelSearchOptions struct {
	CheckIn  string // YYYY-MM-DD
	CheckOut string // YYYY-MM-DD
	Guests   int
	Stars    int    // 0 = any, 2-5 filter
	Sort     string // "cheapest", "rating", "distance", "stars"
	Currency string // default "USD"

	// Post-fetch filters.
	MinPrice      float64 // minimum price per night (0 = no filter)
	MaxPrice      float64 // maximum price per night (0 = no filter)
	MinRating     float64 // minimum guest rating, e.g. 4.0 (0 = no filter)
	MaxDistanceKm float64 // max km from city center (0 = no filter)
	CenterLat     float64 // city center latitude (resolved automatically if 0)
	CenterLon     float64 // city center longitude (resolved automatically if 0)
}

// SearchHotels searches for hotels in the given location.
//
// The location can be a city name, address, or any text that Google Travel
// accepts as a destination query. We fetch the Google Travel Hotels page
// directly and parse the embedded JSON data from AF_initDataCallback blocks.
func SearchHotels(ctx context.Context, location string, opts HotelSearchOptions) (*models.HotelSearchResult, error) {
	if opts.CheckIn == "" || opts.CheckOut == "" {
		return nil, fmt.Errorf("check-in and check-out dates are required")
	}
	if opts.Guests <= 0 {
		opts.Guests = 2
	}
	if opts.Currency == "" {
		opts.Currency = "USD"
	}

	// Validate dates.
	_, err := parseDateArray(opts.CheckIn)
	if err != nil {
		return nil, fmt.Errorf("parse check-in date: %w", err)
	}
	_, err = parseDateArray(opts.CheckOut)
	if err != nil {
		return nil, fmt.Errorf("parse check-out date: %w", err)
	}

	client := DefaultClient()

	// Build the Google Travel Hotels URL.
	travelURL := buildTravelURL(location, opts)

	status, body, err := client.Get(ctx, travelURL)
	if err != nil {
		return nil, fmt.Errorf("hotel search request: %w", err)
	}

	if status == 403 {
		return nil, batchexec.ErrBlocked
	}
	if status != 200 {
		return nil, fmt.Errorf("hotel search returned status %d", status)
	}
	if len(body) < 1000 {
		return nil, fmt.Errorf("hotel search returned empty response")
	}

	// Parse hotel data from the page's AF_initDataCallback blocks.
	hotels, err := parseHotelsFromPage(string(body), opts.Currency)
	if err != nil {
		return nil, fmt.Errorf("parse hotel results: %w", err)
	}

	// Resolve city center for distance filter/sort if needed.
	if opts.MaxDistanceKm > 0 || strings.EqualFold(opts.Sort, "distance") {
		if opts.CenterLat == 0 && opts.CenterLon == 0 {
			lat, lon, err := ResolveLocation(ctx, location)
			if err == nil {
				opts.CenterLat = lat
				opts.CenterLon = lon
			}
		}
	}

	// Apply post-filters.
	hotels = filterHotels(hotels, opts)

	// Sort results.
	sortHotels(hotels, opts.Sort, opts.CenterLat, opts.CenterLon)

	// Add booking URLs to each hotel.
	for i := range hotels {
		hotels[i].BookingURL = buildHotelBookingURL(location, opts.CheckIn, opts.CheckOut)
	}

	return &models.HotelSearchResult{
		Success: true,
		Count:   len(hotels),
		Hotels:  hotels,
	}, nil
}

// buildHotelBookingURL constructs a Google Hotels deep link for a location and dates.
func buildHotelBookingURL(location, checkIn, checkOut string) string {
	encoded := url.PathEscape(location)
	return fmt.Sprintf("https://www.google.com/travel/hotels/%s?q=%s+hotels&dates=%s,%s",
		encoded, url.QueryEscape(location), checkIn, checkOut)
}

// buildTravelURL constructs the Google Travel Hotels search URL.
//
// Format: https://www.google.com/travel/hotels/{location}?q={location}&dates={checkin},{checkout}&adults={n}&hl=en-US&currency={cur}
func buildTravelURL(location string, opts HotelSearchOptions) string {
	encoded := url.PathEscape(location)
	query := url.Values{}
	query.Set("q", location)
	query.Set("dates", opts.CheckIn+","+opts.CheckOut)
	query.Set("adults", fmt.Sprintf("%d", opts.Guests))
	query.Set("hl", "en-US")
	query.Set("gl", "us")
	query.Set("currency", opts.Currency)

	return fmt.Sprintf("https://www.google.com/travel/hotels/%s?%s", encoded, query.Encode())
}

// filterHotels applies all post-fetch filters to hotel results.
func filterHotels(hotels []models.HotelResult, opts HotelSearchOptions) []models.HotelResult {
	filtered := make([]models.HotelResult, 0, len(hotels))
	for _, h := range hotels {
		if opts.Stars > 0 && h.Stars < opts.Stars {
			continue
		}
		if opts.MinPrice > 0 && h.Price > 0 && h.Price < opts.MinPrice {
			continue
		}
		if opts.MaxPrice > 0 && h.Price > 0 && h.Price > opts.MaxPrice {
			continue
		}
		if opts.MinRating > 0 && h.Rating > 0 && h.Rating < opts.MinRating {
			continue
		}
		if opts.MaxDistanceKm > 0 && h.Lat != 0 && h.Lon != 0 && opts.CenterLat != 0 {
			dist := Haversine(opts.CenterLat, opts.CenterLon, h.Lat, h.Lon)
			if dist > opts.MaxDistanceKm {
				continue
			}
		}
		filtered = append(filtered, h)
	}
	return filtered
}

// filterByStars removes hotels below the requested star rating.
func filterByStars(hotels []models.HotelResult, minStars int) []models.HotelResult {
	filtered := make([]models.HotelResult, 0, len(hotels))
	for _, h := range hotels {
		if h.Stars >= minStars {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

// sortHotels sorts hotel results in-place by the given criteria.
func sortHotels(hotels []models.HotelResult, sortBy string, centerLat, centerLon float64) {
	switch strings.ToLower(sortBy) {
	case "cheapest", "price", "":
		// Sort by price ascending. Hotels with price=0 go to the end.
		sort.Slice(hotels, func(i, j int) bool {
			return lessPrice(hotels[i], hotels[j])
		})
	case "rating":
		// Sort by rating descending.
		sort.Slice(hotels, func(i, j int) bool {
			return hotels[i].Rating > hotels[j].Rating
		})
	case "stars":
		// Sort by star rating descending.
		sort.Slice(hotels, func(i, j int) bool {
			return hotels[i].Stars > hotels[j].Stars
		})
	case "distance":
		// Sort by distance from city center ascending.
		if centerLat != 0 || centerLon != 0 {
			sort.Slice(hotels, func(i, j int) bool {
				di := Haversine(centerLat, centerLon, hotels[i].Lat, hotels[i].Lon)
				dj := Haversine(centerLat, centerLon, hotels[j].Lat, hotels[j].Lon)
				return di < dj
			})
		}
	}
}

// Haversine returns the great-circle distance in kilometers between two
// points specified in decimal degrees.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}

func lessPrice(a, b models.HotelResult) bool {
	if a.Price == 0 {
		return false
	}
	if b.Price == 0 {
		return true
	}
	return a.Price < b.Price
}

// parseDateArray converts "YYYY-MM-DD" to [year, month, day].
func parseDateArray(s string) ([3]int, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return [3]int{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD", s)
	}
	return [3]int{t.Year(), int(t.Month()), t.Day()}, nil
}
