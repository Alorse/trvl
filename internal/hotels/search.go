package hotels

import (
	"context"
	"fmt"
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
	Sort     string // "cheapest", "rating", "distance"
	Currency string // default "USD"
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

	// Apply post-filters.
	if opts.Stars > 0 {
		hotels = filterByStars(hotels, opts.Stars)
	}

	// Sort results.
	sortHotels(hotels, opts.Sort)

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
func sortHotels(hotels []models.HotelResult, sortBy string) {
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
	}
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
