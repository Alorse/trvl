package hotels

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

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
// The location can be a city name, address, or any text that Nominatim can
// geocode. Google's hotel search rpcid (AtySUc) appears to work best with
// coordinates rather than text strings, so we geocode the location first.
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

	checkIn, err := parseDateArray(opts.CheckIn)
	if err != nil {
		return nil, fmt.Errorf("parse check-in date: %w", err)
	}
	checkOut, err := parseDateArray(opts.CheckOut)
	if err != nil {
		return nil, fmt.Errorf("parse check-out date: %w", err)
	}

	client := batchexec.NewClient()

	// Strategy 1: Try the text-based search first (sometimes works for
	// well-known cities).
	result, err := tryTextSearch(ctx, client, location, checkIn, checkOut, opts)
	if err == nil && result != nil && len(result.Hotels) > 0 {
		return result, nil
	}

	// Strategy 2: Geocode the location and use coordinates in the payload.
	lat, lon, geoErr := ResolveLocation(ctx, location)
	if geoErr != nil {
		// If geocoding also fails, return the original error.
		if err != nil {
			return nil, fmt.Errorf("hotel search failed (text: %v, geocode: %v)", err, geoErr)
		}
		return nil, fmt.Errorf("geocode %q: %w", location, geoErr)
	}

	result, err = tryCoordinateSearch(ctx, client, location, lat, lon, checkIn, checkOut, opts)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// tryTextSearch attempts a hotel search using the location as a text string.
func tryTextSearch(ctx context.Context, client *batchexec.Client, location string, checkIn, checkOut [3]int, opts HotelSearchOptions) (*models.HotelSearchResult, error) {
	encoded := batchexec.BuildHotelSearchPayload(location, checkIn, checkOut, opts.Guests)

	status, body, err := client.BatchExecute(ctx, encoded)
	if err != nil {
		return nil, fmt.Errorf("hotel search request: %w", err)
	}

	if status == 403 {
		return nil, batchexec.ErrBlocked
	}
	if status != 200 {
		return nil, fmt.Errorf("hotel search returned status %d", status)
	}
	if len(body) < 100 {
		return nil, fmt.Errorf("hotel search returned empty response")
	}

	return decodeHotelResponse(body, opts)
}

// tryCoordinateSearch attempts a hotel search using lat/lon coordinates.
func tryCoordinateSearch(ctx context.Context, client *batchexec.Client, location string, lat, lon float64, checkIn, checkOut [3]int, opts HotelSearchOptions) (*models.HotelSearchResult, error) {
	// Build a coordinate-based payload for AtySUc.
	encoded := buildCoordinateSearchPayload(location, lat, lon, checkIn, checkOut, opts)

	status, body, err := client.BatchExecute(ctx, encoded)
	if err != nil {
		return nil, fmt.Errorf("hotel search request: %w", err)
	}

	if status == 403 {
		return nil, batchexec.ErrBlocked
	}
	if status != 200 {
		return nil, fmt.Errorf("hotel search returned status %d", status)
	}
	if len(body) < 100 {
		return nil, fmt.Errorf("hotel search returned empty response")
	}

	return decodeHotelResponse(body, opts)
}

// buildCoordinateSearchPayload constructs a batchexecute payload for hotel
// search using coordinates instead of text. This is a more reliable approach
// since Google's hotel search appears to work better with coordinates.
//
// The payload structure is based on observed Chrome traffic patterns.
func buildCoordinateSearchPayload(location string, lat, lon float64, checkIn, checkOut [3]int, opts HotelSearchOptions) string {
	// Format coordinates with enough precision.
	latStr := strconv.FormatFloat(lat, 'f', 6, 64)
	lonStr := strconv.FormatFloat(lon, 'f', 6, 64)

	// The AtySUc rpcid payload with coordinates.
	// Based on reverse-engineering, the payload includes the location coordinates
	// and date/guest parameters at specific positions in the nested array.
	args := fmt.Sprintf(
		`[null,null,null,null,null,null,null,null,null,null,[%q,[%s,%s]],`+
			`null,[%d,%d,%d],[%d,%d,%d],null,%d,null,null,null,null,`+
			`null,null,null,null,null,null,null,null,null,%q]`,
		location, latStr, lonStr,
		checkIn[0], checkIn[1], checkIn[2],
		checkOut[0], checkOut[1], checkOut[2],
		opts.Guests,
		opts.Currency,
	)

	return batchexec.EncodeBatchExecute("AtySUc", args)
}

// decodeHotelResponse decodes the raw HTTP response body into hotel results.
func decodeHotelResponse(body []byte, opts HotelSearchOptions) (*models.HotelSearchResult, error) {
	entries, err := batchexec.DecodeBatchResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode hotel response: %w", err)
	}

	hotels, err := ParseHotelSearchResponse(entries, opts.Currency)
	if err != nil {
		return nil, fmt.Errorf("parse hotel results: %w", err)
	}

	// Apply post-filters.
	if opts.Stars > 0 {
		hotels = filterByStars(hotels, opts.Stars)
	}

	// Sort results.
	sortHotels(hotels, opts.Sort)

	return &models.HotelSearchResult{
		Success: true,
		Count:   len(hotels),
		Hotels:  hotels,
	}, nil
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
		for i := 1; i < len(hotels); i++ {
			for j := i; j > 0; j-- {
				if lessPrice(hotels[j], hotels[j-1]) {
					hotels[j], hotels[j-1] = hotels[j-1], hotels[j]
				}
			}
		}
	case "rating":
		// Sort by rating descending.
		for i := 1; i < len(hotels); i++ {
			for j := i; j > 0; j-- {
				if hotels[j].Rating > hotels[j-1].Rating {
					hotels[j], hotels[j-1] = hotels[j-1], hotels[j]
				}
			}
		}
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
